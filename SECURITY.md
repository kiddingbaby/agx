# Security Policy

中文：[SECURITY.zh.md](SECURITY.zh.md).

## Reporting a Vulnerability

If you believe you have found a security issue in agx, **please do not
open a public GitHub issue**. Instead, report privately:

- Email: **kiddingbaby@163.com** with subject line `agx security: <short summary>`
- Or use GitHub's [Private Vulnerability Reporting](https://github.com/kiddingbaby/agx/security/advisories/new)

We aim to acknowledge reports within **72 hours** and to ship a fix or
mitigation for confirmed high-severity issues within **14 days**.

When reporting, please include:

- agx version (`agx version` output)
- OS / arch
- Minimal reproduction steps
- The actual impact you observed (what an attacker could do)

## Scope

agx is a **single-user local CLI** that:

- Reads / writes config files under `~/.config/agx/` and into per-agent
  config locations (`~/.codex`, `~/.claude`, etc.) when invoked
- Stores user-provided API keys in plain YAML under `~/.config/agx/profiles/`
- Spawns native agent CLIs (`codex`, `claude`, ...) with environment
  variables that include API keys

Issues we treat as in-scope security bugs:

- Privilege escalation (agx running as user X gaining ability to read /
  write outside its own directory tree)
- Disclosure of user API keys to other local users or to logs / world-
  readable paths
- Path traversal / arbitrary file write via profile / target names or
  flag values
- Code execution via crafted config / state / journal files
- Mutation-guard / rollback being bypassed in a way that leaves agent
  config in a known-bad state

Things we generally do **not** treat as security issues:

- An attacker with write access to `~/.config/agx/` can break agx. This
  is by design — agx trusts its own state dir
- An attacker on the same host with the same UID can read your keys.
  Standard Unix permissions apply; agx uses `0600` / `0700` modes
- Loss of data caused by deleting the AGX directory or running
  destructive commands explicitly

## Disclosure

We follow coordinated disclosure: once a fix is available, we publish a
GitHub Security Advisory with a CVSS score and credit the reporter
unless they prefer to remain anonymous.

## Supported versions

Only the **latest minor release** receives security fixes. Pre-1.0
releases are not LTS — please track `main` or pin to the latest tag.
