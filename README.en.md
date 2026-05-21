# agx

[![CI](https://github.com/kiddingbaby/agx/actions/workflows/ci.yml/badge.svg)](https://github.com/kiddingbaby/agx/actions/workflows/ci.yml)
[![Go Reference](https://pkg.go.dev/badge/github.com/kiddingbaby/agx.svg)](https://pkg.go.dev/github.com/kiddingbaby/agx)
[![Go Report Card](https://goreportcard.com/badge/github.com/kiddingbaby/agx)](https://goreportcard.com/report/github.com/kiddingbaby/agx)
[![Go version](https://img.shields.io/github/go-mod/go-version/kiddingbaby/agx)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/platform-linux%20%7C%20macOS-lightgrey.svg)](#install)

<p align="center">
  <img src="docs/assets/demo.svg" alt="agx demo: add a relay profile, switch, run codex / claude" width="780">
</p>

> One relay profile (`base_url` + `api_key`) drives `codex`, `claude`,
> `gemini`, and `opencode`. Each agent runs in its own isolated context.

中文：[README.md](README.md) · User guide: [docs/user-guide.en.md](docs/user-guide.en.md)

---

## Why agx

> "Relay" here means any OpenAI-compatible (`base_url` + `api_key`) endpoint — a self-hosted gateway, a third-party proxy, LiteLLM, etc.

Switching relays across multiple AI coding agents normally means:

- Maintaining four separate config files
  (`~/.codex/config.toml`, `~/.claude/settings.json`, `~/.gemini/.env`,
  `~/.config/opencode/opencode.json`) by hand
- Fearing every relay switch in case one of those files gets clobbered
  with no rollback point
- Re-editing the same `base_url` / `api_key` / `model` fields across
  accounts, gateways, and providers

agx collapses this into four commands:

```bash
agx add  <name>   # register a relay profile
agx use  <name>   # switch the current profile
agx run  <agent>  # launch the native CLI inside an isolated context
agx doctor        # actionable recovery advice when something is off
```

Every write is snapshotted first; any failure rolls back the entire
change. All four agents speak the same profile model.

## Quick start

```bash
brew install kiddingbaby/agx/agx

agx add work \
  --base-url https://relay.example/v1 \
  --api-key  sk-live

agx use work
agx run codex      # every agent picks up this relay
agx run claude
```

Per-agent config materializes under `~/.config/agx/contexts/<agent>/<name>/`
and is injected when the native CLI is launched. Your host-level
`~/.codex` / `~/.claude` / `~/.gemini` / `~/.config/opencode` are left
untouched. Profiles are stored as plaintext YAML (mode 0600) under
`~/.config/agx/profiles/`; no OS keychain integration yet.

## Install

### Homebrew (macOS / Linuxbrew)

```bash
brew install kiddingbaby/agx/agx
```

### Prebuilt binary

Download the matching archive from the
[latest release](https://github.com/kiddingbaby/agx/releases/latest):

```bash
VERSION=v0.1.0
OS=$(uname -s | tr '[:upper:]' '[:lower:]')   # linux / darwin
ARCH=$(uname -m | sed 's/aarch64/arm64/;s/x86_64/x86_64/')
curl -L "https://github.com/kiddingbaby/agx/releases/download/${VERSION}/agx_${VERSION#v}_${OS}_${ARCH}.tar.gz" \
  | tar -xz -C /tmp
install -m 0755 /tmp/agx ~/.local/bin/agx
```

### `go install`

```bash
go install github.com/kiddingbaby/agx/cmd/agx@latest
```

Linux / macOS on amd64 / arm64. Building from source is covered in
[CONTRIBUTING.md](CONTRIBUTING.md).

Uninstall: `brew uninstall agx` (or delete the binary); to wipe all
profiles / contexts: `rm -rf ~/.config/agx`.

## Common tasks

See what's currently set:

```bash
agx ls         # list every profile; * marks the current one
agx current    # print just the current profile name
```

Switch between relays:

```bash
agx add openai-direct   --base-url https://api.openai.com/v1            --api-key sk-...
agx add anthropic-relay --base-url https://relay.example/anthropic/v1   --api-key sk-...

agx use openai-direct   && agx run codex
agx use anthropic-relay && agx run claude
```

Use a relay just once without changing the default:

```bash
agx run codex openai-direct -- --help     # use openai-direct just this launch; args after -- are forwarded to codex
AGX_PROFILE=openai-direct agx run codex   # this shell; pairs with direnv for per-directory pinning
```

Snapshot before a risky edit, roll back if needed:

```bash
agx backup codex                          # snapshot codex's current profile
agx edit work --api-key sk-rotated
agx run codex
agx restore codex                         # back to the latest snapshot
```

Full command reference: [user guide](docs/user-guide.en.md).

## When something is off

```bash
agx doctor
```

`doctor` lists every detected issue with a severity and an `issue code`,
plus an **actionable** fix suggestion — usually a single
`agx restore <agent>` does it. Every issue code is documented in
[doctor-issues.md](docs/doctor-issues.md).

To snapshot automatically before each `agx run`:

```bash
export AGX_AUTO_BACKUP=1
```

## How it works

```
┌──────────────┐    agx add / edit       ┌─────────────────────┐
│ profile YAML │ ◀────────────────────── │   agx CLI           │
│  store       │                          │                     │
└──────┬───────┘                          └──────────┬──────────┘
       │ resolve                                     │
       ▼                                             │
┌──────────────┐    agx run <agent>                  │
│  derived     │ ◀───────── exec native CLI ◀────────┘
│  per-agent   │            (codex / claude / gemini / opencode)
│  config      │
└──────────────┘
```

- **Isolated contexts.** Per-agent config lives in `contexts/<agent>/<name>/`; host dotfiles stay untouched
- **Rollback built-in.** See the backup / restore example above
- **No daemon.** A plain CLI; a file lock is only held while a command is running

Architecture details: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Scope

agx only handles **OpenAI-compatible** (`base_url` + `api_key`) relay
endpoints. If what you actually need is:

- OAuth login, native SDK, or an agent's built-in provider — launch the
  native CLI directly; agx is not in the loop
- Multi-user / team-level secret management — out of scope; agx is a
  single-user tool

## Status

Pre-1.0. CLI commands, exit codes, JSON output, and doctor issue codes
are stable promises; the on-disk layout is not yet frozen — integrate
through the CLI rather than reading `~/.config/agx/` directly. See
[compatibility policy](COMPATIBILITY.md) · [CLI contract](docs/cli-contract.md).

Latest release: [release notes](docs/release-notes/) · Direction:
[ROADMAP.md](ROADMAP.md)

## Community

- Ideas & discussion: [GitHub Discussions](https://github.com/kiddingbaby/agx/discussions)
- Bugs & feature requests: [GitHub Issues](https://github.com/kiddingbaby/agx/issues)
- Pull requests: [CONTRIBUTING.md](CONTRIBUTING.md) · [Code of Conduct](CODE_OF_CONDUCT.md)

## License

[MIT](LICENSE) © 2026 kiddingbaby
