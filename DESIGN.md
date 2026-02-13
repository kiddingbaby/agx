# AGX UX 设计文档

> AGX = **AI CLI Session Orchestrator**
> 目标：CLI 优先、最小操作成本、多会话并行、Power User 取向

---

## 一、核心设计原则

**当前痛点（已解决）：**

```text
旧流程: agx → 选 Agent → 选目录 → 启动
              ↑          ↑
              多余！      多余！
```

**新设计理念：**

- 类似 tmux attach 的快速模式
- 默认当前目录，无需选择
- 支持透传 CLI 参数（如 `claude -c`）
- TUI 仅用于复杂管理场景

---

## 二、使用模式

### CLI 优先（日常使用）

```bash
# 快速启动（当前目录）
agx claude              # 在 cwd 启动 claude-code
agx claude -c           # 透传 -c 给 claude CLI
agx claude -r           # 透传 -r
agx codex               # 启动 codex-cli

# 会话管理（类似 tmux）
agx ls                  # 列出所有 AI 会话
agx attach claude       # 切换到 ai-claude 会话
agx a claude            # attach 简写
agx kill claude         # 终止会话
```

### TUI 模式（复杂管理）

```bash
agx                     # 进入 Session Dashboard
agx keys                # 进入 Key 管理
agx --tui               # 强制进入完整 TUI
```

---

## 三、命令解析逻辑

```go
agx                     → TUI Dashboard
agx keys                → TUI Key Manager
agx ls                  → CLI: 列出会话
agx attach <name>       → CLI: 切换会话
agx a <name>            → CLI: attach 简写
agx kill <name>         → CLI: 终止会话
agx <agent> [args...]   → CLI: 启动 session（当前目录）
```

### 参数透传

```bash
agx claude -c           # 实际执行: claude -c
agx claude --dangerously-skip-permissions  # 透传长参数
```

---

## 四、Session Dashboard (TUI)

当 `agx` 无参数时进入，显示所有会话：

```text
┌─────────────────────────────────────────────────────────────┐
│  AGX SESSION DASHBOARD                           [K]eys    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  ACTIVE SESSIONS                                            │
│  ─────────────────────────────────────────────────────────  │
│  > ai-claude      ~/projects/agx        2 windows           │
│    ai-codex       ~/projects/blog       1 window            │
│                                                             │
│  QUICK START                                                │
│  ─────────────────────────────────────────────────────────  │
│    [1] claude-code  ✓                                       │
│    [2] codex-cli    ✗ (no key)                              │
│    [3] gemini-cli   ✗ (no key)                              │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Enter Attach  n New  d Delete  K Keys  q Quit               │
└─────────────────────────────────────────────────────────────┘
```

**布局说明：**

- **上半部分**：已有会话列表（Enter 可 attach）
- **下半部分**：快速启动（数字键直接启动）
- **焦点默认在已有会话**：方便快速 attach
- **空会话时引导**：显示 "Press K to manage keys, or 1-3 to launch"

---

## 五、Key 管理完整设计

### 5.1 Key 数据模型

```go
type Key struct {
    ID        string    `yaml:"id"`
    Provider  Provider  `yaml:"provider"`
    Name      string    `yaml:"name"`
    APIKey    string    `yaml:"api_key"`            // AES-GCM 加密
    BaseURL   string    `yaml:"base_url,omitempty"` // 明文，可选
    Tags      []string  `yaml:"tags,omitempty"`
    Active    bool      `yaml:"active"`
    CreatedAt time.Time `yaml:"created_at"`
}
```

**BaseURL 映射（参见 CLI_ENV.md）：**

| Provider | API Key 环境变量 | Base URL 环境变量 |
|----------|------------------|-------------------|
| Claude   | `ANTHROPIC_API_KEY` | `ANTHROPIC_BASE_URL` |
| OpenAI   | `OPENAI_API_KEY` | `OPENAI_API_BASE` |
| Gemini   | `GOOGLE_API_KEY` | `GEMINI_BASE_URL` |

### 5.2 Key 列表页（主页面）

```text
┌─────────────────────────────────────────────────────────────┐
│  KEY MANAGER                                     [Esc] Back │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  CLAUDE                                                     │
│  ────────────────────────────────────────────────────────   │
│  > * my-claude-key     cache, code           2026-02-04     │
│      backup-key        -                     2026-01-15     │
│                                                             │
│  OPENAI                                                     │
│  ────────────────────────────────────────────────────────   │
│    (no keys - press 'a' to add)                             │
│                                                             │
│  GEMINI                                                     │
│  ────────────────────────────────────────────────────────   │
│    (no keys - press 'a' to add)                             │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Enter Activate  a Add  d Delete  Esc Back                   │
└─────────────────────────────────────────────────────────────┘
```

**导航设计：**

- `keyRows` 包含 provider header 行 + key 行
- Provider header 可选中（`keyIdx=-1`），用于确定 `a` 新增时的 provider
- j/k 在所有可选行间移动（包括 provider header）
- 按 `a` 时根据当前光标所在 provider 预选表单

**操作说明：**

| 按键 | 操作 |
|------|------|
| `↑↓/jk` | 上下导航（含 provider header） |
| `Enter` | 激活选中的 Key |
| `a` | 添加新 Key（预选当前 provider） |
| `d` | 删除选中的 Key（需确认）|
| `Esc` | 返回 Dashboard |

---

### 5.3 添加 Key 页面

```text
┌─────────────────────────────────────────────────────────────┐
│  ADD KEY                                                    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Provider:  < claude >                                      │
│                                                             │
│  Name:      [ my-new-key_________________ ]                 │
│                                                             │
│  Base URL:  [ https://api.anthropic.com__ ]                 │
│             (optional, leave empty for default)             │
│                                                             │
│  API Key:   [ ******************************** ]            │
│             (will be encrypted)                             │
│                                                             │
│  Tags:      [ cache, code________________ ]                 │
│             (comma separated, optional)                     │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Tab Next field  Shift+Tab Previous  Enter Save  Esc Cancel  │
└─────────────────────────────────────────────────────────────┘
```

**字段顺序（formFocus 0-4）：**

| # | 字段 | 必填 | 说明 |
|---|------|------|------|
| 0 | Provider | ✓ | h/l 切换：Claude / OpenAI / Gemini |
| 1 | Name | ✓ | 自定义名称 |
| 2 | Base URL | - | 可选，按 provider 显示 placeholder |
| 3 | API Key | ✓ | 密码遮罩，加密存储 |
| 4 | Tags | - | 逗号分隔标签 |

**交互修复：**
- 进入表单后 `formFocus=1`，`formName` 自动 Focus（provider 已预选）
- Tab/Shift+Tab 循环切换字段

---

### 5.4 删除确认

```text
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   DELETE KEY                                                │
│                                                             │
│   Are you sure you want to delete "my-claude-key"?          │
│                                                             │
│   This action cannot be undone.                             │
│                                                             │
│              [ Cancel ]         [ Delete ]                  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

- 默认焦点在 `Cancel` 上（防误删）
- `Enter` 执行当前按钮，`Esc` 取消

---

### 5.5 CLI 命令支持

```bash
# 列出所有 Key
agx keys ls
agx keys ls --provider claude

# 添加 Key（非交互）
agx keys add --provider claude --name my-key --key sk-xxx [--base-url https://...] [--tags cache,code]

# 激活 Key
agx keys activate <key-id-or-name>

# 删除 Key
agx keys delete <key-id-or-name>

# 进入 Key 管理 TUI
agx keys
```

---

## 六、Agent 定义

```go
type Agent struct {
    Name          string
    Command       string
    EnvVar        string // API Key 环境变量
    BaseURLEnvVar string // Base URL 环境变量
    Provider      string
}

// DefaultAgents returns the list of supported AI CLI tools
func DefaultAgents() []Agent {
    return []Agent{
        {Name: "claude-code", Command: "claude", EnvVar: "ANTHROPIC_API_KEY", BaseURLEnvVar: "ANTHROPIC_BASE_URL", Provider: "claude"},
        {Name: "codex-cli",   Command: "codex",  EnvVar: "OPENAI_API_KEY",    BaseURLEnvVar: "OPENAI_API_BASE",    Provider: "openai"},
        {Name: "gemini-cli",  Command: "gemini", EnvVar: "GOOGLE_API_KEY",    BaseURLEnvVar: "GEMINI_BASE_URL",    Provider: "gemini"},
    }
}
```

Launch 时若 `activeKey.BaseURL != ""`，额外注入 `agent.BaseURLEnvVar` 环境变量。

---

## 七、主题设计 (Catppuccin Mocha)

```text
颜色系统:
├── 背景
│   ├── BgPrimary:   #1e1e2e (深蓝灰)
│   ├── BgSecondary: #313244 (次级背景)
│   └── BgHighlight: #45475a (高亮背景)
├── 前景
│   ├── FgPrimary:   #cdd6f4 (主文本)
│   ├── FgSecondary: #a6adc8 (次级文本)
│   └── FgMuted:     #6c7086 (灰化文本)
├── 语义色
│   ├── Accent:      #89b4fa (蓝色强调)
│   ├── Success:     #a6e3a1 (绿色成功)
│   ├── Warning:     #f9e2af (黄色警告)
│   └── Error:       #f38ba8 (红色错误)
└── 边框
    ├── Border:      #585b70 (普通边框)
    └── BorderFocus: #89b4fa (聚焦边框)
```

---

## 八、验证方案

1. **CLI 流程测试**
   - `agx claude` 在当前目录启动
   - `agx claude -c` 正确透传参数
   - `agx ls` 显示会话列表

2. **TUI 测试**
   - `agx` 进入 Dashboard，无 session 时显示引导
   - Key Manager j/k 在 provider 间导航
   - 按 `a` 进入 form，provider 预选正确
   - Tab 切换字段，textinput 正常工作
   - BaseURL 字段可输入，launch 时注入环境变量

3. **边界情况**
   - 无会话时 Dashboard 显示 "Press K to manage keys"
   - Agent 无 Key 时显示提示
   - BaseURL 为空时不注入环境变量
