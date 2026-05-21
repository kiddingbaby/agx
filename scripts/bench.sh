#!/usr/bin/env bash
# Tiny CLI-invocation benchmark. Prints per-command average wall time over
# N runs. Targets the high-frequency commands users hit interactively;
# slow regressions show up here long before they show up in unit tests.
#
# Usage:
#   scripts/bench.sh                 # uses AGX_BIN or ~/.cache/agx/bin/agx
#   AGX_BIN=/path/to/agx scripts/bench.sh
#   AGX_BENCH_RUNS=50 scripts/bench.sh
#
# Exit code 0 when all measured averages are under AGX_BENCH_BUDGET_MS
# (default 50ms). Non-zero when any command crosses the budget.

set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BIN="${AGX_BIN:-$HOME/.cache/agx/bin/agx}"
RUNS="${AGX_BENCH_RUNS:-20}"
BUDGET_MS="${AGX_BENCH_BUDGET_MS:-50}"

if [[ ! -x "$BIN" ]]; then
  echo "agx binary not found at $BIN; run 'task build' or set AGX_BIN" >&2
  exit 1
fi

if ! command -v python3 >/dev/null 2>&1; then
  echo "python3 required for sub-second timing" >&2
  exit 1
fi

# Sandbox a HOME with a single profile so 'ls', 'show', 'doctor' have
# something to talk about.
SANDBOX="$(mktemp -d -t agx-bench-XXXXXX)"
trap 'rm -rf "$SANDBOX"' EXIT
HOME="$SANDBOX" "$BIN" add work --base-url https://relay.example/v1 --api-key sk-bench >/dev/null
HOME="$SANDBOX" "$BIN" use work >/dev/null

commands=(
  "version"
  "ls"
  "current"
  "show work"
  "doctor"
)

over_budget=0
printf "%-14s %10s %10s\n" "command" "avg(ms)" "p95(ms)"
printf "%-14s %10s %10s\n" "--------------" "----------" "----------"

for cmd in "${commands[@]}"; do
  read -r avg p95 < <(python3 - "$BIN" "$SANDBOX" "$RUNS" "$cmd" <<'PY'
import os, subprocess, sys, time
bin_path, sandbox, runs, cmd = sys.argv[1:5]
runs = int(runs)
env = {"HOME": sandbox, "PATH": os.environ.get("PATH", "")}
samples = []
args = [bin_path, *cmd.split()]
for _ in range(runs):
    t = time.perf_counter()
    subprocess.run(args, env=env, stdout=subprocess.DEVNULL, stderr=subprocess.DEVNULL)
    samples.append((time.perf_counter() - t) * 1000.0)
samples.sort()
avg = sum(samples) / len(samples)
p95_idx = max(0, int(len(samples) * 0.95) - 1)
print(f"{avg:.1f} {samples[p95_idx]:.1f}")
PY
)
  printf "%-14s %10s %10s\n" "$cmd" "$avg" "$p95"
  avg_int=$(printf '%.0f' "$avg")
  if (( avg_int > BUDGET_MS )); then
    over_budget=$((over_budget + 1))
    echo "  → over budget ${BUDGET_MS}ms" >&2
  fi
done

echo ""
echo "runs=$RUNS  budget=${BUDGET_MS}ms  binary=$BIN"
exit $over_budget
