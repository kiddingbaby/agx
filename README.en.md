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
> `gemini`, and `opencode`. Each agent runs in an isolated managed context.

中文：[README.md](README.md) · Docs: [User Guide](docs/user-guide.en.md) · [Architecture](docs/ARCHITECTURE.md)

---

## Why agx

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

Every mutation snapshots profile / state / agent config before writing,
so `backup` / `restore` / `doctor` can roll any change back. All four
agents speak the same profile model.

## Quick start

```bash
# 1. Install (pick one path; see Install below for the others)
brew install kiddingbaby/agx/agx

# 2. Register a relay
agx add work \
  --base-url https://relay.example/v1 \
  --api-key  sk-live

# 3. Launch any agent — they all use this relay automatically
agx use work
agx run codex
agx run claude
```

agx materializes per-agent config under
`~/.config/agx/contexts/<agent>/<name>/` and injects it when the native
CLI is launched. Your host-level `~/.codex` / `~/.claude` / `~/.gemini`
configs are left untouched.

## Install

### Homebrew (macOS / Linuxbrew)

```bash
brew install kiddingbaby/agx/agx
```

### Prebuilt binary

Download the matching archive from the
[latest release](https://github.com/kiddingbaby/agx/releases/latest)
(substitute the version you want):

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

### Build from source

```bash
git clone https://github.com/kiddingbaby/agx.git
cd agx
task dev:setup
task build
task install
```

- Binary installs to `~/.local/bin/agx` by default
  (override with `BINDIR=/usr/local/bin task install`)
- Go 1.24+; `mise.toml` declares the required toolchain
- Linux / macOS on amd64 / arm64

## Examples

Switch between relays:

```bash
agx add openai-direct   --base-url https://api.openai.com/v1            --api-key sk-...
agx add anthropic-relay --base-url https://relay.example/anthropic/v1   --api-key sk-...

agx use openai-direct  && agx run codex
agx use anthropic-relay && agx run claude
```

Use a relay just once without changing the default:

```bash
agx run codex openai-direct -- --help   # positional: this launch only
AGX_PROFILE=openai-direct agx run codex # env: this shell / direnv-friendly
```

Snapshot before a risky edit, roll back if needed:

```bash
agx backup codex     # explicit snapshot of the current target
agx edit work --api-key sk-rotated
agx run codex
# If the new key is wrong:
agx restore codex    # roll back to the latest snapshot
```

Diagnose and self-heal:

```bash
agx doctor
# Issues like unfinished_operation / orphan_managed_target each include
# a suggested action — usually `agx restore <agent>`.
```

## How it works

```
┌──────────────┐    agx add / edit       ┌─────────────────────┐
│ profile YAML │ ◀────────────────────── │   agx CLI (this)    │
│  store       │                          │  • mutation guard   │
└──────┬───────┘                          │  • per-target ctx   │
       │ resolve                          │  • doctor / restore │
       ▼                                  └──────────┬──────────┘
┌──────────────┐                                     │
│  derived     │   agx run <agent>                   │
│  per-agent   │ ◀───────── exec native CLI ◀────────┘
│  config      │            (codex / claude / gemini / opencode)
└──────────────┘
```

- **Profile is the first-class concept.** Every command revolves around
  "which relay am I on right now"
- **Mutation guard.** Each write captures a pre-image of profile /
  state / agent config; any failure rolls every step back — panics
  included (recover + re-panic preserves the non-zero exit)
- **Per-target context.** Each agent runs inside its own
  `contexts/<agent>/<name>/` tree; host-level config is never touched
- **No daemon.** A plain CLI; an `flock` is only held while a command
  is executing

Full layout and extension points: [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

## Scope

agx only handles **OpenAI-compatible** (`base_url` + `api_key`) relay
endpoints. If what you actually need is:

- OAuth login, native SDK, or an agent's built-in provider — launch
  the native CLI directly; agx is not in the loop
- Multi-user / team-level secret management — out of scope; agx is a
  single-user tool

## Status

Pre-1.0:

- The main path (add / use / run / backup / restore / doctor) goes
  through the full verification chain
  (unit + interactive + e2e + Docker matrix)
- Public contract: [CLI contract](docs/cli-contract.md) ·
  [Doctor issue catalog](docs/doctor-issues.md) ·
  [compatibility policy](COMPATIBILITY.md)
- Internal state fields and on-disk layout may shift before 1.0 —
  please do not depend on them

Latest release notes: [docs/release-notes/](docs/release-notes/) ·
Direction: [ROADMAP.md](ROADMAP.md)

## Contributing

Issues and PRs are welcome. Development flow, verification matrix, and
PR checklist live in [CONTRIBUTING.md](CONTRIBUTING.md). Everyone is
expected to follow the [Code of Conduct](CODE_OF_CONDUCT.md).

Repo-level AI agent collaboration constraints: [AGENTS.md](AGENTS.md).

## License

[MIT](LICENSE) © 2026 kiddingbaby
