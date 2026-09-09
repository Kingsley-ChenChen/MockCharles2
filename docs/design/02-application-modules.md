# 02 应用模块与接口

状态：已确认产品约束下的实现建议。产品未决项统一见 [决策登记](decisions.md)。

## 产品模块到核心能力的映射

以 [四模块规划](../project-plan.md) 为准。设备管理调用 device/proxy 的接入、证书和绑定能力；规则管理调用 matching/headers/mock/generation/catalog；规则集管理调用规则集配置和 scenario；流量调用 traffic 的统一查询。后台技术包保持职责分离，不要求压成四个包。

流量查询建议统一接收 sourceIp（缺省为全部）、projectFilter（全部/未绑定/指定项目）、时间与其他条件。流式和目录式复用筛选及 flowId；目录聚合/子节点读取支持分页，不能用当前列表页拼出假完整目录。总数、游标和记录范围需要一致，正文仍独立读取。默认范围待 D18。

## 1. 部署与目录

交付为桌面应用。Go 核心与 UI 代码分层，运行不依赖用户部署远程服务器。系统 WebView 可有自己的进程，不能用“同一个应用”推导只有一个操作系统进程。

建议目录如下；当前仅为设计，不创建这些源文件。

```text
main.go / app.go      Wails 根入口与依赖组装
cmd/mcp-stdio/        连接已运行应用的薄适配器
internal/application/ 命令、查询、授权、事务编排
internal/proxy/       HTTP/TLS 传输与连接
internal/device/      来源 IP、设备选择
internal/matching/    编译和匹配原始请求
internal/headers/     请求/响应 Header 修改
internal/mock/        Mock 定义、响应版本
internal/generation/  固定/随机结果生成
internal/scenario/    组定义与运行状态机
internal/catalog/     样本与业务记录
internal/traffic/     捕获和流量查询
internal/storage/     SQLite 仓储与迁移
internal/transport/   Wails、MCP HTTP 适配器
frontend/src/         Vue3/TypeScript
```

领域模块不引用 Wails、MCP SDK 或具体 SQL 驱动；入口将框架对象转换为领域输入。生成器不读业务记录数据库，匹配器不修改 Header，存储层不决定是否推进步骤。

## 2. 核心契约

| 接口 | 输入 | 输出及不变量 |
| --- | --- | --- |
| ResolveDevice | 规范化实际来源 IP | 设备模式、绑定版本；未知 IP 透传 |
| CompileMatcher | 匹配定义 | 不可变匹配计划或字段校验错误 |
| Match | 原始请求视图、编译计划 | matched、条件结果；不得修改请求 |
| PlanRequest | 请求视图、设备状态 | 透传计划或 Mock 计划，携带规则/运行版本 |
| GenerateResponse | 响应版本、单请求随机上下文 | 状态码、多值 Header、正文；不推进运行 |
| ApplyHeaders | Header 列表、已选定操作 | 新 Header 列表；不改变匹配结果 |
| CompleteExecution | 预占标识、写出结果 | 执行结果与可选新游标；完整写出成功才推进；并发预占待 D04 |
| SaveMockResponse | mockId、expectedRevision、完整配置 | 新版本；原子提交 |
| ReadTraffic | 过滤条件、分页游标 | 摘要；正文独立按需读取 |

计划应包含 requestId、sourceIp、mode、bindingRevision、runId/generation/stepId、mockId、responseRevision、headerRevision、routeReason。日志依据实际计划，不能在完成后重新读取活动版本来猜本次配置。

## 3. 应用接口错误模型

建议统一结果对象：

```json
{
  "code": "REVISION_CONFLICT",
  "message": "对象已被另一入口修改",
  "details": { "expectedRevision": 7, "currentRevision": 8 },
  "requestId": "管理请求标识"
}
```

校验错误带 JSON 字段路径；权限错误不执行写入；版本冲突不自动覆盖。管理接口错误与手机收到的 HTTP 错误是两套封装，共用稳定错误分类而非相同协议状态码。

建议分类：VALIDATION_FAILED、NOT_FOUND、REVISION_CONFLICT、REFERENCE_CONFLICT、FORBIDDEN、APP_UNAVAILABLE、GENERATION_FAILED、UPSTREAM_FAILED。内部调用堆栈只写诊断日志，不作为手机 JSON 正文。

## 4. 并发与事件

每 IP 的运行拥有独立状态保护；网络调用、随机大列表生成不持有全局锁。运行推进不可放入允许丢弃的 UI 事件队列。配置提交后发布递增序号事件，UI 发现缺口重新查询权威快照；流量列表允许批量通知，正文不通过每条事件广播。

建议事件：DeviceObserved、BindingChanged、RunStateChanged、MockRevisionChanged、TrafficBatchAvailable。携带对象版本及作用域，前端丢弃早于已知版本的迟到事件。

SQLite 仓储注入 application；事务只覆盖必要持久化，不在事务中等待手机写出或上游响应。捕获落盘使用有界队列，容量策略待 D09，不能无限积压内存。

## 5. 生命周期

启动顺序建议：加载配置目录 → 数据库迁移 → 加载核心配置 → 管理入口就绪 → 界面就绪。已确认 D07：启动后不自动开启代理监听，用户手动开启后才监听。运行状态无论是否保存，重启均不得自动恢复为 running；设备选择恢复策略仍待确认。

关闭窗口触发一次幂等 Shutdown：禁止新管理写入和代理连接 → 按 D05 排空/取消在途请求 → 终止本机运行 → 结束写入 → 关闭监听与数据库 → 退出。退出不会远程修改手机 Wi-Fi 代理设置。

stdio 适配器无数据库所有权；连接不到桌面应用返回 APP_UNAVAILABLE。独立适配器存活不等于代理服务仍在运行。

## 6. 后期团队服务边界

本期数据保留 projectId、对象 ID、revision；管理用例可由未来 HTTP 服务复用。设备 IP、CA 私钥和运行游标是本机资源。共享业务记录与 Mock 配置需要另行设计同步、身份与冲突处理，不能简单把当前 SQLite 文件放到共享盘作为团队服务器。

## 7. 验收

同一份响应经 UI 和获准 MCP 操作更新，得到相同校验及版本结果；运行控制只有桌面可信入口可达；断开 UI 事件流后重连能恢复真实游标；停止应用后没有代理监听或第二个数据库写入所有者。
