# 当前架构

OStrm 使用单个 Go 进程提供 HTTP API、静态网页、任务调度和数据存储。Nuxt SPA 在构建时嵌入二进制，运行时无需 Java、Node 或容器。

## 代码边界

- `cmd/ostrm/main.go`：启动、备份、旧数据迁移、系统服务安装及管理命令。
- `internal/app/http*.go`：当前 HTTP 路由和输入校验；API 实现以这些文件为准。
- `internal/app/store.go`、`auth.go`：SQLite、会话、密码与持久化。
- `internal/app/openlist.go`：分页扫描、限流、读请求重试、远端访问。
- `internal/app/tasks.go`：Cron、全量／增量任务、进度、生成与清理。
- `internal/app/movie*.go`、`metadata.go`、`manual.go`：电影识别、画质筛选、TMDB/AI/NFO 和显式远端整理。
- `internal/app/outputs.go`：输出归属、写入日志与隔离恢复；不覆盖外部修改的文件。
- `internal/app/migrate.go`：直接读取旧 SQLite 和配置文件，不依赖旧服务或旧源码。
- `internal/web/embed.go`：嵌入网页与 SPA 路由回退。
- `frontend/app`：唯一 Nuxt 源目录；包含页面、组件、Pinia 状态和 API 客户端。`frontend/app/app.vue` 是入口。

## 数据与外部服务

默认数据目录由操作系统确定，CLI 可用 `--data-dir`、`--strm-root` 或 `--portable` 覆盖。任务记录、配置与输出归属写入 SQLite；通过 CLI backup 进行一致性备份。

OpenList 提供文件树和视频 URL；TMDB/AI、Emby/Jellyfin 与 Apprise 都是可选集成。普通 STRM 任务保留原视频；显式远端整理允许重命名、移动和上传。无法确认远端写入结果时保留作业状态，停止盲目重试。

旧版迁移使用 `migrate-legacy --dry-run` 预览，再执行导入。迁移测试在 `internal/app` 中维护，旧 Java 文件已从源码树删除，历史版本仍保存在 Git 中。

## 构建与测试

前端依赖锁定在 frontend/package-lock.json，Go 依赖锁定在 go.sum。`.github/workflows/go-ci.yml` 验证网页与六个平台；`release.yml` 在合法 PR 合并后发布经验证的原生安装包。

测试包括本地逻辑与 mock 集成、浏览器交互，以及各平台安装/升级/卸载的数据保留。mock 集成不等同全部真实服务兼容性验证。
