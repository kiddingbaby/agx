# Project Prompt Entrypoint

本文件是项目提示词装配入口（给 Codex/Claude 共用）。

## READ_ORDER (locked)

1. `prompts/shared_base.md`
2. `prompts/project_patch.md`
3. `prompts/base.txt`
4. `docs/agent-language-policy.md`
5. `prompts/prompt_contract_v1.md`
6. `prompts/roles.yaml`
7. `docs/prompt-matrix.md`
8. `workflows/coding/<workflow>.yaml`

## WORKFLOW_SELECTION

- 新功能：`workflows/coding/implement_feature.yaml`
- 缺陷修复：`workflows/coding/fix_bug.yaml`

## ROLE_ACTIVATION

- 默认启用角色（7）：
  - `requirements_analyst`
  - `product_designer`
  - `solution_architect`
  - `planner`
  - `file_selector`
  - `executor`
  - `verifier`
- 按需启用角色：
  - `map_query`
  - `router`
  - `bug_hunter`
  - `skill_extractor`
  - `architecture_reviewer`
  - `referee`（默认禁用）

## OUTPUT_EXPECTATIONS

- 所有角色输出必须满足 Prompt Contract v1 固定块：
  - `STATUS`
  - `SUMMARY`
  - `ARTIFACTS`
  - `RISKS`
  - `NEXT_ACTION`
- `file_selector` 必须输出精确 `FILES:` 格式。

## FALLBACK_RULES

- 输入缺失：返回 `STATUS: BLOCKED`，并声明 `MISSING_INPUTS`。
- 路由不确定：回退 `planner` 收敛合同。
- map 不可用：`file_selector` 直接基于 contract 收敛最小文件集。
- shared 与 project 冲突：以 `prompts/project_patch.md` 为准。
