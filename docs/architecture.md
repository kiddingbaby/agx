# AGX 架构（As-Is，2026-02-15）

> 本文以当前仓库代码为准。

## 1. 分层结构

```text
cmd/agx
  -> internal/interfaces/{cli,tui}
    -> internal/app/bootstrap
      -> internal/usecase
        -> internal/domain + internal/ports
          -> internal/adapters/{keyfile,tmux}
```

## 2. 关键职责

| 层 | 目录 | 职责 |
|---|---|---|
| cmd | `cmd/agx` | 进程入口与退出码 |
| interfaces | `internal/interfaces/cli`, `internal/interfaces/tui` | 参数解析、交互流程、错误展示 |
| app | `internal/app` | 依赖装配（paths/secret/repo/runtime/services） |
| usecase | `internal/usecase` | 业务编排与统一错误模型 |
| domain | `internal/domain` | 核心模型与规则（agent/key/session naming） |
| ports | `internal/ports` | 外部能力抽象（repo/runtime/provider） |
| adapters | `internal/adapters` | IO/系统实现（YAML+AES-GCM、tmux） |

## 3. 关键调用链

### 3.1 CLI 启动

1. `cmd/agx/main.go` 调 `app.Bootstrap()`
2. 生成 `interfaces/cli.Root`
3. `Root.Execute(args)` 分发到 keys/sessions/launch
4. 通过 usecase 调用 ports，最终落到 adapters

### 3.2 TUI 启动

1. `cmd/agx/tui.go` 调 `interfaces/tui.RunDashboard`
2. Dashboard/KeyManager 模型复用 `internal/tui/*`
3. 仅依赖 usecase，不直接 import adapters

### 3.3 Launch 链路

1. `LaunchService.BuildSessionConfig`
2. 从 `domain/agent` 解析 provider/env
3. 从 `KeyService.GetActive` 获取解密 key
4. 通过 `domain/session.SessionName` 生成 `ai-*`
5. `tmux.Runtime.Launch` 执行会话操作

## 4. 错误模型（统一）

`internal/usecase/errors.go` 统一定义并复用：

- `UnknownAgentError`
- `NoActiveKeyError`
- `KeyNotFoundError`
- `RuntimeError`

接口层统一通过 `errors.As/Is` 映射文案，避免散落分支。

## 5. 安全边界

- API Key 明文仅存在于 usecase 运行时内存
- 持久化使用 AES-GCM（adapter/keyfile）
- 不将明文 key 注入命令串，使用 tmux session env 注入
- secret 生命周期统一由 `config.SecretProvider` 管理

## 6. 兼容策略

本仓库当前不保留历史兼容层，直接使用新分层路径：

- key 存储实现：`internal/adapters/keyfile`
- tmux runtime 实现：`internal/adapters/tmux`
