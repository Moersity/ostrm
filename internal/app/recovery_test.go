package app

import (
	"bytes"
	"crypto/md5"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestSeasonCompatibility(t *testing.T) {
	for _, c := range []struct {
		name string
		n    int
		ok   bool
	}{{"The Boys Season 4 Remux", 4, true}, {"The.Boys.S04.2160p", 4, true}, {"The Boys Specials", 0, true}, {"黑袍纠察队 第四季1080p", 4, true}, {"第十二季", 12, true}, {"S04 Season 4 第四季", 4, true}, {"S03 第四季", 0, false}, {"四季酒店", 0, false}, {"S04E01", 0, false}} {
		n, ok := parseSeason(c.name)
		if n != c.n || ok != c.ok {
			if ok || c.ok {
				t.Errorf("%q got %d,%t want %d,%t", c.name, n, ok, c.n, c.ok)
			}
		}
	}
	s, e := mediaNumbers("/show/第十二季/E03.mkv")
	if s != 12 || e != 3 {
		t.Fatal(s, e)
	}
	if structureReason("show/特别篇/E01.mkv", "tv") != "" {
		t.Fatal("specials rejected")
	}
	if m := tmdbIDRE.FindStringSubmatch("Movie {tmdbid-123}"); m == nil || m[1] != "123" {
		t.Fatal(m)
	}
}
func TestOutputJournalAndUserEdits(t *testing.T) {
	a := testApp(t)
	root := t.TempDir()
	if e := os.WriteFile(filepath.Join(root, "user.nfo"), []byte("mine"), 0600); e != nil {
		t.Fatal(e)
	}
	if _, e := a.ownedWrite(1, root, "user.nfo", "/x", []byte("replace")); e == nil {
		t.Fatal("overwrote unowned file")
	}
	if _, e := a.ownedWrite(1, root, "movie.strm", "/m", []byte("one")); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(root, "movie.strm"), []byte("edited"), 0600)
	if _, e := a.ownedWrite(1, root, "movie.strm", "/m", []byte("two")); e == nil {
		t.Fatal("overwrote user edit")
	}
	// Crash after replacement but before SQL ownership commit.
	b := []byte("written before crash")
	os.WriteFile(filepath.Join(root, "pending.strm"), b, 0600)
	_, e := a.Store.DB.Exec("INSERT INTO output_journal(task_id,path,root,source,hash) VALUES(1,'pending.strm',?,'/pending',?)", root, hashBytes(b))
	if e != nil {
		t.Fatal(e)
	}
	if e = a.recoverOutputs(); e != nil {
		t.Fatal(e)
	}
	owned, e := a.Store.Owned(1)
	if e != nil || str(obj(owned, "pending.strm"), "hash") != hashBytes(b) {
		t.Fatal(owned, e)
	}
	if _, e = a.ownedWrite(1, root, "pending.strm", "/pending", []byte("next")); e != nil {
		t.Fatal(e)
	}
}
func TestLegacyMigrationIdempotentAndMD5Upgrade(t *testing.T) {
	source := t.TempDir()
	os.MkdirAll(filepath.Join(source, "db"), 0700)
	os.MkdirAll(filepath.Join(source, "config"), 0700)
	db, e := sql.Open("sqlite", filepath.Join(source, "db", "openlist2strm.db"))
	if e != nil {
		t.Fatal(e)
	}
	_, e = db.Exec(`CREATE TABLE openlist_config(id INTEGER,username TEXT,base_url TEXT,token TEXT,is_active INTEGER);INSERT INTO openlist_config VALUES(1,'legacy','http://localhost:1234','test',1);CREATE TABLE task_config(id INTEGER,task_name TEXT,openlist_config_id INTEGER,path TEXT,is_active INTEGER,cron TEXT);INSERT INTO task_config VALUES(1,'task',1,'/media',1,'0 0 * * * ?');`)
	if e != nil {
		t.Fatal(e)
	}
	db.Close()
	password := "legacy-secret"
	user := fmt.Sprintf(`{"username":"legacy","pwd":"%x"}`, md5.Sum([]byte(password)))
	os.WriteFile(filepath.Join(source, "config", "userInfo.json"), []byte(user), 0600)
	c := Config{DataDir: t.TempDir(), Listen: "127.0.0.1:0"}
	var out bytes.Buffer
	if e = MigrateLegacy(source, c, true, &out); e != nil {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(c.DataDir, "ostrm.db")); !os.IsNotExist(e) {
		t.Fatal("dry run created database")
	}
	if e = MigrateLegacy(source, c, false, &out); e != nil {
		t.Fatal(e)
	}
	if e = MigrateLegacy(source, c, false, &out); e != nil {
		t.Fatal("repeat", e)
	}
	a, e := New(c)
	if e != nil {
		t.Fatal(e)
	}
	defer a.Close()
	tasks, e := a.Store.List("tasks")
	if e != nil || len(tasks) != 1 || boolean(tasks[0], "isActive", true) {
		t.Fatal(tasks, e)
	}
	if e = a.password("legacy", password); e != nil {
		t.Fatal(e)
	}
	var hash string
	a.Store.DB.QueryRow("SELECT password FROM admin").Scan(&hash)
	if len(hash) < 50 {
		t.Fatal("MD5 not upgraded")
	}
	b, _ := os.ReadFile(filepath.Join(source, "config", "userInfo.json"))
	if string(b) != user {
		t.Fatal("source modified")
	}
}
