# tests

`tests/` 保存 AGX 的 shell-level smoke/integration suites。
Go 包内单元测试仍和源码同目录共置；这里主要覆盖跨目录、跨入口的回归链。

当前目录：

- `e2e/`
- `integration/`

约束：

- shell suites 只做跨层验证，不替代包内单元测试
- 若新增用户路径或关键集成点，应同步补这里的 smoke 覆盖
