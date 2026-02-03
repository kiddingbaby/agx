# AGX 技术规范文档

## 项目概述

**AGX** (AI CLI Session Orchestrator) 是一个基于 tmux 的 AI CLI 会话编排工具，提供 Key 管理、Agent 启动、目录选择和多会话并行管理功能。

## 技术架构

### 核心模块

```text
┌─────────────────────────────────────┐
│           AGX TUI Layer             │
│  ┌──────────┬──────────┬─────────┐ │
│  │ Launcher │ DirPicker│ KeyMgr  │ │
│  └──────────┴──────────┴─────────┘ │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│        Session Orchestrator         │
│  - tmux session/window management   │
│  - Environment variable injection   │
└──────────────┬──────────────────────┘
               │
┌──────────────▼──────────────────────┐
│         AI CLI Tools                │
│  - claude-code                      │
│  - codex-cli                        │
│  - gemini-cli                       │
└─────────────────────────────────────┘
```

### 技术栈

| 组件 | 技术 | 说明 |
|------|------|------|
| 语言 | Go 1.22+ | 单二进制，跨平台 |
| TUI | tview + tcell | 终端 UI 框架 |
| 加密 | AES-GCM | API Key 加密存储 |
| 会话 | tmux | session/window 管理 |
| 配置 | YAML | Key 存储格式 |

## 功能模块规范

### 1. Key Manager

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

#### 功能需求

- **Add Key**: 添加新 Key，自动加密存储
- **Edit Key**: 修改 Key 信息（Name/Tags）
- **Delete Key**: 删除 Key
- **Activate Key**: 激活 Key（同 Provider 只能有一个 Active）
- **List Keys**: 显示所有 Key，标记 Active 状态

#### 存储位置

- 默认路径: `~/.config/agx/keys.yaml`
- 权限: `0600` (仅用户可读写)
- 加密密钥: 从环境变量 `AGX_SECRET` 读取（32 字节）

### 2. CLI Launcher

#### 功能需求

- **Agent 选择**: 列表选择 AI CLI 工具
  - claude-code
  - codex-cli
  - gemini-cli
- **目录选择**: Tree 风格目录选择器
- **启动会话**: 创建/复用 tmux session

#### 操作流程

```text
1. 显示 Agent 列表
2. 用户选择 Agent (↑↓ + Enter)
3. 显示目录树
4. 用户选择目录 (hjkl + Enter)
5. 检查 Active Key
6. 创建 tmux session/window
7. 注入环境变量
8. attach 到 session
```

### 3. Directory Picker

#### 功能需求

- **Tree 显示**: 显示目录树结构
- **Vim 键位**: hjkl 导航
- **历史记忆**: 记录最近使用的目录
- **快速跳转**: 输入路径快速跳转

#### UI 设计

```text
┌─ Select Directory ─────────────┐
│ /home/user                     │
│ ├─ projects/                   │
│ │  ├─ agx/          ← selected │
│ │  └─ other/                   │
│ └─ documents/                  │
│                                │
│ [Enter] Confirm  [Esc] Cancel  │
└────────────────────────────────┘
```

### 4. Session Orchestrator

#### tmux 会话设计

| 层级 | 命名规则 | 说明 |
|------|----------|------|
| Session | `ai-<agent>` | 每个 Agent 一个 session |
| Window | `<dir-name>` | 每个目录一个 window |

#### 创建逻辑

```bash
# 检查 session 是否存在
if ! tmux has-session -t ai-claude 2>/dev/null; then
    # 创建新 session
    tmux new-session -d -s ai-claude -c /path/to/dir \
        "export ANTHROPIC_API_KEY=xxx && claude-code"
else
    # 创建新 window
    tmux new-window -t ai-claude -c /path/to/dir \
        "export ANTHROPIC_API_KEY=xxx && claude-code"
fi

# attach 到 session
tmux attach -t ai-claude
```

#### 环境变量注入

| Provider | 环境变量 |
|----------|----------|
| OpenAI | `OPENAI_API_KEY` |
| Claude | `ANTHROPIC_API_KEY` |
| Gemini | `GOOGLE_API_KEY` |

## 项目结构

```text
agx/
├── cmd/
│   └── agx/
│       └── main.go           # 入口文件
├── internal/
│   ├── key/
│   │   └── store.go          # Key 管理 (已实现)
│   ├── tui/
│   │   ├── launcher.go       # CLI Launcher
│   │   ├── dirpicker.go      # 目录选择器
│   │   └── keymgr.go         # Key 管理 UI
│   ├── session/
│   │   └── orchestrator.go   # tmux 会话管理
│   └── config/
│       └── config.go         # 配置管理
├── go.mod
├── go.sum
└── README.md
```

## MVP 功能清单

### Phase 1: 核心功能 (当前)

- [x] Key Store 实现 (AES-GCM 加密)
- [ ] CLI Launcher TUI
- [ ] Directory Picker TUI
- [ ] Key Manager TUI
- [ ] Session Orchestrator (tmux 集成)
- [ ] 环境变量注入
- [ ] 基础命令行参数

### Phase 2: 增强功能 (后续)

- [ ] 历史目录记忆
- [ ] Session Dashboard
- [ ] Pane 支持 (日志/调试)
- [ ] 配置文件管理
- [ ] 自定义 Agent 支持

## 开发规范

### 代码风格

- 遵循 Go 官方代码规范
- 使用 `gofmt` 格式化代码
- 错误处理必须显式检查

### 测试要求

- 单元测试覆盖率 > 80%
- 关键路径必须有集成测试
- 使用 `go test -race` 检查并发问题

### 安全要求

- API Key 必须加密存储
- 配置文件权限 `0600`
- 不在日志中输出敏感信息
- 使用 `gitleaks` 检查敏感信息泄露

## 依赖管理

### 核心依赖

```go
require (
    github.com/google/uuid v1.6.0
    github.com/rivo/tview v0.0.0-20240101153935-32b1c8f8d4b1
    github.com/gdamore/tcell/v2 v2.7.0
    gopkg.in/yaml.v3 v3.0.1
)
```

### 外部工具

- **tmux**: 版本 >= 3.0
- **gitleaks**: 敏感信息检测
- **trivy**: 依赖漏洞扫描 (可选)

## 配置文件

### keys.yaml

```yaml
keys:
  - id: "uuid-1"
    provider: "claude"
    name: "key-claude"
    api_key: "encrypted-base64-string"
    tags: ["cache", "code"]
    active: true
    created_at: "2026-02-04T00:00:00Z"
```

### config.yaml (未来)

```yaml
default_dir: "/home/user/projects"
history_size: 10
agents:
  - name: "claude-code"
    command: "claude-code"
  - name: "codex-cli"
    command: "codex-cli"
```

## 构建与部署

### 构建命令

```bash
# 开发构建
go build -o agx ./cmd/agx

# 生产构建
go build -ldflags="-s -w" -o agx ./cmd/agx

# 跨平台构建
GOOS=linux GOARCH=amd64 go build -o agx-linux ./cmd/agx
GOOS=darwin GOARCH=arm64 go build -o agx-darwin ./cmd/agx
```

### 安装

```bash
# 安装到 $GOPATH/bin
go install ./cmd/agx

# 或手动复制
cp agx /usr/local/bin/
```

## 使用示例

### 首次使用

```bash
# 1. 设置加密密钥
export AGX_SECRET="your-32-byte-secret-key-here"

# 2. 添加 API Key
agx key add --provider claude --name key-claude --key sk-xxx --tags cache,code

# 3. 激活 Key
agx key activate <key-id>

# 4. 启动 AGX
agx
```

### 日常使用

```bash
# 直接启动 TUI
agx

# 或指定 Agent
agx --agent claude-code

# 或指定目录
agx --dir /path/to/project
```

## 故障排查

### 常见问题

1. **tmux session 无法创建**
   - 检查 tmux 是否安装: `which tmux`
   - 检查 tmux 版本: `tmux -V`

2. **API Key 解密失败**
   - 检查 `AGX_SECRET` 环境变量
   - 确保密钥长度为 32 字节

3. **TUI 显示异常**
   - 检查终端是否支持 256 色: `echo $TERM`
   - 尝试设置: `export TERM=xterm-256color`

## 参考资料

- [tview 文档](https://github.com/rivo/tview)
- [tmux 手册](https://man.openbsd.org/tmux)
- [Go AES-GCM 示例](https://pkg.go.dev/crypto/cipher#example-NewGCM)
