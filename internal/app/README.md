# app

`internal/app/` 负责应用装配。
这里把 paths、secret provider、repositories、services 和 CLI root 组装起来。

当前文件：

- `bootstrap.go`
- `container.go`

约束：

- 这里只做 wiring，不放业务规则
- 若依赖图变化，应保持与 `docs/architecture.md` 的启动链一致
