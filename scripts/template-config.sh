#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Recognized keys (v1 spec).
KNOWN_KEYS=("default_branch" "no_template_check" "require_signature" "ingest_lint_threshold")

usage() {
  cat <<EOF
usage:
  template-config.sh get <path> <key> [default]
  template-config.sh validate <path>
EOF
  exit 2
}

cmd_get() {
  local path="$1" key="$2" default="${3:-}"
  if [[ ! -f "$path" ]]; then
    echo -n "$default"
    echo
    return 0
  fi
  local val
  val=$(awk -F= -v k="$key" '
    !/^#/ && !/^[[:space:]]*$/ && $1==k { sub(/^[^=]*=/, ""); print; exit }
  ' "$path")
  if [[ -z "$val" ]]; then
    echo -n "$default"
  else
    echo -n "$val"
  fi
  echo
}

cmd_validate() {
  local path="$1"
  if [[ ! -f "$path" ]]; then return 0; fi
  local lineno=0
  local rc=0
  while IFS= read -r line; do
    lineno=$((lineno + 1))
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    if ! [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*=.*$ ]]; then
      echo "config error: malformed line $lineno: $line" >&2
      rc=1
      continue
    fi
    local key="${line%%=*}"
    local known=0
    for k in "${KNOWN_KEYS[@]}"; do
      if [[ "$k" == "$key" ]]; then known=1; break; fi
    done
    if [[ $known -eq 0 ]]; then
      echo "config warning: unknown key $key (line $lineno)" >&2
    fi
  done < "$path"
  return $rc
}

main() {
  if [[ $# -lt 2 ]]; then usage; fi
  local subcmd="$1"; shift
  case "$subcmd" in
    get) cmd_get "$@" ;;
    validate) cmd_validate "$@" ;;
    *) usage ;;
  esac
}

main "$@"
