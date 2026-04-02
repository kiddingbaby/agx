# cli

`internal/interfaces/cli/` 是 AGX 的命令行接口层。
这里负责 root command、子命令装配、输入解析、错误映射与用户视图。

当前关注面：

- root/status/use/undo
- site 与 key CRUD
- apply/init/sync
- 视图与 help flags

约束：

- 参数解析与用户输出留在这里
- 业务决策继续调用 `../../usecase/`
