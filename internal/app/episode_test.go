package app

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestEpisodeNamesAndContext(t *testing.T) {
	for _, c := range []struct {
		path            string
		season, episode int
	}{
		{"/tv/Show/Season 02/03.mkv", 2, 3},
		{"/tv/collections/Show/Season 02/1080p/EP03.mkv", 2, 3},
		{"Show.2x03.1080p.mkv", 2, 3},
		{"Show.S02EP03.mkv", 2, 3},
		{"Show.Season.2.Episode.3.mkv", 2, 3},
		{"剧名第二季第三集.mkv", 2, 3},
		{"/tv/剧名/特别篇/第一百零二话.mkv", 0, 102},
		{"/anime/Show/Season 1/[Group] Show - 03 [1080p].mkv", 1, 3},
		{"1917.2019.1080p.mkv", 1, 0}, {"1080p.mkv", 1, 0}, {"03.mkv", 1, 0},
	} {
		s, e := mediaNumbers(c.path)
		if s != c.season || e != c.episode {
			t.Errorf("%s: got %d/%d want %d/%d", c.path, s, e, c.season, c.episode)
		}
	}
}
func TestTVIdentity(t *testing.T) {
	for _, c := range []struct{ path, title, year string }{
		{"/tv/合集/剧名 (2020)/Season 02/1080p/EP03.mkv", "剧名", "2020"},
		{"/tv/Show (2020)/Season 02/Show.S02E03.mkv", "Show", "2020"},
		{"/tv/Show.S02E03.1080p.WEB-DL.mkv", "Show", ""},
		{"/tv/剧名第二季第三集.mkv", "剧名", ""},
	} {
		info := identifyTV("/tv", c.path)
		if info.Title != c.title || info.Year != c.year {
			t.Errorf("%s: %+v", c.path, info)
		}
		if reason := structureReason(c.path, "tv"); reason != "" {
			t.Error(reason)
		}
	}
}
func TestTVMatchingUsesTitleAndYear(t *testing.T) {
	a := testApp(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/3/search/tv":
			if r.URL.Query().Get("query") != "Show" || r.URL.Query().Get("first_air_date_year") != "2020" {
				t.Error(r.URL.RawQuery)
			}
			json.NewEncoder(w).Encode(Object{"results": []Object{{"id": 1, "name": "Other", "first_air_date": "2020-01-01"}, {"id": 2, "name": "Show", "first_air_date": "2020-01-01"}}})
		case "/3/tv/2/season/2/episode/3":
			json.NewEncoder(w).Encode(Object{"name": "Episode Three"})
		case "/3/tv/2":
			json.NewEncoder(w).Encode(Object{"name": "Show", "first_air_date": "2020-01-01"})
		default:
			t.Error(r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	s := DefaultSettings()
	obj(s, "tmdb")["baseUrl"] = server.URL
	obj(s, "tmdb")["apiKey"] = "test"
	m, e := a.recognize(context.Background(), s, "/tv/Show (2020)/Season 02/Show.S02E03.mkv", "tv", "", "", 0)
	if e != nil || num(m, "tmdbId") != 2 {
		t.Fatal(m, e)
	}
	outputs := map[string]string{}
	task := Object{"path": "/tv", "strmPath": t.TempDir(), "libraryType": "tv"}
	file := remoteFile{Name: "EP03.mkv", Path: "/tv/collections/Show (2020)/Season 02/1080p/EP03.mkv"}
	err := a.processMetadata(context.Background(), task, Object{}, s, file, nil, "Show/Season 02/Show S02E03.strm", func(rel, source string, data []byte) error { outputs[rel] = string(data); return nil })
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, data := range outputs {
		if strings.Contains(data, "<episodedetails>") {
			found = true
			if !strings.Contains(data, "<season>2</season>") || !strings.Contains(data, "<episode>3</episode>") || !strings.Contains(data, "Episode Three") {
				t.Fatal(data)
			}
		}
	}
	if !found {
		t.Fatal(outputs)
	}

}
func TestBilingualFallbackAfterUnrelatedResult(t *testing.T) {
	a := testApp(t)
	searches := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/3/search/movie" {
			searches++
			name := "Unrelated"
			if r.URL.Query().Get("query") == "求救信号" {
				name = "Mayday"
			}
			json.NewEncoder(w).Encode(Object{"results": []Object{{"id": 2, "title": name, "release_date": "2026-01-01"}}})
		} else {
			json.NewEncoder(w).Encode(Object{"title": "Mayday", "release_date": "2026-01-01"})
		}
	}))
	defer server.Close()
	s := DefaultSettings()
	obj(s, "tmdb")["baseUrl"] = server.URL
	obj(s, "tmdb")["apiKey"] = "test"
	_, e := a.recognize(context.Background(), s, "", "movie", "求救信号 Mayday", "2026", 0)
	if e != nil || searches != 2 {
		t.Fatal(e, searches)
	}
	if _, e = selectMovieResult([]any{map[string]any{"id": float64(1), "title": "Unrelated"}}, "Mayday", ""); e == nil {
		t.Fatal("accepted unrelated result")
	}
}

func TestAIRecognitionCache(t *testing.T) {
	a := testApp(t)
	calls := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		json.NewEncoder(w).Encode(Object{"choices": []Object{{"message": Object{"content": `{"title":"Show","year":2020,"mediaType":"tv"}`}}}})
	}))
	defer server.Close()
	c := Object{"baseUrl": server.URL, "apiKey": "test", "model": "test"}
	for i := 0; i < 2; i++ {
		m, err := a.ai(context.Background(), c, "Show")
		if err != nil || str(m, "year") != "2020" || str(m, "title") != "Show" {
			t.Fatal(m, err)
		}
		m["title"] = "mutated"
	}
	if calls != 1 {
		t.Fatal("repeated AI calls", calls)
	}
	c["model"] = "different-model"
	if _, err := a.ai(context.Background(), c, "Show"); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("config change used stale cache", calls)
	}
}
