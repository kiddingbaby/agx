# Workflow 使用规范

## 单一配置源

当前工程 workflow 仅读取：

- `.workflow/config/workflow.yml`

`.spec-workflow.conf` 已弃用，不再生效。

## 根目录约束

本项目不接受任何 wf 生成产物落在仓库根目录。  
wf 相关产物必须全部位于 `.workflow/`（例如 docs/state/archive/reports/tasks）。

以下路径在本项目视为不允许出现：

- `workflow.config.yml`
- `tools/workflow/gate.py`
- `.github/workflows/workflow-gate.yml`
- `.spec-workflow.conf`

## 最小配置原则

项目侧不做复杂配置，直接使用 skill 默认模板值：

- `core.docs.*` 默认指向 `.workflow/docs/{DESIGN,SPEC,TODO}.md`
- `feature.uat.plan_file` 默认 `.workflow/docs/UAT.md`
- 其余 quality/uat/security 开关沿用模板默认

需要修改时，只改这一个文件并执行：

```bash
bash ~/.claude/skills/workflow/scripts/workflow.sh config lint
```

## 文档位置（最新）

workflow 运行文档统一在：

- `.workflow/docs/DESIGN.md`
- `.workflow/docs/SPEC.md`
- `.workflow/docs/TODO.md`
- `.workflow/docs/UAT.md`

## 推荐流程

推荐顺序：

`design -> spec -> coding -> quality -> review -> test -> uat -> scan -> pr`

说明：`dispatch` 可直接进入任一 phase；团队协作时建议按推荐顺序推进。

## 清理产物（P1）

`.workflow/archive/` 会随 workflow 运行持续增长；建议定期清理。

默认仅 dry-run（不删除）：

```bash
bash scripts/cleanup-workflow.sh
```

实际删除（危险操作，显式开启）：

```bash
bash scripts/cleanup-workflow.sh --apply
```

可选参数（覆盖默认保留策略：14 天 + 最多 10 个 archive）：

```bash
bash scripts/cleanup-workflow.sh --keep-days 14 --max-archives 10
```
