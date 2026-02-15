# AGX 目录结构（Current Baseline）

## 1. 当前目录树

```text
agx/
├── cmd/
│   └── agx/
│       ├── main.go
│       └── tui.go
├── internal/
│   ├── adapters/
│   │   ├── keyfile/
│   │   │   ├── crypto.go
│   │   │   └── repository.go
│   │   └── tmux/
│   │       └── runtime.go
│   ├── app/
│   │   ├── bootstrap.go
│   │   └── container.go
│   ├── config/
│   │   ├── paths.go
│   │   └── secret_provider.go
│   ├── domain/
│   │   ├── agent/
│   │   ├── key/
│   │   └── session/
│   ├── interfaces/
│   │   ├── cli/
│   │   └── tui/
│   ├── ports/
│   ├── usecase/
│   └── tui/          # Bubble Tea model 实现
├── docs/
├── tests/
│   ├── integration/
│   └── e2e/
├── DESIGN.md
├── SPEC.md
└── TODO.md
```

## 2. 依赖规则（落地约束）

1. `domain` 不依赖 usecase/adapters/interfaces
2. `usecase` 仅依赖 `domain + ports`
3. `interfaces` 不直接依赖 adapters
4. `cmd` 不承载业务逻辑，仅入口与 wiring
5. adapters 只通过 ports 向上暴露能力

## 3. 关键迁移结果

| 旧位置 | 新位置 | 状态 |
|---|---|---|
| `internal/key/store.go`（实现） | `internal/adapters/keyfile/*` | 已迁移，不保留兼容层 |
| `internal/session/orchestrator.go`（实现） | `internal/adapters/tmux/runtime.go` | 已迁移，不保留兼容层 |
| `cmd/agx/main.go`（命令解析） | `internal/interfaces/cli/*` | 已迁移 |
| `cmd/agx/tui.go`（生命周期） | `internal/interfaces/tui/app.go` | 已迁移 |
| 分散错误定义 | `internal/usecase/errors.go` | 已统一 |

## 4. 兼容策略

- 本项目当前阶段不维护历史兼容层
- 新代码统一依赖 `domain/ports/usecase/adapters/interfaces`
