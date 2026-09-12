# 开发与发布

需要 `.go-version` 指定的 Go，以及 Node 24.19.0。依赖分别由根目录 `go.mod` / `go.sum` 和 `frontend/package.json` / `frontend/package-lock.json` 管理；根目录不需要执行 npm install。

## 完整构建

在仓库根目录执行：

```sh
node build/frontend.mjs
go vet ./...
go test -race ./...
node build/build.mjs
```

前端脚本执行 npm ci 与静态生成，然后复制页面到 `internal/web/dist`；Go 使用 go:embed 编入二进制。首次运行 Go 测试前必须构建完整前端。Windows 使用 `go test ./...`；竞态测试在 Linux/macOS 执行。

`node build/build.mjs darwin/arm64` 可只构建一个目标。支持 linux/darwin/windows 的 amd64/arm64；默认版本为 3.0.0-dev，可通过 `OSTRM_VERSION` 指定。编译结果在 dist 中，安装包由 `node build/package.mjs <os> <arch>` 创建；打包工具要求与各平台 CI 保持一致。

## 本地调试

先完成一次前端构建，再启动 Go：

```sh
go run ./cmd/ostrm serve --listen 127.0.0.1:3111 --data-dir ./tmp/dev-data
```

另一个终端启动网页热更新：

```sh
npm --prefix frontend run dev
```

Nuxt 开发代理将 `/api` 转到 Go 的 `127.0.0.1:3111`。生产只运行 Go，无需单独启动 Node 或反向代理。

## 验证

```sh
npm --prefix frontend run typecheck
node tests/packaging/smoke.mjs dist/ostrm_3.0.0-dev_darwin_arm64/ostrm
```

浏览器测试在 frontend 中执行 `npx playwright test`，使用 `OSTRM_BINARY` 指定已构建二进制的绝对路径；先通过 `npx playwright install chromium` 准备浏览器。CI 的 Linux amd64 job 执行此检查。

Go CI 在六种原生目标上运行测试、打包和安装生命周期检查。应用逻辑变更应增加相应回归测试。删除文件后需确认前端生成、Go 嵌入和打包脚本无悬空引用。

## 发布分支

开发分支直接通过 PR 合并到 main，不再经过 beta。合并后 release.yml 从已发布的正式版本自动计算 SemVer：默认 patch，feat 为 minor，带 ! 或 BREAKING CHANGE 为 major。semver 标签只能提高级别，不能降级；脚本综合上次正式版之后的提交和本次 PR 标题。重跑复用版本，上传失败保留草稿；重跑发布 job 会校验已上传附件的 SHA-256，只补传缺失附件。不要修改已发布标签或强制覆盖附件。

无需维护 app_version.json。版本只在成功的正式发布账本中前进，并注入同一次构建的后端、网页和安装包。自动化不会识别任意自然语言的新功能，PR 标题必须使用正确的语义前缀。规则见 AGENTS.md；可用 `python3 -m unittest discover -s tests/release` 验证。开发构建不等同正式版本。

默认安装包未签名，macOS 未公证。配置签名和公证前不要声称已完成。

## 页面更新提示与安全升级

登录后每小时检查 `Moersity/ostrm` 的最新正式 Release，服务端缓存一小时。
“跳过此版本”只忽略当前浏览器中的该版本，后续新版本会再次提醒。

Linux/macOS 的可写程序目录支持“直接升级”：服务端重新确认版本，下载对应系统和架构的压缩包，
对照该 Release 的 `checksums.txt` 验证 SHA-256，再执行新版程序的 `version` 检查。
任务运行中不能升级；升级期间暂停新的任务和修改请求。数据库使用 SQLite `VACUUM INTO` 备份到
数据目录中的 `upgrade-backup-*`，旧二进制保留为原程序路径旁的 `.backup-upgrade-backup-*` 文件。
最终只原子替换二进制，重启沿用已解析的数据目录、STRM 目录和监听地址，不删除数据、配置、日志或输出文件。
备份不会自动删除。下载、校验、备份或替换失败均保留原程序；操作系统拒绝启动新程序时尝试恢复旧程序。
如果新程序已启动但后续启动初始化失败，请停止服务、恢复旁边保留的旧二进制，再使用原数据目录启动；
数据库备份应先复制到另一个目录检查，不能直接覆盖仍在使用的数据库及 WAL 文件。

Windows、无程序目录写权限（如以 `ostrm` 用户运行的 deb/rpm 系统服务）的部署会显示安装包入口和原因，
需要使用新版安装包覆盖安装并保留原数据目录，不会尝试提权或删除数据。SHA-256 校验依赖 fork 的
GitHub Release 与 HTTPS，不代表独立的代码签名。
