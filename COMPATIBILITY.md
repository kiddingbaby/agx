# Compatibility Policy

This document defines what is and isn't part of the public contract for
the `agx` project. It complements [docs/cli-contract.md](docs/cli-contract.md)
(which specifies the actual shapes) and informs version bump decisions
under [semver](https://semver.org/).

中文：[COMPATIBILITY.zh.md](COMPATIBILITY.zh.md).

## What is part of the contract

The following are **stable**. Breaking changes require a major version bump
(or a documented deprecation cycle for additive replacements).

- CLI subcommands, their argument shapes, and their flag names
- Exit codes (see [docs/cli-contract.md](docs/cli-contract.md))
- JSON output shapes for documented commands, including key names and
  enum string values
- `agx doctor` issue `code` values that are explicitly documented
- Environment variables prefixed `AGX_*`
- Standard install paths (`~/.local/bin/agx` default, `BINDIR` override)
- The agent set supported by `agx run <agent>`: `codex`, `claude`,
  `gemini`, `opencode`

## What is **not** part of the contract

Treat these as implementation details. They may change in any release —
including patch releases — without notice.

- The on-disk layout of `~/.config/agx/` (state, profiles, contexts,
  backups, journal, lockfile)
- Field names and YAML key shapes inside `state.yaml`, profile YAML,
  journal records
- The format and contents of agent-specific files agx writes
  (`~/.codex/config.toml`'s "AGX managed" block, `settings.json` shape,
  `.env` keys, etc.) — these track upstream agent CLIs and may be
  reshaped to follow them
- Help text wording (the structure / sections are stable, the prose is not)
- The exact text of error messages — match on the `doctor` issue `code`
  instead
- Internal Go packages under `internal/` — embedding agx as a library is
  not supported

## Versioning

agx uses [Semantic Versioning](https://semver.org/) starting at v0.x:

- `v0.x.y` — major can break **anything except the contract** between
  minor versions. Once we reach 1.0 the contract section above tightens.
- Patch releases are bug-fix only and do not break the contract.
- Tagged pre-releases (`-rc.N`, `-alpha.N`) are released for risky
  changes; they may break compatibility with the previous pre-release.

## Deprecation

Any contract change that is not strictly additive follows this flow:

1. Document the new shape in the release notes and mark the old shape
   `deprecated` in the help text + this file.
2. Keep the old shape working for at least one minor release.
3. Remove only at a major version bump (or a minor bump for clearly
   pre-1.0 churn, with explicit release-notes mention).

If you depend on agx in a script, pin to a tag (`@v0.x.y`) and read
release notes before upgrading.
