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

if [[ -n "$DRY_RUN" ]]; then
  echo "PLAN|spec=$SPEC|repo_key=$REPO_KEY|repo_name=$REPO_NAME|checkout=$CHECKOUT|head=$HEAD_SHA|branch=$DEFAULT_BRANCH|private=${PRIVATE_FLAG:-0}"
  exit 0
fi

# (subsequent tasks add walk + transform + write + housekeeping)
echo "OK|repo_key=$REPO_KEY|head=$HEAD_SHA"
