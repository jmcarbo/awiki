# Phase 07 — Phase 3 Commit C (Bootstrap Steps) + Commit D (Provenance)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement (a) **Commit C** — replay new + content-changed bootstrap steps (skipping `bootstrap.dangerous.ids`); (b) **Commit D** — single `template.json` write, cache rotation (keep current + previous), state-file deletion.

**Architecture:** New helper `scripts/_template_helpers/bootstrap_replay.py` extracts step body via `bootstrap_hash.body`, runs the commands inline, records result in state file. Commit D reads state file, applies all pending writes to `template.json`, rotates `.awiki/template-cache/`, deletes state file.

**Spec sections:** `Update flow / Phase 3 — Commit C`, `Update flow / Phase 3 — Commit D`, `Bootstrap integration / Content-hash tracking / Dangerous steps`.

---

## File structure

**Created:**
- `scripts/_template_helpers/bootstrap_replay.py`
- `scripts/_template_helpers/cache_rotate.py`
- `tests/template-update-bootstrap-step.bats`
- `tests/template-update-commit-d.bats`

**Modified:** `scripts/template-update.sh` (Commit C + Commit D wiring).

**Depends on:** Phase 06.

---

## Task 1: Bootstrap replay helper

**Files:**
- Create: `scripts/_template_helpers/bootstrap_replay.py`
- Create: `tests/template-update-bootstrap-step.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-bootstrap-step.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "bootstrap-replay: emits dangerous-skipped for dangerous IDs" {
  cd "$TMP"
  cat > BOOTSTRAP.md <<'EOF'
### Step 1
<!-- bootstrap-step: theme -->
mkdir -p themes/foo
EOF
  cat > template.manifest.toml <<'EOF'
schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite=[]
preserve=[]
three_way=[]
attributes_merge=[]
template_only=[]
[new_file_default]
strategy="prompt"
[bootstrap]
ordered_steps = ["theme"]
[bootstrap.dangerous]
ids = ["theme"]
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_replay.py" classify \
    --bootstrap BOOTSTRAP.md --manifest template.manifest.toml --id theme
  [ "$status" -eq 0 ]
  [ "$output" = "dangerous" ]
}

@test "bootstrap-replay: classifies new step" {
  cd "$TMP"
  cat > BOOTSTRAP.md <<'EOF'
### Step 1
<!-- bootstrap-step: domain -->
ask domain
EOF
  cat > template.manifest.toml <<'EOF'
schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite=[]
preserve=[]
three_way=[]
attributes_merge=[]
template_only=[]
[new_file_default]
strategy="prompt"
[bootstrap]
ordered_steps = ["domain"]
[bootstrap.dangerous]
ids = []
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_replay.py" classify \
    --bootstrap BOOTSTRAP.md --manifest template.manifest.toml --id domain
  [ "$status" -eq 0 ]
  [ "$output" = "ok" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/bootstrap_replay.py`:

```python
#!/usr/bin/env python3
"""Bootstrap step replay helper.

Subcommands:
  classify --bootstrap <md> --manifest <toml> --id <id>
    -> prints "ok" | "dangerous" | "missing"
  body --bootstrap <md> --id <id>
    -> prints raw body (for orchestrator to surface to user)
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import bootstrap_hash  # noqa: E402
import manifest_parse  # noqa: E402


def cmd_classify(args: argparse.Namespace) -> int:
    md = args.bootstrap.read_text(encoding="utf-8")
    bodies = bootstrap_hash.parse(md)
    if args.id not in bodies:
        print("missing")
        return 1
    m = manifest_parse.load(args.manifest)
    dangerous = m.get("bootstrap", {}).get("dangerous", {}).get("ids", [])
    if args.id in dangerous:
        print("dangerous")
        return 0
    print("ok")
    return 0


def cmd_body(args: argparse.Namespace) -> int:
    md = args.bootstrap.read_text(encoding="utf-8")
    bodies = bootstrap_hash.parse(md)
    if args.id not in bodies:
        print(f"missing step: {args.id}", file=sys.stderr)
        return 1
    sys.stdout.write(bodies[args.id])
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_c = sub.add_parser("classify")
    p_c.add_argument("--bootstrap", required=True, type=Path)
    p_c.add_argument("--manifest", required=True, type=Path)
    p_c.add_argument("--id", required=True)
    p_b = sub.add_parser("body")
    p_b.add_argument("--bootstrap", required=True, type=Path)
    p_b.add_argument("--id", required=True)
    args = parser.parse_args()
    return {"classify": cmd_classify, "body": cmd_body}[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/bootstrap_replay.py tests/template-update-bootstrap-step.bats
git commit -m "feat: bootstrap_replay.py classify + body"
```

---

## Task 2: Wire Commit C in orchestrator

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-bootstrap-step.bats`

- [ ] **Step 1: Failing test for end-to-end Commit C**

Append to `tests/template-update-bootstrap-step.bats`:

```bash
@test "template-update --apply: Commit C records new bootstrap steps as pending" {
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  # No new steps in v1 (manifest unchanged); but content_hash for steps recorded.
  # Add a content-changed step for testing.
  TMP_V=$(mktemp -d)
  cp -R "$V1/." "$TMP_V/"
  sed -i.bak 's/Ask user: which domain/Ask user (UPDATED): which domain/' "$TMP_V/BOOTSTRAP.md" && rm "$TMP_V/BOOTSTRAP.md.bak"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$TMP_V" --accept-source-change --apply --non-interactive
  PENDING=$(python3 "$REPO_ROOT/scripts/_template_helpers/state.py" get \
    .awiki/template-cache/_fetch/.update-state.json bootstrap_steps_pending)
  echo "$PENDING" | grep -q '"id": "domain"'
  rm -rf "$TMP_V"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Commit C in `template-update.sh`**

Replace the trailing `echo "info: Commits C/D not yet implemented"` and `exit 0` with:

```bash
# === Phase 3 — Commit C: bootstrap steps ===
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-c --status started

# Manifest's ordered_steps and dangerous IDs (NEW manifest).
ORDERED=$(bash "$SCRIPT_DIR/template-manifest.sh" bootstrap-ids "$NEW_MANIFEST")

# Existing applied steps in pin.
EXISTING=$(python3 -c "
import json
d=json.load(open('$PJ'))
for s in d.get('bootstrap_steps_done', []):
    print(s.get('id', ''), s.get('status', ''), s.get('content_hash', ''))
")

while IFS= read -r SID; do
  [[ -z "$SID" ]] && continue

  # Compute upstream content_hash from FETCH_DIR/BOOTSTRAP.md.
  NEW_HASH=$(python3 "$HELPERS/bootstrap_hash.py" hash "$FETCH_DIR/BOOTSTRAP.md" "$SID" 2>/dev/null || true)
  [[ -z "$NEW_HASH" ]] && continue   # step not present in new BOOTSTRAP.md

  # Look up existing record.
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
    # Replay body as bash. Spec: extract body and execute. For v1 keep simple: write to tmpfile + bash.
    BODY=$(python3 "$HELPERS/bootstrap_replay.py" body --bootstrap "$FETCH_DIR/BOOTSTRAP.md" --id "$SID")
    BODY_FILE=$(mktemp)
    echo "$BODY" > "$BODY_FILE"
    if bash "$BODY_FILE"; then
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
```

(Then continue with Commit D below — same exit point.)

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-bootstrap-step.bats
git commit -m "feat: Phase 3 Commit C (bootstrap step replay + content_hash + dangerous skip)"
```

---

## Task 3: Cache rotation helper

**Files:**
- Create: `scripts/_template_helpers/cache_rotate.py`
- Create: `tests/template-update-commit-d.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-commit-d.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "cache-rotate: keeps current + previous; deletes older" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa .awiki/template-cache/bbb .awiki/template-cache/ccc
  # Make ccc oldest, aaa newest by mtime.
  touch -t 202001011200 .awiki/template-cache/ccc
  touch -t 202101011200 .awiki/template-cache/bbb
  touch -t 202201011200 .awiki/template-cache/aaa
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/aaa ]
  [ -d .awiki/template-cache/bbb ]
  [ ! -d .awiki/template-cache/ccc ]
}

@test "cache-rotate: ignores non-sha entries (_fetch, _check-stamp)" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa .awiki/template-cache/_fetch
  touch .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ -d .awiki/template-cache/_fetch ]
  [ -f .awiki/template-cache/_check-stamp ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/cache_rotate.py`:

```python
#!/usr/bin/env python3
"""Cache rotation: keep <current> + immediate previous (by mtime), delete older."""
from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--cache-dir", required=True, type=Path)
    p.add_argument("--current", required=True)
    args = p.parse_args()

    if not args.cache_dir.is_dir():
        return 0
    sha_dirs = []
    for entry in args.cache_dir.iterdir():
        if entry.name.startswith("_"):
            continue
        if not entry.is_dir():
            continue
        sha_dirs.append(entry)

    # Sort by mtime descending.
    sha_dirs.sort(key=lambda p: p.stat().st_mtime, reverse=True)

    keep = {args.current}
    # Add up to 1 most recent that isn't current.
    for d in sha_dirs:
        if d.name not in keep and len(keep) < 2:
            keep.add(d.name)
            break

    for d in sha_dirs:
        if d.name not in keep:
            shutil.rmtree(d, ignore_errors=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/cache_rotate.py tests/template-update-commit-d.bats
git commit -m "feat: cache_rotate.py keeps current + previous"
```

---

## Task 4: Wire Commit D in orchestrator

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-commit-d.bats`

- [ ] **Step 1: Failing test for end-to-end Commit D**

Append to `tests/template-update-commit-d.bats`:

```bash
@test "template-update --apply: Commit D writes template.json once + rotates cache" {
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT_OLD=$(git rev-parse HEAD)
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --non-interactive
  [ "$status" -eq 0 ]
  # Pin advanced.
  PIN_VERSION=$(python3 -c "import json; print(json.load(open('.awiki/template.json'))['version'])")
  [ "$PIN_VERSION" = "0.2.0" ]
  # applied_migrations updated.
  python3 -c "import json; d=json.load(open('.awiki/template.json')); ids=[m['id'] for m in d['applied_migrations']]; assert '0001-add-tagline' in ids"
  # State file removed.
  [ ! -f .awiki/template-cache/_fetch/.update-state.json ]
  # Cache rotated: only current pin should exist (and old one).
  ls .awiki/template-cache/ | grep -v "^_" | wc -l | grep -qE '^[1-2]$'
  # Last commit is provenance pin.
  git log --format=%s -1 | grep -q "pin to"
}

@test "template-update --apply --persist-source: updates repo, original_repo unchanged" {
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --persist-source --non-interactive >/dev/null
  REPO_URL=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json repo)
  [ "$REPO_URL" = "$V1" ]
  ORIG=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json original_repo)
  [ "$ORIG" = "$REPO_ROOT/tests/fixtures/template-update/v0" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Commit D**

In `scripts/template-update.sh`, after the Commit C block, add:

```bash
# === Phase 3 — Commit D: provenance ===
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

# Append pending migrations + bootstrap steps from state file.
PENDING_MIGS=$(python3 "$HELPERS/state.py" get "$FETCH_DIR/.update-state.json" applied_migrations_pending)
PENDING_STEPS=$(python3 "$HELPERS/state.py" get "$FETCH_DIR/.update-state.json" bootstrap_steps_pending)
python3 -c "
import json
pj_path = '$PJ'
d = json.load(open(pj_path))
d.setdefault('applied_migrations', []).extend(json.loads('''$PENDING_MIGS'''))
d.setdefault('bootstrap_steps_done', []).extend(json.loads('''$PENDING_STEPS'''))
json.dump(d, open(pj_path, 'w'), indent=2)
open(pj_path, 'a').write('\n')
"

# Move _fetch -> template-cache/<commit_new>/.
NEW_CACHE_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_NEW"
[[ -d "$NEW_CACHE_DIR" ]] && rm -rf "$NEW_CACHE_DIR"
# Strip .git to keep cache lean.
rm -rf "$FETCH_DIR/.git"
rm -rf "$FETCH_DIR/_scratch-merge"
rm -f "$FETCH_DIR/.update-state.json"
mv "$FETCH_DIR" "$NEW_CACHE_DIR"

# Rotate cache.
python3 "$HELPERS/cache_rotate.py" --cache-dir "$REPO_ROOT/.awiki/template-cache" --current "$COMMIT_NEW"

# Stage + commit.
git add "$PJ" .gitignore 2>/dev/null || true
git add "$PJ"
git commit -q -m "chore(template): pin to $NEW_VERSION"
echo "info: Commit D complete; pinned to $COMMIT_NEW ($NEW_VERSION)"

cat <<EOF
Update branch ready: $BRANCH_NAME

Review with:   git diff main
Merge with:    git switch main && git merge --no-ff $BRANCH_NAME
Pending LLM migrations: .awiki/pending-prompts/
EOF

exit 0
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-commit-d.bats
git commit -m "feat: Phase 3 Commit D (single template.json write + cache rotation + final summary)"
```

---

## Task 5: CHANGELOG + verify

- [ ] **Step 1: Run all Phase 07 tests**

```bash
bats tests/template-update-bootstrap-step.bats tests/template-update-commit-d.bats
```

- [ ] **Step 2: Append CHANGELOG**

```markdown
### Added
- `template-update.sh` Phase 3 Commit C (bootstrap step replay with content_hash + dangerous-step skip) and Commit D (single `template.json` write + cache rotation + state-file deletion + summary print) (Phase 07).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 07 commits C + D"
```

---

## Phase 07 — Definition of done

- [ ] `bootstrap_replay.py classify` returns `ok | dangerous | missing`.
- [ ] Commit C re-runs new + content-changed steps; auto-skips dangerous.
- [ ] Commit C records each step in state file's `bootstrap_steps_pending` (id, status, content_hash).
- [ ] Commit D applies all pending writes to `template.json` exactly once.
- [ ] Commit D rotates cache to keep current + previous SHA dirs only.
- [ ] State file deleted at end of Commit D.
- [ ] `--persist-source` updates `repo` only; `original_repo` immutable.
- [ ] CHANGELOG entry added.

Phase 08 implements recovery flows (`--continue`, `--abort`, `--re-pin`, `--rerun-bootstrap-step`, `--gc`, `--non-interactive` already partially in).
