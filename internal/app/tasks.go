package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/reugn/go-quartz/quartz"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

func cronTrigger(expr, tz string) (*quartz.CronTrigger, error) {
	p := strings.Fields(expr)
	if len(p) == 5 {
		if p[4] != "*" {
			p[2] = "?"
		} else {
			p[4] = "?"
		}
		expr = "0 " + strings.Join(p, " ")
	}
	loc := time.Local
	var e error
	if tz != "" && tz != "Local" {
		loc, e = time.LoadLocation(tz)
		if e != nil {
			return nil, e
		}
	}
	return quartz.NewCronTriggerWithLoc(expr, loc)
}
func (a *App) scheduleLoop() {
	defer a.wg.Done()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	next := map[int64]int64{}
	finger := map[int64]string{}
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			ts, e := a.Store.List("tasks")
			if e != nil {
				continue
			}
			s, e := a.settings()
			if e != nil {
				continue
			}
			for _, t := range ts {
				id := num(t, "id")
				expr := str(t, "cron")
				if expr == "" || !boolean(t, "isActive", true) {
					delete(next, id)
					continue
				}
				key := expr + str(s, "timezone")
				tr, e := cronTrigger(expr, str(s, "timezone"))
				if e != nil {
					continue
				}
				n := time.Now().UnixNano()
				if finger[id] != key || next[id] == 0 {
					finger[id] = key
					next[id], _ = tr.NextFireTime(n)
					continue
				}
				if n >= next[id] {
					_, e = a.submit(id, nil)
					if e != nil {
						a.Log.Warn("scheduled task rejected", "taskId", id, "error", e.Error())
					}
					next[id], _ = tr.NextFireTime(n)
				}
			}
		}
	}
}
func (a *App) submit(id int64, increment *bool) (Object, error) {
	t, c, e := a.taskConfig(id)
	if e != nil {
		return nil, e
	}
	s, e := a.settings()
	if e != nil {
		return nil, e
	}
	if increment != nil {
		t["isIncrement"] = *increment
	}
	a.mu.Lock()
	if _, ok := a.active[id]; ok {
		a.mu.Unlock()
		return nil, errors.New("任务正在执行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.active[id] = cancel
	a.mu.Unlock()
	r, e := a.Store.Save("runs", 0, Object{"taskId": id, "status": "QUEUED", "progress": 0, "processed": 0, "failed": 0, "stage": "DISCOVERY", "isIncremental": boolean(t, "isIncrement", true), "issues": []any{}})
	if e != nil {
		a.mu.Lock()
		delete(a.active, id)
		a.mu.Unlock()
		cancel()
		return nil, e
	}
	result := clone(r)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() { cancel(); a.mu.Lock(); delete(a.active, id); a.mu.Unlock() }()
		a.execute(ctx, t, c, s, r)
	}()
	return result, nil
}
func (a *App) updateRun(r Object) {
	if _, e := a.Store.Save("runs", num(r, "id"), r); e != nil {
		a.Log.Error("persist run", "error", e.Error())
	}
}
func (a *App) execute(ctx context.Context, t, c, s, r Object) {
	r["status"] = "RUNNING"
	r["startedAt"] = now()
	a.updateRun(r)
	err := a.executeFiles(ctx, t, c, s, r)
	if err != nil {
		r["status"] = "FAILED"
		r["errorMessage"] = err.Error()
	}
	if ctx.Err() != nil {
		r["status"] = "CANCELED"
	}
	if str(r, "status") == "RUNNING" {
		r["status"] = "SUCCESS"
		if num(r, "failed") > 0 {
			r["status"] = "PARTIAL_SUCCESS"
		}
	}
	r["completedAt"] = now()
	r["progress"] = 100
	r["stage"] = "FINALIZE"
	a.updateRun(r)
	a.configMu.Lock()
	fresh, e := a.Store.Get("tasks", num(t, "id"))
	if e == nil {
		fresh["lastExecTime"] = time.Now().UnixMilli()
		_, e = a.Store.Save("tasks", num(t, "id"), fresh)
	}
	a.configMu.Unlock()
	if e != nil {
		a.Log.Error("persist last execution", "error", e.Error())
	}
	if ctx.Err() == nil {
		if str(r, "status") == "SUCCESS" || str(r, "status") == "PARTIAL_SUCCESS" {
			r["mediaRefresh"], e = a.refreshAfter(ctx, t, num(r, "changed") > 0, boolean(t, "isIncrement", true))
			if e != nil {
				r["mediaRefreshError"] = e.Error()
			}
		}
		if e = a.notify(ctx, obj(s, "notifications"), t, r); e != nil {
			r["notificationError"] = e.Error()
		}
		a.updateRun(r)
	}
}
func (a *App) executeFiles(ctx context.Context, t, c, s, r Object) error {
	files, e := a.scan(ctx, c, str(t, "path"))
	if e != nil {
		return e
	}
	if boolean(t, "autoRenameMedia", false) {
		if e = a.autoRename(ctx, t, c, s, files); e != nil {
			return e
		}
		files, e = a.scan(ctx, c, str(t, "path"))
		if e != nil {
			return e
		}
	}
	root := str(t, "strmPath")
	if e = os.MkdirAll(root, 0755); e != nil {
		return e
	}
	manifest, _ := a.Store.Get("manifest", num(t, "id"))
	previous := obj(manifest, "entries")
	ownedRec, e := a.Store.Get("owned", num(t, "id"))
	if e != nil {
		ownedRec = Object{}
	}
	owned := obj(ownedRec, "files")
	entries := Object{}
	dirs := map[string][]remoteFile{}
	for _, f := range files {
		b, _ := json.Marshal(f)
		entries[f.Path] = hashBytes(b)
		dirs[path.Dir(f.Path)] = append(dirs[path.Dir(f.Path)], f)
	}
	fp, _ := json.Marshal(Object{"task": Object{"path": t["path"], "strmPath": t["strmPath"], "renameRegex": t["renameRegex"], "libraryType": t["libraryType"], "needScrap": t["needScrap"], "skipInvalidStructure": t["skipInvalidStructure"]}, "openlist": c, "system": s})
	fingerprint := hashBytes(fp)
	changedDirs := map[string]bool{}
	for p, h := range entries {
		if previous[p] != h {
			changedDirs[path.Dir(p)] = true
		}
	}
	for p, h := range previous {
		if entries[p] != h {
			changedDirs[path.Dir(p)] = true
		}
	}
	reset := str(manifest, "fingerprint") != fingerprint || !boolean(t, "isIncrement", true)
	desired := map[string]bool{}
	sourceByOutput := map[string]string{}
	vids := []remoteFile{}
	for _, f := range files {
		if f.IsDir || !video(f.Name, s) {
			continue
		}
		rel, e := remoteRelative(str(t, "path"), f.Path)
		if e != nil {
			return e
		}
		if boolean(t, "skipInvalidStructure", false) && str(t, "libraryType") != "auto" && structureReason(rel, str(t, "libraryType")) != "" {
			continue
		}
		dest, e := outputPath(str(t, "path"), f.Path, str(t, "renameRegex"))
		if e != nil {
			return e
		}
		key := strings.ToLower(dest)
		if src, ok := sourceByOutput[key]; ok && src != f.Path {
			return errors.New("输出文件名冲突: " + dest)
		}
		sourceByOutput[key] = f.Path
		desired[dest] = true
		vids = append(vids, f)
	}
	r["total"] = len(vids)
	r["stage"] = "WRITE_STRM"
	a.updateRun(r)
	write := func(rel, source string, b []byte) error {
		if old, ok := owned[rel]; ok {
			m, _ := old.(map[string]any)
			if str(m, "source") != source && str(m, "source") != "" {
				if str(m, "hash") == hashBytes(b) {
					desired[rel] = true
					return nil
				}
				return fmt.Errorf("输出已属于其他源文件: %s", rel)
			}
		}
		ch, e := writeOutput(root, rel, b)
		if e != nil {
			return e
		}
		if ch {
			r["changed"] = num(r, "changed") + 1
		}
		owned[rel] = Object{"source": source, "hash": hashBytes(b)}
		_, e = a.Store.Save("owned", num(t, "id"), Object{"files": owned})
		desired[rel] = true
		return e
	}
	for i, f := range vids {
		if e = ctx.Err(); e != nil {
			return e
		}
		dest, _ := outputPath(str(t, "path"), f.Path, str(t, "renameRegex"))
		_, missing := os.Stat(filepath.Join(root, dest))
		if reset || changedDirs[path.Dir(f.Path)] || missing != nil {
			e = write(dest, f.Path, []byte(strmURL(c, f)))
			if e == nil && boolean(t, "needScrap", false) {
				e = a.processMetadata(ctx, t, c, s, f, dirs[path.Dir(f.Path)], dest, write)
			}
			if e != nil {
				r["failed"] = num(r, "failed") + 1
				issues, _ := r["issues"].([]any)
				r["issues"] = append(issues, Object{"sourcePath": f.Path, "reason": e.Error()})
			} else {
				r["processed"] = num(r, "processed") + 1
			}
		} else {
			r["skipped"] = num(r, "skipped") + 1
		}
		r["progress"] = ((i + 1) * 90) / max(1, len(vids))
		a.updateRun(r)
	}
	if num(r, "failed") > 0 {
		return nil
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	r["stage"] = "CLEANUP"
	a.updateRun(r)
	// Keep metadata whose source remains present. Only owned outputs from deleted sources are cleaned.
	for rel, v := range owned {
		m, _ := v.(map[string]any)
		src := str(m, "source")
		if desired[rel] || entries[src] != nil && !strings.HasSuffix(strings.ToLower(rel), ".strm") {
			continue
		}
		rr, e := os.OpenRoot(root)
		if e != nil {
			return e
		}
		b, e := rr.ReadFile(rel)
		if os.IsNotExist(e) {
			rr.Close()
			delete(owned, rel)
			continue
		}
		if e != nil {
			rr.Close()
			return e
		}
		if hashBytes(b) != str(m, "hash") {
			rr.Close()
			continue
		}
		trash := filepath.Join(fmt.Sprint(num(r, "id")), rel)
		if _, e = writeOutput(filepath.Join(a.Config.DataDir, "trash"), trash, b); e != nil {
			rr.Close()
			return e
		}
		if e = rr.Remove(rel); e != nil {
			rr.Close()
			return e
		}
		rr.Close()
		delete(owned, rel)
		r["changed"] = num(r, "changed") + 1
		r["cleaned"] = num(r, "cleaned") + 1
	}
	if _, e = a.Store.Save("owned", num(t, "id"), Object{"files": owned}); e != nil {
		return e
	}
	_, e = a.Store.Save("manifest", num(t, "id"), Object{"entries": entries, "fingerprint": fingerprint})
	return e
}
