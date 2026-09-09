# 01 开发环境与构建约定

日期：2026-09-09。已确认技术栈：Wails v2、Go、Vue 3/TypeScript、SQLite；目标为 Windows/macOS 本地桌面，优先控制包体并保持运行效率。本文件规定准备与验收方式；没有安装工具、配置 CI 或操作证书。本机实际盘点由主任务完成，结果见下文；其余版本是官方要求或建议，不表示已安装。

## 本机只读盘点

主任务在当前 Windows 环境读取到 Node v22.22.0、npm 10.9.4、Git 2.52.0.windows.1；go、wails 在当前 PATH 中不可用，这不等于确认机器没有安装。项目目前只有设计文档与术语表，尚无应用代码和依赖锁文件。Node 22.22.0 满足下表当前 Vue/Vite 要求，可继续使用，不要求先升级；Node 24 LTS 只是新环境统一版本的建议。后续先定位已有 Go/Wails 和 PATH，再决定是否安装。

## 1. 工具版本：最低要求与建议分开

| 工具 | 官方要求或当前状态 | 项目建议，首次集成后锁定 |
| --- | --- | --- |
| Wails | v2 文档当前标记 2.15.0 | 固定 v2.15.0，CLI 与 go.mod 一致 |
| Go | Wails 要求 1.21+；macOS 15+ 要求 1.23.3+ | 先验证当前稳定 1.27.1，结合目标 OS 确认 |
| Node | Vue quickstart 为 ^22.18.0 或 >=24.12.0；Vite 为 20.19+/22.12+ | Node 24 LTS，至少 24.12；固定实际补丁版本 |
| Vue/Vite/TS | Vue 官方脚手架使用 Vite，支持 TypeScript | Vue 3 + Composition API + script setup；提交 npm 锁文件 |
| MCP Go SDK | 官方 latest 为 v1.7.0，模块声明 Go 1.25.0 | 优先验证并锁定 v1.7.0 |

依据：[Wails 依赖](https://wails.io/docs/gettingstarted/installation/)、[Go 下载](https://go.dev/dl/)、[Vue quickstart](https://vuejs.org/guide/quick-start.html)、[Vite](https://vite.dev/guide/)、[Node 发布线](https://nodejs.org/en/about/previous-releases)、[MCP SDK 发布](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0)、[SDK go.mod](https://github.com/modelcontextprotocol/go-sdk/blob/v1.7.0/go.mod)。

不能只照 Wails 的 Node 15 最低值安装：完整工具链取各依赖要求的交集。当前 Node 24 是 LTS，Node 26 是 Current；选择前者是项目建议。SQLite 已确定，但 Go 驱动尚未确定；若驱动涉及 CGO，要追加相应 C 编译环境，不能提前宣称所有平台均无需原生编译器。

MCP v1.7.0 支持 2026-07-28 并保留旧协议兼容；新协议 HTTP 入口需要启用 Stateless。后续以 CRUD、stdio/HTTP 和目标客户端实测验收，不使用未固定的 main 或预发布依赖。[SDK 版本说明](https://github.com/modelcontextprotocol/go-sdk/releases/tag/v1.7.0)

## 2. 操作系统环境

Windows 开发需要 Go、Node/npm、Wails CLI 和 WebView2 Runtime。WebView2 是用户端也需要的运行组件；建议使用共享 Evergreen，缺失时引导安装。离线完整包和 Fixed Version 是另一分发选择，会增加包体及维护责任，待离线需求确认。[Wails 安装](https://wails.io/docs/gettingstarted/installation/)、[Microsoft 分发说明](https://learn.microsoft.com/en-us/microsoft-edge/webview2/concepts/distribution)

macOS 开发需要 Go、Node/npm、Wails CLI、Xcode Command Line Tools。Wails 文档给出 Intel 开发 10.15+、Apple Silicon 11+，但当前 Go 下载页已标 macOS 13+；实际下限必须同时满足 Go、WebKit、前端构建目标与数据库驱动。不能直接将 Wails 历史最低版本写成产品兼容承诺。[Wails 安装](https://wails.io/docs/gettingstarted/installation/)、[Go 平台下载条件](https://go.dev/dl/)

项目 CPU 支持范围已确认（D11，2026-09-10）：首版支持 Windows x64（Intel/AMD）、macOS Intel（amd64）和 Apple Silicon（arm64）；Windows ARM64 不纳入首版。最低系统版本及 macOS 使用分架构包还是 Universal 包仍待确认。开发机选择较新系统，也不能替代最低支持系统测试。

以下命令仅供后续环境准备或验证执行：

```text
go version
go env GOOS GOARCH CGO_ENABLED
node --version
npm --version
go install github.com/wailsapp/wails/v2/cmd/wails@v2.15.0
wails version
wails doctor
```

macOS 补充：

```sh
xcode-select --install  # 安装动作，已有工具时无需重复执行
xcode-select -p
clang --version
```

安装后确认 Go bin 目录进入 PATH，重新打开终端；以 doctor 输出记录缺失项，勿把其他环境的示例输出当本机结果。[Wails 安装诊断](https://wails.io/docs/gettingstarted/installation/)

## 3. 目录与构建策略

建议沿用 Wails 根目录布局，核心 Go 包不依赖桌面运行时，避免未来变更入口时重写业务：

```text
main.go / app.go / wails.json   桌面入口及窄绑定
internal/                     代理、规则、运行、存储、应用服务
frontend/src/                 Vue/TypeScript
frontend/wailsjs/             生成绑定，不手工修改
migrations/                   数据库迁移
build/                        平台配置，bin 为构建产物
scripts/                      后续可复用的检查/打包脚本
docs/design/                  详细设计
```

提交 go.mod/go.sum、package.json/package-lock.json；工具版本另行集中记录。Node/npm 用于前端构建，不作为用户安装前提；发布嵌入前端静态资源。Wails build 默认输出 build/bin。[Wails 工作原理](https://wails.io/docs/howdoesitwork/)、[构建文档](https://wails.io/docs/gettingstarted/building/)

日常使用 wails dev；已有锁文件后在 frontend 执行 npm ci。正式构建通过 wails build，不跳过前端或绑定生成。建议 Windows 产物在 Windows 构建验收、macOS 产物在 macOS 构建验收；这是降低系统依赖风险的项目策略，不是声称所有交叉编译都不支持。CLI 有 windows/amd64、windows/arm64、darwin/amd64、darwin/arm64、darwin/universal 目标，具体启用项取决于支持矩阵。[开发模式](https://wails.io/docs/gettingstarted/development/)、[CLI 目标](https://wails.io/docs/reference/cli/)

## 4. 开发运行与对外分发

本地调试先验证可构建和可运行，不把购买签名证书作为开始开发的前提。发布准备单独处理：

- Windows：需要正式签名时准备签名身份和 Windows SDK SignTool；它支持签名、时间戳及验证。[Microsoft SignTool](https://learn.microsoft.com/en-us/windows/win32/seccrypto/signtool)
- macOS：对外分发按 Developer ID、必要 entitlements/hardened runtime、notarytool 公证和 stapler 流程验收；使用支持这些工具的 Xcode。Apple 已停止接受 altool 上传，不照搬旧示例。[Apple 分发说明](https://developer.apple.com/developer-id/)

包体先测普通 release 构建和前端依赖，不默认启用二进制压缩，也不写未经测量的 MB/内存指标。分发签名与代理 HTTPS 根证书是两类独立证书，不混用。

## 5. dev/release 验证矩阵

| 阶段 | Windows 与 macOS 都要验证 | 完成证据 |
| --- | --- | --- |
| 环境 | doctor、工具版本、架构和编译器 | 环境清单 |
| dev | Vue 类型检查、热更新、绑定调用、临时 SQLite 写读 | 命令结果与启动记录 |
| 核心 | Go 测试；设备运行隔离、严格步骤、完成透传和手动重置 | 可重复测试结果 |
| release | 实际用户包启动、WebView、资源/数据路径、退出恢复 | 对应平台验收记录 |
| 分发 | 安装/升级、签名、公证及最低系统启动 | 已签名包与校验结果 |

首个开发阶段的完成标准：两类 OS 至少各完成一次开发启动和普通 release 构建；代理最小链路和数据库驱动验证后冻结工具补丁版本。尚未具备 Mac 构建条件时，明确记录 macOS 未验收，不能用 Windows 结果代替。
