#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)"
OUT="${1:-}"

if [[ $# -ne 1 || -z "$OUT" ]]; then
  echo "usage: $0 OUTPUT" >&2
  exit 1
fi

if [[ "$OUT" != /* ]]; then
  OUT="$PWD/$OUT"
fi

OUT_DIR="$(dirname "$OUT")"
mkdir -p "$OUT_DIR"

if [[ ! -w "$OUT_DIR" ]]; then
  echo "error: output directory is not writable: $OUT_DIR" >&2
  exit 1
fi

HAS_GIT_INFO=1
if VERSION="$(cd "$ROOT" && git describe --tags --always --dirty 2>/dev/null)"; then
  :
else
  VERSION="dev"
  HAS_GIT_INFO=0
fi

if COMMIT="$(cd "$ROOT" && git rev-parse --short HEAD 2>/dev/null)"; then
  :
else
  COMMIT="unknown"
  HAS_GIT_INFO=0
fi

if [[ "$HAS_GIT_INFO" -eq 1 ]]; then
  DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo unknown)"
else
  DATE="unknown"
fi
LDFLAGS="-s -w -X main.version=$VERSION -X main.commit=$COMMIT -X main.date=$DATE"

cd "$ROOT"
go build -buildvcs=false -ldflags="$LDFLAGS" -o "$OUT" ./cmd/agx
