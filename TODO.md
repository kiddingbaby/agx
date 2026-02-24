# TODO List

Generated from: SPEC.md
Branch: feat/key-env-v1
Created: 2026-02-15 09:38:28

## Tasks

### [1] done - 抽离 domain/key 并重定义 KeyRepository 边界

- File: internal/domain/key/model.go, internal/domain/key/rules.go, internal/ports/key_repository.go, internal/usecase/key_service.go
- Description: 新建 key 领域模型与规则，`ports.KeyRepository` 改为依赖 domain/key，usecase 不再依赖 `internal/key`。
- Dependencies: []
- Steps:
  - [x] coding    @ 2026-02-15 13:19:43 (1ecc94d)
  - [x] tidy    @ 2026-02-15 13:21:00 (1ecc94d)
  - [x] lint    @ 2026-02-15 13:21:05 (1ecc94d)
  - [x] review    @ 2026-02-15 13:21:26 (1ecc94d)
  - [x] test    @ 2026-02-15 13:21:39 (1ecc94d)
  - [x] uat    @ 2026-02-15 13:21:52 (1ecc94d)
  - [x] scan    @ 2026-02-15 13:22:04 (1ecc94d)
  - [x] pr    @ 2026-02-15 13:22:29 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [2] done - 拆分 key 存储到 adapters/keyfile

- File: internal/adapters/keyfile/repository.go, internal/adapters/keyfile/crypto.go, internal/adapters/keyfile/repository_test.go
- Description: 将 YAML/AES-GCM 逻辑下沉到 adapter，实现新 `KeyRepository` 接口，清理旧 `internal/key` 直接领域暴露。
- Dependencies: [1]
- Steps:
  - [x] coding    @ 2026-02-15 13:52:00 (1ecc94d)
  - [x] tidy    @ 2026-02-15 13:52:15 (1ecc94d)
  - [x] lint    @ 2026-02-15 13:52:16 (1ecc94d)
  - [x] review    @ 2026-02-15 13:52:27 (1ecc94d)
  - [x] test    @ 2026-02-15 13:52:42 (1ecc94d)
  - [x] uat    @ 2026-02-15 13:52:55 (1ecc94d)
  - [x] scan    @ 2026-02-15 13:53:09 (1ecc94d)
  - [x] pr    @ 2026-02-15 13:54:04 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [3] done - 拆分 session runtime 到 adapters/tmux 并上移命名规则

- File: internal/adapters/tmux/runtime.go, internal/domain/session/naming.go,
  internal/usecase/session_service.go, internal/usecase/launch_service.go
- Description: tmux 细节下沉 adapter；`ai-` 命名规则迁移到 domain/usecase，避免 runtime 与业务语义耦合。
- Dependencies: [1]
- Steps:
  - [x] coding    @ 2026-02-15 13:52:00 (1ecc94d)
  - [x] tidy    @ 2026-02-15 13:53:55 (1ecc94d)
  - [x] lint    @ 2026-02-15 13:53:56 (1ecc94d)
  - [x] review    @ 2026-02-15 13:53:56 (1ecc94d)
  - [x] test    @ 2026-02-15 13:53:56 (1ecc94d)
  - [x] uat    @ 2026-02-15 13:53:56 (1ecc94d)
  - [x] scan    @ 2026-02-15 13:53:56 (1ecc94d)
  - [x] pr    @ 2026-02-15 13:54:04 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [4] done - 统一 usecase 错误模型与错误映射

- File: internal/usecase/errors.go, internal/usecase/key_service.go, internal/usecase/launch_service.go, internal/usecase/session_service.go
- Description: 统一定义 typed errors 与错误码，清理散落在 CLI/TUI 的重复错误判断分支。
- Dependencies: [1,2,3]
- Steps:
  - [x] coding    @ 2026-02-15 14:01:08 (1ecc94d)
  - [x] tidy    @ 2026-02-15 14:01:26 (1ecc94d)
  - [x] lint    @ 2026-02-15 14:01:27 (1ecc94d)
  - [x] review    @ 2026-02-15 14:01:43 (1ecc94d)
  - [x] test    @ 2026-02-15 14:02:05 (1ecc94d)
  - [x] uat    @ 2026-02-15 14:02:17 (1ecc94d)
  - [x] scan    @ 2026-02-15 14:02:37 (1ecc94d)
  - [x] pr    @ 2026-02-15 14:02:55 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [5] done - 新增 app/bootstrap 与 config/secret provider

- File: internal/app/bootstrap.go, internal/app/container.go, internal/config/paths.go, internal/config/secret_provider.go
- Description: 集中装配依赖并统一 secret/path 生命周期，替代 `main.go` 中分散初始化逻辑。
- Dependencies: [2,3]
- Steps:
  - [x] coding    @ 2026-02-15 14:01:08 (1ecc94d)
  - [x] tidy    @ 2026-02-15 14:01:27 (1ecc94d)
  - [x] lint    @ 2026-02-15 14:01:28 (1ecc94d)
  - [x] review    @ 2026-02-15 14:01:43 (1ecc94d)
  - [x] test    @ 2026-02-15 14:02:05 (1ecc94d)
  - [x] uat    @ 2026-02-15 14:02:17 (1ecc94d)
  - [x] scan    @ 2026-02-15 14:02:37 (1ecc94d)
  - [x] pr    @ 2026-02-15 14:02:55 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [6] done - 拆分 CLI interfaces 并收敛 main.go

- File: internal/interfaces/cli/root.go, internal/interfaces/cli/keys.go,
  internal/interfaces/cli/sessions.go, internal/interfaces/cli/launch.go,
  cmd/agx/main.go
- Description: CLI 命令解析迁移到 interfaces 层，`main.go` 只保留 bootstrap + command entry。
- Dependencies: [4,5]
- Steps:
  - [x] coding    @ 2026-02-15 14:09:33 (1ecc94d)
  - [x] tidy    @ 2026-02-15 14:09:42 (1ecc94d)
  - [x] lint    @ 2026-02-15 14:09:43 (1ecc94d)
  - [x] review    @ 2026-02-15 14:09:56 (1ecc94d)
  - [x] test    @ 2026-02-15 14:09:56 (1ecc94d)
  - [x] uat    @ 2026-02-15 14:09:57 (1ecc94d)
  - [x] scan    @ 2026-02-15 14:09:57 (1ecc94d)
  - [x] pr    @ 2026-02-15 14:09:57 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [7] done - 拆分 TUI interfaces 并移除 adapter 直连

- File: internal/interfaces/tui/app.go, internal/interfaces/tui/dashboard/model.go,
  internal/interfaces/tui/keymgr/model.go, internal/tui/dashboard.go,
  internal/tui/keymgr.go, cmd/agx/tui.go
- Description: 将 TUI 生命周期入口迁入 interfaces 层，并仅通过 usecase 访问业务能力；页面模型当前由 `internal/tui` 提供并在 `internal/interfaces/tui` 复用。
- Dependencies: [4,5]
- Steps:
  - [x] coding    @ 2026-02-15 14:09:33 (1ecc94d)
  - [x] tidy    @ 2026-02-15 14:09:43 (1ecc94d)
  - [x] lint    @ 2026-02-15 14:09:44 (1ecc94d)
  - [x] review    @ 2026-02-15 14:09:56 (1ecc94d)
  - [x] test    @ 2026-02-15 14:09:56 (1ecc94d)
  - [x] uat    @ 2026-02-15 14:09:57 (1ecc94d)
  - [x] scan    @ 2026-02-15 14:09:57 (1ecc94d)
  - [x] pr    @ 2026-02-15 14:09:58 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

### [8] done - 文档收敛与回归基线固化

- File: README.md, docs/architecture.md, docs/directory-structure.md, docs/refactor-roadmap.md, tests/integration/, tests/e2e/
- Description: 更新文档到新目录与调用链，补齐最小 smoke 脚本，固定重构回归基线。
- Dependencies: [6,7]
- Steps:
  - [x] coding    @ 2026-02-15 14:13:01 (1ecc94d)
  - [x] tidy    @ 2026-02-15 14:13:10 (1ecc94d)
  - [x] lint    @ 2026-02-15 14:13:11 (1ecc94d)
  - [x] review    @ 2026-02-15 14:13:11 (1ecc94d)
  - [x] test    @ 2026-02-15 14:13:12 (1ecc94d)
  - [x] uat    @ 2026-02-15 14:13:12 (1ecc94d)
  - [x] scan    @ 2026-02-15 14:13:12 (1ecc94d)
  - [x] pr    @ 2026-02-15 14:13:12 (1ecc94d)
- Created: 2026-02-15 09:38:28
- Updated: 2026-02-15 09:38:28

## Status

- Total: 8
- Pending: 0
- In Progress: 0
- Done: 8

## Completion Scope

- 以上状态仅表示重构任务清单已执行完成，不等价于所有产品目标 100% 达成。
- 当前剩余差距以 `SPEC.md` 的 `Current Gaps` 为准。

## Notes

- Use `$coding <task-id>` to start implementing a task
- Task status: pending | in_progress | done
