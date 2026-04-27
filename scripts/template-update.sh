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

# === Subcommand dispatch (recovery flows) ===

# --re-pin <commit>: rollback escape hatch. Validate commit upstream, drop orphan cache,
# rebuild target ancestor cache, write new pin to template.json. Refuses if pending-prompts
# or state file present.
if [[ -n "$RE_PIN" ]]; then
  if [[ -d "$REPO_ROOT/.awiki/pending-prompts" ]] && [[ -n "$(ls -A "$REPO_ROOT/.awiki/pending-prompts" 2>/dev/null)" ]]; then
    echo "halt: pending-prompts present — resolve before --re-pin" >&2
    exit 1
  fi
  if [[ -f "$REPO_ROOT/.awiki/template-cache/_fetch/.update-state.json" ]]; then
    echo "halt: update in progress — run --abort first" >&2
    exit 1
  fi

  ORIGINAL_REPO=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" original_repo)
  TMP_VALIDATE=$(mktemp -d)
  REACHED=0
  if [[ -d "$ORIGINAL_REPO/.git" ]] || [[ "$ORIGINAL_REPO" == http* ]]; then
    if ! git clone --depth 50 "$ORIGINAL_REPO" "$TMP_VALIDATE/orig" >/dev/null 2>&1; then
      echo "halt: cannot reach $ORIGINAL_REPO. Required to validate --re-pin commit." >&2
      rm -rf "$TMP_VALIDATE"
      exit 1
    fi
    REACHED=1
    if ! git -C "$TMP_VALIDATE/orig" cat-file -e "$RE_PIN^{commit}" 2>/dev/null; then
      # Try fetching the specific commit (may fail on shallow clones for arbitrary SHAs).
      if ! git -C "$TMP_VALIDATE/orig" fetch --depth 50 origin "$RE_PIN" >/dev/null 2>&1 \
           || ! git -C "$TMP_VALIDATE/orig" cat-file -e "$RE_PIN^{commit}" 2>/dev/null; then
        echo "halt: commit $RE_PIN not resolvable in $ORIGINAL_REPO" >&2
        rm -rf "$TMP_VALIDATE"
        exit 1
      fi
    fi
  fi

  # Drop orphaned cache for previous pin.
  CUR_COMMIT=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" commit)
  if [[ -n "$CUR_COMMIT" && "$CUR_COMMIT" != "$RE_PIN" ]]; then
    rm -rf "$REPO_ROOT/.awiki/template-cache/$CUR_COMMIT"
  fi

  # Rebuild target ancestor cache.
  TARGET_CACHE="$REPO_ROOT/.awiki/template-cache/$RE_PIN"
  if [[ ! -d "$TARGET_CACHE" ]] && [[ $REACHED -eq 1 ]]; then
    git -C "$TMP_VALIDATE/orig" checkout -q "$RE_PIN" 2>/dev/null || true
    mkdir -p "$TARGET_CACHE"
    git -C "$TMP_VALIDATE/orig" archive --format=tar HEAD | tar -x -C "$TARGET_CACHE"
  fi
  rm -rf "$TMP_VALIDATE"

  bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" commit "$RE_PIN" >/dev/null
  echo "info: re-pinned to $RE_PIN; cache rebuilt + orphan dropped."
  echo "info: if you reverted the merge, content is back to pre-update state."
  exit 0
fi

# --continue: resume an in-progress update from the state file.
if [[ $CONTINUE -eq 1 ]]; then
  FETCH_DIR="$REPO_ROOT/.awiki/template-cache/_fetch"
  STATE="$FETCH_DIR/.update-state.json"
  if [[ ! -f "$STATE" ]]; then
    echo "halt: No update in progress (no state file at $STATE)." >&2
    exit 1
  fi
  PHASE=$(python3 "$HELPERS/state.py" get "$STATE" phase)
  STATUS_FROM_STATE=$(python3 "$HELPERS/state.py" get "$STATE" status)
  LAST_SHA=$(python3 "$HELPERS/state.py" get "$STATE" last_completed_commit 2>/dev/null || echo "")
  BRANCH=$(python3 "$HELPERS/state.py" get "$STATE" branch)

  # Switch to update branch if needed.
  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "")
  if [[ "$CUR_BRANCH" != "$BRANCH" ]]; then
    git checkout -q "$BRANCH" 2>/dev/null || { echo "halt: cannot switch to $BRANCH" >&2; exit 1; }
  fi

  HEAD_SHA=$(git rev-parse HEAD 2>/dev/null || echo "")
  if [[ -n "$LAST_SHA" && "$LAST_SHA" != "None" && "$LAST_SHA" != "null" \
        && "$LAST_SHA" != "$HEAD_SHA" && $ACCEPT_MANUAL_COMMITS -eq 0 ]]; then
    echo "halt: Manual commits detected on update branch (HEAD=$HEAD_SHA, expected=$LAST_SHA). Re-run with --accept-manual-commits." >&2
    exit 1
  fi

  echo "info: --continue resuming after phase=$PHASE status=$STATUS_FROM_STATE"

  # If the in-progress phase is "started", roll back to last_completed_commit so the
  # phase re-runs from a clean slate.
  if [[ "$STATUS_FROM_STATE" == "started" && -n "$LAST_SHA" \
        && "$LAST_SHA" != "None" && "$LAST_SHA" != "null" ]]; then
    git reset --hard "$LAST_SHA" >/dev/null
  fi

  export CONTINUE_FROM_PHASE="$PHASE"
  export CONTINUE_FROM_STATUS="$STATUS_FROM_STATE"
  RESUMED=1

  # Re-derive variables that the linear flow expects.
  COMMIT_OLD=$(python3 "$HELPERS/state.py" get "$STATE" commit_old)
  COMMIT_NEW=$(python3 "$HELPERS/state.py" get "$STATE" commit_new)
  ANCESTOR_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_OLD"
  NEW_MANIFEST="$FETCH_DIR/template.manifest.toml"
  RESOLVED_SOURCE=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" repo)
  SHORT_NEW=$(echo "$COMMIT_NEW" | head -c 12)
  BRANCH_NAME="$BRANCH"
  # Force --apply on resume (otherwise we'd just print the plan again).
  APPLY=1
  # Re-derive schema for Phase 1.5 gate.
  if [[ -f "$NEW_MANIFEST" ]]; then
    NEW_SCHEMA=$(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST" | awk -F= '$1=="schema_version"{print $2}')
    PIN_SCHEMA=$(python3 -c "import json; print(json.load(open('$PJ'))['schema_version'])")
  fi
  # Fall through to linear flow; phase blocks gated by should_skip_phase will skip already-committed work.
fi

# --abort: clean up _fetch + delete update branch + restore default branch.
if [[ $ABORT -eq 1 ]]; then
  FETCH_DIR="$REPO_ROOT/.awiki/template-cache/_fetch"
  STATE="$FETCH_DIR/.update-state.json"
  if [[ -f "$STATE" ]]; then
    BRANCH=$(python3 "$HELPERS/state.py" get "$STATE" branch 2>/dev/null || echo "")
    DEFAULT_BRANCH=$(bash "$SCRIPT_DIR/template-config.sh" get "$REPO_ROOT/.awiki/config" default_branch main)
    git checkout -q "$DEFAULT_BRANCH" 2>/dev/null || true
    if [[ -n "$BRANCH" && "$BRANCH" != "null" ]] && git rev-parse --verify --quiet "refs/heads/$BRANCH" >/dev/null; then
      git branch -D "$BRANCH" >/dev/null 2>&1 || true
    fi
    rm -rf "$FETCH_DIR"
    echo "info: aborted update; $DEFAULT_BRANCH restored"
  else
    # Even with no state, clean up bare _fetch if present.
    [[ -d "$FETCH_DIR" ]] && rm -rf "$FETCH_DIR"
    echo "info: no in-progress update"
  fi
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

# === Phase 3 — Commit A: sync ===
if should_skip_phase commit-a; then
  echo "info: resume — skipping Commit A"
else
SHORT_NEW=$(echo "$COMMIT_NEW" | head -c 12)
BRANCH_NAME="awiki-template-update/$SHORT_NEW"

# Branch may already exist if Phase 1.5 created it.
CUR=$(git rev-parse --abbrev-ref HEAD)
if [[ "$CUR" != "$BRANCH_NAME" ]]; then
  if git rev-parse --verify --quiet "refs/heads/$BRANCH_NAME" >/dev/null; then
    echo "halt: branch $BRANCH_NAME already exists. Resolve or --abort first." >&2
    exit 1
  fi
  git checkout -q -b "$BRANCH_NAME"
fi

python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-a --status started

# Walk plan output line-by-line.
THREE_PATHS_FOR_MARKER_CHECK=()

while IFS= read -r line; do
  [[ "$line" =~ ^PLAN\| ]] || continue
  IFS='|' read -ra parts <<< "$line"
  TYPE="${parts[1]}"
  case "$TYPE" in
    overwrite)
      python3 "$HELPERS/sync.py" apply-overwrite \
        --new-tree "$FETCH_DIR" --user-tree "$REPO_ROOT" --rel "${parts[2]}"
      ;;
    three_way)
      REL="${parts[2]}"
      python3 "$HELPERS/sync.py" apply-three-way \
        --old-tree "$ANCESTOR_DIR" --new-tree "$FETCH_DIR" --user-tree "$REPO_ROOT" --rel "$REL" || true
      THREE_PATHS_FOR_MARKER_CHECK+=("$REL")
      ;;
    attributes_merge)
      REL="${parts[2]}"
      ATTR_ARGS=(--old-tree "$ANCESTOR_DIR" --new-tree "$FETCH_DIR" --user-tree "$REPO_ROOT" --rel "$REL")
      [[ $ACCEPT_ATTRIBUTE_CHANGES -eq 1 ]] && ATTR_ARGS+=(--accept-attribute-changes)
      if ! python3 "$HELPERS/sync.py" apply-attributes "${ATTR_ARGS[@]}"; then
        echo "halt: attributes_merge gate. Re-run with --accept-attribute-changes." >&2
        exit 1
      fi
      THREE_PATHS_FOR_MARKER_CHECK+=("$REL")
      ;;
    new_file)
      REL="${parts[2]}"
      DECISION="skip"
      if [[ $NON_INTERACTIVE -eq 0 ]]; then
        echo "New file from template: $REL"
        echo "  [o]verwrite  [s]kip  [m]ark-as-user-deleted (default: skip)"
        read -r -p "> " ans
        case "$ans" in
          o|overwrite) DECISION="overwrite" ;;
          m|mark-as-user-deleted) DECISION="mark-as-user-deleted" ;;
          *) DECISION="skip" ;;
        esac
      fi
      python3 "$HELPERS/sync.py" apply-new-file \
        --new-tree "$FETCH_DIR" --user-tree "$REPO_ROOT" --rel "$REL" --decision "$DECISION"
      # Record mark-as-user-deleted in state so Commit D writes template.json.deleted[].
      if [[ "$DECISION" == "mark-as-user-deleted" ]]; then
        python3 "$HELPERS/state.py" add-deleted-pending "$FETCH_DIR/.update-state.json" \
          --rel "$REL" --reason "user marked at new_file prompt"
      fi
      ;;
    deletion-in-template)
      REL="${parts[2]}"; LOCAL_MOD="${parts[3]:-false}"
      DECISION="remove"
      if [[ "$LOCAL_MOD" == "true" ]]; then
        if [[ $NON_INTERACTIVE -eq 1 ]]; then
          DECISION="preserve-local"
        else
          echo "Locally-modified file removed in template: $REL"
          echo "  [r]emove  [p]reserve-local (default: preserve-local)"
          read -r -p "> " ans
          if [[ "$ans" =~ ^r ]]; then
            DECISION="remove"
          else
            DECISION="preserve-local"
          fi
        fi
      fi
      python3 "$HELPERS/sync.py" apply-deletion \
        --user-tree "$REPO_ROOT" --rel "$REL" --decision "$DECISION"
      python3 "$HELPERS/state.py" add-deletion-decision "$FETCH_DIR/.update-state.json" \
        --rel "$REL" --decision "$DECISION"
      ;;
    template_only)
      # Spec: same as overwrite but suppressed from human summary.
      python3 "$HELPERS/sync.py" apply-overwrite \
        --new-tree "$FETCH_DIR" --user-tree "$REPO_ROOT" --rel "${parts[2]}"
      ;;
    preserve)
      ;;  # no-op (user wins)
    *)
      ;;
  esac
done <<< "$PLAN_OUT"

# Halt if conflict markers in any merged path.
if [[ ${#THREE_PATHS_FOR_MARKER_CHECK[@]} -gt 0 ]]; then
  if ! python3 "$HELPERS/sync.py" has-conflict-markers --tree "$REPO_ROOT" \
       --paths "${THREE_PATHS_FOR_MARKER_CHECK[@]}"; then
    echo "halt: conflict markers present in merged file(s). Resolve and re-run with --continue." >&2
    exit 1
  fi
fi

# Stage and commit.
git add -A
NEW_VERSION=$(awk -F= '$1=="template_version"{print $2}' <(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST"))
git commit -q -m "chore(template): sync to $NEW_VERSION ($SHORT_NEW)"
SYNC_SHA=$(git rev-parse HEAD)

python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-a --status committed
python3 "$HELPERS/state.py" set-last-completed "$FETCH_DIR/.update-state.json" "$SYNC_SHA"

echo "info: Commit A complete ($SYNC_SHA)"
fi  # end Commit A (skipped on resume past)

# === Phase 3 — Commit B: migrations ===
if should_skip_phase commit-b; then
  echo "info: resume — skipping Commit B"
else
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-b --status started

OLD_VERSION=$(python3 -c "import json; print(json.load(open('$PJ'))['version'])")
NEW_VERSION=$(awk -F= '$1=="template_version"{print $2}' <(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST"))

# Already-applied IDs.
APPLIED_IDS=$(python3 -c "
import json
d=json.load(open('$PJ'))
for m in d.get('applied_migrations', []):
    print(m.get('id', ''))
")

# Iterate migrations in numeric order (by filename).
MIG_LIST=$(ls "$FETCH_DIR/migrations/"*.sh "$FETCH_DIR/migrations/"*.prompt.md 2>/dev/null | sort || true)
for MIG in $MIG_LIST; do
  [[ -e "$MIG" ]] || continue
  FNAME=$(basename "$MIG")
  [[ "$FNAME" == "README.md" || "$FNAME" == ".gitkeep" ]] && continue
  [[ "$FNAME" == schema-*.sh ]] && continue   # handled in Phase 1.5

  if [[ "$FNAME" == *.sh ]]; then
    MID="${FNAME%.sh}"
  else
    MID="${FNAME%.prompt.md}"
  fi

  # Skip if already applied.
  if echo "$APPLIED_IDS" | grep -qx "$MID"; then continue; fi
  if [[ -n "$SKIP_MIGRATION" && "$SKIP_MIGRATION" == "$MID" ]]; then
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "$MID" --status skipped --reason "user --skip-migration"
    continue
  fi

  if [[ "$FNAME" == *.sh ]]; then
    if ! python3 "$HELPERS/migration.py" run "$MIG" \
         --repo-root "$REPO_ROOT" --old-version "$OLD_VERSION" --new-version "$NEW_VERSION"; then
      echo "halt: migration $MID failed. Fix or run with --skip-migration $MID then --continue." >&2
      exit 1
    fi
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "$MID" --status applied
  else
    # LLM prompt: stage.
    PEND_DIR="$REPO_ROOT/.awiki/pending-prompts"
    STAGE_ARGS=(--user-tree "$REPO_ROOT" --pending-dir "$PEND_DIR")
    [[ -f "$REPO_ROOT/.gitattributes" ]] && STAGE_ARGS+=(--gitattributes "$REPO_ROOT/.gitattributes")
    if ! python3 "$HELPERS/migration.py" stage-prompt "$MIG" "${STAGE_ARGS[@]}"; then
      echo "halt: prompt staging failed for $MID" >&2
      exit 1
    fi
    # If risk: high under --non-interactive → record skipped instead of staged.
    RISK=$(awk -F: '/^risk:/{gsub(/[ "'\'']/, "", $2); print $2; exit}' "$MIG" || echo medium)
    if [[ "$RISK" == "high" && $NON_INTERACTIVE -eq 1 ]]; then
      rm -f "$PEND_DIR/$(basename "$MIG")"
      python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
        --id "$MID" --status skipped --reason "non-interactive high-risk"
      continue
    fi
    # Note: applied_migrations[] entry for LLM prompts is appended LATER by the agent;
    # state.applied_migrations_pending only tracks staged prompts so Commit D can list them.
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "$MID" --status applied --reason "staged-as-prompt"
  fi
done

# Stage and commit (only if there were changes).
git add -A
if git diff --cached --quiet; then
  echo "info: no migration changes to commit"
else
  git commit -q -m "chore(template): run migrations"
  COMMIT_B_SHA=$(git rev-parse HEAD)
  python3 "$HELPERS/state.py" set-last-completed "$FETCH_DIR/.update-state.json" "$COMMIT_B_SHA"
fi
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-b --status committed

echo "info: Commit B complete"
fi  # end Commit B (skipped on resume)

# === Phase 3 — Commit C: bootstrap steps ===
if should_skip_phase commit-c; then
  echo "info: resume — skipping Commit C"
else
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-c --status started

# Manifest's ordered_steps and dangerous IDs (NEW manifest).
ORDERED=$(bash "$SCRIPT_DIR/template-manifest.sh" bootstrap-ids "$NEW_MANIFEST")

# Existing applied steps in pin: emit "id status content_hash" lines.
EXISTING=$(bash "$SCRIPT_DIR/template-provenance.sh" list-steps "$PJ")

while IFS= read -r SID; do
  [[ -z "$SID" ]] && continue

  # Compute upstream content_hash from FETCH_DIR/BOOTSTRAP.md.
  NEW_HASH=$(python3 "$HELPERS/bootstrap_hash.py" hash "$FETCH_DIR/BOOTSTRAP.md" "$SID" 2>/dev/null || true)
  [[ -z "$NEW_HASH" ]] && continue   # step not present in new BOOTSTRAP.md

  # Look up existing record. Use awk -v to avoid shell interpolation hazards.
  EXISTING_LINE=$(echo "$EXISTING" | awk -v id="$SID" '$1==id {print; exit}')
  EXISTING_STATUS=$(echo "$EXISTING_LINE" | awk '{print $2}')
  EXISTING_HASH=$(echo "$EXISTING_LINE" | awk '{print $3}')

  # Already applied with matching hash → skip.
  if [[ "$EXISTING_STATUS" == "applied" && "$EXISTING_HASH" == "$NEW_HASH" ]]; then
    continue
  fi

  # Classify (dangerous?).
  CLASS=$(python3 "$HELPERS/bootstrap_replay.py" classify \
    --bootstrap "$FETCH_DIR/BOOTSTRAP.md" --manifest "$NEW_MANIFEST" --id "$SID")
  if [[ "$CLASS" == "dangerous" ]]; then
    echo "info: skipping dangerous step $SID — re-run with --rerun-bootstrap-step"
    continue
  fi

  # Replay (or auto-decline under --non-interactive).
  if [[ $NON_INTERACTIVE -eq 1 ]]; then
    python3 "$HELPERS/state.py" add-bootstrap-pending "$FETCH_DIR/.update-state.json" \
      --id "$SID" --status skipped --reason "non-interactive default" --content-hash "$NEW_HASH"
    continue
  fi

  echo "Bootstrap step '$SID':"
  python3 "$HELPERS/bootstrap_replay.py" body --bootstrap "$FETCH_DIR/BOOTSTRAP.md" --id "$SID"
  read -r -p "Run this step now? [y/N] " ans
  if [[ "$ans" =~ ^[Yy] ]]; then
    BODY=$(python3 "$HELPERS/bootstrap_replay.py" body --bootstrap "$FETCH_DIR/BOOTSTRAP.md" --id "$SID")
    BODY_FILE=$(mktemp)
    printf '%s' "$BODY" > "$BODY_FILE"
    # Replay with stripped env (matching migration trust model).
    if env -i \
        PATH="$PATH" HOME="$HOME" LANG="${LANG:-}" LC_ALL="${LC_ALL:-}" \
        AWIKI_REPO_ROOT="$REPO_ROOT" \
        bash "$BODY_FILE"; then
      python3 "$HELPERS/state.py" add-bootstrap-pending "$FETCH_DIR/.update-state.json" \
        --id "$SID" --status applied --content-hash "$NEW_HASH"
    else
      python3 "$HELPERS/state.py" add-bootstrap-pending "$FETCH_DIR/.update-state.json" \
        --id "$SID" --status skipped --reason "user replay failed" --content-hash "$NEW_HASH"
    fi
    rm -f "$BODY_FILE"
  else
    python3 "$HELPERS/state.py" add-bootstrap-pending "$FETCH_DIR/.update-state.json" \
      --id "$SID" --status skipped --reason "user declined" --content-hash "$NEW_HASH"
  fi
done <<< "$ORDERED"

git add -A
if git diff --cached --quiet; then
  echo "info: no bootstrap-step changes to commit"
else
  git commit -q -m "chore(template): bootstrap steps"
  COMMIT_C_SHA=$(git rev-parse HEAD)
  python3 "$HELPERS/state.py" set-last-completed "$FETCH_DIR/.update-state.json" "$COMMIT_C_SHA"
fi
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-c --status committed

echo "info: Commit C complete"
fi  # end Commit C

# === Phase 3 — Commit D: provenance ===
if should_skip_phase commit-d; then
  echo "info: resume — skipping Commit D"
else
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-d --status started

# Apply pending updates from state file → template.json.
NEW_VERSION=$(awk -F= '$1=="template_version"{print $2}' <(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST"))
NEW_REF="${REF:-main}"

bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" commit "$COMMIT_NEW" >/dev/null
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" version "$NEW_VERSION" >/dev/null
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" ref "$NEW_REF" >/dev/null
if [[ $PERSIST_SOURCE -eq 1 ]]; then
  bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" repo "$RESOLVED_SOURCE" >/dev/null
fi

# Append pending entries from state file → template.json. Use literal heredoc + os.environ to
# avoid shell-injection from user-controlled state contents (migration ids, reasons, paths).
AWIKI_PJ="$PJ" AWIKI_STATE="$FETCH_DIR/.update-state.json" python3 - <<'PY'
import json, os
pj_path = os.environ["AWIKI_PJ"]
state_path = os.environ["AWIKI_STATE"]
with open(pj_path) as f:
    d = json.load(f)
with open(state_path) as f:
    s = json.load(f)
d.setdefault("applied_migrations", []).extend(s.get("applied_migrations_pending", []))
d.setdefault("bootstrap_steps_done", []).extend(s.get("bootstrap_steps_pending", []))
d.setdefault("deleted", []).extend(s.get("deleted_pending", []))
with open(pj_path, "w") as f:
    json.dump(d, f, indent=2)
    f.write("\n")
PY

# Move _fetch -> template-cache/<commit_new>/.
NEW_CACHE_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_NEW"
[[ -d "$NEW_CACHE_DIR" ]] && rm -rf "$NEW_CACHE_DIR"
# Strip ephemeral artefacts to keep cache lean.
rm -rf "$FETCH_DIR/.git"
rm -rf "$FETCH_DIR/_scratch-merge"
rm -f "$FETCH_DIR/.update-state.json"
mv "$FETCH_DIR" "$NEW_CACHE_DIR"

# Rotate cache: keep current + immediate previous.
python3 "$HELPERS/cache_rotate.py" --cache-dir "$REPO_ROOT/.awiki/template-cache" --current "$COMMIT_NEW"

# Stage + commit template.json (the only file touched in Commit D).
git add "$PJ"
git commit -q -m "chore(template): pin to $NEW_VERSION"
echo "info: Commit D complete; pinned to $COMMIT_NEW ($NEW_VERSION)"

cat <<EOF
Update branch ready: $BRANCH_NAME

Review with:   git diff main
Merge with:    git switch main && git merge --no-ff $BRANCH_NAME
Pending LLM migrations: .awiki/pending-prompts/
EOF
fi  # end Commit D

exit 0
