# Phase 05 — Phase 3 Commit A (Sync)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 3 Commit A: walk the plan, apply per-strategy actions (overwrite, three_way, attributes_merge, new_files, deletions, user_deleted skip), halt on conflicts, gate `attributes_merge` filter changes behind `--accept-attribute-changes`, commit as `chore(template): sync to <version>`.

**Architecture:** New helper `scripts/_template_helpers/sync.py` walks plan output and executes mutations. Orchestrator creates branch (if not already created in Phase 1.5), invokes sync helper, checks for conflict markers, commits.

**Tech Stack:** bash, git (`add`, `commit`, `merge-file`, `check-attr`), Python for plan parsing.

**Spec sections:** `Update flow / Phase 3 — apply / Commit A`, `Manifest schema / Strategy semantics`, `attributes_merge strategy`.

---

## File structure

**Created:**
- `scripts/_template_helpers/sync.py` — applies categories from plan output.
- `tests/template-update-sync.bats`

**Modified:**
- `scripts/template-update.sh` — wire Phase 3 Commit A.

**Depends on:** Phases 01–04.

---

## Task 1: Sync helper — overwrite

**Files:**
- Create: `scripts/_template_helpers/sync.py`
- Create: `tests/template-update-sync.bats`

- [ ] **Step 1: Failing test**

Create `tests/template-update-sync.bats`:

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
}
teardown() { rm -rf "$TMP"; }

@test "sync apply-overwrite: copies file from new tree to user tree" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-overwrite \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh
  [ "$status" -eq 0 ]
  [ -f "$TMP/scripts/new-helper.sh" ]
  diff "$V1/scripts/new-helper.sh" "$TMP/scripts/new-helper.sh"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement skeleton**

Create `scripts/_template_helpers/sync.py`:

```python
#!/usr/bin/env python3
"""Phase 3 Commit A sync operations.

Subcommands:
  apply-overwrite     --new-tree <dir> --user-tree <dir> --rel <relpath>
  apply-three-way     --old-tree <dir> --new-tree <dir> --user-tree <dir> --rel <relpath>
  apply-attributes    --old-tree <dir> --new-tree <dir> --user-tree <dir> --rel <relpath>
                      [--accept-attribute-changes]
  apply-new-file      --new-tree <dir> --user-tree <dir> --rel <relpath> --decision <overwrite|skip|mark-as-user-deleted>
  apply-deletion      --user-tree <dir> --rel <relpath> [--decision remove|preserve-local]
  has-conflict-markers --tree <dir> --paths <relpath>...
"""
from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path


def cmd_apply_overwrite(args: argparse.Namespace) -> int:
    src = args.new_tree / args.rel
    dst = args.user_tree / args.rel
    if not src.is_file():
        print(f"source not found: {src}", file=sys.stderr)
        return 1
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(src, dst)
    # Preserve mode bits (executable).
    dst.chmod(src.stat().st_mode)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_o = sub.add_parser("apply-overwrite")
    p_o.add_argument("--new-tree", required=True, type=Path)
    p_o.add_argument("--user-tree", required=True, type=Path)
    p_o.add_argument("--rel", required=True)
    args = parser.parse_args()
    return {"apply-overwrite": cmd_apply_overwrite}[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
```

`chmod +x scripts/_template_helpers/sync.py`.

- [ ] **Step 4: Run test — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/sync.py tests/template-update-sync.bats
git commit -m "feat: sync.py apply-overwrite"
```

---

## Task 2: Sync — three_way merge

**Files:**
- Modify: `scripts/_template_helpers/sync.py`
- Modify: `tests/template-update-sync.bats`

- [ ] **Step 1: Failing tests**

```bash
@test "sync apply-three-way: clean merge with no user changes" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-three-way \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$V1" \
    --user-tree "$TMP" \
    --rel WIKI.md
  [ "$status" -eq 0 ]
  ! grep -q '<<<<<<<' "$TMP/WIKI.md"
  grep -q '## Added in v1' "$TMP/WIKI.md"
}

@test "sync apply-three-way: conflict leaves markers + nonzero" {
  cd "$TMP"
  cat > WIKI.md <<'EOF'
# Wiki schema (user)
USER EDITED
EOF
  git add WIKI.md
  git -c user.email=a@b -c user.name=t commit -q -m user-edit
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-three-way \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$V1" --user-tree "$TMP" --rel WIKI.md
  [ "$status" -ne 0 ]
  grep -q '<<<<<<<' "$TMP/WIKI.md"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/_template_helpers/sync.py`:

```python
def cmd_apply_three_way(args: argparse.Namespace) -> int:
    base = args.old_tree / args.rel
    new = args.new_tree / args.rel
    cur = args.user_tree / args.rel
    if not new.is_file():
        return 0  # not in new tree — nothing to do
    if not cur.is_file():
        # User deleted the file; treat as new-file-with-base.
        cur.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(new, cur)
        return 0
    if not base.is_file():
        # No ancestor — fall back to overwrite-with-warning.
        shutil.copyfile(new, cur)
        print(f"warn: no ancestor for {args.rel}; overwrote", file=sys.stderr)
        return 0
    rc = subprocess.call(
        ["git", "merge-file", "--diff3", "-L", "current", "-L", "base", "-L", "new",
         str(cur), str(base), str(new)]
    )
    return rc


# In main(), add subparser:
def _add_three_way(sub):
    p = sub.add_parser("apply-three-way")
    p.add_argument("--old-tree", required=True, type=Path)
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)
```

Wire `_add_three_way(sub)` and dispatch entry `"apply-three-way": cmd_apply_three_way`.

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/sync.py tests/template-update-sync.bats
git commit -m "feat: sync.py apply-three-way"
```

---

## Task 3: Sync — attributes_merge with audit gate

**Files:**
- Modify: `scripts/_template_helpers/sync.py`
- Modify: `tests/template-update-sync.bats`

- [ ] **Step 1: Failing tests**

```bash
@test "sync apply-attributes: encryption-flip halts without --accept-attribute-changes" {
  cd "$TMP"
  echo "" > .gitattributes
  git add .gitattributes
  git -c user.email=a@b -c user.name=t commit -q -m init-attrs
  TMP_NEW=$(mktemp -d)
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." "$TMP_NEW/"
  printf 'secrets/* filter=git-crypt diff=git-crypt\n' > "$TMP_NEW/.gitattributes"
  mkdir -p secrets && echo k > secrets/key.age
  git add secrets && git commit -q -m add-secrets
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-attributes \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$TMP_NEW" \
    --user-tree "$TMP" --rel .gitattributes
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "Encryption pattern change"
  rm -rf "$TMP_NEW"
}

@test "sync apply-attributes: encryption-flip proceeds with --accept-attribute-changes" {
  cd "$TMP"
  echo "" > .gitattributes
  git add .gitattributes
  git -c user.email=a@b -c user.name=t commit -q -m init-attrs
  TMP_NEW=$(mktemp -d)
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." "$TMP_NEW/"
  printf 'secrets/* filter=git-crypt diff=git-crypt\n' > "$TMP_NEW/.gitattributes"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-attributes \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$TMP_NEW" \
    --user-tree "$TMP" --rel .gitattributes \
    --accept-attribute-changes
  [ "$status" -eq 0 ]
  grep -q 'filter=git-crypt' .gitattributes
  rm -rf "$TMP_NEW"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/_template_helpers/sync.py`:

```python
def cmd_apply_attributes(args: argparse.Namespace) -> int:
    rel = args.rel
    base = args.old_tree / rel
    new = args.new_tree / rel
    cur = args.user_tree / rel
    if not new.is_file():
        return 0
    # 3-way merge first.
    if not cur.is_file():
        cur.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(new, cur)
    elif base.is_file():
        rc = subprocess.call(
            ["git", "merge-file", "--diff3",
             "-L", "current", "-L", "base", "-L", "new",
             str(cur), str(base), str(new)]
        )
        if rc != 0:
            return rc
    else:
        shutil.copyfile(new, cur)
    # Attr audit: list tracked files.
    tracked = subprocess.run(
        ["git", "-C", str(args.user_tree), "ls-files"],
        capture_output=True, text=True
    ).stdout.splitlines()
    flipped = False
    for p in tracked:
        old_attrs = subprocess.run(
            ["git", "-c", f"core.attributesfile={base if base.is_file() else '/dev/null'}",
             "check-attr", "-a", p],
            capture_output=True, text=True
        ).stdout
        new_attrs = subprocess.run(
            ["git", "-c", f"core.attributesfile={cur}",
             "check-attr", "-a", p],
            capture_output=True, text=True
        ).stdout
        if old_attrs != new_attrs:
            old_enc = "filter: git-crypt" in old_attrs
            new_enc = "filter: git-crypt" in new_attrs
            if old_enc != new_enc:
                flipped = True
                print(f"PLAN|attribute-change|{p}|filter-flipped", file=sys.stderr)
    if flipped and not args.accept_attribute_changes:
        print("Encryption pattern change detected. Re-run with --accept-attribute-changes.", file=sys.stderr)
        return 1
    return 0


def _add_attributes(sub):
    p = sub.add_parser("apply-attributes")
    p.add_argument("--old-tree", required=True, type=Path)
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)
    p.add_argument("--accept-attribute-changes", action="store_true")
```

Add to dispatch.

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/sync.py tests/template-update-sync.bats
git commit -m "feat: sync.py apply-attributes with check-attr audit + accept flag"
```

---

## Task 4: Sync — new_file decisions + deletion handling + has-conflict-markers

**Files:**
- Modify: `scripts/_template_helpers/sync.py`
- Modify: `tests/template-update-sync.bats`

- [ ] **Step 1: Failing tests**

```bash
@test "sync apply-new-file overwrite: copies into user tree" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-new-file \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh \
    --decision overwrite
  [ "$status" -eq 0 ]
  [ -f scripts/new-helper.sh ]
}

@test "sync apply-new-file skip: does not copy" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-new-file \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh \
    --decision skip
  [ "$status" -eq 0 ]
  [ ! -f scripts/new-helper.sh ]
}

@test "sync apply-deletion remove: deletes file" {
  cd "$TMP"
  echo x > scripts/old.sh
  git add . && git -c user.email=a@b -c user.name=t commit -q -m add
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-deletion \
    --user-tree "$TMP" --rel scripts/old.sh --decision remove
  [ "$status" -eq 0 ]
  [ ! -f scripts/old.sh ]
}

@test "sync apply-deletion preserve-local: keeps file" {
  cd "$TMP"
  echo x > scripts/keepme.sh
  git add . && git -c user.email=a@b -c user.name=t commit -q -m add
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-deletion \
    --user-tree "$TMP" --rel scripts/keepme.sh --decision preserve-local
  [ "$status" -eq 0 ]
  [ -f scripts/keepme.sh ]
}

@test "sync has-conflict-markers: detects" {
  cd "$TMP"
  cat > WIKI.md <<EOF
<<<<<<< current
a
=======
b
>>>>>>> new
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" has-conflict-markers \
    --tree "$TMP" --paths WIKI.md
  [ "$status" -eq 1 ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Add to `scripts/_template_helpers/sync.py`:

```python
def cmd_apply_new_file(args: argparse.Namespace) -> int:
    src = args.new_tree / args.rel
    dst = args.user_tree / args.rel
    if args.decision == "skip":
        return 0
    if args.decision == "overwrite":
        if not src.is_file():
            print(f"source not found: {src}", file=sys.stderr)
            return 1
        dst.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(src, dst)
        dst.chmod(src.stat().st_mode)
        return 0
    if args.decision == "mark-as-user-deleted":
        # Caller (orchestrator) records the deletion in template.json.deleted[].
        return 0
    print(f"unknown decision: {args.decision}", file=sys.stderr)
    return 2


def cmd_apply_deletion(args: argparse.Namespace) -> int:
    target = args.user_tree / args.rel
    if args.decision == "preserve-local":
        return 0
    if args.decision == "remove":
        if target.is_file():
            target.unlink()
        return 0
    print(f"unknown decision: {args.decision}", file=sys.stderr)
    return 2


def cmd_has_conflict_markers(args: argparse.Namespace) -> int:
    found = False
    for p in args.paths:
        full = args.tree / p
        if not full.is_file():
            continue
        with full.open("rb") as f:
            for line in f:
                if line.startswith(b"<<<<<<<"):
                    found = True
                    print(p)
                    break
    return 1 if found else 0


def _add_remaining(sub):
    p = sub.add_parser("apply-new-file")
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)
    p.add_argument("--decision", required=True, choices=["overwrite", "skip", "mark-as-user-deleted"])

    p = sub.add_parser("apply-deletion")
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)
    p.add_argument("--decision", required=True, choices=["remove", "preserve-local"])

    p = sub.add_parser("has-conflict-markers")
    p.add_argument("--tree", required=True, type=Path)
    p.add_argument("--paths", required=True, nargs="+")
```

Add to dispatch + update `main()` to call all `_add_*` functions.

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/sync.py tests/template-update-sync.bats
git commit -m "feat: sync.py apply-new-file + apply-deletion + has-conflict-markers"
```

---

## Task 5: Wire Phase 3 Commit A in orchestrator

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-sync.bats`

- [ ] **Step 1: Failing test for end-to-end Commit A**

Append to `tests/template-update-sync.bats`:

```bash
@test "template-update --apply: Commit A creates branch and applies sync" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --non-interactive
  [ "$status" -eq 0 ] || true
  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  [[ "$CUR_BRANCH" =~ ^awiki-template-update/ ]]
  [ -f scripts/new-helper.sh ]
  grep -q "Added in v1" WIKI.md
  git log --format=%s -1 | grep -q "sync to"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Commit A in `template-update.sh`**

Replace the trailing `echo "info: phase 3 not yet implemented..."` and `exit 0` with:

```bash
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
PLAN_PATHS_THREE=()
PLAN_PATHS_ATTR=()
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
      # Record mark-as-user-deleted in state for Commit D to write template.json.deleted[].
      if [[ "$DECISION" == "mark-as-user-deleted" ]]; then
        python3 "$HELPERS/state.py" add-deleted-pending "$FETCH_DIR/.update-state.json" \
          --rel "$REL" --reason "user marked at new_file prompt"
      fi
      ;;
    deletion-in-template)
      REL="${parts[2]}"; LOCAL_MOD="${parts[3]}"
      DECISION="remove"
      if [[ "$LOCAL_MOD" == "true" ]]; then
        if [[ $NON_INTERACTIVE -eq 1 ]]; then
          DECISION="preserve-local"
        else
          echo "Locally-modified file removed in template: $REL"
          echo "  [r]emove  [p]reserve-local (default: preserve-local)"
          read -r -p "> " ans
          [[ "$ans" =~ ^r ]] && DECISION="remove" || DECISION="preserve-local"
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
  if python3 "$HELPERS/sync.py" has-conflict-markers --tree "$REPO_ROOT" \
       --paths "${THREE_PATHS_FOR_MARKER_CHECK[@]}"; then
    : # no markers
  else
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

echo "info: Commits B/C/D not yet implemented"
exit 0
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-sync.bats
git commit -m "feat: Phase 3 Commit A wiring (overwrite, three_way, attr, new_file, deletion)"
```

---

## Task 6: CHANGELOG + verify

- [ ] **Step 1: Run all Phase 05 tests**

```bash
bats tests/template-update-sync.bats
```

- [ ] **Step 2: Append CHANGELOG**

```markdown
### Added
- `template-update.sh` Phase 3 Commit A: sync per-strategy (overwrite, three_way, attributes_merge with `--accept-attribute-changes` gate, new_file prompts, deletions with locally-modified prompt), conflict-marker halt, branch creation if needed (Phase 05).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 05 commit A sync"
```

---

## Phase 05 — Definition of done

- [ ] `sync.py` covers: apply-overwrite, apply-three-way, apply-attributes (with audit), apply-new-file (3 decisions), apply-deletion (2 decisions), has-conflict-markers.
- [ ] Orchestrator walks plan output, dispatches per strategy.
- [ ] `attributes_merge` halts on encryption flip without `--accept-attribute-changes`.
- [ ] Conflict markers halt the apply.
- [ ] `--non-interactive` defaults: new_file=skip, deletion-locally-modified=preserve-local.
- [ ] Commit A messaged `chore(template): sync to <version> (<short-sha>)`.
- [ ] State file phase=commit-a status=committed last_completed_commit set.
- [ ] CHANGELOG entry added.

Phase 06 implements Commit B (mechanical migrations + LLM staging).
