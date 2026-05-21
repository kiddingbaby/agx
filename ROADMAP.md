# Roadmap

This is a rolling snapshot of where agx is headed. Items here are
**directional, not promises** — they may shift between releases.

中文：[ROADMAP.zh.md](ROADMAP.zh.md).

## Now (v0.1.0 baseline)

v0.1.0 is the initial single-user relay-aggregator baseline. The
default for the next cycle is **wait for issues** — concrete features
will be promoted as real asks come in. The Later section below stays
open.

## Later

- **Nix flake** — for the Nix-using audience
- **Scoop manifest** — Windows distribution via WSL2 + Scoop
- **Audit log** — opt-in append-only log of mutations for forensic use

## Out of scope

These are conscious "no" calls. Open an issue if you disagree, but the
default answer for now is no.

- **Native Windows support** — WSL2 is the supported path; native
  Windows means rewriting the flock and path layers for a single-digit
  share of agx's audience
- **Multi-user / team profile sharing** — agx is intentionally a
  single-user tool; team-level secret management belongs in a real
  secret store (Vault, AWS SM, doppler, etc.)
- **Agent orchestration** — agx launches one agent at a time; chaining
  / pipelines / parallel orchestration is out of scope
- **Provider-specific OAuth flows** — for OAuth, use the native CLI
  directly; agx is a relay (`base_url` + `api_key`) aggregator

## How to influence the roadmap

- Open a [Discussion](https://github.com/kiddingbaby/agx/discussions)
  for an idea
- Open an issue with a concrete use case for a bug or feature request
- Open a PR for anything in **Now** or **Later** — please link the
  related issue / discussion first so we can confirm fit
