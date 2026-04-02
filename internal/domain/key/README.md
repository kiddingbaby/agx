# key domain

`domain/key/` 定义 key 的核心模型、校验与规则。
这里处理的是 key 命名、激活、约束与合法性判断。

约束：

- 密文存储和 AES-GCM 实现不放这里
- 业务规则保持与 `usecase/key_service.go` 的调用边界清晰分离
