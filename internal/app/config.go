package app

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"runtime"
)

type Config struct {
	Listen      string `yaml:"listen"`
	DataDir     string `yaml:"dataDir"`
	StrmRoot    string `yaml:"strmRoot"`
	OpenBrowser bool   `yaml:"openBrowser"`
}

func LoadConfig(file string) (Config, error) {
	home, _ := os.UserHomeDir()
	dir := ""
	if home != "" {
		dir = filepath.Join(home, ".local", "share", "ostrm")
	}
	if runtime.GOOS == "windows" {
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "OStrm")
	} else if runtime.GOOS == "darwin" && home != "" {
		dir = filepath.Join(home, "Library", "Application Support", "OStrm")
	} else if x := os.Getenv("XDG_DATA_HOME"); x != "" {
		dir = filepath.Join(x, "ostrm")
	}
	c := Config{Listen: "127.0.0.1:3111", DataDir: dir}
	if file != "" {
		b, e := os.ReadFile(file)
		if e != nil {
			return c, e
		}
		if e = yaml.Unmarshal(b, &c); e != nil {
			return c, e
		}
	}
	for k, p := range map[string]*string{"OSTRM_LISTEN": &c.Listen, "OSTRM_DATA_DIR": &c.DataDir, "OSTRM_STRM_ROOT": &c.StrmRoot} {
		if v := os.Getenv(k); v != "" {
			*p = v
		}
	}
	return c, nil
}
func (c *Config) Resolve() error {
	if c.DataDir == "" {
		return fmt.Errorf("无法确定用户数据目录，请通过 --data-dir 指定")
	}
	var e error
	c.DataDir, e = filepath.Abs(c.DataDir)
	if e != nil {
		return e
	}
	if c.StrmRoot == "" {
		c.StrmRoot = filepath.Join(c.DataDir, "strm")
	}
	c.StrmRoot, e = filepath.Abs(c.StrmRoot)
	if e != nil {
		return e
	}
	for _, p := range []string{c.DataDir, c.StrmRoot, filepath.Join(c.DataDir, "logs"), filepath.Join(c.DataDir, "trash")} {
		if e = os.MkdirAll(p, 0700); e != nil {
			return fmt.Errorf("创建目录 %s: %w", p, e)
		}
	}
	return nil
}

//go:embed defaults.json
var defaultsJSON []byte

func DefaultSettings() Object {
	var m Object
	if err := json.Unmarshal(defaultsJSON, &m); err != nil {
		panic(err)
	}
	obj(m, "log")["reportUsageData"] = false
	m["timezone"] = "Local"
	return m
}
