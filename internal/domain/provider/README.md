# provider domain

`domain/provider/` 定义 provider、site、target 与 binding 的核心模型和规则。
这里是 AGX 多 provider / 多 target 语义的纯规则层。

约束：

- provider 兼容性与约束在这里表达
- 配置持久化与同步仍留在 adapters
