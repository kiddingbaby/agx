# agx

[![CI](https://github.com/kiddingbaby/agx/actions/workflows/ci.yml/badge.svg)](https://github.com/kiddingbaby/agx/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kiddingbaby/agx.svg)](https://pkg.go.dev/github.com/kiddingbaby/agx)
[![Go Report Card](https://goreportcard.com/badge/github.com/kiddingbaby/agx)](https://goreportcard.com/report/github.com/kiddingbaby/agx)
[![Go version](https://img.shields.io/github/go-mod/go-version/kiddingbaby/agx)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS-lightgrey.svg)](#install)

<p align="center">
  <img src="docs/assets/demo.svg" alt="agx demo：登记一份中转 profile，切换，启动 codex / claude" width="780">
</p>

> 一份中转 profile（`base_url` + `api_key`），同步给 `codex` / `claude` / `gemini` / `opencode`，每个 agent 跑在隔离的受管上下文里。

English: [README.en.md](README.en.md) · 文档：[用户指南](docs/user-guide.md) · [架构](docs/ARCHITECTURE.zh.md)

---

## Why agx

在多个 AI coding agent 之间切换中转时，你通常要：

- 手动维护 4 份各家 CLI 的配置文件
  （`~/.codex/config.toml`、`~/.claude/settings.json`、`~/.gemini/.env`、
  `~/.config/opencode/opencode.json`）
- 切换中转时担心改坏其中一份留不下回滚点
- 在不同账号 / 中转 / 模型之间反复编辑同样的字段

agx 把这件事收成 4 个命令：

```bash
agx add  <name>   # 登记一份中转 profile
agx use  <name>   # 切到这份 profile
agx run  <agent>  # 在隔离上下文里启动原生 CLI
agx doctor        # 出问题时给可执行的恢复建议
```

每次写盘前 snapshot profile / state / agent config，配合 `backup` / `restore` / `doctor` 可随时回滚。所有 agent 走相同的 profile 模型。

## Quick start

```bash
# 1. 安装（任选其一，详见下面 Install）
brew install kiddingbaby/agx/agx

# 2. 登记一份中转
agx add work \
  --base-url https://relay.example/v1 \
  --api-key  sk-live

# 3. 启动任意 agent，所有 agent 自动用这份中转
agx use work
agx run codex
agx run claude
```

agx 会在 `~/.config/agx/contexts/<agent>/<name>/` 下生成各 agent 的受管配置，
并在原生 CLI 启动时把它注入。宿主机的 `~/.codex` / `~/.claude` / `~/.gemini`
等用户级配置不被改动。

## Install

### Homebrew（macOS / Linuxbrew）

```bash
brew install kiddingbaby/agx/agx
```

### 预编译二进制

从 [Releases](https://github.com/kiddingbaby/agx/releases/latest) 下载对应平台的归档（替换为你需要的版本）：

```bash
VERSION=v0.1.0
OS=$(uname -s | tr '[:upper:]' '[:lower:]')   # linux / darwin
ARCH=$(uname -m | sed 's/aarch64/arm64/;s/x86_64/x86_64/')
curl -L "https://github.com/kiddingbaby/agx/releases/download/${VERSION}/agx_${VERSION#v}_${OS}_${ARCH}.tar.gz" \
  | tar -xz -C /tmp
install -m 0755 /tmp/agx ~/.local/bin/agx
```

### `go install`

```bash
go install github.com/kiddingbaby/agx/cmd/agx@latest
```

### 从源码构建

```bash
git clone https://github.com/kiddingbaby/agx.git
cd agx
task dev:setup
task build
task install
```

- 二进制默认安装到 `~/.local/bin/agx`，可用 `BINDIR=/usr/local/bin task install` 覆盖
- Go 1.24+；`mise.toml` 已声明所需工具链
- 支持 Linux / macOS、amd64 / arm64

## Examples

切换不同中转：

```bash
agx add openai-direct  --base-url https://api.openai.com/v1            --api-key sk-...
agx add anthropic-relay --base-url https://relay.example/anthropic/v1  --api-key sk-...

agx use openai-direct  && agx run codex
agx use anthropic-relay && agx run claude
```

把同一份中转临时挂给某个 agent（不改 `current`）：

```bash
agx run codex openai-direct -- --help    # 位置参数：本次启动有效
AGX_PROFILE=openai-direct agx run codex  # 环境变量：本 shell / 配合 direnv
```

危险编辑前留底，并在出错时回滚：

```bash
agx backup codex     # 显式快照当前 target
agx edit work --api-key sk-rotated
agx run codex
# 如果发现新 key 不对：
agx restore codex    # 恢复最近一次 snapshot
```

诊断 + 自愈：

```bash
agx doctor
# 输出可能包含 unfinished_operation / orphan_managed_target 等问题，
# 每条都附 suggested action（通常就是 `agx restore <agent>`）。
```

## How it works

```
┌──────────────┐    agx add / edit       ┌─────────────────────┐
│ profile YAML │ ◀────────────────────── │   agx CLI (this)    │
│  store       │                          │  • mutation guard   │
└──────┬───────┘                          │  • per-target ctx   │
       │ resolve                          │  • doctor / restore │
       ▼                                  └──────────┬──────────┘
┌──────────────┐                                     │
│  derived     │   agx run <agent>                   │
│  per-agent   │ ◀───────── exec native CLI ◀────────┘
│  config      │            (codex / claude / gemini / opencode)
└──────────────┘
```

- **Profile 是一等公民**：所有命令围绕"当前用哪份中转"展开
- **Mutation guard**：每次写盘前 snapshot profile / state / agent config；
  任一步失败整体回滚，panic 也走 recover + re-panic 保留非零退出
- **Per-target context**：四个 agent 各自跑在 `contexts/<agent>/<name>/`，不串用户级配置
- **No daemon**：纯 CLI，仅在执行命令期间持有 flock

完整分层与扩展点见 [docs/ARCHITECTURE.zh.md](docs/ARCHITECTURE.zh.md)。

## Scope

仅处理 **OpenAI 兼容**（`base_url` + `api_key`）形式的中转接入。如果你要的是：

- OAuth 登录、agent 原生 SDK、agent 内置 provider —— 直接用原生 CLI，不需要 agx
- 多人共享配置 / 团队级 secret 管理 —— 不在 scope，agx 是单用户工具

## Status

Pre-1.0：

- 主流程（add / use / run / backup / restore / doctor）走完整验证链
  （unit + interactive + e2e + docker matrix）
- 公开合同：[CLI contract](docs/cli-contract.zh.md) · [Doctor issue 目录](docs/doctor-issues.zh.md) · [兼容性策略](COMPATIBILITY.zh.md)
- 内部 state 字段、文件布局可能在 1.0 前调整，请勿依赖

最近版本说明：[docs/release-notes/](docs/release-notes/) · 方向规划：[ROADMAP.zh.md](ROADMAP.zh.md)

## Contributing

欢迎 issue 与 PR。开发流程、验证矩阵和 PR 模板见 [CONTRIBUTING.md](CONTRIBUTING.md)。所有参与者请遵守 [行为准则](CODE_OF_CONDUCT.md)。

仓库级 agent 协作约束：[AGENTS.md](AGENTS.md)。

## License

[MIT](LICENSE) © 2026 kiddingbaby
