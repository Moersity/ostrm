# Repository Instructions

## 合并与发布

按用户最新授权，开发分支直接通过 PR 合并到 `main` 并发布，不再经过 `beta`。

- 日常开发在 `dev` 或功能分支完成，禁止直接提交到 `main`。
- 用户要求合并时，创建或更新同仓库开发分支 → main 的 PR；确认 head、base、提交 SHA 和测试状态后合并。
- 合并到 main 自动触发 `.github/workflows/release.yml`，六个平台原生构建与安装检查通过后发布正式版本。
- 不需要再次询问是否发版或日常版本号；按下述语义规则自动管理。
- 不主动创建 beta 晋级 PR，不覆盖已发布标签或附件，不把 dev artifacts 当成正式 Release。

## 自动语义化版本

正式 GitHub Releases 是版本账本，版本格式为主版本.次版本.补丁版本，无需手工修改版本文件。

- 默认 patch：兼容修复、清理、维护，例如 fix:、refactor:、chore:、docs:。
- minor：兼容的新功能，PR 标题或提交使用 `feat:` / `feat(scope):`。
- major：不兼容变更，使用 `feat!:`、`fix(scope)!:` 等，或正文中的 `BREAKING CHANGE:`。
- 可通过 `semver:patch` / `semver:minor` / `semver:major` 标签声明最低级别；多种信号同时存在时取最高级别，不能把破坏性变更降为 patch。
- 判断范围为最近正式版之后至本次合并提交的全部提交，以及本次 PR 标题、正文和标签。新增功能必须写明 feat，自动化不推测自然语言或源代码含义。
- 本次重跑沿用同一提交的已发布版本或草稿版本，不再次递增；旧提交不能覆盖较新的正式版本。
- 构建脚本将版本注入 Go 二进制、嵌入网页与安装包。默认 3.0.0-dev 仅是本地开发占位，不代表最新正式版本。

发布规则脚本是 `build/release.py`，回归测试在 `tests/release`。修改发布逻辑必须运行这些测试和现有 CI。
