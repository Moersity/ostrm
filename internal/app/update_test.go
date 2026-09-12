package app

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

type updateTransport func(*http.Request) (*http.Response, error)

func (f updateTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
func archiveForUpdate(t *testing.T, name, content string, typ byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tr := tar.NewWriter(gz)
	size := int64(len(content))
	if typ != tar.TypeReg {
		size = 0
	}
	if err := tr.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: size, Typeflag: typ, Linkname: "../../outside"}); err != nil {
		t.Fatal(err)
	}
	if size > 0 {
		tr.Write([]byte(content))
	}
	tr.Close()
	gz.Close()
	return buf.Bytes()
}
func TestUpdatePreservesDataAndRejectsCorruption(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix executable replacement")
	}
	for _, corrupt := range []bool{false, true} {
		t.Run(fmt.Sprint(corrupt), func(t *testing.T) {
			a := testApp(t)
			if err := a.Store.Set("sentinel", Object{"value": "keep me"}); err != nil {
				t.Fatal(err)
			}
			strm := filepath.Join(a.Config.StrmRoot, "keep.strm")
			os.WriteFile(strm, []byte("https://example.com/media"), 0600)
			config := filepath.Join(a.Config.DataDir, "config.yaml")
			os.WriteFile(config, []byte("keep config"), 0600)
			exe := filepath.Join(t.TempDir(), "ostrm")
			os.WriteFile(exe, []byte("old executable"), 0755)
			version := "99.0.0"
			name := updateAssetName(version)
			candidate := fmt.Sprintf("#!/bin/sh\nprintf '%%s\\n' '{\"version\":\"%s\",\"os\":\"%s\",\"arch\":\"%s\"}'\n", version, runtime.GOOS, runtime.GOARCH)
			archive := archiveForUpdate(t, strings.TrimSuffix(name, ".tar.gz")+"/ostrm", candidate, tar.TypeReg)
			checksum := fmt.Sprintf("%x  %s\n", sha256.Sum256(archive), name)
			if corrupt {
				checksum = strings.Repeat("0", 64) + "  " + name
			}
			r := releaseInfo{Tag: "v" + version}
			for _, n := range []string{name, "checksums.txt"} {
				r.Assets = append(r.Assets, releaseAsset{Name: n, URL: repositoryURL + "/releases/download/" + r.Tag + "/" + n})
			}
			metadata, _ := json.Marshal(r)
			a.Client.Transport = updateTransport(func(req *http.Request) (*http.Response, error) {
				var data []byte
				switch req.URL.String() {
				case releaseAPI:
					data = metadata
				case r.Assets[0].URL:
					data = archive
				case r.Assets[1].URL:
					data = []byte(checksum)
				default:
					t.Fatalf("unexpected URL %s", req.URL)
				}
				return &http.Response{StatusCode: 200, Body: io.NopCloser(bytes.NewReader(data)), Header: make(http.Header)}, nil
			})
			err := a.installUpdateAt(context.Background(), version, exe)
			if corrupt && err == nil {
				t.Fatal("corrupt archive accepted")
			}
			if !corrupt && err != nil {
				t.Fatal(err)
			}
			got, _ := os.ReadFile(exe)
			want := candidate
			if corrupt {
				want = "old executable"
			}
			if string(got) != want {
				t.Fatal("unexpected executable")
			}
			for p, w := range map[string]string{strm: "https://example.com/media", config: "keep config"} {
				b, _ := os.ReadFile(p)
				if string(b) != w {
					t.Fatal("data modified", p)
				}
			}
			value, err := a.Store.Setting("sentinel", nil)
			if err != nil || str(value, "value") != "keep me" {
				t.Fatal(value, err)
			}
			if !corrupt {
				state := a.updateStatus()
				old, _ := os.ReadFile(str(state, "binaryBackupPath"))
				if string(old) != "old executable" {
					t.Fatal("missing old binary")
				}
				db, err := sql.Open("sqlite", filepath.Join(str(state, "backupPath"), "ostrm.db"))
				if err != nil {
					t.Fatal(err)
				}
				defer db.Close()
				var check string
				if err = db.QueryRow("PRAGMA integrity_check").Scan(&check); err != nil || check != "ok" {
					t.Fatal("bad backup", check, err)
				}
			}
		})
	}
}
func TestUpdateArchiveRejectsLinksAndTraversal(t *testing.T) {
	for _, entry := range []struct {
		name string
		typ  byte
	}{{"../../outside", tar.TypeReg}, {strings.TrimSuffix(updateAssetName("9.0.0"), ".tar.gz") + "/ostrm", tar.TypeSymlink}} {
		dir := t.TempDir()
		archive := filepath.Join(dir, "release.tar.gz")
		os.WriteFile(archive, archiveForUpdate(t, entry.name, "evil", entry.typ), 0600)
		if err := extractUpdate(archive, filepath.Join(dir, "candidate"), "9.0.0"); err == nil {
			t.Fatal("unsafe archive accepted")
		}
	}
}
func TestUpdateAndLogsRequireAuthentication(t *testing.T) {
	a := testApp(t)
	h := a.Handler()
	for _, route := range []struct{ method, path string }{{"GET", "/api/version/check"}, {"POST", "/api/version/upgrade"}, {"GET", "/api/logs/backend/tail"}, {"GET", "/api/logs/backend/download"}, {"DELETE", "/api/logs/backend"}} {
		code, _ := call(t, h, route.method, route.path, "", nil)
		if code != 401 {
			t.Fatal(route, code)
		}
	}
	call(t, h, "POST", "/api/auth/sign-up", "", Object{"username": "admin", "password": "secret123"})
	_, login := call(t, h, "POST", "/api/auth/sign-in", "", Object{"username": "admin", "password": "secret123"})
	token := str(obj(login, "data"), "token")
	a.Log.Info("log authentication test")
	status, result := call(t, h, "GET", "/api/logs/backend/tail", token, nil)
	if status != 200 || num(result, "code") != 200 {
		t.Fatal(result)
	}
	a.mu.Lock()
	a.updating = true
	a.mu.Unlock()
	status, _ = call(t, h, "DELETE", "/api/logs/backend", token, nil)
	if status != 503 {
		t.Fatal("mutations allowed during update", status)
	}
}
func TestReleaseAssetValidation(t *testing.T) {
	for _, target := range []string{"http://github.com/Moersity/ostrm/releases/download/v9.0.0/checksums.txt", "https://github.com/other/repo/releases/download/v9.0.0/checksums.txt", "https://example.com/checksums.txt"} {
		r := releaseInfo{Tag: "v9.0.0", Assets: []releaseAsset{{Name: "checksums.txt", URL: target}}}
		if _, err := r.asset("checksums.txt"); err == nil {
			t.Fatal(target)
		}
	}
	if _, err := expectedChecksum(strings.Repeat("a", 64)+"  a\n"+strings.Repeat("a", 64)+"  a", "a"); err == nil {
		t.Fatal("duplicate checksum")
	}
}

func TestReleaseCacheAndStableVersions(t *testing.T) {
	a := testApp(t)
	calls := 0
	a.Client.Transport = updateTransport(func(req *http.Request) (*http.Response, error) {
		calls++
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(`{"tag_name":"v99.0.0","draft":false,"prerelease":false}`)), Header: make(http.Header)}, nil
	})
	for i := 0; i < 2; i++ {
		if _, err := a.latestRelease(context.Background(), false); err != nil {
			t.Fatal(err)
		}
	}
	if calls != 1 {
		t.Fatal("release check was not cached", calls)
	}
	if _, err := a.latestRelease(context.Background(), true); err != nil {
		t.Fatal(err)
	}
	if calls != 2 {
		t.Fatal("upgrade did not refresh release metadata")
	}
	for _, body := range []string{`{"tag_name":"v99.0.0","draft":true}`, `{"tag_name":"v99.0.0","prerelease":true}`, `{"tag_name":"v99.0.0-beta"}`, `{"tag_name":"../../evil"}`} {
		a.Client.Transport = updateTransport(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(body)), Header: make(http.Header)}, nil
		})
		if _, err := a.latestRelease(context.Background(), true); err == nil {
			t.Fatal("invalid release accepted", body)
		}
	}
}
