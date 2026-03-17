# Prompt Contract v1

## 1) 目标

统一本仓库所有角色提示词的输入占位符、输出结构和失败语义，确保可机读、可串联、可审计。

## 2) 语言规范

- 结构字段键名: English (固定)
- 字段内容: 中文
- 禁止随意改键名；如需扩展，先升级契约版本。

## 3) Placeholder 白名单

仅允许以下占位符（按角色按需使用）：

- `{contract}`
- `{map_snippet}`
- `{date}`
- `{files}`
- `{findings}`
- `{run_logs}`

如果缺少必需输入，角色必须输出 `STATUS: BLOCKED` 并声明 `MISSING_INPUTS`。

## 4) 全角色固定输出块（必填）

- `STATUS:` `OK | BLOCKED | FAILED`
- `SUMMARY:`
- `ARTIFACTS:`
- `RISKS:`
- `NEXT_ACTION:`

## 5) 失败语义

- `BLOCKED`:
  - 必填 `MISSING_INPUTS` 或 `CONFLICTS`
  - 不得伪造结果
- `FAILED`:
  - 必填 `CAUSE`
  - 必填 `RETRY_HINT`

## 6) 约束

- 不得泄露 secrets/token/env。
- 不得越权修改合同 OUT scope。
- 不得弱化测试以求通过。

## 7) 兼容规则

- `prompts/file_selector.txt` 必须保留精确格式：
  - 第一行: `FILES:`
  - 后续: `- path/to/file`
