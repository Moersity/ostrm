package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type remoteFile struct {
	Name     string `json:"name"`
	Path     string `json:"path"`
	IsDir    bool   `json:"is_dir"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Sign     string `json:"sign"`
	URL      string `json:"raw_url"`
}
type limiter struct {
	mu   sync.Mutex
	hits []time.Time
}

func (l *limiter) wait(ctx context.Context, qps, qpm int64) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		l.mu.Lock()
		n := time.Now()
		start := 0
		for start < len(l.hits) && n.Sub(l.hits[start]) >= time.Minute {
			start++
		}
		l.hits = l.hits[start:]
		wait := time.Duration(0)
		if qpm > 0 && len(l.hits) >= int(qpm) {
			wait = time.Minute - n.Sub(l.hits[len(l.hits)-int(qpm)])
		}
		if qps > 0 && len(l.hits) >= int(qps) {
			w := time.Second - n.Sub(l.hits[len(l.hits)-int(qps)])
			if w > wait {
				wait = w
			}
		}
		if wait <= 0 {
			if qps > 0 || qpm > 0 {
				l.hits = append(l.hits, n)
			}
			l.mu.Unlock()
			return nil
		}
		l.mu.Unlock()
		t := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			t.Stop()
			return ctx.Err()
		case <-t.C:
		}
	}
}
func (a *App) limit(c Object) *limiter {
	k := fmt.Sprint(c["id"]) + str(c, "baseUrl")
	a.mu.Lock()
	defer a.mu.Unlock()
	l := a.limits[k]
	if l == nil {
		l = &limiter{}
		a.limits[k] = l
	}
	return l
}
func validURL(s string) error {
	u, e := url.Parse(s)
	if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil {
		return errors.New("必须使用有效的 HTTP/HTTPS 地址")
	}
	return nil
}
func (a *App) request(ctx context.Context, method, target string, headers map[string]string, body io.Reader) ([]byte, int, error) {
	return a.requestClient(ctx, a.Client, method, target, headers, body)
}
func (a *App) requestClient(ctx context.Context, client *http.Client, method, target string, headers map[string]string, body io.Reader) ([]byte, int, error) {
	if e := validURL(target); e != nil {
		return nil, 0, e
	}
	req, e := http.NewRequestWithContext(ctx, method, target, body)
	if e != nil {
		return nil, 0, e
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, e := client.Do(req)
	if e != nil {
		if ctx.Err() != nil {
			return nil, 0, ctx.Err()
		}
		return nil, 0, fmt.Errorf("外部服务请求失败 (%s)", errorKind(e))
	}
	defer resp.Body.Close()
	b, e := io.ReadAll(io.LimitReader(resp.Body, 32<<20+1))
	if e != nil {
		return nil, resp.StatusCode, e
	}
	if len(b) > 32<<20 {
		return nil, resp.StatusCode, errors.New("响应超过32MiB")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		retry := time.Duration(0)
		if sec, e := strconv.Atoi(resp.Header.Get("Retry-After")); e == nil {
			retry = time.Duration(sec) * time.Second
		} else if at, e := http.ParseTime(resp.Header.Get("Retry-After")); e == nil {
			retry = time.Until(at)
		}
		return nil, resp.StatusCode, &externalError{status: resp.StatusCode, retryAfter: retry}
	}
	return b, resp.StatusCode, nil
}
func errorKind(e error) string {
	if errors.Is(e, context.Canceled) {
		return "canceled"
	}
	if errors.Is(e, context.DeadlineExceeded) {
		return "timeout"
	}
	return "network"
}
func (a *App) remote(ctx context.Context, c Object, endpoint string, payload Object) (Object, error) {
	b, e := json.Marshal(payload)
	if e != nil {
		return nil, e
	}
	readOnly := endpoint == "list" || endpoint == "get"
	tries := 1
	if readOnly {
		tries = 3
	}
	var last error
	for i := 0; i < tries; i++ {
		if e = a.limit(c).wait(ctx, num(c, "fsApiQpsLimit"), num(c, "fsApiQpmLimit")); e != nil {
			return nil, e
		}
		raw, status, e := a.request(ctx, "POST", strings.TrimRight(str(c, "baseUrl"), "/")+"/api/fs/"+endpoint, map[string]string{"Authorization": str(c, "token"), "Content-Type": "application/json"}, bytes.NewReader(b))
		if e == nil {
			var envelope struct {
				Code    int    `json:"code"`
				Message string `json:"message"`
				Data    Object `json:"data"`
			}
			if e = json.Unmarshal(raw, &envelope); e != nil {
				return nil, errors.New("OpenList 返回无效JSON")
			}
			if envelope.Code != 200 {
				return nil, fmt.Errorf("OpenList 业务错误 %d", envelope.Code)
			}
			return envelope.Data, nil
		}
		last = e
		if !readOnly || (status != 0 && status != 429 && status < 500) {
			break
		}
		delay := time.Duration(i+1) * time.Second
		var ee *externalError
		if errors.As(e, &ee) && ee.retryAfter > delay {
			delay = ee.retryAfter
		}
		if i+1 == tries {
			break
		}
		a.contextLogger(ctx).Warn("OpenList 请求失败，即将重试", "operation", endpoint, "sourcePath", str(payload, "path"), "attempt", i+1, "maxAttempts", tries, "retryInMs", delay.Milliseconds(), "error", e.Error())
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-time.After(delay):
		}
	}
	return nil, last
}
func (a *App) list(ctx context.Context, c Object, dir string) ([]remoteFile, error) {
	out := []remoteFile{}
	for page := 1; page <= 100000; page++ {
		m, e := a.remote(ctx, c, "list", Object{"path": dir, "password": "", "page": page, "per_page": 500, "refresh": false})
		if e != nil {
			return nil, e
		}
		b, _ := json.Marshal(m["content"])
		var files []remoteFile
		if e = json.Unmarshal(b, &files); e != nil {
			return nil, e
		}
		for i := range files {
			f := &files[i]
			if f.Name == "" || f.Name == "." || f.Name == ".." || strings.ContainsAny(f.Name, "/\\\x00") {
				return nil, errors.New("OpenList 返回不安全文件名")
			}
			f.Path = path.Join(dir, f.Name)
		}
		if len(out)+len(files) > scanEntryLimit {
			return nil, errors.New("目录条目超过100万")
		}
		out = append(out, files...)
		total := num(m, "total")
		if len(files) == 0 {
			if int64(len(out)) < total {
				return nil, errors.New("OpenList 分页不完整")
			}
			return out, nil
		}
		if (total > 0 && int64(len(out)) >= total) || (total == 0 && len(files) < 500) {
			return out, nil
		}
	}
	return nil, errors.New("目录分页超过限制")
}

const scanEntryLimit = 1000000

func (a *App) scan(ctx context.Context, c Object, root string) ([]remoteFile, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	all := []remoteFile{}
	dirs := []string{root}
	seen := map[string]bool{root: true}
	for len(dirs) > 0 {
		type result struct {
			files []remoteFile
			err   error
		}
		jobs := make(chan string)
		results := make(chan result, 4)
		var wg sync.WaitGroup
		for i := 0; i < min(4, len(dirs)); i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				for d := range jobs {
					if ctx.Err() != nil {
						return
					}
					logger := a.contextLogger(ctx)
					logger.Debug("正在读取目录", "stage", "DISCOVERY", "sourcePath", d)
					f, e := a.list(ctx, c, d)
					if e != nil {
						e = fmt.Errorf("读取目录 %s 失败: %w", d, e)
					}
					results <- result{f, e}
					if e != nil {
						return
					}
				}
			}()
		}
		wg.Add(1)
		go func(batch []string) {
			defer wg.Done()
			defer close(jobs)
			for _, d := range batch {
				select {
				case jobs <- d:
				case <-ctx.Done():
					return
				}
			}
		}(dirs)
		go func() { wg.Wait(); close(results) }()
		next := []string{}
		var firstErr error
		for r := range results {
			// Drain workers before returning so canceled requests cannot outlive the scan.
			if firstErr != nil {
				continue
			}
			if r.err != nil {
				firstErr = r.err
				cancel()
				continue
			}
			for _, f := range r.files {
				if len(all) >= scanEntryLimit {
					firstErr = errors.New("扫描条目超过100万")
					cancel()
					break
				}
				all = append(all, f)
				if f.IsDir && !seen[f.Path] {
					seen[f.Path] = true
					next = append(next, f.Path)
				}
			}
		}
		if firstErr != nil {
			return nil, firstErr
		}
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		dirs = next
	}
	sort.Slice(all, func(i, j int) bool { return all[i].Path < all[j].Path })
	return all, ctx.Err()
}
func (a *App) download(ctx context.Context, c Object, f remoteFile) ([]byte, error) {
	m, e := a.remote(ctx, c, "get", Object{"path": f.Path, "password": ""})
	if e != nil {
		return nil, e
	}
	target := str(m, "raw_url")
	if target == "" {
		target = str(c, "baseUrl") + "/d" + f.Path
		if sign := str(m, "sign"); sign != "" {
			target += "?sign=" + sign
		}
	}
	b, _, e := a.request(ctx, "GET", smartURL(target), nil, nil)
	return b, e
}
func (a *App) upload(ctx context.Context, c Object, p string, b []byte) error {
	if e := a.limit(c).wait(ctx, num(c, "fsApiQpsLimit"), num(c, "fsApiQpmLimit")); e != nil {
		return e
	}
	raw, _, e := a.request(ctx, "PUT", strings.TrimRight(str(c, "baseUrl"), "/")+"/api/fs/put", map[string]string{"Authorization": str(c, "token"), "File-Path": url.PathEscape(p), "As-Task": "false", "Content-Type": "application/octet-stream"}, bytes.NewReader(b))
	if e != nil {
		return e
	}
	var m Object
	if e = json.Unmarshal(raw, &m); e != nil {
		return e
	}
	if num(m, "code") != 200 {
		return errors.New("OpenList 上传失败")
	}
	return nil
}

type externalError struct {
	status     int
	retryAfter time.Duration
}

func (e *externalError) Error() string { return fmt.Sprintf("外部服务 HTTP %d", e.status) }
func (a *App) configuredClient(c Object) (*http.Client, error) {
	b, _ := json.Marshal(c)
	key := hashBytes(b)
	a.mu.Lock()
	defer a.mu.Unlock()
	if client := a.clients[key]; client != nil {
		return client, nil
	}
	client := *a.Client
	if n := num(c, "timeout"); n > 0 {
		client.Timeout = time.Duration(n) * time.Second
	}
	if host := str(c, "proxyHost"); host != "" {
		if !strings.Contains(host, "://") {
			host = "http://" + host
		}
		u, e := url.Parse(host)
		if e != nil {
			return nil, e
		}
		if port := str(c, "proxyPort"); port != "" {
			u.Host = u.Hostname() + ":" + port
		}
		if e = validURL(u.String()); e != nil {
			return nil, e
		}
		transport, ok := a.Client.Transport.(*http.Transport)
		if !ok {
			return nil, errors.New("当前HTTP传输不支持代理")
		}
		copy := transport.Clone()
		copy.Proxy = http.ProxyURL(u)
		client.Transport = copy
	}
	a.clients[key] = &client
	return &client, nil
}
