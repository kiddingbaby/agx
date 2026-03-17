# Agent Language Policy（语言策略 v1）

## 目标

- 默认中文输出，保证交付可读与协作一致。
- 保留必要英文（代码、命令、协议键名、专有名词），避免语义漂移。
- 在多代理/多阶段链路中保持稳定语言，不因角色切换而抖动。

## 语言配置（推荐）

1. 系统层：固定主语言

- 明确声明：`默认使用中文（简体）`。
- 明确例外：代码、命令、路径、API 字段、错误码保留原文英文。

1. 合同层：输出结构不变

- 结构键名保持英文（如 `STATUS/SUMMARY/ARTIFACTS`）。
- 字段内容使用中文；必要术语可中英并列一次，后续统一简称。

1. 角色层：防止语言漂移

- 在角色提示词中增加“沿用上游语言约束，不自行切换”。
- 若输入为英文材料：先中文摘要，再保留关键英文引用。

1. 工作流层：单轮一致性

- 单轮内（plan -> execute -> verify）语言必须一致。
- 复审/对抗审查可更严格，但不改变主语言。

## 输出规范

- 默认：中文句子 + 英文技术标识（命令、路径、字段名）。
- 不翻译：`JSON key`、`CLI flags`、`代码符号`、`协议名`、`错误码`。
- 允许保留英文的场景：避免歧义或行业约定术语（如 `fail-fast`、`idempotent`）。

## 质量门禁

- 新增 preflight：检查提示词是否声明主语言与例外规则。
- 回归检查：同一任务多轮输出中，主语言不可来回切换。
- 验收建议：抽样检查合同产物、运行摘要、review 结论的语言一致性。

## 参考实践（2026-03-05 检索）

- Anthropic Prompt Engineering（强调明确指令与输出约束）：
  - <https://docs.anthropic.com/en/docs/build-with-claude/prompt-engineering/overview>
- Anthropic System Prompts（角色/行为边界前置）：
  - <https://docs.anthropic.com/en/release-notes/system-prompts>
- OpenAI Prompt Engineering Guide（固定格式与分隔符、明确任务边界）：
  - <https://platform.openai.com/docs/guides/prompt-engineering>
- LangChain Multi-agent（多代理上下文与职责边界）：
  - <https://docs.langchain.com/oss/python/langchain/multi-agent>

## 本仓库落地建议

1. 在 `prompts/project_entrypoint.md` READ_ORDER 中纳入本文件。
2. 在 `scripts/check_prompt_assembly.py` 中增加语言策略检查项。
3. 在合同模板中保留英文结构键名，但将说明文本默认中文化。
