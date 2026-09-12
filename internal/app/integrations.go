package app

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"
)

func (a *App) mediaRequest(ctx context.Context, c Object, method, endpoint string) ([]byte, error) {
	b, _, e := a.request(ctx, method, strings.TrimRight(str(c, "apiBaseUrl"), "/")+endpoint, map[string]string{"X-Emby-Token": str(c, "apiKey")}, nil)
	return b, e
}
func (a *App) libraries(ctx context.Context, c Object) ([]Object, error) {
	b, e := a.mediaRequest(ctx, c, "GET", "/Library/VirtualFolders")
	if e != nil {
		return nil, e
	}
	var in []Object
	if e = json.Unmarshal(b, &in); e != nil {
		return nil, e
	}
	out := []Object{}
	for _, v := range in {
		locations := v["Locations"]
		if locations == nil {
			locations = []string{}
		}
		out = append(out, Object{"id": v["ItemId"], "name": v["Name"], "collectionType": v["CollectionType"], "locations": locations})
	}
	return out, nil
}
func (a *App) mediaTest(ctx context.Context, c Object) (Object, error) {
	b, e := a.mediaRequest(ctx, c, "GET", "/System/Info")
	if e != nil {
		return nil, e
	}
	var info Object
	if e = json.Unmarshal(b, &info); e != nil {
		return nil, e
	}
	libs, e := a.libraries(ctx, c)
	if e != nil {
		return nil, e
	}
	return Object{"serverName": info["ServerName"], "version": info["Version"], "productName": info["ProductName"], "libraryCount": len(libs)}, nil
}
func (a *App) refresh(ctx context.Context, c, m Object) (Object, error) {
	scope := strings.ToUpper(str(m, "scope"))
	if scope == "NONE" || scope == "" {
		return Object{"status": "SKIPPED", "message": "未配置刷新"}, nil
	}
	if !boolean(c, "isActive", true) {
		return nil, errors.New("媒体服务器已禁用")
	}
	endpoint := "/Library/Refresh"
	if scope == "LIBRARY" {
		if str(m, "libraryId") == "" {
			return nil, errors.New("媒体库ID不能为空")
		}
		endpoint = "/Items/" + url.PathEscape(str(m, "libraryId")) + "/Refresh?Recursive=true&MetadataRefreshMode=Default&ImageRefreshMode=Default&ReplaceAllMetadata=false&ReplaceAllImages=false"
	} else if scope != "ALL" {
		return nil, errors.New("无效刷新范围")
	}
	_, e := a.mediaRequest(ctx, c, "POST", endpoint)
	if e != nil {
		return nil, e
	}
	return Object{"status": "SUCCESS", "scope": scope, "libraryId": m["libraryId"], "message": "刷新请求已提交"}, nil
}
func (a *App) refreshAfter(ctx context.Context, t Object, changed, inc bool) (Object, error) {
	if inc && !changed {
		return Object{"status": "SKIPPED", "message": "增量无变化"}, nil
	}
	if num(t, "mediaServerConfigId") == 0 || str(t, "mediaRefreshScope") == "NONE" {
		return Object{"status": "SKIPPED"}, nil
	}
	c, e := a.Store.Get("media", num(t, "mediaServerConfigId"))
	if e != nil {
		return nil, e
	}
	return a.refresh(ctx, c, Object{"scope": t["mediaRefreshScope"], "libraryId": t["mediaLibraryId"]})
}
func (a *App) notify(ctx context.Context, c, t, r Object) error {
	if !boolean(c, "enabled", false) {
		return nil
	}
	key := "notifyOnSuccess"
	typ := "success"
	switch str(r, "status") {
	case "FAILED", "FAILURE":
		key = "notifyOnFailure"
		typ = "failure"
	case "PARTIAL_SUCCESS":
		key = "notifyOnPartialSuccess"
		typ = "warning"
	}
	if !boolean(c, key, true) {
		return nil
	}
	if !regexp.MustCompile(`^[A-Za-z0-9_-]{1,128}$`).MatchString(str(c, "configKey")) {
		return errors.New("无效 Apprise Config ID")
	}
	body := fmt.Sprintf("任务: %s\n状态: %s\n处理: %d, 失败: %d", str(t, "taskName"), str(r, "status"), num(r, "processed"), num(r, "failed"))
	if boolean(c, "includeFullPath", true) {
		body += "\n源目录: " + str(t, "path") + "\n输出: " + str(t, "strmPath")
	}
	issues, _ := r["issues"].([]any)
	count := int(num(c, "maxDetailItems"))
	if count < 1 {
		count = 5
	}
	for i, x := range issues {
		if i >= min(count, 20) {
			break
		}
		m, _ := x.(map[string]any)
		body += "\n" + str(m, "reason")
		if boolean(c, "includeFullPath", true) {
			body += " " + str(m, "sourcePath")
		}
	}
	b, _ := json.Marshal(Object{"title": "OStrm · " + str(t, "taskName"), "body": body, "type": typ, "format": "text", "tag": str(c, "tags")})
	_, status, e := a.request(ctx, "POST", strings.TrimRight(str(c, "serverUrl"), "/")+"/notify/"+str(c, "configKey")+"/", map[string]string{"Content-Type": "application/json"}, bytes.NewReader(b))
	if status == 204 {
		return errors.New("Apprise 未接受通知")
	}
	return e
}
func versionGreater(a, b string) bool {
	ap := strings.Split(strings.SplitN(a, "-", 2)[0], ".")
	bp := strings.Split(strings.SplitN(b, "-", 2)[0], ".")
	for i := 0; i < 3; i++ {
		if len(ap) <= i || len(bp) <= i {
			return false
		}
		x, _ := strconv.Atoi(ap[i])
		y, _ := strconv.Atoi(bp[i])
		if x != y {
			return x > y
		}
	}
	return !strings.Contains(a, "-") && strings.Contains(b, "-")
}

var _ = time.Second
