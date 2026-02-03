# AGX 产品设计文档（专业版）

> AGX = **AI CLI Session Orchestrator**  
> 目标：最小操作成本、多会话并行、CLI 原生体验、Power User 取向  
> 内置 tmux 作为 session backend，管理多 AI CLI 会话

---

## 一、产品定位

| 属性 | 描述 |
|------|------|
| 产品类型 | CLI/TUI 工具 |
| 核心功能 | Key 管理、Agent 启动、目录选择、会话编排 |
| 用户群体 | DevOps、AI 工程师、Power User |
| 核心价值 | 快速启动任意 AI CLI、并行会话管理、简洁操作体验 |

---

## 二、功能模块概览

```
┌───────────────────────┐
│        AGX TUI        │
│ ──────────────────── │
│ CLI Launcher          │
│ Directory Picker      │
│ Key Manager           │
│ Session Orchestration │
└─────────┬─────────────┘
          │ 调用 tmux CLI
┌─────────▼─────────────┐
│       tmux Backend     │
│ session/window/pane    │
└─────────┬─────────────┘
          │
┌─────────▼─────────────┐
│ AI CLI Tools (unmodified) │
│ - claude-code             │
│ - codex-cli               │
│ - gemini-cli              │
└──────────────────────────┘
```

模块说明：

| 模块 | 职责 |
|------|------|
| CLI Launcher | Agent 选择、目录确认、启动会话 |
| Directory Picker | Tree 风格目录选择，可记忆历史 |
| Key Manager | API Key 管理（Add/Edit/Delete/Activate），标签语义化 |
| Session Orchestration | 调用 tmux 管理 session/window，注入环境变量 |
| tmux Backend | 多会话并行执行，管理窗口生命周期 |
| AI CLI Tools | 实际执行的 AI CLI，不做改造 |

---

## 三、用户操作流

### 1. 启动 AGX
- 打开 TUI，进入 CLI Launcher  

### 2. 选择 Agent
```
> claude-code
  codex-cli
  gemini-cli
```

### 3. 选择目录（Tree Picker）
- Vim 键位上下左右选择
- Enter 确认目录

### 4. 使用 Key
- Key Manager 已设置 Active Key
- 用户无需重复选择
- 新建会话时自动注入环境变量

### 5. 启动 AI CLI
- Enter → 创建 tmux session / window
- TUI 可以退出，直接进入子 shell

---

## 四、Key 管理设计

### Key 属性

| 属性 | 描述 |
|------|------|
| Provider | 内置：OpenAI、Claude、Gemini |
| Name | 自定义标识 |
| API Key | 安全存储（AES-GCM 或系统 Keychain） |
| Tags | 功能能力标签（cache、cheap、code、translate 等） |

### Add / Edit Key 页面（示意）

```
╔════════════════════════╗
║ Provider: [Claude ▼]   ║
║ Name:     [key-claude] ║
║ API Key:  [***********]║
║ Tags:     [cache, code]║
╠════════════════════════╣
║ [Enter] Save & Activate ║
║ [Esc]  Cancel           ║
╚════════════════════════╝
```

---

## 五、tmux 会话设计

| 层级 | 映射 | 说明 |
|------|------|------|
| Session | Agent 工作空间 | 每个 Agent 对应一个 session |
| Window  | CLI 实例 | 每个窗口运行一个 AI CLI 命令 |
| Pane    | 可选工具 | 日志、调试、状态面板（未来扩展） |

### 创建/复用逻辑（伪代码）

```text
if tmux session not exists:
    tmux new-session -d -s ai-<agent> -c <dir> <cmd>
else:
    tmux new-window -t ai-<agent> -c <dir> <cmd>
tmux attach -t ai-<agent>
```

---

## 六、技术栈

| 层 | 技术选型 | 说明 |
|----|----------|------|
| 主语言 | Go | 单二进制，可并发执行 tmux 命令 |
| TUI 框架 | tview + tcell | List、Tree、Form、Modal 等组件 |
| Key 存储 | YAML + AES-GCM / Keychain | 安全、可加密、可扩展 |
| tmux 集成 | os/exec 调用 tmux CLI | session/window/pane 编排，环境变量注入 |
| Provider 接口 | Go 接口抽象 | 内置三种：OpenAI / Claude / Gemini，可扩展 |

---

## 七、MVP 功能清单

| 功能 | 描述 | 必须 |
|------|------|------|
| CLI Launcher | Agent 选择 + 目录确认 | ✅ |
| Directory Picker | Tree 风格目录 | ✅ |
| Key Manager | Add/Edit/Delete/Activate | ✅ |
| tmux Session/Window | 会话创建与 attach | ✅ |
| Active Key 注入 | 自动环境变量 | ✅ |
| Vim 键位支持 | TUI 操作 | ✅ |
| Pane 支持 | 日志 / side tool | ❌（后置） |
| Session Dashboard | 管理多会话 | ❌（后置） |

---

## 八、AGX 核心优势

- 极简操作：Enter 一步启动 AI CLI
- 多会话并行：tmux 自然支持
- Key 标签语义化，易筛选
- Tree 目录管理直观
- 可扩展 Provider 与 CLI 工具
- 技术架构清晰，易维护
- Power User 上手快，CLI 原生体验

---


