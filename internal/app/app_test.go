package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func testApp(t *testing.T) *App {
	t.Helper()
	a, e := New(Config{DataDir: t.TempDir(), Listen: "127.0.0.1:0"})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { a.Close() })
	return a
}
func call(t *testing.T, h http.Handler, method, path, token string, body Object) (int, Object) {
	t.Helper()
	b, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(b))
	if token != "" {
		r.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	var m Object
	if e := json.Unmarshal(w.Body.Bytes(), &m); e != nil {
		t.Fatalf("%s: %s", path, w.Body.String())
	}
	return w.Code, m
}
func TestAuthLifecycle(t *testing.T) {
	a := testApp(t)
	h := a.Handler()
	_, m := call(t, h, "GET", "/api/auth/check-user", "", nil)
	if num(m, "code") != 404 {
		t.Fatal(m)
	}
	_, m = call(t, h, "POST", "/api/auth/sign-up", "", Object{"username": "admin", "password": "secret123"})
	if num(m, "code") != 200 {
		t.Fatal(m)
	}
	_, m = call(t, h, "POST", "/api/auth/sign-in", "", Object{"username": "admin", "password": "secret123"})
	token := str(obj(m, "data"), "token")
	if token == "" {
		t.Fatal(m)
	}
	status, _ := call(t, h, "GET", "/api/task-config", "", nil)
	if status != 401 {
		t.Fatal(status)
	}
	_, m = call(t, h, "POST", "/api/auth/refresh", token, nil)
	newToken := str(obj(m, "data"), "token")
	if newToken == "" {
		t.Fatal(m)
	}
	status, _ = call(t, h, "GET", "/api/auth/validate", token, nil)
	if status != 401 {
		t.Fatal("old session remains valid")
	}
	_, m = call(t, h, "POST", "/api/auth/change-password", newToken, Object{"oldPassword": "secret123", "newPassword": "different456"})
	if num(m, "code") != 200 {
		t.Fatal(m)
	}
	status, _ = call(t, h, "GET", "/api/auth/validate", newToken, nil)
	if status != 401 {
		t.Fatal("password change did not revoke session")
	}
}
func TestRegistrationAndInstanceExclusion(t *testing.T) {
	a := testApp(t)
	var wg sync.WaitGroup
	success := make(chan bool, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			success <- a.register(Object{"username": "admin", "password": "secret123"}) == nil
		}()
	}
	wg.Wait()
	n := 0
	for i := 0; i < 2; i++ {
		if <-success {
			n++
		}
	}
	if n != 1 {
		t.Fatalf("registered %d", n)
	}
	if b, e := New(a.Config); e == nil {
		b.Close()
		t.Fatal("second writer accepted")
	}
}
func TestURLGolden(t *testing.T) {
	cases := map[string]string{"http://host/d/电影 a+b.mkv?sign=a=:0": "http://host/d/%E7%94%B5%E5%BD%B1%20a+b.mkv?sign=a%3D%3A0", "https://host/d/%E7%94%B5%20movie.mkv?sign=a%3D%3A0": "https://host/d/%E7%94%B5%20movie.mkv?sign=a%3D%3A0"}
	for in, want := range cases {
		got := smartURL(in)
		if got != want {
			t.Fatalf("%s != %s", got, want)
		}
		if smartURL(got) != got {
			t.Fatal("double encoding")
		}
	}
}
func TestOutputPathsAndSymlinks(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	if _, e := outputPath("/tv", "/tv/../secret.mkv", ""); e == nil {
		t.Fatal("traversal accepted")
	}
	if safeName("CON") == "CON" {
		t.Fatal("reserved name")
	}
	if e := os.Symlink(outside, filepath.Join(root, "escape")); e == nil {
		if _, e = writeOutput(root, "escape/test.strm", []byte("x")); e == nil {
			t.Fatal("symlink escape")
		}
	}
	if _, e := writeOutput(root, "a/test.strm", []byte("url")); e != nil {
		t.Fatal(e)
	}
	changed, e := writeOutput(root, "a/test.strm", []byte("url"))
	if e != nil || changed {
		t.Fatal(changed, e)
	}
}

type mockList struct {
	mu    sync.Mutex
	files []remoteFile
	fail  bool
}

func (m *mockList) handler(w http.ResponseWriter, r *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.fail {
		w.WriteHeader(403)
		return
	}
	var q Object
	json.NewDecoder(r.Body).Decode(&q)
	fs := []remoteFile{}
	for _, f := range m.files {
		if strings.TrimSuffix(f.Path, "/") != "" && filepath.ToSlash(filepath.Dir(f.Path)) == str(q, "path") {
			fs = append(fs, f)
		}
	}
	json.NewEncoder(w).Encode(Object{"code": 200, "data": Object{"content": fs, "total": len(fs)}})
}
func waitRun(t *testing.T, a *App, id int64) Object {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		rs, e := a.Store.List("runs")
		if e != nil {
			t.Fatal(e)
		}
		if len(rs) > 0 {
			r := rs[len(rs)-1]
			s := str(r, "status")
			a.mu.Lock()
			active := a.active[id] != nil
			a.mu.Unlock()
			if !active && s != "RUNNING" && s != "QUEUED" {
				return r
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("run timed out")
	return nil
}
func TestFullIncrementalAndFailureSafety(t *testing.T) {
	a := testApp(t)
	mock := &mockList{files: []remoteFile{{Name: "movie.mkv", Path: "/media/movie.mkv", Sign: "a=:0", Size: 100}}}
	server := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer server.Close()
	c, e := a.Store.Save("openlist", 0, Object{"baseUrl": server.URL, "token": "test", "isActive": true})
	if e != nil {
		t.Fatal(e)
	}
	root := filepath.Join(a.Config.StrmRoot, "movies")
	task, e := a.Store.Save("tasks", 0, Object{"taskName": "movies", "path": "/media", "openlistConfigId": c["id"], "strmPath": root, "isIncrement": true})
	if e != nil {
		t.Fatal(e)
	}
	id := num(task, "id")
	run := func() Object {
		if _, e := a.submit(id, nil); e != nil {
			t.Fatal(e)
		}
		return waitRun(t, a, id)
	}
	r := run()
	if str(r, "status") != "SUCCESS" {
		t.Fatal(r)
	}
	p := filepath.Join(root, "movie.strm")
	b, e := os.ReadFile(p)
	if e != nil || !strings.Contains(string(b), "sign=a%3D%3A0") {
		t.Fatal(string(b), e)
	}
	stat, _ := os.Stat(p)
	r = run()
	stat2, _ := os.Stat(p)
	if num(r, "changed") != 0 || stat.ModTime() != stat2.ModTime() {
		t.Fatal("unchanged rewritten", r)
	}
	mock.mu.Lock()
	mock.files[0].Sign = "new"
	mock.mu.Unlock()
	r = run()
	if num(r, "changed") != 1 {
		t.Fatal(r)
	}
	os.Remove(p)
	r = run()
	if num(r, "changed") != 1 {
		t.Fatal("missing not rebuilt", r)
	}
	os.WriteFile(filepath.Join(root, "user.nfo"), []byte("user"), 0600)
	mock.mu.Lock()
	mock.fail = true
	mock.mu.Unlock()
	r = run()
	if str(r, "status") != "FAILED" {
		t.Fatal(r)
	}
	if _, e = os.Stat(p); e != nil {
		t.Fatal("failed scan deleted output")
	}
	mock.mu.Lock()
	mock.fail = false
	mock.files = nil
	mock.mu.Unlock()
	r = run()
	if num(r, "cleaned") != 1 {
		t.Fatal(r)
	}
	if _, e = os.Stat(filepath.Join(root, "user.nfo")); e != nil {
		t.Fatal("deleted unowned")
	}
}
func TestCronQuartzSemantics(t *testing.T) {
	for _, tc := range []struct{ expr, want string }{{"0 0 12 ? * MON", "2026-09-14T12:00:00Z"}, {"0 0 12 L * ?", "2026-09-30T12:00:00Z"}, {"0 0 12 ? * 6#3", "2026-09-18T12:00:00Z"}} {
		tr, e := cronTrigger(tc.expr, "UTC")
		if e != nil {
			t.Fatal(e)
		}
		base, _ := time.Parse(time.RFC3339, "2026-09-08T00:00:00Z")
		n, e := tr.NextFireTime(base.UnixNano())
		if e != nil {
			t.Fatal(e)
		}
		if got := time.Unix(0, n).UTC().Format(time.RFC3339); got != tc.want {
			t.Fatal(tc.expr, got, tc.want)
		}
	}
}
func TestRateLimitCancellation(t *testing.T) {
	l := &limiter{}
	if e := l.wait(context.Background(), 1, 0); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()
	if e := l.wait(ctx, 1, 0); e == nil {
		t.Fatal("cancel ignored")
	}
}
func TestBackupReopen(t *testing.T) {
	a := testApp(t)
	m, e := a.Store.Save("tasks", 0, Object{"taskName": "persist"})
	if e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(t.TempDir(), "backup.db")
	if e = a.Store.Backup(p); e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(p)
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
	got, e := a.Store.Get("tasks", num(m, "id"))
	if e != nil || str(got, "taskName") != "persist" {
		t.Fatal(got, e)
	}
}
func TestStaticApp(t *testing.T) {
	a := testApp(t)
	h := a.Handler()
	for _, p := range []string{"/", "/task-management/1"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != 200 || !strings.Contains(w.Body.String(), "_nuxt") {
			t.Fatal(p, w.Code, "missing generated UI")
		}
	}
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/_nuxt/missing.js", nil))
	if w.Code != 404 {
		t.Fatal(w.Code)
	}
}
func TestStructureAndRegex(t *testing.T) {
	if structureReason("Movie/file.mkv", "movie") != "" {
		t.Fatal("movie rejected")
	}
	if structureReason("Show/Season 01/file.mkv", "tv") != "" {
		t.Fatal("tv rejected")
	}
	if structureReason("file.mkv", "movie") == "" {
		t.Fatal("flat movie accepted")
	}
	got, e := renameFile("movie [test].mkv", `\s*\[.*?\]|`)
	if e != nil || got != "movie.strm" {
		t.Fatal(got, e)
	}
	got, e = renameFile("Show S01E02.mkv", `(?<=S)01|02`)
	if e != nil || got != "Show S02E02.strm" {
		t.Fatal(got, e)
	}
}

var _ = fmt.Sprint
