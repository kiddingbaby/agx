# config

`internal/config/` 保存路径解析与 secret provider。
它是 AGX 运行时环境约束的集中入口。

当前文件：

- `paths.go`
- `secret_provider.go`

约束：

- 配置解析逻辑集中在这里，不在各 adapter/usecase 里重复拼路径
- secret 生命周期继续由统一 provider 管理
