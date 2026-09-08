package app

import (
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
	home, e := os.UserHomeDir()
	if e != nil {
		return Config{}, e
	}
	dir := filepath.Join(home, ".local", "share", "ostrm")
	if runtime.GOOS == "windows" {
		dir = filepath.Join(os.Getenv("LOCALAPPDATA"), "OStrm")
	} else if runtime.GOOS == "darwin" {
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
func DefaultSettings() Object {
	return Object{
		"videoExtensions": []string{"mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "ts", "m2ts", "iso", "rmvb", "rm", "mpg", "mpeg", "3gp", "vob"},
		"tmdb":            Object{"apiKey": "", "baseUrl": "https://api.themoviedb.org", "imageBaseUrl": "https://image.tmdb.org", "language": "zh-CN", "region": "CN", "timeout": 30, "retryCount": 3, "posterSize": "w500", "backdropSize": "w1280"},
		"scraping":        Object{"enabled": true, "nfoFormat": "kodi", "keepSubtitleFiles": false, "useExistingScrapingInfo": false},
		"ai":              Object{"enabled": false, "baseUrl": "https://api.openai.com/v1", "apiKey": "", "model": "", "qpmLimit": 60},
		"notifications":   Object{"enabled": false, "notifyOnSuccess": true, "notifyOnPartialSuccess": true, "notifyOnFailure": true, "includeFullPath": true, "maxDetailItems": 5, "serverUrl": "", "configKey": "ostrm", "tags": "all"},
		"scrapingRegex":   Object{}, "log": Object{"retentionDays": 7, "level": "info", "reportUsageData": false}, "timezone": "Local"}
}
