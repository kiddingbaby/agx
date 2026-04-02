# adapters

`internal/adapters/` 保存 AGX 的基础设施实现。
这一层通过 `ports/` 向上暴露能力，负责落地文件读写、命令执行、原生 CLI 配置同步和 undo 存储。

当前目录：

- `configfile/`
- `executil/`
- `keyfile/`
- `tmux/`
- `toolconfig/`
- `undofile/`

约束：

- adapter 只做 IO/系统实现，不承载业务规则
- 若需要新增外部能力，先定义 `../ports/`，再在这里落实现
