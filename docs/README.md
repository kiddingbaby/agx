# AGX Docs

## 阅读顺序

1. `README.md`：产品定位 + 快速开始 + 常用命令
2. `docs/product.md`：产品视角（用户旅程与最短路径）
3. `docs/user-guide.md`：用户操作指南（按场景最短路径）
4. `docs/manual-checklist.md`：手动验收清单（覆盖多 provider / 多 key 关键路径）
5. `docs/config-format.md`：配置文件格式（kebab-case 规范）
6. `docs/ux.md`：UI/UX 交互设计（当前与 vNext）
7. `docs/architecture.md`：当前已落地架构与调用链
8. `docs/ops.md`：落地/运维（回归与 E2E 冒烟）
9. `docs/troubleshooting.md`：故障定位（常见问题）
10. `docs/security.md`：密钥/配置与信任边界
11. `docs/upgrade.md`：版本演进与迁移提示
12. `docs/directory-structure.md`：当前目录基线与依赖约束
13. `docs/workflow.md`：workflow 命令、phase 流程与配置源
14. `docs/refactor-roadmap.md`：重构路线图与回归门禁

## 视角索引

- 产品视角：`README.md`（把 AGX 当作“keys/配置/同步的统一入口”来理解）
- 技术视角：`docs/architecture.md`、`docs/directory-structure.md`
- 运维/回归：`docs/ops.md`、`docs/troubleshooting.md` + `tests/` 目录脚本

## 回归命令

```bash
go test ./...
bash tests/integration/smoke-go.sh
```
