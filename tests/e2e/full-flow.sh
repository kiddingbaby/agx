#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
BIN="${1:-$ROOT/.tmp/agx-full}"

export GOCACHE="${GOCACHE:-$ROOT/.tmp/go-build-cache}"
mkdir -p "$GOCACHE"

mkdir -p "$(dirname "$BIN")"
(cd "$ROOT" && go build -o "$BIN" ./cmd/agx)

TMP_HOME="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME"' EXIT

export HOME="$TMP_HOME"
export AGX_SECRET="12345678901234567890123456789012"

fail() {
  echo "[FAIL] $*" >&2
  exit 1
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  [[ "$haystack" == *"$needle"* ]] || fail "$message (missing=$needle)"
}

assert_not_contains() {
  local haystack="$1"
  local needle="$2"
  local message="$3"
  [[ "$haystack" != *"$needle"* ]] || fail "$message (unexpected=$needle)"
}

assert_file_contains() {
  local path="$1"
  local needle="$2"
  local message="$3"
  [[ -f "$path" ]] || fail "$message (missing file=$path)"
  rg -n --fixed-strings "$needle" "$path" >/dev/null || fail "$message (missing=$needle file=$path)"
}

assert_file_not_contains() {
  local path="$1"
  local needle="$2"
  local message="$3"
  [[ -f "$path" ]] || return 0
  if rg -n --fixed-strings "$needle" "$path" >/dev/null; then
    fail "$message (unexpected=$needle file=$path)"
  fi
  return 0
}

expect_fail() {
  set +e
  "$@" >/dev/null 2>/dev/null
  local rc=$?
  set -e
  [[ $rc -ne 0 ]] || fail "expected failure: $*"
}

file_hash() {
  local path="$1"
  if [[ ! -f "$path" ]]; then
    echo "<missing>"
    return 0
  fi
  sha256sum "$path" | awk '{print $1}'
}

# Baseline: built-in official sites exist and are bound.
status="$("$BIN" status)"
assert_contains "$status" "openai -> openai" "baseline status should bind openai to official"
assert_contains "$status" "claude -> claude" "baseline status should bind claude to official"
assert_contains "$status" "gemini -> gemini" "baseline status should bind gemini to official"

init_text="$("$BIN" init --print)"
assert_contains "$init_text" "keys: []" "init template should include keys"
assert_contains "$init_text" "targets: []" "init template should include targets"

sites="$("$BIN" get sites)"
assert_contains "$sites" "openai  family=openai" "get sites should include openai official"
assert_contains "$sites" "claude  family=claude" "get sites should include claude official"
assert_contains "$sites" "gemini  family=gemini" "get sites should include gemini official"

# --- OpenAI official: key add -> switch -> config sync ---
"$BIN" create key oai-1 --site openai --api-key sk-oai-1 --activate >/dev/null
"$BIN" use openai >/dev/null
jq -e '.OPENAI_API_KEY=="sk-oai-1" and .auth_mode=="apikey"' "$HOME/.codex/auth.json" >/dev/null || fail "codex auth.json should be updated for openai official"
assert_file_not_contains "$HOME/.codex/config.toml" 'env_key = ' "codex provider should not require environment variable key"
assert_file_contains "$HOME/.codex/config.toml" 'wire_api = "responses"' "openai official should use responses API"
assert_file_contains "$HOME/.codex/config.toml" 'requires_openai_auth = true' "openai official should require openai auth by default"
assert_file_not_contains "$HOME/.codex/config.toml" 'base_url = "' "openai official should not set base_url"

# Official target override via `site edit openai`, then reset via `site rm openai`.
expect_fail "$BIN" patch site openai --wire-api chat_completions
"$BIN" patch site openai --model gpt-4.1 --no-requires-openai-auth >/dev/null
"$BIN" use openai >/dev/null
assert_file_contains "$HOME/.codex/config.toml" 'model = "gpt-4.1"' "openai official should honor model override"
assert_file_contains "$HOME/.codex/config.toml" 'wire_api = "responses"' "openai official should keep wire_api=responses"
assert_file_contains "$HOME/.codex/config.toml" 'requires_openai_auth = false' "openai official should honor requires-openai-auth override"
"$BIN" delete site openai >/dev/null
"$BIN" use openai >/dev/null
assert_file_not_contains "$HOME/.codex/config.toml" 'model = "gpt-4.1"' "reset should remove official model override"
assert_file_contains "$HOME/.codex/config.toml" 'wire_api = "responses"' "reset should restore default wire-api"
assert_file_contains "$HOME/.codex/config.toml" 'requires_openai_auth = true' "reset should restore default requires-openai-auth"

# --- OpenRouter: target add + 2 keys + pick + strategy + switch ---
"$BIN" create site openrouter --template openrouter --no-keys >/dev/null
"$BIN" create key or-1 --site openrouter --api-key sk-or-1 --activate >/dev/null
"$BIN" create key or-2 --site openrouter --api-key sk-or-2 >/dev/null
"$BIN" patch key or-2 --site openrouter --activate >/dev/null
"$BIN" use openrouter >/dev/null
jq -e '.OPENAI_API_KEY=="sk-or-2"' "$HOME/.codex/auth.json" >/dev/null || fail "openrouter pick should activate or-2"
assert_file_contains "$HOME/.codex/config.toml" 'base_url = "https://openrouter.ai/api/v1"' "openrouter should set base_url"
assert_file_contains "$HOME/.codex/config.toml" 'wire_api = "responses"' "openrouter should use responses API"
assert_file_contains "$HOME/.codex/config.toml" 'requires_openai_auth = false' "openrouter should default requires_openai_auth=false"

# Removing a bound site should fail.
expect_fail "$BIN" delete site openrouter

# Undo should restore OpenAI official binding + codex config.
"$BIN" undo >/dev/null
status="$("$BIN" status)"
assert_contains "$status" "openai -> openai" "undo should restore openai binding"
jq -e '.OPENAI_API_KEY=="sk-oai-1"' "$HOME/.codex/auth.json" >/dev/null || fail "undo should restore codex auth.json"
assert_file_contains "$HOME/.codex/config.toml" 'wire_api = "responses"' "undo should restore wire_api=responses"
assert_file_not_contains "$HOME/.codex/config.toml" 'base_url = "' "undo should restore no base_url for official"

# Editing openrouter advanced options should reflect in codex config.
"$BIN" patch site openrouter --wire-api responses --requires-openai-auth >/dev/null
"$BIN" use openrouter >/dev/null
assert_file_contains "$HOME/.codex/config.toml" 'wire_api = "responses"' "site edit should update wire_api"
assert_file_contains "$HOME/.codex/config.toml" 'requires_openai_auth = true' "site edit should update requires_openai_auth"

# Round-robin rotation across two keys.
cat <<'YAML' | "$BIN" apply --stdin >/dev/null
profiles:
  - provider: openai
    profile: openrouter
    strategy: round_robin
YAML
"$BIN" use openrouter >/dev/null
jq -e '.OPENAI_API_KEY=="sk-or-1"' "$HOME/.codex/auth.json" >/dev/null || fail "round_robin first switch should pick or-1"
"$BIN" use openrouter >/dev/null
jq -e '.OPENAI_API_KEY=="sk-or-2"' "$HOME/.codex/auth.json" >/dev/null || fail "round_robin second switch should pick or-2"

# Explicit key override by name + rename + remove.
"$BIN" use openrouter --key or-1 >/dev/null
jq -e '.OPENAI_API_KEY=="sk-or-1"' "$HOME/.codex/auth.json" >/dev/null || fail "--key override should force or-1"
"$BIN" patch key or-1 --site openrouter --name or-1-renamed >/dev/null
"$BIN" use openrouter --key or-1-renamed >/dev/null
jq -e '.OPENAI_API_KEY=="sk-or-1"' "$HOME/.codex/auth.json" >/dev/null || fail "renamed key should still work"
"$BIN" delete key or-2 --site openrouter >/dev/null
"$BIN" use openrouter >/dev/null
jq -e '.OPENAI_API_KEY=="sk-or-1"' "$HOME/.codex/auth.json" >/dev/null || fail "after rm, only remaining key should be used"

# --- Selector: use -l TAGS (AND match) + ambiguity behavior ---
"$BIN" create site tagtest --template openai-compatible --base-url https://tagtest.local --no-keys >/dev/null
"$BIN" create key tag-0 --site tagtest --api-key sk-tag-0 --activate >/dev/null
"$BIN" create key tag-1 --site tagtest --api-key sk-tag-1 --tags work,primary >/dev/null
"$BIN" create key tag-2 --site tagtest --api-key sk-tag-2 --tags work,secondary >/dev/null

expect_fail "$BIN" use tagtest -l work
expect_fail "$BIN" use tagtest -l missingtag
"$BIN" use tagtest -l work,primary >/dev/null
jq -e '.OPENAI_API_KEY=="sk-tag-1"' "$HOME/.codex/auth.json" >/dev/null || fail "-l work,primary should pick tag-1"
"$BIN" use tagtest -l work >/dev/null
jq -e '.OPENAI_API_KEY=="sk-tag-1"' "$HOME/.codex/auth.json" >/dev/null || fail "-l work should use current active when multiple match"

# --- Dry-run: no file writes + no round-robin advancement ---
"$BIN" create site rrtest --template openai-compatible --base-url https://rr.local --no-keys >/dev/null
"$BIN" create key rr-1 --site rrtest --api-key sk-rr-1 --activate >/dev/null
"$BIN" create key rr-2 --site rrtest --api-key sk-rr-2 >/dev/null
"$BIN" patch key rr-2 --site rrtest --activate >/dev/null
cat <<'YAML' | "$BIN" apply --stdin >/dev/null
profiles:
  - provider: openai
    profile: rrtest
    strategy: round_robin
YAML

keys_hash_before="$(file_hash "$HOME/.config/agx/keys.yaml")"
providers_hash_before="$(file_hash "$HOME/.config/agx/providers.yaml")"
codex_auth_hash_before="$(file_hash "$HOME/.codex/auth.json")"
codex_cfg_hash_before="$(file_hash "$HOME/.codex/config.toml")"

"$BIN" use rrtest --dry-run >/dev/null
"$BIN" use rrtest --dry-run >/dev/null

keys_hash_after="$(file_hash "$HOME/.config/agx/keys.yaml")"
providers_hash_after="$(file_hash "$HOME/.config/agx/providers.yaml")"
codex_auth_hash_after="$(file_hash "$HOME/.codex/auth.json")"
codex_cfg_hash_after="$(file_hash "$HOME/.codex/config.toml")"

[[ "$keys_hash_before" == "$keys_hash_after" ]] || fail "dry-run should not modify keys.yaml"
[[ "$providers_hash_before" == "$providers_hash_after" ]] || fail "dry-run should not modify providers.yaml"
[[ "$codex_auth_hash_before" == "$codex_auth_hash_after" ]] || fail "dry-run should not modify codex auth.json"
[[ "$codex_cfg_hash_before" == "$codex_cfg_hash_after" ]] || fail "dry-run should not modify codex config.toml"

keys_out="$("$BIN" get keys --site rrtest)"
assert_contains "$keys_out" "* rr-2" "dry-run should not mutate active key state"

"$BIN" use rrtest >/dev/null
jq -e '.OPENAI_API_KEY=="sk-rr-1"' "$HOME/.codex/auth.json" >/dev/null || fail "dry-run should not advance round_robin pointer (expect rr-1)"

# --- Claude: env set/unset/clear should not drift ---
mkdir -p "$HOME/.claude"
printf '%s\n' '{"env":{"KEEP":"yes","BAR":"legacy"}}' > "$HOME/.claude/settings.json"

"$BIN" create site claude-proxy --template claude-proxy --base-url https://claude-proxy.local --no-keys >/dev/null
"$BIN" create key cl-1 --site claude-proxy --api-key sk-cl-1 --activate >/dev/null
"$BIN" patch site claude-proxy --env FOO=bar --env BAR=baz --model sonnet >/dev/null
"$BIN" use claude-proxy >/dev/null
assert_file_contains "$HOME/.claude/settings.json" '"FOO": "bar"' "claude settings should include managed env"
assert_file_contains "$HOME/.claude/settings.json" '"BAR": "baz"' "claude settings should include managed env BAR=baz"
assert_file_contains "$HOME/.claude/settings.json" '"KEEP": "yes"' "claude settings should preserve user env"

"$BIN" patch site claude-proxy --env-unset BAR >/dev/null
"$BIN" use claude-proxy >/dev/null
assert_file_not_contains "$HOME/.claude/settings.json" '"BAR": "baz"' "env-unset should remove managed env keys"
assert_file_contains "$HOME/.claude/settings.json" '"KEEP": "yes"' "env-unset should not remove user env"

"$BIN" patch site claude-proxy --clear-env >/dev/null
"$BIN" use claude-proxy >/dev/null
assert_file_not_contains "$HOME/.claude/settings.json" '"FOO": "bar"' "clear-env should remove managed env keys"
assert_file_contains "$HOME/.claude/settings.json" '"KEEP": "yes"' "clear-env should preserve user env"

# Switching back to official should also clear managed env keys like ANTHROPIC_BASE_URL.
"$BIN" create key cl-official --site claude --api-key sk-cl-official --activate >/dev/null
"$BIN" use claude >/dev/null
assert_file_not_contains "$HOME/.claude/settings.json" "ANTHROPIC_BASE_URL" "official claude should not set base url"

# --- Import (Claude): bootstrap from native settings.json ---
mkdir -p "$HOME/.claude"
cat > "$HOME/.claude/settings.json" <<'JSON'
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-imported-1",
    "ANTHROPIC_BASE_URL": "https://import.local/v1"
  },
  "model": "claude-import-model"
}
JSON
"$BIN" import claude --site import-auto --tags imported >/dev/null
jq -e '.site.target=="import-auto" and .site.family=="claude" and .site.base_url=="https://import.local" and .site.model=="claude-import-model"' <("$BIN" describe site import-auto -o json) >/dev/null || fail "import should create claude proxy site and normalize base_url"
"$BIN" get keys --site import-auto | rg -n "tags=imported" >/dev/null || fail "imported key should be tagged"

"$BIN" create site import-existing --template claude-proxy --base-url https://import-existing.local --no-keys >/dev/null
"$BIN" create key import-keep --site import-existing --api-key sk-keep --activate >/dev/null
cat > "$HOME/.claude/settings.json" <<'JSON'
{
  "env": {
    "ANTHROPIC_API_KEY": "sk-imported-2"
  }
}
JSON
"$BIN" import claude --site import-existing --no-activate --tags imported2 >/dev/null
keys_out="$("$BIN" get keys --site import-existing)"
assert_contains "$keys_out" "* import-keep" "--no-activate should preserve active key"
echo "$keys_out" | rg -n "tags=imported2" >/dev/null || fail "imported key should be tagged (imported2)"

"$BIN" use claude >/dev/null
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-cl-official"' "post-import should restore official claude config for later tests"
assert_file_not_contains "$HOME/.claude/settings.json" "ANTHROPIC_BASE_URL" "post-import official claude should not set base url"

# --- Gemini: managed .env block + env update should be stable ---
mkdir -p "$HOME/.gemini"
printf '%s\n' '{"ui":{"theme":"ANSI"},"security":{"auth":{"selectedType":"none"}}}' > "$HOME/.gemini/settings.json"
printf '%s\n' 'USER_KEEP=1' > "$HOME/.gemini/.env"

"$BIN" create site gemini-proxy --template gemini-proxy --base-url https://gemini-proxy.local --no-keys >/dev/null
"$BIN" create key gm-1 --site gemini-proxy --api-key sk-gm-1 --activate >/dev/null
"$BIN" patch site gemini-proxy --env FOO=bar --model gemini-2.0-pro >/dev/null
"$BIN" use gemini-proxy >/dev/null
assert_file_contains "$HOME/.gemini/settings.json" '"selectedType": "gemini-api-key"' "gemini settings should select api key auth"
assert_file_contains "$HOME/.gemini/settings.json" '"theme": "ANSI"' "gemini settings should preserve user fields"
assert_file_contains "$HOME/.gemini/.env" "USER_KEEP=1" "gemini .env should preserve non-managed lines"
assert_file_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://gemini-proxy.local" "gemini .env should include base url"
assert_file_contains "$HOME/.gemini/.env" "FOO=bar" "gemini .env should include managed env"

"$BIN" patch site gemini-proxy --env-unset FOO >/dev/null
"$BIN" use gemini-proxy >/dev/null
assert_file_not_contains "$HOME/.gemini/.env" "FOO=bar" "gemini env-unset should remove managed env"
assert_file_contains "$HOME/.gemini/.env" "USER_KEEP=1" "gemini env-unset should preserve user .env lines"

# --- NewAPI template: multi-protocol host normalization ---
"$BIN" create site newapi --template newapi --base-url https://newapi.local/v1 --no-keys >/dev/null
jq -e '.site.target=="newapi-codex" and .site.family=="openai" and .site.base_url=="https://newapi.local/v1"' <("$BIN" describe site newapi-codex -o json) >/dev/null || fail "newapi codex site should keep /v1 base_url"
jq -e '.site.target=="newapi-claude" and .site.family=="claude" and .site.base_url=="https://newapi.local"' <("$BIN" describe site newapi-claude -o json) >/dev/null || fail "newapi claude site should drop /v1"
jq -e '.site.target=="newapi-gemini" and .site.family=="gemini" and .site.base_url=="https://newapi.local"' <("$BIN" describe site newapi-gemini -o json) >/dev/null || fail "newapi gemini site should drop /v1"

# Default: `agx use <site>-codex` should sync codex only (safe default).
"$BIN" create key na-1 --site newapi-codex --api-key sk-na-1 --activate >/dev/null
"$BIN" use newapi-codex >/dev/null
jq -e '.OPENAI_API_KEY=="sk-na-1"' "$HOME/.codex/auth.json" >/dev/null || fail "newapi use should update codex auth.json"
assert_file_contains "$HOME/.codex/config.toml" 'base_url = "https://newapi.local/v1"' "newapi use should set codex base_url"
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-cl-official"' "newapi use should not touch claude (safe default)"
assert_file_not_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-na-1"' "newapi use should not touch claude (safe default)"
assert_file_not_contains "$HOME/.claude/settings.json" '"ANTHROPIC_BASE_URL": "https://newapi.local"' "newapi use should not touch claude (safe default)"
assert_file_contains "$HOME/.gemini/.env" "GEMINI_API_KEY=sk-gm-1" "newapi use should not touch gemini (safe default)"
assert_file_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://gemini-proxy.local" "newapi use should not touch gemini (safe default)"
assert_file_not_contains "$HOME/.gemini/.env" "GEMINI_API_KEY=sk-na-1" "newapi use should not touch gemini (safe default)"
assert_file_not_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://newapi.local" "newapi use should not touch gemini (safe default)"

"$BIN" use newapi-codex --agents all >/dev/null
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-na-1"' "newapi use --agents all should update claude key"
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_BASE_URL": "https://newapi.local"' "newapi use --agents all should set claude base url"
assert_file_contains "$HOME/.gemini/.env" "GEMINI_API_KEY=sk-na-1" "newapi use --agents all should update gemini key"
assert_file_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://newapi.local" "newapi use --agents all should set gemini base url"

# --- NewAPI template: create with --agents subset (no gemini endpoint) ---
"$BIN" create site newapi-lite --template newapi --base-url https://newapi-lite.local --agents codex,claude --no-keys >/dev/null
jq -e '.site.target=="newapi-lite-codex" and .site.family=="openai" and .site.base_url=="https://newapi-lite.local/v1"' <("$BIN" describe site newapi-lite-codex -o json) >/dev/null || fail "newapi-lite codex site should keep /v1 base_url"
jq -e '.site.target=="newapi-lite-claude" and .site.family=="claude" and .site.base_url=="https://newapi-lite.local"' <("$BIN" describe site newapi-lite-claude -o json) >/dev/null || fail "newapi-lite claude site should drop /v1"
expect_fail "$BIN" describe site newapi-lite-gemini

"$BIN" create key na-lite-1 --site newapi-lite-codex --api-key sk-na-lite-1 --activate >/dev/null
"$BIN" use newapi-lite-codex >/dev/null
assert_file_contains "$HOME/.codex/config.toml" 'base_url = "https://newapi-lite.local/v1"' "newapi-lite use should set codex base_url"
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-na-1"' "newapi-lite use should not touch claude (safe default)"
assert_file_not_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-na-lite-1"' "newapi-lite use should not touch claude (safe default)"
assert_file_not_contains "$HOME/.claude/settings.json" '"ANTHROPIC_BASE_URL": "https://newapi-lite.local"' "newapi-lite use should not touch claude (safe default)"
assert_file_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://newapi.local" "newapi-lite use should not touch gemini (safe default)"
assert_file_not_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://newapi-lite.local" "newapi-lite use should not touch gemini (safe default)"

"$BIN" use newapi-lite-codex --agents codex,claude >/dev/null
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_API_KEY": "sk-na-lite-1"' "newapi-lite use --agents codex,claude should update claude key"
assert_file_contains "$HOME/.claude/settings.json" '"ANTHROPIC_BASE_URL": "https://newapi-lite.local"' "newapi-lite use --agents codex,claude should set claude base url"
assert_file_contains "$HOME/.gemini/.env" "GOOGLE_GEMINI_BASE_URL=https://newapi.local" "newapi-lite use --agents codex,claude should not touch gemini"
expect_fail "$BIN" use newapi-lite-codex --agents all

# --- apply bundle: key-env read once (no drift) ---
export OAI_KEY_ENV_TEST="sk-from-env"
bundle="$HOME/bundle.yml"
cat > "$bundle" <<'YAML'
keys:
  - provider: openai
    profile: default
    name: oai-env
    key-env: OAI_KEY_ENV_TEST
    activate: true
YAML
cat "$bundle" | "$BIN" apply --stdin >/dev/null
unset OAI_KEY_ENV_TEST
"$BIN" use openai >/dev/null
jq -e '.OPENAI_API_KEY=="sk-from-env"' "$HOME/.codex/auth.json" >/dev/null || fail "apply key-env should be read once and stored"

# --- cleanup paths: unbind then remove third-party sites ---
"$BIN" use openai >/dev/null
"$BIN" use claude >/dev/null
"$BIN" create key gm-official --site gemini --api-key sk-gm-official --activate >/dev/null
"$BIN" use gemini >/dev/null
"$BIN" delete site openrouter >/dev/null
"$BIN" delete site newapi-codex >/dev/null
"$BIN" delete site newapi-claude >/dev/null
"$BIN" delete site newapi-gemini >/dev/null
"$BIN" delete site newapi-lite-codex >/dev/null
"$BIN" delete site newapi-lite-claude >/dev/null
"$BIN" delete site claude-proxy >/dev/null
"$BIN" delete site gemini-proxy >/dev/null
"$BIN" delete site tagtest >/dev/null
"$BIN" delete site rrtest >/dev/null
"$BIN" delete site import-auto >/dev/null
"$BIN" delete site import-existing >/dev/null

echo "[ok] full flow passed"
