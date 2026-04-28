# AGENTS.md

## 范围

AGX 是一个 CLI，用来管理 relay profile，并把 profile 同步到 `codex`、`claude`、`gemini`。

## 事实来源

- 行为、路径、命令以代码、测试、`Taskfile.yml` 为准
- 结论建立在当前仓库和直接证据上
- 推断与事实分开表达

## 变更约定

- 优先交付最小、可回退、可验证的修改
- 公共 CLI 行为保持稳定，除非任务明确要求变更
- 结构整理优先拆文件、收窄职责、保持调用面稳定
- 让改动与验证保持接近

## 文档约定

- `README.md` 作为项目入口页，只承载项目定位、安装入口、文档入口
- `docs/user-guide.md` 承载日常使用与运行时行为
- `CONTRIBUTING.md` 承载开发与验证流程
- `AGENTS.md` 承载仓库级 agent 协作约束
- 子目录文档只在该子树存在独立兼容性、流程或接口约定时出现

## 验证约定

- 优先使用 `task`
- 每次改动至少运行最近的有效验证
- 改 CLI 流或原生配置同步：`task check`
- 改 JSON 输出或合同：再跑 `task cue`
- 需要验证真实本机 agent CLI 兼容性时：`task real-smoke`

## 局部覆盖

只有当某个子目录确实需要不同约定时，才增加更具体的指令文件。
