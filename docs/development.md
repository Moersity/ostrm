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

严格遵循 `dev → beta → main`，beta/main 只能通过对应 PR 接收修改，详见根目录 AGENTS.md。版本号在 dev 的 app_version.json 修改；合法 PR 合并触发 release.yml，安装检查通过后创建 GitHub Release。开发构建不等同正式版本。

默认安装包未签名，macOS 未公证。配置签名和公证前不要声称已完成。
