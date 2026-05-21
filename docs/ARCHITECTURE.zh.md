# 架构

English: [ARCHITECTURE.md](ARCHITECTURE.md).

`agx` 是本地中转聚合器：用同一份 profile（`base_url` + `api_key`）驱动 `codex` / `claude` / `gemini` / `opencode`。

## 目录布局

```
cmd/agx                       main 入口
internal/
├── domain/profile/           值类型与校验
├── ports/                    usecase 层依赖的接口
├── usecase/                  业务逻辑（ProfileService）
├── adapters/
│   ├── profilefile/          YAML 形式的 Profile + State 仓库
│   ├── codexconfig/          Codex TOML 同步
│   ├── claudeconfig/         Claude settings.json 同步
│   ├── geminiconfig/         Gemini .env 同步
│   ├── opencodeconfig/       OpenCode config.json 同步
│   ├── opjournal/            进行中操作的 journal
│   ├── lockfile/             mutation 互斥锁
│   └── fileutil/             原子写 + 存在时读
└── interfaces/cli/           Cobra 命令树 + native runtime
tests/
├── e2e/                      bats smoke 套件
└── interactive/              go-expect PTY 测试
```

六边形 / Clean 架构：依赖向内 —— CLI → usecase → ports；adapter 实现 ports，不反向引用。

## 核心概念

| 术语 | 含义 |
| --- | --- |
| **Profile** | 一个中转端点：`name`、`base_url`、`api_key`，可选 model。v0.2 起 `kind` 永远为 `relay`。 |
| **Binding** | 某个 agent 当前从哪个 profile 取配置，落在 agent 的 state 结构里。 |
| **Target** | 给某个 agent 激活的 managed profile，拥有独立 context 目录 `~/.config/agx/contexts/<agent>/<name>/` 与按 target 的 backup。 |
| **Managed runtime** | 通过 `ProfileService.SetManagedRuntime` 注入的 agent syncer 工厂，`agx run` 必需。 |
| **Mutation guard** | 修改前捕获 profile registry + state + agent 配置的快照，任一环节失败可整体回滚。 |

## 各层职责

### `domain/profile`

纯 Go 值类型与校验：`Profile`、`State`、`Agent`、`Backup`、`TargetState`，以及 `Normalize*` / `Validate*` 等 helper。零依赖。

### `ports/`

只有接口：

- `ProfileRepository` —— 对底层存储做 Profile CRUD
- `StateRepository` —— 单文档 `State` 的 load/save
- `MutationLocker` —— flock 风格的写互斥
- `OperationJournal` —— 进行中操作的记录
- `AgentSyncer` —— 5 个公共方法
  （`Snapshot` / `CreateBackup` / `Restore` / `RemoveConfig` / `DeleteBackup`）；
  各 agent 专属接口嵌入并加 `Sync` 签名

### `usecase/`

`ProfileService` 是唯一公开类型，内部组合 4 个逻辑关注点：

| 关注点 | 文件 | 责任 |
| --- | --- | --- |
| 中转 profile CRUD | `profile_service_profiles.go`、`..._targets.go`、`..._relay_bindings.go` | `Add` / `Edit` / `Remove` / `List`；target CRUD；binding 应用 |
| Agent ↔ profile binding | `..._bindings.go`、`..._restore.go`、`..._state.go`、`mutation_guard.go` | `AgentSet` / `AgentBind` / `Use` / `Clear` / `Backup` / `Restore` |
| Managed runtime / targets | `..._managed_profiles.go`、`..._managed_ops.go`、`..._managed_targets.go` | `ActivateManagedProfile` / `EditManagedProfile`；per-target context |
| Diagnostics | `..._doctor.go`、`..._runtime_state.go` | `Doctor` 报告；运行时 state 推断 |

横切 helper 在 `profile_service.go`（DTO + 构造函数）、
`mutation_guard.go`（capture + rollback）、
`agent_syncer_resolver.go`（agent → syncer 路由）、
`errors.go`（typed error）。

### `adapters/`

每个 agent 的 syncer 负责一种配置格式：

- **codexconfig** —— TOML；维护 "AGX managed" 块与根级 `profile = "<name>"`
- **claudeconfig** —— JSON `settings.json`；写 `apiKeyHelper` 与 `env.ANTHROPIC_BASE_URL`
- **geminiconfig** —— dotenv；写 `GOOGLE_GEMINI_BASE_URL` 与 `GEMINI_API_KEY`
- **opencodeconfig** —— JSON `config.json`；对每个 managed profile 写 3 个 provider（`agx-<name>-openai-compatible` / `-anthropic` / `-gemini`），共用同一份 base_url + api_key；`settings.model` 按 profile.model 名启发选 default provider。运行时切换协议交给 opencode 自身的 `/provider` 命令

横切设施：`profilefile/` 把 Profile + State 持久成 `~/.config/agx/profiles/`
下的 YAML；`lockfile/` 提供 flock；`opjournal/` 让 `agx doctor` 能识别崩溃残留。

### `interfaces/cli/`

Cobra 命令树。`RunE` boilerplate 收敛在 `cobra_shared_commands.go` 的 3 个 helper：

- `preflight(cmd, args, wantArgs)` —— 解析 `-o/--output` 并校验参数数量
- `reportError(err)` —— `printUserError` + `exitCodeError{code: 1}`
- `emitJSON(payload)` —— `exitCodeError{code: writeJSON(payload)}`

`native_runtime.go` 负责在 managed target 的 context 目录里 exec 原生 CLI。

## 命令链路示例：`agx use codex relay-a`

```
cmd/agx/main.go
  └─ Root.Execute(["use", "codex", "relay-a"])
       └─ newProfileUseCommand RunE
            ├─ r.preflight(cmd, args, 1)
            ├─ r.profiles.UseManagedProfile(args[0])
            └─ r.emitJSON / fmt.Fprintf

usecase.ProfileService.UseManagedProfile + ActivateManagedProfile:
  ├─ validateManagedActivation
  ├─ lockMutations()
  ├─ loadStoredState()
  ├─ prepareManagedTarget
  ├─ ensureManagedContextReady
  ├─ syncManagedRelayTarget
  │     └─ resolveAgentSyncer(agent).Sync(profile, …)
  │         └─ adapters/codexconfig 写 TOML
  └─ recordManagedActivation
```

## Mutation 保证

`ProfileService` 的每个修改类入口——包括 `Add` / `Edit` / `Remove`、
managed profile 流程（`AddManagedProfile` / `EditManagedProfile` /
`RemoveManagedProfile` / `UseManagedProfile` / `ActivateManagedProfile`）、
launcher target API（`upsertRelayTarget` / `UseTarget` / `RemoveTarget`）、
binding helper（`AgentSet` / `AgentBind` / `Clear`）、恢复表面
（`Backup` / `Restore` / `BackupManagedTarget` / `RestoreManagedTarget`）
——都走 `withMutationGuard`：

1. 拿到 mutation 锁（进程内 mutex + 磁盘 flock）
2. 捕获 pre-image：相关 profile YAML（含派生 `<agent>.<name>` profile）、
   整份 `State`、动作触及的各 agent 磁盘配置
3. 在同一把锁下运行 mutation 闭包
4. 失败时按 pre-image 还原；panic 时跑完 rollback 再重新 panic，让进程
   仍以非零退出
5. 动作 commit 后清掉 operation journal 条目

这就是半应用 sync 能被 `agx doctor` / `agx restore` 恢复的原因。
`renameManagedProfileTargets` 还额外记录它执行的每次目录 `os.Rename`
并在失败时倒回——因为磁盘上的 context 目录树不在 mutation guard 的
默认捕获范围内。

## 测试分层

- **单元** (`internal/**/*_test.go`) —— table-driven；使用
  `fakeProfileRepo`、`fakeStateRepo` 和各 agent 的 fake syncer
- **属性** (`property_test.go`) —— `pgregory.net/rapid` 校验 binding + state 不变式
- **交互** (`tests/interactive/`) —— launcher 命令的 `go-expect` PTY 用例
- **E2E** (`tests/e2e/cli-smoke.{bats,sh}`) —— 对编译产物的端到端测试

`task verify` 跑 build → lint → unit → interactive → smoke → bats。
CI 在此基础上加 `docker-bake.hcl` 里的 docker 矩阵。

## 扩展点

- **加 agent**：实现 `ports.AgentSyncer` 和 agent 专属扩展接口；在
  `resolveAgentSyncer` 加 case；注册到 `domainprofile.SupportedAgents` 与
  `SetManagedRuntime` 工厂
- **加修改类命令**：实现 `*Locked` worker，外层套 `withMutationGuard`
- **加 CLI 命令**：通过现有的 `preflight / reportError / emitJSON` helper
  接入 usecase 层，再补一个 bats case
