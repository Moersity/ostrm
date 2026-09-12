package app

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Moersity/ostrm/internal/web"
	"github.com/golang-jwt/jwt/v5"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type claimsKey struct{}
type endpoint func(*http.Request) (any, error)

func readBody(r *http.Request) (Object, error) {
	if r.Body == nil {
		return Object{}, nil
	}
	var m Object
	d := json.NewDecoder(io.LimitReader(r.Body, 8<<20+1))
	e := d.Decode(&m)
	if errors.Is(e, io.EOF) {
		return Object{}, nil
	}
	if e != nil {
		return nil, errors.New("请求 JSON 无效")
	}
	var more any
	if e = d.Decode(&more); e != io.EOF {
		return nil, errors.New("请求包含多余数据")
	}
	if m == nil {
		m = Object{}
	}
	return m, nil
}
func idParam(r *http.Request, key string) int64 {
	n, _ := strconv.ParseInt(r.PathValue(key), 10, 64)
	return n
}
func response(w http.ResponseWriter, status, code int, message string, data any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(Object{"code": code, "message": message, "data": data})
}
func (a *App) Handler() http.Handler {
	mux := http.NewServeMux()
	handle := func(pattern string, fn endpoint) {
		mux.HandleFunc(pattern, func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if v := recover(); v != nil {
					a.Log.Error("request panic", "path", r.URL.Path)
					response(w, 500, 500, "内部错误", nil)
				}
			}()
			r.Body = http.MaxBytesReader(w, r.Body, 8<<20)
			data, e := fn(r)
			if e != nil {
				code := 400
				if errors.Is(e, sql.ErrNoRows) {
					code = 404
				}
				response(w, 200, code, e.Error(), nil)
				return
			}
			status := 200
			if pattern == "POST /api/auth/sign-up" {
				status = 201
			}
			response(w, status, 200, "success", data)
		})
	}
	mux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		response(w, 200, 200, "healthy", Object{"version": Version})
	})
	mux.HandleFunc("GET /api/auth/check-user", func(w http.ResponseWriter, r *http.Request) {
		exists, e := a.hasUser()
		if e != nil {
			response(w, 500, 500, "数据库错误", nil)
			return
		}
		code := 200
		if !exists {
			code = 404
		}
		response(w, 200, code, "success", Object{"exists": exists})
	})
	handle("POST /api/auth/sign-up", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		return nil, a.register(m)
	})
	handle("POST /api/auth/sign-in", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		if e = a.limit(Object{"id": "login", "baseUrl": host}).wait(r.Context(), 1, 10); e != nil {
			return nil, e
		}
		if e = a.password(str(m, "username"), str(m, "password")); e != nil {
			return nil, e
		}
		return a.token(str(m, "username"))
	})
	handle("GET /api/auth/validate", func(r *http.Request) (any, error) {
		c := r.Context().Value(claimsKey{}).(*jwt.RegisteredClaims)
		return Object{"valid": true, "username": c.Subject, "expiresAt": c.ExpiresAt.Time.UTC().Format(time.RFC3339Nano), "issuedAt": c.IssuedAt.Time.UTC().Format(time.RFC3339Nano)}, nil
	})
	handle("POST /api/auth/refresh", func(r *http.Request) (any, error) {
		c := r.Context().Value(claimsKey{}).(*jwt.RegisteredClaims)
		m, e := a.token(c.Subject)
		if e == nil {
			_, e = a.Store.DB.Exec("DELETE FROM sessions WHERE id=?", c.ID)
		}
		return m, e
	})
	handle("POST /api/auth/sign-out", func(r *http.Request) (any, error) {
		c := r.Context().Value(claimsKey{}).(*jwt.RegisteredClaims)
		_, e := a.Store.DB.Exec("DELETE FROM sessions WHERE id=?", c.ID)
		return nil, e
	})
	handle("POST /api/auth/change-password", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		return nil, a.changePassword(r.Context().Value(claimsKey{}).(*jwt.RegisteredClaims).Subject, m)
	})
	for prefix, kind := range map[string]string{"/api/openlist-config": "openlist", "/api/task-config": "tasks", "/api/media-servers": "media"} {
		handle("GET "+prefix, func(r *http.Request) (any, error) {
			rs, e := a.Store.List(kind)
			if kind == "media" {
				for i, m := range rs {
					rs[i] = mediaView(m)
				}
			}
			return rs, e
		})
		handle("POST "+prefix, func(r *http.Request) (any, error) { return a.saveConfig(r, kind, 0) })
		handle("PUT "+prefix+"/{id}", func(r *http.Request) (any, error) { return a.saveConfig(r, kind, idParam(r, "id")) })
		handle("DELETE "+prefix+"/{id}", func(r *http.Request) (any, error) {
			a.configMu.Lock()
			defer a.configMu.Unlock()
			id := idParam(r, "id")
			a.mu.Lock()
			active := a.active[id] != nil
			a.mu.Unlock()
			if kind == "tasks" && active {
				return nil, errors.New("请先停止任务")
			}
			if kind != "tasks" {
				ts, e := a.Store.List("tasks")
				if e != nil {
					return nil, e
				}
				field := "openlistConfigId"
				if kind == "media" {
					field = "mediaServerConfigId"
				}
				for _, t := range ts {
					if num(t, field) == id {
						return nil, errors.New("配置仍被任务引用")
					}
				}
			}
			return nil, a.Store.Delete(kind, id)
		})
		if kind == "media" {
			continue
		}
		handle("GET "+prefix+"/{id}", func(r *http.Request) (any, error) { return a.Store.Get(kind, idParam(r, "id")) })
		handle("GET "+prefix+"/active", func(r *http.Request) (any, error) {
			all, e := a.Store.List(kind)
			out := []Object{}
			for _, m := range all {
				if boolean(m, "isActive", true) {
					out = append(out, m)
				}
			}
			return out, e
		})
		handle("PATCH "+prefix+"/{id}/status", func(r *http.Request) (any, error) { return a.saveConfig(r, kind, idParam(r, "id")) })
	}
	handle("GET /api/openlist-config/username/{username}", func(r *http.Request) (any, error) {
		return a.findConfig("openlist", "username", r.PathValue("username"))
	})
	handle("POST /api/openlist-config/validate", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		dir := str(m, "basePath")
		if dir == "" {
			dir = "/"
		}
		_, e = a.list(r.Context(), m, dir)
		return Object{"valid": e == nil, "message": "连接成功"}, e
	})
	handle("POST /api/openlist-config/validate-path", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		c, e := a.Store.Get("openlist", num(m, "openlistConfigId"))
		if e != nil {
			return nil, e
		}
		_, e = a.list(r.Context(), c, str(m, "taskPath"))
		return nil, e
	})
	handle("GET /api/task-config/task-name/{taskName}", func(r *http.Request) (any, error) { return a.findConfig("tasks", "taskName", r.PathValue("taskName")) })
	handle("GET /api/task-config/path", func(r *http.Request) (any, error) { return a.findConfig("tasks", "path", r.URL.Query().Get("path")) })
	handle("GET /api/task-config/scheduled", func(r *http.Request) (any, error) {
		all, e := a.Store.List("tasks")
		out := []Object{}
		for _, t := range all {
			if str(t, "cron") != "" {
				out = append(out, t)
			}
		}
		return out, e
	})
	handle("PATCH /api/task-config/{id}/last-exec-time", func(r *http.Request) (any, error) { return a.saveConfig(r, "tasks", idParam(r, "id")) })
	handle("POST /api/task-config/{id}/submit", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		var inc *bool
		if b, ok := m["isIncremental"].(bool); ok {
			inc = &b
		}
		_, e = a.submit(idParam(r, "id"), inc)
		return "任务已提交执行", e
	})
	for suffix, kind := range map[string]string{"runs": "runs", "manual-scraping/jobs": "manual"} {
		handle("GET /api/task-config/{id}/"+suffix+"/latest", func(r *http.Request) (any, error) {
			rs, e := a.Store.List(kind)
			for i := len(rs) - 1; i >= 0; i-- {
				if num(rs[i], "taskId") == idParam(r, "id") {
					return publicRun(rs[i]), e
				}
			}
			return nil, e
		})
		handle("GET /api/task-config/{id}/"+suffix+"/{jobId}", func(r *http.Request) (any, error) {
			m, e := a.Store.Get(kind, idParam(r, "jobId"))
			if e == nil && num(m, "taskId") != idParam(r, "id") {
				return nil, sql.ErrNoRows
			}
			return publicRun(m), e
		})
	}
	handle("POST /api/task-config/{id}/cancel", func(r *http.Request) (any, error) {
		a.mu.Lock()
		cancel := a.active[idParam(r, "id")]
		a.mu.Unlock()
		if cancel != nil {
			cancel()
		}
		return "取消请求已提交", nil
	})
	handle("GET /api/system/config", func(r *http.Request) (any, error) {
		s, e := a.settings()
		s["paths"] = Object{"strmRoot": a.Config.StrmRoot, "dataDir": a.Config.DataDir}
		return s, e
	})
	handle("POST /api/system/config", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		a.configMu.Lock()
		defer a.configMu.Unlock()
		old, e := a.settings()
		if e != nil {
			return nil, e
		}
		s := merge(old, m)
		obj(s, "log")["reportUsageData"] = false
		delete(s, "paths")
		if tz := str(s, "timezone"); tz != "" {
			if _, e = time.LoadLocation(tz); e != nil {
				return nil, e
			}
		}
		if e = a.Store.Set("system", s); e != nil {
			return nil, e
		}
		a.logSettings(s)
		return "配置已保存", nil
	})
	handle("POST /api/system/test-ai-config", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		if nested := obj(m, "ai"); len(nested) > 0 {
			m = nested
		}
		_, e = a.ai(r.Context(), m, "The Matrix (1999)")
		return "AI 配置连接成功", e
	})
	handle("POST /api/system/test-notification", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		if nested := obj(m, "notifications"); len(nested) > 0 {
			m = nested
		}
		m["enabled"] = true
		return "通知已发送", a.notify(r.Context(), m, Object{"taskName": "测试通知"}, Object{"status": "SUCCESS"})
	})
	for _, p := range []string{"event", "events"} {
		handle("POST /api/data-report/"+p, func(r *http.Request) (any, error) {
			return Object{"accepted": false, "reason": "telemetry disabled"}, nil
		})
	}
	handle("GET /api/version/check", func(r *http.Request) (any, error) {
		return a.checkVersion(r.Context(), r.URL.Query().Get("includePrerelease") == "true")
	})
	handle("GET /api/version/latest", func(r *http.Request) (any, error) { return a.checkVersion(r.Context(), false) })
	handle("DELETE /api/version/cache/clear", func(r *http.Request) (any, error) {
		a.versionMu.Lock()
		a.releaseUntil = time.Time{}
		a.versionMu.Unlock()
		return "版本缓存已清除", nil
	})
	handle("POST /api/version/upgrade", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		return a.startUpdate(str(m, "version"))
	})
	handle("GET /api/version/upgrade", func(r *http.Request) (any, error) { return a.updateStatus(), nil })
	a.registerMedia(handle)
	a.registerTrash(handle)
	a.registerManual(handle)
	a.registerLogs(mux, handle)
	mux.HandleFunc("/api/", func(w http.ResponseWriter, r *http.Request) { response(w, 404, 404, "接口不存在", nil) })
	mux.Handle("/", web.Handler())
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		p := r.URL.Path
		public := p == "/api/auth/sign-in" || p == "/api/auth/sign-up" || p == "/api/auth/check-user"
		if strings.HasPrefix(p, "/api/") && !public {
			token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
			c, e := a.verify(token)
			if e != nil {
				response(w, 401, 401, "未授权访问", nil)
				return
			}
			r = r.WithContext(context.WithValue(r.Context(), claimsKey{}, c))
		}
		a.mu.Lock()
		updating := a.updating
		a.mu.Unlock()
		if updating && strings.HasPrefix(p, "/api/") && r.Method != "GET" && r.Method != "HEAD" && p != "/api/auth/refresh" && p != "/api/auth/sign-in" {
			response(w, 503, 503, "正在安全升级，请稍后操作", nil)
			return
		}
		mux.ServeHTTP(w, r)
	})
}
func publicRun(m Object) Object { m = clone(m); delete(m, "plan"); delete(m, "steps"); return m }
func mediaView(m Object) Object {
	v := clone(m)
	v["apiKeyConfigured"] = str(m, "apiKey") != ""
	v["active"] = boolean(m, "isActive", true)
	delete(v, "apiKey")
	return v
}
func (a *App) findConfig(kind, key, value string) (Object, error) {
	rs, e := a.Store.List(kind)
	if e != nil {
		return nil, e
	}
	for _, m := range rs {
		if str(m, key) == value {
			return m, nil
		}
	}
	return nil, sql.ErrNoRows
}
func (a *App) saveConfig(r *http.Request, kind string, id int64) (any, error) {
	m, e := readBody(r)
	if e != nil {
		return nil, e
	}
	a.configMu.Lock()
	defer a.configMu.Unlock()
	if id > 0 {
		old, e := a.Store.Get(kind, id)
		if e != nil {
			return nil, e
		}
		if kind == "media" && str(m, "apiKey") == "" {
			delete(m, "apiKey")
		}
		m = merge(old, m)
	} else {
		m = merge(Object{"isActive": true}, m)
	}
	switch kind {
	case "openlist":
		if e = require(m, "baseUrl", "token", "username"); e != nil {
			return nil, e
		}
		if e = validURL(str(m, "baseUrl")); e != nil {
			return nil, e
		}
		if str(m, "strmBaseUrl") != "" {
			if e = validURL(str(m, "strmBaseUrl")); e != nil {
				return nil, e
			}
		}
		if num(m, "fsApiQpsLimit") < 0 || num(m, "fsApiQpsLimit") > 1000 || num(m, "fsApiQpmLimit") < 0 || num(m, "fsApiQpmLimit") > 6000 {
			return nil, errors.New("限流参数超出范围")
		}
	case "tasks":
		if e = require(m, "taskName", "path"); e != nil {
			return nil, e
		}
		if !strings.HasPrefix(str(m, "path"), "/") {
			return nil, errors.New("源路径必须以 / 开头")
		}
		if _, e = a.Store.Get("openlist", num(m, "openlistConfigId")); e != nil {
			return nil, e
		}
		m = merge(Object{"libraryType": "auto", "movieVersions": "best", "movieNaming": "smart", "skipMovieExtras": true, "isIncrement": true, "needScrap": false, "mediaRefreshScope": "NONE"}, m)
		if str(m, "cron") != "" {
			settings, e := a.settings()
			if e != nil {
				return nil, e
			}
			if _, e = cronTrigger(str(m, "cron"), str(settings, "timezone")); e != nil {
				return nil, e
			}
		}
		if mode := str(m, "movieVersions"); mode != "best" && mode != "all" {
			return nil, errors.New("电影版本策略必须为 best 或 all")
		}
		if mode := str(m, "movieNaming"); mode != "smart" && mode != "original" {
			return nil, errors.New("电影 STRM 命名方式必须为 smart 或 original")
		}
		if _, e = renameFile("test.mkv", str(m, "renameRegex")); e != nil {
			return nil, e
		}
		root := str(m, "strmPath")
		if root == "" {
			root = filepath.Join(a.Config.StrmRoot, safeName(str(m, "taskName")))
		}
		if !filepath.IsAbs(root) {
			if !filepath.IsLocal(root) {
				return nil, errors.New("相对输出路径不能越界")
			}
			root = filepath.Join(a.Config.StrmRoot, root)
		}
		root, e = filepath.Abs(root)
		if e != nil {
			return nil, e
		}
		root = canonicalRoot(root)
		for index, protected := range []string{a.Config.DataDir, filepath.Join(a.Config.DataDir, "logs"), filepath.Join(a.Config.DataDir, "trash")} {
			// The default STRM subtree is inside data-dir; protect its siblings and the data root itself.
			protected = canonicalRoot(protected)
			if index == 0 {
				if pathsOverlap(root, protected) && !strings.HasPrefix(strings.ToLower(root), strings.ToLower(protected)+string(os.PathSeparator)) {
					return nil, errors.New("输出目录不能覆盖数据目录")
				}
			} else if pathsOverlap(root, protected) {
				return nil, errors.New("输出目录与内部数据冲突")
			}
		}
		m["strmPath"] = root
		all, e := a.Store.List("tasks")
		if e != nil {
			return nil, e
		}
		for _, t := range all {
			if num(t, "id") == id {
				continue
			}
			if strings.EqualFold(str(t, "taskName"), str(m, "taskName")) {
				return nil, errors.New("任务名称已存在")
			}
			if pathsOverlap(root, str(t, "strmPath")) {
				return nil, errors.New("任务输出目录重叠")
			}
		}
		a.mu.Lock()
		active := a.active[id] != nil
		a.mu.Unlock()
		if active && r.Method != "PATCH" {
			return nil, errors.New("请等待运行结束再修改任务")
		}
	case "media":
		if e = require(m, "name", "serverType", "apiBaseUrl", "apiKey"); e != nil {
			return nil, e
		}
		if e = validURL(str(m, "apiBaseUrl")); e != nil {
			return nil, e
		}
		m["serverType"] = strings.ToUpper(str(m, "serverType"))
		if str(m, "serverType") != "EMBY" && str(m, "serverType") != "JELLYFIN" {
			return nil, errors.New("无效服务器类型")
		}
	}
	saved, e := a.Store.Save(kind, id, m)
	if kind == "media" {
		saved = mediaView(saved)
	}
	return saved, e
}
func (a *App) registerMedia(handle func(string, endpoint)) {
	handle("POST /api/media-servers/test", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		return a.mediaTest(r.Context(), m)
	})
	for _, action := range []string{"test", "libraries", "refresh"} {
		method := "POST"
		if action == "libraries" {
			method = "GET"
		}
		handle(method+" /api/media-servers/{id}/"+action, func(r *http.Request) (any, error) {
			m, e := a.Store.Get("media", idParam(r, "id"))
			if e != nil {
				return nil, e
			}
			switch action {
			case "test":
				return a.mediaTest(r.Context(), m)
			case "libraries":
				return a.libraries(r.Context(), m)
			default:
				req, e := readBody(r)
				if e != nil {
					return nil, e
				}
				return a.refresh(r.Context(), m, req)
			}
		})
	}
}

var _ = fmt.Sprint
