# Architecture

中文：[ARCHITECTURE.zh.md](ARCHITECTURE.zh.md).

`agx` is a local relay aggregator: one profile (`base_url` + `api_key`)
drives `codex`, `claude`, `gemini`, and `opencode`.

## Layout

```
cmd/agx                       main entrypoint
internal/
├── domain/profile/           value types + validation
├── ports/                    interfaces the usecase layer depends on
├── usecase/                  business logic (ProfileService)
├── adapters/
│   ├── profilefile/          YAML Profile + State repositories
│   ├── codexconfig/          Codex TOML sync
│   ├── claudeconfig/         Claude settings.json sync
│   ├── geminiconfig/         Gemini .env sync
│   ├── opencodeconfig/       OpenCode config.json sync
│   ├── opjournal/            in-flight operation journal
│   ├── lockfile/             mutation mutex
│   └── fileutil/             atomic write + read-if-exists
└── interfaces/cli/           Cobra command tree + native runtime
tests/
├── e2e/                      bats smoke suite
└── interactive/              go-expect PTY tests
```

Hexagonal / clean architecture. Dependencies point inward: CLI → usecase →
ports; adapters implement ports without importing usecase or CLI.

## Concepts

| Term | Meaning |
| --- | --- |
| **Profile** | A relay endpoint: `name`, `base_url`, `api_key`, optional model. `kind` is always `relay` since v0.2. |
| **Binding** | An agent's record of which profile drives its config. Lives on the per-agent state struct. |
| **Target** | A managed profile activated for one agent. Owns `~/.config/agx/contexts/<agent>/<name>/` and per-target backups. |
| **Managed runtime** | Agent syncer factories wired via `ProfileService.SetManagedRuntime`. Required for `agx run`. |
| **Mutation guard** | Pre-mutation snapshot of profile registry, state, and agent configs. Rolls back on any failure. |

## Layers

### `domain/profile`

Pure Go value types and validators (`Profile`, `State`, `Agent`, `Backup`,
`TargetState`, plus `Normalize*` / `Validate*` helpers). Zero external deps.

### `ports/`

Interfaces only. Notable ones:

- `ProfileRepository` — Profile CRUD against a backing store
- `StateRepository` — single-document `State` load/save
- `MutationLocker` — flock-style mutual exclusion
- `OperationJournal` — in-flight operation record for crash-safety
- `AgentSyncer` — shared 5-op surface
  (`Snapshot` / `CreateBackup` / `Restore` / `RemoveConfig` / `DeleteBackup`);
  per-agent ports (`CodexSyncer`, `ClaudeSyncer`, `GeminiSyncer`,
  `OpenCodeSyncer`) embed it and add their `Sync` signature

### `usecase/`

`ProfileService` is the single public façade. Internally it composes four concerns:

| Concern | Files | Responsibilities |
| --- | --- | --- |
| Relay profile CRUD | `profile_service_profiles.go`, `..._targets.go`, `..._relay_bindings.go` | `Add` / `Edit` / `Remove` / `List`; target CRUD; binding application |
| Agent ↔ profile binding | `..._bindings.go`, `..._restore.go`, `..._state.go`, `mutation_guard.go` | `AgentSet` / `AgentBind` / `Use` / `Clear` / `Backup` / `Restore` |
| Managed runtime / targets | `..._managed_profiles.go`, `..._managed_ops.go`, `..._managed_targets.go` | `ActivateManagedProfile` / `EditManagedProfile`; per-target context roots |
| Diagnostics | `..._doctor.go`, `..._runtime_state.go` | `Doctor` report; runtime-state inference |

Cross-cutting helpers live in `profile_service.go` (DTOs + constructor),
`mutation_guard.go` (capture + rollback), `agent_syncer_resolver.go`
(agent → syncer lookup), and `errors.go` (typed errors).

### `adapters/`

Per-agent syncers wrap one config format each:

- **codexconfig** — TOML; maintains an "AGX managed" block plus root `profile = "<name>"`
- **claudeconfig** — JSON `settings.json`; writes `apiKeyHelper` and `env.ANTHROPIC_BASE_URL`
- **geminiconfig** — dotenv; writes `GOOGLE_GEMINI_BASE_URL` and `GEMINI_API_KEY`
- **opencodeconfig** — JSON `config.json`; manages a provider plus `model`

Cross-cutting: `profilefile/` persists YAML under `~/.config/agx/profiles/`;
`lockfile/` provides flock; `opjournal/` records in-flight ops for `agx doctor`.

### `interfaces/cli/`

Cobra command tree. `RunE` boilerplate collapses into three helpers in
`cobra_shared_commands.go`:

- `preflight(cmd, args, wantArgs)` — parses `-o/--output` and checks arg count
- `reportError(err)` — `printUserError` plus `exitCodeError{code: 1}`
- `emitJSON(payload)` — `exitCodeError{code: writeJSON(payload)}`

`native_runtime.go` execs each agent's native CLI inside the managed target's context root.

## Command flow: `agx use codex relay-a`

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
  │         └─ adapters/codexconfig writes TOML
  └─ recordManagedActivation
```

## Mutation guarantees

Every mutating entry point in `ProfileService` — that includes
`Add` / `Edit` / `Remove`, the managed-profile flow
(`AddManagedProfile` / `EditManagedProfile` / `RemoveManagedProfile` /
`UseManagedProfile` / `ActivateManagedProfile`), the launcher target
API (`upsertRelayTarget` / `UseTarget` / `RemoveTarget`), the binding
helpers (`AgentSet` / `AgentBind` / `Clear`), and the recovery surface
(`Backup` / `Restore` / `BackupManagedTarget` /
`RestoreManagedTarget`) — goes through `withMutationGuard`, which:

1. Takes the mutation lock (in-process mutex + on-disk flock)
2. Captures pre-images of the relevant profile YAMLs (including derived
   `<agent>.<name>` profiles), the full `State`, and per-agent on-disk
   configs touched by the action
3. Runs the mutation closure under the same lock
4. On error, restores each captured pre-image; on panic, runs the
   rollback and re-raises the panic so the process still exits non-zero
5. Clears the operation journal entry once the action commits

This is what makes half-applied syncs recoverable by `agx doctor` /
`agx restore`. The `renameManagedProfileTargets` helper additionally
records every directory `os.Rename` it performs and undoes them on
failure, since the on-disk context tree is outside the mutation guard's
default capture set.

## Testing

- **Unit** (`internal/**/*_test.go`) — table-driven; uses `fakeProfileRepo`, `fakeStateRepo`, and per-agent fake syncers
- **Property** (`property_test.go`) — `pgregory.net/rapid` invariants on bindings + state
- **Interactive** (`tests/interactive/`) — `go-expect` PTY drills for launcher commands
- **E2E** (`tests/e2e/cli-smoke.{bats,sh}`) — end-to-end against the built binary

`task verify` runs build → lint → unit → interactive → smoke → bats.
CI mirrors that plus the Docker matrix in `docker-bake.hcl`.

## Extension points

- **New agent**: implement `ports.AgentSyncer` + the agent-specific extension
  interface; add a `case` in `resolveAgentSyncer`; register it in
  `domainprofile.SupportedAgents` and the `SetManagedRuntime` factory
- **New mutating command**: write a `*Locked` worker and wrap it with
  `withMutationGuard`
- **New CLI command**: add `r.newFooCommand()` using
  `preflight / reportError / emitJSON`, route it to the usecase layer, and
  add a bats case
