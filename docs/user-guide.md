# 用户指南

English: [user-guide.en.md](user-guide.en.md).

## 概览

agx 围绕中转 profile 提供四个核心动作：

- 用 `add` / `edit` / `rm` 管理全局 profile
- 用 `use` / `current` 选择默认 profile
- 用 `run <agent>` 在隔离上下文里启动原生 CLI
- 用 `doctor` / `backup` / `restore` 维护可回滚状态

## Commands

### Profile

```bash
agx add <name> --base-url <url> --api-key <key> [--model <id>]
agx edit <name> [--name <new>] [--base-url ...] [--api-key ...] [--model ...]
agx rm <name>
agx ls
agx show <name>
agx use <name>
agx current
```

### Launcher

```bash
agx run codex
agx run claude
agx run gemini
agx run opencode
agx run codex work -- --help
agx detach codex work
```

兼容别名：`agx codex` / `agx claude` / `agx gemini` / `agx opencode` 仍可用作 `agx run <agent>`。

### Diagnostics

```bash
agx doctor
agx backup <agent>
agx restore <agent>
```

## Profile 规则

- 名称非空，仅允许字母、数字、`-`、`_`、`.`
- `base_url` 和 `api_key` 必填；`model` 可选，但启动 `opencode` 时必填
- `edit --name` 重命名 profile 时会迁移引用它的所有 binding 和 managed target
- 其他 `edit` 操作会立即 re-sync 所有引用该 profile 的 managed target（best-effort，失败结果在输出中列出）
- `use` 仅切换 agx 当前 profile，不会改写所有 agent 的运行时

## Launcher 规则

- `agx run <agent> [profile] [-- <native args...>]` 在受控上下文中启动原生 CLI
- Profile 解析优先级：位置参数 > `AGX_PROFILE` 环境变量 > `agx current`
- 显式传入 `profile` 仅影响本次启动；如需设为默认请用 `agx use`
- `AGX_PROFILE` 适合配合 `direnv` 等工具按目录 pin 中转，不会改全局 `current`
- `AGX_AUTO_BACKUP=1` 时启动前会自动对当前 target 做 snapshot
- `agx ls` 的 `AGENTS` 列显示 profile 当前绑定的 agent，`agx detach <agent> <profile>` 解除其中一项绑定

## 诊断与恢复

- `agx doctor` 健康时输出 `ok`，异常时列出问题与建议动作
- `agx backup <agent>` 给当前 target 拍 context 快照，仅支持 `codex` / `claude` / `gemini`（`opencode` 无 per-target context）
- `agx restore <agent>` 默认恢复当前 target 的最近一次 snapshot

## 接入边界

agx 是中转聚合器，仅处理 OpenAI 兼容（`base_url` + `api_key`）接入：

- 第三方中转：`agx add` 直接加入
- 官方 API key：作为中转加入，`--base-url` 填官方 endpoint
- OAuth / native SDK / agent 内置 provider：直接用原生 CLI，不要经过 agx

## 文件位置

| 路径 | 用途 |
| --- | --- |
| `~/.config/agx/state.yaml` | 全局 state |
| `~/.config/agx/profiles/` | profile store |
| `~/.config/agx/contexts/` | managed 上下文 |
| `~/.config/agx/backups/managed/` | managed snapshot |
