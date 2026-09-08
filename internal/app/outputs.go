package app

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Persist intent before a filesystem write. Restart can reconcile the hash without
// treating arbitrary pre-existing user files as application-owned output.
func (a *App) ownedWrite(taskID int64, root, rel, source string, b []byte) (bool, error) {
	rr, e := os.OpenRoot(root)
	if e != nil {
		return false, e
	}
	old, readErr := rr.ReadFile(rel)
	rr.Close()
	var previous string
	err := a.Store.DB.QueryRow("SELECT hash FROM output_files WHERE task_id=? AND path=?", taskID, rel).Scan(&previous)
	if readErr == nil {
		if err != nil {
			return false, errors.New("输出目标已有未认领文件，保留原文件: " + rel)
		}
		if hashBytes(old) != previous {
			return false, errors.New("输出文件被外部修改，保留原文件: " + rel)
		}
	} else if !os.IsNotExist(readErr) {
		return false, readErr
	}
	h := hashBytes(b)
	_, e = a.Store.DB.Exec("INSERT INTO output_journal(task_id,path,root,source,hash) VALUES(?,?,?,?,?) ON CONFLICT(task_id,path) DO UPDATE SET root=excluded.root,source=excluded.source,hash=excluded.hash", taskID, rel, root, source, h)
	if e != nil {
		return false, e
	}
	changed, e := writeOutput(root, rel, b)
	if e != nil {
		return false, e
	}
	tx, e := a.Store.DB.Begin()
	if e != nil {
		return changed, e
	}
	defer tx.Rollback()
	_, e = tx.Exec("INSERT INTO output_files(task_id,path,source,hash) VALUES(?,?,?,?) ON CONFLICT(task_id,path) DO UPDATE SET source=excluded.source,hash=excluded.hash", taskID, rel, source, h)
	if e != nil {
		return changed, e
	}
	if _, e = tx.Exec("DELETE FROM output_journal WHERE task_id=? AND path=?", taskID, rel); e != nil {
		return changed, e
	}
	return changed, tx.Commit()
}
func (a *App) recoverOutputs() error {
	rows, e := a.Store.DB.Query("SELECT task_id,path,root,source,hash FROM output_journal")
	if e != nil {
		return e
	}
	pending := []Object{}
	for rows.Next() {
		var id int64
		var p, root, src, h string
		if e = rows.Scan(&id, &p, &root, &src, &h); e != nil {
			rows.Close()
			return e
		}
		pending = append(pending, Object{"taskId": id, "path": p, "root": root, "source": src, "hash": h})
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	for _, m := range pending {
		rr, e := os.OpenRoot(str(m, "root"))
		if e != nil {
			continue
		}
		b, e := rr.ReadFile(str(m, "path"))
		rr.Close()
		if e == nil && hashBytes(b) == str(m, "hash") {
			if e = a.Store.Own(num(m, "taskId"), str(m, "path"), str(m, "source"), str(m, "hash")); e != nil {
				return e
			}
		}
		if _, e = a.Store.DB.Exec("DELETE FROM output_journal WHERE task_id=? AND path=?", num(m, "taskId"), str(m, "path")); e != nil {
			return e
		}
	}
	return nil
}
func (a *App) registerTrash(handle func(string, endpoint)) {
	handle("GET /api/task-config/{id}/trash/list", func(r *http.Request) (any, error) {
		all, e := a.Store.List("trash")
		out := []Object{}
		for _, m := range all {
			if num(m, "taskId") == idParam(r, "id") && str(m, "status") == "ISOLATED" {
				out = append(out, m)
			}
		}
		return out, e
	})
	handle("POST /api/task-config/{id}/trash/{trashId}/restore", func(r *http.Request) (any, error) {
		id := idParam(r, "id")
		a.mu.Lock()
		defer a.mu.Unlock()
		if a.active[id] != nil {
			return nil, errors.New("运行中不能恢复隔离文件")
		}
		m, e := a.Store.Get("trash", idParam(r, "trashId"))
		if e != nil {
			return nil, e
		}
		if num(m, "taskId") != id || str(m, "status") != "ISOLATED" {
			return nil, errors.New("无效隔离记录")
		}
		t, e := a.Store.Get("tasks", id)
		if e != nil {
			return nil, e
		}
		root := str(t, "strmPath")
		if root != str(m, "root") {
			return nil, errors.New("输出根目录已改变，请人工恢复")
		}
		rr, e := os.OpenRoot(root)
		if e != nil {
			return nil, e
		}
		defer rr.Close()
		if _, e = rr.Stat(str(m, "path")); !os.IsNotExist(e) {
			return nil, errors.New("恢复目标已存在")
		}
		tr, e := os.OpenRoot(filepath.Join(a.Config.DataDir, "trash"))
		if e != nil {
			return nil, e
		}
		defer tr.Close()
		b, e := tr.ReadFile(str(m, "trashPath"))
		if e != nil {
			return nil, e
		}
		if hashBytes(b) != str(m, "hash") {
			return nil, errors.New("隔离文件校验失败")
		}
		if _, e = a.ownedWrite(id, root, str(m, "path"), str(m, "source"), b); e != nil {
			return nil, e
		}
		m["status"] = "RESTORED"
		if _, e = a.Store.Save("trash", num(m, "id"), m); e != nil {
			return nil, e
		}
		return "已恢复", tr.Remove(str(m, "trashPath"))
	})
}
func (a *App) maintenance() {
	defer a.wg.Done()
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			a.Store.DB.Exec("DELETE FROM sessions WHERE expires < ?", time.Now().Unix())
			records, e := a.Store.List("trash")
			if e != nil {
				continue
			}
			root, e := os.OpenRoot(filepath.Join(a.Config.DataDir, "trash"))
			if e != nil {
				continue
			}
			for _, m := range records {
				if str(m, "status") != "ISOLATED" || num(m, "expiresAt") > time.Now().Unix() {
					continue
				}
				if e = root.Remove(str(m, "trashPath")); e == nil || os.IsNotExist(e) {
					m["status"] = "EXPIRED"
					a.Store.Save("trash", num(m, "id"), m)
				}
			}
			root.Close()
		}
	}
}

// Resolve existing ancestors so alternate symlink spellings cannot overlap tasks.
func canonicalRoot(p string) string {
	if q, e := filepath.EvalSymlinks(p); e == nil {
		return q
	}
	parent := filepath.Dir(p)
	if parent == p {
		return p
	}
	return filepath.Join(canonicalRoot(parent), filepath.Base(p))
}
func pathsOverlap(a, b string) bool {
	a = strings.ToLower(filepath.Clean(canonicalRoot(a)))
	b = strings.ToLower(filepath.Clean(canonicalRoot(b)))
	return a == b || strings.HasPrefix(a, b+string(os.PathSeparator)) || strings.HasPrefix(b, a+string(os.PathSeparator))
}
