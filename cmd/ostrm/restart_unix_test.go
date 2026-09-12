//go:build linux || darwin

package main

import (
	"context"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Moersity/ostrm/internal/app"
)

func TestRestartPreservesResolvedPaths(t *testing.T) {
	dir := t.TempDir()
	self, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	src, err := os.Open(self)
	if err != nil {
		t.Fatal(err)
	}
	defer src.Close()
	exe := filepath.Join(dir, "ostrm-test")
	dst, err := os.OpenFile(exe, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
	if err != nil {
		t.Fatal(err)
	}
	_, err = io.Copy(dst, src)
	dst.Close()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestRestartProcessHelper$")
	cmd.Env = append(os.Environ(), "OSTRM_RESTART_TEST_DIR="+dir)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("restart failed: %v %s", err, output)
	}
	data, err := os.ReadFile(filepath.Join(dir, "result"))
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(strings.TrimSpace(string(data)), "\n")
	if len(args) != 7 || args[0] != "serve" || args[3] != "--data-dir" || args[4] != filepath.Join(dir, "data") || args[5] != "--strm-root" || args[6] != filepath.Join(dir, "output") {
		t.Fatalf("restart changed paths: %q", args)
	}
	a, err := app.New(app.Config{DataDir: filepath.Join(dir, "data"), StrmRoot: filepath.Join(dir, "output")})
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	value, err := a.Store.Setting("restart-sentinel", nil)
	if err != nil || value["value"] != "preserved" {
		t.Fatal(value, err)
	}
}

func TestRestartProcessHelper(t *testing.T) {
	dir := os.Getenv("OSTRM_RESTART_TEST_DIR")
	if dir == "" {
		return
	}
	a, err := app.New(app.Config{Listen: "127.0.0.1:0", DataDir: filepath.Join(dir, "data"), StrmRoot: filepath.Join(dir, "output")})
	if err != nil {
		t.Fatal(err)
	}
	if err = a.Store.Set("restart-sentinel", app.Object{"value": "preserved"}); err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("tcp", a.Config.Listen)
	if err != nil {
		t.Fatal(err)
	}
	p := &program{app: a, listener: listener, server: &http.Server{Handler: a.Handler()}, restart: make(chan struct{}, 1)}
	configureRestart(p)
	p.Start(nil)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	// Only the parent's temporary copy is replaced, never the test runner binary.
	if filepath.Dir(exe) != dir {
		t.Fatal("unexpected helper path")
	}
	candidate := exe + ".next"
	if err = os.WriteFile(candidate, []byte("#!/bin/sh\nprintf '%s\\n' \"$@\" > \"$OSTRM_RESTART_TEST_DIR/result\"\n"), 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.Rename(candidate, exe); err != nil {
		t.Fatal(err)
	}
	a.Restart()
	<-p.restart
	if err = restartProcess(p); err != nil {
		t.Fatal(err)
	}
	t.Fatal("exec returned without replacing the process")
}
