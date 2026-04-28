#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
if [[ -n "${AGX_CACHE_DIR:-}" ]]; then
  CACHE_DIR="$AGX_CACHE_DIR"
elif [[ -n "${XDG_CACHE_HOME:-}" ]]; then
  CACHE_DIR="$XDG_CACHE_HOME/agx"
else
  CACHE_DIR="$HOME/.cache/agx"
fi
BIN="${1:-$CACHE_DIR/bin/agx-smoke}"

export GOCACHE="${GOCACHE:-/tmp/agx-go-build}"
mkdir -p "$GOCACHE"
mkdir -p "$(dirname "$BIN")"

(cd "$ROOT" && go build -o "$BIN" ./cmd/agx)

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT
RM_BOUND_OUT="$TMP_HOME/rm-bound.out"
RM_BOUND_ERR="$TMP_HOME/rm-bound.err"
INVALID_OUT="$TMP_HOME/invalid-args.out"
INVALID_ERR="$TMP_HOME/invalid-args.err"

export HOME="$TMP_HOME"

HELP_OUT="$("$BIN" --help)"
grep -q "agx add <relay>" <<<"$HELP_OUT"
grep -q "agx edit <relay>" <<<"$HELP_OUT"
grep -q "agx backup ls --agent" <<<"$HELP_OUT"
grep -q "agx doctor" <<<"$HELP_OUT"

DOCTOR_OUT="$("$BIN" doctor)"
grep -q "Doctor: ok" <<<"$DOCTOR_OUT"

mkdir -p "$HOME/.codex" "$HOME/.claude"
printf 'profile = "before"\n' >"$HOME/.codex/config.toml"
cat >"$HOME/.claude/settings.json" <<'JSON'
{
  "env": {
    "KEEP_ME": "1"
  }
}
JSON

"$BIN" add relay-a --base-url https://relay-a.example/v1 --api-key sk-a >/dev/null
SHOW_OUT="$("$BIN" show relay-a)"
grep -q "agents=-" <<<"$SHOW_OUT"

LIST_OUT="$("$BIN" ls)"
grep -q "agents=-" <<<"$LIST_OUT"

DOCTOR_OUT="$("$BIN" doctor)"
grep -q "Doctor: ok" <<<"$DOCTOR_OUT"
grep -q '^profile = "before"$' "$HOME/.codex/config.toml"

BIND_OUT="$("$BIN" edit relay-a --bind codex,claude)"
grep -q "Updated relay bindings: relay-a" <<<"$BIND_OUT"
grep -q "bind claude" <<<"$BIND_OUT"
grep -q "bind codex" <<<"$BIND_OUT"

SHOW_OUT="$("$BIN" show relay-a)"
grep -q "agents=codex,claude" <<<"$SHOW_OUT"

LIST_OUT="$("$BIN" ls)"
grep -q "agents=codex,claude" <<<"$LIST_OUT"

AGENT_LIST_OUT="$("$BIN" ls --agent codex)"
grep -q "Current: relay-a" <<<"$AGENT_LIST_OUT"
grep -q '\* relay-a' <<<"$AGENT_LIST_OUT"

BACKUP_OUT="$("$BIN" backup ls --agent codex)"
grep -q "backup_id=before-codex-sync-" <<<"$BACKUP_OUT"
grep -q "restore_mode=restore_file" <<<"$BACKUP_OUT"

"$BIN" edit relay-a --base-url https://relay-a-new.example/v1 --api-key sk-rotated >/dev/null
LIST_OUT="$("$BIN" ls)"
grep -q "agents=codex,claude" <<<"$LIST_OUT"
grep -q 'base_url = "https://relay-a-new.example/v1"' "$HOME/.codex/config.toml"
grep -q '"ANTHROPIC_BASE_URL": "https://relay-a-new.example/v1"' "$HOME/.claude/settings.json"

set +e
"$BIN" rm relay-a >"$RM_BOUND_OUT" 2>"$RM_BOUND_ERR"
rc=$?
set -e
[ "$rc" -ne 0 ]
grep -q "relay relay-a is currently bound to codex, claude" "$RM_BOUND_ERR"

UNBIND_OUT="$("$BIN" edit relay-a --unbind codex)"
grep -q "unbind codex" <<<"$UNBIND_OUT"

SHOW_OUT="$("$BIN" show relay-a)"
grep -q "agents=claude" <<<"$SHOW_OUT"

RESTORE_OUT="$("$BIN" restore --agent codex)"
grep -q "Restored agent: codex" <<<"$RESTORE_OUT"

BACKUP_OUT="$("$BIN" backup ls --agent codex)"
grep -q "Backups for codex:" <<<"$BACKUP_OUT"

python3 - "$BIN" "$HOME" <<'PY'
import os
import pty
import select
import subprocess
import sys
import time

bin_path = sys.argv[1]
home = sys.argv[2]
master, slave = pty.openpty()
proc = subprocess.Popen(
    [bin_path, "add"],
    stdin=slave,
    stdout=slave,
    stderr=slave,
    env={**os.environ, "HOME": home},
)
os.close(slave)

time.sleep(0.2)
os.write(master, b"relay-paste\nhttps://relay-paste.example/v1\nsk-paste\n")

output = b""
deadline = time.time() + 5
while time.time() < deadline:
    if proc.poll() is not None:
        break
    ready, _, _ = select.select([master], [], [], 0.2)
    if not ready:
        continue
    try:
        chunk = os.read(master, 4096)
    except OSError:
        break
    if not chunk:
        break
    output += chunk
    if b"Added relay: relay-paste" in output:
        break

rc = proc.wait(timeout=5)
text = output.decode("utf-8", errors="replace")
if rc != 0:
    sys.stderr.write(text)
    raise SystemExit(rc)
if "Added relay: relay-paste" not in text:
    sys.stderr.write(text)
    raise SystemExit("interactive add over PTY did not complete as expected")
PY

JSON_OUT="$("$BIN" show relay-a -o json)"
jq -e '.agent_bindings | length >= 1' >/dev/null <<<"$JSON_OUT"

set +e
"$BIN" add relay-old --base-url https://relay-old.example/v1 --api-key sk-old --agent codex >"$INVALID_OUT" 2>"$INVALID_ERR"
rc=$?
set -e
[ "$rc" -ne 0 ]
grep -q "Usage: agx add <relay> --base-url URL --api-key KEY" "$INVALID_ERR"

set +e
"$BIN" restore codex >"$INVALID_OUT" 2>"$INVALID_ERR"
rc=$?
set -e
[ "$rc" -ne 0 ]
grep -q "Usage: agx restore --agent codex\|claude\|gemini \[--to BACKUP_ID\] \[-o json\]" "$INVALID_ERR"

echo "[ok] relay smoke passed"
