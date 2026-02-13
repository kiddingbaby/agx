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
│              TUI Layer (Bubble Tea)        │
│  ┌────────────┬────────────┐              │
│  │ Dashboard  │  KeyMgr    │  Elm Arch   │
│  └────────────┴────────────┘              │
├────────────────────────────────────────────┤
│         Session Orchestrator               │
│  - tmux session/window management          │
│  - 环境变量注入（API Key + Base URL）       │
│  - 参数透传                                 │
├────────────────────────────────────────────┤
│           Key Store                        │
│  - AES-GCM 加密存储（API Key）              │
│  - Base URL 明文存储                        │
│  - Provider 分组                           │
├────────────────────────────────────────────┤
│         AI CLI Tools (unmodified)          │
│  - claude-code / codex-cli / gemini-cli    │
└────────────────────────────────────────────┘
```

### 技术栈

| 组件 | 技术               | 说明                           |
| ---- | ------------------ | ------------------------------ |
| 语言 | Go 1.24+           | 单二进制，跨平台               |
| TUI  | Bubble Tea + Bubbles + Lip Gloss | Elm 架构 TUI 框架 |
| 加密 | AES-GCM            | API Key 加密存储               |
| 会话 | tmux >= 3.0        | session/window 管理            |
| 配置 | YAML               | Key 存储格式                   |
| 主题 | Catppuccin Mocha   | 统一配色方案 (via Lip Gloss)   |

## 功能模块规范

### 1. Key Store

#### 数据结构

```go
type Key struct {
    ID        string    // UUID
    Provider  Provider  // openai/claude/gemini
    Name      string    // 用户自定义名称
    APIKey    string    // AES-GCM 加密存储
    BaseURL   string    // 明文存储，可选
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

| 操作 | CLI 命令                                                      | TUI 按键 |
| ---- | ------------------------------------------------------------- | -------- |
| 添加 | `agx keys add --provider P --name N --key K [--base-url URL]` | `a`      |
| 删除 | `agx keys delete <id>`                                        | `d`      |
| 激活 | `agx keys activate <id>`                                      | `Enter`  |
| 列表 | `agx keys ls [--provider P]`                                  | 主页面   |

### 2. Agent 定义

```go
type Agent struct {
    Name          string // e.g. "claude-code"
    Command       string // e.g. "claude"
    EnvVar        string // API Key 环境变量
    BaseURLEnvVar string // Base URL 环境变量
    Provider      string // e.g. "claude"
}
```

#### 环境变量映射

| Provider | API Key 变量        | Base URL 变量        |
| -------- | ------------------- | -------------------- |
| Claude   | `ANTHROPIC_API_KEY` | `ANTHROPIC_BASE_URL` |
| OpenAI   | `OPENAI_API_KEY`    | `OPENAI_API_BASE`    |
| Gemini   | `GOOGLE_API_KEY`    | `GEMINI_BASE_URL`    |

### 3. Session Dashboard (TUI)

`agx` 无参数时进入。

- 上半部分: Active Sessions 列表
- 下半部分: Quick Start（Agent 列表）
- 空 session 时: 显示 "Press K to manage keys, or 1-3 to launch"
- 按 `K` 进入 Key Manager

### 4. Key Manager (TUI)

`agx keys` 无子命令时进入。

#### 列表页导航

- `keyRows` 包含 provider header 行（`keyIdx=-1`）+ key 行
- j/k 在所有可选行间移动，含 provider header
- 按 `a` 根据当前光标 provider 预选表单

#### 表单字段（formFocus 0-4）

| # | 字段     | 必填 | 说明                                  |
|---|----------|------|---------------------------------------|
| 0 | Provider | Y    | h/l 切换，从列表上下文预选            |
| 1 | Name     | Y    | 自定义名称                            |
| 2 | Base URL | -    | 可选，placeholder 按 provider 动态变化 |
| 3 | API Key  | Y    | 密码遮罩，AES-GCM 加密存储           |
| 4 | Tags     | -    | 逗号分隔标签                          |

进入表单后 `formFocus=1`，`formName` 自动 Focus。

## 安全要求

- API Key 使用 AES-GCM 加密存储
- Base URL 明文存储（非敏感信息）
- 配置文件权限 `0600`
- 不在日志/终端输出中显示明文 Key
- Shell 命令中 Key 使用完整转义
