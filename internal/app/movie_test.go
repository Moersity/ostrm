package app

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"os"
	"path"
	"path/filepath"
	"strings"
	"testing"
)

func TestMovieNamesAndNestedIdentity(t *testing.T) {
	cases := []struct {
		source, title, year string
		id                  int64
	}{
		{"/movies/The.Matrix.1999.1080p.BluRay.x264-GROUP.mkv", "The Matrix", "1999", 0},
		{"/movies/求救信号 Mayday (2026)/Mayday.2026.2160p.ATVP.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265.mkv", "Mayday", "2026", 0},
		{"/movies/科幻/合集/流浪地球 (2019)/2160p/正片.mkv", "流浪地球", "2019", 0},
		{"/movies/科幻/Interstellar.2014/1080p/movie.mkv", "Interstellar", "2014", 0},
		{"/movies/合集/Movie20201234.mkv", "Movie20201234", "", 0},
		{"/movies/1917.2019.1080p.mkv", "1917", "2019", 0},
		{"/movies/2001.A.Space.Odyssey.1968.mkv", "2001 A Space Odyssey", "1968", 0},
		{"/movies/1984.mkv", "1984", "", 0},
		{"/movies/[GROUP] The.Matrix.1999.WEB-DL.mkv", "The Matrix", "1999", 0},
		{"/movies/[Mayday] (2026).2160p.mkv", "Mayday", "2026", 0},
		{"/movies/【求救信号】.2026.2160p.mkv", "求救信号", "2026", 0},
		{"/movies/The.Matrix.1999.[tmdbid-603].mkv", "The Matrix", "1999", 603},
		{"/movies/合集 (2020)/Other.2010.mkv", "Other", "2010", 0},
		{"/movies/The.Matrix (1999) {tmdb=603}/The.Matrix.mkv", "The Matrix", "1999", 603},
		{"/movies/合集/Amelie.1080p.WEB-DL.x265.mkv", "Amelie", "", 0},
	}
	for _, tc := range cases {
		t.Run(tc.source, func(t *testing.T) {
			got := identifyMovie("/movies", tc.source)
			if got.Title != tc.title || got.Year != tc.year || got.ID != tc.id {
				t.Fatalf("got %+v, want %s %s %d", got, tc.title, tc.year, tc.id)
			}
			if reason := structureReason(strings.TrimPrefix(tc.source, "/movies/"), "movie"); reason != "" {
				t.Fatal(reason)
			}
		})
	}
	if got := identifyMovie("/movies/Arrival.2020", "/movies/Arrival.2020/movie.mkv"); got.Title != "Arrival" || got.Year != "2020" {
		t.Fatal(got)
	}
	if got := identifyMovie("/movies", "/movies/movie.mkv"); usefulMovieTitle(got.Title) {
		t.Fatal("invented a title from library root", got)
	}
}

func TestNestedMovieOutputsMetadataAndIncremental(t *testing.T) {
	a := testApp(t)
	mock := &mockList{}
	paths := []string{
		"/movies/合集/科幻/Arrival.2020/1080p/movie.mkv",
		"/movies/合集/科幻/Arrival.2020/2160p/movie.mkv",
		"/movies/Other.2021.WEB-DL.mkv",
		"/movies/合集/Arrival.2020/Sample/sample.mkv",
	}
	dirs := map[string]bool{}
	for _, p := range paths {
		mock.files = append(mock.files, remoteFile{Path: p, Name: path.Base(p), Sign: "signed=:0", Size: 100})
		for d := path.Dir(p); d != "/movies"; d = path.Dir(d) {
			dirs[d] = true
		}
	}
	for d := range dirs {
		mock.files = append(mock.files, remoteFile{Path: d, Name: path.Base(d), IsDir: true})
	}
	queries := map[string]int{}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.URL.Path == "/api/fs/list":
			mock.handler(w, r)
		case r.URL.Path == "/3/search/movie":
			q := r.URL.Query().Get("query")
			queries[q]++
			id := 1
			if q == "Other" {
				id = 2
			}
			if q != "Arrival" && q != "Other" {
				t.Errorf("wrong query %q", q)
			}
			json.NewEncoder(w).Encode(Object{"results": []Object{{"id": id}}})
		case strings.HasPrefix(r.URL.Path, "/3/movie/"):
			title, year := "Arrival", "2020"
			if strings.HasSuffix(r.URL.Path, "/2") {
				title, year = "Other", "2021"
			}
			json.NewEncoder(w).Encode(Object{"title": title, "release_date": year + "-01-01", "poster_path": "/poster.jpg"})
		case strings.HasSuffix(r.URL.Path, "poster.jpg"):
			w.Write([]byte("poster"))
		default:
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s := DefaultSettings()
	obj(s, "tmdb")["baseUrl"] = server.URL
	obj(s, "tmdb")["imageBaseUrl"] = server.URL
	obj(s, "tmdb")["apiKey"] = "test"
	if e := a.Store.Set("system", s); e != nil {
		t.Fatal(e)
	}
	root := t.TempDir()
	task := Object{"id": 1, "path": "/movies", "strmPath": root, "libraryType": "movie", "skipInvalidStructure": true, "movieVersions": "all", "needScrap": true, "isIncrement": true}
	c := Object{"baseUrl": server.URL, "enableUrlEncoding": true}
	run := func() Object {
		r := Object{"id": 1}
		if e := a.executeFiles(context.Background(), task, c, s, r); e != nil {
			t.Fatal(e)
		}
		if num(r, "failed") != 0 {
			t.Fatal(r)
		}
		return r
	}
	first := run()
	if num(first, "total") != 3 {
		t.Fatal(first)
	}
	strms := []string{}
	filepath.WalkDir(root, func(p string, d os.DirEntry, e error) error {
		if e != nil {
			return e
		}
		if strings.HasSuffix(p, ".strm") {
			strms = append(strms, p)
		}
		return nil
	})
	if len(strms) != 3 {
		t.Fatal(strms)
	}
	for _, p := range strms {
		raw, e := os.ReadFile(p)
		if e != nil || !strings.Contains(string(raw), "sign=signed%3D%3A0") {
			t.Fatal(string(raw), e)
		}
		b, e := os.ReadFile(strings.TrimSuffix(p, ".strm") + ".nfo")
		if e != nil {
			t.Fatal(e)
		}
		var n nfo
		if e = xml.Unmarshal(b, &n); e != nil {
			t.Fatal(e)
		}
		if n.Title != "Arrival" && n.Title != "Other" || n.Unique.Type != "tmdb" || n.Unique.Value == "" {
			t.Fatal(n)
		}
		if _, e = os.Stat(strings.TrimSuffix(p, ".strm") + "-poster.jpg"); e != nil {
			t.Fatal(e)
		}
	}
	second := run()
	if num(second, "changed") != 0 || num(second, "skipped") != 3 {
		t.Fatal(second)
	}
	if queries["Arrival"] != 1 || queries["Other"] != 1 {
		t.Fatal(queries)
	}
	// No remote mutations are used during normal STRM generation (the server rejects them).
}

func TestMovieOriginalModeAndExtras(t *testing.T) {
	task := Object{"path": "/movies", "libraryType": "movie"}
	got, e := taskOutputPath(task, "/movies/合集/Arrival.2020.1080p.mkv")
	if e != nil || filepath.ToSlash(got) != "Arrival (2020)/Arrival (2020).strm" {
		t.Fatal(got, e)
	}
	task["movieNaming"] = "original"
	got, e = taskOutputPath(task, "/movies/合集/Arrival.2020.1080p.mkv")
	if e != nil || filepath.ToSlash(got) != "合集/Arrival.2020.1080p.strm" {
		t.Fatal(got, e)
	}
	for _, p := range []string{"Arrival/Sample/sample.MKV", "Arrival/extras/interview.mp4", "Arrival.2020-trailer.mp4"} {
		if !movieExtra(p) {
			t.Fatal(p)
		}
	}
	for _, p := range []string{"The.Sample.2020.mkv", "Trailer.Park.Boys.2006.mkv"} {
		if movieExtra(p) {
			t.Fatal(p)
		}
	}
}

func TestMaydaySearchAndAmbiguousMatches(t *testing.T) {
	a := testApp(t)
	searches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3/search/movie" {
			searches++
			if r.URL.Query().Get("query") != "Mayday" || r.URL.Query().Get("year") != "2026" {
				t.Errorf("wrong query %s", r.URL.RawQuery)
			}
			json.NewEncoder(w).Encode(Object{"results": []Object{
				{"id": 1, "title": "求救信号", "original_title": "Mayday", "release_date": "2021-01-01"},
				{"id": 2, "title": "求救信号", "original_title": "Mayday", "release_date": "2026-01-01"},
			}})
		} else if r.URL.Path == "/3/movie/2" {
			json.NewEncoder(w).Encode(Object{"title": "求救信号", "original_title": "Mayday", "release_date": "2026-01-01"})
		} else {
			t.Errorf("wrong endpoint %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s := DefaultSettings()
	obj(s, "tmdb")["apiKey"] = "test"
	obj(s, "tmdb")["baseUrl"] = server.URL
	input := "Mayday.2026.2160p.ATVP.WEB-DL.DDP5.1.Atmos.DV.HDR.H.265.mkv"
	m, e := a.recognize(context.Background(), s, input, "movie", "", "", 0)
	if e != nil || num(m, "tmdbId") != 2 || str(m, "title") != "求救信号" || searches != 1 {
		t.Fatal(m, e, searches)
	}
	rs := []any{map[string]any{"id": float64(1), "title": "Mayday", "release_date": "2026-01-01"}, map[string]any{"id": float64(2), "title": "Mayday", "release_date": "2026-01-01"}}
	if _, e = selectMovieResult(rs, "Mayday", "2026"); e == nil {
		t.Fatal("ambiguous result silently accepted")
	}
}

func TestMovieSidecarsStayWithTheirFilm(t *testing.T) {
	a := testApp(t)
	source := remoteFile{Name: "Mayday.2026.mkv", Path: "/movies/Mayday.2026.mkv"}
	files := []remoteFile{source, {Name: "Other.2021.mkv", Path: "/movies/Other.2021.mkv"}}
	bodies := map[string]string{
		"Mayday.2026.nfo":        "<movie><title>求救信号</title></movie>",
		"Mayday.2026.zh.srt":     "subtitle",
		"Mayday.2026-poster.jpg": "correct poster",
		"Other.2021.nfo":         "WRONG",
		"Other.2021-poster.jpg":  "WRONG",
		"movie.nfo":              "AMBIGUOUS",
		"poster.jpg":             "AMBIGUOUS",
	}
	for name := range bodies {
		files = append(files, remoteFile{Name: name, Path: "/movies/" + name})
	}
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/fs/get" {
			var q Object
			json.NewDecoder(r.Body).Decode(&q)
			json.NewEncoder(w).Encode(Object{"code": 200, "data": Object{"raw_url": server.URL + "/asset/" + path.Base(str(q, "path"))}})
		} else if strings.HasPrefix(r.URL.Path, "/asset/") {
			body := bodies[path.Base(r.URL.Path)]
			if body == "WRONG" || body == "AMBIGUOUS" {
				t.Error("read unrelated sidecar", r.URL.Path)
			}
			w.Write([]byte(body))
		} else {
			t.Errorf("unexpected request %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s := DefaultSettings()
	obj(s, "scraping")["useExistingScrapingInfo"] = true
	obj(s, "scraping")["keepSubtitleFiles"] = true
	task := Object{"path": "/movies", "strmPath": t.TempDir(), "libraryType": "movie"}
	written := map[string]string{}
	e := a.processMetadata(context.Background(), task, Object{"baseUrl": server.URL}, s, source, files, filepath.FromSlash("Mayday (2026)/Mayday (2026).strm"), func(rel, src string, b []byte) error {
		if src != source.Path {
			t.Error(src)
		}
		written[filepath.ToSlash(rel)] = string(b)
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	for _, suffix := range []string{".nfo", ".zh.srt", "-poster.jpg"} {
		if written["Mayday (2026)/Mayday (2026)"+suffix] == "" {
			t.Fatal(suffix, written)
		}
	}
	if len(written) != 3 {
		t.Fatal(written)
	}
}

func TestMoviePreviewDoesNotTreatCollectionAsOneMovie(t *testing.T) {
	a := testApp(t)
	mock := &mockList{files: []remoteFile{{Name: "Mayday.2026.mkv", Path: "/movies/Mayday.2026.mkv"}, {Name: "Arrival.2016.mkv", Path: "/movies/Arrival.2016.mkv"}}}
	server := httptest.NewServer(http.HandlerFunc(mock.handler))
	defer server.Close()
	_, e := a.preview(context.Background(), Object{"path": "/movies", "libraryType": "movie"}, Object{"baseUrl": server.URL}, DefaultSettings(), Object{"directoryPath": "/movies", "tmdbId": 123})
	if e == nil || !strings.Contains(e.Error(), "多部电影") {
		t.Fatal(e)
	}
}
