# User Guide

中文：[user-guide.md](user-guide.md).

## Overview

agx organizes four core actions around a relay profile:

- Manage global profiles with `add` / `edit` / `rm`
- Pick a default profile with `use` / `current`
- Launch native CLIs in isolated contexts with `run <agent>`
- Keep state recoverable with `doctor` / `backup` / `restore`

## Commands

### Profile

```bash
agx add <name> --base-url <url> --api-key <key> [--model <id>]
agx edit <name> [--name <new>] [--base-url ...] [--api-key ...] [--model ...]
agx rm <name>
agx ls
agx show <name>
agx use <name>
agx current
```

### Launcher

```bash
agx run codex
agx run claude
agx run gemini
agx run opencode
agx run codex work -- --help
agx detach codex work
```

Compat aliases: `agx codex` / `agx claude` / `agx gemini` / `agx opencode` still resolve to `agx run <agent>`.

### Diagnostics

```bash
agx doctor
agx backup <agent>
agx restore <agent>
```

## Profile rules

- Name is non-empty and limited to letters, digits, `-`, `_`, `.`
- `base_url` and `api_key` are required; `model` is optional in general but required when launching `opencode`
- `edit --name` migrates every binding and managed target referencing the profile
- Other `edit` flags re-sync every managed target that references the profile
  (best-effort; failures are listed in the output)
- `use` only switches agx's current profile; it does not rewrite every agent's runtime

## Launcher rules

- `agx run <agent> [profile] [-- <native args...>]` launches the native CLI inside a controlled context
- Profile resolution precedence: positional arg > `AGX_PROFILE` env var > `agx current`
- An explicit `profile` only affects this launch; promote it to the default with `agx use`
- `AGX_PROFILE` pairs well with `direnv` for per-directory relay pinning and never flips the global `current`
- `AGX_AUTO_BACKUP=1` snapshots the current target before launch
- The `AGENTS` column in `agx ls` shows which agents the profile is bound to;
  `agx detach <agent> <profile>` removes one such binding

## Diagnostics & recovery

- `agx doctor` prints `ok` when healthy, otherwise lists issues plus suggested actions
- `agx backup <agent>` snapshots the current target's context (only
  `codex` / `claude` / `gemini`; `opencode` has no per-target context)
- `agx restore <agent>` restores the latest snapshot for the current target

## Integration boundary

agx is a relay aggregator and only handles OpenAI-compatible (`base_url` + `api_key`) endpoints:

- Third-party gateway: `agx add` it
- Official API keys: add them as relays with `--base-url` set to the official endpoint
- OAuth / native SDK / built-in provider: launch the native CLI directly, bypassing agx

## File layout

| Path | Purpose |
| --- | --- |
| `~/.config/agx/state.yaml` | global state |
| `~/.config/agx/profiles/` | profile store |
| `~/.config/agx/contexts/` | managed contexts |
| `~/.config/agx/backups/managed/` | managed snapshots |
