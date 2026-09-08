package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

func MigrateLegacy(source string, c Config, dry bool, w io.Writer) error {
	if source == "" {
		return errors.New("需要 --source 旧数据目录（请先停止旧程序）")
	}
	source, e := filepath.Abs(source)
	if e != nil {
		return e
	}
	if e = c.Resolve(); e != nil {
		return e
	}
	if source == c.DataDir {
		return errors.New("源目录和目标目录必须不同")
	}
	dbPath := filepath.Join(source, "db", "openlist2strm.db")
	if _, e = os.Stat(dbPath); e != nil {
		return e
	}
	old, e := sql.Open("sqlite", "file:"+filepath.ToSlash(dbPath)+"?mode=ro")
	if e != nil {
		return e
	}
	defer old.Close()
	report := Object{"source": source, "target": c.DataDir, "dryRun": dry, "tasksDisabled": true, "counts": Object{}, "warnings": []string{"旧任务时区默认 Asia/Shanghai；请核对后启用", "旧输出未认领，不会自动清理", "首次导入后请检查正则和 Cron"}}
	data := map[string][]Object{}
	for table, kind := range map[string]string{"openlist_config": "openlist", "task_config": "tasks", "media_server_config": "media"} {
		rows, e := old.Query("SELECT * FROM " + table)
		if e != nil {
			if kind == "media" {
				continue
			}
			return e
		}
		cols, e := rows.Columns()
		if e != nil {
			rows.Close()
			return e
		}
		for rows.Next() {
			vals := make([]any, len(cols))
			ptrs := make([]any, len(cols))
			for i := range vals {
				ptrs[i] = &vals[i]
			}
			if e = rows.Scan(ptrs...); e != nil {
				rows.Close()
				return e
			}
			m := Object{}
			for i, k := range cols {
				parts := strings.Split(k, "_")
				key := parts[0]
				for _, p := range parts[1:] {
					if p != "" {
						key += strings.ToUpper(p[:1]) + p[1:]
					}
				}
				v := vals[i]
				if b, ok := v.([]byte); ok {
					v = string(b)
				}
				if strings.HasPrefix(key, "is") || strings.HasPrefix(key, "need") || key == "enableUrlEncoding" || key == "skipInvalidStructure" || key == "autoRenameMedia" {
					if n, ok := v.(int64); ok {
						v = n != 0
					}
				}
				m[key] = v
			}
			if kind == "tasks" {
				m["isActive"] = false
				m["strmPath"] = filepath.Join(c.StrmRoot, safeName(str(m, "taskName")))
			}
			data[kind] = append(data[kind], m)
		}
		if e = rows.Err(); e != nil {
			rows.Close()
			return e
		}
		rows.Close()
		obj(report, "counts")[kind] = len(data[kind])
	}
	if dry {
		return json.NewEncoder(w).Encode(report)
	}
	s, e := OpenStore(c.DataDir)
	if e != nil {
		return e
	}
	defer s.DB.Close()
	var count int
	if e = s.DB.QueryRow("SELECT count(*) FROM records").Scan(&count); e != nil {
		return e
	}
	if count > 0 {
		return errors.New("目标数据库非空；禁止重复覆盖导入")
	}
	tx, e := s.DB.Begin()
	if e != nil {
		return e
	}
	defer tx.Rollback()
	for kind, ms := range data {
		var maxID int64
		for _, m := range ms {
			id := num(m, "id")
			if id > maxID {
				maxID = id
			}
			b, _ := json.Marshal(m)
			if _, e = tx.Exec("INSERT INTO records(kind,id,body) VALUES(?,?,?)", kind, id, string(b)); e != nil {
				return e
			}
		}
		if _, e = tx.Exec("INSERT INTO sequences(kind,value) VALUES(?,?)", kind, maxID); e != nil {
			return e
		}
	}
	for file, key := range map[string]string{"systemconf.json": "system"} {
		p := filepath.Join(source, "config", file)
		if b, e := os.ReadFile(p); e == nil {
			var m Object
			if e = json.Unmarshal(b, &m); e != nil {
				return e
			}
			m["timezone"] = "Asia/Shanghai"
			m = merge(DefaultSettings(), m)
			obj(m, "log")["reportUsageData"] = false
			b, _ = json.Marshal(m)
			if _, e = tx.Exec("INSERT INTO settings(key,value) VALUES(?,?)", key, string(b)); e != nil {
				return e
			}
		}
	}
	if b, e := os.ReadFile(filepath.Join(source, "config", "userInfo.json")); e == nil {
		var m Object
		if e = json.Unmarshal(b, &m); e != nil {
			return e
		}
		if str(m, "username") != "" && str(m, "pwd") != "" {
			if _, e = tx.Exec("INSERT INTO admin(id,username,password) VALUES(1,?,?)", str(m, "username"), str(m, "pwd")); e != nil {
				return e
			}
		}
	}
	if e = tx.Commit(); e != nil {
		return fmt.Errorf("迁移失败: %w", e)
	}
	return json.NewEncoder(w).Encode(report)
}
