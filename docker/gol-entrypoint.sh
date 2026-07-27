#!/usr/bin/env bash
set -euo pipefail

bool_enabled() {
  case "${1:-}" in
    1 | true | TRUE | yes | YES | on | ON) return 0 ;;
    *) return 1 ;;
  esac
}

append_lines() {
  local value="${1:-}"
  local prefix="$2"
  local line

  value="${value//\\n/$'\n'}"
  while IFS= read -r line || [[ -n "$line" ]]; do
    line="${line%$'\r'}"
    [[ -z "${line//[[:space:]]/}" ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    gol_args+=("${prefix}=${line}")
  done <<< "$value"
}

case "${1:-}" in
  -h | --help | -help | help | -v | --v | -version | --version)
    exec gol "$@"
    ;;
esac

if [[ $# -gt 0 && "$1" != -* ]]; then
  exec "$@"
fi

gol_args=(
  "--host=0.0.0.0"
  "--port=${GOL_PORT:-3003}"
  "--open=false"
)

[[ -n "${GOL_BASE_URL:-}" && "${GOL_BASE_URL}" != "/" ]] && gol_args+=("--base-url=${GOL_BASE_URL}")
[[ -n "${GOL_EVERY:-}" ]] && gol_args+=("--every=${GOL_EVERY}")
[[ -n "${GOL_LIMIT:-}" ]] && gol_args+=("--limit=${GOL_LIMIT}")
bool_enabled "${GOL_ACCESS:-}" && gol_args+=("--access")

append_lines "${GOL_FILE_PATTERNS:-}" "-f"
append_lines "${GOL_SSH_TARGETS:-}" "-s"
if bool_enabled "${GOL_DOCKER_ALL:-}"; then
  gol_args+=("-d=")
fi
append_lines "${GOL_DOCKER_TARGETS:-}" "-d"

exec gol "${gol_args[@]}" "$@"
