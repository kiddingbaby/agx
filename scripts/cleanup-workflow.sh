#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

APPLY=0
KEEP_DAYS="${AGX_WORKFLOW_KEEP_DAYS:-14}"
MAX_ARCHIVES="${AGX_WORKFLOW_MAX_ARCHIVES:-10}"

usage() {
  cat <<'EOF'
agx .workflow 清理（默认 dry-run）

Usage:
  bash scripts/cleanup-workflow.sh [--apply] [--keep-days N] [--max-archives N]

Options:
  --apply            实际删除（否则仅输出将删除内容）
  --keep-days N      仅保留 N 天内 archive/reports（默认 14；0 表示不按天数清理）
  --max-archives N   最多保留 N 个 archive 条目（默认 10；0 表示不按数量清理）
  -h, --help         帮助

Env:
  AGX_WORKFLOW_KEEP_DAYS
  AGX_WORKFLOW_MAX_ARCHIVES
EOF
}

is_uint() {
  [[ "$1" =~ ^[0-9]+$ ]]
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --apply)
      APPLY=1
      shift
      ;;
    --keep-days)
      KEEP_DAYS="${2:?missing value for --keep-days}"
      shift 2
      ;;
    --max-archives)
      MAX_ARCHIVES="${2:?missing value for --max-archives}"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown arg: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
done

is_uint "${KEEP_DAYS}" || { echo "--keep-days must be a non-negative integer: ${KEEP_DAYS}" >&2; exit 2; }
is_uint "${MAX_ARCHIVES}" || { echo "--max-archives must be a non-negative integer: ${MAX_ARCHIVES}" >&2; exit 2; }

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_dir="$(cd "${script_dir}/.." && pwd)"
workflow_dir="${repo_dir}/.workflow"
archive_dir="${workflow_dir}/archive"
reports_dir="${workflow_dir}/reports"

if [[ ! -d "${workflow_dir}" ]]; then
  echo ".workflow 不存在：${workflow_dir}"
  exit 0
fi

du_safe() {
  if command -v du >/dev/null 2>&1; then
    du -sh "$@" 2>/dev/null || true
  fi
}

echo "[agx] .workflow: ${workflow_dir}"
du_safe "${workflow_dir}" "${archive_dir}" "${reports_dir}"

declare -A archives=()
if [[ -d "${archive_dir}" ]]; then
  while IFS= read -r name; do
    [[ -n "${name}" ]] || continue
    archives["$name"]=1
  done < <(find "${archive_dir}" -mindepth 1 -maxdepth 1 -type d -printf '%f\n' 2>/dev/null | sort || true)
fi

declare -A del_archives=()

if [[ "${KEEP_DAYS}" != "0" && -d "${archive_dir}" ]]; then
  while IFS= read -r name; do
    [[ -n "${name}" ]] || continue
    del_archives["$name"]=1
  done < <(find "${archive_dir}" -mindepth 1 -maxdepth 1 -type d -mtime +"${KEEP_DAYS}" -printf '%f\n' 2>/dev/null | sort || true)
fi

if [[ "${MAX_ARCHIVES}" != "0" ]]; then
  mapfile -t all_sorted < <(printf '%s\n' "${!archives[@]}" | sort)
  total="${#all_sorted[@]}"
  if [[ "${total}" -gt "${MAX_ARCHIVES}" ]]; then
    cutoff=$((total - MAX_ARCHIVES))
    for ((i=0; i<cutoff; i++)); do
      del_archives["${all_sorted[$i]}"]=1
    done
  fi
fi

declare -a archive_paths=()
for name in "${!del_archives[@]}"; do
  [[ -n "${name}" ]] || continue
  archive_paths+=("${archive_dir}/${name}")
done
IFS=$'\n' archive_paths=($(printf '%s\n' "${archive_paths[@]}" | sort))
unset IFS

declare -a report_paths=()
if [[ -d "${reports_dir}" && "${KEEP_DAYS}" != "0" ]]; then
  while IFS= read -r -d '' path; do
    report_paths+=("$path")
  done < <(find "${reports_dir}" -mindepth 1 -maxdepth 1 -mtime +"${KEEP_DAYS}" -print0 2>/dev/null || true)
fi

echo
echo "[agx] archive delete candidates: ${#archive_paths[@]} (keep-days=${KEEP_DAYS}, max-archives=${MAX_ARCHIVES})"
echo "[agx] reports delete candidates: ${#report_paths[@]} (keep-days=${KEEP_DAYS})"

preview_list() {
  local title="$1"; shift
  local -a items=("$@")
  echo
  echo "${title}"
  local n="${#items[@]}"
  if [[ "$n" -eq 0 ]]; then
    echo "  (none)"
    return 0
  fi
  local limit=60
  local show="$n"
  if [[ "$show" -gt "$limit" ]]; then
    show="$limit"
  fi
  for ((i=0; i<show; i++)); do
    echo "  - ${items[$i]}"
  done
  if [[ "$n" -gt "$limit" ]]; then
    echo "  ... (${n} total)"
  fi
}

preview_list "[agx] will clean archive:" "${archive_paths[@]}"
preview_list "[agx] will clean reports:" "${report_paths[@]}"

if [[ "${APPLY}" -ne 1 ]]; then
  echo
  echo "dry-run 完成；实际删除请加 --apply"
  exit 0
fi

echo
echo "[agx] APPLY=1: removing..."

for path in "${archive_paths[@]}"; do
  [[ "${path}" == "${archive_dir}/"* ]] || { echo "refuse to remove (out of scope): ${path}" >&2; exit 3; }
  rm -rf -- "${path}"
done

for path in "${report_paths[@]}"; do
  [[ "${path}" == "${reports_dir}/"* ]] || { echo "refuse to remove (out of scope): ${path}" >&2; exit 3; }
  rm -rf -- "${path}"
done

echo
echo "[agx] after cleanup:"
du_safe "${workflow_dir}" "${archive_dir}" "${reports_dir}"
