package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestManualRenameAmbiguousResponseReconciles(t *testing.T) {
	a := testApp(t)
	var mu sync.Mutex
	fs := map[string]remoteFile{"/media/Old (2020)": {Name: "Old (2020)", Path: "/media/Old (2020)", IsDir: true}, "/media/Old (2020)/old.mkv": {Name: "old.mkv", Path: "/media/Old (2020)/old.mkv", Size: 10}}
	calls := map[string]int{}
	uploads := 0
	failed := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		reply := func(m any) { json.NewEncoder(w).Encode(m) }
		if strings.HasPrefix(r.URL.Path, "/3/") {
			if strings.Contains(r.URL.Path, "search") {
				reply(Object{"results": []Object{{"id": 1}}})
			} else {
				reply(Object{"id": 1, "title": "Movie", "release_date": "2020-01-01", "overview": "Plot"})
			}
			return
		}
		if r.URL.Path == "/api/fs/put" {
			uploads++
			reply(Object{"code": 200})
			return
		}
		var q Object
		json.NewDecoder(r.Body).Decode(&q)
		switch r.URL.Path {
		case "/api/fs/list":
			files := []remoteFile{}
			for p, f := range fs {
				if path.Dir(p) == str(q, "path") {
					files = append(files, f)
				}
			}
			reply(Object{"code": 200, "data": Object{"content": files, "total": len(files)}})
		case "/api/fs/rename":
			src := str(q, "path")
			calls[src]++
			dest := path.Join(path.Dir(src), str(q, "name"))
			for p, f := range fs {
				if p == src || strings.HasPrefix(p, src+"/") {
					delete(fs, p)
					f.Path = dest + strings.TrimPrefix(p, src)
					f.Name = path.Base(f.Path)
					fs[f.Path] = f
				}
			}
			if !failed {
				failed = true
				w.WriteHeader(500)
				return
			}
			reply(Object{"code": 200, "data": Object{}})
		default:
			reply(Object{"code": 404})
		}
	}))
	defer server.Close()
	s := DefaultSettings()
	obj(s, "tmdb")["apiKey"] = "test"
	obj(s, "tmdb")["baseUrl"] = server.URL
	if e := a.Store.Set("system", s); e != nil {
		t.Fatal(e)
	}
	c, _ := a.Store.Save("openlist", 0, Object{"baseUrl": server.URL, "token": "test"})
	task, _ := a.Store.Save("tasks", 0, Object{"path": "/media", "taskName": "test", "libraryType": "movie", "openlistConfigId": c["id"]})
	id := num(task, "id")
	job, e := a.submitManual(id, Object{"directoryPath": "/media/Old (2020)", "tmdbId": 1, "renameMedia": true})
	if e != nil {
		t.Fatal(e)
	}
	wait := func() Object {
		for i := 0; i < 200; i++ {
			a.mu.Lock()
			active := a.active[id] != nil
			a.mu.Unlock()
			if !active {
				m, e := a.Store.Get("manual", num(job, "id"))
				if e != nil {
					t.Fatal(e)
				}
				return m
			}
			time.Sleep(10 * time.Millisecond)
		}
		t.Fatal("manual timeout")
		return nil
	}
	result := wait()
	if str(result, "status") != "FAILED" {
		t.Fatal(result)
	}
	if _, e = a.retryManual(id, num(job, "id")); e != nil {
		t.Fatal(e)
	}
	result = wait()
	if str(result, "status") != "SUCCEEDED" {
		t.Fatal(result)
	}
	mu.Lock()
	defer mu.Unlock()
	if calls["/media/Old (2020)/old.mkv"] != 1 {
		t.Fatal("rename repeated", calls)
	}
	if uploads != 1 {
		t.Fatal("expected generated NFO upload", uploads)
	}
}
func TestTMDBAndConfigurablePatterns(t *testing.T) {
	a := testApp(t)
	s := DefaultSettings()
	m := capturePatterns(obj(s, "scrapingRegex")["movieRegexps"], "The.Matrix.1999.1080p.mkv")
	if str(m, "title") != "The.Matrix" || str(m, "year") != "1999" {
		t.Fatal(m)
	}
	if !video("foo.3g2", s) {
		t.Fatal("baseline extension missing")
	}
	s["mediaExtensions"] = []any{".custom"}
	if video("foo.mp4", s) || !video("foo.custom", s) {
		t.Fatal("extension setting ignored")
	}
	_ = a
	_ = context.Background()
}
