package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"
	"strconv"
	"strings"
)

func (a *App) directoryTree(ctx context.Context, t, c Object, dir string) (Object, error) {
	if dir != str(t, "path") {
		if _, e := remoteRelative(str(t, "path"), dir); e != nil {
			return nil, e
		}
	}
	fs, e := a.list(ctx, c, dir)
	if e != nil {
		return nil, e
	}
	s, e := a.settings()
	if e != nil {
		return nil, e
	}
	children := []Object{}
	count := 0
	for _, f := range fs {
		if f.IsDir {
			children = append(children, Object{"name": f.Name, "path": f.Path, "mediaFile": false, "videoFileCount": 0, "childrenLoaded": false, "children": []Object{}})
		} else if video(f.Name, s) {
			count++
			children = append(children, Object{"name": f.Name, "path": f.Path, "mediaFile": true, "videoFileCount": 1, "childrenLoaded": true, "children": []Object{}})
		}
	}
	return Object{"name": path.Base(dir), "path": dir, "mediaFile": false, "videoFileCount": count, "childrenLoaded": true, "children": children}, nil
}
func (a *App) preview(ctx context.Context, t, c, s, req Object) (Object, error) {
	dir := str(req, "directoryPath")
	if dir != str(t, "path") {
		if _, e := remoteRelative(str(t, "path"), dir); e != nil {
			return nil, e
		}
	}
	files, e := a.scan(ctx, c, dir)
	flat := false
	if e != nil || video(path.Base(dir), s) && len(files) == 0 {
		m, ge := a.remote(ctx, c, "get", Object{"path": dir, "password": ""})
		if ge != nil || boolean(m, "is_dir", true) {
			return nil, errors.New("无法读取所选媒体路径")
		}
		if !video(path.Base(dir), s) {
			return nil, errors.New("请选择媒体目录或视频文件")
		}
		var selected remoteFile
		raw, _ := json.Marshal(m)
		_ = json.Unmarshal(raw, &selected)
		selected.Name = path.Base(dir)
		selected.Path = dir
		files = []remoteFile{selected}
		flat = true
		siblings, se := a.list(ctx, c, path.Dir(dir))
		if se != nil {
			return nil, se
		}
		base := strings.TrimSuffix(path.Base(dir), path.Ext(dir))
		for _, asset := range siblings {
			if !asset.IsDir && asset.Path != dir && strings.HasPrefix(asset.Name, base+".") && !video(asset.Name, s) {
				files = append(files, asset)
			}
		}
	}
	typ := str(t, "libraryType")
	if str(req, "mediaType") != "" {
		typ = str(req, "mediaType")
	}
	if typ == "auto" || typ == "" {
		for _, f := range files {
			if f.IsDir || !video(f.Name, s) {
				continue
			}
			_, ep := mediaNumbersConfig(s, f.Path)
			if ep > 0 {
				typ = "tv"
				break
			}
		}
	}
	m, e := a.recognize(ctx, s, dir, typ, str(req, "title"), str(req, "year"), num(req, "tmdbId"))
	if e != nil {
		return Object{"directoryPath": dir, "matched": false, "matchMessage": e.Error(), "proposedFileRenames": []Object{}, "proposedDirectoryRenames": []Object{}, "proposedDirectoryCreates": []string{}, "generatedFiles": []string{}, "renamedGeneratedFiles": []string{}}, nil
	}
	title := safeName(str(m, "title"))
	directoryName := title + " (" + str(m, "year") + ") {tmdb=" + fmt.Sprint(m["tmdbId"]) + "}"
	renames := []Object{}
	generated := []string{}
	count := 0
	for _, f := range files {
		if f.IsDir || !video(f.Name, s) {
			continue
		}
		count++
		newBase := title
		if str(m, "mediaType") == "tv" {
			season, ep := mediaNumbersConfig(s, f.Path)
			if ep == 0 {
				return nil, fmt.Errorf("无法识别集数: %s", f.Name)
			}
			newBase += fmt.Sprintf(" S%02dE%02d", season, ep)
		}
		newName := newBase + path.Ext(f.Name)
		renames = append(renames, Object{"sourcePath": f.Path, "sourceName": f.Name, "targetName": newName, "targetDirectory": "", "assetType": "video"})
		oldBase := strings.TrimSuffix(f.Name, path.Ext(f.Name))
		for _, asset := range files {
			if asset.IsDir || asset.Path == f.Path || path.Dir(asset.Path) != path.Dir(f.Path) || !strings.HasPrefix(asset.Name, oldBase+".") {
				continue
			}
			renames = append(renames, Object{"sourcePath": asset.Path, "sourceName": asset.Name, "targetName": newBase + strings.TrimPrefix(asset.Name, oldBase), "targetDirectory": "", "assetType": "sidecar"})
		}
		generated = append(generated, strings.TrimSuffix(f.Name, path.Ext(f.Name))+".nfo")
	}
	seasonRenames := []Object{}
	if str(m, "mediaType") == "tv" {
		for _, f := range files {
			if !f.IsDir {
				continue
			}
			if n, ok := parseSeason(f.Name); ok {
				target := fmt.Sprintf("Season %02d", n)
				if target != f.Name {
					seasonRenames = append(seasonRenames, Object{"sourcePath": f.Path, "sourceName": f.Name, "targetName": target, "seasonNumber": n})
				}
			}
		}
		sort.Slice(seasonRenames, func(i, j int) bool {
			return len(str(seasonRenames[i], "sourcePath")) > len(str(seasonRenames[j], "sourcePath"))
		})
	}
	targets := map[string]string{}
	for _, item := range renames {
		key := strings.ToLower(path.Join(path.Dir(str(item, "sourcePath")), str(item, "targetName")))
		if old, ok := targets[key]; ok && old != str(item, "sourcePath") {
			return nil, errors.New("整理后文件重名，请修正识别信息")
		}
		targets[key] = str(item, "sourcePath")
	}
	b, _ := json.Marshal(files)
	p := Object{"directoryPath": dir, "mediaType": m["mediaType"], "matched": true, "searchTitle": req["title"], "searchYear": req["year"], "matchMessage": "匹配成功", "tmdbId": m["tmdbId"], "title": m["title"], "originalTitle": m["original_title"], "year": m["year"], "overview": m["overview"], "voteAverage": m["vote_average"], "videoFileCount": count, "organizeFlatMovie": flat, "proposedDirectoryName": directoryName, "proposedDirectoryRenames": seasonRenames, "proposedDirectoryCreates": []string{}, "proposedFileRenames": renames, "generatedFiles": generated, "renamedGeneratedFiles": generated, "sourceFingerprint": hashBytes(b), "metadata": m}
	if flat {
		p["proposedDirectoryCreates"] = []string{path.Join(path.Dir(dir), directoryName)}
	}
	for field, image := range map[string]string{"posterUrl": "poster_path", "backdropUrl": "backdrop_path"} {
		if str(m, image) != "" {
			p[field] = strings.TrimRight(str(obj(s, "tmdb"), "imageBaseUrl"), "/") + "/t/p/w500" + str(m, image)
		}
	}
	raw, _ := json.Marshal(Object{"directoryPath": dir, "sourceFingerprint": p["sourceFingerprint"], "directoryName": directoryName, "renames": renames, "seasonRenames": seasonRenames, "metadata": m, "settings": s, "openlist": c})
	p["planHash"] = hashBytes(raw)
	_, e = a.Store.Save("preview", num(t, "id"), p)
	return p, e
}
func (a *App) submitManual(id int64, req Object) (Object, error) {
	t, c, e := a.taskConfig(id)
	if e != nil {
		return nil, e
	}
	s, e := a.settings()
	if e != nil {
		return nil, e
	}
	a.mu.Lock()
	if a.active[id] != nil {
		a.mu.Unlock()
		return nil, errors.New("任务正在执行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.active[id] = cancel
	a.mu.Unlock()
	p, e := a.preview(ctx, t, c, s, req)
	if e == nil && !boolean(p, "matched", false) {
		e = errors.New(str(p, "matchMessage"))
	}
	if e == nil && str(req, "planHash") != "" && str(req, "planHash") != str(p, "planHash") {
		e = errors.New("预览已变化，请重新预览")
	}
	if e != nil {
		cancel()
		a.mu.Lock()
		delete(a.active, id)
		a.mu.Unlock()
		return nil, e
	}
	r, e := a.Store.Save("manual", 0, Object{"taskId": id, "directoryPath": req["directoryPath"], "finalDirectoryPath": req["directoryPath"], "mediaType": p["mediaType"], "tmdbId": p["tmdbId"], "renameMedia": boolean(req, "renameMedia", false), "status": "PENDING", "stage": "PENDING", "progress": 0, "uploadedFiles": []string{}, "steps": Object{}, "plan": p})
	if e != nil {
		cancel()
		a.mu.Lock()
		delete(a.active, id)
		a.mu.Unlock()
		return nil, e
	}
	result := publicRun(r)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() { cancel(); a.mu.Lock(); delete(a.active, id); a.mu.Unlock() }()
		r["status"] = "RUNNING"
		r["startedAt"] = now()
		e := a.executeManual(ctx, t, c, s, r)
		r["status"] = "SUCCEEDED"
		if e != nil {
			r["status"] = "FAILED"
			r["errorMessage"] = e.Error()
			r["message"] = e.Error()
		}
		if ctx.Err() != nil {
			r["status"] = "FAILED"
			r["message"] = "已取消；已完成的远端操作保留"
		}
		r["progress"] = 100
		r["stage"] = "COMPLETED"
		r["completedAt"] = now()
		a.Store.Save("manual", num(r, "id"), r)
		if ctx.Err() == nil {
			_ = a.notify(ctx, obj(s, "notifications"), t, r)
		}
	}()
	return result, nil
}
func (a *App) manualStep(ctx context.Context, c, r Object, key, endpoint string, payload Object) error {
	steps := obj(r, "steps")
	if str(obj(steps, key), "state") == "done" {
		return nil
	}
	if str(obj(steps, key), "state") == "intent" {
		switch endpoint {
		case "rename":
			source := str(payload, "path")
			target := path.Join(path.Dir(source), str(payload, "name"))
			siblings, err := a.list(ctx, c, path.Dir(source))
			if err != nil {
				return err
			}
			oldExists, newExists := false, false
			for _, f := range siblings {
				if f.Path == source {
					oldExists = true
				}
				if f.Path == target {
					newExists = true
				}
			}
			if !oldExists && newExists {
				steps[key] = Object{"state": "done", "endpoint": endpoint, "payload": payload}
				_, err = a.Store.Save("manual", num(r, "id"), r)
				return err
			}
			if oldExists && newExists || !oldExists && !newExists {
				return errors.New("远端状态不明确，请人工核对重命名结果")
			}
		case "mkdir":
			siblings, err := a.list(ctx, c, path.Dir(str(payload, "path")))
			if err != nil {
				return err
			}
			for _, f := range siblings {
				if f.Path == str(payload, "path") && f.IsDir {
					steps[key] = Object{"state": "done"}
					_, err = a.Store.Save("manual", num(r, "id"), r)
					return err
				}
			}
		case "move":
			src, e := a.list(ctx, c, str(payload, "src_dir"))
			if e != nil {
				return e
			}
			dst, e := a.list(ctx, c, str(payload, "dst_dir"))
			if e != nil {
				return e
			}
			srcNames, dstNames := map[string]bool{}, map[string]bool{}
			for _, f := range src {
				srcNames[f.Name] = true
			}
			for _, f := range dst {
				dstNames[f.Name] = true
			}
			raw, _ := json.Marshal(payload["names"])
			var names []string
			_ = json.Unmarshal(raw, &names)
			complete, untouched := true, true
			for _, name := range names {
				complete = complete && !srcNames[name] && dstNames[name]
				untouched = untouched && srcNames[name] && !dstNames[name]
			}
			if complete {
				steps[key] = Object{"state": "done", "endpoint": endpoint, "payload": payload}
				_, e = a.Store.Save("manual", num(r, "id"), r)
				return e
			}
			if !untouched {
				return errors.New("移动结果部分完成或存在冲突，请人工核对远端目录")
			}
		}
	}
	steps[key] = Object{"state": "intent", "endpoint": endpoint, "payload": payload}
	r["steps"] = steps
	if _, e := a.Store.Save("manual", num(r, "id"), r); e != nil {
		return e
	}
	_, e := a.remote(ctx, c, endpoint, payload)
	if e != nil {
		return fmt.Errorf("远端操作结果需核对 (%s): %w", endpoint, e)
	}
	steps[key] = Object{"state": "done", "endpoint": endpoint, "payload": payload}
	_, e = a.Store.Save("manual", num(r, "id"), r)
	return e
}
func (a *App) executeManual(ctx context.Context, t, c, s, r Object) error {
	p := obj(r, "plan")
	dir := str(p, "directoryPath")
	m := obj(p, "metadata")
	flat := boolean(p, "organizeFlatMovie", false)
	r["stage"] = "RENAMING"
	if _, e := a.Store.Save("manual", num(r, "id"), r); e != nil {
		return e
	}
	if boolean(r, "renameMedia", false) {
		items, _ := p["proposedFileRenames"].([]any)
		if items == nil {
			raw, _ := json.Marshal(p["proposedFileRenames"])
			_ = json.Unmarshal(raw, &items)
		}
		for i, x := range items {
			item, _ := x.(map[string]any)
			if str(item, "sourceName") == str(item, "targetName") {
				continue
			}
			source := str(item, "sourcePath")
			key := "rename-" + strconv.Itoa(i)
			state := str(obj(obj(r, "steps"), key), "state")
			if state == "" {
				siblings, err := a.list(ctx, c, path.Dir(source))
				if err != nil {
					return err
				}
				for _, f := range siblings {
					if strings.EqualFold(f.Name, str(item, "targetName")) {
						return errors.New("目标名称已存在，停止重命名")
					}
				}
			}
			if err := a.manualStep(ctx, c, r, key, "rename", Object{"path": source, "name": str(item, "targetName")}); err != nil {
				return err
			}
			if state != "done" {
				r["renamedFileCount"] = num(r, "renamedFileCount") + 1
			}
			if flat && str(item, "assetType") == "video" {
				dir = path.Join(path.Dir(dir), str(item, "targetName"))
			}
		}
		rawDirs, _ := json.Marshal(p["proposedDirectoryRenames"])
		var seasonItems []Object
		_ = json.Unmarshal(rawDirs, &seasonItems)
		for i, item := range seasonItems {
			source := str(item, "sourcePath")
			key := "season-" + strconv.Itoa(i)
			if str(obj(obj(r, "steps"), key), "state") == "" {
				siblings, err := a.list(ctx, c, path.Dir(source))
				if err != nil {
					return err
				}
				for _, f := range siblings {
					if strings.EqualFold(f.Name, str(item, "targetName")) {
						return errors.New("目标季目录已存在")
					}
				}
			}
			if err := a.manualStep(ctx, c, r, key, "rename", Object{"path": source, "name": str(item, "targetName")}); err != nil {
				return err
			}
		}
		if !flat && path.Base(dir) != str(p, "proposedDirectoryName") && dir != str(t, "path") {
			target := path.Join(path.Dir(dir), str(p, "proposedDirectoryName"))
			if str(obj(obj(r, "steps"), "rename-dir"), "state") == "" {
				siblings, err := a.list(ctx, c, path.Dir(dir))
				if err != nil {
					return err
				}
				for _, f := range siblings {
					if f.Path == target {
						return errors.New("目标目录已存在")
					}
				}
			}
			if e := a.manualStep(ctx, c, r, "rename-dir", "rename", Object{"path": dir, "name": str(p, "proposedDirectoryName")}); e != nil {
				return e
			}
			dir = target
			r["renamedDirectoryCount"] = 1
		}
		if flat {
			target := path.Join(path.Dir(dir), str(p, "proposedDirectoryName"))
			if e := a.manualStep(ctx, c, r, "mkdir", "mkdir", Object{"path": target}); e != nil {
				return e
			}
			if e := a.manualStep(ctx, c, r, "move", "move", Object{"src_dir": path.Dir(dir), "dst_dir": target, "names": flatNames(p)}); e != nil {
				return e
			}
			dir = target
		}
	}
	r["finalDirectoryPath"] = dir
	r["stage"] = "UPLOADING"
	a.Store.Save("manual", num(r, "id"), r)
	var files []remoteFile
	var e error
	if flat && !boolean(r, "renameMedia", false) {
		files = []remoteFile{{Name: path.Base(dir), Path: dir}}
	} else {
		files, e = a.scan(ctx, c, dir)
	}
	if e != nil {
		return e
	}
	uploaded := []string{}
	seen := map[string]bool{}
	for _, f := range files {
		if f.IsDir || !video(f.Name, s) {
			continue
		}
		assets, e := a.metadataFiles(ctx, s, m, f.Path)
		if e != nil {
			return e
		}
		for name, b := range assets {
			dest := path.Join(path.Dir(f.Path), name)
			if name == "tvshow.nfo" {
				dest = path.Join(dir, name)
			}
			if seen[dest] {
				continue
			}
			seen[dest] = true
			steps := obj(r, "steps")
			prior := obj(steps, "upload:"+dest)
			if str(prior, "state") == "done" && str(prior, "hash") == hashBytes(b) {
				uploaded = append(uploaded, dest)
				continue
			}
			if str(prior, "state") == "intent" {
				remote, err := a.download(ctx, c, remoteFile{Path: dest})
				if err == nil && hashBytes(remote) == hashBytes(b) {
					steps["upload:"+dest] = Object{"state": "done", "hash": hashBytes(b)}
					r["uploadedFiles"] = append(uploaded, dest)
					if _, err = a.Store.Save("manual", num(r, "id"), r); err != nil {
						return err
					}
					uploaded = append(uploaded, dest)
					continue
				}
				return errors.New("上传结果未知；请核对远端文件，不自动覆盖重试")
			}
			steps["upload:"+dest] = Object{"state": "intent", "hash": hashBytes(b)}
			r["steps"] = steps
			if _, e = a.Store.Save("manual", num(r, "id"), r); e != nil {
				return e
			}
			if e = a.upload(ctx, c, dest, b); e != nil {
				return e
			}
			steps["upload:"+dest] = Object{"state": "done", "hash": hashBytes(b)}
			uploaded = append(uploaded, dest)
			r["uploadedFiles"] = uploaded
			r["progress"] = min(95, len(uploaded)*5)
			if _, e = a.Store.Save("manual", num(r, "id"), r); e != nil {
				return e
			}
		}
	}
	r["message"] = "刮削资料已上传"
	return nil
}
func (a *App) autoRename(ctx context.Context, t, c, s Object, files []remoteFile) error {
	dirs := map[string]bool{}
	for _, f := range files {
		if !f.IsDir && video(f.Name, s) {
			rel, e := remoteRelative(str(t, "path"), f.Path)
			if e != nil {
				return e
			}
			parts := strings.Split(rel, "/")
			if len(parts) > 1 {
				dirs[path.Join(str(t, "path"), parts[0])] = true
			}
		}
	}
	for dir := range dirs {
		p, e := a.preview(ctx, t, c, s, Object{"directoryPath": dir})
		if e != nil {
			return e
		}
		if !boolean(p, "matched", false) {
			return fmt.Errorf("自动整理未匹配 %s: %s", dir, str(p, "matchMessage"))
		}
		r, e := a.Store.Save("manual", 0, Object{"taskId": t["id"], "directoryPath": dir, "renameMedia": true, "plan": p, "status": "RUNNING", "steps": Object{}})
		if e != nil {
			return e
		}
		e = a.executeManual(ctx, t, c, s, r)
		r["status"] = "SUCCEEDED"
		if e != nil {
			r["status"] = "FAILED"
			r["errorMessage"] = e.Error()
		}
		a.Store.Save("manual", num(r, "id"), r)
		if e != nil {
			return e
		}
	}
	return nil
}

// Retry reuses the durable plan. Ambiguous remote writes require reconciliation before retry.
func (a *App) retryManual(taskID, jobID int64) (Object, error) {
	r, e := a.Store.Get("manual", jobID)
	if e != nil {
		return nil, e
	}
	if num(r, "taskId") != taskID {
		return nil, errors.New("作业不属于当前任务")
	}
	if str(r, "status") != "FAILED" && str(r, "status") != "INTERRUPTED" {
		return nil, errors.New("仅失败或中断任务可重试")
	}
	t, c, e := a.taskConfig(taskID)
	if e != nil {
		return nil, e
	}
	s, e := a.settings()
	if e != nil {
		return nil, e
	}
	a.mu.Lock()
	if a.active[taskID] != nil {
		a.mu.Unlock()
		return nil, errors.New("任务正在执行")
	}
	ctx, cancel := context.WithCancel(a.ctx)
	a.active[taskID] = cancel
	a.mu.Unlock()
	result := publicRun(r)
	a.wg.Add(1)
	go func() {
		defer a.wg.Done()
		defer func() { cancel(); a.mu.Lock(); delete(a.active, taskID); a.mu.Unlock() }()
		r["status"] = "RUNNING"
		r["errorMessage"] = ""
		e = a.executeManual(ctx, t, c, s, r)
		r["status"] = "SUCCEEDED"
		if e != nil {
			r["status"] = "FAILED"
			r["errorMessage"] = e.Error()
		}
		r["completedAt"] = now()
		a.Store.Save("manual", jobID, r)
	}()
	return result, nil
}

func flatNames(p Object) []string {
	var items []Object
	b, _ := json.Marshal(p["proposedFileRenames"])
	_ = json.Unmarshal(b, &items)
	names := []string{}
	for _, item := range items {
		names = append(names, str(item, "targetName"))
	}
	return names
}
