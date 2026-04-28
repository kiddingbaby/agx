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

if ! command -v cue >/dev/null 2>&1; then
  echo "Error: cue not found in PATH" >&2
  exit 1
fi

TMP_HOME="$(mktemp -d)"
TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_HOME" "$TMP_DIR"' EXIT

export HOME="$TMP_HOME"
mkdir -p "$HOME/.codex"
printf 'profile = "before"\n' >"$HOME/.codex/config.toml"

"$BIN" add relay-a --base-url https://relay-a.example/v1 --api-key sk-a >/dev/null
"$BIN" add relay-b --base-url https://relay-b.example/v1 --api-key sk-b -o json >"$TMP_DIR/add.json"
"$BIN" ls -o json >"$TMP_DIR/ls.json"
"$BIN" show relay-a -o json >"$TMP_DIR/show.json"
"$BIN" edit relay-a --bind codex -o json >"$TMP_DIR/set.json"
"$BIN" backup ls --agent codex -o json >"$TMP_DIR/backup.json"
"$BIN" restore --agent codex -o json >"$TMP_DIR/restore.json"
"$BIN" rm relay-a -o json >"$TMP_DIR/rm.json"
"$BIN" doctor -o json >"$TMP_DIR/doctor.json"

cue vet -d "#AddView" "$ROOT/cue/contracts.cue" "$TMP_DIR/add.json"
cue vet -d "#ListView" "$ROOT/cue/contracts.cue" "$TMP_DIR/ls.json"
cue vet -d "#ShowView" "$ROOT/cue/contracts.cue" "$TMP_DIR/show.json"
cue vet -d "#SetView" "$ROOT/cue/contracts.cue" "$TMP_DIR/set.json"
cue vet -d "#BackupListView" "$ROOT/cue/contracts.cue" "$TMP_DIR/backup.json"
cue vet -d "#RestoreView" "$ROOT/cue/contracts.cue" "$TMP_DIR/restore.json"
cue vet -d "#RemoveView" "$ROOT/cue/contracts.cue" "$TMP_DIR/rm.json"
cue vet -d "#DoctorView" "$ROOT/cue/contracts.cue" "$TMP_DIR/doctor.json"
cue vet -d "#StateFile" "$ROOT/cue/contracts.cue" "$HOME/.config/agx/state.yaml"
