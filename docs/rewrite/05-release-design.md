# GitHub Actions、跨平台安装包与发布设计

## 1. 交付矩阵

正式版必须同时生成下列架构的便携包和安装包。归档不是安装程序，不能只上传 zip/tar.gz 后称已满足全部平台安装要求。OS 最低版本须按锁定 Go 工具链、依赖、CI 实测确定并写入 release；不沿用安装器自身支持的老系统范围。

| OS / arch | 便携包 | 安装程序 | 安装实现 |
|---|---|---|---|
| windows/amd64 | `ostrm_VERSION_windows_amd64.zip` | `ostrm_VERSION_windows_amd64_setup.exe` | Inno Setup |
| windows/arm64 | `ostrm_VERSION_windows_arm64.zip` | `ostrm_VERSION_windows_arm64_setup.exe` | Inno Setup ARM64 对应配置 |
| darwin/amd64 | `ostrm_VERSION_darwin_amd64.tar.gz` | `ostrm_VERSION_darwin_amd64.pkg` | pkgbuild/productbuild |
| darwin/arm64 | `ostrm_VERSION_darwin_arm64.tar.gz` | `ostrm_VERSION_darwin_arm64.pkg` | pkgbuild/productbuild |
| linux/amd64 | `ostrm_VERSION_linux_amd64.tar.gz` | `.deb`、`.rpm` | nFPM |
| linux/arm64 | `ostrm_VERSION_linux_arm64.tar.gz` | `.deb`、`.rpm` | nFPM |

附加产物：`checksums.txt`、`release-manifest.json`、`SOURCE.tar.gz`、`THIRD_PARTY_NOTICES`、依赖/SBOM 清单和 `validation-report.json`。Debian 架构名 amd64/arm64；RPM 按目标包规范映射 x86_64/aarch64。包版本不能直接使用带 `v` 的 tag：SemVer、Debian version、RPM version-release、Windows numeric version 各自生成，预发布标识不可丢失。

当前未创建 workflow 或安装器，以下是实现合同，不是可以不经验证直接运行的 YAML。

## 2. 构建模型

1. Node 版本在 P0 按 Nuxt lockfile 验证并锁定；保留 npm lockfile，执行 `npm ci`。前端只构建一次，所有平台取同一份、带 SHA-256 的 UI artifact。
2. 构建前清空 dist，复制到 `internal/web/dist`；断言 index.html 和 `_nuxt` 下非空 JS 存在，验证 embedded 资源清单。
3. 使用相同源码 SHA、Go patch 版本、依赖锁、前端 hash 构建六目标。发布 `CGO_ENABLED=0`，不能依赖构建机安装的 sqlite DLL/so/dylib。
4. Go 可交叉编译便携程序，但“能交叉编译”不等于“在目标系统可运行”。安装器按目标 OS 原生构建，测试需覆盖各架构。
5. 测试模式的 `go test -race` 在支持的原生 runner 上启用 CGO/C 编译器；发布仍 CGO=0。不要强迫 CGO=0 的 race 测试运行。
6. 使用 `-trimpath`、版本 ldflags；版本元数据包含 version、commit、构建工具链和 UI hash。不在每个任务中注入不同当前时间；需时间戳则使用该源码提交时间。
7. 构建后 `version --json` 与 release manifest 对比；所有文件签名完成后才生成最终 checksum。不要对签名之前的二进制计算最终哈希。

建议模块 `github.com/Moersity/ostrm`，产品名 OStrm Go、命令名 ostrm。Go 版本本次查到为 1.27.1，后续实现如更新必须提交工具链变更及 CI 证据。参考 [setup-go](https://github.com/actions/setup-go)。

## 3. 工作流拆分

### ci.yml

触发：push dev、pull_request 到 dev/beta/main、workflow_dispatch。默认 `contents: read`。禁止在 pull_request_target 下执行来自 fork 的未信任代码。

依赖图：

```text
branch-policy ───────────────┐
toolchain + frontend-build ──┼─> go-tests (3 OS) ─> integration + UI E2E
                            └─> build (6 targets) ─> package-smoke
```

检查项目：gofmt、go vet、go test、适用 runner 的 race、数据库迁移、mock OpenList 故障场景、Nuxt typecheck、UI E2E、六目标 CGO=0 构建。P0 记录原前端 typecheck 基线；已有错误不能无限期忽略，发布前必须清除或有具体经审查的范围豁免。

每个 matrix 行必须带 `goos`、`goarch`、`runner`、`nativeTest`。P0 使用官方 runner 表和仓库可用性验证确切标签，再提交固定标签；不能用 `macos-latest` 猜 CPU。ARM64 runner 无法使用时可以交叉构建诊断包，但正式 release 标为未通过并阻止发布，不伪称测试成功。可后续接入用户提供的自托管 runner，不能自行创建付费基础设施。

参考：[GitHub-hosted runners](https://docs.github.com/en/actions/how-tos/manage-runners/github-hosted-runners/use-github-hosted-runners)。

### release.yml

保持已有仓库 `AGENTS.md`：开发在 dev，`dev -> beta -> main`。不新增从其他分支直接发布正式版的旁路。

触发采用 `pull_request: types: [closed], branches: [beta, main]`，job 先验证：

- `merged == true`。
- base 为 beta 时 head 必须 dev；base 为 main 时 head 必须 beta。
- head repository 和 base repository 都必须是当前 fork，不能同名外部仓库。
- checkout `merge_commit_sha`，不是 head.sha，不是“此刻最新 main”。
- beta 读取该 merge SHA 的 `app_version.json.beta_version`，main 读取 release_version。
- 新系列初始版本建议 `3.0.0-beta.1`/`3.0.0`；由 P0 核对现有 tags 后设置，不复用上游已经发布的同名 tag。
- 创建 tag 前验证 tag 不存在，或已有 tag 恰指向该 merge SHA 且未发布。其他情况直接失败，不能移动 tag。

手动重跑仅接受已合规合并的确切 commit/version，并重新校验其分支/PR 关系。默认 workflow_dispatch 可生成 snapshot artifacts，不发布。

发布依赖图：

```text
validate-promotion-and-version
  -> checkout-exact-sha + frontend
  -> test-and-build-six-targets
  -> package-linux / package-macos / package-windows
  -> install-upgrade-uninstall-tests
  -> sign-when-configured
  -> final-hashes + source + validation-report
  -> create-draft-release + upload-all
  -> verify-required-assets
  -> publish-release
```

普通 build/package jobs 只读；最后 release job 才授予 `contents: write`。不需要 Docker Hub secrets 或 packages:write。若使用 OIDC provenance，单独最小化权限，不为了方便给全部 job write-all。

Concurrency 按 repository + channel + version 分组，同一版发布不可互相取消。matrix `fail-fast: false` 保留全部失败诊断，但任何必需目标失败都阻止发布。若发布到一半失败，保留 draft，重跑先比对已有产物 hash，再补齐；已公开发布的文件不可用 `--clobber` 静默替换。

预发布勾选 prerelease 且不更新 latest；正式版完成后更新 latest。检查新版本查询逻辑从 `Moersity/ostrm` 获取，并按用户选择 channel 过滤 prerelease。

### 旧工作流处理

在 dev 中将原 `.github/workflows/docker-build-push.yml` 的自动发布触发移除或改为明确的可选手动工作流；保留其历史，不让它与新发布流程争抢同一 release/tag。原流程依赖 `DOCKERHUB_USERNAME/TOKEN`，Go 主发布不应再需要它们。修改经 dev -> beta -> main 晋级，不直接编辑受保护分支。

第三方 Actions 用核实后的完整 commit SHA，注释其 tag；构建工具（nFPM/Inno 等）使用锁定版本并校验下载 checksum。不得在设计阶段写假的 SHA 或把 `latest` 当作可复现版本。

## 4. Windows 安装器

Inno Setup 为每个架构生成单独 setup.exe。可非管理员安装到用户 Programs 目录，data-dir 固定为 LOCALAPPDATA/OStrm；创建“启动 OStrm”和“打开管理页面”快捷方式。安装结束可勾选启动，卸载默认保留数据与 STRM。

便携二进制直接运行能显示管理地址；不要求安装器才能使用。首版允许 console 窗口，不用隐藏窗口掩盖启动错误。需要开机启动时由用户选择独立服务安装命令或用户启动项，不在默认安装过程中静默安装系统服务。

系统服务可选模式需 Windows SCM 适配、明确 ProgramData 数据路径与服务账户权限，不能假设映射盘符对服务账户可见。无此能力时 UI/帮助不要展示可用的 service install；最终服务功能是否列入正式要求由 P0 明确（本设计默认三系统服务命令为正式版要求）。

升级流程：识别现有安装/数据路径 -> 请求进程退出并等待 -> 备份 schema/config -> 替换程序 -> 迁移 -> health 检查。不能在进程占用 exe 时盲目覆盖；失败保留旧程序和备份，清楚提示恢复方式。卸载不递归删除用户输出目录。

ARM64 安装器必须检查目标架构并安装 ARM64 payload。用脚本验证 PE machine、解压清单、静默安装 `/VERYSILENT`、启动/HTTP、第二次安装升级、卸载后数据保留。实际参数以锁定 Inno 文档为准。

参考：[Inno Setup](https://jrsoftware.org/isinfo.php)。

## 5. macOS 安装程序

原生 pkg 交付每个架构；便携 tar.gz 中直接包含 Go 程序。建议 pkg 安装 `OStrm.app/Contents/MacOS/ostrm`，Info.plist 指向同一程序，无参数启动 HTTP 并打开浏览器，不引入 Electron。应用 bundle 位于 /Applications，运行数据仍在用户 Application Support 下。

需要命令行便捷入口时额外安装稳定位置的 CLI/link，不能用 app 当前工作目录存数据。用户手动开启 launchd 服务时，LaunchAgent 指向该固定二进制且传 `serve`，避免后台启动反复开浏览器。卸载文档/命令先卸载 LaunchAgent，再删除程序，默认保留数据。

Apple 签名分开处理：Developer ID Application 签 app/二进制；Developer ID Installer 签 pkg。公证后 staple 对应载体，验证签名与 Gatekeeper。CI keychain 临时创建、导入、清理，不在仓库保存 p12 或密钥。

没有证书仍能生成 unsigned pkg/tar.gz，并在文件名或 release metadata 和说明中标注未签名/未公证；不能声称用户一定能无提示双击。首版可允许未签名公开发布，但不得将其标为已公证。不要提供关闭系统保护的自动脚本。

测试：pkg payload/权限/架构、安装、app 启动、HTTP、launch agent、升级保留数据、卸载保留数据。最低 macOS 版本以 Go/依赖/实测为准，Info.plist 与 release 文档一致。

参考：[Apple 公证文档](https://developer.apple.com/documentation/security/notarizing-macos-software-before-distribution)。

## 6. Linux 安装包

nFPM 生成 deb/rpm，安装 `/usr/bin/ostrm`、示例 `/etc/ostrm/config.yaml`、systemd unit；使用专用非登录用户 ostrm、`/var/lib/ostrm`。默认监听 loopback。配置文件标记 conffile/配置保留，升级不覆盖用户修改。

unit 设置明确 WorkingDirectory/数据目录、Restart=on-failure、合理超时和文件权限；硬化配置不能阻止用户指定 STRM root。包安装建立目录与账户；服务启停策略按发行版惯例实现并写入脚本，升级不意外改变用户原来禁用的状态。卸载保留业务数据；purge 数据必须用户显式执行另一个命令。

CI 在 Debian/Ubuntu 与 RPM 系环境分别安装检查，后者可使用临时容器测试包事务，但运行器上的 native 二进制测试仍必需；容器内不支持 systemd 时不能假装已测 systemd 服务。服务生命周期在可运行 systemd 的 VM/runner 验证。没有 Docker 的用户仍可原生安装，CI 使用容器不增加用户依赖。

参考：[nFPM 集成说明](https://goreleaser.com/customization/package/nfpm/)。本方案可直接调用开源 nFPM，无需为 macOS/Windows 使用 GoReleaser 商业功能。

## 7. 签名凭据与默认行为

| 能力 | 凭据 | 缺失时行为 |
|---|---|---|
| GitHub release | 工作流 GITHUB_TOKEN | 权限不足则失败，保留 artifacts |
| Apple app/pkg 签名 | 对应证书 p12、密码 | 生成标注 unsigned 的包 |
| Apple 公证 | Developer 账号/API 凭据 | 不公证，不伪造验证结果 |
| Windows Authenticode | 证书/受支持签名服务 | 生成 unsigned setup.exe，说明状态 |

SHA256 校验和不是发布者身份签名。`release-manifest.json` 分别记录 signed/notarized/native_tested，不能用一个 success 字段混为一谈。首版不要求用户先购买签名证书才能完成 Go 版本构建；有凭据后签名流程自动启用。

## 8. 必须通过的发布门槛

- 六个二进制实际运行，所有页面、API、SQLite 创建/重启、STRM mock 闭环通过。
- 各安装包包含正确架构 payload；安装、升级、卸载路径正确，保留数据。
- 同一版 UI hash 和源码 SHA 一致；缺失任一必需包不发布。
- 网络断开时程序依然启动并展示 UI；外部服务不可达时只影响相关任务。
- 包内没有 token、真实用户数据、日志、开发路径、node_modules、JRE、Docker 配置依赖。
- 原项目 LICENSE、修改声明及第三方许可证随包提供；对应源码归档包含前端、Go 和构建安装脚本。
- 发布说明列出与上游功能差异、迁移步骤、OS 最低版本、已测平台与签名状态。
- 任何未实现的正式范围功能、未通过的架构原生测试都必须阻止“完整替代版”发布。

## 9. 实现文件合同

```text
.github/workflows/ci.yml
.github/workflows/release.yml
.go-version
build/tool-versions.json
build/frontend.mjs             # 清理/生成/复制/校验
build/build.mjs                # 六目标，版本与 UI hash 注入
build/verify-release.mjs       # 必需产物/架构/hash/测试状态
packaging/linux/nfpm.yaml
packaging/linux/ostrm.service
packaging/linux/scripts/*
packaging/macos/Info.plist
packaging/macos/build-pkg.sh
packaging/macos/sign-notarize.sh
packaging/windows/installer.iss
packaging/windows/build.ps1
packaging/windows/sign.ps1
tests/packaging/*
```

脚本必须清晰区分 dry-run/build/package/publish；本地构建永不自动推 tag 或发 release。源文件名可调整，但每项职责和验收不能省略。
