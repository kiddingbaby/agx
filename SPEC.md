# SPEC: AGX 架构重构（Layered Architecture Refactor）

## Overview

本次交付将现有“半分层”代码收敛为完整分层架构，目标是降低维护成本并提升可扩展性与可测试性。
截至 2026-02-24，核心重构任务已完成，本文档同步记录现状与剩余差距。

## Design Reference

> 产品与架构决策见 `DESIGN.md`。  
> 当前代码事实与重构蓝图参考：`docs/architecture.md`、`docs/directory-structure.md`、`docs/refactor-roadmap.md`。

## Goals

- [ ] Goal 1: 新增 Agent 仅修改 `internal/domain/agent`
  （未完全达成：`internal/interfaces/cli/root.go` 与 `internal/interfaces/cli/launch.go` 仍有 agent 列表文案硬编码）。
- [x] Goal 2: `usecase` 测试可在无 tmux 环境稳定通过。
- [x] Goal 3: key 存储实现可替换（YAML -> 其他）且不影响 interfaces 层。
- [x] Goal 4: `cmd/agx/main.go` 收敛为薄入口（仅装配+分发）。
- [ ] Goal 5: 密钥明文不进入日志/命令串/错误输出（部分达成：未写入日志/错误；运行期仍通过 `tmux set-environment` 参数传递 secret，需继续评估本机进程可见性风险）。

## Non-Goals

- 不做 UI 视觉重构与交互重做。
- 不接入外部 Secret Manager（Vault/1Password/KMS）。
- 不实现历史兼容迁移层（按约束可直接重构）。

## Historical Background (Before Refactor)

当前主要问题（以代码为准）：

1. `cmd/agx/main.go` 过载，业务路由与初始化耦合。
2. `internal/key/store.go` 混合领域、仓储、加密职责。
3. `internal/session/orchestrator.go` 同时承担业务命名规则和 tmux 细节。
4. `ports.KeyRepository` 暴露 `internal/key` 类型，边界泄漏。

## Design

### Architecture

目标依赖方向：

```text
Interfaces (cli/tui)
  -> Usecase
    -> Domain
      -> Ports
        -> Adapters (keyfile/tmux/ossecret)
```

必须满足：

1. `domain` 不 import 其他业务层。
2. `usecase` 仅依赖 `domain + ports`。
3. `interfaces` 不直接依赖 adapters concrete。
4. `cmd` 只负责启动装配与入口分发。

### API Changes

1. `ports.KeyRepository` 参数/返回类型从 `internal/key` 切换到 `internal/domain/key`。
2. 新增 `usecase/errors.go` 统一错误类型（如 UnknownAgent / NoActiveKey / KeyNotFound / RuntimeError）。
3. 新增 `internal/app/bootstrap` 暴露 container（聚合 services + interfaces 启动依赖）。

### Data Model

1. 新增 `internal/domain/key/model.go`：
   - `Provider`
   - `Key`（业务字段，不包含文件序列化细节）
2. `internal/adapters/keyfile` 内部保留 YAML 编解码结构，避免 domain 被持久化细节污染。
3. `internal/domain/session` 增加命名规则函数（`SessionName(agent)`）。

## Implementation Plan

1. [x] Step 1: 引入 `domain/key`，重写 `ports.KeyRepository`，调整 `usecase/key_service` 依赖。
2. [x] Step 2: 拆分 `internal/key` 到 `internal/adapters/keyfile`（仓储 + crypto），并补齐测试。
3. [x] Step 3: 下沉 `internal/session` 到 `internal/adapters/tmux`，把会话命名规则上移到 domain/usecase。
4. [x] Step 4: 引入 `internal/app/bootstrap` + `internal/config`，收敛 secret/path 生命周期。
5. [x] Step 5: 拆分 CLI 到 `internal/interfaces/cli`，将 `main.go` 收敛为薄入口。
6. [x] Step 6: 拆分 TUI 入口到 `internal/interfaces/tui/app.go`，移除对 adapter 的直接依赖。
7. [x] Step 7: 统一 usecase 错误模型并同步 CLI/TUI 错误映射。
8. [x] Step 8: 更新 README/docs 与 smoke 测试脚本，锁定回归基线。

## Testing Strategy

- [x] Unit tests: `domain/*` 纯规则测试；`usecase/*` 使用 fake ports。
- [x] Integration tests: `adapters/keyfile`（文件+加密）、`adapters/tmux`（有 tmux 时运行）。
- [x] Regression tests: `go test ./...` + CLI smoke（`agx --help`、`agx keys ls`、`agx ls`、无 active key launch 错误路径）。

## Current Gaps

1. Goal 1 尚未完全达成：agent 列表文案仍有重复定义。
2. Goal 5 尚未完全达成：secret 仍通过 `tmux set-environment` 参数注入，需进一步收敛暴露面。

## Security Considerations

1. 禁止将 API Key 明文拼接进 shell command。
2. 错误与日志默认脱敏，禁止打印完整 key。
3. secret 获取统一入口（env/file），避免多点读取导致策略漂移。

## Rollback Plan

1. 每个阶段独立提交（按 TODO task 粒度），可逐步回滚。
2. 若 Phase N 异常，回滚该阶段提交并保留前序阶段。
3. 保持 smoke 基线命令可快速验证回滚后行为。

## References

- `DESIGN.md`
- `docs/architecture.md`
- `docs/directory-structure.md`
- `docs/refactor-roadmap.md`
