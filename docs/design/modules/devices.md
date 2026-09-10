# 设备管理模块规格

状态：基于已确认功能的详细设计草案。D07 已确认保存设备配置且不自动监听/续跑；D20 已确认项目 Header。D05 已确认改绑/重置允许旧请求完成但不推进新运行；证书重建/移除与应用退出排空仍需细化。本文件不声称代码已经实现。

关联功能：[DEV-01-01 至 DEV-02-05](../../functional-analysis.md)。代理协议见 [03](../03-proxy-devices-tls.md)，存储见 [05](../05-storage-versioning.md)，顺序运行见 [07](../07-scenario-runtime.md)。

## 1. 职责与依赖

设备模块维护来源 IP、备注和配置绑定；代理提供真实观察事件，证书组件管理本机 CA，运行引擎拥有步骤状态。页面不能直接修改数据库或游标。查询设备是应用级查询，不受“查看项目”暗中限制；可显式筛选实际绑定项目。

后台以规范化 IP 作为本机代理内来源键，不含远端临时端口，不读取客户端 Header 推断设备身份。IP 改变创建新来源；同一 IP 的多个终端不能独立配置。不承诺自动检测 IP 已换到另一台手机。

## 2. 数据契约草案

| 字段 | 类型/空值 | 来源与校验 | 保存/更新 |
| --- | --- | --- | --- |
| sourceIp | 非空字符串 | 核心规范化的 IPv4/IPv6；UI 不改来源键 | 保存，主身份 |
| label | 字符串，可空字符串 | 用户备注；不解析为 IP 或规则条件 | 保存；独立修改不改绑定 |
| projectId | ID 或 null | 非空必须存在 | 保存；null 表示未绑定 |
| mode | pass_through / standalone_mock / scenario | 未绑定只允许 pass_through | 保存；新来源默认 pass_through |
| linkedRuleSets | 关联列表：associationId、setKind、setId | 每个引用须属于 projectId，同一设备/定义关联唯一 | 持久保存，多对多关系 |
| lastSelectedAssociationId | 关联 ID 或 null | 若非空必须仍有关联 | 保存上次启用选择，停止不删除 |
| activeAssociationId | 关联 ID 或 null | 同时至多一个，类型与 mode 一致 | 保存；null 对应真实转发；顺序运行状态单独管理 |
| revision | 非负整数 | 核心生成，不接受客户端任意新值 | 保存；修改带 expectedRevision |
| firstSeenAt | UTC 时间 | 首次实际观察 | 保存 |
| lastSeenAt | UTC 时间或 null | 最近实际观察；不当成心跳 | 由核心维护；落盘频率属实现策略 |
| runSummary | 只读对象或 null | 运行引擎查询，含 runId/state/currentStep | 不作为可写设备字段 |

上述字段为拟定 DTO；持久化可拆 device_sources 与 device_bindings，不能因 UI 一行而把运行游标混成可编辑绑定。备注与绑定使用同一版本或独立版本均可，但接口需保持明确冲突反馈，不覆盖另一入口更新。

## 3. 命令与查询草案

统一失败结果沿用 [02](../02-application-modules.md)：code、message、details、requestId。下面的名称是内部接口建议，最终桌面绑定从同一应用用例生成。

| 操作 | 输入 | 成功输出 | 关键校验/失败 |
| --- | --- | --- | --- |
| devices.list | 可选 projectFilter、sourceIp、cursor、limit | items、nextCursor；含实际绑定和最近活动 | limit 有界；查询失败不回退为假空列表 |
| devices.get | sourceIp | DeviceView | 不存在返回 NOT_FOUND |
| devices.setLabel | sourceIp、label、expectedRevision | 新 DeviceView | REVISION_CONFLICT 保留草稿 |
| bindings.assign | sourceIp、projectId、mode、两个选择 ID、expectedRevision | 新 DeviceView 与运行变更摘要 | 不存在、跨项目、模式缺选择拒绝；变更整体提交 |
| proxy.getState | 无 | stopped/starting/listening/stopping/failed、实际地址、错误 | 只读真实监听状态 |
| proxy.start | 已校验监听地址、端口 | 实际状态与端点 | 地址不可用/端口占用返回明确错误，不显示已开启 |
| proxy.stop | 无 | 实际停止状态 | 停止完成语义依 D05；不能只改按钮状态 |
| certificates.get | 无 | absent/available、公共指纹、有效期、下载状态 | 无私钥内容；不返回设备信任推测 |
| certificates.generate | 无或受控生成选项 | 公共证书元信息 | 已有 CA 不隐式覆盖；重建另行设计 D21 |
| certificates.download | 当前公共证书标识 | 导出结果/受控下载入口 | CA 不存在、写入失败、入口不可达分开表达 |

移除设备的命令暂不冻结：须先确定活动运行处理与历史保留，不把“删除设备”实施成级联清理。开始/停止/重置运行复用运行模块命令，仅接受设备及运行版本，不接受 UI 自填游标。

## 4. D07 已确认的恢复行为

启动加载 IP、备注、项目、模式及已选规则集/顺序定义，恢复为上次保存配置。代理保持 stopped；运行不恢复为 running。

手动开启代理后：pass_through 真实转发；standalone_mock 按保存集合匹配；scenario 显示已选定义但等待用户手动开始，未开始的请求真实转发。所有已绑定项目的可处理 HTTP 路径应用该项目通用 Header；未绑定来源不改写 Header（D20）。

存储中出现失效引用时不得猜测另一集合或项目；返回不可用配置及可见修复状态，具体数据修复策略在迁移/删除设计中说明。

## 5. 事件与在途边界

DeviceObserved 更新来源与最近活动；BindingChanged 在持久提交后携带 sourceIp/revision；RunStateChanged 来自运行引擎；ProxyStateChanged 来自监听生命周期。UI 丢弃旧版本事件，断线重连查询快照。

每个请求在接收时读取绑定快照。改绑不修改已选定响应版本，不允许旧运行推进新运行。D05 已确认旧请求继续按已选定计划完成，不因改绑/重置主动取消；普通网络超时不变。停止代理或退出应用的排空时限单独细化，不让进程无限等待。

已确认切出顺序模式或改绑其他组停止旧运行，新运行手动开始。项目切换浏览不发布 BindingChanged。UI 不用前端先加游标来掩盖核心事件延迟。

## 6. UI 状态输入与验收

设备页显示：监听状态、证书元信息、来源 IP/备注/最近活动、实际项目/模式/规则集、运行摘要。证书下载成功只表示已下载，不显示“设备已信任”。未配置、加载失败和无设备分别显示。

验收：应用重启后的配置与保存值一致、未监听且未续跑；开启后独立模式使用保存集合；顺序模式仍待手动开始。两 IP 的备注/绑定独立；更新一个设备失败不改另一个。跳转流量携带 sourceIp；未绑定设备请求真实转发且 Header 不被项目规则改写。

剩余设计入口：D05 停止代理/退出排空时限，D21 CA 下载/重建与移除记录。相关决定前可完成字段、只读查询、备注及恢复规格，不能宣称整个设备模块设计已冻结。

UI 评审决定 D22：运行控制属于设备，规则集页只编辑定义和查看关联设备，不直接开始/停止/重置。D23 已确认多关联、单个启用；界面布局通过不等于所有关联语义均已冻结。

## 7. 设备侧关联与运行（D22/D23 已确认）

设备可关联多个优先集合/顺序定义，均须属于设备绑定项目；只能有一个实际启用。规则集侧只读查看关联设备，不有全局运行。候选下拉选择只是前端草稿；执行启用后原子替换 activeAssociationId、mode、lastSelectedAssociationId，不能先停后启产生半保存状态。

设备侧操作：管理关联、启用所选规则集、停止设备规则，以及当前顺序运行的开始/重置。优先集合启用后按首匹配路由；顺序定义在设备侧手动开始。停止设备规则使本设备真实转发、停止旧运行，但保留关联及上次选择；不关闭共享代理、不影响其他设备。已绑定项目的通用 Header 继续生效。

关联保存与启用分开：devices.saveAssociations 接收 IP、expectedRevision、完整类型化关联列表，不自动启用新增集合；devices.activateRuleSet 接收 IP、associationId、expectedRevision，校验项目、关联、定义有效性，提交后新请求使用新配置。devices.deactivateRuleSet 只停止当前设备规则；顺序 runs.reset 的归属仍是设备。命令通过同一 application 核心，不给规则集定义增加 running 字段。

候选更改不改变实际启用。跨项目改绑清除不合法关联，旧在途遵循 D05；是否保留历史关联另行设计，不让跨项目引用生效。正在启用的关联建议先停止再移除，避免“移除关联”隐式切模式；该删除保护属于 D10 具体交互建议。

D07 恢复语义扩展到已关联列表、上次选择及实际启用配置；代理仍不自动监听。优先集合在手动开启监听后可使用保存的实际启用配置；顺序即使保存为已选配置仍不得自动续跑。
