# AGX 任务列表

## 任务状态说明

- `pending`: 待开始
- `in_progress`: 进行中
- `done`: 已完成

---

## Phase 1: 核心功能实现

### [1] done - 实现 Key Store 加密存储

- **File**: `internal/key/store.go`
- **Description**: 使用 AES-GCM 实现 API Key 的加密存储，支持 Add/Delete/Activate/GetActive 操作
- **Dependencies**: []
- **Created**: 2026-02-03 18:47:00
- **Updated**: 2026-02-03 18:47:00
- **Status**: ✅ 已完成

---

### [2] pending - 实现 CLI Launcher TUI

- **File**: `internal/tui/launcher.go`
- **Description**: 实现 Agent 选择界面，支持 claude-code/codex-cli/gemini-cli 三种 AI CLI 工具的选择，使用 tview List 组件
- **Dependencies**: []
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**实现要点**:

- 使用 `tview.List` 显示 Agent 列表
- 支持 Vim 键位 (j/k 上下移动)
- Enter 确认选择
- Esc 退出
- 选择后进入 Directory Picker

---

### [3] pending - 实现 Directory Picker TUI

- **File**: `internal/tui/dirpicker.go`
- **Description**: 实现目录树选择器，支持 Vim 键位导航 (hjkl)，显示目录树结构
- **Dependencies**: [2]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**实现要点**:

- 使用 `tview.TreeView` 显示目录树
- 支持 hjkl 导航
- Enter 确认目录
- Esc 返回上一步
- 显示当前路径

---

### [4] pending - 实现 Key Manager TUI

- **File**: `internal/tui/keymgr.go`
- **Description**: 实现 Key 管理界面，支持 Add/Edit/Delete/Activate/List 操作
- **Dependencies**: []
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**实现要点**:

- 使用 `tview.Table` 显示 Key 列表
- 使用 `tview.Form` 实现 Add/Edit 表单
- 支持快捷键操作 (a: add, d: delete, Enter: activate)
- 显示 Active 状态标记
- 密码输入使用 `*` 遮罩

---

### [5] pending - 实现 Session Orchestrator

- **File**: `internal/session/orchestrator.go`
- **Description**: 实现 tmux 会话管理，支持 session/window 创建、环境变量注入、attach 操作
- **Dependencies**: [2, 3, 4]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**实现要点**:

- 检查 tmux 是否安装
- 检查 session 是否存在
- 创建 session/window
- 注入环境变量 (ANTHROPIC_API_KEY/OPENAI_API_KEY/GOOGLE_API_KEY)
- attach 到 session
- 错误处理

---

### [6] pending - 实现主程序入口

- **File**: `cmd/agx/main.go`
- **Description**: 实现主程序入口，整合所有模块，支持命令行参数
- **Dependencies**: [2, 3, 4, 5]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**实现要点**:

- 解析命令行参数 (--agent, --dir, --help)
- 初始化 Key Store
- 启动 TUI
- 错误处理和日志
- 优雅退出

---

### [7] pending - 添加单元测试

- **File**: `internal/*/\*_test.go`
- **Description**: 为所有模块添加单元测试，覆盖率 > 80%
- **Dependencies**: [2, 3, 4, 5, 6]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**测试范围**:

- Key Store 加密/解密
- Key CRUD 操作
- Session 创建逻辑
- 环境变量注入
- 错误处理

---

### [8] pending - 编写 README 和使用文档

- **File**: `README.md`
- **Description**: 编写项目 README，包括安装、配置、使用说明
- **Dependencies**: [6]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

**文档内容**:

- 项目介绍
- 安装步骤
- 快速开始
- 配置说明
- 使用示例
- 故障排查

---

## Phase 2: 增强功能 (后续)

### [9] pending - 实现历史目录记忆

- **File**: `internal/config/history.go`
- **Description**: 记录最近使用的目录，支持快速跳转
- **Dependencies**: [3]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

---

### [10] pending - 实现 Session Dashboard

- **File**: `internal/tui/dashboard.go`
- **Description**: 显示所有活跃的 tmux session，支持切换和管理
- **Dependencies**: [5]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

---

### [11] pending - 支持自定义 Agent

- **File**: `internal/config/config.go`
- **Description**: 支持用户自定义 AI CLI 工具配置
- **Dependencies**: [6]
- **Created**: 2026-02-04 00:45:00
- **Updated**: 2026-02-04 00:45:00
- **Status**: ⏳ 待开始

---

## 任务统计

- **总任务数**: 11
- **已完成**: 1
- **进行中**: 0
- **待开始**: 10

---

## 优先级排序

### P0 (必须完成)

- [2] CLI Launcher TUI
- [3] Directory Picker TUI
- [4] Key Manager TUI
- [5] Session Orchestrator
- [6] 主程序入口

### P1 (重要)

- [7] 单元测试
- [8] README 文档

### P2 (增强功能)

- [9] 历史目录记忆
- [10] Session Dashboard
- [11] 自定义 Agent

---

## 下一步行动

**建议从任务 [2] 开始**: 实现 CLI Launcher TUI

这是用户交互的入口，完成后可以快速验证整体流程。
