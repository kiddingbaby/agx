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
BIN="${1:-$CACHE_DIR/bin/agx-real-smoke}"

export GOCACHE="${GOCACHE:-/tmp/agx-go-build}"
mkdir -p "$GOCACHE"
mkdir -p "$(dirname "$BIN")"

(cd "$ROOT" && bash scripts/build-agx.sh "$BIN")

TMP_DIRS=()
cleanup() {
  for dir in "${TMP_DIRS[@]:-}"; do
    rm -rf "$dir"
  done
}
trap cleanup EXIT

mkhome() {
  local dir
  dir="$(mktemp -d "${TMPDIR:-/tmp}/agx-real-cli-home.XXXXXX")"
  TMP_DIRS+=("$dir")
  echo "$dir"
}

have_any=0

run_codex_smoke() {
  if ! command -v codex >/dev/null 2>&1; then
    echo "[skip] codex not found"
    return 0
  fi
  have_any=1

  local home out
  home="$(mkhome)"
  out="$home/codex.out"

  HOME="$home" "$BIN" add linuxdo --base-url https://relay.example/v1 --api-key sk-fake >/dev/null
  HOME="$home" "$BIN" edit linuxdo --bind codex >/dev/null

  grep -q '^\[model_providers\]$' "$home/.codex/config.toml"
  grep -q '^\[profiles\]$' "$home/.codex/config.toml"
  grep -q '^name = "linuxdo"$' "$home/.codex/config.toml"
  grep -q '^\[profiles\."linuxdo"\]$' "$home/.codex/config.toml"

  HOME="$home" codex -p linuxdo debug prompt-input >"$out" 2>&1
  grep -q '"type": "message"' "$out"

  echo "[ok] codex real smoke passed"
}

run_claude_smoke() {
  if ! command -v claude >/dev/null 2>&1; then
    echo "[skip] claude not found"
    return 0
  fi
  have_any=1

  local home debug_log stderr_log rc
  home="$(mkhome)"
  debug_log="$home/claude-debug.log"
  stderr_log="$home/claude.stderr"

  HOME="$home" "$BIN" add relay-a --base-url https://127.0.0.1:9/v1 --api-key sk-fake >/dev/null
  HOME="$home" "$BIN" edit relay-a --bind claude >/dev/null

  grep -q '"apiKeyHelper"' "$home/.claude/settings.json"
  grep -q '"ANTHROPIC_BASE_URL": "https://127.0.0.1:9/v1"' "$home/.claude/settings.json"

  set +e
  HOME="$home" timeout 8s claude --bare --settings "$home/.claude/settings.json" --debug-file "$debug_log" -p 'say ok' --output-format json --permission-mode plan > /dev/null 2>"$stderr_log"
  rc=$?
  set -e

  if [[ "$rc" -ne 0 && "$rc" -ne 124 ]]; then
    echo "[fail] claude exited with rc=$rc" >&2
    sed -n '1,160p' "$stderr_log" >&2 || true
    return 1
  fi

  grep -q 'settingsEnv keys: ANTHROPIC_BASE_URL,CLAUDE_CODE_API_KEY_HELPER_TTL_MS' "$debug_log"
  grep -q 'ANTHROPIC_BASE_URL=https://127.0.0.1:9/v1' "$debug_log"
  grep -q '\[API REQUEST\]' "$debug_log"

  echo "[ok] claude real smoke passed"
}

run_gemini_smoke() {
  if ! command -v gemini >/dev/null 2>&1; then
    echo "[skip] gemini not found"
    return 0
  fi
  have_any=1

  local home out rc
  home="$(mkhome)"
  out="$home/gemini.out"

  HOME="$home" "$BIN" add relay-a --base-url https://127.0.0.1:9/v1 --api-key sk-fake >/dev/null
  HOME="$home" "$BIN" edit relay-a --bind gemini >/dev/null

  grep -q '^GEMINI_API_KEY="sk-fake"$' "$home/.gemini/.env"
  grep -q '^GOOGLE_GEMINI_BASE_URL="https://127.0.0.1:9/v1"$' "$home/.gemini/.env"

  set +e
  HOME="$home" timeout 8s gemini -p 'say ok' --output-format json >"$out" 2>&1
  rc=$?
  set -e

  if [[ "$rc" -eq 0 ]]; then
    echo "[fail] gemini unexpectedly succeeded against fake relay" >&2
    sed -n '1,160p' "$out" >&2 || true
    return 1
  fi

  # Newer Gemini CLI versions may time out before surfacing a transport error
  # string. Treat any non-successful run as acceptable here as long as the CLI
  # does not complain that the AGX-managed API key was missing.
  if grep -q 'must specify the GEMINI_API_KEY environment variable' "$out"; then
    echo "[fail] gemini did not load AGX-managed .env" >&2
    sed -n '1,160p' "$out" >&2 || true
    return 1
  fi

  echo "[ok] gemini real smoke passed"
}

run_codex_smoke
run_claude_smoke
run_gemini_smoke

if [[ "$have_any" -eq 0 ]]; then
  echo "[skip] no supported agent CLI found"
  exit 0
fi

echo "[ok] real cli smoke passed"
