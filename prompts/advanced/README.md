# 高阶提示词待办

本目录用于后续扩展生命周期角色（默认不启用）：

- `release_manager`
- `ops_observer`
- `postmortem_analyst`

启用策略：

- 先在 `prompts/roles.yaml` 注册并设置 `enabled: false`
- 确认 engine 已具备消费路径后再切换为 `true`

当前版本（v1）仅保证“需求/产品/技术方案 + coding/quality/learning”闭环。

当前新增的复用块：

- `delivery_learnback_block.md`：从 `release_plan / delivery_gate_result` 提炼 `SKILL_CANDIDATE / WORKFLOW_PATTERN / ADOPTION_HINT`
