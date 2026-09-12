package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLatestTaskRecord(t *testing.T) {
	a := testApp(t)
	for _, item := range []struct {
		kind string
		task int64
	}{{"runs", 1}, {"manual", 1}, {"runs", 2}, {"runs", 1}, {"manual", 2}} {
		if _, err := a.Store.Save(item.kind, 0, Object{"taskId": item.task, "status": "SUCCESS"}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tc := range []struct {
		kind     string
		task, id int64
	}{{"runs", 1, 3}, {"runs", 2, 2}, {"manual", 1, 1}, {"manual", 2, 2}, {"runs", 99, 0}} {
		got, err := a.Store.LatestTaskRecord(tc.kind, tc.task)
		if err != nil || num(got, "id") != tc.id {
			t.Fatalf("%+v: %v, %v", tc, got, err)
		}
	}
	// Updating an old run must not make it the latest execution.
	if _, err := a.Store.Save("runs", 1, Object{"taskId": 1, "status": "FAILED"}); err != nil {
		t.Fatal(err)
	}
	got, err := a.Store.LatestTaskRecord("runs", 1)
	if err != nil || num(got, "id") != 3 {
		t.Fatal(got, err)
	}
	if _, err = a.Store.LatestTaskRecord("tasks", 1); err == nil {
		t.Fatal("unexpected record kind accepted")
	}
	rows, err := a.Store.DB.Query(`EXPLAIN QUERY PLAN SELECT body FROM records WHERE kind IN ('runs','manual') AND kind=? AND json_extract(body,'$.taskId')=? ORDER BY id DESC LIMIT 1`, "runs", 1)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	used := false
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err = rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(detail, "records_task_latest") {
			used = true
		}
	}
	if err = rows.Err(); err != nil {
		t.Fatal(err)
	}
	if !used {
		t.Fatal("latest execution query did not use its index")
	}
}

func TestPathsOverlapRootAndSiblingBoundaries(t *testing.T) {
	base := t.TempDir()
	root := filepath.VolumeName(base) + string(filepath.Separator)
	for _, tc := range []struct {
		a, b string
		want bool
	}{
		{root, base, true}, {base, root, true}, {base, base, true},
		{base, filepath.Join(base, "child"), true},
		{filepath.Join(base, "movies"), filepath.Join(base, "movies-old"), false},
		{filepath.Join(base, "a"), filepath.Join(base, "b"), false},
	} {
		if got := pathsOverlap(tc.a, tc.b); got != tc.want {
			t.Fatalf("overlap(%q,%q)=%v", tc.a, tc.b, got)
		}
	}
}

func TestScanStopsOtherDirectoriesOnFailure(t *testing.T) {
	a := testApp(t)
	var calls atomic.Int32
	ready := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q Object
		if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
			return
		}
		if str(q, "path") == "/media" {
			files := []Object{}
			for i := 0; i < 24; i++ {
				files = append(files, Object{"name": fmt.Sprintf("dir-%02d", i), "is_dir": true})
			}
			json.NewEncoder(w).Encode(Object{"code": 200, "data": Object{"content": files, "total": len(files)}})
			return
		}
		if calls.Add(1) == 4 {
			close(ready)
		}
		if strings.HasSuffix(str(q, "path"), "dir-00") {
			select {
			case <-ready:
				w.WriteHeader(http.StatusForbidden)
			case <-r.Context().Done():
			}
			return
		}
		<-r.Context().Done()
	}))
	defer server.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	_, err := a.scan(ctx, Object{"baseUrl": server.URL}, "/media")
	if err == nil || !strings.Contains(err.Error(), "403") || !strings.Contains(err.Error(), "dir-00") {
		t.Fatalf("lost original directory error: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatal("scan waited for unrelated directories to time out")
	}
	if calls.Load() != 4 {
		t.Fatalf("continued queued requests after failure: %d", calls.Load())
	}
}

func TestCanceledLimiterDoesNotConsumeQuota(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	l := &limiter{}
	if err := l.wait(ctx, 1, 10); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if len(l.hits) != 0 {
		t.Fatal("canceled request consumed rate limit quota")
	}
}

func TestLatestExecutionAPIContract(t *testing.T) {
	a := testApp(t)
	session, err := a.token("admin")
	if err != nil {
		t.Fatal(err)
	}
	handler := a.Handler()
	for _, route := range []struct{ kind, path string }{{"runs", "runs"}, {"manual", "manual-scraping/jobs"}} {
		endpoint := "/api/task-config/42/" + route.path + "/latest"
		status, response := call(t, handler, "GET", endpoint, str(session, "token"), nil)
		if status != 200 || num(response, "code") != 200 || response["data"] != nil {
			t.Fatal(response)
		}
		want, err := a.Store.Save(route.kind, 0, Object{"taskId": 42, "status": "RUNNING", "plan": Object{"secret": "hidden"}, "steps": Object{}})
		if err != nil {
			t.Fatal(err)
		}
		if _, err = a.Store.Save(route.kind, 0, Object{"taskId": 99}); err != nil {
			t.Fatal(err)
		}
		_, response = call(t, handler, "GET", endpoint, str(session, "token"), nil)
		got := obj(response, "data")
		if num(got, "id") != num(want, "id") || got["plan"] != nil || got["steps"] != nil {
			t.Fatal(response)
		}
	}
}

func TestLatestIndexCreatedForExistingHistory(t *testing.T) {
	dir := t.TempDir()
	store, err := OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Save("runs", 0, Object{"taskId": 7}); err != nil {
		t.Fatal(err)
	}
	if _, err = store.DB.Exec("DROP INDEX records_task_latest"); err != nil {
		t.Fatal(err)
	}
	if err = store.DB.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = OpenStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer store.DB.Close()
	record, err := store.LatestTaskRecord("runs", 7)
	if err != nil || num(record, "id") != 1 {
		t.Fatal(record, err)
	}
	var count int
	if err = store.DB.QueryRow("SELECT count(*) FROM sqlite_master WHERE type='index' AND name='records_task_latest'").Scan(&count); err != nil || count != 1 {
		t.Fatal(count, err)
	}
}
