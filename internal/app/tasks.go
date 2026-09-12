package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/reugn/go-quartz/quartz"
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
	if a.updating {
		a.mu.Unlock()
		return nil, errors.New("正在升级，请稍后执行任务")
	}
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
	a.taskLogger(t, r).Info("任务已入队", "stage", "QUEUED", "incremental", boolean(t, "isIncrement", true))
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
		a.Log.Error("保存任务执行状态失败", "taskId", num(r, "taskId"), "runId", num(r, "id"), "stage", str(r, "stage"), "error", e.Error())
	}
}
func (a *App) execute(ctx context.Context, t, c, s, r Object) {
	started := time.Now()
	logger := a.taskLogger(t, r)
	ctx = context.WithValue(ctx, taskLogKey{}, logger)
	logger.Info("任务开始", "stage", "DISCOVERY", "sourcePath", str(t, "path"), "outputPath", str(t, "strmPath"), "incremental", boolean(t, "isIncrement", true))
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
	level := slog.LevelInfo
	if str(r, "status") == "FAILED" {
		level = slog.LevelError
	}
	if str(r, "status") == "PARTIAL_SUCCESS" || str(r, "status") == "CANCELED" {
		level = slog.LevelWarn
	}
	finishStage := str(r, "stage")
	if str(r, "failureStage") != "" {
		finishStage = str(r, "failureStage")
	}
	logger.Log(ctx, level, "任务结束", "stage", finishStage, "status", str(r, "status"), "total", num(r, "total"), "processed", num(r, "processed"), "failed", num(r, "failed"), "skipped", num(r, "skipped"), "changed", num(r, "changed"), "cleaned", num(r, "cleaned"), "durationMs", time.Since(started).Milliseconds(), "error", str(r, "errorMessage"))
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
		logger.Error("保存任务最后执行时间失败", "error", e.Error())
	}
	if ctx.Err() == nil {
		if str(r, "status") == "SUCCESS" || str(r, "status") == "PARTIAL_SUCCESS" {
			r["mediaRefresh"], e = a.refreshAfter(ctx, t, num(r, "changed") > 0, boolean(t, "isIncrement", true))
			if e != nil {
				r["mediaRefreshError"] = e.Error()
				logger.Warn("媒体库刷新失败", "stage", "MEDIA_REFRESH", "error", e.Error())
			}
		}
		if e = a.notify(ctx, obj(s, "notifications"), t, r); e != nil {
			r["notificationError"] = e.Error()
			logger.Warn("任务通知发送失败", "stage", "NOTIFY", "error", e.Error())
		}
		a.updateRun(r)
	}
}
func (a *App) executeFiles(ctx context.Context, t, c, s, r Object) (runErr error) {
	logger := a.contextLogger(ctx)
	stage, source, output := "DISCOVERY", str(t, "path"), str(t, "strmPath")
	defer func() {
		if runErr != nil && ctx.Err() == nil {
			r["failureStage"] = stage
			logger.Error("任务阶段失败", "stage", stage, "sourcePath", source, "outputPath", output, "error", runErr.Error())
		}
	}()
	logger.Info("开始扫描目录", "stage", "DISCOVERY", "sourcePath", str(t, "path"))
	files, e := a.scan(ctx, c, str(t, "path"))
	if e != nil {
		return e
	}
	logger.Info("目录扫描完成", "stage", "DISCOVERY", "entries", len(files))
	if boolean(t, "autoRenameMedia", false) {
		stage = "RENAME"
		logger.Info("开始自动整理", "stage", "RENAME")
		if e = a.autoRename(ctx, t, c, s, files); e != nil {
			return e
		}
		stage = "DISCOVERY"
		files, e = a.scan(ctx, c, str(t, "path"))
		if e != nil {
			return e
		}
	}
	stage = "PREPARE_OUTPUT"
	root := str(t, "strmPath")
	if e = os.MkdirAll(root, 0755); e != nil {
		return e
	}
	manifest, _ := a.Store.Get("manifest", num(t, "id"))
	previous := obj(manifest, "entries")
	owned, e := a.Store.Owned(num(t, "id"))
	if e != nil {
		return e
	}
	entries := Object{str(t, "path"): "directory"}
	dirs := map[string][]remoteFile{}
	for _, f := range files {
		b, _ := json.Marshal(f)
		entries[f.Path] = hashBytes(b)
		dirs[path.Dir(f.Path)] = append(dirs[path.Dir(f.Path)], f)
	}
	fp, _ := json.Marshal(Object{"movieOutputVersion": 1, "recognitionVersion": 2, "task": Object{"movieVersions": t["movieVersions"], "movieNaming": t["movieNaming"], "skipMovieExtras": t["skipMovieExtras"], "path": t["path"], "strmPath": t["strmPath"], "renameRegex": t["renameRegex"], "libraryType": t["libraryType"], "needScrap": t["needScrap"], "skipInvalidStructure": t["skipInvalidStructure"]}, "openlist": c, "system": s})
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
	destinations := map[string]string{}
	outputGroups := map[string][]string{}
	eligible := map[string]bool{}
	vids := []remoteFile{}
	for _, f := range files {
		source = f.Path
		if f.IsDir || !video(f.Name, s) {
			continue
		}
		rel, e := remoteRelative(str(t, "path"), f.Path)
		if e != nil {
			return e
		}
		if str(t, "libraryType") == "movie" && boolean(t, "skipMovieExtras", true) && movieExtra(rel) {
			continue
		}
		if boolean(t, "skipInvalidStructure", false) && str(t, "libraryType") != "auto" && structureReason(rel, str(t, "libraryType")) != "" {
			continue
		}
		dest, e := taskOutputPath(t, f.Path)
		if e != nil {
			return e
		}
		destinations[f.Path] = dest
		eligible[f.Path] = true
		vids = append(vids, f)
	}
	var selections []Object
	vids, selections = selectMovieVersions(t, vids)
	r["filteredVersions"] = len(selections)
	r["movieSelections"] = selections
	for _, selection := range selections {
		delete(destinations, str(selection, "filtered"))
	}
	for _, f := range vids {
		key := strings.ToLower(destinations[f.Path])
		outputGroups[key] = append(outputGroups[key], f.Path)
	}
	// Preserve every edition/file without overwriting when clean names coincide.
	for _, f := range vids {
		dest := destinations[f.Path]
		if len(outputGroups[strings.ToLower(dest)]) > 1 && str(t, "libraryType") == "movie" && str(t, "movieNaming") != "original" && str(t, "renameRegex") == "" {
			dest = strings.TrimSuffix(dest, ".strm") + " - version-" + hashBytes([]byte(f.Path))[:12] + ".strm"
		}
		key := strings.ToLower(dest)
		if src, ok := sourceByOutput[key]; ok && src != f.Path {
			return fmt.Errorf("输出文件名冲突: %s，源文件: %s 与 %s", dest, src, f.Path)
		}
		sourceByOutput[key] = f.Path
		destinations[f.Path] = dest
		desired[dest] = true
	}

	stage = "WRITE_STRM"
	r["total"] = len(vids)
	r["stage"] = "WRITE_STRM"
	a.updateRun(r)
	logger.Info("文件筛选完成", "stage", "WRITE_STRM", "total", len(vids), "filteredVersions", len(selections))
	write := func(rel, source string, b []byte) error {
		if old, ok := owned[rel]; ok {
			m, _ := old.(map[string]any)
			if str(m, "source") != source && str(m, "source") != "" && !canReplaceMovieSource(t, str(m, "source"), source) {
				if str(m, "hash") == hashBytes(b) {
					desired[rel] = true
					return nil
				}
				return fmt.Errorf("输出已属于其他源文件: %s", rel)
			}
		}
		ch, e := a.ownedWrite(num(t, "id"), root, rel, source, b)
		if e != nil {
			return e
		}
		if ch {
			r["changed"] = num(r, "changed") + 1
		}
		owned[rel] = Object{"source": source, "hash": hashBytes(b)}

		desired[rel] = true
		return e
	}
	for i, f := range vids {
		if e = ctx.Err(); e != nil {
			return e
		}
		source = f.Path
		dest := destinations[f.Path]
		output = filepath.Join(root, dest)
		_, missing := os.Stat(filepath.Join(root, dest))
		metadataMissing := false
		if boolean(t, "needScrap", false) && boolean(obj(s, "scraping"), "enabled", true) {
			_, err := os.Stat(filepath.Join(root, strings.TrimSuffix(dest, ".strm")+".nfo"))
			metadataMissing = os.IsNotExist(err)
		}
		sourceChanged := str(obj(owned, dest), "source") != f.Path
		if reset || changedDirs[path.Dir(f.Path)] || missing != nil || metadataMissing || sourceChanged {
			fileStarted := time.Now()
			fileLog := logger.With("sourcePath", f.Path, "outputPath", filepath.Join(root, dest), "fileIndex", i+1, "total", len(vids))
			stage = "WRITE_STRM"
			fileLog.Info("开始处理文件", "stage", stage)
			e = write(dest, f.Path, []byte(strmURL(c, f)))
			if e == nil && boolean(t, "needScrap", false) {
				stage = "METADATA"
				fileLog.Info("开始处理元数据及附属文件", "stage", stage)
				e = a.processMetadata(ctx, t, c, s, f, dirs[path.Dir(f.Path)], dest, write)
			}
			if e != nil {
				if ctx.Err() != nil {
					fileLog.Warn("文件处理已取消", "stage", stage, "error", ctx.Err().Error(), "durationMs", time.Since(fileStarted).Milliseconds())
					return ctx.Err()
				}
				fileLog.Error("文件处理失败", "stage", stage, "error", e.Error(), "durationMs", time.Since(fileStarted).Milliseconds())
				r["failed"] = num(r, "failed") + 1
				issues, _ := r["issues"].([]any)
				r["issues"] = append(issues, Object{"sourcePath": f.Path, "outputPath": filepath.Join(root, dest), "stage": stage, "reason": e.Error()})
			} else {
				fileLog.Info("文件处理完成", "stage", stage, "durationMs", time.Since(fileStarted).Milliseconds())
				r["processed"] = num(r, "processed") + 1
			}
		} else {
			logger.Debug("跳过未变化文件", "stage", "WRITE_STRM", "sourcePath", f.Path, "outputPath", filepath.Join(root, dest))
			r["skipped"] = num(r, "skipped") + 1
		}
		r["progress"] = ((i + 1) * 90) / max(1, len(vids))
		if i%25 == 0 || i+1 == len(vids) {
			a.updateRun(r)
		}
	}
	if num(r, "failed") > 0 {
		logger.Warn("存在失败文件，跳过清理与扫描快照保存，下次执行将重试", "stage", "CLEANUP", "failed", num(r, "failed"))
		return nil
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	stage = "CLEANUP"
	logger.Info("开始清理失效输出", "stage", "CLEANUP")
	r["stage"] = "CLEANUP"
	a.updateRun(r)
	// Keep metadata whose source remains present. Only owned outputs from deleted sources are cleaned.
	for rel, v := range owned {
		m, _ := v.(map[string]any)
		src := str(m, "source")
		keep := entries[src] != nil && (!eligible[src] || !strings.HasSuffix(strings.ToLower(rel), ".strm"))
		if keep && eligible[src] && str(t, "libraryType") == "movie" && !strings.HasSuffix(strings.ToLower(rel), ".strm") {
			stem := strings.TrimSuffix(destinations[src], ".strm")
			keep = strings.HasPrefix(rel, stem+".") || strings.HasPrefix(rel, stem+"-poster.") || strings.HasPrefix(rel, stem+"-fanart.")
		}
		if desired[rel] || keep {
			continue
		}
		source, output = src, filepath.Join(root, rel)
		logger.Debug("检查失效输出", "stage", "CLEANUP", "sourcePath", src, "outputPath", filepath.Join(root, rel))
		rr, e := os.OpenRoot(root)
		if e != nil {
			return e
		}
		b, e := rr.ReadFile(rel)
		if os.IsNotExist(e) {
			rr.Close()
			delete(owned, rel)
			if e = a.Store.Unown(num(t, "id"), rel); e != nil {
				return e
			}
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
		record, e := a.Store.Save("trash", 0, Object{"taskId": t["id"], "root": root, "path": rel, "trashPath": trash, "source": src, "hash": str(m, "hash"), "status": "ISOLATED", "expiresAt": time.Now().Add(7 * 24 * time.Hour).Unix()})
		_ = record
		if e != nil {
			rr.Close()
			return e
		}
		if e = rr.Remove(rel); e != nil {
			rr.Close()
			return e
		}
		rr.Close()
		delete(owned, rel)
		if e = a.Store.Unown(num(t, "id"), rel); e != nil {
			return e
		}
		r["changed"] = num(r, "changed") + 1
		r["cleaned"] = num(r, "cleaned") + 1
		logger.Info("失效输出已移至回收站", "stage", "CLEANUP", "sourcePath", src, "outputPath", filepath.Join(root, rel))
	}
	stage, source, output = "SAVE_MANIFEST", str(t, "path"), root
	_, e = a.Store.Save("manifest", num(t, "id"), Object{"entries": entries, "fingerprint": fingerprint})
	return e
}
