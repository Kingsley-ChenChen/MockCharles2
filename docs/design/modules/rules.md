# 规则管理模块规格

状态：详细设计草案。关联 [RUL-01/02/03](../../functional-analysis.md)。产品范围已确定；D06 保存生效已确认；D08 生成细则、D10 删除/AI、D14 匹配细则仍需按决策记录收口。

## 1. 所有权与页面边界

规则属于项目，接口规则在此定义匹配、固定/随机响应与专用 Header；通用请求/响应 Header 按项目管理（D20 已确认）。规则集仅引用接口规则和排列优先级，不拥有另一份响应正文，不选择通用 Header。

样本和业务记录是本模块的编辑材料。已保存响应是独立版本，源记录更新/删除不隐式改变响应。流量会话关闭不影响已提取样本。不得把完整流量历史藏在样本或规则诊断字段中自动持久保存。

## 2. 对象与字段

| 对象 | 字段草案 | 约束与空值 |
| --- | --- | --- |
| MockRule | id、projectId、name、note、matcher、standaloneEnabled、activeResponseId、revision | projectId 必须存在；响应引用属于本规则；排序不在规则对象中 |
| Matcher | method、host、port、path、conditions | 条件读取原始请求；无有效匹配配置不可启用；具体操作符 D14 |
| ResponseRevision | id、mockId、number、mode、status、headers、body/template、generation、provenance | 不可变完整版本；fixed 用正文，random 用模板/生成配置；mode 不控制设备模式 |
| HeaderRule | id、projectId、name、matcher、requestOps、responseOps、position、enabled、revision | D20 只有项目级作用域；不允许 ruleSetId 再增加覆盖层 |
| HeaderOperation | name、action、values | 多值列表；set/remove/append 的具体语义复用 04；remove 不需要值 |
| ResponseSample | id、projectId、name、content、sourceMetadata、revision | 内容独立拷贝；来源流量仅用于解释，不是读取正文的必要依赖 |
| BusinessRecord | id、datasetId、data、revision | 仅编辑阶段 CRUD/选取，不参与请求时业务查询 |

名称、备注与 JSON 中的业务字段不得混用。核心分配 ID/revision，客户端写入 expectedRevision。重复名称是否允许属于 UI 校验设计，不以名称代替引用 ID。业务 JSON 原值保持类型，不将字符串数字自动转数值。

## 3. 应用命令契约

操作名是拟定内部用例，错误统一为 [02](../02-application-modules.md) 的结构。输入携带稳定 ID，不以列表位置标识对象。

| 命令/查询 | 输入 | 结果 | 失败与事务边界 |
| --- | --- | --- | --- |
| rules.list/get | projectId、筛选/分页或 ruleId | 摘要页/完整定义 | NOT_FOUND 与空列表分别表示；正文按需 |
| rules.create | projectId、name、matcher、完整响应 | RuleView 与初始响应版本 | 编译/校验失败无半条规则 |
| rules.updateDefinition | ruleId、expectedRevision、name/note/matcher | 新定义版本及生效说明 | 条件非法定位字段；冲突不覆盖 |
| rules.saveResponse | ruleId、expectedRevision、完整响应草稿 | 新活动响应 ID、对象版本、next_request | 插入版本与切活动指针在同一事务；无效配置不发布 |
| rules.setStandaloneEnabled | ruleId、expectedRevision、enabled | 新版本及受影响集合摘要 | 不改变任何顺序步骤开关；D06 生效时点 |
| rules.references | ruleId | 优先集合、顺序定义、运行引用摘要 | 只读；可从 UI 跳到拥有者 |
| rules.delete | ruleId、expectedRevision | 删除结果/引用列表 | 引用处理 D10；不得隐式停止运行或删除步骤 |
| responses.preview | 完整草稿或 responseId | 实际生成结果、问题、预览身份 | 无运行状态写入；不持久录制为流量 |
| headers.list/save/reorder | projectId 或 headerId、版本、完整操作/顺序 | 已保存项目 Header 版本 | 多字段整体提交；无换行注入，无半更新顺序 |
| samples.save/get/delete | projectId/sampleId、内容、版本 | 独立样本版本或引用错误 | 不把“引用源流量”当成保存正文完成 |
| records.list/save/delete | datasetId/recordId、JSON、版本 | 业务记录版本 | JSON 非法明确拒绝；不级联改变响应快照 |

预览不要求对象先保存；编辑器必须显示“草稿预览”。保存与另一入口冲突后返回当前版本信息，UI 保留草稿供对比，不自动重试覆盖对方。

## 4. 匹配与 Header 执行

执行顺序：读取原始请求 → 选择设备绑定项目与模式 → 匹配独立规则或当前步骤 → 应用项目通用请求 Header → 命中接口专用请求 Header → 上游转发或生成 Mock → 项目通用响应 Header → 接口响应 Header → 协议必要规范化。

未绑定项目的新 IP 真实转发且不改写 Header。已绑定真实转发和顺序未命中/完成路径仍执行适用项目通用 Header。未解密隧道不进入 HTTP Header 处理。

匹配器不重新读取改写后的 Header。完整 Mock 不为改写请求头调用上游。响应版本里的基础 Header 与项目/接口操作要有明确合并顺序，不能出现第二次接口覆盖或合并 Set-Cookie 为单值。

D14 建议方案待确认：条件 AND，路径精确匹配优先，存在/不存在/等于和数值比较；重复键的取值语义与正则/模板路径首版范围在匹配控件定稿前明确。非法 JSON 与缺字段不等于 JSON null。

## 5. 响应与版本

固定响应返回已保存内容。随机响应使用保存模板，按请求独立生成，只随机配置字段；不同设备不共享可变随机上下文。单响应唯一值空间不足直接报错，不无限重试，不回退真实服务。

响应内容保存后下一请求使用新版本，在途使用旧版本。D05 已确认改绑或重置不主动中断已有请求；正在生成/写出的响应仍使用其已选版本，但不可推进替换后的运行。

D06 已确认 Header、独立匹配/启用/排序下一请求生效，顺序匹配与编排下次开始/重置生效，响应仍下一请求生效；保存结果按字段返回符合该操作的生效标识。界面并列显示活动版本与草稿，不把草稿渲染为当前手机结果。

## 6. 删除与权限

建议被引用规则拒绝删除并显示引用；先移除引用再删除，无级联删除和隐式停止。是否归档及 AI 对启用/排序/Header/组定义的编辑范围仍按 D10 收口。既有 AI 数据/规则 CRUD 继续保留，不能因权限未细化就全部降为只读。

可信入口决定权限，客户端请求 JSON 不可自报桌面身份。任何通用更新都拒绝 device binding、mode、run cursor 或自动开始参数。UI/AI 都必须使用相同业务校验和版本机制。

## 7. 状态与验证入口

编辑器覆盖未选择、加载失败、草稿有修改、校验失败、预览失败、保存中、保存成功、版本冲突、引用阻止删除。切换项目和列表不得静默丢失草稿；具体恢复方式在 D16/UI 评审确定。

可验证链路：真实响应建规则 → 保存固定内容 → 加入优先集合并绑定 → 命中 → 改响应 → 下一请求新内容；另一在途仍旧内容。通用 Header 和专用 Header 同名时专用生效，原始 Header 匹配不受改写影响；未绑定来源保持不改写。

随机生成的校验和预览使用同一核心生成器；生成失败不推进顺序步骤。材料删除不影响已保存响应。规则被两集合使用时，独立启用影响范围明确显示；组步骤开关独立。

未收口：D08、D10、D14、D16。接口命名和内部对象拆分可自主调整，产品语义需以相应答复为准。
