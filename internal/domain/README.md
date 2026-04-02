# domain

`internal/domain/` 保存 AGX 的核心模型与规则。
domain 不依赖 usecase、adapters 或 interfaces。

当前目录：

- `agent/`
- `key/`
- `provider/`
- `session/`

约束：

- 规则要保持纯粹、可测试
- 只有在产品边界真的变化时，才允许扩 domain 语义
