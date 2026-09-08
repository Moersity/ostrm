package app

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Object = map[string]any

func str(m Object, k string) string { s, _ := m[k].(string); return s }
func num(m Object, k string) int64 {
	switch v := m[k].(type) {
	case float64:
		return int64(v)
	case int:
		return int64(v)
	case int64:
		return v
	case json.Number:
		n, _ := v.Int64()
		return n
	}
	return 0
}
func boolean(m Object, k string, d bool) bool {
	v, ok := m[k].(bool)
	if ok {
		return v
	}
	return d
}
func obj(m Object, k string) Object {
	v, _ := m[k].(map[string]any)
	if v == nil {
		return Object{}
	}
	return v
}
func clone(m Object) Object {
	b, _ := json.Marshal(m)
	var v Object
	_ = json.Unmarshal(b, &v)
	if v == nil {
		v = Object{}
	}
	return v
}
func now() string { return time.Now().Format("2006-01-02T15:04:05.000") }
func merge(a, b Object) Object {
	r := clone(a)
	for k, v := range b {
		if m, ok := v.(map[string]any); ok {
			r[k] = merge(obj(r, k), m)
		} else {
			r[k] = v
		}
	}
	return r
}

type Store struct {
	DB *sql.DB
	mu sync.Mutex
}

func OpenStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "ostrm.db"))
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{DB: db}
	fail := func(e error) (*Store, error) { db.Close(); return nil, e }
	if _, err = db.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;`); err != nil {
		return fail(err)
	}
	var version int
	if err = db.QueryRow("PRAGMA user_version").Scan(&version); err != nil {
		return fail(err)
	}
	if version > 1 {
		return fail(errors.New("数据库版本高于程序支持的版本"))
	}
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS records(kind TEXT NOT NULL,id INTEGER NOT NULL,body TEXT NOT NULL,PRIMARY KEY(kind,id));
 CREATE TABLE IF NOT EXISTS sequences(kind TEXT PRIMARY KEY,value INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS admin(id INTEGER PRIMARY KEY CHECK(id=1),username TEXT NOT NULL,password TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS sessions(id TEXT PRIMARY KEY,username TEXT NOT NULL,expires INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS settings(key TEXT PRIMARY KEY,value TEXT NOT NULL);
 PRAGMA user_version=1;`)
	if err != nil {
		return fail(err)
	}
	_ = os.Chmod(filepath.Join(dir, "ostrm.db"), 0600)
	return s, nil
}
func (s *Store) List(kind string) ([]Object, error) {
	rows, e := s.DB.Query("SELECT body FROM records WHERE kind=? ORDER BY id", kind)
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	out := []Object{}
	for rows.Next() {
		var b string
		if e = rows.Scan(&b); e != nil {
			return nil, e
		}
		var m Object
		if e = json.Unmarshal([]byte(b), &m); e != nil {
			return nil, e
		}
		out = append(out, m)
	}
	return out, rows.Err()
}
func (s *Store) Get(kind string, id int64) (Object, error) {
	var b string
	e := s.DB.QueryRow("SELECT body FROM records WHERE kind=? AND id=?", kind, id).Scan(&b)
	if e != nil {
		return nil, e
	}
	var m Object
	e = json.Unmarshal([]byte(b), &m)
	return m, e
}
func (s *Store) Save(kind string, id int64, m Object) (Object, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	tx, e := s.DB.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	m = clone(m)
	if id == 0 {
		if _, e = tx.Exec(`INSERT INTO sequences(kind,value) VALUES(?,1) ON CONFLICT(kind) DO UPDATE SET value=value+1`, kind); e != nil {
			return nil, e
		}
		if e = tx.QueryRow("SELECT value FROM sequences WHERE kind=?", kind).Scan(&id); e != nil {
			return nil, e
		}
		m["createdAt"] = now()
	}
	m["id"] = id
	m["updatedAt"] = now()
	b, e := json.Marshal(m)
	if e != nil {
		return nil, e
	}
	_, e = tx.Exec("INSERT INTO records(kind,id,body) VALUES(?,?,?) ON CONFLICT(kind,id) DO UPDATE SET body=excluded.body", kind, id, string(b))
	if e != nil {
		return nil, e
	}
	return m, tx.Commit()
}
func (s *Store) Delete(kind string, id int64) error {
	_, e := s.DB.Exec("DELETE FROM records WHERE kind=? AND id=?", kind, id)
	return e
}
func (s *Store) Setting(key string, def Object) (Object, error) {
	var b string
	e := s.DB.QueryRow("SELECT value FROM settings WHERE key=?", key).Scan(&b)
	if errors.Is(e, sql.ErrNoRows) {
		return clone(def), nil
	}
	if e != nil {
		return nil, e
	}
	var m Object
	e = json.Unmarshal([]byte(b), &m)
	return merge(def, m), e
}
func (s *Store) Set(key string, m Object) error {
	b, e := json.Marshal(m)
	if e != nil {
		return e
	}
	_, e = s.DB.Exec("INSERT INTO settings(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, string(b))
	return e
}
func (s *Store) Backup(dest string) error {
	if _, e := os.Stat(dest); e == nil {
		return errors.New("备份文件已存在")
	}
	_, e := s.DB.Exec("VACUUM INTO ?", dest)
	return e
}
func require(m Object, keys ...string) error {
	for _, k := range keys {
		if str(m, k) == "" {
			return fmt.Errorf("%s 不能为空", k)
		}
	}
	return nil
}
