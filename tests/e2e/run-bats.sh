#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
SUITE="$ROOT/tests/e2e/cli-smoke.bats"
LOCAL_RUNNER="$ROOT/tests/e2e/bats-local.sh"

if ! command -v bats >/dev/null 2>&1; then
  echo "[info] bats not found in PATH; using repo-local compatibility runner" >&2
  exec bash "$LOCAL_RUNNER" "$SUITE" "$@"
fi

exec bats "$SUITE" "$@"
