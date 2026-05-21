#!/usr/bin/env bash

setup_agx_suite() {
  ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
  if [[ -n "${AGX_BIN:-}" ]]; then
    BIN="$AGX_BIN"
  elif [[ -n "${AGX_CACHE_DIR:-}" ]]; then
    BIN="${AGX_CACHE_DIR}/bin/agx"
  elif [[ -n "${XDG_CACHE_HOME:-}" ]]; then
    BIN="${XDG_CACHE_HOME}/agx/bin/agx"
  else
    BIN="${HOME}/.cache/agx/bin/agx"
  fi

  if [[ ! -x "$BIN" ]]; then
    echo "error: $BIN not built; run 'task build' first" >&2
    return 1
  fi

  export ROOT
  export BIN
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
