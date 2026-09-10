# AI Agent 接入调研

检索日期：2026-09-09（Asia/Shanghai）。范围：让 ChatGPT、Claude Code、DeepSeek Harness 对本工具的 Mock 数据和编排执行增删查改。本文件是有来源的设计输入，建议项尚待用户确认；没有实施、联网暴露服务或修改任何客户端配置。

用户后续已确认：DeepSeek Harness 即官方 deepseek-ai/deepseek-harness；ChatGPT 指桌面 App 和 Codex；当前本地使用、团队共享后期实现。下文网页、公开插件分发与 Tunnel 属于后期参考，非首期必需；具体客户端版本仍待确认。

第二轮补充已确认：本工具为 Windows/macOS 桌面应用，要求性能高效、安装包尽量小；业务记录在编辑时生成模板，首版支持固定响应与每次请求动态随机（列表数量及字段），执行时不访问源业务记录。多设备可选择不同组；当前步骤未命中以及组完成后都走真实服务，手动重置才重新运行。请求/响应 Header 都可 Mock，不需要手动暂停。

最新确认：技术栈采用 Wails v2 + Go + Vue 3/TypeScript + SQLite。关闭桌面窗口即停止代理和本机管理执行；MCP 适配器应连接该应用实例，不能自行启动一套独立 Mock 数据库或在应用退出后延续运行。新 IP 默认真实转发，手动绑定；未绑定项目不改写 Header，见已确认 [D20](../design/decisions.md)。

用户最终确认的首版权限：AI 可增删改查 Mock 数据和规则；不可切换设备模式、绑定组、开始/停止/重置运行。这些运行控制由桌面界面执行，后续再考虑开放。下文候选工具已按此收窄。

## 结论

建议采用“统一应用服务 + MCP 适配器（stdio、Streamable HTTP）+ REST 管理 API”，需要脚本自动化时增加 CLI。MCP 是这些客户端目前有共同支持证据的工具接入协议；REST 是产品自己的稳定控制接口。两者共同调用业务服务，避免实现两套规则、权限和数据写入逻辑。这是架构建议，不是协议强制要求。

截至检索日，MCP 官方 `latest` 实际重定向到 **2026-07-28**。不能再把 `2025-11-25` 写为最新规范。新规范涉及实质性协议变化，兼容实现仍应覆盖目标客户端实际支持的旧版本，不能仅依照最新版本假定三款客户端已经全部支持。[MCP 当前规范](https://modelcontextprotocol.io/specification/2026-07-28)

## 1. MCP 版本与传输

| 项目 | 核实结果 | 本项目影响 |
| --- | --- | --- |
| 最新规范 | 官方 latest 指向 2026-07-28；变更页对照前版 2025-11-25 | 设计文档区分规范版本与客户端版本 |
| stdio | 客户端启动本地子进程，用标准输入输出交换消息 | 适合本机桌面客户端和 Claude Code |
| Streamable HTTP | 单一 MCP endpoint，以 POST 请求返回 JSON 或请求范围的 SSE | 适合长驻本地服务、团队服务和远程接入 |
| 旧 HTTP+SSE | 已被弃用；Streamable HTTP 内仍可使用 SSE | 不应把“SSE 被弃用”理解为所有 SSE 流都不能用 |

来源：[传输总览](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports)、[Streamable HTTP](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)、[版本变更](https://modelcontextprotocol.io/specification/2026-07-28/changelog)。

2026-07-28 移除了协议层会话和 `initialize` 握手，改为请求携带协议版本与能力；新增 `server/discover`。需要跨调用状态时使用业务参数携带的服务端句柄。编排运行必须拥有自己的 `runId`、游标和业务上下文，不能依赖 MCP 连接会话保存流程状态。最后一句是基于规范变化的设计推论。[版本变更](https://modelcontextprotocol.io/specification/2026-07-28/changelog)

实现建议：选用明确支持新旧版本兼容的官方 SDK，固定并验证版本；以 `tools/list`、`tools/call` 的普通 CRUD 为最低共同能力。资源、提示模板、MCP Apps UI、异步 Tasks 等按客户端能力增量开启。最新规范把 Tasks 放在可选扩展层；第一版业务编排不需要依赖客户端实现 Tasks 扩展。[规范及扩展](https://modelcontextprotocol.io/specification/2026-07-28)

## 2. 客户端兼容矩阵

| 客户端/入口 | 官方证据中的能力 | 推荐连接方式 | 尚需确认 |
| --- | --- | --- | --- |
| ChatGPT 桌面应用，本地主机配置 | 与同一 Codex host 的 CLI、IDE 共享 MCP 配置；支持 stdio、Streamable HTTP | 本机 stdio 或受认证的 loopback HTTP | 用户安装的具体客户端、版本及使用模式 |
| ChatGPT 网页 Developer mode | 可接入远程 MCP；支持读和写工具；提供逐工具开关 | 远程 HTTPS，或 Secure MCP Tunnel | 账户权限、工作区策略 |
| ChatGPT 网页 Chat/Work 插件 | 使用插件提供的远程 MCP 工具；不读取本地 Codex 配置 | MCP 插件打包作为后续分发层 | 是否要求插件分发、公开上架 |
| Claude Code | 本地 stdio；远程 HTTP；旧 SSE 仍可配置但官方建议 HTTP | 本机 stdio 或 HTTP | 最低兼容版本、管理策略 |
| DeepSeek Harness（已确认官方 dsh） | 通用 MCP 客户端支持 stdio、Streamable HTTP，发现并暴露工具 | stdio 或 HTTP | 预览版具体版本 |

OpenAI 来源：[ChatGPT/Codex MCP](https://learn.chatgpt.com/docs/extend/mcp)、[Developer mode](https://developers.openai.com/api/docs/guides/developer-mode)。Claude 来源：[Claude Code MCP](https://code.claude.com/docs/en/mcp)。DeepSeek 来源：[通用 MCP 接入说明](https://deepseek-harness.github.io/deepseek-harness/en/guide/mcp-memory)。这张表是文档兼容性，尚未进行实际端到端连接测试。

### ChatGPT 的读写、私有网络和产品名称

Developer mode 官方文档当前列出网页 Pro、Plus、Business、Enterprise、Education 账户，并明确支持读写工具。写动作默认要求确认；`readOnlyHint` 用于识别只读工具。无需为 CRUD 人为设计 `search`/`fetch` 两种工具限制。账户可用性仍受实际工作区策略约束。[Developer mode](https://developers.openai.com/api/docs/guides/developer-mode)

检索时原 Apps SDK 的连接页重定向到 Plugins 文档。应区分：MCP 负责调用协议，插件负责打包分发，UI 是可选层。实现 Mock CRUD 不必先开发聊天内界面。[插件连接和测试](https://developers.openai.com/plugins/deploy/connect-chatgpt)

**私有服务现有官方 Secure MCP Tunnel 方案。** 内网运行 `tunnel-client`，通过出站 HTTPS 转交 MCP 请求；上游可以是 stdio 或 HTTP 服务，不必公开本地入站端口。需要 tunnel 身份、运行凭据及组织/工作区关联与权限。普通聊天数据仍会进入选用 AI 服务的调用路径，隧道解决可达性，不代表数据不离开本地。[Secure MCP Tunnel](https://developers.openai.com/api/docs/guides/secure-mcp-tunnels)

官方插件测试文档接受公共 HTTPS 或 Secure MCP Tunnel；公开插件提交仍要求公共 HTTPS endpoint。这是“内部自用”和“公开分发”的不同要求，不应先要求用户为本地 Mock 工具部署公网服务器。[插件连接和测试](https://developers.openai.com/plugins/deploy/connect-chatgpt)

### Claude Code

Claude Code 文档支持命令添加 HTTP、stdio 服务，并支持项目 `.mcp.json` 配置；HTTP 可携带认证头，也支持 OAuth。文档还包含 WebSocket 选项，但纯 Mock CRUD 没有采用它的必要，stdio/HTTP 的兼容面更合适。[Claude Code MCP](https://code.claude.com/docs/en/mcp)

### DeepSeek Harness 与 DeepSeek API

已找到由 DeepSeek 官方 API 文档链接的 **DeepSeek Harness（dsh）**，仓库为 `deepseek-ai/deepseek-harness`，当前标记 developer preview，明确可能有不兼容变化。用户已通过仓库链接确认就是这个产品。[DeepSeek API 首页](https://api-docs.deepseek.com/)、[DeepSeek Harness README](https://github.com/deepseek-ai/deepseek-harness/blob/master/README.md)

该 Harness 的 `@deepseek-ai/dsh-mcp-client` 是通用 MCP 客户端。官方文档明确可把示例配置用于其他 MCP 服务；stdio 子进程随插件生命周期启动/停止；HTTP 上游需要自行运行。配置目录中提供 `transport: 'stdio' | 'streamable-http'`，HTTP 支持 `headers`。不能仅凭此承诺任意 OAuth 浏览器授权流程已兼容。[接入说明](https://deepseek-harness.github.io/deepseek-harness/en/guide/mcp-memory)、[客户端配置](https://deepseek-harness.github.io/deepseek-harness/en/reference/config-catalog)

如果用户实际指自建 DeepSeek Agent，API `tools`/function calling 也是可选方案：模型输出函数名和参数，宿主执行操作并回传结果，模型本身不会执行函数。官方 strict schema 功能目前仍标 Beta。API 工具调用能力与某款客户端能否导入 MCP 是两个独立问题。[DeepSeek Tool Calls](https://api-docs.deepseek.com/guides/tool_calls/)

## 3. 各种入口的分工（设计建议）

| 方案 | 本项目适用范围 | 成本或限制 |
| --- | --- | --- |
| MCP | 多种现有 Agent 客户端发现并调用 Mock 工具 | 需要客户端配置、认证、协议兼容测试 |
| REST + OpenAPI | UI、插件桥接、外部系统、自动化脚本的稳定管理入口 | 仅有 REST 不等于所有聊天客户端自动识别 |
| CLI | 本地 Agent、开发者、CI 执行明确命令，支持 JSON 输入输出 | 需要宿主允许进程运行与本地可达性 |
| Function calling 适配器 | 自建 Agent，或没有 MCP 支持的宿主 | 自己负责模型调用循环、工具执行、认证和错误恢复 |
| 插件/Skill | 配置分发、操作说明、常用流程模板 | 是体验与分发层，不应承载唯一的业务实现 |

推荐结构：

```text
ChatGPT / Claude Code / DeepSeek Harness
                  │
          MCP stdio / HTTP 适配层
                  │
UI ── REST ── 统一应用服务 ── 数据库及编排运行时
                  │
           CLI / Function adapter（可选）
```

MCP 服务处理管理请求，不进入每条被代理 HTTP 请求的实时关键路径。AI 创建、检查、修改 Mock 规则和场景，确定性的代理/编排引擎执行规则。这样 AI 延迟和不可用不会拖慢页面业务流程。

## 4. 初版工具能力建议

工具名称是建议契约，尚未冻结：

- 查询：`list_projects`、`list_endpoints`、`list_mocks`、`get_mock`、`get_scenario`、`get_run`。
- 编辑：`create_mock`、`update_mock`、`delete_mock`；支持 response body、status、request/response headers 等结构化字段。
- 配置组合：`create_mock_group`、`update_scenario`、`validate_scenario`；不附带激活、绑定或启动副作用。
- 首版不向 AI 暴露：切换设备模式、绑定组、`activate_group`（若其行为是激活运行）、`start_run`、`stop_run`、`reset_run`；这些控制留在桌面界面。
- 辅助：`create_mock_from_capture`、`preview_response`、`explain_match`。

建议所有业务状态通过 `projectId`、`groupId`、`scenarioId`、`runId` 明确指定，不使用“当前选中项目”作为 AI 写操作的隐式目标。分页与字段选择限制抓包正文回传量。更新接受 `expectedRevision` 防止覆盖界面中的并发修改；写请求接受 `idempotencyKey`，避免网络重试重复创建。AI 运行控制已确认首版不允许；具体引用中对象的删除策略和规则启用字段仍需在 CRUD 契约中明确。

权限在应用服务执行，不依赖模型遵守提示词或工具注解。首版除不注册运行控制工具外，共享服务还要拒绝 AI 身份的对应命令及通用写接口中的运行控制字段；调用身份来自可信适配层，不能由 AI 请求体伪造。代理端口、管理端口分开：移动设备连接代理不自动获得 Mock 数据管理权。本地 HTTP 绑定 loopback 并校验 Origin；远程入口单独鉴权。规范明确要求 Origin 校验并建议本地仅监听 localhost、对连接做适当认证。[Streamable HTTP 安全要求](https://modelcontextprotocol.io/specification/2026-07-28/basic/transports/streamable-http)

## 5. 待确认与后续验证

1. 已确认官方 dsh；具体版本/安装包仍待确定。
2. 已确认 ChatGPT 桌面 App 和 Codex；实际版本及权限设置待确定，网页非首期目标。
3. 已确认 Windows/macOS 本地桌面应用、后期团队共享；后期同步方式待确认。
4. 已确认 AI 保留数据/规则 CRUD，首版不提供切模式、绑定、运行启停/重置；规则启用字段与引用中配置的删除语义待细化。
5. MCP 首期要兼容哪些实际客户端版本？最新规范不等于这些客户端都已完成新协议迁移。

技术验证建议：同一组 Mock CRUD 合约依次通过 MCP Inspector、三类客户端和 REST 调用；覆盖分页、未知 ID、版本冲突、超时重试、未授权写入和并行修改。用同一条实际请求确认 UI 与 AI 编辑结果产生一致的响应。协议测试和业务编排测试分别执行。

## 检索记录与限制

- MCP：打开 latest、2026-07-28 主规范、变更及传输正文；最新入口已指向 2026-07-28。早期搜索摘要仍频繁返回 2025-11-25，已按实际正文纠正。
- OpenAI：打开官方 Developer mode、ChatGPT Learn MCP、Plugins 连接测试、Secure MCP Tunnel 正文。记录的是检索时在线文档，没有获得用户账户或客户端版本的实测结果。
- Claude Code：打开官方 MCP 正文，包含多个 2.1.x 行为版本注记；没有把文档中的某个注记推定为用户安装版本。
- DeepSeek：通过 API 官方站追到 Harness 官方仓库与文档，读取 MCP 接入和配置正文；Harness 为 developer preview，文档来源是在线页面及 master README，未固定发布版本或提交哈希。
- 本次是架构前置调研；没有实际部署 MCP server，也没有端到端验证客户端对 2026-07-28 全部能力的支持。
