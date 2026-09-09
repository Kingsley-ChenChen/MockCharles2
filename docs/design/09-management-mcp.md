# 09 统一管理接口与 MCP

已确认目标：ChatGPT 桌面与 Codex、Claude Code、DeepSeek Harness 的 Mock 数据与规则 CRUD。采用 stdio 和 Streamable HTTP 适配同一管理核心；客户端事实与来源见 [AI 调研](../research/ai-integration.md)，SDK 环境见 [01](01-development-environment.md)。

## 1. 入口与能力边界

Wails 适配器、MCP 适配器负责协议转换，application 负责权限、校验、事务和版本。不要让 MCP 直接写 SQLite 或暴露任意 Go 方法调用。

手机代理端口只接收被调试流量；MCP HTTP 使用独立本机入口。stdio 适配器连接已运行应用，不能启动第二个代理/数据库核心。断开 AI 或重建协议连接均不改变手机 runId。

协议版本采用 SDK 支持范围并对实际客户端验证；当前最新协议与旧客户端的兼容不能只由“支持 MCP”四字推断。实现应跟踪工具能力，而非把 MCP 会话当作业务运行。

## 2. 共享命令草案

| 命令 | 必要输入 | 结果 | AI 首版 |
| --- | --- | --- | --- |
| mocks.list/get | projectId、过滤/分页或 mockId | 摘要/定义/版本 | 允许 |
| mocks.create | projectId、name、matcher、response | Mock 与 revision | 允许 |
| mocks.updateResponse | mockId、expectedRevision、完整响应配置 | 新响应及对象版本 | 允许，下一请求生效 |
| mocks.updateDefinition | mockId、expectedRevision、匹配/名称 | 新定义版本 | CRUD 内，匹配生效时机 D06 |
| mocks.delete | mockId、expectedRevision | 删除结果或引用冲突 | 允许 CRUD，引用处理 D10 |
| records.list/get/create/update/delete | datasetId、recordId、版本与 JSON | 记录/分页/冲突 | 允许 |
| samples.list/get/create/update/delete | projectId、sampleId、版本与内容 | 样本/冲突 | 允许，保留策略 D09 |
| responses.preview | 保存版本或草稿 | 结果、校验问题 | 不改变运行；供 AI 的暴露细节待工具评审 |
| ruleSets.save/reorder | setId、expectedRevision、orderedIds | 规则集版本 | 启用/排序修改权限 D10 |
| groups.save | groupId、expectedRevision、steps | 组定义版本 | 组定义编辑范围 D10，不能自动运行 |
| devices.list / runs.get | 项目/设备/运行 ID | 只读状态 | 建议只读，暴露范围待评审 |
| bindings.setMode/assign | 设备、版本、模式/选择 | 新绑定，停止旧运行 | 禁止 |
| runs.start/stop/reset | 设备/运行、expectedGeneration | 新运行状态 | 禁止 |

表中“允许”不等于可绕过未决引用限制。AI CRUD 正常写入已保存配置，不因为禁止运行控制就强制只创建草稿。运行控制参数不得混入 records/mocks/import 等通用入口。

## 3. 更新请求与结果

```json
{
  "mockId": "mock-order-detail",
  "expectedRevision": 7,
  "response": {
    "mode": "fixed",
    "status": 200,
    "headers": [{"name": "Content-Type", "values": ["application/json"]}],
    "body": {"code": 0, "data": {"orderId": "O-FIXED-001", "state": "ready"}}
  }
}
```

建议结果包含 mockId、revision、responseRevision、effectiveFrom="next_request"。只在该操作确实具备此生效语义时返回 next_request；组顺序更新应返回 next_run，不共用误导性字段。

版本冲突返回 currentRevision 和可供读取当前对象的 ID；AI 必须重新读取再决定修改，不能服务端自动重试覆盖。校验返回字段路径，未知字段拒绝以防静默忽略控制字段。

## 4. 身份与权限

可信 actor 来自入口注册和连接认证，不能由请求 JSON 指定 actor="desktop"。MCP 不注册运行控制工具；核心同样检查能力，防止未来适配器误暴露。

通用更新禁止 mode、binding、selectedGroup、cursor、run.state、generation 等运行字段。导入/批量也不得包含“导入后自动开始”；删除引用配置不能级联执行停止运行。允许的响应修改会影响下一请求，这是已确认能力而非权限绕过。

建议本机管理 HTTP 采用应用生成凭据、回环监听及连接校验；凭据由本机配置安全交给适配器，不输出到流量记录或模型工具结果。远程暴露、团队身份及隧道接入另行设计；桌面客户端本机接入不需要为了兼容默认公开公网服务。

## 5. MCP 工具封装

工具采用明确名称、输入 Schema、字段描述与结构化错误。分页使用游标和上限，批量读取正文按需进行，避免一次 list 将整个抓包库送进模型上下文。

协议错误（无效工具、参数不符）与领域错误（版本冲突、引用、生成失败）区分。SDK 工具注解只辅助客户端，不作为服务端权限保证。模型回复“已修改”不能替代后端返回保存版本。

首版 CRUD 联调闭环：读取样本 → 创建 Mock → 查询 → 更新响应 → 从界面核对版本 → 手机请求确认实际内容 → 删除无引用 Mock。绑定和开始由用户在桌面完成。

## 6. 验收矩阵

逐个目标客户端测试 stdio/HTTP 实际支持的组合、版本协商、列表/读/创建/更新/删除、错误显示、断线重连。只测试一个客户端不宣称其余通过。

额外验证：MCP 工具列表无运行控制；用 AI 身份直接调用核心控制命令仍拒绝；通用 update 夹带运行字段拒绝；版本冲突无覆盖；关闭应用后 stdio 适配器返回 APP_UNAVAILABLE；重连不重置组游标。
