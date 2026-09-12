package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestProgressVisibleDuringMetadataRequest(t *testing.T) {
	a := testApp(t)
	entered := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/3/") {
			select {
			case entered <- struct{}{}:
			default:
			}
			<-r.Context().Done()
			return
		}
		json.NewEncoder(w).Encode(Object{"code": 200, "data": Object{"content": []Object{{"name": "Interstellar.2014.mkv", "is_dir": false}}, "total": 1}})
	}))
	defer server.Close()
	task, err := a.Store.Save("tasks", 0, Object{"taskName": "progress", "path": "/media", "strmPath": filepath.Join(a.Config.StrmRoot, "movies"), "libraryType": "movie", "needScrap": true})
	if err != nil {
		t.Fatal(err)
	}
	run, err := a.Store.Save("runs", 0, Object{"taskId": task["id"], "stage": "DISCOVERY", "progress": 0})
	if err != nil {
		t.Fatal(err)
	}
	settings, err := a.settings()
	if err != nil {
		t.Fatal(err)
	}
	settings["tmdb"] = Object{"apiKey": "test", "baseUrl": server.URL, "retryCount": 1}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	done := make(chan struct{})
	go func() { defer close(done); a.execute(ctx, task, Object{"baseUrl": server.URL}, settings, run) }()
	defer func() { cancel(); <-done }()
	select {
	case <-entered:
	case <-ctx.Done():
		t.Fatal("metadata request not reached")
	}
	snapshot, err := a.Store.Get("runs", num(run, "id"))
	if err != nil {
		t.Fatal(err)
	}
	if str(snapshot, "status") != "RUNNING" || str(snapshot, "stage") != "METADATA" || str(snapshot, "currentFile") != "/media/Interstellar.2014.mkv" || str(snapshot, "currentOutput") == "" || num(snapshot, "currentFileIndex") != 1 || num(snapshot, "total") != 1 {
		t.Fatal(snapshot)
	}
	cancel()
	<-done
	snapshot, err = a.Store.Get("runs", num(run, "id"))
	if err != nil || str(snapshot, "status") != "CANCELED" || num(snapshot, "progress") == 100 || str(snapshot, "stage") != "METADATA" {
		t.Fatal(snapshot, err)
	}
}
