# Workflow 使用规范

## 单一配置源

当前工程 workflow 仅读取：

- `.workflow/config/workflow.yml`

`.spec-workflow.conf` 已弃用，不再生效。

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

> 根目录 `DESIGN.md/SPEC.md/TODO.md` 可作为工程长期文档保留，但不是 workflow 默认运行入口。

## 推荐流程

推荐顺序：

`design -> spec -> coding -> quality -> review -> test -> uat -> scan -> pr`

说明：`dispatch` 可直接进入任一 phase；团队协作时建议按推荐顺序推进。
