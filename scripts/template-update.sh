#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS="$SCRIPT_DIR/_template_helpers"
REPO_ROOT="$(pwd)"

# Defaults
REF=""
SOURCE=""
APPLY=0
DRY_RUN=0
CONTINUE=0
ABORT=0
STATUS=0
SCHEMA_UPGRADE=0
ACCEPT_SOURCE_CHANGE=0
ACCEPT_ATTRIBUTE_CHANGES=0
ACCEPT_MANUAL_COMMITS=0
PERSIST_SOURCE=0
PRINT_MIGRATIONS=0
NON_INTERACTIVE=0
VERIFY_SIGNATURE=0
GC=0
RE_PIN=""
RERUN_BOOTSTRAP_STEP=""
SKIP_MIGRATION=""

usage() {
  cat <<'EOF'
usage: template-update.sh [flags]

Default = dry-run plan. --apply executes onto a dedicated update branch.

flags:
  --ref <sha-or-tag>          Pin a specific upstream ref (default: origin/main HEAD)
  --source <url-or-path>      Override repo source for one invocation
  --apply                     Execute the update (default is dry-run)
  --dry-run                   Force dry-run even with --apply
  --continue                  Resume after conflicts / migration failure
  --abort                     Discard in-progress update branch
  --status                    Print current pin + pending state; read-only
  --schema-upgrade            Allow schema-version bump
  --accept-source-change      Confirm --source differs from pin
  --accept-attribute-changes  Confirm .gitattributes filter changes
  --accept-manual-commits     Allow --continue past manual commits on update branch
  --persist-source            Save --source to .awiki/template.json.repo on success
  --rerun-bootstrap-step <id> Re-run a single (often dangerous) bootstrap step
  --print-migrations          Include full migration script bodies in plan
  --re-pin <commit>           Set pin without running an update (rollback escape hatch)
  --gc                        Prune orphaned cache dirs and exit
  --non-interactive           Auto-resolve prompts to safe defaults; CI mode
  --verify-signature          Require git verify-tag/verify-commit on the fetched ref
  --skip-migration <id>       Skip a specific migration during apply
EOF
}

# Parse args.
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --ref) REF="$2"; shift 2 ;;
    --source) SOURCE="$2"; shift 2 ;;
    --apply) APPLY=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --continue) CONTINUE=1; shift ;;
    --abort) ABORT=1; shift ;;
    --status) STATUS=1; shift ;;
    --schema-upgrade) SCHEMA_UPGRADE=1; shift ;;
    --accept-source-change) ACCEPT_SOURCE_CHANGE=1; shift ;;
    --accept-attribute-changes) ACCEPT_ATTRIBUTE_CHANGES=1; shift ;;
    --accept-manual-commits) ACCEPT_MANUAL_COMMITS=1; shift ;;
    --persist-source) PERSIST_SOURCE=1; shift ;;
    --rerun-bootstrap-step) RERUN_BOOTSTRAP_STEP="$2"; shift 2 ;;
    --print-migrations) PRINT_MIGRATIONS=1; shift ;;
    --re-pin) RE_PIN="$2"; shift 2 ;;
    --gc) GC=1; shift ;;
    --non-interactive) NON_INTERACTIVE=1; shift ;;
    --verify-signature) VERIFY_SIGNATURE=1; shift ;;
    --skip-migration) SKIP_MIGRATION="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

# Dry-run wins if both flags set.
if [[ $DRY_RUN -eq 1 ]]; then APPLY=0; fi

PJ="$REPO_ROOT/.awiki/template.json"
[[ -f "$PJ" ]] || { echo "halt: no .awiki/template.json. Run 'just template-init' or 'just template-retrofit'." >&2; exit 1; }

# Opt-out: repo == "none" disables update.
PIN_REPO_OPTOUT=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" repo 2>/dev/null)
if [[ "$PIN_REPO_OPTOUT" == "none" ]]; then
  if [[ $STATUS -eq 1 ]]; then
    echo "version: $(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" version)"
    echo "commit:  $(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" commit)"
    echo "repo:    none (updates disabled)"
    exit 0
  fi
  echo "info: template updates disabled (.awiki/template.json.repo = \"none\"). Re-enable by editing template.json or using --persist-source." >&2
  exit 0
fi

# Phase-skip helper used by all later phases when RESUMED=1.
# Phase order: fetch < schema-upgrade < commit-a < commit-b < commit-c < commit-d
PHASE_ORDER=(fetch schema-upgrade commit-a commit-b commit-c commit-d)
phase_index() {
  local target="$1" idx=0
  for p in "${PHASE_ORDER[@]}"; do
    [[ "$p" == "$target" ]] && { echo $idx; return 0; }
    idx=$((idx+1))
  done
  echo -1
}
should_skip_phase() {
  local target="$1"
  [[ "${RESUMED:-0}" -eq 1 ]] || return 1
  local target_idx
  target_idx=$(phase_index "$target")
  local resumed_idx
  resumed_idx=$(phase_index "$CONTINUE_FROM_PHASE")
  if [[ $target_idx -lt $resumed_idx ]]; then return 0; fi
  if [[ $target_idx -eq $resumed_idx && "$CONTINUE_FROM_STATUS" == "committed" ]]; then return 0; fi
  return 1
}
export -f phase_index should_skip_phase

# Defer --status, --re-pin, --gc, --abort, --continue, --rerun-bootstrap-step to later phases.
if [[ -n "$RE_PIN" || $GC -eq 1 || $ABORT -eq 1 || $CONTINUE -eq 1 || $STATUS -eq 1 || -n "$RERUN_BOOTSTRAP_STEP" ]]; then
  echo "stub: this command path is implemented in a later phase" >&2
  exit 0
fi

# === Phase 0a: pre-fetch preflight ===
if [[ "${RESUMED:-0}" -eq 1 ]]; then
  echo "info: resume — skipping Phase 0a (already validated this cycle)"
else
DEFAULT_BRANCH=$(bash "$SCRIPT_DIR/template-config.sh" get "$REPO_ROOT/.awiki/config" default_branch main)

python3 "$HELPERS/preflight.py" check-tree
python3 "$HELPERS/preflight.py" check-branch --expected "$DEFAULT_BRANCH"
python3 "$HELPERS/preflight.py" check-pending-prompts

# Encryption preflight uses CURRENT manifest in the repo (template-shipped at bootstrap).
if [[ -f "$REPO_ROOT/template.manifest.toml" ]]; then
  python3 "$HELPERS/preflight.py" check-encryption --manifest "$REPO_ROOT/template.manifest.toml"
fi

# Source-change check.
PIN_REPO=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" repo)
RESOLVED_SOURCE="${SOURCE:-$PIN_REPO}"
SC_ARGS=(--provenance "$PJ" --source "$RESOLVED_SOURCE")
[[ $ACCEPT_SOURCE_CHANGE -eq 1 ]] && SC_ARGS+=(--accept-source-change)
bash "$SCRIPT_DIR/template-source-check.sh" "${SC_ARGS[@]}"

echo "info: phase 0a preflight ok"
fi  # end Phase 0a (skipped on --continue)

# Phases 1, 0b, 1.5, 2, 3 — STUBBED until later tasks/phases.
echo "info: subsequent phases not yet implemented"
exit 0
