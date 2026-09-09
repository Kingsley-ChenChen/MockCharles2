# 05 数据库与版本设计

实现建议：SQLite 由 Go 单一应用所有者管理；迁移脚本后续实现。本阶段不创建数据库。项目并发作用范围仍受 D15 约束。

## 1. 实体与关系

| 实体 | 关键字段 | 约束/索引建议 |
| --- | --- | --- |
| projects | id, name, revision, created_at | id 稳定；预留后期共享归属 |
| datasets | id, project_id, name, schema_json, revision | 项目内查询索引；schema 强制性另行确定 |
| dataset_records | id, dataset_id, data_json, revision | 按集合分页；JSON 类型校验 |
| response_samples | id, project_id, source_flow_id, status, headers_json, body_json, revision | 样本正文独立保存，不依赖捕获永久保留 |
| mock_rules | id, project_id, name, matcher_json, standalone_enabled, active_response_id, revision | 独立启用字段不供组步骤判断 |
| response_revisions | id, mock_id, number, mode, status, response_headers_json, body_json, generation_json, provenance_json | UNIQUE(mock_id, number)，已提交版本不可变 |
| rule_sets / rule_set_members | set_id, mock_id, position, revision | 已确认每设备选择可复用规则集；顺序唯一并整体更新 |
| general_header_rules | id, project_id, matcher_json, request_ops, response_ops, position, revision | 作用域与生效时机受 D06 约束 |
| mock_groups | id, project_id, name, active_revision_id, revision | 组身份与组内容分离 |
| group_revisions | id, group_id, number | 一个完整组定义版本 |
| group_steps | id, group_revision_id, position, enabled, matcher_json, mock_id | 版本内 position 唯一；引用稳定 Mock ID |
| device_bindings | id, project_id, source_ip, mode, selected_group_id, selected_rule_set_id, revision | 同一有效路由作用域内 IP 唯一；selected_rule_set_id 已确认，项目作用域待 D15 |
| scenario_runs | id, binding_id, group_revision_id, generation, state, cursor, revision | 每绑定至多一个活动运行 |
| step_executions | id, run_id, step_id, request_id, response_revision_id, outcome, generation_id | request_id 可定位实际选用版本 |
| capture_policies | id, project_id, matcher_json, persist_enabled, revision | 不参与 Mock 是否启用 |
| flow_records | id, project_id, source_ip, started_at, route, status, request_body_ref, response_body_ref | (project_id, started_at, id)、(source_ip, started_at) |
| change_records | id, actor_kind, object_id, previous_revision, new_revision, time | 记录可信入口来源，不信客户端自报 |

这是一份关系设计，不是最终 SQL。大正文可选 SQLite BLOB 或本机正文文件仓储；选择由大小/保留测量决定。若选文件，必须有临时写入、原子归档、数据库引用及孤儿清理流程，不能让未提交数据库指向缺失文件。

## 2. 源数据与响应脱钩

编辑时从业务记录/抓包样本读取内容 → 构造完整模板 → 用户预览/保存响应版本。provenance 保存来源 ID、来源版本及必要描述。执行时只读响应版本，不执行记录查询。

删除/修改来源记录不会改变已保存响应。抓包清理不会使已提取的样本失效。跨项目引用须校验归属，不能只验证 ID 存在。

## 3. 并发编辑事务

保存响应建议在单事务完成：

1. 比较 mock_rules.revision 与 expectedRevision。
2. 校验配置并插入不可变 response_revisions。
3. 条件更新 active_response_id 与 revision。
4. 插入 change_records。
5. 提交后发布配置变更事件。

任何一步失败则全部回滚。两入口从同一版本编辑，只有一个成功，另一个收到 REVISION_CONFLICT；返回当前版本供对比，不最后写入覆盖。

保存组版本连同步骤整体提交；重排规则验证 ID 完整、无重复、归属一致，再统一更新顺序。不要逐条提交中间顺序，使请求看到临时错序。

## 4. 运行记录与持久化

运行逻辑由内存状态机处理并用 generation 隔离旧请求。是否持久化游标不改变“重启后手动开始”的已确认行为。若保存运行，启动将未正常结束记录标记为 interrupted，不恢复执行权。

已确认完整响应写出成功后才提交推进；生成或写出失败不推进。网络写出与 SQLite 事务无法组成原子事务：即便采用写出成功再提交，崩溃也可能发生在两者之间。文档和界面不得承诺跨崩溃 exactly-once。旧运行结束后手动开始新运行是可解释恢复边界。

执行状态不能依赖可丢弃的流量记录队列。实际正文是否保存和最小执行元数据保留由 D09 确认，关闭抓包不应意外仍永久保存全部敏感正文。

## 5. 删除、清理与迁移

引用删除策略待 D10。建议被组引用的 Mock 返回 REFERENCE_CONFLICT 和引用列表，避免级联删除组步骤或停止运行。历史版本在保留期内由执行记录引用，不能按“非活动版本”立即删除。

迁移使用 schema_version 顺序执行；迁移失败不启动代理写入，提供可读错误。升级前备份、备份位置/生命周期及用户可见恢复方式在分发设计中明确。索引/WAL/连接数量依并发实验调整，不能仅凭开启 WAL 就宣称没有写入争用。

## 6. 验收

并发 UI/AI 更新产生可复现版本冲突；删源数据后固定/随机模板仍可生成；组保存失败无半组；旧请求日志记录旧响应版本；关闭捕获仍可运行 Mock；模拟事务中断后数据库引用完整；重启不恢复旧运行。
