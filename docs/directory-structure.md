# AGX 目录结构（Current Baseline）

## 1. 当前目录树

```text
agx/
├── cmd/
│   └── agx/
│       └── main.go
├── internal/
│   ├── adapters/
│   │   ├── configfile/     # providers.yaml
│   │   ├── keyfile/        # keys.yaml + AES-GCM
│   │   └── toolconfig/     # sync into native CLI configs
│   ├── app/
│   │   ├── bootstrap.go
│   │   └── container.go
│   ├── config/
│   │   ├── paths.go
│   │   └── secret_provider.go
│   ├── domain/
│   │   ├── agent/
│   │   ├── key/
│   │   ├── provider/
│   ├── interfaces/
│   │   └── cli/
│   ├── ports/
│   ├── usecase/
├── docs/
│   ├── README.md
│   ├── architecture.md
│   ├── directory-structure.md
│   ├── refactor-roadmap.md
│   └── workflow.md
├── tests/
│   ├── integration/
│   └── e2e/
```

## 2. 依赖规则（落地约束）

1. `domain` 不依赖 usecase/adapters/interfaces
2. `usecase` 仅依赖 `domain + ports`
3. `interfaces` 不直接依赖 adapters
4. `cmd` 不承载业务逻辑，仅入口与 wiring
5. adapters 只通过 ports 向上暴露能力

## 3. 关键迁移结果

| 旧位置 | 新位置 | 状态 |
| --- | --- | --- |
| `internal/key/store.go`（实现） | `internal/adapters/keyfile/*` | 已迁移，不保留兼容层 |
| `cmd/agx/main.go`（命令解析） | `internal/interfaces/cli/*` | 已迁移 |
| 各 CLI 原生配置写入 | `internal/adapters/toolconfig/*` | 已迁移 |
| 分散错误定义 | `internal/usecase/errors.go` | 已统一 |

## 4. 兼容策略

- 本项目当前阶段不维护历史兼容层
- 新代码统一依赖 `domain/ports/usecase/adapters/interfaces`
