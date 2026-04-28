# AGX

AGX 用来管理第三方 relay / gateway relay，并把 relay 绑定并同步到 `codex`、`claude`、`gemini` 的用户级原生配置。
它只处理四件事：`relay name`、`base_url`、`api_key`、把 relay 绑定到目标 agent。

## 支持范围

- agent CLI: `codex`、`claude`、`gemini`
- 目标系统: `linux`、`darwin`
- 发布架构: `amd64`、`arm64`

## 安装

```bash
task setup
task build
task install
```

日常使用命令、运行时行为和文件路径见 `docs/user-guide.md`。

## 文档入口

- 使用说明与运行时行为: `docs/user-guide.md`
- 开发与验证流程: `CONTRIBUTING.md`
- 协作约定: `CODE_OF_CONDUCT.md`
- Agent 协作约束: `AGENTS.md`

更多命令和约束见 `docs/user-guide.md`。
