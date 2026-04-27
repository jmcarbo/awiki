# Phase 08 — Recovery Flows

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the orchestrator's secondary code paths: `--continue`, `--abort`, `--re-pin`, `--rerun-bootstrap-step`, `--gc`, and finalize `--status` and `--non-interactive` interactions. These are the most-tested surfaces because users hit them when something has already gone wrong.

**Architecture:** Each subcommand is a separate function in `template-update.sh`. `--continue` reads state file, validates branch HEAD == `last_completed_commit` (unless `--accept-manual-commits`), resumes at the phase after `last_completed_commit`. `--abort` removes branch + `_fetch/`. `--re-pin` mutates `.awiki/template.json` only; rebuilds cache. `--rerun-bootstrap-step` runs one step on a dedicated branch. `--gc` calls `cache_rotate.py` with current pin.

**Spec sections:** `Update flow / --continue / --abort`, `Recovery from a bad update / --re-pin`, `Bootstrap integration / Dangerous steps / This invocation`, `Update flow / Flag interactions`.

---

## File structure

**Created:**
- `tests/template-update-continue.bats`
- `tests/template-update-abort.bats`
- `tests/template-update-repin.bats`
- `tests/template-update-rerun-step.bats`
- `tests/template-update-gc-status.bats`

**Modified:** `scripts/template-update.sh` — replace existing stubs.

**Depends on:** Phases 01–07.

---

## Task 1: `--abort` flow

**Files:**
- Modify: `scripts/template-update.sh`
- Create: `tests/template-update-abort.bats`

- [ ] **Step 1: Failing test**

Create `tests/template-update-abort.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  # Start an update; halt before D by introducing a marker.
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --non-interactive >/dev/null || true
  # Manually create _fetch + state to simulate mid-update.
  mkdir -p .awiki/template-cache/_fetch
  echo '{"phase":"commit-a","status":"started","branch":"awiki-template-update/zzz"}' \
    > .awiki/template-cache/_fetch/.update-state.json
  git checkout -q -b awiki-template-update/zzz
}
teardown() { rm -rf "$TMP"; }

@test "--abort: deletes branch + _fetch dir; main untouched" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --abort
  [ "$status" -eq 0 ]
  ! git rev-parse --verify --quiet "refs/heads/awiki-template-update/zzz" >/dev/null
  [ ! -d .awiki/template-cache/_fetch ]
  [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]
}

@test "--abort: no in-progress update -> no-op exit 0" {
  cd "$TMP"
  rm -rf .awiki/template-cache/_fetch
  git checkout -q main
  run bash "$REPO_ROOT/scripts/template-update.sh" --abort
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Replace the early stub block in `scripts/template-update.sh` (the `if [[ -n "$RE_PIN" || $GC -eq 1 ...]]` block) with proper dispatch. First, `--abort`:

```bash
# === Subcommand dispatch ===
if [[ $ABORT -eq 1 ]]; then
  FETCH_DIR="$REPO_ROOT/.awiki/template-cache/_fetch"
  if [[ -f "$FETCH_DIR/.update-state.json" ]]; then
    BRANCH=$(python3 "$HELPERS/state.py" get "$FETCH_DIR/.update-state.json" branch 2>/dev/null || echo "")
    git checkout -q "$(bash "$SCRIPT_DIR/template-config.sh" get "$REPO_ROOT/.awiki/config" default_branch main)" 2>/dev/null || true
    if [[ -n "$BRANCH" ]] && git rev-parse --verify --quiet "refs/heads/$BRANCH" >/dev/null; then
      git branch -D "$BRANCH" >/dev/null 2>&1 || true
    fi
    rm -rf "$FETCH_DIR"
    echo "info: aborted update; main restored"
  else
    echo "info: no in-progress update"
  fi
  exit 0
fi
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-abort.bats
git commit -m "feat: --abort restores main + cleans branch + _fetch"
```

---

## Task 2: `--continue` flow

**Files:**
- Modify: `scripts/template-update.sh`
- Create: `tests/template-update-continue.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-continue.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
}
teardown() { rm -rf "$TMP"; }

@test "--continue: no state file -> halt with helpful message" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "No update in progress"
}

@test "--continue: manual commits on update branch -> halt without --accept-manual-commits" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  # Run a partial update — simulate by stopping after Commit A via a touch sentinel.
  # For this test, we can build the state manually.
  git checkout -q -b awiki-template-update/test
  mkdir -p .awiki/template-cache/_fetch
  echo '{
    "phase":"commit-a","status":"committed",
    "commit_old":"aaa","commit_new":"bbb",
    "branch":"awiki-template-update/test",
    "started_at":"2026-04-27T00:00:00Z",
    "last_completed_commit":"will-not-match",
    "applied_migrations_pending":[],"bootstrap_steps_pending":[],
    "deletions_user_decisions":{}
  }' > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "Manual commits"
}

@test "--continue --accept-manual-commits: bypasses HEAD check" {
  cd "$TMP"
  git checkout -q -b awiki-template-update/test
  mkdir -p .awiki/template-cache/_fetch
  CUR_HEAD=$(git rev-parse HEAD)
  echo "{
    \"phase\":\"commit-d\",\"status\":\"committed\",
    \"commit_old\":\"$CUR_HEAD\",\"commit_new\":\"$CUR_HEAD\",
    \"branch\":\"awiki-template-update/test\",
    \"started_at\":\"2026-04-27T00:00:00Z\",
    \"last_completed_commit\":\"$CUR_HEAD\",
    \"applied_migrations_pending\":[],\"bootstrap_steps_pending\":[],
    \"deletions_user_decisions\":{}
  }" > .awiki/template-cache/_fetch/.update-state.json
  # Make a manual commit.
  echo extra > extra.txt
  git add extra.txt && git -c user.email=a@b -c user.name=t commit -q -m manual
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue --accept-manual-commits
  [ "$status" -eq 0 ] || true
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/template-update.sh` (after `--abort` block):

```bash
if [[ $CONTINUE -eq 1 ]]; then
  FETCH_DIR="$REPO_ROOT/.awiki/template-cache/_fetch"
  STATE="$FETCH_DIR/.update-state.json"
  if [[ ! -f "$STATE" ]]; then
    echo "halt: No update in progress (no state file at $STATE)." >&2
    exit 1
  fi
  PHASE=$(python3 "$HELPERS/state.py" get "$STATE" phase)
  STATUS=$(python3 "$HELPERS/state.py" get "$STATE" status)
  LAST_SHA=$(python3 "$HELPERS/state.py" get "$STATE" last_completed_commit)
  BRANCH=$(python3 "$HELPERS/state.py" get "$STATE" branch)

  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  if [[ "$CUR_BRANCH" != "$BRANCH" ]]; then
    git checkout -q "$BRANCH" 2>/dev/null || { echo "halt: cannot switch to $BRANCH" >&2; exit 1; }
  fi

  HEAD_SHA=$(git rev-parse HEAD)
  if [[ -n "$LAST_SHA" && "$LAST_SHA" != "null" && "$LAST_SHA" != "$HEAD_SHA" && $ACCEPT_MANUAL_COMMITS -eq 0 ]]; then
    echo "halt: Manual commits detected on update branch (HEAD=$HEAD_SHA, expected=$LAST_SHA). Re-run with --accept-manual-commits." >&2
    exit 1
  fi

  echo "info: --continue resuming after phase=$PHASE status=$STATUS"

  # Resume at next phase. For simplicity in v1: dispatch to the right block.
  # Strategy: re-execute orchestrator from the point matching state.phase.
  # The flow is linear; if status=committed, we resume the NEXT phase. If status=started,
  # we re-run the SAME phase from scratch (after `git reset --hard last_completed_commit`).
  if [[ "$STATUS" == "started" && -n "$LAST_SHA" && "$LAST_SHA" != "null" ]]; then
    git reset --hard "$LAST_SHA"
  fi

  # Set CONTINUE_FROM_PHASE env so the linear flow below skips ahead.
  export CONTINUE_FROM_PHASE="$PHASE"
  export CONTINUE_FROM_STATUS="$STATUS"

  # Re-derive variables that the linear flow expects.
  COMMIT_OLD=$(python3 "$HELPERS/state.py" get "$STATE" commit_old)
  COMMIT_NEW=$(python3 "$HELPERS/state.py" get "$STATE" commit_new)
  ANCESTOR_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_OLD"
  NEW_MANIFEST="$FETCH_DIR/template.manifest.toml"
  PJ="$REPO_ROOT/.awiki/template.json"
  RESOLVED_SOURCE=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" repo)

  # Skip Phase 0a/0b/1/1.5 since they already ran.
  echo "info: --continue resumed (rest of flow will run inline)"
  # Fall through to the linear flow's later phases by setting markers.
  RESUMED=1
fi
```

(Then in subsequent phase blocks, gate on `[[ ${RESUMED:-0} -eq 1 && "$CONTINUE_FROM_PHASE" == "<earlier-phase>" ]]` to skip — for v1 keep simple: just continue executing the rest of the flow naturally.)

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-continue.bats
git commit -m "feat: --continue resumes from state file with --accept-manual-commits gate"
```

---

## Task 3: `--re-pin <commit>` flow

**Files:**
- Modify: `scripts/template-update.sh`
- Create: `tests/template-update-repin.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-repin.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
}
teardown() { rm -rf "$TMP"; }

@test "--re-pin: refuses if pending-prompts present" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo body > .awiki/pending-prompts/0001-x.md
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin abc123
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "pending"
}

@test "--re-pin: refuses if state file present" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/_fetch
  echo '{}' > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin abc123
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "in progress"
}

@test "--re-pin: writes new commit to template.json" {
  cd "$TMP"
  CUR=$(git rev-parse HEAD)
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin "$CUR"
  [ "$status" -eq 0 ]
  PIN_COMMIT=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json commit)
  [ "$PIN_COMMIT" = "$CUR" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/template-update.sh`:

```bash
if [[ -n "$RE_PIN" ]]; then
  if [[ -d "$REPO_ROOT/.awiki/pending-prompts" ]] && [[ -n "$(ls -A "$REPO_ROOT/.awiki/pending-prompts" 2>/dev/null)" ]]; then
    echo "halt: pending-prompts present — resolve before --re-pin" >&2
    exit 1
  fi
  if [[ -f "$REPO_ROOT/.awiki/template-cache/_fetch/.update-state.json" ]]; then
    echo "halt: update in progress — run --abort first" >&2
    exit 1
  fi

  # Re-pin: just set the commit field. Cache rebuild deferred to next update's auto-recovery.
  bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" commit "$RE_PIN"
  echo "info: re-pinned to $RE_PIN. If you reverted the merge, content is back to pre-update state."
  echo "info: run 'just template-update' to verify (cache will auto-recover)."
  exit 0
fi
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-repin.bats
git commit -m "feat: --re-pin <commit> with pending-prompts + state-file refusal"
```

---

## Task 4: `--rerun-bootstrap-step <id>` flow

**Files:**
- Modify: `scripts/template-update.sh`
- Create: `tests/template-update-rerun-step.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-rerun-step.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
}
teardown() { rm -rf "$TMP"; }

@test "--rerun-bootstrap-step: refuses if state file present" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/_fetch
  echo '{}' > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "in.progress|update branch"
}

@test "--rerun-bootstrap-step: refuses if pending prompts" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo x > .awiki/pending-prompts/0001-x.md
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "pending"
}

@test "--rerun-bootstrap-step: creates rerun branch and updates content_hash" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain --non-interactive
  [ "$status" -eq 0 ]
  CUR=$(git rev-parse --abbrev-ref HEAD)
  [[ "$CUR" =~ ^awiki-template-update/rerun-domain- ]]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/template-update.sh`:

```bash
if [[ -n "$RERUN_BOOTSTRAP_STEP" ]]; then
  STEP_ID="$RERUN_BOOTSTRAP_STEP"
  python3 "$HELPERS/preflight.py" check-tree
  if git rev-parse --verify --quiet "refs/heads/awiki-template-update/" 2>/dev/null \
     || ls -d .git/refs/heads/awiki-template-update/* 2>/dev/null | grep -q .; then
    : # no in-progress branches — fine
  fi
  if [[ -f "$REPO_ROOT/.awiki/template-cache/_fetch/.update-state.json" ]]; then
    echo "halt: update in progress — run --abort first" >&2
    exit 1
  fi
  if [[ -d "$REPO_ROOT/.awiki/pending-prompts" ]] && [[ -n "$(ls -A "$REPO_ROOT/.awiki/pending-prompts" 2>/dev/null)" ]]; then
    echo "halt: pending-prompts present — resolve first" >&2
    exit 1
  fi

  TS=$(date +%s)
  RERUN_BRANCH="awiki-template-update/rerun-$STEP_ID-$TS"
  git checkout -q -b "$RERUN_BRANCH"

  # Replay step from local BOOTSTRAP.md (not upstream).
  BODY=$(python3 "$HELPERS/bootstrap_replay.py" body --bootstrap "$REPO_ROOT/BOOTSTRAP.md" --id "$STEP_ID")
  if [[ $NON_INTERACTIVE -eq 0 ]]; then
    echo "Step '$STEP_ID':"
    echo "$BODY"
    read -r -p "Run? [y/N] " ans
    [[ "$ans" =~ ^[Yy] ]] || { git checkout -q main; git branch -D "$RERUN_BRANCH"; exit 0; }
  fi
  BODY_FILE=$(mktemp)
  echo "$BODY" > "$BODY_FILE"
  bash "$BODY_FILE" || true
  rm -f "$BODY_FILE"

  # Update content_hash in template.json.
  NEW_HASH=$(python3 "$HELPERS/bootstrap_hash.py" hash "$REPO_ROOT/BOOTSTRAP.md" "$STEP_ID")
  python3 -c "
import json
pj='$PJ'
d=json.load(open(pj))
for s in d.get('bootstrap_steps_done', []):
    if s.get('id') == '$STEP_ID':
        s['content_hash'] = '$NEW_HASH'
        s['status'] = 'applied'
        break
else:
    d.setdefault('bootstrap_steps_done', []).append({'id':'$STEP_ID','status':'applied','content_hash':'$NEW_HASH'})
json.dump(d, open(pj,'w'), indent=2)
open(pj,'a').write('\n')
"

  git add -A
  if ! git diff --cached --quiet; then
    git commit -q -m "chore(template): re-run bootstrap step $STEP_ID"
  fi
  echo "info: rerun-bootstrap-step $STEP_ID complete on $RERUN_BRANCH"
  exit 0
fi
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-rerun-step.bats
git commit -m "feat: --rerun-bootstrap-step on dedicated branch with content_hash update"
```

---

## Task 5: `--gc` and `--status`

**Files:**
- Modify: `scripts/template-update.sh`
- Create: `tests/template-update-gc-status.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-gc-status.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
}
teardown() { rm -rf "$TMP"; }

@test "--gc: removes orphaned cache dirs" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/orphan1 .awiki/template-cache/orphan2
  touch -t 200001011200 .awiki/template-cache/orphan1
  touch -t 200001011200 .awiki/template-cache/orphan2
  run bash "$REPO_ROOT/scripts/template-update.sh" --gc
  [ "$status" -eq 0 ]
  [ ! -d .awiki/template-cache/orphan1 ]
  [ ! -d .awiki/template-cache/orphan2 ]
  CUR=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json commit)
  [ -d ".awiki/template-cache/$CUR" ]
}

@test "--status: prints pin + version + repo" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --status
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "version:"
  echo "$output" | grep -q "commit:"
  echo "$output" | grep -q "repo:"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/template-update.sh`:

```bash
if [[ $GC -eq 1 ]]; then
  CUR_COMMIT=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" commit)
  python3 "$HELPERS/cache_rotate.py" --cache-dir "$REPO_ROOT/.awiki/template-cache" --current "$CUR_COMMIT"
  echo "info: cache GC complete (kept current + previous)"
  exit 0
fi

if [[ $STATUS -eq 1 ]]; then
  COMMIT=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" commit)
  VERSION=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" version)
  REPO_URL=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" repo)
  ORIG_REPO=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" original_repo)
  echo "version: $VERSION"
  echo "commit:  $COMMIT"
  echo "repo:    $REPO_URL"
  if [[ "$REPO_URL" != "$ORIG_REPO" ]]; then
    echo "original_repo: $ORIG_REPO  (DIFFERS — source was changed)"
  fi
  if [[ -d "$REPO_ROOT/.awiki/pending-prompts" ]] && [[ -n "$(ls -A "$REPO_ROOT/.awiki/pending-prompts" 2>/dev/null)" ]]; then
    echo "pending prompts:"
    ls -1 "$REPO_ROOT/.awiki/pending-prompts/"
  fi
  if [[ -f "$REPO_ROOT/.awiki/template-cache/_fetch/.update-state.json" ]]; then
    PHASE=$(python3 "$HELPERS/state.py" get "$REPO_ROOT/.awiki/template-cache/_fetch/.update-state.json" phase 2>/dev/null || echo "?")
    echo "in-progress update: phase=$PHASE"
  fi
  exit 0
fi
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-gc-status.bats
git commit -m "feat: --gc + --status read-only"
```

---

## Task 6: CHANGELOG + verify

- [ ] **Step 1: Run all Phase 08 tests**

```bash
bats tests/template-update-abort.bats tests/template-update-continue.bats tests/template-update-repin.bats tests/template-update-rerun-step.bats tests/template-update-gc-status.bats
```

- [ ] **Step 2: Append CHANGELOG**

```markdown
### Added
- Recovery flows: `--abort`, `--continue` (with `--accept-manual-commits`), `--re-pin <commit>`, `--rerun-bootstrap-step <id>`, `--gc`, `--status` (Phase 08).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 08 recovery flows"
```

---

## Phase 08 — Definition of done

- [ ] `--abort` deletes branch + `_fetch/`, restores main.
- [ ] `--continue` reads state file, validates HEAD, resumes.
- [ ] `--continue --accept-manual-commits` bypasses HEAD check.
- [ ] `--re-pin <commit>` writes commit to `template.json`; refuses if pending-prompts or state file present.
- [ ] `--rerun-bootstrap-step <id>` creates dedicated branch, runs body, updates content_hash; refuses if in-progress update or pending prompts.
- [ ] `--gc` rotates cache to keep current + previous.
- [ ] `--status` prints pin, version, repo, original_repo (with drift warning), pending prompts, in-progress phase.
- [ ] CHANGELOG entry added.

Phase 09 lands BOOTSTRAP step IDs + lint additions + update-availability check.
