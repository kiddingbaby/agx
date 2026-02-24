#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="${1:-$ROOT/.tmp/agx-smoke}"

if ! command -v tmux >/dev/null 2>&1; then
  echo "[skip] tmux not found"
  exit 0
fi

if [[ ! -x "$BIN" ]]; then
  mkdir -p "$(dirname "$BIN")"
  (cd "$ROOT" && go build -o "$BIN" ./cmd/agx)
fi

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

export HOME="$TMP_HOME"
export AGX_SECRET="12345678901234567890123456789012"

"$BIN" --help >/dev/null
"$BIN" keys ls | grep -q "No keys configured"
"$BIN" keys add --provider claude --name smoke-claude --key sk-test >/dev/null
"$BIN" keys activate smoke-claude >/dev/null
"$BIN" keys ls --provider claude | grep -q "smoke-claude"
"$BIN" ls >/dev/null

set +e
OUT="$($BIN codex-cli 2>&1)"
RC=$?
set -e
if [[ $RC -eq 0 ]]; then
  echo "[fail] expected codex-cli launch to fail without active openai key" >&2
  exit 1
fi

echo "$OUT" | grep -q "No active key for openai"
echo "[ok] e2e smoke passed"
