# OStrm Go 架构与功能一致性设计

## 1. 目标与范围

将 OStrm 的运行时后端从 Java 改为 Go，复用现有 Nuxt/Vue 前端，通过 `go:embed` 将静态网页嵌入二进制。最终交付 macOS、Linux、Windows 的 amd64/arm64 便携包与安装程序。以既有接口、任务行为和用户数据可迁移为主要约束，而非逐行翻译 Spring 类。

首个正式版必须覆盖下表 F01–F12，不能把只支持生成 STRM 的 MVP 称为完整替代版。中间里程碑可以发布明确标注功能缺口的 prerelease。暂不增加桌面 WebView、视频转码、云端账号同步、自动覆盖安装和多节点调度。

运行依赖：OS + 单个 Go 二进制 + 可写数据目录；OpenList 仍是外部服务。构建依赖：Go、Node.js、npm，以及 CI 对应系统的安装包工具。Apprise 通知保持调用外部 HTTP 服务，不在二进制内嵌 Python/Apprise。

## 2. 基线证据与现有实现

固定分析提交：`e25c766753c758f00aa47818057d3d9df0b4969a`。所有相对源码路径均以仓库根为起点。源码清单见 `03-source-contract-index.md`。

| 事实 | 源码证据 | 重写含义 |
|---|---|---|
| Nuxt 关闭 SSR，使用 generate 输出静态页面 | `frontend/nuxt.config.ts`、`Dockerfile` | 可直接嵌入网页，不需 Node 服务 |
| 后端与网页在现有镜像中由 Java/Caddy 分别提供 | `Dockerfile`、`Caddyfile` | Go HTTP 服务合并二者 |
| API 主要响应为 code/message/data | `dto/ApiResponse.java` | 不能把所有返回格式换成新格式 |
| 有 62 个控制器 HTTP 路由 | 本交接包路由索引 | 必须逐条核实，不按猜测补接口 |
| 增量清单含 type/size/modified/sign；以父目录判定变化 | `service/TaskManifestService.java` | 字幕变化也可能触发同目录视频处理 |
| 自动重命名先于生成，重命名后重新扫描 | `service/TaskExecutionService.java` | 会修改远端，不能误当成本地文件名规则 |
| 全量当前先完整扫描，再清空输出目录 | 同上 | 允许优化为阶段写入与保守清理，需登记行为变化 |
| 有多个处理器，但主服务还负责调度、过滤和清理 | 同上、`handler/*` | 不可只照抄 Handler 顺序遗漏主服务 |
| Apprise 使用 `/notify/{configKey}/` | `service/NotificationService.java` | 保持 HTTP 兼容 |
| 旧管理员密码保存在 userInfo.json 中，使用 MD5 | `service/SignService.java` | 新密码升级为现代哈希，保留一次性导入路径 |
| Caddy 预留 /ws，源码未检索到完整 WS 服务实现 | `Caddyfile`、Java/frontend 全文检索 | 不假设已有 WS 消息协议；进度先保留轮询 |

上述是静态分析结果，尚未用 Java 运行时做差分验证。控制器、DTO、测试、前端实际调用优先于过时 README；冲突必须记录。

## 3. 功能对照表

| ID | 必须保留的能力 | Go 归属 | 验收核心 |
|---|---|---|---|
| F01 | 首次注册、登录、检查用户、退出、刷新、验证、修改密码 | auth | 原前端登录闭环、刷新失效和单用户初始化 |
| F02 | 多 OpenList 配置、启停、连接/目录验证、Base URL、编码、QPS/QPM | openlist/settings | 配置隔离、边界值和限流重试 |
| F03 | 任务 CRUD、启停、手动/定时执行、全量/增量覆盖选项 | tasks/scheduler | 保存后重启仍可调度；同任务不能重入 |
| F04 | 递归扫描、视频过滤、相对目录、文件名规则、签名 URL、STRM | scan/strm | 路径及内容 golden fixtures 一致 |
| F05 | 任务 manifest、配置指纹、缺失文件重建、孤立清理 | manifest/outputs | 无变化不重写、失败不提交清单、不误删 |
| F06 | 字幕、已有 NFO/图片、TMDB 刮削、AI 识别、电影/剧集/动漫 | metadata | 保留选项和处理优先级，AI 非必需 |
| F07 | 目录结构检查、懒加载检查、skipInvalidStructure | structure | 移植旧测试和异常树 DTO |
| F08 | 手动刮削树/预览/执行/状态、远端重命名上传及检查点 | manualjobs | 刷新页面可恢复，网络模糊失败不重复改名 |
| F09 | 普通任务 autoRenameMedia，重命名后重新列目录 | tasks/manualjobs | 使用新路径生成 URL，失败有分类 |
| F10 | Emby/Jellyfin 配置、连通测试、媒体库列表、全量/指定刷新 | mediaserver | 增量无变化避免刷新，刷新失败独立报告 |
| F11 | Apprise 终态通知、部分成功分类、路径和详情上限 | notify | 成功/部分成功/失败过滤和重试边界 |
| F12 | 系统设置、AI/通知测试、日志读取/tail/下载/清理、版本检查 | settings/logs/version | UI 所有入口可用、从 fork 查询版本 |

原 `/api/data-report/*` 仍返回兼容响应，但新版本默认关闭遥测，不发送到上游作者服务。此项作为明确优化列入变更说明。备份、日志清理、版本检查等内部 Job 也要盘点：有用户可见设置的必须迁移；过时邮件 Job 若无有效调用路径，允许记录证据后移除。

## 4. 技术选型

| 组件 | 决策 | 说明 |
|---|---|---|
| Go | 本次核对为 1.27.1，实施时确认后锁定 | go.mod 与 `.go-version` 一致；CI 不浮动升级 |
| HTTP | 标准库 net/http，显式分层 handler/service/repository | 减少框架依赖，统一中间件处理恢复、鉴权、日志 |
| 静态 UI | 保留 Nuxt 构建，内嵌 `.output/public` | 不重写页面，不让 Go 工程反向依赖 Nuxt 开发服务器 |
| 数据库 | database/sql + modernc.org/sqlite | 避免 CGO；必须先验证六目标兼容并锁定驱动/传递依赖 |
| 密码 | golang.org/x/crypto 的 bcrypt | 新密码使用 bcrypt；旧 MD5 只用于导入后首次验证升级 |
| 定时器 | 调度器与 Cron 解析器隔离 | robfig/cron/v3 仅可处理验证过的子集，不能声称兼容全部 Quartz |
| JSON/XML | encoding/json、encoding/xml | DTO 不泄漏数据库结构；NFO 以 XML 结构比较 |
| 并发 | context + 有界 worker pool | 不按文件数无界创建 goroutine |
| 日志 | slog + 按日/大小轮转组件 | 组件版本在 P0 锁定，日志读取保留原契约 |
| 发布 | GitHub Actions + 原生安装包脚本 | Linux 用 nFPM；macOS pkgbuild/productbuild；Windows Inno Setup |

所有第三方版本在 P0 验证后落入 `go.mod/go.sum` 和构建工具版本表。不要在本设计未验证时编造版本号，也不要引入需要商业版工具才能完成的必选步骤。

本次核对 SQLite 驱动文档已列出目标六种 OS/arch；这不替代实际构建测试。其文档特别要求 `modernc.org/libc` 与驱动 go.mod 使用相同版本，不可单独随意升级该传递依赖。

参考：[Go 下载](https://go.dev/dl/?mode=json)、[embed](https://pkg.go.dev/embed)、[SQLite 驱动](https://pkg.go.dev/modernc.org/sqlite)、[cron](https://pkg.go.dev/github.com/robfig/cron/v3)。上述技术决定属于本项目设计，不代表依赖已经过交叉平台测试。

## 5. 目录布局与职责

```text
cmd/ostrm/main.go
internal/app/                  # 依赖组装、启动/关闭
internal/config/               # 配置、路径、不可变快照
internal/httpapi/              # 路由、旧 API 适配、鉴权、错误
internal/auth/
internal/store/                # SQLite repositories + migrations/*.sql
internal/openlist/             # HTTP 客户端、限流、重试、分页
internal/tasks/                # 作业状态、执行服务、锁
internal/scheduler/            # CronDialect、下次执行、misfire
internal/scan/                 # 目录发现和资源索引
internal/strm/                 # URL/名称/输出计划
internal/manifest/             # 快照与配置指纹
internal/outputs/              # 路径约束、原子写入、归属清单/清理
internal/structure/
internal/metadata/             # TMDB、AI、NFO、图片/字幕
internal/manualjobs/           # 远端变更计划与持久化检查点
internal/mediaserver/
internal/notify/
internal/logs/
internal/version/
internal/migratelegacy/
internal/web/embed.go
internal/web/dist/             # 构建时复制的静态站点
frontend/                     # 尽量复用
backend/                      # 过渡期只作基线，正式打包不包含 JRE
build/                        # Node/Go 构建脚本
packaging/{linux,macos,windows}/
testdata/{contracts,openlist,legacy,golden}/
docs/rewrite/                 # 后续复制本交接文档及进度
```

业务服务只依赖小接口：`OpenListClient`（List/Get/Rename/Upload）、`OutputStore`（Plan/Write/Commit/Cleanup）、`TaskRepository`、`Clock`、`Notifier`。HTTP 和 CLI 共用服务，不通过访问自己的 HTTP 接口执行业务。禁止将所有逻辑放进 main.go 或巨大 handler。

## 6. 单二进制服务与前端

1. `npm ci`、`npm run generate`，完整复制 `frontend/.output/public` 到 `internal/web/dist`。先清理旧 dist，避免旧 hash 资源残留。
2. `//go:embed all:dist`，再 `fs.Sub(assets, "dist")`。必须用 `all:`，否则 `_nuxt` 可能被遗漏；不能 embed `../../frontend`。
3. `/api/` 路由优先；未知 API 返回 JSON 404，不能返回 SPA 页面。不存在的 `/_nuxt/*`、图片、脚本返回 404；仅 HTML 页面导航回退 index.html。HEAD、MIME、Content-Length、缓存头应正确。
4. hash 静态文件可长期缓存；index.html 与运行配置端点禁止长期缓存。构建版本注入前端与 Go 后端，二者必须一致。
5. 生产 API 使用同源 `/api`。现有轮询保留。新增进度先提供 `/api/task-config/{id}/runs/latest`、`/runs/{runId}` 与取消接口；DTO 作为 Go 扩展，不能篡改原 submit 响应。
6. 暂不实现 WS。若后来加 SSE/WS，必须鉴权、断线重连与有界缓冲，保留轮询降级；不能照抄原安全配置中的匿名 `/ws/**`。
7. 运行二进制放进全新临时目录时仍能显示完整 UI，是发布硬门槛；浏览器不能向 localhost:3000/8080 等开发地址发请求。

## 7. 配置、启动和平台路径

命令：`ostrm serve`；无参数等价 `serve --open-browser`，用于便携双击。`serve` 默认不自动开浏览器，适合服务进程。另提供 `version`、`doctor`、`migrate-legacy`、`backup` 和 `service install/uninstall/start/stop`。服务安装必须由用户显式执行；安装包不会自动暴露局域网端口。

监听默认 `127.0.0.1:3111`。支持 `--listen`、`--data-dir`、`--strm-root`、`--config`、`--open-browser`。优先级：CLI > OSTRM_* 环境变量 > 配置文件 > 默认值。启动参数和系统 UI 业务配置分开；UI 修改不能覆盖 CLI 的监听地址。

| 场景 | 默认数据路径 |
|---|---|
| Windows 用户模式 | `%LOCALAPPDATA%/OStrm` |
| macOS 用户模式 | `~/Library/Application Support/OStrm` |
| Linux 用户模式 | `$XDG_DATA_HOME/ostrm`，未设置则 `~/.local/share/ostrm` |
| 显式便携模式 `--portable` | EXE/二进制所在目录下 `data`，不可写则明确报错 |
| Linux 系统服务 | `/var/lib/ostrm`，配置 `/etc/ostrm`，日志由服务配置决定 |

默认 STRM root 为 data-dir/strm；用户可设置本地磁盘或可写共享目录。相对参数在启动时解析成绝对路径并记录；不随当前 shell 目录漂移。支持用户目录中文、空格；禁止数据默认写在 Program Files 或 macOS .app 内部。

仅允许一个进程写同一 data-dir，采用跨平台 OS 文件锁；锁内保存 PID 只用于诊断，不能把 PID 文件存在当成有效锁。二次双击仅在验证既有实例是同一数据目录的 OStrm 后打开页面；端口被其他程序占用则报错。

内嵌 `time/tzdata`，保证 Windows/极简 Linux 无系统时区库时也可使用 IANA 时区。新安装默认系统时区，UI 可配置；旧 Docker 导入要求显示并确认原时区（原镜像设置 Asia/Shanghai），不能悄悄按新机器时区执行。

## 8. API 契约与鉴权

现有 62 路由的 request/query/path/response 从控制器 + DTO + frontend 调用共同提取，形成 OpenAPI 与 fixtures。特别记录：业务失败仍可能 HTTP 200；sign、日志下载和 data-report 不一定使用相同 envelope。不存在数据的 null、空数组、空对象不可互换。

Go DTO 保持 camelCase；可选布尔用 `*bool`，避免 PUT 未传字段被误置 false。`lastExecTime` 时间单位、LocalDateTime 格式、枚举大小写、错误消息依赖必须在 P0 逐项验证。Go 内部状态不直接映射到原字符串，使用兼容适配层。

保持前端现有 Bearer token 登录方式，核实原 token 格式后实现兼容刷新流程。新实例生成随机持久化 secret，无默认 `secret`。退出撤销会话，改密撤销全部会话；第一次注册必须在事务内限制单管理员。只对必要的登录/检查用户开放匿名访问。限制登录尝试，日志不记录密码、token、AI key 或完整签名 URL。

业务配置导出必须支持脱敏；内部存储 OpenList token 仍需供后台调用，不能用不可逆哈希代替。保留受文件权限保护的本地存储，首版不承诺硬件密钥库或全盘加密。非本机监听由用户显式配置。

## 9. 任务执行算法

### 9.1 状态和互斥

内部状态：QUEUED -> RUNNING -> SUCCESS / PARTIAL_SUCCESS / FAILED / CANCELED；RUNNING 重启后转 INTERRUPTED，不伪造成功。阶段：DISCOVERY、AUTO_RENAME、PLAN、WRITE_STRM、METADATA、CLEANUP、FINALIZE。媒体库刷新/通知另存 delivery 状态，不能因通知失败重跑所有媒体任务。

同 taskId 的普通/手动任务互斥。不同任务输出目录相同或祖先/子目录重叠时，第一版直接拒绝配置；不尝试共享目录并发清理。任务使用创建时的不可变配置快照，配置修改下轮生效。取消通过 context 传播到限流等待、HTTP、扫描和下载；取消时不进入清理、不提交 manifest。

### 9.2 OpenList 客户端

保留 `/api/fs/list`、`/api/fs/get`、`/api/fs/rename`、`/api/fs/put` 的现有请求体、认证头、上传编码、分页及响应处理。mock 要区分 HTTP 成功而业务 code 失败。baseUrl 含子路径时不可丢失前缀。

每个配置共用 QPS + QPM 限流器，扫描、下载元信息、重命名、上传全部计入原规则；0 的含义为不限制。动态修改在下一快照或受控刷新时生效。默认有界目录 worker=4、下载 worker=4，并受限流器进一步约束，P0 测试后可调整。

只对幂等读取的网络错误/429/明确可重试 5xx 退避重试，尊重 Retry-After。重命名/上传遇到超时属于结果未知，先查询远端现状，不盲目再次执行。分页失败必须标记扫描不完整，禁止当成空目录。

### 9.3 执行顺序

1. 取得任务/输出根锁，保存 run 和配置快照；验证源/目标路径。
2. 完整递归发现源目录并验证每页成功；生成目录索引与扫描完整性标记。
3. 若显式开启 autoRenameMedia，生成远端计划并持久化检查点；执行后重新扫描。不得复用改名前 URL。
4. 按系统视频扩展配置和目录结构规则筛选。记录被跳过路径及原因；区分远端真实删除与策略过滤。
5. 读取上次成功 manifest，比较配置指纹和路径元数据；全量选择所有合格视频；增量选择变化目录中的视频及缺少本地 STRM 的视频。
6. 为每个视频建立输出计划，执行 URL/名称规则、原子 STRM 写入、选项控制的字幕/NFO/图片/刮削。失败记录到文件维度，其他文件可继续。
7. 仅扫描完整且未取消、所有相关目录可确定时，计算本任务拥有的孤立输出。先预览/隔离再删除，不扫描删除未知用户文件。
8. 若核心处理失败则保留旧成功 manifest，下轮重试；先完成输出持久化，再在事务内提交 manifest 和 run 结果。不得只更新部分 snapshot 却标记全轮成功。
9. 按 hasChanges/刷新范围调用媒体服务器，再发送终态通知。保存失败分类，不覆盖核心任务状态。

### 9.4 增量与输出一致性

旧指纹从 `manifestConfigurationFingerprint()` 提取完整字段，不仅包含任务路径，还包括影响 URL、过滤、重命名、刮削的配置。Go 指纹使用版本化、字段有序的 canonical JSON + SHA-256，不依赖 map 迭代顺序。

远端条目继续比较 type/size/modified/sign；签名变更即使视频大小未变也须更新 URL。新增/删除字幕应让同目录资源重新评估。没有 manifest、损坏、配置变化均完整重新评估，但不先删除输出。STRM 内容相同则不写入，保持 mtime。

旧全量的“先清空再写入”改成“生成新计划、按文件替换、最后清理本程序拥有的过期输出”。这是明确行为优化，不保证保留旧版清空未知文件的副作用。全目录事务在不同磁盘不可假设；仅承诺单文件替换与可恢复 journal，崩溃后对账，再进行下一轮。

### 9.5 URL、名称和 Windows

远端路径用 `/` 和 URL 语义；本地用 filepath。URL 禁止直接 filepath.Join。编码保留协议、host、端口和 query；避免 `%` 重复编码，区分空格、加号、中文、`#`、`?`、已有 `%2F`；签名只按旧测试规则添加，不重复拼接。

旧 `renameRegex` 是按第一个 `|` 拆为模式与替换，Java replaceAll 语义；不是纯模式字符串。Go RE2 不支持 Java 的所有 lookaround/backreference。P0 盘点默认正则和示例；支持子集必须 fixture 验证。不支持时保存/导入明确报错并保留原值，不能默默忽略。若兼容层必须增加 regex 引擎，必须限制运行时间并证明六平台可构建；不通过运行外部 Java/Python 实现。

本地路径守卫检查绝对路径、盘符/UNC、`..`、NUL 和符号链接/Windows junction，输出不能逃出 root。文件名处理覆盖 CON/PRN/AUX/NUL/COM1/LPT1、尾随点/空格、非法字符、大小写冲突、Unicode 归一化和长路径。非法组件做确定性替换并追加源路径短哈希防碰撞，保存源到输出映射；有效且无冲突名称保持原样。重命名产生冲突时禁止覆盖另一个视频。

新建文件：同目录临时文件 -> flush/close -> 平台适配替换 -> 更新归属清单。Windows 对已存在/被占用目标的替换和恢复需单测；不能假定 os.Rename 在所有 OS 都同样原子。

### 9.6 清理隔离与资源默认值

新版本默认把孤立的自有文件移动到 `data-dir/trash/<runId>/`，保留 7 天，UI 提供查看/恢复，再由维护任务删除过期项。跨盘移动先复制并校验，确认成功才删源；磁盘不足则停止清理并报告，不提交对应删除完成状态。恢复检查目标冲突，不覆盖新生成文件。变更过滤策略时，被过滤项目先列入清理计划，不能把它们当作远端丢失后立即删掉。

清理计划记录 sourcePath/outputPath、归属证据、原因和文件 hash；在隔离前复核文件没有被用户修改。导入既有目录时默认不认领旧未知文件，用户明确采用迁移计划后才建立映射。全量“重建”也遵循这一归属规则。

新配置的起始默认值：HTTP 连接超时 10 秒、普通 API 超时 30 秒、最多 3 次总尝试、目录/文件 worker 各 4、JSON body 上限 8 MiB、日志上传上限 1 MiB、元数据单文件下载上限 32 MiB；长时间上传/下载另设超时和可配置限额。旧配置导入保留已设置值。上述限额为新设计值，P0 必须确认不会截断有效分页与资源，必要时调整并测试；不能用静默截断满足限额。

## 10. 刮削与远端变更

处理顺序沿用各 Handler 的开关与行为：本地可用资源、OpenList 同目录资源、在线刮削；不能将“优先使用已有资源”强制开启，须保留原选项默认值。电影、剧集、动漫、Season 命名、TMDB ID 提取、集数识别、XML 字段、海报命名全部移植对应 Java 测试。

AI 调用必须有明确开关、限流、超时、解析失败分类；只返回待验证的结构化候选，不可让模型输出决定任意本地路径或直接远端操作。同一任务同媒体目录共享识别缓存，缓存 key 含语言、类型及相关配置；修改 API key/model 时受控失效。

手动刮削预览返回 old/new path、TMDB 选择、待生成/上传文件、冲突和计划 hash。执行校验 hash 与源元信息，变化则要求重新预览。远端 rename/upload 每一步存 intent 和结果；发生中断，恢复时读取远端确认；无法判断则 NEEDS_ATTENTION，不自动扩大操作范围。普通 autoRenameMedia 复用同一引擎，默认关闭。

原作业 DTO 阶段和内部 Go 阶段用适配表映射，不新增后前端无法识别的值。取消是新增能力，不应宣称远端已执行的改名能自动回滚；返回已执行动作清单。

## 11. Cron 兼容

旧控制器接受五字段并转换为 Quartz 六字段；旧数据库可能还有复杂 Quartz 表达式。Unix 与 Quartz 的周数字、`?`、`L`、`W`、`#`、年份和 DST 语义不能简单字符串替换。

每条任务保存 originalExpression、dialect、timezone、normalizedExpression。新 UI 优先五字段 Unix 或明确六字段 Quartz，不混用数字星期。导入旧表达式按 Quartz 解析，比较旧版未来至少 20 次执行时间；覆盖月末、跨年、夏令时和星期边界。需要完整实现的语法以原验证器可接受范围及导入 fixtures 为依据。

P3 可以先交付有测试的常用子集，但不支持表达式必须标记任务停用、显示原因并保留原文，不可静默改变执行日；存在必需语法缺口时不得发布“完整兼容”的正式版。misfire 默认不补跑全部漏掉时刻，只最多补一次并避免同任务并发；此优化需要 UI/迁移报告说明。

## 12. 数据库与旧数据导入

新建独立 `ostrm.db`，不原地对 Flyway/Quartz 表做迁移。建议表：schema_migrations、users、sessions、openlist_configs、task_configs、system_settings、media_servers、task_runs、run_items、manifest_snapshots、manifest_entries、owned_outputs、manual_jobs、manual_job_steps、delivery_attempts。主实体保留可映射旧 ID，时间内部 UTC，API 用适配层输出。

SQL migrations 事务执行并记录版本与 checksum。SQLite 开 WAL、busy timeout、foreign_keys；首版限制写并发，网络调用不占事务，文件系统操作不能伪称与 SQLite 同一事务。manifest_entries 以 snapshot_id + remote_path 索引，owned_outputs 用 taskId + output_path 唯一键，source_path 单独保留。无真实负载证据前不引入 ORM/Redis。

`migrate-legacy --source <目录> --data-dir <新目录> --dry-run`：

1. 要求旧应用停止写入或使用一致性数据库备份；不能只复制活动 SQLite 主文件而忽略 WAL。
2. 只读识别旧 db、config/systemconf.json、userInfo.json 和 task-manifests。枚举 SQL 字段；缺失版本明确报错，未知设置保留在 legacy_extra。
3. 生成路径映射计划，将 `/app/backend/strm`、`/maindata` 映射到目标目录；不修改 OpenList 远端路径。
4. 报告 Cron、正则、输出重叠、密码编码等兼容问题。导入任务默认停用，源数据不变，写入新库事务。
5. 旧 MD5 标记为 legacy_md5，首次成功验证后 bcrypt 替换并撤销旧会话；旧平台编码无法确定时提供本地重置命令。
6. 旧 manifest 可以作差分参考，不能仅凭文件名认领全部现有输出；首次迁移采用保守模式，不清理未建立归属的文件。
7. 导入媒体服务器密钥、通知配置并脱敏输出报告；不沿用旧 JWT。重复导入按 source fingerprint/id mapping 幂等处理，不重复创建任务。
8. `backup` 使用 SQLite 一致性快照 + 配置/secret；排除或显式选择巨大 STRM 输出。恢复到新目录验证后切换；升级前自动备份 schema，旧二进制发现新 schema 应拒绝启动。

## 13. 优化边界与验收原则

允许优化：安全全量更新、明确的取消/恢复、有界并发、流式下载、缓存、配置快照、随机 secret/bcrypt、关闭遥测、平台路径和程序自有输出清理。每项写入 `behavior-differences.md`，包含旧行为、新行为、测试和用户影响。

不能借优化删除：多 OpenList、签名/编码/Base URL、增量/全量、目录结构、手动/自动远端改名、已有元信息复用、TMDB/AI、媒体库刷新或通知。不能把异常变成成功空列表，不能把未匹配的刮削结果算成成功，也不能发布只有 TODO 的功能入口。

正式版必须通过 `02-implementation-plan.md` 的功能、故障、迁移和六平台安装验收。设计不是一致性证明；通过契约 fixtures + 故障注入 + 旧版差分 + 原生安装测试，才可以报告兼容程度。
