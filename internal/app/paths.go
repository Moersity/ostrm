package app

import (
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/dlclark/regexp2"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

func encodePart(s string, isPath bool) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == '%' && i+2 < len(s) {
			if _, e := hex.DecodeString(s[i+1 : i+3]); e == nil {
				b.WriteString(s[i : i+3])
				i += 2
				continue
			}
		}
		ok := c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("-._~", rune(c))
		if isPath && strings.ContainsRune("/:@!$&'()*+,;=", rune(c)) {
			ok = true
		}
		if ok {
			b.WriteByte(c)
		} else {
			fmt.Fprintf(&b, "%%%02X", c)
		}
	}
	return b.String()
}
func smartURL(s string) string {
	parts := strings.SplitN(s, "?", 2)
	base := parts[0]
	if i := strings.Index(base, "://"); i >= 0 {
		j := strings.Index(base[i+3:], "/")
		if j >= 0 {
			j += i + 3
			base = base[:j] + encodePart(base[j:], true)
		}
	} else {
		base = encodePart(base, true)
	}
	if len(parts) == 2 {
		q := []string{}
		for _, p := range strings.Split(parts[1], "&") {
			if p == "" {
				continue
			}
			kv := strings.SplitN(p, "=", 2)
			s := encodePart(kv[0], false)
			if len(kv) == 2 {
				s += "=" + encodePart(kv[1], false)
			}
			q = append(q, s)
		}
		base += "?" + strings.Join(q, "&")
	}
	return base
}
func remoteRelative(root, p string) (string, error) {
	root = strings.TrimRight(root, "/")
	if !strings.HasPrefix(p, root+"/") {
		return "", errors.New("路径不在任务目录内")
	}
	r := strings.TrimPrefix(p, root+"/")
	if r == "" || strings.Contains(r, "\x00") {
		return "", errors.New("路径无效")
	}
	for _, c := range strings.Split(r, "/") {
		if c == ".." || c == "." || c == "" {
			return "", errors.New("路径越界")
		}
	}
	return r, nil
}
func safeName(s string) string {
	orig := s
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) || strings.ContainsRune(`<>:"/\|?*`, r) {
			return '_'
		}
		return r
	}, s)
	s = strings.TrimRight(s, " .")
	base := strings.ToUpper(strings.SplitN(s, ".", 2)[0])
	if base == "CON" || base == "PRN" || base == "AUX" || base == "NUL" || (len(base) == 4 && (strings.HasPrefix(base, "COM") || strings.HasPrefix(base, "LPT")) && base[3] >= '1' && base[3] <= '9') {
		s = "_" + s
	}
	r := []rune(s)
	if len(r) > 100 {
		s = string(r[:100])
	}
	if s == "" {
		s = "_"
	}
	if s != orig {
		s += "_" + hashBytes([]byte(orig))[:8]
	}
	return s
}
func renameFile(name, rule string) (string, error) {
	if rule != "" {
		parts := strings.SplitN(rule, "|", 2)
		if len(parts) != 2 {
			return "", errors.New("重命名规则格式必须为 模式|替换")
		}
		r, e := regexp2.Compile(parts[0], regexp2.None)
		if e != nil {
			return "", e
		}
		r.MatchTimeout = 100 * time.Millisecond
		name, e = r.Replace(name, parts[1], -1, -1)
		if e != nil {
			return "", e
		}
	}
	return strings.TrimSuffix(name, path.Ext(name)) + ".strm", nil
}
func outputPath(root, p, rule string) (string, error) {
	rel, e := remoteRelative(root, p)
	if e != nil {
		return "", e
	}
	bits := strings.Split(rel, "/")
	bits[len(bits)-1], e = renameFile(bits[len(bits)-1], rule)
	if e != nil {
		return "", e
	}
	for i := range bits {
		bits[i] = safeName(bits[i])
	}
	return filepath.Join(bits...), nil
}
func writeOutput(root, rel string, b []byte) (bool, error) {
	if !filepath.IsLocal(rel) {
		return false, errors.New("输出路径越界")
	}
	r, e := os.OpenRoot(root)
	if e != nil {
		return false, e
	}
	defer r.Close()
	if e = r.MkdirAll(filepath.Dir(rel), 0755); e != nil {
		return false, e
	}
	if old, e := r.ReadFile(rel); e == nil && string(old) == string(b) {
		return false, nil
	}
	tmp := rel + ".tmp-" + randomID()[:8]
	f, e := r.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
	if e != nil {
		return false, e
	}
	defer r.Remove(tmp)
	if _, e = f.Write(b); e != nil {
		f.Close()
		return false, e
	}
	if e = f.Sync(); e != nil {
		f.Close()
		return false, e
	}
	if e = f.Close(); e != nil {
		return false, e
	}
	if e = r.Rename(tmp, rel); e != nil {
		return false, e
	}
	return true, nil
}
func strmURL(c Object, f remoteFile) string {
	base := strings.TrimRight(str(c, "strmBaseUrl"), "/")
	if base == "" {
		base = strings.TrimRight(str(c, "baseUrl"), "/")
	}
	u := base + "/d" + f.Path
	if f.Sign != "" {
		u += "?sign=" + f.Sign
	}
	if boolean(c, "enableUrlEncoding", true) {
		return smartURL(u)
	}
	return u
}
func video(name string, settings Object) bool {
	ext := strings.ToLower(strings.TrimPrefix(path.Ext(name), "."))
	xs, _ := settings["mediaExtensions"].([]any)
	if len(xs) == 0 {
		xs = []any{"mp4", "mkv", "avi", "mov", "wmv", "flv", "webm", "m4v", "ts", "m2ts", "iso", "rmvb", "rm", "mpg", "mpeg", "3gp", "vob"}
	}
	for _, x := range xs {
		if strings.EqualFold(strings.TrimPrefix(fmt.Sprint(x), "."), ext) {
			return true
		}
	}
	return false
}
