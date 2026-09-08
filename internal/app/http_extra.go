package app

import (
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

func (a *App) registerManual(handle func(string, endpoint)) {
	for _, suffix := range []string{"tree", "tree/children"} {
		handle("GET /api/task-config/{id}/manual-scraping/"+suffix, func(r *http.Request) (any, error) {
			t, c, e := a.taskConfig(idParam(r, "id"))
			if e != nil {
				return nil, e
			}
			dir := str(t, "path")
			if suffix == "tree/children" {
				dir = r.URL.Query().Get("directoryPath")
			}
			node, e := a.directoryTree(r.Context(), t, c, dir)
			if suffix == "tree/children" {
				return node, e
			}
			return Object{"taskId": t["id"], "taskName": t["taskName"], "libraryType": t["libraryType"], "rootPath": str(t, "path"), "tree": node}, e
		})
	}
	handle("POST /api/task-config/{id}/manual-scraping/preview", func(r *http.Request) (any, error) {
		t, c, e := a.taskConfig(idParam(r, "id"))
		if e != nil {
			return nil, e
		}
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		s, e := a.settings()
		if e != nil {
			return nil, e
		}
		p, e := a.preview(r.Context(), t, c, s, m)
		delete(p, "metadata")
		return p, e
	})
	handle("POST /api/task-config/{id}/manual-scraping/execute", func(r *http.Request) (any, error) {
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		p, e := a.submitManual(idParam(r, "id"), m)
		return publicRun(p), e
	})
	handle("POST /api/task-config/{id}/manual-scraping/jobs/{jobId}/retry", func(r *http.Request) (any, error) { return a.retryManual(idParam(r, "id"), idParam(r, "jobId")) })
	for _, suffix := range []string{"structure-check", "structure-check/directories", "structure-check/directory"} {
		method := "POST"
		if suffix == "structure-check/directories" {
			method = "GET"
		}
		handle(method+" /api/task-config/{id}/"+suffix, func(r *http.Request) (any, error) {
			t, c, e := a.taskConfig(idParam(r, "id"))
			if e != nil {
				return nil, e
			}
			dir := str(t, "path")
			if suffix == "structure-check/directory" {
				m, e := readBody(r)
				if e != nil {
					return nil, e
				}
				dir = str(m, "directoryPath")
				if _, e = remoteRelative(str(t, "path"), dir); e != nil {
					return nil, e
				}
			}
			s, e := a.settings()
			if e != nil {
				return nil, e
			}
			overview := suffix == "structure-check/directories"
			var files []remoteFile
			if overview {
				files, e = a.list(r.Context(), c, dir)
			} else {
				files, e = a.scan(r.Context(), c, dir)
			}
			if e != nil {
				return nil, e
			}
			result := structureResult(t, dir, files, s)
			if !overview {
				return result, nil
			}
			ds := []Object{}
			for _, f := range files {
				if f.IsDir {
					ds = append(ds, Object{"name": f.Name, "path": f.Path})
				}
			}
			return Object{"taskId": t["id"], "taskName": t["taskName"], "libraryType": t["libraryType"], "rootPath": dir, "supported": result["supported"], "expectedStructure": result["expectedStructure"], "message": result["message"], "rootFilesResult": result, "directories": ds}, nil
		})
	}
}
func structureResult(t Object, dir string, files []remoteFile, s Object) Object {
	typ := str(t, "libraryType")
	children := []Object{}
	count := 0
	for _, f := range files {
		if f.IsDir || !video(f.Name, s) {
			continue
		}
		count++
		rel, _ := remoteRelative(str(t, "path"), f.Path)
		reason := structureReason(rel, typ)
		if reason != "" && typ != "auto" {
			children = append(children, Object{"name": f.Name, "path": f.Path, "type": "file", "reason": reason, "children": []Object{}})
		}
	}
	return Object{"taskId": t["id"], "taskName": t["taskName"], "libraryType": typ, "rootPath": dir, "expectedStructure": map[string]string{"movie": "电影目录/视频文件", "tv": "剧名/Season 01/视频文件", "anime": "动画名/视频文件 或 动画名/Season 01/视频文件"}[typ], "supported": typ != "auto", "scannedEntryCount": len(files), "videoFileCount": count, "invalidFileCount": len(children), "message": "检查完成", "tree": Object{"name": path.Base(dir), "path": dir, "type": "directory", "children": children}}
}
func (a *App) logPath(kind string) (string, error) {
	switch kind {
	case "backend", "error", "frontend":
		return filepath.Join(a.Config.DataDir, "logs", kind+".log"), nil
	}
	return "", errors.New("无效日志类型")
}
func (a *App) registerLogs(mux *http.ServeMux, handle func(string, endpoint)) {
	for _, suffix := range []string{"", "/tail", "/stats"} {
		handle("GET /api/logs/{logType}"+suffix, func(r *http.Request) (any, error) {
			p, e := a.logPath(r.PathValue("logType"))
			if e != nil {
				return nil, e
			}
			f, e := os.Open(p)
			if os.IsNotExist(e) {
				if suffix == "/tail" {
					return Object{"lines": []string{}, "cursor": 0, "fileKey": "", "reset": true, "hasMore": false}, nil
				}
				return []string{}, nil
			}
			if e != nil {
				return nil, e
			}
			defer f.Close()
			stat, e := f.Stat()
			if e != nil {
				return nil, e
			}
			if suffix == "/stats" {
				return Object{"size": stat.Size(), "lastModified": stat.ModTime(), "fileName": stat.Name()}, nil
			}
			cursor, _ := strconv.ParseInt(r.URL.Query().Get("cursor"), 10, 64)
			reset := cursor < 0 || cursor > stat.Size()
			if reset {
				cursor = 0
			}
			if suffix == "" || r.URL.Query().Get("cursor") == "" {
				cursor = max(0, stat.Size()-(256<<10))
			}
			b := make([]byte, min(int64(256<<10), stat.Size()-cursor))
			n, e := f.ReadAt(b, cursor)
			if e != nil && n == 0 && stat.Size() != 0 {
				return nil, e
			}
			b = b[:n]
			lines := []string{}
			for _, l := range strings.Split(strings.TrimSuffix(string(b), "\n"), "\n") {
				if l != "" {
					lines = append(lines, l)
				}
			}
			limit, _ := strconv.Atoi(r.URL.Query().Get("lines"))
			if limit <= 0 {
				limit = 500
			}
			if suffix == "" {
				if len(lines) > limit {
					lines = lines[len(lines)-limit:]
				}
				return lines, nil
			}
			return Object{"lines": lines, "cursor": cursor + int64(n), "fileKey": fmt.Sprint(stat.Name()), "reset": reset, "hasMore": cursor+int64(n) < stat.Size()}, nil
		})
	}
	handle("POST /api/logs/frontend", func(r *http.Request) (any, error) {
		r.Body = http.MaxBytesReader(nil, r.Body, 1<<20)
		m, e := readBody(r)
		if e != nil {
			return nil, e
		}
		p, _ := a.logPath("frontend")
		f, e := os.OpenFile(p, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			return nil, e
		}
		defer f.Close()
		_, e = fmt.Fprintln(f, redact(fmt.Sprint(m)))
		return "日志已接收", e
	})
	handle("DELETE /api/logs/{logType}", func(r *http.Request) (any, error) {
		p, e := a.logPath(r.PathValue("logType"))
		if e != nil {
			return nil, e
		}
		f, e := os.OpenFile(p, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
		if e != nil {
			return nil, e
		}
		return "日志已清理", f.Close()
	})
	mux.HandleFunc("GET /api/logs/{logType}/download", func(w http.ResponseWriter, r *http.Request) {
		p, e := a.logPath(r.PathValue("logType"))
		if e != nil {
			response(w, 400, 400, e.Error(), nil)
			return
		}
		w.Header().Set("Content-Disposition", "attachment; filename="+filepath.Base(p))
		http.ServeFile(w, r, p)
	})
}
func redact(s string) string {
	for _, key := range []string{"token", "password", "apiKey", "Authorization", "sign="} {
		if strings.Contains(s, key) {
			return "[敏感日志内容已隐藏]"
		}
	}
	return s
}

var _ = sql.ErrNoRows
