# AGX 技术规范文档

## 项目概述

**AGX** (AI CLI Session Orchestrator) 是一个基于 tmux 的 AI CLI 会话编排工具。采用 CLI-first 设计，提供快速启动、参数透传、会话管理和 Key 管理功能。

## 设计理念

- **CLI 优先**：日常操作通过 CLI 命令完成，TUI 仅用于复杂管理
- **零配置启动**：`agx claude` 即可在当前目录启动会话
- **参数透传**：`agx claude -c` 直接将 `-c` 传给 claude CLI
- **tmux 原生**：利用 tmux session/window 实现多会话并行

## 技术架构

```text
┌────────────────────────────────────────────┐
│              AGX CLI Layer                 │
│  ┌──────────────────────────────────────┐  │
│  │ Command Parser                       │  │
│  │ agx <agent> [args...] → Launch       │  │
│  │ agx ls/attach/kill    → Session Mgmt │  │
│  │ agx keys [sub]        → Key Mgmt     │  │
│  │ agx (no args)         → TUI Dashboard│  │
│  └──────────────────────────────────────┘  │
├────────────────────────────────────────────┤
│              TUI Layer                     │
│  ┌────────────┬────────────┐              │
│  │ Dashboard  │  KeyMgr    │              │
│  └────────────┴────────────┘              │
├────────────────────────────────────────────┤
│         Session Orchestrator               │
│  - tmux session/window management          │
│  - 环境变量注入                              │
│  - 参数透传                                 │
├────────────────────────────────────────────┤
│           Key Store                        │
│  - AES-GCM 加密存储                        │
│  - Provider 分组                           │
├────────────────────────────────────────────┤
│         AI CLI Tools (unmodified)          │
│  - claude-code / codex-cli / gemini-cli    │
└────────────────────────────────────────────┘
```

### 技术栈

| 组件 | 技术             | 说明                |
| ---- | ---------------- | ------------------- |
| 语言 | Go 1.22+         | 单二进制，跨平台    |
| TUI  | tview + tcell    | 终端 UI 框架        |
| 加密 | AES-GCM          | API Key 加密存储    |
| 会话 | tmux >= 3.0      | session/window 管理 |
| 配置 | YAML             | Key 存储格式        |
| 主题 | Catppuccin Mocha | 统一配色方案        |

## 命令体系

### 命令解析

```text
agx                     → TUI Dashboard
agx keys                → TUI Key Manager
agx keys ls             → CLI: 列出所有 Key
agx keys add            → CLI: 添加 Key（交互式/非交互）
agx keys activate <id>  → CLI: 激活 Key
agx keys delete <id>    → CLI: 删除 Key
agx ls                  → CLI: 列出会话
agx attach <name>       → CLI: 切换会话
agx a <name>            → CLI: attach 简写
agx kill <name>         → CLI: 终止会话
agx <agent> [args...]   → CLI: 启动 session（当前目录）
```

### 参数透传机制

`agx <agent> [args...]` 中 `args` 直接附加到 agent 命令后：

```bash
agx claude -c
# 实际执行: ANTHROPIC_API_KEY=xxx claude -c

agx claude --dangerously-skip-permissions
# 实际执行: ANTHROPIC_API_KEY=xxx claude --dangerously-skip-permissions
```

## 功能模块规范

### 1. Key Store

#### 数据结构

```go
type Key struct {
    ID        string    // UUID
    Provider  Provider  // openai/claude/gemini
    Name      string    // 用户自定义名称
    APIKey    string    // AES-GCM 加密存储
    Tags      []string  // 功能标签
    Active    bool      // 是否激活
    CreatedAt time.Time // 创建时间
}
```

#### 存储

- 路径: `~/.config/agx/keys.yaml`
- 权限: `0600`
- 加密密钥优先级:
  1. 环境变量 `AGX_SECRET`（32 字节，可选覆盖）
  2. 文件 `~/.config/agx/secret`（自动生成）

#### 操作

| 操作 | CLI 命令                                       | TUI 按键 |
| ---- | ---------------------------------------------- | -------- |
| 添加 | `agx keys add [--provider P --name N --key K]` | `a`      |
| 编辑 | -                                              | `e`      |
| 删除 | `agx keys delete <id>`                         | `d`      |
| 激活 | `agx keys activate <id>`                       | `Enter`  |
| 列表 | `agx keys ls [--provider P]`                   | 主页面   |
| 搜索 | -                                              | `/`      |

### 2. Session Orchestrator

#### tmux 会话设计

| 层级    | 命名规则     | 说明                    |
| ------- | ------------ | ----------------------- |
| Session | `ai-<agent>` | 每个 Agent 一个 session |
| Window  | `<dir-name>` | 每个目录一个 window     |

#### 启动流程

```text
1. 解析 agent 名称和参数
2. 获取当前目录 (cwd)
3. 查找 agent 对应 Provider 的 Active Key
4. 构建完整命令: <agent-cli> [args...]
5. 构建环境变量 map
6. 检查 tmux session 是否存在
   - 不存在 → new-session
   - 存在 → new-window
7. 检测 $TMUX 环境
   - 在 tmux 内 → switch-client
   - 在 tmux 外 → attach-session
```

#### 环境变量注入

| Provider | 环境变量            |
| -------- | ------------------- |
| OpenAI   | `OPENAI_API_KEY`    |
| Claude   | `ANTHROPIC_API_KEY` |
| Gemini   | `GOOGLE_API_KEY`    |

#### Shell 转义

使用 `$'...'` 语法完整转义 API Key，处理 `'`、`\`、`$`、`` ` `` 等特殊字符。

### 3. Session Dashboard (TUI)

`agx` 无参数时进入。

**布局：**

- 上半部分：Active Sessions 列表（Enter attach, d delete）
- 下半部分：Quick Start（数字键直接启动）
- 状态栏：快捷键提示
- 按 `K` 进入 Key Manager

### 4. Key Manager (TUI)

`agx keys` 无子命令时进入。

**功能：**

- 按 Provider 分组显示
- 空 Provider 显示提示文字
- 支持搜索过滤（`/`）
- 删除需确认（Modal，默认 Cancel）

## 项目结构

```text
agx/
├── cmd/
│   └── agx/
│       └── main.go              # 命令解析、子命令分发
├── internal/
│   ├── key/
│   │   ├── store.go             # Key CRUD + 加密
│   │   └── store_test.go        # Key Store 测试
│   ├── tui/
│   │   ├── theme.go             # Catppuccin Mocha 主题
│   │   ├── dashboard.go         # Session Dashboard (新)
│   │   ├── launcher.go          # Agent 选择（保留，Dashboard 内用）
│   │   ├── dirpicker.go         # 目录选择器（保留，可选）
│   │   └── keymgr.go            # Key 管理 UI
│   ├── session/
│   │   ├── orchestrator.go      # tmux 会话管理
│   │   └── orchestrator_test.go # Orchestrator 测试
│   └── config/
│       └── config.go            # 配置管理
├── DESIGN.md                    # UX 设计文档
├── SPEC.md                      # 技术规范（本文件）
├── TODO.md                      # 任务列表
├── README.md                    # 用户文档
├── go.mod
└── go.sum
```

## 安全要求

- API Key 使用 AES-GCM 加密存储
- 配置文件权限 `0600`
- 不在日志/终端输出中显示明文 Key
- Shell 命令中 Key 使用完整转义
- 使用 `gitleaks` 检查敏感信息泄露

## 构建与部署

```bash
# 开发构建
go build -o agx ./cmd/agx

# 生产构建
go build -ldflags="-s -w" -o agx ./cmd/agx

# 安装
go install ./cmd/agx
```

## 外部依赖

### Go 模块

```go
require (
    github.com/google/uuid v1.6.0
    github.com/rivo/tview v0.0.0-20240101153935-32b1c8f8d4b1
    github.com/gdamore/tcell/v2 v2.7.0
    gopkg.in/yaml.v3 v3.0.1
)
```

### 系统依赖

- **tmux** >= 3.0
- **gitleaks**: 敏感信息检测（开发时）
