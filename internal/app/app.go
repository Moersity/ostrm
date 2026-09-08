package app

import (
	"context"
	"fmt"
	"github.com/gofrs/flock"
	"log/slog"
	"net"
	"net/http"
	"path/filepath"
	"sync"
	"time"
	_ "time/tzdata"
)

var Version = "3.0.0-dev"
var Commit = "unknown"

type cacheEntry struct {
	value Object
	until time.Time
}
type App struct {
	clients  map[string]*http.Client
	cache    map[string]cacheEntry
	Config   Config
	Store    *Store
	Client   *http.Client
	secret   []byte
	mu       sync.Mutex
	configMu sync.Mutex
	limits   map[string]*limiter
	active   map[int64]context.CancelFunc
	wg       sync.WaitGroup
	ctx      context.Context
	cancel   context.CancelFunc
	lock     *flock.Flock
	logFile  *rotatingLog
	logLevel slog.LevelVar
	Log      *slog.Logger
}

func New(c Config) (*App, error) {
	if e := c.Resolve(); e != nil {
		return nil, e
	}
	lock := flock.New(filepath.Join(c.DataDir, "ostrm.lock"))
	ok, e := lock.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, fmt.Errorf("数据目录正被另一 OStrm 实例使用")
	}
	s, e := OpenStore(c.DataDir)
	if e != nil {
		lock.Close()
		return nil, e
	}
	ctx, cancel := context.WithCancel(context.Background())
	a := &App{Config: c, Store: s, Client: &http.Client{Timeout: 30 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, DialContext: (&net.Dialer{Timeout: 10 * time.Second}).DialContext, MaxIdleConns: 32, IdleConnTimeout: 60 * time.Second, ResponseHeaderTimeout: 30 * time.Second}}, clients: map[string]*http.Client{}, cache: map[string]cacheEntry{}, limits: map[string]*limiter{}, active: map[int64]context.CancelFunc{}, ctx: ctx, cancel: cancel, lock: lock}
	f, e := openLog(filepath.Join(c.DataDir, "logs", "backend.log"))
	if e != nil {
		a.Close()
		return nil, e
	}
	a.logFile = f
	a.Log = slog.New(slog.NewJSONHandler(f, &slog.HandlerOptions{Level: &a.logLevel}))
	if settings, err := a.settings(); err == nil {
		a.logSettings(settings)
	}
	if e = a.initSecret(); e != nil {
		a.Close()
		return nil, e
	}
	for _, kind := range []string{"runs", "manual"} {
		rs, e := s.List(kind)
		if e != nil {
			a.Close()
			return nil, e
		}
		for _, r := range rs {
			switch str(r, "status") {
			case "RUNNING", "QUEUED", "PENDING":
				r["status"] = "INTERRUPTED"
				r["message"] = "程序重启，未完成的任务已中断；请检查后重新执行"
				if _, e = s.Save(kind, num(r, "id"), r); e != nil {
					a.Close()
					return nil, e
				}
			}
		}
	}
	if e = a.recoverOutputs(); e != nil {
		a.Close()
		return nil, e
	}
	a.wg.Add(2)
	go a.maintenance()
	go a.scheduleLoop()
	return a, nil
}
func (a *App) Close() error {
	a.cancel()
	a.mu.Lock()
	for _, cancel := range a.active {
		cancel()
	}
	a.mu.Unlock()
	a.wg.Wait()
	if a.logFile != nil {
		a.logFile.Close()
	}
	if a.Store != nil {
		a.Store.DB.Close()
	}
	if a.lock != nil {
		return a.lock.Close()
	}
	return nil
}
func (a *App) settings() (Object, error) { return a.Store.Setting("system", DefaultSettings()) }
func (a *App) taskConfig(id int64) (Object, Object, error) {
	t, e := a.Store.Get("tasks", id)
	if e != nil {
		return nil, nil, e
	}
	c, e := a.Store.Get("openlist", num(t, "openlistConfigId"))
	if e != nil {
		return nil, nil, e
	}
	if !boolean(c, "isActive", true) {
		return nil, nil, fmt.Errorf("OpenList 配置已禁用")
	}
	return t, c, nil
}
