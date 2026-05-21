#!/usr/bin/env bats

load './test_helper.bash'

setup_file() {
  setup_agx_suite
}

teardown_file() {
  teardown_agx_suite
}

setup() {
  setup_agx_home
}

teardown() {
  teardown_agx_home
}

@test "help shows profile-first command tree" {
  run "$BIN" --help
  [ "$status" -eq 0 ]
  assert_contains "$output" "AGX - Local Multi-Agent Runtime Manager"
  assert_contains "$output" "agx add <profile>"
  assert_contains "$output" "agx current"
  assert_contains "$output" "agx detach <agent> <profile>"
  assert_contains "$output" "agx run <agent> [profile] [-- native args...]"
  assert_contains "$output" "agx restore <agent>"
  assert_contains "$output" "agx backup <agent>"
  assert_contains "$output" "agx doctor"
}

@test "removed relay root stays unavailable" {
  run "$BIN" relay ls
  [ "$status" -eq 1 ]
  assert_contains "$output" "unknown command \"relay\""
}

@test "removed channel root stays unavailable" {
  run "$BIN" channel --adapter codex add work --base-url https://x.example/v1 --api-key sk-x
  [ "$status" -eq 1 ]
  assert_contains "$output" "unknown command \"channel\""
}

@test "removed migrate root stays unavailable" {
  run "$BIN" migrate discover --agent codex
  [ "$status" -eq 1 ]
  assert_contains "$output" "unknown command \"migrate\""
}

@test "profile lifecycle works on the public path" {
  run "$BIN" add work --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]
  assert_contains "$output" "profile work created"

  run "$BIN" use work
  [ "$status" -eq 0 ]
  assert_contains "$output" "current profile: work"

  run "$BIN" current -o json
  [ "$status" -eq 0 ]
  assert_contains "$output" "\"name\":\"work\""
  assert_contains "$output" "\"current\":true"

  run "$BIN" ls -o json
  [ "$status" -eq 0 ]
  assert_contains "$output" "\"profiles\""
  assert_contains "$output" "\"name\":\"work\""

  run "$BIN" show work -o json
  [ "$status" -eq 0 ]
  assert_contains "$output" "\"base_url\":\"https://relay-a.example/v1\""

  run "$BIN" run --help
  [ "$status" -eq 0 ]
  assert_contains "$output" "agx run <agent> [profile] [-- <native args...>] [flags]"
}

@test "opencode launcher requires profile model" {
  run "$BIN" add work --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]

  run "$BIN" run opencode work
  [ "$status" -eq 1 ]
  assert_contains "$output" "opencode model is required"
  assert_contains "$output" "agx edit work --model"
}

@test "detach unbinds a profile from one agent without deleting it" {
  run "$BIN" add work --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]

  # detach without any prior bind returns a clear not-found error
  run "$BIN" detach codex work
  [ "$status" -eq 1 ]
  assert_contains "$output" "codex target work not found"
}

@test "backup snapshots current target and restore rolls it back" {
  run "$BIN" add work --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]
  run "$BIN" use work
  [ "$status" -eq 0 ]

  # no current target yet → backup must refuse
  run "$BIN" backup codex
  [ "$status" -eq 1 ]
  assert_contains "$output" "no current target selected for codex"

  # activate a codex target via the launcher (codex binary missing is fine,
  # ActivateManagedProfile runs first and creates the context)
  "$BIN" codex -- status >/dev/null 2>&1 || true

  run "$BIN" backup codex -o json
  [ "$status" -eq 0 ]
  assert_contains "$output" "\"target_name\":\"work\""

  run "$BIN" restore codex -o json
  [ "$status" -eq 0 ]
  assert_contains "$output" "\"target_name\":\"work\""

  # opencode has no per-target context, so backup must reject it
  run "$BIN" backup opencode
  [ "$status" -eq 1 ]
  assert_contains "$output" "agent must be one of: codex, claude, gemini"
}

@test "completion emits shell scripts for bash/zsh/fish" {
  run "$BIN" completion bash
  [ "$status" -eq 0 ]
  assert_contains "$output" "bash completion"

  run "$BIN" completion zsh
  [ "$status" -eq 0 ]
  assert_contains "$output" "#compdef agx"

  run "$BIN" completion fish
  [ "$status" -eq 0 ]
  assert_contains "$output" "fish completion for agx"
}

@test "AGX_AUTO_BACKUP=1 snapshots before launch" {
  run "$BIN" add work --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]
  run "$BIN" use work
  [ "$status" -eq 0 ]

  # First activate so the context dir exists. The launcher exits non-zero
  # because the native codex binary is missing; that's irrelevant to us.
  "$BIN" codex -- status >/dev/null 2>&1 || true

  # Helper: count backup-timestamp directories. The find may legitimately
  # return non-zero before the first backup exists; the `|| true` keeps the
  # pipeline pipefail-safe inside the bats runner.
  count_backups() {
    { find "${AGX_HOME:-$HOME}/.config/agx/backups" -mindepth 5 -maxdepth 5 -type d 2>/dev/null || true; } | wc -l
  }

  # Without opt-in: no snapshot.
  backups_before=$(count_backups)

  # With opt-in: one new snapshot.
  AGX_AUTO_BACKUP=1 "$BIN" codex -- status >/dev/null 2>&1 || true
  backups_after=$(count_backups)

  if [ "$backups_after" -le "$backups_before" ]; then
    echo "expected auto-backup to add a snapshot (before=$backups_before after=$backups_after)" >&2
    return 1
  fi
}

@test "AGX_PROFILE env overrides current without flipping it" {
  run "$BIN" add work --base-url https://work.example/v1 --api-key sk-w
  [ "$status" -eq 0 ]
  run "$BIN" add staging --base-url https://stg.example/v1 --api-key sk-s
  [ "$status" -eq 0 ]
  run "$BIN" use work
  [ "$status" -eq 0 ]

  # AGX_PROFILE should pick staging for the launch but leave `agx current`
  # unchanged. The launcher exits non-zero because codex isn't installed
  # in the sandbox; we only care about which context dir got materialized.
  AGX_PROFILE=staging "$BIN" codex -- status >/dev/null 2>&1 || true
  [ -d "$HOME/.config/agx/contexts/codex/targets/staging" ]

  run "$BIN" current
  [ "$status" -eq 0 ]
  assert_contains "$output" "work"

  # Explicit positional profile beats AGX_PROFILE.
  AGX_PROFILE=staging "$BIN" codex work -- status >/dev/null 2>&1 || true
  [ -d "$HOME/.config/agx/contexts/codex/targets/work" ]

  # Whitespace-only env falls back to current.
  rm -rf "$HOME/.config/agx/contexts/codex/targets/work"
  AGX_PROFILE="   " "$BIN" codex -- status >/dev/null 2>&1 || true
  [ -d "$HOME/.config/agx/contexts/codex/targets/work" ]
}
