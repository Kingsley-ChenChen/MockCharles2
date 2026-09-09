# Windows/macOS 桌面技术栈调研

检索日期：2026-09-09。状态：供用户确认的候选方案，不是技术选型定案。优先目标已确认为运行高效、安装包尽量小。本次依据官方文档和候选库维护者资料，没有实施代码或进行性能测试。

用户已确认采用 Wails v2 + Go + Vue 3/TypeScript + SQLite；关闭窗口即退出并停止代理，重启后手动开始组运行。下文保留选型对照与证据，候选比较不再表示技术栈等待选择；代理库仍需实机验证。

## 已确认的需求边界

- 首期 Windows/macOS 本地桌面应用，后期考虑团队共享。
- 自研手机 App、H5 接入 HTTP/HTTPS 代理，以 JSON 接口为主。
- 固定响应、每次请求随机生成、请求和响应 Header 改写。
- 多设备共用同一代理端口，按实际来源 IP 分别绑定不同 Mock 组，各自保存运行步骤。
- Mock 组严格匹配当前步骤，其他请求转发真实服务；最后一步完成后也转发真实服务，用户手动重置后才重新运行。通用 Header 修改仍应用于这些真实转发请求。
- 抓包样本和业务记录只在编辑阶段作为数据来源，不在每次代理请求时查询源记录。
- AI 使用 MCP 管理 Mock，UI 和 AI 共用应用服务。

这些是已确认产品边界，不是第三方框架能力声明。

## 推荐保留两个主要组合

| 候选 | 组合 | 体积方向 | 集成与维护取舍 |
| --- | --- | --- | --- |
| A，优先验证 | Wails v2 稳定线 + TypeScript Web UI + Go 业务/代理模块 | 系统 WebView，不随应用分发完整 Chromium | UI 桥接与核心都用 Go，首期可同进程；若要求故障隔离，再拆独立 Go 后台 |
| B | Tauri 2 + TypeScript Web UI + Go sidecar | 系统 WebView，Go 程序作为额外发行文件 | 桌面 Rust 桥接与 Go 核心分工清楚，但多一套语言工具链和进程通信 |

两者符合“先减少随包运行时”的方向，但目前没有实测数据证明谁的最终安装包更小、谁的代理吞吐更高。实际结果取决于 WebView 分发方式、数据库驱动、前端资源、签名、压缩和代理依赖。前端已确认 Vue 3/TypeScript。

### A. Wails + Go：优先验证的理由

Wails v2 应用是标准 Go 程序，可以嵌入前端资源并将 Go 方法绑定为 JavaScript 可调用方法。因此代理和应用服务可以作为普通 Go 模块放入同一宿主，通过窄接口让桌面 UI 调用，不必为了 UI 框架额外引入 Rust 或另起 Go sidecar。前半段为官方事实，后半段是本项目的集成建议。[Wails 工作原理](https://wails.io/docs/howdoesitwork/)

Wails 使用平台原生 WebView；Windows 使用 WebView2，macOS 使用系统 WebView。使用系统渲染组件符合缩小随包运行时的目标，但仍需要验证目标 OS 的 UI 差异和 Windows WebView2 安装条件。[Wails v3 官方介绍](https://v3.wails.io/blog/wails-v3-beta/)、[Wails v2 安装说明](https://wails.io/docs/gettingstarted/installation/)

**版本必须区分：检索时 v2 文档显示 v2.15.0；官方明确 v2 仍是稳定线并继续修复。v3 已进入 Beta，桌面 API 有稳定性承诺，但不是最终 3.0 GA。** 因此不能把 v3 写成稳定版，也不应沿用“v3 仍是 Alpha”的旧判断。[v2 文档](https://wails.io/docs/howdoesitwork/)、[v3 Beta 发布说明](https://v3.wails.io/blog/wails-v3-beta/)、[v3 当前状态](https://v3.wails.io/status/)

建议先用 v2 稳定线验证本项目的单窗口主界面和后台生命周期。若需要 v3 的多窗口、系统托盘或其他能力，再明确评估采用 Beta 的维护成本。v2 → v3 是窗口/应用生命周期、绑定和运行时 API 的迁移，官方不把它描述成只改版本号即可完成。[v3 Beta 发布说明](https://v3.wails.io/blog/wails-v3-beta/)

同进程并不要求代理阻塞界面：业务模块保持异步事件和有界队列，UI 渲染只接收摘要及按需正文。另一方面，Go 宿主级故障可能同时影响代理和桌面生命周期；如果“代理在 UI 故障后继续工作”是硬需求，就将同一 Go 核心包编译为独立后台。进程隔离是一项待确认选择，不能假定同进程天然满足它。

### B. Tauri 2 + Go sidecar：系统 WebView 与独立引擎

Tauri 官方通过 `bundle.externalBin` 支持打包任意语言的 sidecar，并提供 Rust/JavaScript 启动方式。各 OS/架构需要对应 target-triple 二进制；从 JavaScript 启动还需配置 capability 权限。[Tauri sidecar](https://v2.tauri.app/develop/sidecar/)

Tauri 在 Windows 使用 WebView2，在 macOS 使用 WKWebView/WebKit；macOS WebView 随系统更新。因此 JSON 编辑器、虚拟列表、快捷键、复制粘贴和文件操作都需要跨平台验证。[Tauri WebView](https://v2.tauri.app/reference/webview-versions/)

Windows 安装文档提供 WebView2 下载引导、离线安装和固定运行时等选项。离线分发是否必须携带运行时，会改变最终包体，不应拿最小空壳包与包含完整依赖的包比较。[Tauri Windows Installer](https://v2.tauri.app/distribute/windows-installer/)

本项目判断：Tauri + Go sidecar 的进程分隔从首期就明确，后续提取团队后台也自然；代价是 TS/Rust/Go 工具链、两套可执行生命周期、后台版本握手和 UI/后台通信。若选择 Wails 同进程，则少了其中的 Rust 和自定义进程协议；若 Wails 也拆 sidecar，两者后台通信成本会趋近，主要差异回到 Go 与 Rust 桌面桥接生态。两者均未进行性能测量。

## Electron 的位置：功能生态备选

Electron 主进程运行 Node.js，UI 使用 Chromium 多进程渲染。它适合已有 Electron/TS 经验、需要一致 Chromium 行为的团队；但随应用分发 Electron 运行时，与本次“安装包尽量小”的优先目标相比，暂不列入前两个主推荐。[Electron 进程模型](https://www.electronjs.org/docs/latest/tutorial/process-model)、[Electron 打包](https://www.electronjs.org/docs/latest/tutorial/application-distribution)

若后续选择 Electron + Go，使用 Node `child_process.spawn()` 启动 Go 可执行程序。`utilityProcess.fork()` 用于 Node 模块入口，不能直接把 Go 二进制当作 JS 脚本传入。固定参数数组、避免 shell，Windows 隐藏辅助控制台；打包工具不会替我们自动解决 Go 二进制的 OS/架构匹配。[Node 子进程](https://nodejs.org/api/child_process.html)、[Electron utilityProcess](https://www.electronjs.org/docs/latest/api/utility-process)

## Go 与 mitmproxy/Python 代理路径

外壳与代理内核分开决策。以下是代理层比较，不增加第三个主桌面推荐。

| 路径 | 已核实能力 | 本项目仍需实现 |
| --- | --- | --- |
| Go + net/http + goproxy 候选库 | net/http 提供 HTTP 客户端/服务端；goproxy 声明 HTTP 代理、CONNECT、HTTPS MITM、请求/响应钩子、自定义 Transport | 证书生命周期、设备隔离、规则、严格步骤、存储、管理 API、协议验收 |
| mitmproxy + Python addon | request/response 事件；官方示例支持 Header 修改和直接返回响应；已有代理协议工具链 | 自有 UI/API、设备和流程状态、产品打包、依赖升级适配 |

来源：[Go net/http](https://pkg.go.dev/net/http)、[goproxy 维护者仓库](https://github.com/elazarl/goproxy)、[mitmproxy Addons](https://docs.mitmproxy.org/stable/addons/overview/)、[mitmproxy 示例](https://docs.mitmproxy.org/stable/addons/examples/)。goproxy 是第三方库，不是 Go 官方完整 MITM 产品。

### Go 路径

在采用 Wails/Go 或 Tauri/Go 时，让代理、编排和应用服务共享 Go 模块，可以避免每次请求跨语言同步调用。这是模块边界建议，不代表 Go 语言自动保证更高吞吐。

不能从 net/http 的 HTTP/2 能力推导某个 MITM 库上下游 HTTP/2 全链路兼容。本次 goproxy README 提供了 CONNECT/MITM 钩子证据，但不足以承诺目标 App 的所有协议协商都成功。应以 iOS/Android 真实网络库测试决定采用何种代理库。[net/http](https://pkg.go.dev/net/http)、[goproxy](https://github.com/elazarl/goproxy)

不建议自行从零实现 TLS 或 HTTP 解析；代理适配层负责协议，产品模块负责请求匹配、响应生成和按设备推进运行步骤。

### mitmproxy/Python 路径

mitmproxy 提供 Windows/macOS 安装路径，预编译包包含 Python 环境和相关依赖；额外 Python 包可按官方 PyPI 方式安装。此路径可利用现成代理能力，但需要纳入发行依赖与更新维护，不能假定目标用户已经安装 Python。[安装说明](https://docs.mitmproxy.org/stable/overview/installation/)

官方协议页记录 HTTP/1、HTTP/2、HTTP/3 等支持及限制，例如 h2c、某些回放与 QUIC 兼容限制。因此 mitmproxy 也不能承诺所有手机协议都可透明处理。[协议说明](https://docs.mitmproxy.org/stable/concepts/protocols/)

在包体优先、核心偏 Go 的前提下，Python 运行环境会增加一套要分发和维护的依赖，因此当前更适合作为抓包链路的验证工具或 Go 候选不能满足协议要求时的替代。如果正式采用，业务规则尽量和 addon 用清晰的同进程模块边界，或由编辑端发布不可变快照，避免每条请求 Python → Go → 数据库来回调用。该判断需最终打包和真实请求测量确认。

addon API 有公开的不兼容变更记录，升级需要固定版本并运行回归测试。[Addon API 变更](https://docs.mitmproxy.org/stable/addons/api-changelog/)

## 两个主要组合共用的核心结构（建议）

```text
桌面宿主（Wails Go / Tauri Rust）
    │
Web UI ── 窄业务接口 ── 应用服务 ── 本地数据库
                          │
AI MCP / 管理 REST ────────┤
                          └── 发布规则快照
                                  │
手机/H5 ── 代理监听 ── 设备识别 ── 该设备当前组/运行
                                  │
                      当前步骤命中 → 生成 Mock → 推进
                      其他请求 / 已完成 → 真实服务
```

- 业务核心不依赖 Wails/Tauri API。桌面桥接、REST、MCP 是调用同一应用服务的适配层。
- 数据库由核心模块统一管理，UI/MCP 不直接操作数据库文件。
- 每台设备分别关联 groupId/runId/currentStep/status，不能用整个应用的一个全局步骤游标。
- 最后一步完成后将该设备运行标为 completed；此后全部透传，直到用户手动重置。
- 样本/业务记录在编辑阶段转换为激活快照；命中时基于快照生成固定或随机结果。
- 单设备运行的“校验步骤 → 形成响应决策 → 推进”有原子边界；不同设备可以独立处理。取消、断连和业务重试是否重复消费，仍需明确产品规则。
- 代理数据事件采用有界队列、分页与按需正文，避免把所有流量无限送入 WebView。
- UI 管理入口与手机代理监听分离；手机接入不自动获得编辑 Mock 的权限。
- Wails 首期可同进程组织；Tauri 则放入 Go sidecar。后续团队部署复用核心包和管理契约，另行设计身份、共享与冲突处理。

以上是架构建议，不是框架内置功能。

## 定案前的最小验证

1. 用户确认 A/B 偏好；前端框架、是否接受 Wails v3 Beta、是否必须进程隔离分别确认。
2. 明确 Windows x64/ARM64、macOS Intel/Apple Silicon 的首期范围、最低 OS 和离线安装需求。
3. 使用真实 App/H5 验证证书信任、CONNECT、HTTP 协商、压缩、重复 Header、超时和取消。
4. 验证多设备不同组、同一步并发、完成后透传、手动重置、随机响应和双向 Header 改写。
5. 检查安装/升级、后台异常退出、端口占用、系统代理恢复和证书管理；代码签名/公证单独验收。
6. 在同样规则、抓包量和 JSON 文档下测量包体、冷启动、空闲内存、持续抓包以及编辑延迟，再确定性能和包体目标。

**当前建议优先验证 Wails v2 + Go + TS UI，以 Tauri 2 + Go sidecar 为第二候选。** 这是针对已确认的小包体目标和 Go 核心边界给出的建议，用户尚未确认最终框架。没有引用框架宣传中的 MB、内存或启动耗时作为本应用承诺。

## 检索记录

- Wails：v2 页面标 v2.15.0；v3 Beta 发布说明更新于 2026-09-08，状态页更新于 2026-09-06。已打开正文，确认 v2 稳定、v3 Beta。
- Tauri：v2 sidecar、WebView 和 Windows 安装正文。
- Electron：latest 进程、utilityProcess、打包正文；Node 子进程官方 API。
- 代理：Go net/http、goproxy 主仓库、mitmproxy stable addon/安装/协议/变更正文。
- 本次未锁定最终依赖版本，也未实测协议、跨平台打包、性能或安装包大小。
