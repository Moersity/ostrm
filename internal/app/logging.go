package app

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Rotation bounds disk use even when external services repeatedly fail.
type rotatingLog struct {
	mu   sync.Mutex
	path string
	file *os.File
}

func openLog(path string) (*rotatingLog, error) {
	f, e := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if e != nil {
		return nil, e
	}
	return &rotatingLog{path: path, file: f}, nil
}
func (l *rotatingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if st, e := l.file.Stat(); e == nil && st.Size()+int64(len(p)) > 10<<20 {
		l.file.Close()
		for i := 4; i > 0; i-- {
			os.Rename(fmt.Sprintf("%s.%d", l.path, i), fmt.Sprintf("%s.%d", l.path, i+1))
		}
		if e = os.Rename(l.path, l.path+".1"); e != nil {
			return 0, e
		}
		f, e := os.OpenFile(l.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return 0, e
		}
		l.file = f
	}
	return l.file.Write(p)
}
func (l *rotatingLog) Close() error { l.mu.Lock(); defer l.mu.Unlock(); return l.file.Close() }
func (a *App) logSettings(s Object) {
	level := slog.LevelInfo
	switch strings.ToLower(str(obj(s, "log"), "level")) {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	}
	a.logLevel.Set(level)
}
func (a *App) expireLogs() {
	s, e := a.settings()
	if e != nil {
		return
	}
	days := num(obj(s, "log"), "retentionDays")
	if days < 1 {
		days = 7
	}
	dir := filepath.Join(a.Config.DataDir, "logs")
	files, _ := os.ReadDir(dir)
	for _, f := range files {
		if f.IsDir() || !strings.Contains(f.Name(), ".log.") {
			continue
		}
		st, e := f.Info()
		if e == nil && time.Since(st.ModTime()) > time.Duration(days)*24*time.Hour {
			os.Remove(filepath.Join(dir, f.Name()))
		}
	}
}

type logFanout struct{ backend, errors *rotatingLog }

func (l logFanout) Write(p []byte) (int, error) {
	n, e := l.backend.Write(p)
	if e != nil {
		return n, e
	}
	if bytes.Contains(p, []byte(`"level":"ERROR"`)) {
		_, e = l.errors.Write(p)
	}
	return n, e
}

// A context carries immutable correlation fields into concurrent directory scans.
type taskLogKey struct{}

func (a *App) taskLogger(t, r Object) *slog.Logger {
	return a.Log.With("taskId", num(t, "id"), "taskName", str(t, "taskName"), "runId", num(r, "id"))
}

func (a *App) contextLogger(ctx context.Context) *slog.Logger {
	if logger, ok := ctx.Value(taskLogKey{}).(*slog.Logger); ok {
		return logger
	}
	return a.Log
}
