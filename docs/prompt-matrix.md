# Prompt Matrix（角色/交接矩阵）

## 目标

定义角色输入、输出、触发与交接，保证 workflow 可串联，同时控制默认复杂度。

## 默认链路

- `implement_feature`:
  - `requirements_analyst -> product_designer -> solution_architect -> planner -> file_selector -> executor -> verifier`
- `fix_bug`:
  - `planner -> file_selector -> executor -> verifier`

## Adaptive Split（按需插入）

- `map_query` 自动触发（任一满足）:
  - 合同 `Context files` > 6
  - `file_selector` 首轮不能收敛到 `<=10 files`
  - 预估改动跨越 `>=2` 个顶层目录
- `router` 自动触发（任一满足）:
  - 同一合同存在多个可行 skill 路线
  - 文件集合跨越多个 skill 边界
  - 主路由置信度不足

## Review Loop（按需启用）

- `review_loop_orchestrator` 触发（任一满足）:
  - 明确要求“做完后子代理审核，再修复，再对抗复审”
  - 高风险合同需要闭环验收
  - 优先级：显式触发词 > 高风险自动触发
- 接口参数:
  - `contract`（必填）
  - `max_rounds`（默认 3）
  - `strict`（默认 true）
  - `adversarial`（默认 true）
- `reviewer_strict`:
  - 代码质量深度审核（Blocking 优先）
- `architecture_reviewer`:
  - 架构合理性审查（简化与边界优先）
- `redteam_reviewer`:
  - 修复后对抗式复审（边界/恶意输入/规约冲突）
- 默认轮次:
  - `max_rounds=3`
  - 退出条件：`Blocking=0` 且合同 Commands 全绿
  - 升级条件：超轮次 / 重复阻断 / Commands 不可复现
  - 机读规范：`docs/review_loop_v1_spec.yaml`
  - 状态映射：`success | command_failed | repeated_blocking_signature | continue_next_round | max_rounds_reached`

## Role Matrix

| role_id | mode | stage | required_inputs | key_outputs | handoff_to |
| --- | --- | --- | --- | --- | --- |
| requirements_analyst | default | upstream | requirements.md | REQUIREMENTS_BRIEF | product_designer |
| product_designer | default | upstream | REQUIREMENTS_BRIEF | PRD_SLICE | solution_architect |
| solution_architect | default | upstream | PRD_SLICE | TECH_SPEC | planner |
| planner | default | coding | requirements.md | CONTRACTS | file_selector |
| file_selector | default | coding | contract | FILES | executor |
| executor | default | coding | contract,files | CHANGES | verifier |
| verifier | default | quality | contract | COMMAND_RESULTS,FINAL_VERDICT | done |
| map_query | on_demand | coding | contract | MAP_SNIPPET | file_selector |
| router | on_demand | coding | contract,files | WORKFLOW_DECISION | executor |
| bug_hunter | on_demand | quality | contract | FINDINGS | verifier |
| skill_extractor | on_demand | learning | run_logs | SKILL_CANDIDATE | planner |
| referee | on_demand(disabled) | quality | findings | ADJUDICATION | verifier |
| review_loop_orchestrator | on_demand | quality | contract | ROUND_PLAN,EXIT_CRITERIA,ESCALATION_RULE | executor |
| reviewer_strict | on_demand | quality | contract,files | VERDICT,BLOCKING_FINDINGS,MINIMAL_PATCH_PLAN | executor |
| architecture_reviewer | on_demand | quality | contract,files | VERDICT,ARCHITECTURE_FINDINGS,MINIMAL_ARCH_PLAN | executor |
| redteam_reviewer | on_demand | quality | contract,findings | VERDICT,BLOCKING_FINDINGS,REDTEAM_SCENARIO,BASELINE_READY | verifier |

## 校验规则

- 所有角色输出必须满足 `prompts/prompt_contract_v1.md` 固定块。
- `file_selector` 必须输出 `FILES:` 精确格式。
- 输入缺失时必须返回 `STATUS: BLOCKED`。
- 默认流程不启用 `map_query/router/bug_hunter/skill_extractor/referee`，仅在触发条件命中或显式请求时启用。
- 默认流程不启用 `review_loop_orchestrator/reviewer_strict/architecture_reviewer/redteam_reviewer`，仅在显式触发时启用。
