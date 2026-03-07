# AGX

AI CLI Session Orchestrator，基于 tmux 管理多 Agent 会话，并统一密钥管理。

> 在 `agent-stack` workspace 中，`agx` 被视为 adjacent tooling，而不是 stack core plane。

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
make build
# 或：
# go build -o .build/agx ./cmd/agx
./.build/agx --help
```

## Makefile

```bash
make help      # 查看全部目标
make build     # 构建 ./.build/agx
make run       # 构建并启动
make test      # 运行 go test ./...
make smoke     # 运行集成冒烟
make clean     # 删除 ./.build/agx
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

## Key Manager（TUI）

完整操作文档见：`docs/key-manager.md`

### 列表页

```text
h/l 或 ←/→ 或 1/2/3   切换 Provider tabs (CLAUDE/OPENAI/GEMINI)
j/k 或 ↑/↓             选择 key
Enter                   激活当前 key
a                       新增 key（默认当前 provider）
e                       编辑当前 key
d                       删除当前 key
i                       显示/隐藏 key 详情
/                       过滤（name/tag/provider）
Esc                     返回 Dashboard
```

### 编辑页（Vim 两态）

```text
NORMAL: j/k 移动字段，h/l（Provider）切换，i 进入 INSERT，x 清空字段，Enter 保存，Esc 返回列表
INSERT: 输入内容，Ctrl+u 清空字段，Esc 回 NORMAL，Enter 保存
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
make test
make smoke
```

`tests/e2e/cli-smoke.sh` 会在临时 `HOME` 下执行最小 CLI 冒烟（不污染本机配置）。

构建与 smoke 产物默认写入 `.build/`，避免与源码树混放。

## Workflow

- `.workflow/` 仅是本仓 repo-local dev process artifact，不属于 `agx` runtime 输入或公共接口
- 统一入口：`wf dev <phase>`（`design/spec/coding/quality/review/test/uat/scan/pr`）
- 文档索引：`docs/index.md`
- 配置源：`.workflow/config/workflow.yml`
- 运行文档源：`.workflow/docs/DESIGN.md` / `.workflow/docs/SPEC.md` / `.workflow/docs/TODO.md`
