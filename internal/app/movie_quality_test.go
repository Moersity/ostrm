package app

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestMovieQualitySelection(t *testing.T) {
	task := Object{"path": "/movies", "libraryType": "movie"}
	if q := movieQuality(remoteFile{Path: "/movies/4K HDR/Mayday.2026.1080p.SDR.mkv"}); q[0] != 1080 || q[1] != 0 {
		t.Fatal(q)
	}
	fs := []remoteFile{
		{Path: "/movies/Mayday (2026)/Mayday.2026.1080p.BluRay.REMUX.mkv", Size: 5000},
		{Path: "/movies/Mayday (2026)/Mayday.2026.2160p.WEB-DL.HDR.H265.mkv", Size: 2000},
		{Path: "/movies/Mayday (2026)/Mayday.2026.2160p.ATVP.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265.mkv", Size: 3000},
	}
	got, decisions := selectMovieVersions(task, fs)
	if len(got) != 1 || got[0].Path != fs[2].Path || len(decisions) != 2 {
		t.Fatal(got, decisions)
	}
	task["movieVersions"] = "all"
	got, decisions = selectMovieVersions(task, fs)
	if len(got) != 3 || len(decisions) != 0 {
		t.Fatal(got, decisions)
	}
	task["movieVersions"] = "best"
	for _, name := range []string{"Arrival.2020.CD1.1080p.mkv", "Arrival.2020.CD2.1080p.mkv", "Arrival.2020.Directors.Cut.1080p.mkv", "Arrival.2020.Extended.1080p.mkv", "Arrival.2021.1080p.mkv"} {
		fs = append(fs, remoteFile{Path: "/movies/" + name})
	}
	got, _ = selectMovieVersions(task, fs)
	if len(got) != 6 {
		t.Fatal(got)
	}
	// No year: matching titles in unrelated directories are not enough to delete an output.
	got, _ = selectMovieVersions(task, []remoteFile{{Path: "/movies/A/Arrival.1080p.mkv"}, {Path: "/movies/B/Arrival.2160p.mkv"}})
	if len(got) != 2 {
		t.Fatal(got)
	}
	// Deterministic winner with equal evidence, regardless of scan completion order.
	a := remoteFile{Path: "/movies/a/Arrival.2016.mkv", Size: 100}
	b := remoteFile{Path: "/movies/b/Arrival.2016.mkv", Size: 100}
	x, _ := selectMovieVersions(task, []remoteFile{b, a})
	y, _ := selectMovieVersions(task, []remoteFile{a, b})
	if x[0].Path != y[0].Path || x[0].Path != a.Path {
		t.Fatal(x, y)
	}
}

func TestBestMovieUpgradesAndRetiresOwnedVariants(t *testing.T) {
	a := testApp(t)
	low := remoteFile{Name: "Mayday.2026.1080p.mkv", Path: "/movies/Mayday.2026.1080p.mkv", Size: 100}
	high := remoteFile{Name: "Mayday.2026.2160p.DV.mkv", Path: "/movies/Mayday.2026.2160p.DV.mkv", Size: 200}
	mock := &mockList{files: []remoteFile{low}}
	server := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer server.Close()
	root := t.TempDir()
	task := Object{"id": 1, "path": "/movies", "strmPath": root, "libraryType": "movie", "isIncrement": true}
	settings := DefaultSettings()
	c := Object{"baseUrl": server.URL}
	run := func() Object {
		r := Object{"id": 1}
		if e := a.executeFiles(context.Background(), task, c, settings, r); e != nil {
			t.Fatal(e)
		}
		if num(r, "failed") != 0 {
			t.Fatal(r)
		}
		return r
	}
	run()
	task["needScrap"] = true
	obj(settings, "scraping")["useExistingScrapingInfo"] = true
	nfoRel := filepath.Join("Mayday (2026)", "Mayday (2026).nfo")
	if _, e := a.ownedWrite(1, root, nfoRel, low.Path, []byte("<movie><title>Mayday</title></movie>")); e != nil {
		t.Fatal(e)
	}
	p := filepath.Join(root, "Mayday (2026)", "Mayday (2026).strm")
	mock.mu.Lock()
	mock.files = append(mock.files, high)
	mock.mu.Unlock()
	r := run()
	if num(r, "total") != 1 || num(r, "filteredVersions") != 1 {
		t.Fatal(r)
	}
	b, e := os.ReadFile(p)
	if e != nil || !strings.Contains(string(b), high.Name) {
		t.Fatal(string(b), e)
	}
	owned, e := a.Store.Owned(1)
	if e != nil || str(obj(owned, filepath.Join("Mayday (2026)", "Mayday (2026).strm")), "source") != high.Path {
		t.Fatal(owned, e)
	}
	if _, err := os.Stat(filepath.Join(root, nfoRel)); err != nil {
		t.Fatal("metadata lost during quality upgrade", err)
	}
	task["needScrap"] = false
	run() // Persist the changed option before checking the incremental no-op.
	if r = run(); num(r, "changed") != 0 {
		t.Fatal("unchanged selection rewritten", r)
	}
	// Enabling 'all' creates distinct versions, and switching back retires them safely.
	task["movieVersions"] = "all"
	run()
	task["movieVersions"] = "best"
	r = run()
	if num(r, "cleaned") < 2 {
		t.Fatal("old versions not retired", r)
	}
	count := 0
	filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e == nil && strings.HasSuffix(p, ".strm") {
			count++
		}
		return e
	})
	if count != 1 {
		t.Fatal(count)
	}
	trash, e := a.Store.List("trash")
	if e != nil || len(trash) < 2 {
		t.Fatal(trash, e)
	}
	// Removing the best source safely falls back to the remaining copy.
	mock.mu.Lock()
	mock.files = []remoteFile{low}
	mock.mu.Unlock()
	run()
	b, e = os.ReadFile(p)
	if e != nil || !strings.Contains(string(b), low.Name) {
		t.Fatal(string(b), e)
	}
	// A user-edited STRM is not overwritten even when a better version returns.
	os.WriteFile(p, []byte("my custom URL"), 0600)
	mock.mu.Lock()
	mock.files = append(mock.files, high)
	mock.mu.Unlock()
	r = Object{"id": 2}
	e = a.executeFiles(context.Background(), task, c, settings, r)
	if e != nil || num(r, "failed") != 1 {
		t.Fatal(r, e)
	}
	b, _ = os.ReadFile(p)
	if string(b) != "my custom URL" {
		t.Fatal("overwrote user file")
	}
}

func TestKnownMovieIDConnectsBilingualVersions(t *testing.T) {
	task := Object{"path": "/movies", "libraryType": "movie"}
	old := remoteFile{Path: "/movies/Mayday.2026.1080p.mkv"}
	upgraded := remoteFile{Path: "/movies/求救信号 (2026)/求救信号 Mayday (2026) [tmdbid-123] - 2160p DV WEB-DL.mkv"}
	selected, decisions := selectMovieVersions(task, []remoteFile{old, upgraded})
	if len(selected) != 1 || selected[0].Path != upgraded.Path || len(decisions) != 1 {
		t.Fatal(selected, decisions)
	}
	if !canReplaceMovieSource(task, old.Path, upgraded.Path) {
		t.Fatal("cannot upgrade to identified source")
	}
	conflict := remoteFile{Path: "/movies/Mayday (2026) [tmdbid-456].1080p.mkv"}
	selected, _ = selectMovieVersions(task, []remoteFile{old, upgraded, conflict})
	if len(selected) != 3 {
		t.Fatal("conflicting IDs merged", selected)
	}
	if canReplaceMovieSource(task, upgraded.Path, conflict.Path) {
		t.Fatal("conflicting ID allowed to replace source")
	}
}
