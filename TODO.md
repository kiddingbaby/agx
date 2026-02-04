# AGX 任务列表

## 任务状态

- `pending`: 待开始
- `in_progress`: 进行中
- `done`: 已完成

---

## Phase 1: 核心功能 ✅

> Phase 1 已完成，实现了基础的 TUI 流程和 Key 管理

| #   | 任务                 | 状态   |
| --- | -------------------- | ------ |
| 1   | Key Store 加密存储   | ✅ done |
| 2   | CLI Launcher TUI     | ✅ done |
| 3   | Directory Picker TUI | ✅ done |
| 4   | Key Manager TUI      | ✅ done |
| 5   | Session Orchestrator | ✅ done |
| 6   | 主程序入口           | ✅ done |
| 7   | 单元测试             | ✅ done |
| 8   | README 文档          | ✅ done |

---

## Phase 2: CLI-First 重构

> 目标：CLI 优先、快速启动、参数透传、Session Dashboard

### P0: 核心功能

#### [P2-1] done - 重构命令解析

- **File**: `cmd/agx/main.go`
- **Description**: 重写 main.go，实现新的命令解析逻辑
- **Details**:
  - `agx` 无参数 → TUI Dashboard
  - `agx keys [sub]` → Key 管理
  - `agx ls/attach/kill` → 会话管理
  - `agx <agent> [args...]` → 快速启动

---

#### [P2-2] done - 实现参数透传

- **File**: `internal/session/orchestrator.go`
- **Description**: 支持 `agx claude -c` 将 `-c` 透传给 claude CLI
- **Details**:
  - `SessionConfig` 增加 `Args []string` 字段
  - `Launch()` 构建命令时附加 args
  - 测试: `agx claude -c` → `claude -c`

---

#### [P2-3] done - 默认当前目录启动

- **File**: `cmd/agx/main.go`, `internal/session/orchestrator.go`
- **Description**: `agx claude` 默认使用 `cwd`，无需选择目录
- **Details**:
  - 移除目录选择步骤
  - `os.Getwd()` 获取当前目录
  - 日志显示启动目录

---

### P1: 会话管理

#### [P2-4] done - 实现 `agx ls` 命令

- **File**: `cmd/agx/main.go`
- **Description**: CLI 列出所有 AI 会话
- **Details**:
  - 调用 `Orchestrator.ListSessions()`
  - 格式化输出: session 名称、window 数量、创建时间

---

#### [P2-5] done - 实现 `agx attach` 命令

- **File**: `cmd/agx/main.go`
- **Description**: CLI 切换到指定会话
- **Details**:
  - `agx attach claude` → `ai-claude`
  - `agx a claude` 简写支持
  - 检测 `$TMUX` 使用 `switch-client`

---

#### [P2-6] done - 实现 `agx kill` 命令

- **File**: `cmd/agx/main.go`, `internal/session/orchestrator.go`
- **Description**: CLI 终止指定会话
- **Details**:
  - `Orchestrator.KillSession(name)` 方法
  - 终止前确认（--force 跳过）

---

### P1: Session Dashboard

#### [P2-7] done - 实现 Session Dashboard TUI

- **File**: `internal/tui/dashboard.go` (新建)
- **Description**: `agx` 无参数时显示会话管理界面
- **Details**:
  - 上半部分: Active Sessions 列表
  - 下半部分: Quick Start（Agent 列表）
  - Enter: attach 选中会话
  - 数字键: 快速启动 Agent
  - K: 进入 Key Manager
  - d: 删除会话

---

### P2: Key 管理改进

#### [P2-8] done - Key Manager 按 Provider 分组

- **File**: `internal/tui/keymgr.go`
- **Description**: Key 列表按 Provider 分组显示
- **Details**:
  - CLAUDE / OPENAI / GEMINI 三个分组
  - 空分组显示 "(no keys - press 'a' to add)"
  - Active Key 显示 `*` 标记

---

#### [P2-9] pending - Key Manager CLI 子命令

- **File**: `cmd/agx/main.go`
- **Description**: 支持 `agx keys ls/add/activate/delete` CLI 命令
- **Details**:
  - `agx keys ls [--provider P]`
  - `agx keys add --provider P --name N --key K`
  - `agx keys activate <id>`
  - `agx keys delete <id>`

---

### P2: 代码质量

#### [P2-10] pending - Shell 转义完整实现

- **File**: `internal/session/orchestrator.go`
- **Description**: 使用 `$'...'` 语法完整转义 API Key
- **Details**:
  - 处理 `'`, `\`, `$`, `` ` ``, `\n` 等字符
  - 添加 `escapeForShell()` 函数测试

---

#### [P2-11] pending - tmux 嵌套检测

- **File**: `internal/session/orchestrator.go`
- **Description**: 检测 `$TMUX` 环境，在 tmux 内使用 `switch-client`
- **Details**:
  - `os.Getenv("TMUX")` 检测
  - 在 tmux 内: `tmux switch-client -t <session>`
  - 在 tmux 外: `tmux attach-session -t <session>`

---

#### [P2-12] pending - 补充测试覆盖

- **File**: `internal/*/\*_test.go`
- **Description**: 增加测试覆盖率
- **Details**:
  - `internal/key/store_test.go` 补充边界测试
  - `escapeForShell()` 测试
  - Dashboard 组件测试（如可行）

---

## 任务统计

| Phase   | 总计 | 完成 | 待开始 |
| ------- | ---- | ---- | ------ |
| Phase 1 | 8    | 8    | 0      |
| Phase 2 | 12   | 8    | 4      |

---

## 优先级排序

### P0 (必须)

- [P2-1] 重构命令解析 ✅
- [P2-2] 参数透传 ✅
- [P2-3] 默认当前目录 ✅

### P1 (重要)

- [P2-4] `agx ls` ✅
- [P2-5] `agx attach` ✅
- [P2-6] `agx kill` ✅
- [P2-7] Session Dashboard ✅

### P2 (改进)

- [P2-8] Key Manager 分组 ✅
- [P2-9] Key CLI 子命令
- [P2-10] Shell 转义
- [P2-11] tmux 嵌套检测
- [P2-12] 补充测试

---

## 下一步行动

**建议从 [P2-9] 开始**: Key Manager CLI 子命令。
