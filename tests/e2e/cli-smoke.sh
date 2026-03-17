#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="${1:-$ROOT/.tmp/agx-smoke}"

export GOCACHE="${GOCACHE:-$ROOT/.tmp/go-build-cache}"
mkdir -p "$GOCACHE"

mkdir -p "$(dirname "$BIN")"
(cd "$ROOT" && go build -o "$BIN" ./cmd/agx)

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

export HOME="$TMP_HOME"
export AGX_SECRET="12345678901234567890123456789012"

"$BIN" --help >/dev/null
"$BIN" get sites | grep -q "openai"

"$BIN" create site claude-proxy --template claude-proxy --base-url https://claude-proxy.local --no-keys >/dev/null
"$BIN" create key smoke-claude --site claude-proxy --api-key sk-test --activate >/dev/null
"$BIN" get keys --site claude-proxy | grep -q "smoke-claude"
"$BIN" use claude-proxy >/dev/null
grep -q '"ANTHROPIC_API_KEY": "sk-test"' "$HOME/.claude/settings.json"
grep -q '"ANTHROPIC_BASE_URL": "https://claude-proxy.local"' "$HOME/.claude/settings.json"

"$BIN" create site openrouter --template openrouter --no-keys >/dev/null
"$BIN" create key smoke-openai --site openrouter --api-key sk-openai --activate >/dev/null
"$BIN" use openrouter >/dev/null
grep -q '"auth_mode": "apikey"' "$HOME/.codex/auth.json"
grep -q '"OPENAI_API_KEY": "sk-openai"' "$HOME/.codex/auth.json"
grep -q 'model_provider = "agx"' "$HOME/.codex/config.toml"
grep -q 'base_url = "https://openrouter.ai/api/v1"' "$HOME/.codex/config.toml"

"$BIN" create site gemini-proxy --template gemini-proxy --base-url https://gemini-proxy.local --no-keys >/dev/null
"$BIN" create key smoke-gemini --site gemini-proxy --api-key sk-gemini --activate >/dev/null
"$BIN" use gemini-proxy >/dev/null
grep -q '"selectedType": "gemini-api-key"' "$HOME/.gemini/settings.json"
grep -q 'GEMINI_API_KEY=sk-gemini' "$HOME/.gemini/.env"
grep -q 'GOOGLE_GEMINI_BASE_URL=https://gemini-proxy.local' "$HOME/.gemini/.env"

"$BIN" status >/dev/null
JSON_OUT="$("$BIN" status -o json)"
echo "$JSON_OUT" | jq -e '.bindings | length >= 3' >/dev/null
echo "[ok] e2e smoke passed"
