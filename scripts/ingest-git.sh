#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# shellcheck source=lib/lock.sh
source "$SCRIPT_DIR/lib/lock.sh"
# shellcheck source=lib/git-clone.sh
source "$SCRIPT_DIR/lib/git-clone.sh"
# shellcheck source=lib/git-state.sh
source "$SCRIPT_DIR/lib/git-state.sh"
# shellcheck source=lib/git-config.sh
source "$SCRIPT_DIR/lib/git-config.sh"

usage() {
  cat >&2 <<USAGE
usage: ingest-git.sh <repo-spec> [flags]

  <repo-spec>          local path | https URL | git@ URL | config alias
  --paths=<csv>        override include paths (default README.md,docs/,rfcs/,adr/)
  --private            force private routing
  --protect-edits      stage conflicts under raw/inbox/checkpoint/.staged/
  --summarize          (reserved; not implemented v1)
  --dry-run            print plan + exit 0; no writes
  --repo-name=<name>   override derived name for slug prefix + entity page
USAGE
}

if [[ $# -lt 1 ]]; then
  usage; exit 1
fi

SPEC="$1"; shift
PATHS_OVERRIDE=""
PRIVATE_FLAG=""
PROTECT_EDITS=""
SUMMARIZE=""
DRY_RUN=""
REPO_NAME_OVERRIDE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --paths=*)        PATHS_OVERRIDE="${1#--paths=}";;
    --private)        PRIVATE_FLAG=1;;
    --protect-edits)  PROTECT_EDITS=1;;
    --summarize)      SUMMARIZE=1;;
    --dry-run)        DRY_RUN=1;;
    --repo-name=*)    REPO_NAME_OVERRIDE="${1#--repo-name=}";;
    -h|--help)        usage; exit 0;;
    *) echo "ERROR: unknown flag: $1" >&2; usage; exit 1;;
  esac
  shift
done

REPO_KEY="$(awiki_git_clone_repo_key "$SPEC")"
REPO_NAME="${REPO_NAME_OVERRIDE:-${REPO_KEY#local-}}"

if awiki_git_clone_is_ssh "$SPEC"; then
  PRIVATE_FLAG=1
fi

# Lock dir
mkdir -p .awiki/lock
LOCK_FILE=".awiki/lock/git-${REPO_KEY}"
: > "$LOCK_FILE.flockfile"

# Per-repo flock (separate from .awiki/lock single-file used by lib/lock.sh).
exec 9>"$LOCK_FILE.flockfile"
if ! flock -w "$AWIKI_LOCK_TIMEOUT_USER" -x 9; then
  echo "ERROR: lock acquire timeout for $REPO_KEY" >&2
  exit 13
fi

# Resolve checkout
RESOLVED="$(awiki_git_clone_resolve "$SPEC" "$REPO_KEY")" || {
  rc=$?; echo "ERROR: resolve failed (rc=$rc)" >&2; exit "$rc";
}
IFS='|' read -r CHECKOUT HEAD_SHA DEFAULT_BRANCH <<<"$RESOLVED"

# Determine include paths
DEFAULT_PATHS="README.md,docs/,rfcs/,adr/"
PATHS="${PATHS_OVERRIDE:-$DEFAULT_PATHS}"

# Walk repo: list every *.md / *.mdx, then filter by paths + exclude vendored
mapfile -t ALL_MD < <(git -C "$CHECKOUT" ls-tree -r HEAD --name-only 2>/dev/null | grep -E '\.(md|mdx)$' | LC_ALL=C sort)

filter_paths() {
  local rel="$1"
  case "$rel" in
    node_modules/*|*/node_modules/*|vendor/*|*/vendor/*|.git/*|*/.git/*) return 1;;
  esac
  IFS=',' read -ra ENTRIES <<<"$PATHS"
  for entry in "${ENTRIES[@]}"; do
    if [[ -z "$entry" ]]; then continue; fi
    if [[ "$entry" == */ ]]; then
      [[ "$rel" == "${entry}"* ]] && return 0
    else
      [[ "$rel" == "$entry" ]] && return 0
    fi
  done
  return 1
}

CURRENT_FILES=()
for rel in "${ALL_MD[@]}"; do
  if filter_paths "$rel"; then
    CURRENT_FILES+=("$rel")
  fi
done

# Build map repo-relpath → blob_sha
declare -A CURRENT_BLOB
for rel in "${CURRENT_FILES[@]}"; do
  CURRENT_BLOB["$rel"]="$(git -C "$CHECKOUT" rev-parse "HEAD:$rel" 2>/dev/null || echo "")"
done

# Load prior state
PRIOR_JSON="$(awiki_git_state_load "$REPO_KEY" 2>/dev/null || echo '{}')"
declare -A PRIOR_BLOB PRIOR_SLUG
while IFS=$'\t' read -r prel pblob pslug; do
  [[ -z "$prel" ]] && continue
  PRIOR_BLOB["$prel"]="$pblob"
  PRIOR_SLUG["$prel"]="$pslug"
done < <(printf '%s\n' "$PRIOR_JSON" | python3 -c '
import json, sys
try:
    obj = json.loads(sys.stdin.read() or "{}")
except Exception:
    obj = {}
for rel, info in (obj.get("files") or {}).items():
    blob = info.get("blob_sha", "")
    slug = info.get("slug", "")
    print(rel + "\t" + blob + "\t" + slug)
')

# Diff
ADDED=()
MODIFIED=()
UNCHANGED=()
REMOVED=()
for rel in "${CURRENT_FILES[@]}"; do
  pblob="${PRIOR_BLOB[$rel]:-}"
  cblob="${CURRENT_BLOB[$rel]}"
  if [[ -z "$pblob" ]]; then
    ADDED+=("$rel")
  elif [[ "$pblob" != "$cblob" ]]; then
    MODIFIED+=("$rel")
  else
    UNCHANGED+=("$rel")
  fi
done
for prel in "${!PRIOR_BLOB[@]}"; do
  if [[ -z "${CURRENT_BLOB[$prel]:-}" ]]; then
    REMOVED+=("$prel")
  fi
done

if [[ -n "$DRY_RUN" ]]; then
  echo "PLAN|spec=$SPEC|repo_key=$REPO_KEY|repo_name=$REPO_NAME|checkout=$CHECKOUT|head=$HEAD_SHA|branch=$DEFAULT_BRANCH|private=${PRIVATE_FLAG:-0}"
  echo "PLAN|added=${#ADDED[@]}|modified=${#MODIFIED[@]}|removed=${#REMOVED[@]}|unchanged=${#UNCHANGED[@]}"
  for f in "${ADDED[@]}";    do echo "PLAN|add|$f"; done
  for f in "${MODIFIED[@]}"; do echo "PLAN|mod|$f"; done
  for f in "${REMOVED[@]}";  do echo "PLAN|rem|$f"; done
  exit 0
fi

# (subsequent tasks add walk + transform + write + housekeeping)
echo "OK|repo_key=$REPO_KEY|head=$HEAD_SHA"
