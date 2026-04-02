# internal

`internal/` 保存 AGX 的核心实现。
当前分层固定为：`interfaces -> app -> usecase -> domain + ports -> adapters`。

当前目录：

- `adapters/`：文件、CLI 配置、undo 等 IO 实现
- `app/`：bootstrap 与 container 装配
- `config/`：路径与 secret provider
- `domain/`：核心模型与规则
- `interfaces/`：CLI 接口层
- `ports/`：外部能力抽象
- `usecase/`：业务编排

约束：

- 不跳层依赖
- 代码树边界与 `docs/architecture.md`、`docs/directory-structure.md` 保持一致
