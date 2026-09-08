# OStrm Go 重写交接包

日期：2026-09-08。本页以下为最初设计阶段的冻结记录；Go 实现与原生安装验证已经完成，当前状态请看 [progress.md](progress.md)。

## 已完成的仓库准备

- Fork：[Moersity/ostrm](https://github.com/Moersity/ostrm)，已通过 GitHub API 确认 parent 为 `hienao/ostrm`。
- 原项目：[hienao/ostrm](https://github.com/hienao/ostrm)。
- 本次分析基线：`e25c766753c758f00aa47818057d3d9df0b4969a`（读取时的 main）。
- 本地源码：`/Users/lixiang5/Documents/Codex/2026-09-08/https-github-com-hienao-ostrm-https/work/ostrm`。
- 本地 `origin` 指向 fork；`upstream` 指向原项目。尚未修改源码、提交或推送设计文档。
- Fork 中已有 `dev`、`beta`、`main`。实现前须遵守仓库 `AGENTS.md` 的 `dev -> beta -> main` 规则。

## 阅读顺序

1. [架构与功能一致性设计](01-design.md)：目标、架构、核心算法、存储、迁移和优化边界。
2. [分阶段实现与验收](02-implementation-plan.md)：具体任务、文件归属、测试、交付标准与可复制提示词。
3. [源码接口与字段索引](03-source-contract-index.md)：62 个现有 HTTP 路由、DTO/实体字段、Java 测试与 SQL 迁移导航。
4. [机器可读路由清单](04-api-inventory.json)：用于生成接口覆盖表，不能代替完整 OpenAPI。
5. [GitHub Actions 与安装包设计](05-release-design.md)：六个平台组合、安装器、发布顺序、签名、回滚和验收。

## 交接给实现模型

将本目录路径和源码目录路径一起交给模型，使用 `02-implementation-plan.md` 末尾的提示词。先完成 P0 基线冻结与契约，再逐阶段推进。不要将本设计中的“验收要求”描述为已经通过的结果。

本设计采用 Go + 现有 Nuxt 静态前端 + `go:embed`，核心功能不依赖 Java、Node.js、Docker、Caddy。OpenList 是必需的外部服务；TMDB、AI、Apprise 和媒体服务器为可选集成。

## 未验证事项

- 没有执行旧版后端、前端或真实 OpenList 任务；源码结论属于静态分析。
- 没有在 Windows、Linux 或 macOS 上运行 Go 重写版；目前没有重写版可供运行。
- 没有签名证书、Apple 公证凭据，也未检查 fork 的 Actions 运行器权限。
- Go 官方下载接口本次返回最新稳定版 `go1.27.1`。实现开始时重新核对，再将确切版本写入构建锁定文件；发布不能浮动使用 `latest`。
