# DESIGN: AGX 技术架构重构（Code-First）

## Problem Statement

当前 AGX 已经引入 `domain/ports/usecase` 雏形，但仍处于“半分层”：

1. `cmd/agx/main.go` 过载（初始化 + 路由 + 业务错误分流混在一起）。
2. `internal/key/store.go` 混合领域模型、存储实现、加密逻辑，边界不清。
3. key/session 核心路径可测试性不足，扩展新 agent/runtime 成本高。

结果是：新增能力时容易触发跨层改动，回归风险和维护成本持续上升。

## User Stories

- 作为产品工程师，我希望新增 agent 只改领域配置，不改 CLI/TUI 主流程，从而降低功能扩展成本。
- 作为维护者，我希望 usecase 在无 tmux 环境下可稳定测试，从而避免基础设施依赖导致回归不稳定。
- 作为架构维护者，我希望 key 存储实现可替换（YAML -> 其他），从而支持未来演进。

## Key Constraints

1. 暂不考虑历史兼容性，以架构清晰优先。
2. 技术栈保持 Go + Bubble Tea + tmux。
3. 安全底线不降低：明文 key 不写日志、不拼接到命令串、不输出到错误信息。

## Target Architecture

采用五层结构并强约束依赖方向：

```text
Interfaces (CLI/TUI)
    -> Usecase
        -> Domain
            -> Ports
                -> Adapters (tmux / keyfile / os secret)
```

### Layer Responsibilities

1. **Interfaces**：参数解析、交互状态、文案展示；不含业务规则。
2. **Usecase**：业务编排和错误归一；不依赖具体 IO 实现。
3. **Domain**：稳定核心模型与规则（agent/key/session）。
4. **Ports**：外部能力抽象（repository/runtime/secret provider）。
5. **Adapters**：实现端口，处理 tmux、文件系统、yaml、crypto 细节。

## Core Design Decisions

1. 引入 `domain/key`，把 `Provider/Key` 从 `internal/key` 中抽离。
2. 将 `internal/key` 下沉为 `internal/adapters/keyfile`，只做持久化与加密。
3. 将 `internal/session` 下沉为 `internal/adapters/tmux`，业务命名规则上移到 domain/usecase。
4. 增加 `internal/app/bootstrap`，统一依赖装配（config/repo/runtime/service）。
5. CLI/TUI 只依赖 usecase/domain，不直接依赖 adapter concrete type。

## Design Scope

### In Scope (P0/P1)

- P0: 入口解耦（薄 `main.go` + interfaces 拆分）。
- P0: key/session 架构分层完整化（domain/ports/usecase/adapters）。
- P0: usecase 可脱离 tmux 做单测。
- P1: key repository 可替换，且不影响 CLI/TUI 代码。

### Out of Scope

- UI 视觉大改或交互重做。
- 云端 secret manager 集成。
- 多租户/多用户权限模型。

## Risks and Mitigations

1. **行为漂移风险**：重构后 CLI/TUI 行为偏移。  
   - 缓解：固定 smoke 基线（`agx --help`、`agx keys ls`、`agx ls`、典型 launch 错误路径）。
2. **密钥泄漏风险**：重构过程中错误输出暴露敏感信息。  
   - 缓解：统一脱敏策略，禁止在日志/错误中输出明文 key。
3. **重构失控风险**：一次性改动过大。  
   - 缓解：分 Phase 独立提交，单阶段可回滚。

## Acceptance Criteria

- [ ] 新增一个 agent 只改 `domain/agent` 与文档。
- [ ] usecase 关键链路测试不依赖 tmux。
- [ ] `cmd/agx/main.go` 不再承载业务规则，仅做装配和入口分发。
- [ ] key 存储实现替换不影响 interfaces 层。
- [ ] `go test ./...` 持续通过。
