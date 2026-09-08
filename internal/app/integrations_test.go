package app

import (
	"context"
	"encoding/json"
	"encoding/xml"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestMediaRefreshNotificationAndAI(t *testing.T) {
	a := testApp(t)
	calls := map[string]int{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls[r.URL.Path]++
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/System/Info":
			json.NewEncoder(w).Encode(Object{"ServerName": "mock", "Version": "1"})
		case "/Library/VirtualFolders":
			json.NewEncoder(w).Encode([]Object{{"ItemId": "lib", "Name": "Movies", "Locations": []string{"/media"}}})
		case "/Items/lib/Refresh":
			if r.Header.Get("X-Emby-Token") != "key" {
				t.Error("missing media auth")
			}
			w.WriteHeader(204)
		case "/notify/ostrm/":
			var m Object
			json.NewDecoder(r.Body).Decode(&m)
			if str(m, "type") != "warning" || !strings.Contains(str(m, "body"), "partial reason") {
				t.Error(m)
			}
			w.WriteHeader(200)
		case "/chat/completions":
			json.NewEncoder(w).Encode(Object{"choices": []Object{{"message": Object{"content": "{\"title\":\"Movie\",\"year\":\"2020\",\"mediaType\":\"movie\"}"}}}})
		default:
			t.Error("unexpected endpoint", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer srv.Close()
	c := Object{"apiBaseUrl": srv.URL, "apiKey": "key"}
	if m, e := a.mediaTest(context.Background(), c); e != nil || str(m, "serverName") != "mock" {
		t.Fatal(m, e)
	}
	if _, e := a.refresh(context.Background(), c, Object{"scope": "LIBRARY", "libraryId": "lib"}); e != nil {
		t.Fatal(e)
	}
	n := Object{"enabled": true, "serverUrl": srv.URL, "configKey": "ostrm", "includeFullPath": false}
	if e := a.notify(context.Background(), n, Object{"taskName": "test"}, Object{"status": "PARTIAL_SUCCESS", "issues": []any{Object{"reason": "partial reason"}}}); e != nil {
		t.Fatal(e)
	}
	n["notifyOnSuccess"] = false
	if e := a.notify(context.Background(), n, nil, Object{"status": "SUCCESS"}); e != nil {
		t.Fatal(e)
	}
	if calls["/notify/ostrm/"] != 1 {
		t.Fatal("notification filter ignored")
	}
	m, e := a.ai(context.Background(), Object{"baseUrl": srv.URL, "apiKey": "test", "model": "test"}, "Movie.mkv")
	if e != nil || str(m, "title") != "Movie" {
		t.Fatal(m, e)
	}
}
func TestNFOEscapesAndPreservesMetadata(t *testing.T) {
	m := Object{"title": "A & B <special>", "tmdbId": int64(42), "vote_average": 8.5, "vote_count": int64(100), "runtime": int64(120), "genres": []any{Object{"name": "Drama"}}, "imdb_id": "tt123"}
	b, e := nfoBytes(m, "movie", 0, 0)
	if e != nil {
		t.Fatal(e)
	}
	var got nfo
	if e = xml.Unmarshal(b, &got); e != nil {
		t.Fatal(e)
	}
	if got.Title != str(m, "title") || got.Runtime != 120 || got.Rating.Votes != 100 || got.Genres[0] != "Drama" || got.IMDB != "tt123" {
		t.Fatal(string(b))
	}
}
