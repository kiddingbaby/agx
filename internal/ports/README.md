# ports

`internal/ports/` 定义 AGX 对外部能力的抽象接口。
usecase 只依赖这些接口，不直接依赖具体 adapter。

当前接口：

- command runner
- key repository
- provider config repository
- secret provider
- tool config syncer
- undo store

约束：

- 新外部依赖先落 ports，再补 adapter
- port 名称应反映业务意图，而不是底层技术细节
