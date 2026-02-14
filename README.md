# AGX

AI CLI Session Orchestrator - 基于 tmux 的 AI CLI 会话编排工具。

## 功能特性

- **多 Agent 支持**: claude-code, codex-cli, gemini-cli
- **安全密钥管理**: AES-GCM 加密存储 API Key
- **tmux 会话管理**: 自动创建 session/window，环境变量注入
- **Vim 键位**: 全程支持 hjkl 导航

## 安装

### 从源码构建

```bash
# 克隆仓库
git clone https://github.com/kiddingbaby/agx.git
cd agx

# 构建
go build -o agx ./cmd/agx

# 安装到 PATH
sudo mv agx /usr/local/bin/
```

### 依赖

- Go 1.22+
- tmux 3.0+

## 快速开始

### 1. 初始化（默认自动托管 secret）

```bash
# 首次运行会自动生成并保存：
# ~/.config/agx/secret (0600)
agx --help >/dev/null
```

可选：如果你希望覆盖默认 secret，可手动设置 `AGX_SECRET`（必须是 32 字节）。

### 2. 添加 API Key

启动 AGX 后按 `K` 进入 Key 管理界面：

| 快捷键 | 功能 |
|--------|------|
| `a` | 添加 Key |
| `d` | 删除 Key |
| `Enter` | 激活 Key |
| `Esc` | 返回 |

说明：AGX 会在启动 agent 时自动注入所需环境变量（如 `OPENAI_API_KEY`），无需手动修改 `~/.bashrc`/`~/.zshrc`。

### 3. 启动会话

```bash
agx
```

1. 选择 Agent (↑↓ 或 j/k)
2. 选择工作目录 (hjkl 导航)
3. 按 Enter 启动 tmux 会话

## 使用说明

### TUI 导航

| 界面 | 快捷键 | 功能 |
|------|--------|------|
| 全局 | `K` | 打开 Key 管理 |
| 全局 | `q` | 退出 |
| 列表 | `j/k` | 上下移动 |
| 目录树 | `h/l` | 折叠/展开 |
| 所有 | `Enter` | 确认 |
| 所有 | `Esc` | 返回/取消 |

### 命令行参数

```bash
agx [options]

Options:
  --agent string   指定 Agent (claude-code, codex-cli, gemini-cli)
  --dir string     指定工作目录
  --keys           直接打开 Key 管理
```

### tmux 会话

AGX 创建的 tmux 会话命名规则：

- Session: `ai-<agent>` (如 `ai-claude-code`)
- Window: `<目录名>`

常用 tmux 命令：

```bash
# 列出会话
tmux ls

# 附加到会话
tmux attach -t ai-claude-code

# 切换窗口
Ctrl+b n  # 下一个
Ctrl+b p  # 上一个

# 分离会话
Ctrl+b d
```

## 配置

### 环境变量

| 变量 | 说明 | 必需 |
|------|------|------|
| `AGX_SECRET` | 32 字节加密密钥（可选覆盖） | ❌ |

### 数据存储

```
~/.config/agx/
├── keys.yaml    # 加密的 API Key 存储
└── secret       # 自动生成的 32 字节主密钥 (0600)
```

## 支持的 AI CLI

| Agent | 命令 | 环境变量 |
|-------|------|----------|
| claude-code | `claude` | `ANTHROPIC_API_KEY` |
| codex-cli | `codex` | `OPENAI_API_KEY` |
| gemini-cli | `gemini` | `GOOGLE_API_KEY` |

## 故障排查

### tmux 会话无法创建

```bash
# 检查 tmux 是否安装
which tmux

# 检查版本
tmux -V
```

### API Key 解密失败

```bash
# 1) 优先检查自动托管 secret 是否存在
ls -l ~/.config/agx/secret

# 2) 如手动设置 AGX_SECRET，长度必须为 32
echo -n "$AGX_SECRET" | wc -c
```

### TUI 显示异常

```bash
# 设置终端类型
export TERM=xterm-256color
```

## 开发

### 构建

```bash
go build -o agx ./cmd/agx
```

### 测试

```bash
go test ./... -v
```

### 项目结构

```
agx/
├── cmd/agx/main.go              # 入口
├── internal/
│   ├── key/store.go             # Key 加密存储
│   ├── tui/
│   │   ├── launcher.go          # Agent 选择
│   │   ├── dirpicker.go         # 目录选择
│   │   └── keymgr.go            # Key 管理
│   └── session/orchestrator.go  # tmux 管理
├── SPEC.md                      # 技术规范
└── TODO.md                      # 任务列表
```

## License

MIT
