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
BIN="${1:-$CACHE_DIR/bin/agx}"

if [[ ! -x "$BIN" ]]; then
  echo "error: $BIN not found or not executable; run 'task build' first" >&2
  exit 1
fi

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
  HOME="$home" "$BIN" use linuxdo >/dev/null

  HOME="$home" "$BIN" codex -- debug prompt-input >"$out" 2>&1

  grep -q '^\[model_providers\]$' "$home/.config/agx/contexts/codex/targets/linuxdo/config.toml"
  grep -q '^\[profiles\]$' "$home/.config/agx/contexts/codex/targets/linuxdo/config.toml"
  grep -q '^name = "codex.linuxdo"$' "$home/.config/agx/contexts/codex/targets/linuxdo/config.toml"
  grep -q '^\[profiles\."agx/codex.linuxdo"\]$' "$home/.config/agx/contexts/codex/targets/linuxdo/config.toml"

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
  HOME="$home" "$BIN" use relay-a >/dev/null

  set +e
  HOME="$home" timeout 8s "$BIN" claude -- --bare --debug-file "$debug_log" -p 'say ok' --output-format json --permission-mode plan > /dev/null 2>"$stderr_log"
  rc=$?
  set -e

  if [[ "$rc" -ne 0 && "$rc" -ne 124 ]]; then
    echo "[fail] claude exited with rc=$rc" >&2
    sed -n '1,160p' "$stderr_log" >&2 || true
    return 1
  fi

  grep -q '"apiKeyHelper"' "$home/.config/agx/contexts/claude/targets/relay-a/settings.json"
  grep -q '"ANTHROPIC_BASE_URL": "https://127.0.0.1:9"' "$home/.config/agx/contexts/claude/targets/relay-a/settings.json"
  grep -qE 'settingsEnv keys:.*ANTHROPIC_BASE_URL' "$debug_log"
  grep -q '\[API REQUEST\]' "$debug_log"
  # Newer claude-code releases stopped echoing the full `ANTHROPIC_BASE_URL=<value>`
  # line in debug output, so we cannot grep that literal anymore. Instead, ensure
  # claude did NOT silently fall back to the default anthropic.com endpoint —
  # if our injection had failed, claude would either resolve to api.anthropic.com
  # or refuse to start. The combination of (settingsEnv loaded ANTHROPIC_BASE_URL)
  # + (API REQUEST emitted) + (no anthropic.com reference) pins it to our relay.
  if grep -q 'api\.anthropic\.com' "$debug_log"; then
    echo "[fail] claude fell back to api.anthropic.com instead of using agx base_url" >&2
    return 1
  fi

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
  HOME="$home" "$BIN" use relay-a >/dev/null

  set +e
  HOME="$home" timeout 8s "$BIN" gemini -- -p 'say ok' --output-format json >"$out" 2>&1
  rc=$?
  set -e

  if [[ "$rc" -eq 0 ]]; then
    echo "[fail] gemini unexpectedly succeeded against fake relay" >&2
    sed -n '1,160p' "$out" >&2 || true
    return 1
  fi

  # AGX injects GEMINI_API_KEY / GOOGLE_GEMINI_BASE_URL straight into the
  # gemini process env; they are never persisted to disk. The only on-disk
  # artifact AGX writes is settings.json (api-key auth + sandbox=false).
  if [[ ! -f "$home/.config/agx/contexts/gemini/targets/relay-a/.gemini/settings.json" ]]; then
    echo "[fail] gemini settings.json was not materialized" >&2
    return 1
  fi
  # Newer Gemini CLI versions may time out before surfacing a transport error
  # string. Treat any non-successful run as acceptable here as long as the CLI
  # does not complain that the AGX-injected API key was missing.
  if grep -q 'must specify the GEMINI_API_KEY environment variable' "$out"; then
    echo "[fail] gemini did not see AGX-injected GEMINI_API_KEY" >&2
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
