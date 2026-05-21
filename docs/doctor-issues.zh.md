# Doctor Issue 目录

`agx doctor` 用稳定的机器可读 `code` 值标记问题。本目录是脚本应该匹配的
权威参考——`message` 文本和 `action` 字符串可能在版本间调整，但 `code`
遵循 [兼容性策略](../COMPATIBILITY.zh.md)。

测试 `internal/usecase/doctor_issue_catalog_test.go` 保证本文件与运行时
emit 一致：加新 code 没文档化 / 文档化的 code 已被删除——任一一种都会
让测试失败。

English: [doctor-issues.md](doctor-issues.md).

## 严重性

| Severity | 含义 |
| --- | --- |
| `error` | 出问题了，不修 agx 不会按预期工作。 |
| `warning` | 可恢复 / pre-1.0 过渡态。建议处理但非必须。 |

## Codes

| Code | Severity | 含义 | 怎么修 |
| --- | --- | --- | --- |
| `unfinished_operation` | error | 上一次 mutation 命令（add / edit / use / activate / restore …）中途崩溃，journal 还指着它。 | 按建议跑 `agx restore <agent>`。restore 成功后 agx 会自动清掉 journal 条目。 |
| `missing_bound_profile` | error | 某 agent 的 native binding（`state.<agent>.SourceProfile`）指向一个 store 里已不存在的 profile。 | 要么 `agx add <profile>` 重建，要么 `agx detach <agent> <profile>` 清掉悬空 binding。 |
| `invalid_binding_status` | error | 某 agent 的 binding status 既不是 `applied` 也不是 `bound`。 | 重新 `agx use <profile>`（或 `agx detach` 清掉）。 |
| `unconfigured_relay` | error | profile 在 store 里但没绑给任何 agent。 | 要么 `agx run <agent> <profile>`（顺便绑），要么 `agx rm <profile>`（如果是 stale）。 |
| `orphan_derived_profile` | error | 派生 profile（`codex.foo` 等）在 store 但没 managed target 引用。 | 按建议跑 `agx rm <derived>`。 |
| `orphan_managed_target` | error | managed target 指向一个不存在的 profile / 派生 profile。 | 按建议跑 `agx detach <agent> <name>`。 |
| `model_id_drift` | error | OpenCode 绑定的 profile 的 `model` 与 managed target 上记录的 model 不一致。 | 跑 `agx edit <profile> --model <id>` 重新 sync target。 |
| `invalid_restore_mode` | error | backup 条目的 `restore_mode` 是 agx 不认识的值（合法值只有 `restore_file` / `remove_created_file`）。 | 该 backup 不可用；手动从 state 移除或用 `agx backup <agent>` 重做。 |
| `missing_backup_path` | error | backup 条目没有 `backup_path` 字段，restore 没东西可读。 | 同上——丢条目或换成新的 `agx backup <agent>`。 |
| `missing_backup_file` | error | backup 条目的 `backup_path` 指的文件已不存在。 | 丢条目；还需要回滚点的话再 `agx backup <agent>` 一次。 |
| `runtime_binding_missing` | error | agent 的磁盘 config 没有 AGX-managed 块，但 state 认为应有。 | 对该 agent 重新 `agx use <profile>`。 |
| `runtime_binding_conflict` | error | agent 的磁盘 config 指向另一个 profile（有人手编辑了、或 agent CLI 自己改了）。 | 决定哪边对，然后 `agx use <profile>` 对齐。 |
| `runtime_binding_incomplete` | error | AGX-managed 块在，但缺必要字段。 | 对该 agent 重新 `agx use <profile>`。 |
| `runtime_config_unreadable` | error / warning | 原生 config 文件解析失败。**warning**：还有 v1 managed target 存在（过渡态）；否则 **error**。 | 修文件（纯 TOML / JSON / dotenv）或删掉让 agx 重建。 |

## 脚本化

```bash
# 有任何 error 严重性的 issue 就退出非零
agx doctor -o json | jq -e '.issues | all(.severity != "error")' >/dev/null

# 列所有 action 字符串
agx doctor -o json | jq -r '.issues[].action'

# 只看某个 code
agx doctor -o json | jq '.issues[] | select(.code == "orphan_managed_target")'
```

`agx doctor` 当且仅当存在 error 严重性 issue 时退出非零。只有 warning
的报告退出 `0`。
