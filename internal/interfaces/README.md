# interfaces

`internal/interfaces/` 保存 AGX 的接口层。
当前唯一接口是 CLI。

当前目录：

- `cli/`

约束：

- interface 层负责参数解析、错误展示与视图输出
- 它不直接依赖 adapters 的具体实现
