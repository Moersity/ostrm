# OStrm Go 分阶段实现与验收计划

## 1. 实施约束

本轮只完成设计与 fork。后续模型实现应在同一 fork 的 dev 分支进行；读取 AGENTS.md，先确认工作区干净，不覆盖用户改动。每次取一个阶段或其小任务，交付可运行代码、测试证据和进度记录，避免一次重写全部后端。

本地参考仓库：`/Users/lixiang5/Documents/Codex/2026-09-08/https-github-com-hienao-ostrm-https/work/ostrm`。远端 origin=`https://github.com/Moersity/ostrm.git`，upstream=`https://github.com/hienao/ostrm.git`。

当前本地仅 shallow main。开始时 fetch origin dev/beta/main，按需 unshallow，不将本地 main 直接改名当成已存在的 dev。分析基线为 main 的 `e25c766753c758f00aa47818057d3d9df0b4969a`；dev 如有额外功能，必须先比较并更新契约，不能用基线覆盖 dev。不要从上游现有 Quarkus/GraalVM 功能分支推断 Go 已实现。

进度记录 `docs/rewrite/progress.md` 至少含：阶段、完成 SHA、实现功能 ID、执行命令和退出状态、未解决问题、下一步。不把“新增了测试”写成“测试通过”；CI 链接是发布证据。

## 2. 阶段依赖与验收

| 阶段 | 范围 | 依赖 | 完成标志 |
|---|---|---|---|
| P0 | 基线、契约、依赖与构建试验 | 无 | 62 路由详单、基线 fixtures、六目标试构建、差异表 |
| P1 | 单文件程序、配置、嵌入前端、存储、登录 | P0 | 无 Node/JRE 环境显示 UI，登录/重启正常 |
| P2 | OpenList + 任务 CRUD + 全量核心闭环 | P1 | 真实行为 mock 下生成正确 STRM，API 对齐 |
| P3 | 增量、调度、清理、取消恢复 | P2 | 无变化/删除/失败/重启/并发全部通过 |
| P4 | 元数据、目录结构、TMDB/AI | P3 | 旧解析测试迁移、资源优先级及失败分类一致 |
| P5 | 手动/自动改名、媒体库、通知、日志/设置/版本 | P4 | 所有原 UI 功能可用，无伪成功接口 |
| P6 | 旧数据导入、跨平台回归、差分收敛 | P5 | 导入后闭环及安全差异表通过 |
| P7 | GitHub Actions、六平台安装包、发布 | P6 | 完整 CI 和安装升级卸载验证通过 |

P1 即建立基础 CI；P7 才启用正式发布，不把构建问题拖到最后发现。服务安装/开机启动跨平台适配可以 P3 开始试验，P7 全面验收。

## 3. P0：冻结基线和契约

任务 P0.1：确认 branch/remotes，复制本交接包到仓库 `docs/rewrite/`（后续实现提交时）。记录分析 SHA、实际开发 SHA、两者 diff。保留 Java 原实现作为 golden 参考，先不删除 backend。

任务 P0.2：以 `04-api-inventory.json` 为路由起点逐条提取 request body/query/path、状态码、envelope、响应字段、默认值、校验、鉴权及 frontend 调用方。输出 `contracts/openapi.yaml`、`contracts/coverage.json` 和脱敏请求响应 fixtures。静态索引未包含 record 字段的完整语义，不可当成自动完成此任务。

任务 P0.3：启动旧 Java 版本时，使用 JDK 21 与本地临时数据目录；不要求 Docker。构造 mock OpenList/TMDB/AI/Emby/Apprise，记录小型 golden corpus。若旧程序不能启动，先修复基线运行条件；不可编造响应，静态推断的 fixture 必须标注未验证。必要时将旧服务测试中的纯逻辑作为 oracle。

任务 P0.4：核对 Go 最新稳定版（本次 1.27.1），验证支持的 OS/arch 和 SQLite 驱动六目标；输出最小 SQLite create/insert/reopen + embed 页面 smoke 测试。确认 Windows ARM64、macOS Intel 的原生 runner。锁定 Node、Go、依赖和安装器版本。此试验不是正式应用功能，不公开 release。

任务 P0.5：从旧测试提取 URL/正则/Cron/NFO/季集/限流/目录结构/manifest 的案例，创建 `compatibility-matrix.md`，每行有来源、期望和 Go test 名。审计旧远端 rename/upload 失败恢复行为，定义可接受改进。

验收：每条路由有负责人状态（not-started/in-progress/verified/deviation）；所有未知项明确列出。Go 和 frontend 构建能够复现，源路径与工具链记录齐全。

## 4. P1：骨架、网页、数据库和登录

新增 cmd/ostrm、internal/app/config/httpapi/web/store/auth。实现配置优先级、平台目录、单实例锁、启动/关闭、health/version、SQLite migrations。生成并嵌入前端，根页及任意合法 SPA 深链接可刷新。

实现 F01 所有端点，保持原前端交互。新增用户 bcrypt、随机 secret、会话撤销、单管理员注册并发保护。时间与 ID 序列化按 fixture，首次安装可完整操作。

验收用例：

- 程序从不含 frontend/JRE/node_modules 的空目录启动；加载 index、JS、CSS、图标和中文文本。
- 未知 API 返回 JSON 404，缺失 JS 返回 404，不被 SPA fallback 吞掉。
- 两个并发注册只成功一个；错误密码/过期 token/刷新/登出/改密撤销都正确。
- 重启账户不丢失；端口冲突/目录不可写/第二实例报错可读；退出释放锁。
- 配置不同优先级有确定结果；`--portable` 与系统默认路径不混用。

## 5. P2：OpenList 和全量 STRM

新增 internal/openlist、tasks、scan、strm、outputs。迁移 OpenlistConfigDto、TaskConfigDto 全字段；尚未实现的高级执行能力必须显式报 unsupported，不能返回 success（正式版前清零）。完成 F02/F03 CRUD 及 F04 普通全量。

实现 /fs/list 分页递归、/fs/get、QPS/QPM、取消超时、Base URL 与签名 URL。构建目录索引；按任务相对路径输出；迁移扩展名与 renameRegex 子集。使用有界队列和 per-task 锁。

验收用例：

- 配置 A/B 不共享 token/限流状态；list 正常、空目录、200+业务错误、401、429、5xx、超时、无效 JSON。
- 多页目录不漏文件；同名文件不同目录保留；中文/空格/`+`/`%`/`#`/query/sign/baseUrl 子路径精确匹配 golden。
- 扩展名大小写、非视频、用户定义扩展、规则无匹配/无效正则与捕获替换正确。
- 全量运行生成后再运行不破坏无关文件；写入失败不伪成功。
- 路径穿越、保留名、大小写冲突与目标输出重叠被处理；测试 Windows 原生文件行为。

## 6. P3：增量、调度、清理和恢复

实现 manifest schema、config fingerprint、run/item 状态、拥有文件表、阶段写入和清理计划。新增 runs 查询/取消 API 与最小前端进度。任务快照、锁、重启 INTERRUPTED 状态。实现 CronDialect 和可验证的 Quartz 兼容解析。

验收用例（必须同时观察文件、mtime、run 状态和 manifest）：

| 案例 | 预期 |
|---|---|
| 第二轮远端无变化 | 不重写 STRM，增量跳过计数正确 |
| 仅 sign 变化 | URL 更新，即使 size/modified 相同 |
| 同目录新增字幕 | 对该目录资源重新评估 |
| 本地 STRM 被删 | 自动重建 |
| Base URL/rename/filter 配置变更 | 指纹变化，重新评估输出 |
| 远端删除一个视频 | 仅清理本任务拥有的对应输出 |
| 某页读取失败/扫描取消 | 不清理，不提交成功 manifest |
| 文件写入失败 | PARTIAL/FAILED、旧成功清单保留，下轮可重试 |
| 输出存在用户自己放的 NFO/图片 | 未认领的文件不删除 |
| 两任务同时写父子目录 | 配置或执行前阻止 |
| 运行中修改任务设置 | 本轮使用旧快照，下一轮用新值 |
| 强制中断进程 | 重启识别未完成 run/临时文件，不伪造成功 |
| 正常空目录 | 明确空目录成功；清理策略按归属和隔离规则执行 |

Cron 测试至少包含五/六字段、星期数字与名称、`?`、步进、跨月、闰日、时区/DST 和原支持的 L/W/#/year 案例。对比未来 20 次时间。拒绝不支持语法并保留原表达式是中间版本允许的降级，正式兼容版须完成实际需要的全集。

清理与写入 journal 崩溃点：写 tmp 后、替换后、更新 owned_outputs 前、manifest commit 前。逐点恢复，不能造成扩大删除。

## 7. P4：刮削与结构

迁移旧 `TaskMediaParserTest`、`SeasonDirectoryNameParserTest`、`TmdbIdExtractorTest`、`TaskDirectoryStructureValidatorTest`、`SingleFileHandlerSkipTest` 等。实现目录结构 overview/check/directory API 和旧树形 DTO。

实现字幕/NFO/图片同目录索引与下载、TMDB、AI、NFO 生成、cache。按旧配置 `scraping`、`scrapingRegex`、`tmdb`、`ai` 保留默认值和选项关系；需要修改的默认值写差异表。

验收：已有资源不重复下载；关闭刮削时 STRM 仍成功；TMDB 无匹配、AI 错误/超时只产生可解释结果；TV season/episode 命名与 XML 节点正确；不同媒体不串缓存；不完整图片不覆盖旧图；恶意 XML/超大下载有边界。

## 8. P5：手动改名和外围功能

实现 F08/F09 手动刮削树、懒加载、预览、持久化执行、查询及普通任务自动改名，先跑 mock 写操作，再使用用户单独指定的测试 OpenList 目录做可选集成验证。自动测试绝不连接用户生产目录修改文件。

移植 `ManualScrapingJobServiceTest`、`ManualScrapingServiceTest`、`MediaServerApiServiceTest`、`NotificationServiceTest`、`NotificationRendererTest`、`LogServiceTest`、`SystemConfigServiceCacheTest`。

远端变更场景：预览后源已改名、目标冲突、重命名成功但响应丢失、上传部分成功、取消、中断重启、普通任务与手动任务重入。每个场景应有确定检查点和重试边界，不能重复远端操作。

完成媒体服务器列表/连接/媒体库/刷新、Apprise 过滤和部分成功详情、日志 cursor/tail/download/delete、系统配置深度合并、AI 和通知测试、版本 channel。完成原前端全部页面/按钮回归，遥测默认关闭且不上报上游。

## 9. P6：旧数据与一致性收敛

制作脱敏 legacy fixture：不同 SQL schema 版本、单用户 MD5、systemconf、多 OpenList、复杂 Cron、正则、媒体服务器、路径和旧 manifest。提供 dry-run 及实际迁移，原目录逐字节不变；重跑导入无重复；旧任务导入后默认停用并提示检查。

导入后登录、查看设置、启用任务、生成输出、重启、增量、通知 mock 全流程。原 token/session 不继续有效；首次 legacy 密码认证升级为 bcrypt。备份还原后数据一致，旧二进制拒绝新 schema。

对照测试分层：

1. API：响应 JSON 结构/状态/默认值对齐，时间和随机 token 使用语义比较。
2. 输出：STRM 路径与字节比较；NFO XML 做 canonical 比较；mtime 和无变化不重写断言。
3. 业务事件：OpenList 调用序列、远端 rename/upload intent、媒体库刷新范围、通知类别对齐。
4. 故障：检查旧版缺陷是否被有意修复，记入差异表；不可为了字节等价保留危险清理行为。

完成 `contracts/coverage.json`，62 路由不能有未标注的空实现。核心功能 F01–F12 全部映射到至少一个集成或 E2E 用例；批准的变化有单独证据。

## 10. P7：安装与发布

严格按 `05-release-design.md` 实现 CI、六目标 build、安装器、服务适配、签名可选与 release promotion。先运行 workflow_dispatch snapshot，只上传 artifacts；验证通过后，走 dev -> beta 的预发布，再 beta -> main 正式版。

每平台测试：安装 -> 启动 -> 注册 -> mock OpenList -> 全量 -> 增量 -> 退出 -> 升级 -> 配置/数据保留 -> 服务启动停止 -> 卸载 -> 数据保留。测试二进制必须来自待发布 artifact，不能另编一个测试版本替代。

证书缺失时可发布明确标识 unsigned 的包；原生架构测试缺失时不可以报告该架构 verified。所有 core test、UI test、migration test、package test 通过才启用正式发布。

## 11. 性能与资源验证

性能指标是验收目标，不是已测结论。基准环境、Go 版本、硬件、数据集、worker/限流配置必须记录。

- 使用 1 万/10 万条目的 mock 目录，比较扫描耗时、总请求数、峰值 RSS、增量写入数和 goroutine 数。
- 默认 worker 数有上限；条目数量增长不能线性增加活跃 goroutine；读取/下载有上限，避免把图片全部驻留内存。
- 与 Java 基线同条件运行。Go 不得通过跳过必要扫描来获得假加速。
- 本机 mock 取消操作目标 2 秒内响应；阻塞网络受请求 timeout 限制，明确报告最坏边界。
- 第二轮无变化 STRM 写入为 0；同媒体目录识别缓存避免重复 API；限流吞吐不能超过用户设置窗口上限。
- 10 万条目内存目标先设低于 512 MiB；P0/P3 实测后有依据调整，不能声称单二进制必然低内存。

## 12. 关键风险及处理

| 风险 | 处理 |
|---|---|
| Java 正则与 Go RE2 不同 | P0 语法审计 + fixture；明确支持范围/兼容引擎 |
| Quartz 与 Unix Cron 星期或特殊符号不同 | dialect 元数据 + 下一执行时间差分 |
| 表面 UI 兼容但高级接口为空 | 路由/功能矩阵 + 端到端验证 |
| SQLite 文件和文件系统无法共同事务 | journal + 恢复对账 + 延迟 manifest commit |
| 远端请求超时后是否已写入未知 | intent 检查点 + 查询现状 + 不盲重试 |
| 源扫描不完整引起误删 | 完整扫描标记 + owned_outputs + 隔离策略 |
| 静态嵌入漏 `_nuxt` | `all:dist` + 二进制资源 smoke |
| ARM 构建通过却运行失败 | 六目标原生测试，缺失则阻止兼容 release |
| 用户更换机器路径/时区 | 显式迁移映射和调度检查 |
| 签名凭据不存在 | unsigned 明示，签名为可选独立步骤 |

## 13. 每阶段提交标准

- 变更只包含本阶段代码及必要测试/文档，运行命令可复现。
- 不修改 main/beta，不直接 push 到上游；涉及发布遵守仓库晋级规则。
- 不在 CI 使用真实外部 API key；fixtures 脱敏，测试不发消息到真实用户。
- 若无法完成某项，保留失败证据和下一步，不通过删除测试、吞异常、返回假成功解决。
- 功能实现代码和单元测试不是最终发布证明，还需 CI 与安装包验证。
- 实现开始后用户若要求只做一阶段，完成该阶段即停；没有这样的限制则按计划连续推进。

## 14. 可复制给实现模型的初始提示词

```text
请实现 Moersity/ostrm 的 Go 重写。先阅读工作区 AGENTS.md，以及本地交接包中的
00-README.md、01-design.md、02-implementation-plan.md、03-source-contract-index.md、
04-api-inventory.json、05-release-design.md。

仓库已有 origin 指向 Moersity/ostrm、upstream 指向 hienao/ostrm。
只在 dev 开发；发布必须 dev -> beta -> main。不要覆盖现有工作区改动。
基线 main SHA 为 e25c766753c758f00aa47818057d3d9df0b4969a；先检查 dev 与基线差异。

先完成 P0 和 P1，暂不发 release。核对 Go 最新稳定版并锁定，不使用浮动 latest。
保留现有 Nuxt 前端，go:embed all:dist 打包；核心运行不依赖 Docker/Java/Node/Caddy。
不得删减核心功能，也不要用返回 success 的空接口假装已经迁移。
P0 的静态路由索引只是导航，必须补齐真实请求/响应契约和黄金测试。
每阶段更新 docs/rewrite/progress.md，记录实际执行的测试及结果、差异和下一步。
完成后报告代码位置、已通过测试、仍未实现的功能，以及继续 P2 的入口。
```

后续提示词：

```text
继续阅读 docs/rewrite/progress.md，从下一个未完成阶段推进。
按设计保持核心功能一致；优化必须在 behavior-differences.md 登记并有测试。
本轮完成一个阶段并验证，不跳过其前置验收，不发布未经测试的安装程序。
```
