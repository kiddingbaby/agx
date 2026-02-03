# SPEC: AGX MVP - AI CLI Session Orchestrator

## Overview

AGX 是一个 CLI/TUI 工具，用于管理多个 AI CLI 会话。MVP 版本实现核心功能：Key 管理、Agent 启动、目录选择、tmux 会话编排。

## Goals

- [ ] Key Manager: 实现 API Key 的 Add/Edit/Delete/Activate 功能
- [ ] Key Manager: 支持 AES-GCM 加密存储 Key
- [ ] Key Manager: 支持 Provider 标签（OpenAI/Claude/Gemini）和自定义 Tags
- [ ] CLI Launcher: 实现 Agent 选择界面（claude-code/codex-cli/gemini-cli）
- [ ] Directory Picker: 实现 Tree 风格目录选择，支持 Vim 键位
- [ ] tmux Integration: 实现 session/window 创建与 attach
- [ ] tmux Integration: 自动注入 Active Key 到环境变量
- [ ] TUI: 使用 tview 实现完整 TUI 界面

## Non-Goals

- 不实现 Pane 支持（日志/side tool）
- 不实现 Session Dashboard（多会话管理界面）
- 不实现系统 Keychain 集成（仅 AES-GCM 文件加密）
- 不实现 Provider 扩展接口（仅内置三种）

## Background

用户需要频繁切换不同的 AI CLI 工具，每次都要手动设置环境变量、选择目录、管理多个终端窗口。AGX 通过 tmux 作为 session backend，提供统一的入口和 Key 管理。

## Design

### Architecture

```text
┌─────────────────────────────┐
│         AGX TUI             │
│  ┌───────────────────────┐  │
│  │ CLI Launcher          │  │
│  │ Directory Picker      │  │
│  │ Key Manager           │  │
│  └───────────────────────┘  │
└─────────────┬───────────────┘
              │ os/exec
┌─────────────▼───────────────┐
│       tmux CLI              │
│  session / window / pane    │
└─────────────┬───────────────┘
              │
┌─────────────▼───────────────┐
│    AI CLI (unmodified)      │
│  claude-code, codex, gemini │
└─────────────────────────────┘
```

### 模块划分

| 模块 | 文件 | 职责 |
|------|------|------|
| main | cmd/agx/main.go | 入口，初始化 TUI |
| tui | internal/tui/*.go | TUI 组件（Launcher, Picker, KeyManager） |
| key | internal/key/*.go | Key 存储、加密、CRUD |
| tmux | internal/tmux/*.go | tmux 命令封装 |
| config | internal/config/*.go | 配置加载 |

### Data Model

Key 存储格式（YAML + AES-GCM 加密）：

```yaml
keys:
  - id: "uuid"
    provider: "claude"
    name: "my-claude-key"
    api_key: "<encrypted>"
    tags: ["cache", "code"]
    active: true
    created_at: "2024-01-01T00:00:00Z"
```

## Implementation Plan

1. [ ] 项目初始化：创建 Go module，目录结构，依赖（tview, tcell, yaml）
2. [ ] Key 模块：实现 Key struct、加密/解密、YAML 存储
3. [ ] Key Manager TUI：实现 Add/Edit/Delete/Activate 界面
4. [ ] Directory Picker：实现 Tree 组件，Vim 键位绑定
5. [ ] CLI Launcher：实现 Agent 选择列表
6. [ ] tmux 模块：封装 session/window 创建、环境变量注入
7. [ ] 主流程集成：Launcher → Picker → tmux attach
8. [ ] 测试与文档

## Testing Strategy

- [ ] Unit tests: Key 加密/解密、YAML 序列化
- [ ] Unit tests: tmux 命令生成
- [ ] Integration tests: 完整 TUI 流程（使用 tcell simulation）
- [ ] Manual testing: 实际 tmux 会话创建与 AI CLI 启动

## Security Considerations

- API Key 使用 AES-256-GCM 加密存储
- 加密密钥派生自用户密码（PBKDF2）
- Key 文件权限设置为 600
- 环境变量仅在 tmux 子进程中可见

## Rollback Plan

MVP 阶段无需回滚计划，如有问题直接修复。

## References

- [tview 文档](https://github.com/rivo/tview)
- [tmux 手册](https://man7.org/linux/man-pages/man1/tmux.1.html)
- plan.md - 产品设计文档
