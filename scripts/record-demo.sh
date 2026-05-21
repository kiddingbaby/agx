#!/usr/bin/env bash
# Re-record the README demo. Produces docs/assets/demo.cast and is the
# canonical source of truth for any future SVG / GIF conversion.
#
# Requires: asciinema (apt: asciinema). For SVG/GIF conversion install
# asterius/agg (cargo install agg) and run:
#   agg docs/assets/demo.cast docs/assets/demo.gif
# or use svg-term / asciinema-player for live web embeds.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX="${SANDBOX_HOME:-$(mktemp -d -t agx-demo-XXXXXX)}"
CAST="${CAST_OUT:-$ROOT/docs/assets/demo.cast}"

if ! command -v asciinema >/dev/null 2>&1; then
  echo "asciinema not installed; install it first (apt-get install asciinema)" >&2
  exit 1
fi

if ! command -v agx >/dev/null 2>&1; then
  echo "agx not on PATH; run 'task install' first" >&2
  exit 1
fi

mkdir -p "$SANDBOX"
trap 'rm -rf "$SANDBOX"' EXIT

cat >"$SANDBOX/demo.sh" <<'SCRIPT'
set -e
ps='\033[36m~ $\033[0m '
type() { printf '%b%s\n' "$ps" "$1"; sleep 0.2; }

type 'agx add work --base-url https://relay.example/v1 --api-key sk-live'
agx add work --base-url https://relay.example/v1 --api-key sk-live
sleep 0.4

type 'agx use work'
agx use work
sleep 0.4

type 'agx run codex'
echo '› codex launches in isolated context — relay = work'
sleep 0.6

type 'agx add staging --base-url https://relay.example/staging/v1 --api-key sk-stg'
agx add staging --base-url https://relay.example/staging/v1 --api-key sk-stg
sleep 0.4

type 'agx use staging && agx run claude'
agx use staging
echo '› claude launches in isolated context — relay = staging'
SCRIPT

HOME="$SANDBOX" asciinema rec \
  --overwrite \
  --idle-time-limit 1.0 \
  --command "bash $SANDBOX/demo.sh" \
  "$CAST"

echo "wrote $CAST"
echo "next: agg \"$CAST\" docs/assets/demo.gif"
