# Go 重写进度（2026-09-08）

开发分支：dev；源基线 main e25c766，实际保留 dev 9f3ae5c。实现提交 c70ab10、9613467 及后续提交。

已实现 F01–F12 的运行入口：登录与会话、OpenList 配置、任务 CRUD/调度、全量与增量 STRM、保守清理与隔离恢复、TMDB/AI/NFO/字幕、目录检查、手动整理及显式自动整理、媒体刷新、Apprise、日志、设置、版本查询。新增单文件运行、CLI、迁移、备份和原生安装流程。

证据：
- Go 1.27.1，CGO_ENABLED=0 六目标编译；本地 go test -race ./... 与 go vet ./...。
- Java dev 使用 JDK 21 启动并提取 contracts/openapi.json（62 路由）、默认设置；UrlEncoderTest、TaskManifestServiceTest、TaskDirectoryStructureValidatorTest、SeasonDirectoryNameParserTest 的原测试通过。
- Go 17 项测试覆盖鉴权、URL、路径、增量与失败保留、Cron、限流取消、备份、嵌入页面、中文季目录、远端重命名响应丢失重试、输出写入恢复、旧 SQLite 导入、通知/AI/媒体刷新及 NFO XML。
- 前端 typecheck 和 Chromium E2E 已通过；E2E 含登录、设置、任务提交到完成、手动刮削目录页与刷新。
- 第一次 CI 34194258782：四种 Windows/macOS 安装通过；Linux 打包模板变量问题已修复。
- 第二次 CI 34195203654：Linux ARM64 安装通过；Windows/macOS 输出路径别名问题已修复；最新提交重新验证。

性能：本机 Apple M4 Pro，Go 1.27.1，单次 mock 分页扫描 1 万条 22.9 ms，10 万条 189.7 ms；分配总量约 26.4 MB/264.8 MB（不是峰值 RSS，也不包含文件写入）。不据此声称全流程比 Java 快。

发布：仅 dev CI artifacts；未提升 beta/main、未发布未经验收的正式版。原生构建/安装流程由 go-ci.yml 验证，release.yml 在 dev→beta 或 beta→main 合并后调用相同流程发布。安装包默认 unsigned，未配置证书与 macOS 公证。

仍需明确的验证边界：没有用户真实 OpenList/TMDB/Emby/Apprise 生产服务凭据，外部变更仅用 mock；未完成 62 个接口全部输入组合的逐响应差分、Java/Go 全流程性能对比和所有历史库版本迁移测试。以上不应表述为 100% 行为等价。
