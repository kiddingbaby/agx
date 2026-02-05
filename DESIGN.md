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

---

## 五、Key 管理完整设计

### 5.1 Key 列表页（主页面）

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
│ Enter Activate  a Add  e Edit  d Delete  / Search  Esc Back │
└─────────────────────────────────────────────────────────────┘
```

**操作说明：**

| 按键 | 操作 |
|------|------|
| `↑↓/jk` | 上下导航 |
| `Enter` | 激活选中的 Key |
| `a` | 添加新 Key |
| `e` | 编辑选中的 Key |
| `d` | 删除选中的 Key（需确认）|
| `/` | 搜索 Key（按名称/标签）|
| `Esc` | 返回 Dashboard |

---

### 5.2 添加 Key 页面

```text
┌─────────────────────────────────────────────────────────────┐
│  ADD NEW KEY                                     [Esc] Cancel│
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Provider:  [ Claude    ▼ ]                                 │
│                                                             │
│  Name:      [ my-new-key_________________ ]                 │
│                                                             │
│  API Key:   [ ******************************** ]            │
│             (will be encrypted)                             │
│                                                             │
│  Tags:      [ cache, code________________ ]                 │
│             (comma separated, optional)                     │
│                                                             │
│  ─────────────────────────────────────────────────────────  │
│                                                             │
│         [ Save & Activate ]    [ Save ]    [ Cancel ]       │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Tab Next field  Shift+Tab Previous  Enter Confirm           │
└─────────────────────────────────────────────────────────────┘
```

**字段说明：**

| 字段 | 必填 | 说明 |
|------|------|------|
| Provider | ✓ | 下拉选择：Claude / OpenAI / Gemini |
| Name | ✓ | 自定义名称，用于识别 |
| API Key | ✓ | 密钥，输入时显示 `*`，加密存储 |
| Tags | - | 可选标签，逗号分隔 |

**按钮行为：**

- `Save & Activate`: 保存并设为当前 Provider 的 Active Key
- `Save`: 仅保存，不激活
- `Cancel`: 放弃，返回列表

---

### 5.3 编辑 Key 页面

```text
┌─────────────────────────────────────────────────────────────┐
│  EDIT KEY: my-claude-key                         [Esc] Cancel│
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Provider:  Claude (cannot change)                          │
│                                                             │
│  Name:      [ my-claude-key______________ ]                 │
│                                                             │
│  API Key:   [ ●●●●●●●●●●●●●●●●●●●●●●●●●●●● ]  [Change]      │
│             (leave empty to keep current)                   │
│                                                             │
│  Tags:      [ cache, code________________ ]                 │
│                                                             │
│  Status:    ✓ Active                                        │
│                                                             │
│  ─────────────────────────────────────────────────────────  │
│                                                             │
│              [ Save ]              [ Cancel ]               │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Tab Next field  Enter Confirm  Esc Cancel                   │
└─────────────────────────────────────────────────────────────┘
```

**编辑限制：**

- Provider 不可修改（需要删除重建）
- API Key 默认显示圆点，点击 `[Change]` 后可输入新值
- 留空表示保持原值

---

### 5.4 删除确认

```text
┌─────────────────────────────────────────────────────────────┐
│                                                             │
│   ⚠ DELETE KEY                                              │
│                                                             │
│   Are you sure you want to delete "my-claude-key"?          │
│                                                             │
│   This action cannot be undone.                             │
│                                                             │
│              [ Delete ]         [ Cancel ]                  │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

- 居中弹窗（Modal）
- 默认焦点在 `Cancel` 上（防误删）
- `Enter` 执行当前按钮，`Esc` 取消

---

### 5.5 搜索模式

按 `/` 进入搜索模式：

```text
┌─────────────────────────────────────────────────────────────┐
│  KEY MANAGER                              Search: cache_    │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  Search results for "cache":                                │
│  ────────────────────────────────────────────────────────   │
│  > * my-claude-key     [cache], code        Claude          │
│      work-key          [cache]              OpenAI          │
│                                                             │
│  (2 keys found)                                             │
│                                                             │
├─────────────────────────────────────────────────────────────┤
│ Enter Select  Esc Clear search  ↑↓ Navigate                 │
└─────────────────────────────────────────────────────────────┘
```

- 实时过滤（按名称和标签匹配）
- 高亮匹配的文本
- `Esc` 清除搜索，返回完整列表

---

### 5.6 CLI 命令支持

除了 TUI，也提供 CLI 命令：

```bash
# 列出所有 Key
agx keys ls
agx keys ls --provider claude

# 添加 Key（交互式）
agx keys add

# 添加 Key（非交互）
agx keys add --provider claude --name my-key --key sk-xxx --tags cache,code

# 激活 Key
agx keys activate <key-id-or-name>

# 删除 Key
agx keys delete <key-id-or-name>

# 导出（不含实际密钥，仅元数据）
agx keys export > keys-meta.yaml

# 进入 Key 管理 TUI
agx keys
```

---

## 六、主题设计 (Catppuccin Mocha)

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

## 七、核心代码结构

```go
// cmd/agx/main.go
func main() {
    if len(os.Args) == 1 {
        // 无参数 → TUI Dashboard
        runDashboard()
        return
    }

    switch os.Args[1] {
    case "keys":
        runKeyManager()
    case "ls":
        listSessions()
    case "attach", "a":
        attachSession(os.Args[2])
    case "kill":
        killSession(os.Args[2])
    default:
        // 假设是 agent 名字
        launchAgent(os.Args[1], os.Args[2:])
    }
}

// launchAgent 在当前目录启动，透传参数
func launchAgent(agent string, args []string) {
    dir, _ := os.Getwd()
    cfg := SessionConfig{
        Agent:   agent,
        Dir:     dir,
        Command: buildCommand(agent, args), // claude -c
        EnvVars: getEnvForAgent(agent),
    }
    orch.Launch(cfg)
}
```

---

## 八、验证方案

1. **CLI 流程测试**
   - `agx claude` 在当前目录启动
   - `agx claude -c` 正确透传参数
   - `agx ls` 显示会话列表

2. **TUI 测试**
   - `agx` 进入 Dashboard，显示会话列表
   - Enter 可 attach 已有会话
   - 数字键可快速启动新会话

3. **边界情况**
   - 无会话时 Dashboard 显示空状态
   - Agent 无 Key 时显示提示
