# Delivery Learnback Block

适用场景：当一次实现已经产出 `release_plan` 与 `delivery_gate_result`，需要把交付经验反哺成可复用资产。

## Inputs

- `release_plan`
- `delivery_gate_result`
- optional: `workspace_handoff`
- optional: `task_execution_contract`
- optional: `task_execution_guardrail`
- optional: `review_loop_packet`

## Required Outputs

- `SKILL_CANDIDATE`
- `WORKFLOW_PATTERN`
- `ADOPTION_HINT`

## Rules

- 只抽取可复用规则、步骤与 adoption signal；不要重新实现 deploy/runtime。
- 若 `delivery_gate_result.status=blocked`，优先总结 blocker taxonomy 与 adoption guardrails。
- 若 `delivery_gate_result.status=passed`，优先总结 packet handoff、verification truth source 与 rollback anchor。
- 保持 `verification_source=task_execution_contract.verification_commands` 不变。
- 明确写出“不代表部署成功，只代表 contract completeness passed”。

## Suggested Structure

- `SKILL_CANDIDATE`: name / description / steps / rules / commands
- `WORKFLOW_PATTERN`: inputs / outputs / sequencing / acceptance
- `ADOPTION_HINT`: trigger phrase / when to use / when not to use / downstream consumer
