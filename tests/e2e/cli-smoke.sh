#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

if [[ -n "${1:-}" ]]; then
  BIN="$1"
elif [[ -n "${AGX_CACHE_DIR:-}" ]]; then
  BIN="${AGX_CACHE_DIR}/bin/agx"
elif [[ -n "${XDG_CACHE_HOME:-}" ]]; then
  BIN="${XDG_CACHE_HOME}/agx/bin/agx"
else
  BIN="${HOME}/.cache/agx/bin/agx"
fi

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT
export HOME="$TMP_HOME"

HELP_OUT="$("$BIN" --help)"
grep -q '^AGX - Local Multi-Agent Runtime Manager$' <<<"$HELP_OUT"
grep -q '^  agx add <profile> --base-url URL --api-key KEY \[--model MODEL\]$' <<<"$HELP_OUT"
grep -q '^  agx current$' <<<"$HELP_OUT"
grep -q '^  agx run <agent> \[profile\] \[-- native args...\]$' <<<"$HELP_OUT"
grep -q '^  agx restore <agent>$' <<<"$HELP_OUT"
grep -q '^  agx doctor$' <<<"$HELP_OUT"

"$BIN" add work --base-url https://relay-a.example/v1 --api-key sk-a >/dev/null
PROFILE_USE_OUT="$("$BIN" use work)"
grep -q '^current profile: work$' <<<"$PROFILE_USE_OUT"

PROFILE_LIST_JSON="$("$BIN" ls -o json)"
grep -q '"profiles"' <<<"$PROFILE_LIST_JSON"
grep -q '"name":"work"' <<<"$PROFILE_LIST_JSON"

PROFILE_SHOW_JSON="$("$BIN" show work -o json)"
grep -q '"base_url":"https://relay-a.example/v1"' <<<"$PROFILE_SHOW_JSON"

CURRENT_JSON="$("$BIN" current -o json)"
grep -q '"name":"work"' <<<"$CURRENT_JSON"
grep -q '"current":true' <<<"$CURRENT_JSON"

RUN_HELP="$("$BIN" run --help)"
grep -Fq 'agx run <agent> [profile] [-- <native args...>] [flags]' <<<"$RUN_HELP"

"$BIN" relay ls >/tmp/agx-legacy-relay.out 2>/tmp/agx-legacy-relay.err || true
grep -q 'unknown command "relay"' /tmp/agx-legacy-relay.err

"$BIN" channel --adapter codex add anything --base-url https://relay-x.example/v1 --api-key sk-x >/tmp/agx-legacy-channel.out 2>/tmp/agx-legacy-channel.err || true
grep -q 'unknown command "channel"' /tmp/agx-legacy-channel.err

"$BIN" migrate discover --agent codex >/tmp/agx-legacy-migrate.out 2>/tmp/agx-legacy-migrate.err || true
grep -q 'unknown command "migrate"' /tmp/agx-legacy-migrate.err

# detach smoke: bind tmp via launcher activation then detach codex side
"$BIN" add tmp-detach --base-url https://relay-c.example/v1 --api-key sk-c >/dev/null
"$BIN" use tmp-detach >/dev/null
DETACH_OUT="$("$BIN" detach codex tmp-detach 2>&1 || true)"
echo "$DETACH_OUT" | grep -qE '(detached codex from profile tmp-detach|codex target tmp-detach not found)'

echo "[ok] profile-first cli smoke passed"
