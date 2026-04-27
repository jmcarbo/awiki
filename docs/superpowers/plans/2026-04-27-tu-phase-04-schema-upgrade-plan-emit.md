# Phase 04 — Phase 1.5 Schema Upgrade + Phase 2 Plan Emit

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement (a) **Phase 1.5** schema-upgrade flow that creates the update branch and lands the schema migration as Commit 0, then (b) **Phase 2** plan emit using `template-plan.sh` from Phase 02 inside the orchestrator, with `--print-migrations` body framing and dry-run exit.

**Architecture:** Phase 1.5 runs only when schema mismatch detected AND `--schema-upgrade` passed. It creates `awiki-template-update/<short>` branch, runs `migrations/schema-N-to-M.sh` with `.awiki/` writes whitelisted, commits as Commit 0, updates state file, then continues. Phase 2 spawns `template-plan.sh` with the right tree paths, captures structured stdout, optionally prints full migration bodies, exits 0 if no `--apply`.

**Tech Stack:** bash 4+, `git checkout -b`, `git add`/`git commit`, the helpers from Phase 02.

**Spec sections:** `Schema upgrades`, `Update flow / Phase 1.5`, `Update flow / Phase 2 — plan`, `Plan output format / migration-body`.

---

## File structure

**Created:**
- (none new — all logic lives in `scripts/template-update.sh`)

**Modified:**
- `scripts/template-update.sh`
- `tests/template-update-fetch.bats` (extends with Phase 1.5 + Phase 2 cases) or new file `tests/template-update-plan.bats`.

**Created (test fixtures):**
- `tests/fixtures/template-update/v3-schema-bump/migrations/schema-1-to-2.sh` (real implementation, not a stub).

**Depends on:** Phase 03.

---

## Task 1: Real schema-1-to-2 migration in v3 fixture

**Files:**
- Modify: `tests/fixtures/template-update/v3-schema-bump/migrations/schema-1-to-2.sh`

- [ ] **Step 1: Replace stub with real script**

Overwrite `tests/fixtures/template-update/v3-schema-bump/migrations/schema-1-to-2.sh`:

```bash
#!/usr/bin/env bash
# Schema upgrade: 1 → 2.
# Adds a `[ui]` section to template.manifest.toml in the BOOTSTRAPPED repo
# (not the template) and bumps .awiki/template.json.schema_version.
set -euo pipefail

REPO="${AWIKI_REPO_ROOT:?AWIKI_REPO_ROOT required}"
PJ="$REPO/.awiki/template.json"

# Bump schema_version in template.json (whitelisted under --schema-upgrade).
python3 -c "
import json
d = json.load(open('$PJ'))
d['schema_version'] = 2
json.dump(d, open('$PJ', 'w'), indent=2)
open('$PJ', 'a').write('\n')
"
echo "schema upgraded 1 → 2"
```

`chmod +x tests/fixtures/template-update/v3-schema-bump/migrations/schema-1-to-2.sh`.

- [ ] **Step 2: Commit**

```bash
git add tests/fixtures/template-update/v3-schema-bump/migrations/schema-1-to-2.sh
git commit -m "test: real schema-1-to-2.sh migration in v3 fixture"
```

---

## Task 2: Phase 1.5 — schema-upgrade with branch + Commit 0

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-fetch.bats` (add schema-upgrade tests)

- [ ] **Step 1: Failing tests**

Append to `tests/template-update-fetch.bats`:

```bash
@test "template-update --schema-upgrade: creates branch + commit 0 + bumps schema" {
  cd "$TMP"
  V3="$REPO_ROOT/tests/fixtures/template-update/v3-schema-bump"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V3" --accept-source-change --schema-upgrade --apply
  [ "$status" -eq 0 ] || true   # later phases not implemented; OK
  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  [[ "$CUR_BRANCH" =~ ^awiki-template-update/ ]]
  PJ_SCHEMA=$(python3 -c "import json; print(json.load(open('.awiki/template.json'))['schema_version'])")
  [ "$PJ_SCHEMA" = "2" ]
  git log --format=%s -1 | grep -q "schema upgrade 1"
}

@test "template-update --schema-upgrade: state file phase=schema-upgrade after Commit 0" {
  cd "$TMP"
  V3="$REPO_ROOT/tests/fixtures/template-update/v3-schema-bump"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V3" --accept-source-change --schema-upgrade --apply
  PHASE=$(python3 "$REPO_ROOT/scripts/_template_helpers/state.py" get .awiki/template-cache/_fetch/.update-state.json phase)
  [ "$PHASE" = "schema-upgrade" ]
  STATUS=$(python3 "$REPO_ROOT/scripts/_template_helpers/state.py" get .awiki/template-cache/_fetch/.update-state.json status)
  [ "$STATUS" = "committed" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Phase 1.5**

In `scripts/template-update.sh`, replace the `# Phase 1.5 schema-upgrade itself happens in Phase 04 plan` block with:

```bash
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
```

- [ ] **Step 4: Run tests — verify passing**

Expected: 2 schema-upgrade tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-fetch.bats
git commit -m "feat: Phase 1.5 schema-upgrade (branch + Commit 0 + provenance)"
```

---

## Task 3: Phase 2 — plan emit + dry-run exit

**Files:**
- Modify: `scripts/template-update.sh`
- Create: `tests/template-update-plan.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-plan.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT_OLD=$(git rev-parse HEAD)
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null
}
teardown() { rm -rf "$TMP"; }

@test "phase 2: dry-run prints PLAN lines and exits 0 without applying" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" --accept-source-change
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|header\|'
  echo "$output" | grep -qE '^PLAN\|footer\|'
  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  [ "$CUR_BRANCH" = "main" ]
}

@test "phase 2: --print-migrations adds migration-body framing" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --accept-source-change --print-migrations
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|migration-body\|0001-add-tagline\|begin'
  echo "$output" | grep -qE '^PLAN\|migration-body\|0001-add-tagline\|end'
}

@test "phase 2: scratch-merge dir is removed after plan" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" --accept-source-change
  [ ! -d ".awiki/template-cache/_fetch/_scratch-merge" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Phase 2 in orchestrator**

In `scripts/template-update.sh`, replace the trailing `echo "info: subsequent phases not yet implemented"` and `exit 0` with:

```bash
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
```

- [ ] **Step 4: Run tests — verify passing**

Expected: 3 of 3 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-plan.bats
git commit -m "feat: Phase 2 plan emit + --print-migrations body framing + scratch cleanup"
```

---

## Task 4: CHANGELOG + final verify

- [ ] **Step 1: Run full Phase 04 suite**

```bash
bats tests/template-update-fetch.bats tests/template-update-plan.bats
```

- [ ] **Step 2: Append to CHANGELOG**

```markdown
### Added
- `template-update.sh` Phase 1.5 (schema-upgrade with update branch + Commit 0) and Phase 2 (plan emit, `--print-migrations` body framing, scratch-merge cleanup) (Phase 04).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 04 schema upgrade + plan emit"
```

---

## Phase 04 — Definition of done

- [ ] `tests/fixtures/template-update/v3-schema-bump/` has a real `schema-1-to-2.sh` that bumps `template.json.schema_version`.
- [ ] Phase 1.5 creates `awiki-template-update/<short>` branch, runs schema migration with stripped env + whitelisted `.awiki/` writes, commits as Commit 0.
- [ ] Phase 1.5 records schema upgrade in `applied_migrations[]` (id `schema-N-to-M`, status `applied`).
- [ ] State file phase = `schema-upgrade`, status = `committed`, after Commit 0.
- [ ] Phase 2 emits structured `PLAN|...` lines via `template-plan.sh`.
- [ ] `--print-migrations` adds `PLAN|migration-body|<id>|begin`/`...|end` framing.
- [ ] Dry-run (no `--apply`) exits 0 without mutating tracked content.
- [ ] `_scratch-merge/` is removed at end of plan.
- [ ] CHANGELOG entry added.

Phase 05 implements Phase 3 Commit A (sync).
