# cmd

`cmd/` 保存 AGX 的进程入口。
这一层只负责 binary entrypoint 与退出码，不承载业务逻辑。

当前目录：

- `agx/`：主 CLI 可执行入口

约束：

- 业务编排必须继续下沉到 `../internal/`
- 新命令若需要共享逻辑，应先进入 interfaces/usecase，而不是在 `cmd/` 复制流程
