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

@test "help shows relay-centric commands" {
  run "$BIN" --help
  [ "$status" -eq 0 ]
  assert_contains "$output" "agx add <relay>"
  assert_contains "$output" "agx edit <relay>"
  assert_contains "$output" "backup ls --agent codex|claude|gemini"
  assert_contains "$output" "agx doctor"
}

@test "same relay can be bound to codex and claude" {
  mkdir -p "$HOME/.codex" "$HOME/.claude"
  printf 'profile = "before"\n' >"$HOME/.codex/config.toml"
  cat >"$HOME/.claude/settings.json" <<'JSON'
{
  "env": {
    "KEEP_ME": "1"
  }
}
JSON

  run "$BIN" add relay-a --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]

  run "$BIN" edit relay-a --bind codex
  [ "$status" -eq 0 ]
  assert_contains "$output" "Updated relay bindings: relay-a"
  assert_contains "$output" "bind codex"

  run "$BIN" edit relay-a --bind claude
  [ "$status" -eq 0 ]
  assert_contains "$output" "Updated relay bindings: relay-a"
  assert_contains "$output" "bind claude"

  run "$BIN" ls
  [ "$status" -eq 0 ]
  assert_contains "$output" "agents=codex,claude"
}

@test "edit auto-syncs active agents and backup list works" {
  mkdir -p "$HOME/.codex" "$HOME/.claude"
  printf 'profile = "before"\n' >"$HOME/.codex/config.toml"
  cat >"$HOME/.claude/settings.json" <<'JSON'
{
  "env": {
    "KEEP_ME": "1"
  }
}
JSON

  run "$BIN" add relay-a --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]

  run "$BIN" edit relay-a --bind codex,claude
  [ "$status" -eq 0 ]

  run "$BIN" edit relay-a --base-url https://relay-a-new.example/v1
  [ "$status" -eq 0 ]
  assert_contains "$output" "Edited relay: relay-a"

  run "$BIN" ls
  [ "$status" -eq 0 ]
  assert_contains "$output" "agents=codex,claude"

  run "$BIN" backup ls --agent codex
  [ "$status" -eq 0 ]
  assert_contains "$output" "Backups for codex:"
  assert_contains "$output" "backup_id=before-codex-sync-"
}

@test "invalid CLI args fail and json list exposes relays" {
  run "$BIN" add relay-a --base-url https://relay-a.example/v1 --api-key sk-a --agent codex
  [ "$status" -eq 1 ]
  assert_contains "$output" "Usage: agx add <relay> --base-url URL --api-key KEY"

  run "$BIN" add relay-a --base-url https://relay-a.example/v1 --api-key sk-a
  [ "$status" -eq 0 ]

  run "$BIN" ls -o json
  [ "$status" -eq 0 ]
  assert_contains "$output" "\"relays\""
  assert_contains "$output" "\"agents\""

  run "$BIN" backup ls --agent wrong
  [ "$status" -eq 1 ]
  assert_contains "$output" "Agent must be one of: codex, claude, gemini."
}
