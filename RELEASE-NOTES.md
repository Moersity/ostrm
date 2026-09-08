## Go 原生版本

- 使用 Go 1.27.1，Nuxt 网页嵌入二进制；运行不需要 Docker、Java、Node 或 Caddy。
- 支持多 OpenList、签名 URL、全量/增量 STRM、Cron、进度与取消。
- 支持 TMDB/可选 AI、NFO/字幕/图片、目录检查、手动整理及显式自动整理。
- 支持 Emby/Jellyfin 刷新、Apprise、日志、旧数据导入、备份与后台服务。
- 提供 Windows、macOS、Linux 的 amd64/arm64 安装包与便携包。

## 安装

普通 Windows 电脑选择 windows_amd64_setup.exe；Apple Silicon Mac 选择 darwin_arm64.pkg；Intel Mac 选择 darwin_amd64.pkg；Linux 选择对应 CPU 的 DEB/RPM 或 tar.gz。

安装后启动 OStrm，访问 http://127.0.0.1:3111 注册管理员并添加 OpenList。

## 迁移与注意事项

先停止旧版，以独立的新数据目录执行 migrate-legacy --dry-run，核对后导入。导入任务默认停用，原目录不修改。详细命令参见 README.md。

安装包未签名，macOS 未公证。外部集成使用 mock 验证，未穷举全部真实服务和历史数据库版本；请先用测试媒体目录核对行为。

六种原生架构均在 Actions 中验证构建、安装升级、增量运行、数据保留和后台服务。校验值见 checksums.txt，构建验证记录见 release-manifest.json。
