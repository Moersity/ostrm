package app

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"time"
)

const releaseAPI = "https://api.github.com/repos/Moersity/ostrm/releases/latest"
const repositoryURL = "https://github.com/Moersity/ostrm"

var stableVersion = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

type releaseAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type releaseInfo struct {
	Tag        string         `json:"tag_name"`
	Body       string         `json:"body"`
	Draft      bool           `json:"draft"`
	Prerelease bool           `json:"prerelease"`
	Assets     []releaseAsset `json:"assets"`
}

func (a *App) latestRelease(ctx context.Context, fresh bool) (releaseInfo, error) {
	a.versionMu.Lock()
	defer a.versionMu.Unlock()
	if !fresh && time.Now().Before(a.releaseUntil) {
		return a.release, nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseAPI, nil)
	if err != nil {
		return releaseInfo{}, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "OStrm")
	resp, err := a.Client.Do(req)
	if err != nil {
		return releaseInfo{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return releaseInfo{}, fmt.Errorf("检查更新失败：GitHub HTTP %d", resp.StatusCode)
	}
	var r releaseInfo
	if err = json.NewDecoder(io.LimitReader(resp.Body, 2<<20)).Decode(&r); err != nil {
		return r, err
	}
	if r.Draft || r.Prerelease || !stableVersion.MatchString(r.Tag) {
		return r, errors.New("未找到有效正式版本")
	}
	a.release, a.releaseUntil = r, time.Now().Add(time.Hour)
	return r, nil
}

func updateAssetName(version string) string {
	return fmt.Sprintf("ostrm_%s_%s_%s.tar.gz", version, runtime.GOOS, runtime.GOARCH)
}
func (r releaseInfo) asset(name string) (string, error) {
	for _, asset := range r.Assets {
		if asset.Name != name {
			continue
		}
		u, err := url.Parse(asset.URL)
		prefix := "/Moersity/ostrm/releases/download/" + r.Tag + "/"
		if err != nil || u.Scheme != "https" || u.Host != "github.com" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != prefix+name {
			return "", errors.New("发布附件地址无效")
		}
		return asset.URL, nil
	}
	return "", fmt.Errorf("发布中缺少 %s", name)
}

func (a *App) updateUnavailable() string {
	if runtime.GOOS == "windows" {
		return "Windows 请下载新版安装包覆盖安装，保留原数据目录"
	}
	if a.Restart == nil {
		return "当前启动方式不支持在线重启，请使用新版安装包覆盖安装"
	}
	exe, err := os.Executable()
	if err != nil {
		return err.Error()
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err.Error()
	}
	f, err := os.CreateTemp(filepath.Dir(exe), ".ostrm-write-check-*")
	if err != nil {
		return "程序目录不可写，请使用安装包覆盖安装，保留原数据目录"
	}
	f.Close()
	os.Remove(f.Name())
	return ""
}
func (a *App) checkVersion(ctx context.Context, _ bool) (Object, error) {
	r, err := a.latestRelease(ctx, false)
	if err != nil {
		return nil, err
	}
	latest := strings.TrimPrefix(r.Tag, "v")
	reason := a.updateUnavailable()
	if reason == "" {
		for _, name := range []string{updateAssetName(latest), "checksums.txt"} {
			if _, err := r.asset(name); err != nil {
				reason = err.Error()
				break
			}
		}
	}
	return Object{"currentVersion": Version, "latestVersion": latest, "hasUpdate": versionGreater(latest, Version), "releaseUrl": repositoryURL + "/releases/tag/" + r.Tag, "releaseNotes": r.Body, "canUpgrade": reason == "", "upgradeReason": reason}, nil
}
func (a *App) updateStatus() Object { a.mu.Lock(); defer a.mu.Unlock(); return clone(a.upgradeStatus) }
func (a *App) setUpdateStatus(stage, message string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.upgradeStatus == nil {
		a.upgradeStatus = Object{}
	}
	a.upgradeStatus["stage"] = stage
	a.upgradeStatus["message"] = message
}
func (a *App) startUpdate(version string) (Object, error) {
	if !stableVersion.MatchString(version) || !versionGreater(version, Version) {
		return nil, errors.New("请选择更新的正式版本")
	}
	if reason := a.updateUnavailable(); reason != "" {
		return nil, errors.New(reason)
	}
	a.mu.Lock()
	if a.updating || len(a.active) != 0 {
		a.mu.Unlock()
		return nil, errors.New("正在升级或执行任务，请等待完成后再升级")
	}
	a.updating = true
	a.upgradeStatus = Object{"stage": "downloading", "message": "正在下载并校验升级文件", "version": version}
	a.wg.Add(1)
	a.mu.Unlock()
	go func() {
		defer a.wg.Done()
		ctx, cancel := context.WithTimeout(a.ctx, 10*time.Minute)
		defer cancel()
		err := a.installUpdate(ctx, version)
		if err != nil {
			a.setUpdateStatus("failed", err.Error())
			a.mu.Lock()
			a.updating = false
			a.mu.Unlock()
			return
		}
		a.setUpdateStatus("restarting", "已备份并替换程序，正在重启")
		a.Restart()
	}()
	return a.updateStatus(), nil
}

func (a *App) downloadUpdate(ctx context.Context, target string, out io.Writer, limit int64) error {
	req, err := http.NewRequestWithContext(ctx, "GET", target, nil)
	if err != nil {
		return err
	}
	// Separate long download timeout from normal API requests; keep the configured transport.
	client := &http.Client{Transport: a.Client.Transport, Timeout: 8 * time.Minute, CheckRedirect: func(r *http.Request, via []*http.Request) error {
		if len(via) > 5 || r.URL.Scheme != "https" {
			return errors.New("下载重定向无效")
		}
		return nil
	}}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("下载失败：HTTP %d", resp.StatusCode)
	}
	n, err := io.Copy(out, io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return err
	}
	if n > limit {
		return errors.New("升级文件超出大小限制")
	}
	return nil
}
func expectedChecksum(text, name string) (string, error) {
	found := ""
	for _, line := range strings.Split(text, "\n") {
		p := strings.Fields(line)
		if len(p) == 2 && strings.TrimPrefix(p[1], "*") == name {
			if found != "" {
				return "", errors.New("重复校验记录")
			}
			b, e := hex.DecodeString(p[0])
			if e != nil || len(b) != 32 {
				return "", errors.New("校验记录无效")
			}
			found = strings.ToLower(p[0])
		}
	}
	if found == "" {
		return "", errors.New("缺少升级文件校验记录")
	}
	return found, nil
}
func extractUpdate(archive, output, version string) error {
	f, err := os.Open(archive)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(io.LimitReader(gz, 512<<20))
	expected := strings.TrimSuffix(updateAssetName(version), ".tar.gz") + "/ostrm"
	found := false
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if h.Name != expected {
			continue
		}
		if found || h.Typeflag != tar.TypeReg || h.Size <= 0 || h.Size > 256<<20 {
			return errors.New("程序附件无效")
		}
		found = true
		dst, err := os.OpenFile(output, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0700)
		if err != nil {
			return err
		}
		_, err = io.Copy(dst, tr)
		if err == nil {
			err = dst.Sync()
		}
		closeErr := dst.Close()
		if err != nil {
			return err
		}
		if closeErr != nil {
			return closeErr
		}
	}
	if !found {
		return errors.New("压缩包缺少匹配平台的程序")
	}
	return nil
}
func (a *App) installUpdate(ctx context.Context, version string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	exe, err = filepath.EvalSymlinks(exe)
	if err != nil {
		return err
	}
	return a.installUpdateAt(ctx, version, exe)
}

func (a *App) installUpdateAt(ctx context.Context, version, exe string) error {
	r, err := a.latestRelease(ctx, true)
	if err != nil {
		return err
	}
	if strings.TrimPrefix(r.Tag, "v") != version {
		return errors.New("最新版本已变化，请重新检查更新")
	}
	name := updateAssetName(version)
	archiveURL, err := r.asset(name)
	if err != nil {
		return err
	}
	sumsURL, err := r.asset("checksums.txt")
	if err != nil {
		return err
	}
	var sums strings.Builder
	if err = a.downloadUpdate(ctx, sumsURL, &sums, 1<<20); err != nil {
		return err
	}
	checksum, err := expectedChecksum(sums.String(), name)
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(exe), ".ostrm-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	archive := filepath.Join(stage, "release.tar.gz")
	f, err := os.OpenFile(archive, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	hash := sha256.New()
	err = a.downloadUpdate(ctx, archiveURL, io.MultiWriter(f, hash), 256<<20)
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if hex.EncodeToString(hash.Sum(nil)) != checksum {
		return errors.New("升级文件 SHA-256 校验失败，已取消升级")
	}
	candidate := filepath.Join(stage, "ostrm")
	if err = extractUpdate(archive, candidate, version); err != nil {
		return err
	}
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	output, err := exec.CommandContext(probeCtx, candidate, "version").Output()
	if err != nil {
		return fmt.Errorf("新版程序运行检查失败：%w", err)
	}
	var info struct{ Version, OS, Arch string }
	if json.Unmarshal(output, &info) != nil || info.Version != version || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		return errors.New("新版程序版本或平台不匹配")
	}
	a.setUpdateStatus("backup", "正在备份数据库和旧程序")
	backup, err := os.MkdirTemp(filepath.Join(a.Config.DataDir), "upgrade-backup-*")
	if err != nil {
		return err
	}
	if err = a.Store.Backup(filepath.Join(backup, "ostrm.db")); err != nil {
		return fmt.Errorf("数据库备份失败，升级已取消：%w", err)
	}
	old := exe + ".backup-" + filepath.Base(backup)
	if err = os.Link(exe, old); err != nil {
		return fmt.Errorf("旧程序备份失败，升级已取消：%w", err)
	}
	a.mu.Lock()
	a.upgradeStatus["backupPath"] = backup
	a.upgradeStatus["binaryBackupPath"] = old
	a.mu.Unlock()
	if err = ctx.Err(); err != nil {
		return err
	}
	infoMode, err := os.Stat(exe)
	if err != nil {
		return err
	}
	if err = os.Chmod(candidate, infoMode.Mode().Perm()); err != nil {
		return err
	}
	if err = os.Rename(candidate, exe); err != nil {
		return fmt.Errorf("替换程序失败，原程序仍保留：%w", err)
	}
	return nil
}

// PreviousExecutable is retained after a successful replacement for restart failure recovery.
func (a *App) PreviousExecutable() string {
	a.mu.Lock()
	defer a.mu.Unlock()
	return str(a.upgradeStatus, "binaryBackupPath")
}
