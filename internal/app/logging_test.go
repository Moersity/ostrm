package app

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func readLogRecords(t *testing.T, a *App, kind string) []Object {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(a.Config.DataDir, "logs", kind+".log"))
	if err != nil {
		t.Fatal(err)
	}
	var records []Object
	for _, line := range strings.Split(strings.TrimSpace(string(b)), "\n") {
		if line == "" {
			continue
		}
		var record Object
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatal(err)
		}
		records = append(records, record)
	}
	return records
}

func TestTaskLogsCorrelateFilesAndFailures(t *testing.T) {
	for _, scenario := range []string{"success", "scan_failure", "metadata_failure", "canceled"} {
		t.Run(scenario, func(t *testing.T) {
			a := testApp(t)
			mock := &mockList{files: []remoteFile{{Name: "movie.mkv", Path: "/media/movie.mkv", Size: 100}}, fail: scenario == "scan_failure"}
			server := httptest.NewServer(http.HandlerFunc(mock.handler))
			defer server.Close()
			task, err := a.Store.Save("tasks", 0, Object{"taskName": "电影任务", "path": "/media", "strmPath": filepath.Join(a.Config.StrmRoot, "movies"), "needScrap": scenario == "metadata_failure"})
			if err != nil {
				t.Fatal(err)
			}
			run, err := a.Store.Save("runs", 0, Object{"taskId": task["id"], "stage": "DISCOVERY"})
			if err != nil {
				t.Fatal(err)
			}
			settings, err := a.settings()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			if scenario == "canceled" {
				cancel()
			}
			a.execute(ctx, task, Object{"baseUrl": server.URL, "token": "private-token"}, settings, run)
			records := readLogRecords(t, a, "backend")
			foundFile, foundFinish := false, false
			for _, record := range records {
				if num(record, "taskId") != num(task, "id") || num(record, "runId") != num(run, "id") || str(record, "taskName") != "电影任务" {
					t.Fatalf("missing correlation: %v", record)
				}
				if str(record, "msg") == "开始处理文件" {
					foundFile = true
					if str(record, "sourcePath") != "/media/movie.mkv" || str(record, "outputPath") == "" {
						t.Fatal(record)
					}
				}
				if str(record, "msg") == "任务结束" {
					foundFinish = true
					if record["durationMs"] == nil {
						t.Fatal(record)
					}
				}
				encoded, _ := json.Marshal(record)
				if strings.Contains(string(encoded), "private-token") {
					t.Fatal("credential in log")
				}
			}
			if !foundFinish || ((scenario == "success" || scenario == "metadata_failure") && !foundFile) {
				t.Fatal(records)
			}
			errors := readLogRecords(t, a, "error")
			switch scenario {
			case "success", "canceled":
				if len(errors) != 0 {
					t.Fatal(errors)
				}
			case "scan_failure":
				if str(run, "status") != "FAILED" || !strings.Contains(str(run, "errorMessage"), "/media") || len(errors) == 0 {
					t.Fatal(run, errors)
				}
			case "metadata_failure":
				found := false
				for _, record := range errors {
					if str(record, "msg") == "文件处理失败" {
						found = true
						if str(record, "stage") != "METADATA" || !strings.Contains(str(record, "error"), "TMDB") || str(record, "sourcePath") != "/media/movie.mkv" {
							t.Fatal(record)
						}
					}
				}
				if !found || str(run, "status") != "PARTIAL_SUCCESS" {
					t.Fatal(run, errors)
				}
			}
		})
	}
}

func TestIncrementalSkipLoggingLevel(t *testing.T) {
	a := testApp(t)
	mock := &mockList{files: []remoteFile{{Name: "movie.mkv", Path: "/media/movie.mkv"}}}
	server := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer server.Close()
	task, _ := a.Store.Save("tasks", 0, Object{"taskName": "movies", "path": "/media", "strmPath": filepath.Join(a.Config.StrmRoot, "movies"), "isIncrement": true})
	settings, _ := a.settings()
	for i := 0; i < 3; i++ {
		if i == 2 {
			a.logLevel.Set(slog.LevelDebug)
		}
		run, _ := a.Store.Save("runs", 0, Object{"taskId": task["id"], "stage": "DISCOVERY"})
		a.execute(context.Background(), task, Object{"baseUrl": server.URL}, settings, run)
		if i > 0 && num(run, "skipped") != 1 {
			t.Fatal(run)
		}
	}
	skips := 0
	for _, record := range readLogRecords(t, a, "backend") {
		if str(record, "msg") == "跳过未变化文件" {
			skips++
			if str(record, "level") != "DEBUG" {
				t.Fatal(record)
			}
		}
	}
	if skips != 1 {
		t.Fatalf("expected only debug run to log skips, got %d", skips)
	}
}
