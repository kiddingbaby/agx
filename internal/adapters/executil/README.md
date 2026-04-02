# executil

`executil/` 提供命令执行的基础适配。
这一层负责把外部命令调用封装成可替换的 adapter 能力。

约束：

- 只提供执行原语，不嵌入 CLI 业务分支
- 若调用策略变化，应经由 `../../ports/command_runner.go` 向上暴露
