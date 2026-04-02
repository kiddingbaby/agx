# usecase

`internal/usecase/` 保存 AGX 的业务编排。
这里把 domain 规则、ports 和错误模型组合成用户可见能力。

当前服务：

- `key_service`
- `provider_service`
- `switch_service`
- `env_sync_service`

约束：

- usecase 只依赖 `domain + ports`
- 统一错误模型继续集中在这里，而不是散落到 CLI 或 adapters
