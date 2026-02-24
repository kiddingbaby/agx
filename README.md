# AGX

AI CLI Session Orchestrator，基于 tmux 管理多 Agent 会话，并统一密钥管理。

## 核心能力

- 多 Agent：`claude-code` / `codex-cli` / `gemini-cli`
- Key 管理：`keys.yaml` 持久化 + AES-GCM 加密
- 会话管理：tmux session/window 自动创建与复用
- TUI + CLI 双入口：同一套 usecase

## 依赖

- Go 1.22+
- tmux 3.0+

## 快速开始

```bash
go build -o agx ./cmd/agx
./agx --help
```

首次运行会初始化：

- `~/.config/agx/keys.yaml`
- `~/.config/agx/secret`

也可显式提供：`AGX_SECRET`（必须 32 bytes）。

## 常用命令

```bash
agx                           # Dashboard TUI
agx keys                      # Key Manager TUI
agx keys ls [--provider P]
agx keys add --provider P --name N --key K [--base-url URL] [--tags T]
agx keys activate <id|name>
agx keys delete <id|name>
agx ls
agx attach <name>
agx kill <name>
agx <agent> [args...]
```

## 架构分层（当前）

```text
cmd
  -> interfaces (cli/tui)
    -> app/bootstrap
      -> usecase
        -> domain + ports
          -> adapters (keyfile/tmux)
```

## 目录概览

```text
cmd/agx/
internal/
  adapters/        # keyfile, tmux
  app/             # bootstrap + container
  config/          # paths + secret provider
  domain/          # agent, key, session
  interfaces/      # cli, tui
  ports/           # repository/runtime/provider interfaces
  usecase/         # key/session/launch + unified errors
  tui/             # bubbletea model implementation
```

## 回归基线

```bash
go test ./...
bash tests/integration/smoke-go.sh
```

`tests/e2e/cli-smoke.sh` 会在临时 `HOME` 下执行最小 CLI 冒烟（不污染本机配置）。
