# AGENTS.md

仓库级 agent 协作约束。人类贡献者请看 [`CONTRIBUTING.md`](CONTRIBUTING.md)。

## 范围

agx 是一个 CLI，管理中转 profile 并把 profile 同步到 `codex` / `claude` / `gemini` / `opencode`，同时内嵌一个本地 MCP gateway（`agx mcp serve`）把四家 agent 共用一个聚合 endpoint。

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

| 文件 | 承载内容 |
| --- | --- |
| `README.md` | 项目概览、安装入口、最短上手、文档入口 |
| `docs/user-guide.md` | 用户命令与运行时参考 |
| `docs/cli-contract.md` | exit code / JSON 形状 / 稳定 key 集合 |
| `docs/doctor-issues.md` | doctor 能 emit 的每个 issue code |
| `docs/ARCHITECTURE.md` | 架构与扩展点 |
| `COMPATIBILITY.md` | 公开合同范围与 deprecation 流程 |
| `CONTRIBUTING.md` | 开发、验证、提交流程 |
| `AGENTS.md` | 仓库级 agent 协作约束 |

公开合同范围以 [COMPATIBILITY.md](COMPATIBILITY.md) 为准。

## 验证约定

- 优先使用 `task`
- 每次改动至少跑最接近的有效验证：
  - CLI 流程或原生配置同步 → `task verify`
  - 真实本机 agent CLI 兼容性 → `task verify:agents`
  - 改子命令树 → 再跑 `task docs:man` 重新生成 man 页

## 局部覆盖

只在子目录确实需要不同约定时，才新增更具体的 `AGENTS.md`。
