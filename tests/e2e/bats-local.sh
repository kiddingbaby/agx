#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: bats-local.sh <suite.bats> [more suites...]" >&2
  exit 1
fi

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

pass_count=0
fail_count=0

run_suite() {
  local suite="$1"
  local suite_abs
  suite_abs="$(cd "$(dirname "$suite")" && pwd)/$(basename "$suite")"
  local suite_dir
  suite_dir="$(dirname "$suite_abs")"
  local transformed="$tmp_dir/$(basename "$suite").transformed.sh"

  awk -v suite_dir="$suite_dir" '
    function flush_test() {
      if (!in_test) {
        return
      }
      print "__AGX_BATS_END__"
      in_test = 0
      in_heredoc = 0
      heredoc = ""
    }
    /^load[[:space:]]+/ {
      helper = $0
      sub(/^load[[:space:]]+["'"'"']/, "", helper)
      sub(/["'"'"'][[:space:]]*$/, "", helper)
      print "source \"" suite_dir "/" helper "\""
      next
    }
    /^@test[[:space:]]+"/ {
      flush_test()
      name = $0
      sub(/^@test[[:space:]]+"/, "", name)
      sub(/"[[:space:]]+\{[[:space:]]*$/, "", name)
      print "__agx_register_test \"" name "\" <<'\''__AGX_BATS_END__'\''"
      in_test = 1
      next
    }
    in_test && in_heredoc && $0 == heredoc {
      in_heredoc = 0
      heredoc = ""
      print
      next
    }
    in_test && !in_heredoc && match($0, /<<-?[[:space:]]*["'"'"']?[A-Za-z_][A-Za-z0-9_]*["'"'"']?/) {
      heredoc = substr($0, RSTART, RLENGTH)
      sub(/^<<-?[[:space:]]*["'"'"']?/, "", heredoc)
      sub(/["'"'"']?$/, "", heredoc)
      in_heredoc = 1
      print
      next
    }
    in_test && !in_heredoc && $0 == "}" {
      flush_test()
      next
    }
    { print }
    END {
      flush_test()
    }
  ' "$suite_abs" >"$transformed"

  # shellcheck disable=SC1090
  source <(
    cat <<'EOF'
declare -a __agx_test_names=()
declare -a __agx_test_files=()
__agx_test_counter=0

__agx_register_test() {
  local name="$1"
  __agx_test_counter=$((__agx_test_counter + 1))
  local body_file="${AGX_BATS_TMP_DIR}/test-${__agx_test_counter}.body.sh"
  cat >"$body_file"
  __agx_test_names+=("$name")
  __agx_test_files+=("$body_file")
}
EOF
    cat "$transformed"
  )

  if declare -F setup_file >/dev/null; then
    setup_file
  fi

  local i
  for i in "${!__agx_test_names[@]}"; do
    local name="${__agx_test_names[$i]}"
    local body="${__agx_test_files[$i]}"
    local runner="$tmp_dir/test-runner-$i.sh"

    if declare -F setup >/dev/null; then
      setup
    fi

    cat >"$runner" <<EOF
#!/usr/bin/env bash
set -euo pipefail
source "$suite_dir/test_helper.bash"
run() {
  local __out __status
  set +e
  __out="\$("\$@" 2>&1)"
  __status=\$?
  set -e
  output="\$__out"
  status=\$__status
}
source "$body"
EOF
    chmod +x "$runner"

    if bash "$runner"; then
      printf 'ok - %s\n' "$name"
      pass_count=$((pass_count + 1))
    else
      printf 'not ok - %s\n' "$name" >&2
      fail_count=$((fail_count + 1))
    fi

    if declare -F teardown >/dev/null; then
      teardown
    fi
  done

  if declare -F teardown_file >/dev/null; then
    teardown_file
  fi

  unset -f setup_file teardown_file setup teardown __agx_register_test || true
  unset __agx_test_names __agx_test_files __agx_test_counter
}

export AGX_BATS_TMP_DIR="$tmp_dir"

for suite in "$@"; do
  run_suite "$suite"
done

printf '%s\n' "1..$((pass_count + fail_count))"
printf '%s\n' "# pass $pass_count"
if [[ $fail_count -ne 0 ]]; then
  printf '%s\n' "# fail $fail_count" >&2
  exit 1
fi
printf '%s\n' "# fail 0"
