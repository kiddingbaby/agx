# integration

`tests/integration/` 保存集成级 smoke。
这里主要验证 Go 构建与关键命令链在当前仓状态下可运行。

当前脚本：

- `smoke-go.sh`

约束：

- integration 关注构建链与跨包拼接
- 更细粒度规则仍由包内 `_test.go` 承担
