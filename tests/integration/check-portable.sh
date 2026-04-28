#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
cd "$ROOT"

if [[ -n "${AGX_CACHE_DIR:-}" ]]; then
  CACHE_DIR="$AGX_CACHE_DIR"
elif [[ -n "${XDG_CACHE_HOME:-}" ]]; then
  CACHE_DIR="$XDG_CACHE_HOME/agx"
else
  CACHE_DIR="$HOME/.cache/agx"
fi

BIN_DIR="$CACHE_DIR/bin"
BIN="$BIN_DIR/agx"

export GOCACHE="${GOCACHE:-/tmp/agx-go-build}"
mkdir -p "$GOCACHE" "$BIN_DIR"

go build -o "$BIN" ./cmd/agx
go test ./...
go test -tags=contract ./tests/script/... ./tests/contract/json/...
go test -tags=interactive ./tests/interactive/...
bash tests/integration/cue-check.sh "$BIN"
bash tests/e2e/cli-smoke.sh
bash tests/e2e/run-bats.sh
