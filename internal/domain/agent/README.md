# agent domain

`domain/agent/` 定义 AGX 支持的 agent family 与 registry 规则。
这里回答的是“AGX 能同步哪些 CLI”和“它们的基本能力模型是什么”。

约束：

- 只保存模型与 registry 规则
- CLI 配置文件的具体写入实现仍在 adapters
