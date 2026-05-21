# CLI Contract

This document specifies the **public contract** for the `agx` CLI:
- exit codes
- JSON output schemas
- stdout / stderr conventions

These signatures are part of agx's stability promise (see
[COMPATIBILITY.md](../COMPATIBILITY.md)). Internal state file layout
(`~/.config/agx/...`) is **not** covered here.

中文版本：[cli-contract.zh.md](cli-contract.zh.md).

---

## Exit codes

| Code | Meaning |
| --- | --- |
| `0` | Success |
| `1` | agx error (validation, IO, profile not found, mutation rejected) |
| `<exit code of native CLI>` | Launcher (`agx run <agent>`, `agx <agent>`) forwards the underlying CLI's exit code verbatim |

The `--version` flag, `--help` flag, and unknown subcommands always exit `0`
when help is requested or `1` when help is forced due to invalid usage.

## Streams

- **stdout** — primary output (table / JSON). Safe to pipe.
- **stderr** — errors, warnings, suggestions, deprecation notices. Always
  prefixed `Error:` or `warning:`.
- **stdin** — only consumed by `agx __api-key` (internal); user-facing
  commands never block on stdin.

When `-o json` is set, stdout is guaranteed to be a **single line of JSON
terminated by `\n`**. Diagnostic text still goes to stderr.

## Output formats

Every mutating / listing command accepts `-o json`. Omitting `-o` gives a
table or plain text intended for humans.

```bash
agx ls -o json | jq '.profiles[] | select(.current).name'
agx show work -o json | jq -r .profile.base_url
agx doctor -o json | jq '.issues[] | select(.severity == "error")'
```

`-o` is reserved for future formats (`yaml` may be added). Anything other
than `json` currently produces `Error: -o requires value json` on stderr
and exit `1`.

---

## Command schemas

### `agx add <name> --base-url URL --api-key KEY [--model MODEL] [--codex-wire-api chat|responses]`

```json
{
  "profile": {
    "name": "work",
    "kind": "relay",
    "current": false,
    "base_url": "https://relay.example/v1",
    "api_key": "sk-...",
    "credential_ref": "api_key",
    "model": "",
    "codex_wire_api": "",
    "provider_family": ""
  }
}
```

Empty `codex_wire_api` means "use the adapter default" (`responses`); pass `chat` to pin the profile to OpenAI Chat Completions wire.

### `agx edit <name> [flags]`

```json
{
  "profile":         { /* managedProfileView, same shape as add */ },
  "resynced_targets": [
    { "agent": "codex", "target": "work", "config_path": "..." }
  ],
  "failed_targets":   [
    { "agent": "opencode", "target": "work", "error": "..." }
  ]
}
```

Both arrays are omitted when empty.

### `agx rm <name>`

```json
{ "profile": { /* managedProfileView, the removed profile */ } }
```

### `agx ls [--all]`

```json
{
  "current": "work",
  "profiles": [
    { /* managedProfileView */ }
  ]
}
```

`current` is omitted when no profile is selected.

### `agx show <name>`

```json
{ "profile": { /* managedProfileView */ } }
```

### `agx use <name>`

```json
{ "profile": { /* managedProfileView */ } }
```

### `agx current`

```json
{ "profile": { /* managedProfileView */ } | null }
```

`profile` is `null` when no profile is selected.

### `agx detach <agent> <profile>`

```json
{ "agent": "codex", "profile": "work" }
```

### `agx backup <agent>`

```json
{
  "agent":  "codex",
  "backup": {
    "id": "20260520T101011Z",
    "target_kind": "relay",
    "target_name": "work",
    "path": "/home/.../backups/managed/codex/relay/work/20260520T101011Z",
    "created_at": "2026-05-20T10:10:11Z"
  }
}
```

### `agx restore <agent>`

Same shape as `agx backup`. The restored snapshot is returned in `backup`.

### `agx doctor`

```json
{
  "ok": false,
  "operation": {
    "id": "set-codex-20260520T1010...",
    "command": "set",
    "agent": "codex",
    "profile": "work",
    "stage": "started"
  },
  "issues": [
    {
      "severity": "error",
      "code": "unfinished_operation",
      "message": "...",
      "action": "agx restore codex"
    }
  ]
}
```

`operation` is omitted when no recent operation crashed. `issues` is `[]`
when everything is healthy.

### `agx version`

Plain text only. The schema is:

```
agx <version>
commit=<git sha>
date=<RFC3339 build time>
go=<go runtime>
os=<linux|darwin>
arch=<amd64|arm64>
```

### `agx completion {bash|zsh|fish|powershell}`

Plain text only — emits a shell completion script to stdout. The
underlying generator is Cobra's built-in.

### `agx run <agent> [profile] [-- <native args...>]` / `agx <agent> [...]`

Forwards stdin / stdout / stderr to the native CLI. agx itself prints
nothing to stdout in this mode. Exit code = native CLI exit code.

Profile resolution precedence (highest first):

1. Positional `profile` argument
2. `AGX_PROFILE` environment variable (whitespace-only treated as unset)
3. `agx current` selection

When `AGX_AUTO_BACKUP=1` is set in the environment, agx snapshots the
current target's context before exec-ing the native CLI. Snapshot
failures are written to stderr as `warning: ...` and do not block the
launch.

---

## Stable shapes (compat promise)

The following keys are **stable** and will not be removed or renamed
without a deprecation cycle:

- `managedProfileView`: `name`, `kind`, `current`, `base_url`, `api_key`,
  `credential_ref`, `model`, `codex_wire_api`, `provider_family`, `agents`
- `contextBackupView`: `id`, `target_kind`, `target_name`, `path`,
  `created_at`
- `DoctorReport`: `ok`, `operation`, `issues`
- `DoctorIssue`: `severity`, `code`, `message`, `action`
- `OperationRecord`: `id`, `command`, `agent`, `profile`, `stage`,
  `config_path`, `backup_path`, `backup_id`, `started_at`, `updated_at`

New keys may be **added** in any version (consumers must ignore unknown
keys). Removal or rename requires a major version bump.

The `doctor` issue code set is the closed catalog at
[doctor-issues.md](doctor-issues.md). A test enforces two-way sync
between that catalog and the runtime emitters. Consumers should match
on the `code` field, not on `message` text.
