#!/usr/bin/env bash

setup_agx_suite() {
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  if [[ -n "${AGX_CACHE_DIR:-}" ]]; then
    CACHE_DIR="$AGX_CACHE_DIR"
  elif [[ -n "${XDG_CACHE_HOME:-}" ]]; then
    CACHE_DIR="$XDG_CACHE_HOME/agx"
  else
    CACHE_DIR="$HOME/.cache/agx"
  fi
  BIN="${CACHE_DIR}/bin/agx-smoke-bats"
  GOCACHE_DIR="${GOCACHE:-/tmp/agx-go-build}"

  export ROOT
  export BIN
  export GOCACHE="$GOCACHE_DIR"

  mkdir -p "$GOCACHE_DIR"
  mkdir -p "$(dirname "$BIN")"
  (
    cd "$ROOT"
    go build -o "$BIN" ./cmd/agx
  )
}

teardown_agx_suite() {
  :
}

setup_agx_home() {
  TMP_HOME="$(mktemp -d)"
  export TMP_HOME
  export HOME="$TMP_HOME"
}

teardown_agx_home() {
  rm -rf "$TMP_HOME"
}

assert_contains() {
  local haystack="$1"
  local needle="$2"
  if [[ "$haystack" != *"$needle"* ]]; then
    echo "expected output to contain: $needle" >&2
    echo "actual output: $haystack" >&2
    return 1
  fi
}
