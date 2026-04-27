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

# === Phase 1: fetch ===
FETCH_DIR="$REPO_ROOT/.awiki/template-cache/_fetch"

if should_skip_phase fetch; then
  echo "info: resume — skipping Phase 1 (fetch already committed this cycle)"
  # Re-derive variables from state file.
  COMMIT_OLD=$(python3 "$HELPERS/state.py" get "$FETCH_DIR/.update-state.json" commit_old)
  COMMIT_NEW=$(python3 "$HELPERS/state.py" get "$FETCH_DIR/.update-state.json" commit_new)
  ANCESTOR_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_OLD"
  NEW_MANIFEST="$FETCH_DIR/template.manifest.toml"
else
COMMIT_OLD=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" commit)
ORIGINAL_REPO=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" original_repo)

mkdir -p "$REPO_ROOT/.awiki/template-cache"
if [[ -d "$FETCH_DIR/.git" ]]; then
  # Reuse existing fetch dir: fetch updates.
  git -C "$FETCH_DIR" fetch --depth 50 origin >/dev/null 2>&1 || true
elif [[ -d "$FETCH_DIR" ]]; then
  # Stale dir without .git (e.g. local copy fixture). Wipe + re-clone or copy.
  rm -rf "$FETCH_DIR"
fi

if [[ ! -d "$FETCH_DIR/.git" ]]; then
  if [[ "$RESOLVED_SOURCE" == /* ]] || [[ -d "$RESOLVED_SOURCE/.git" ]]; then
    # Local path: prefer git clone if it's a repo, else snapshot copy.
    if [[ -d "$RESOLVED_SOURCE/.git" ]]; then
      git clone --depth 50 "$RESOLVED_SOURCE" "$FETCH_DIR" >/dev/null
    else
      mkdir -p "$FETCH_DIR"
      cp -R "$RESOLVED_SOURCE/." "$FETCH_DIR/"
      # Init a throwaway git repo so `git rev-parse` works.
      git -C "$FETCH_DIR" init -q
      git -C "$FETCH_DIR" add -A
      git -C "$FETCH_DIR" -c user.email=fetch@local -c user.name=fetch commit -q -m "snapshot: $RESOLVED_SOURCE"
    fi
  else
    git clone --depth 50 "$RESOLVED_SOURCE" "$FETCH_DIR" >/dev/null
  fi
fi

# Resolve commit_new.
if [[ -n "$REF" ]]; then
  COMMIT_NEW=$(git -C "$FETCH_DIR" rev-parse "$REF")
else
  COMMIT_NEW=$(git -C "$FETCH_DIR" rev-parse HEAD)
fi

if [[ "$COMMIT_NEW" == "$COMMIT_OLD" ]]; then
  echo "already up to date (pinned at $COMMIT_OLD)"
  rm -rf "$FETCH_DIR"
  exit 0
fi

# Ancestor cache check + auto-recover.
ANCESTOR_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_OLD"
if [[ ! -d "$ANCESTOR_DIR" ]]; then
  echo "info: ancestor cache missing for $COMMIT_OLD; rebuilding from $ORIGINAL_REPO"
  TMP_ANC=$(mktemp -d)
  if [[ -d "$ORIGINAL_REPO/.git" ]] || [[ "$ORIGINAL_REPO" == http* ]]; then
    if git clone --depth 50 "$ORIGINAL_REPO" "$TMP_ANC/orig" >/dev/null 2>&1; then
      git -C "$TMP_ANC/orig" checkout -q "$COMMIT_OLD" 2>/dev/null || \
        git -C "$TMP_ANC/orig" fetch --depth 50 origin "$COMMIT_OLD" >/dev/null 2>&1 || true
      git -C "$TMP_ANC/orig" checkout -q "$COMMIT_OLD"
      mkdir -p "$ANCESTOR_DIR"
      git -C "$TMP_ANC/orig" archive --format=tar HEAD | tar -x -C "$ANCESTOR_DIR"
      echo "Re-built ancestor cache from pin"
    else
      echo "halt: cannot reach original_repo $ORIGINAL_REPO. Run 'just template-retrofit'." >&2
      rm -rf "$TMP_ANC"
      exit 1
    fi
  else
    # Local non-git source: snapshot copy.
    mkdir -p "$ANCESTOR_DIR"
    cp -R "$ORIGINAL_REPO/." "$ANCESTOR_DIR/"
    echo "Re-built ancestor cache from pin"
  fi
  rm -rf "$TMP_ANC"
fi

# Initialize state file.
SHORT_NEW=$(echo "$COMMIT_NEW" | head -c 12)
BRANCH_NAME="awiki-template-update/$SHORT_NEW"
python3 "$HELPERS/state.py" init "$FETCH_DIR/.update-state.json" \
  --commit-old "$COMMIT_OLD" --commit-new "$COMMIT_NEW" --branch "$BRANCH_NAME"

echo "info: phase 1 fetch ok (commit_new=$COMMIT_NEW)"
fi  # end Phase 1 (skipped on resume)

# === Phase 0b: post-fetch preflight ===
if [[ "${RESUMED:-0}" -eq 1 ]]; then
  echo "info: resume — skipping Phase 0b"
else
NEW_MANIFEST="$FETCH_DIR/template.manifest.toml"
[[ -f "$NEW_MANIFEST" ]] || { echo "halt: fetched template missing template.manifest.toml" >&2; exit 1; }

NEW_SCHEMA=$(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST" | awk -F= '$1=="schema_version"{print $2}')
PIN_SCHEMA=$(python3 -c "import json; print(json.load(open('$PJ'))['schema_version'])")

if [[ "$NEW_SCHEMA" != "$PIN_SCHEMA" ]]; then
  if [[ $SCHEMA_UPGRADE -eq 0 ]]; then
    echo "halt: template schema_version=$NEW_SCHEMA, pin schema_version=$PIN_SCHEMA. Re-run with --schema-upgrade." >&2
    exit 1
  fi
fi

# Encryption recheck against NEW manifest.
python3 "$HELPERS/preflight.py" check-encryption --manifest "$NEW_MANIFEST"

# Optional signature verification.
REQUIRE_SIG=$(bash "$SCRIPT_DIR/template-config.sh" get "$REPO_ROOT/.awiki/config" require_signature false)
if [[ $VERIFY_SIGNATURE -eq 1 ]] || [[ "$REQUIRE_SIG" == "true" ]]; then
  TARGET="${REF:-$COMMIT_NEW}"
  # Try verify-tag first; fall back to verify-commit.
  if ! git -C "$FETCH_DIR" verify-tag "$TARGET" 2>/dev/null \
       && ! git -C "$FETCH_DIR" verify-commit "$TARGET" 2>/dev/null; then
    echo "halt: signature verification failed for $TARGET" >&2
    exit 1
  fi
  echo "info: signature verified for $TARGET"
fi

echo "info: phase 0b ok"
fi  # end Phase 0b

# === Phase 1.5: schema upgrade (Commit 0) ===
if should_skip_phase schema-upgrade; then
  echo "info: resume — skipping Phase 1.5 (schema-upgrade already committed)"
elif [[ "$NEW_SCHEMA" != "$PIN_SCHEMA" ]] && [[ $SCHEMA_UPGRADE -eq 1 ]]; then
  if [[ $APPLY -eq 0 ]]; then
    echo "info: schema-upgrade required; would run migrations/schema-${PIN_SCHEMA}-to-${NEW_SCHEMA}.sh on --apply"
  else
    SU_SCRIPT="$FETCH_DIR/migrations/schema-${PIN_SCHEMA}-to-${NEW_SCHEMA}.sh"
    [[ -x "$SU_SCRIPT" ]] || { echo "halt: schema-upgrade script missing: $SU_SCRIPT" >&2; exit 1; }

    SHORT_NEW=$(echo "$COMMIT_NEW" | head -c 12)
    BRANCH_NAME="awiki-template-update/$SHORT_NEW"
    if git rev-parse --verify --quiet "refs/heads/$BRANCH_NAME" >/dev/null; then
      echo "halt: branch $BRANCH_NAME already exists. Resolve or --abort first." >&2
      exit 1
    fi
    git checkout -q -b "$BRANCH_NAME"

    python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase schema-upgrade --status started

    # Run with stripped env. .awiki/ writes whitelisted (no post-run audit).
    env -i \
      PATH="$PATH" HOME="$HOME" LANG="${LANG:-}" LC_ALL="${LC_ALL:-}" \
      AWIKI_REPO_ROOT="$REPO_ROOT" \
      AWIKI_TEMPLATE_OLD_VERSION="$(python3 -c "import json; print(json.load(open('$PJ'))['version'])")" \
      AWIKI_TEMPLATE_NEW_VERSION="$(awk -F= '$1=="template_version"{print $2}' <(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST"))" \
      bash "$SU_SCRIPT"

    git add -A
    git commit -q -m "chore(template): schema upgrade ${PIN_SCHEMA} → ${NEW_SCHEMA}"
    SU_SHA=$(git rev-parse HEAD)

    python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase schema-upgrade --status committed
    python3 "$HELPERS/state.py" set-last-completed "$FETCH_DIR/.update-state.json" "$SU_SHA"

    # Record schema upgrade in state.applied_migrations_pending — Commit D writes template.json.
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "schema-${PIN_SCHEMA}-to-${NEW_SCHEMA}" --status applied
    echo "info: phase 1.5 schema upgrade committed ($SU_SHA)"
  fi
fi

# === Phase 2: plan ===
if [[ "${RESUMED:-0}" -eq 1 ]]; then
  echo "info: resume — skipping Phase 2 (plan output not regenerated)"
  PLAN_OUT=""   # downstream phases must not depend on PLAN_OUT after resume; orchestrator already committed Commit A from the original plan.
else
SCRATCH="$FETCH_DIR/_scratch-merge"
mkdir -p "$SCRATCH"

# user-tree = bootstrapped repo (REPO_ROOT) but excluding .awiki/.
USER_TREE_TMP="$SCRATCH/user-tree"
mkdir -p "$USER_TREE_TMP"
git -C "$REPO_ROOT" archive --format=tar HEAD | tar -x -C "$USER_TREE_TMP"

# Capture plan stdout.
PLAN_OUT=$(bash "$SCRIPT_DIR/template-plan.sh" \
  --old-tree "$ANCESTOR_DIR" \
  --new-tree "$FETCH_DIR" \
  --user-tree "$USER_TREE_TMP" \
  --manifest "$NEW_MANIFEST" \
  --commit-old "$COMMIT_OLD" \
  --commit-new "$COMMIT_NEW" \
  --provenance "$PJ")

echo "$PLAN_OUT"

# If --print-migrations, frame each migration body.
if [[ $PRINT_MIGRATIONS -eq 1 ]]; then
  for MIG in "$FETCH_DIR/migrations/"*.sh; do
    [[ -f "$MIG" ]] || continue
    [[ "$(basename "$MIG")" == "schema-"* ]] && continue
    MID="$(basename "$MIG" .sh)"
    echo "PLAN|migration-body|$MID|begin"
    while IFS= read -r line; do
      ESC=$(python3 "$HELPERS/escape.py" "$line")
      echo "PLAN|migration-body|$MID|$ESC"
    done < "$MIG"
    echo "PLAN|migration-body|$MID|end"
  done
fi

# Clean up scratch.
rm -rf "$SCRATCH"

if [[ $APPLY -eq 0 ]]; then
  # Dry-run: clean up _fetch too unless the user might re-run with --apply.
  # Spec leaves _fetch around between dry-run and --apply for cache reuse.
  exit 0
fi
fi  # end Phase 2 (skipped on resume)

echo "info: phase 3 not yet implemented (Commit A lands in Phase 05)"
exit 0
