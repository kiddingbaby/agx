# AGX Docs Index

## 阅读顺序

1. `docs/architecture.md`：当前已落地架构与调用链
2. `docs/directory-structure.md`：当前目录基线与依赖约束
3. `docs/key-orchestration-v2.md`：多 Provider/多 Key 核心设计（含 Codex env_key）
4. `docs/key-manager.md`：Key Manager TUI 交互与快捷键（含 Vim 两态）
5. `docs/refactor-roadmap.md`：重构执行结果与回归门禁
6. `.workflow/docs/{DESIGN,SPEC,TODO}.md`：workflow 运行文档与执行记录
7. `.workflow/config/workflow.yml`：workflow 单一配置源

> `.workflow/` 仅用于本仓开发流程记录，不属于 `agx` runtime 的公开接口或配置输入。

## 回归命令

```bash
make test
make smoke
```
