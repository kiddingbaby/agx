# Project Patch Prompt

本文件只存当前项目特有规则，覆盖 `prompts/shared_base.md` 的同主题内容。

## Repo Context（agx）

本仓库是 **agent env plane**（配置切换 + 全局资产同步）：

- 关键入口：`agx sync`（system prompts / skills / MCP）与 `agx use <site>`（切换 provider + 同步原生配置）
- 高风险写入：`~/.codex` / `~/.claude` / `~/.gemini`（必须幂等、可回滚、可 dry-run）

### Non‑Negotiables

- 禁止把 keys/tokens/私钥写进仓库与日志（包括测试快照）。
- 任何会写 HOME 的逻辑：必须具备 dry-run 路径、备份/回滚路径、以及可重复运行的幂等语义。
- 默认行为要保守：不自动启用可选资产（例如 MCP）除非配置显式开启。

### Fast Validation

- 单测：`go test ./...`
- 集成冒烟：`bash tests/integration/smoke-go.sh`

## Patch Scope

- 角色激活策略：默认 7 角色 + 按需角色
- workflow 选择：feature 与 bugfix 分流
- 输出协议：遵循 Prompt Contract v1 + FILES 精确格式
- 回退规则：路由冲突、map 缺失、输入缺失处理

## Project Defaults

- 默认 profile: `adaptive_split`
- 默认链:
  - `implement_feature`: `requirements_analyst -> product_designer -> solution_architect -> planner -> file_selector -> executor -> verifier`
  - `fix_bug`: `planner -> file_selector -> executor -> verifier`
- 按需角色:
  - `map_query`
  - `router`
  - `bug_hunter`
  - `skill_extractor`
  - `architecture_reviewer`
  - `referee`（disabled by default）

## Project Output Rules

- 必须遵循 `prompts/prompt_contract_v1.md`
- `file_selector` 输出必须为：
  - `FILES:`
  - `- path/to/file`

## Conflict Rule

当 `shared_base` 与项目规则冲突时：

1. 以本文件为准（project patch）
2. 再以 `prompts/base.txt`、`prompts/roles.yaml`、`docs/prompt-matrix.md` 校验一致性
