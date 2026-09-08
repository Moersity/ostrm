# OStrm Go

Go 1.27.1 + SQLite + 内嵌 Nuxt 网页。不需要 Docker、Java、Node 或 Caddy 运行时。

```sh
ostrm serve --listen 127.0.0.1:3111
```

打开 http://127.0.0.1:3111 注册管理员，添加 OpenList 配置和任务。无参数启动会打开浏览器。
默认数据存储在系统用户数据目录；`--data-dir` 与 `--strm-root` 可覆盖，`--portable` 使用程序旁的 data 目录。

构建（Node 24.19.0、Go 1.27.1）：

```sh
node build/frontend.mjs
go test -race ./...
node build/build.mjs
```

`ostrm backup --output /path/to/backup.db` 创建一致性 SQLite 备份。
旧版迁移先停止旧程序，使用 `ostrm migrate-legacy --source /old/maindata --data-dir /new/data --dry-run` 预览，再去掉 dry-run。
迁移任务默认停用；导入不认领或删除原有输出文件。

可选后台服务：以适当权限执行 `ostrm service install --data-dir <固定数据目录>`，随后 `ostrm service start`；卸载服务使用 `ostrm service uninstall`。
Windows 服务使用独立账户环境，请勿依赖交互用户的映射盘符。

外部集成包括 TMDB、兼容 Chat Completions 的 AI、Emby/Jellyfin 和 Apprise HTTP。不配置时核心 STRM 生成仍可使用。
远端重命名/上传仅由手动整理或显式自动重命名开关触发。失败的远端写入如结果不明确会停止重试，避免重复修改。

无变化增量保留 STRM mtime；仅清理本程序记录拥有且未被用户修改的输出，放入数据目录 trash。
未签名安装包可能显示操作系统安全提示；是否签名和公证以 release 说明为准。

GPL-3.0-or-later。基于 hienao/ostrm 的前端及业务行为重写，原版权和 LICENSE 保留。
开发与发布流程详见 [开发指南](docs/development.md)，内部结构见 [架构说明](docs/architecture.md)。

## 电影目录与多画质片源

电影支持根目录和任意层级子目录。默认标准化 STRM 名称并只选同片最佳画质，可在任务中预览保留／过滤结果或选择保留所有版本；不删除 OpenList 原视频。详见 [电影目录、识别与画质筛选](docs/movie-library.md)。

## 下载与目录

正式安装包见 [GitHub Releases](https://github.com/Moersity/ostrm/releases)。`dev` 的变更需经过 beta → main 发布后才进入正式版。

- `cmd/ostrm`：CLI 和服务入口。
- `internal/app`：Go API、SQLite、任务、刮削及迁移。
- `internal/web`：内嵌前端资源。
- `frontend`：Nuxt 网页源码与浏览器测试。
- `build`、`packaging`、`tests/packaging`：构建和原生安装验证。
- `.github/workflows`：Go CI 与 Release。
- `docs`：当前功能和开发说明。

原 Java 实现及历史设计可在 Git 历史中查阅；当前源码树只使用 Go 原生架构。
