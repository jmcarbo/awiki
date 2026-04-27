# Phase 02 — Plan + Merge Helpers

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the read-only and merge-helper layer used by the orchestrator: `template-plan.sh` (emits formal `PLAN|...` output), `template-merge.sh` (3-way wrapper), `template-attr-audit.sh` (`.gitattributes` change detector), `template-source-check.sh` (source-change confirmation).

**Architecture:** Each helper is single-responsibility, callable from bash. Plan generation diffs `commit_old..commit_new` paths against the new manifest, predicts conflicts via test-merges in a scratch dir, emits structured stdout. Merge helper wraps `git merge-file --diff3` with consistent exit codes. Attr audit diffs `git check-attr` output between old and new file. Source check halts unless `--accept-source-change`.

**Tech Stack:** bash 4+, `git diff`, `git merge-file`, `git check-attr`, `git ls-tree`, Python 3.11+ for path comparison.

**Spec sections:** `Update flow / Phase 2 — plan`, `Plan output format (formal contract)`, `attributes_merge strategy`, `Conflict path`, `Trust Model / Source identity`.

---

## File structure

**Created:**
- `scripts/template-plan.sh` — pure read; emits `PLAN|...` lines.
- `scripts/template-merge.sh` — wraps `git merge-file --diff3` for `three_way` + `attributes_merge`.
- `scripts/template-attr-audit.sh` — `git check-attr` diff helper.
- `scripts/template-source-check.sh` — source-change comparator.
- `scripts/_template_helpers/plan_emit.py` — diff resolver + line emitter (Python).
- `scripts/_template_helpers/escape.py` — pipe/newline/percent escape helpers (shared util).
- `tests/template-plan.bats`
- `tests/template-merge.bats`
- `tests/template-attr-audit.bats`
- `tests/template-source-check.bats`
- `tests/fixtures/template-update/v1/` — extends v0: adds one new file, modifies `WIKI.md`, adds migration `0001-x.sh`.

**Depends on:** Phase 01 (`template-manifest.sh`, `template-provenance.sh`, fixtures/v0).

---

## Task 1: Escape helper for PLAN output

**Files:**
- Create: `scripts/_template_helpers/escape.py`
- Create: `tests/template-plan.bats` (with escape-test stubs)

- [ ] **Step 1: Write failing test**

Create `tests/template-plan.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "escape: pipe -> %7C" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a|b'
  [ "$status" -eq 0 ]
  [ "$output" = "a%7Cb" ]
}

@test "escape: newline -> %0A" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" "$(printf 'a\nb')"
  [ "$status" -eq 0 ]
  [ "$output" = "a%0Ab" ]
}

@test "escape: percent -> %25 (encoded first)" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a%b'
  [ "$status" -eq 0 ]
  [ "$output" = "a%25b" ]
}

@test "escape: combined" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a|b%c'
  [ "$status" -eq 0 ]
  [ "$output" = "a%7Cb%25c" ]
}
```

- [ ] **Step 2: Run test — verify failure**

```bash
bats tests/template-plan.bats
```

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/escape.py`:

```python
#!/usr/bin/env python3
"""Escape PLAN field values per spec: % first, then | and newline."""
import sys


def escape(s: str) -> str:
    return s.replace("%", "%25").replace("|", "%7C").replace("\n", "%0A")


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: escape.py <string>", file=sys.stderr)
        return 2
    print(escape(sys.argv[1]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

`chmod +x scripts/_template_helpers/escape.py`.

- [ ] **Step 4: Run tests — verify passing**

Expected: 4 of 4 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/escape.py tests/template-plan.bats
git commit -m "feat: PLAN-output escape helper (% | newline)"
```

---

## Task 2: v1 fixture (template with diff against v0)

**Files:**
- Create: `tests/fixtures/template-update/v1/` (copy of v0 + targeted changes)

- [ ] **Step 1: Copy v0 to v1**

```bash
cp -R tests/fixtures/template-update/v0 tests/fixtures/template-update/v1
```

- [ ] **Step 2: Bump version + add a script + add a migration + edit WIKI.md (create one)**

Edit `tests/fixtures/template-update/v1/template.manifest.toml`:

```toml
schema_version = 1
template_version = "0.2.0"

[strategies]
overwrite        = ["scripts/**", "BOOTSTRAP.md"]
preserve         = ["content/**", "raw/**"]
three_way        = ["WIKI.md", "hugo.toml"]
attributes_merge = [".gitattributes"]
template_only    = ["template.manifest.toml", "migrations/**"]

[new_file_default]
strategy = "prompt"

[bootstrap]
ordered_steps = ["dep-check", "domain", "stage-commit"]

[bootstrap.dangerous]
ids = []
```

Create `tests/fixtures/template-update/v1/scripts/new-helper.sh`:

```bash
#!/usr/bin/env bash
# New helper added in v1.
echo "v1 helper"
```

Create `tests/fixtures/template-update/v1/migrations/0001-add-tagline.sh`:

```bash
#!/usr/bin/env bash
# migration: 0001-add-tagline
# requires: agent=false
# touches: WIKI.md
# idempotent: yes
set -euo pipefail
if [[ "${1:-}" == "--dry-run" ]]; then
  echo "would append tagline to WIKI.md"
  exit 0
fi
grep -q "## Tagline" WIKI.md || echo -e "\n## Tagline\n" >> WIKI.md
```

Create both `tests/fixtures/template-update/v0/WIKI.md` and v1, with v1 changed:

`tests/fixtures/template-update/v0/WIKI.md`:

```markdown
# Wiki schema (v0)

Conventions go here.
```

`tests/fixtures/template-update/v1/WIKI.md`:

```markdown
# Wiki schema (v1)

Conventions go here.

## Added in v1
New rules.
```

- [ ] **Step 3: Verify diff is what we expect**

```bash
diff -r tests/fixtures/template-update/v0 tests/fixtures/template-update/v1
```

Expected output mentions: differing manifest, differing WIKI.md, only-in-v1 `scripts/new-helper.sh` and `migrations/0001-add-tagline.sh`.

- [ ] **Step 4: Commit**

```bash
git add tests/fixtures/template-update/
git commit -m "test: add v1 fixture (new file + edited hybrid + migration)"
```

---

## Task 3: `template-plan.sh` — enumerate diff against manifest

**Files:**
- Create: `scripts/_template_helpers/plan_emit.py`
- Create: `scripts/template-plan.sh`
- Modify: `tests/template-plan.bats`

- [ ] **Step 1: Add failing tests**

Append to `tests/template-plan.bats`:

```bash
@test "plan: header line emitted" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" \
    --new-tree "$V1" \
    --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc123 \
    --commit-new def456
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|header\|1\|abc123\|def456$'
}

@test "plan: emits overwrite for new scripts/new-helper.sh" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|overwrite\|scripts/new-helper.sh\|'
}

@test "plan: emits three_way for changed WIKI.md" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  # WIKI.md changed in v1; user-tree = v0, so test-merge predicts clean (no user edits).
  echo "$output" | grep -qE '^PLAN\|three_way\|WIKI.md\|clean'
}

@test "plan: emits migration line for new 0001-add-tagline" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|migration\|0001-add-tagline\|migrations/0001-add-tagline.sh\|'
  echo "$output" | grep -qE '^PLAN\|migration-content\|0001-add-tagline\|sha256:[0-9a-f]+\|[0-9]+$'
}

@test "plan: emits footer with counts" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|footer\|errors=[0-9]+\|warnings=[0-9]+\|prompts=[0-9]+\|conflicts=[0-9]+$'
}

@test "plan: predicted conflict when user edited WIKI.md" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  TMP=$(mktemp -d)
  cp -R "$V0/." "$TMP/"
  # User edits the same area new template touches.
  cat > "$TMP/WIKI.md" <<'EOF'
# Wiki schema (user-edited!)

Different conventions go here.
EOF
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$TMP" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|three_way\|WIKI.md\|conflict-predicted\|[1-9]+'
  rm -rf "$TMP"
}
```

- [ ] **Step 2: Run tests — verify failure**

- [ ] **Step 3: Implement Python emitter**

Create `scripts/_template_helpers/plan_emit.py`:

```python
#!/usr/bin/env python3
"""template-plan emitter. Diffs old/new template trees against a user tree
through the new manifest's strategies. Emits PLAN|... lines.

Args (all required):
  --old-tree  <dir>     # snapshot of last-applied template
  --new-tree  <dir>     # snapshot of incoming template
  --user-tree <dir>     # current bootstrapped repo (working tree minus .awiki/)
  --manifest  <path>    # incoming template's template.manifest.toml
  --commit-old <sha>
  --commit-new <sha>
"""
from __future__ import annotations

import argparse
import filecmp
import hashlib
import os
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import manifest_parse  # noqa: E402
from escape import escape  # noqa: E402


def emit(*cols: str) -> None:
    print("PLAN|" + "|".join(escape(c) for c in cols))


def list_files(root: Path) -> set[str]:
    out: set[str] = set()
    for p in root.rglob("*"):
        if p.is_file():
            rel = str(p.relative_to(root))
            if rel.startswith(".git/") or rel == ".git":
                continue
            out.add(rel)
    return out


def file_sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def line_count(path: Path) -> int:
    with path.open("rb") as f:
        return sum(1 for _ in f)


def test_merge(base: Path, new: Path, user: Path, scratch: Path) -> tuple[str, int]:
    """Run git merge-file --diff3 in scratch dir. Return (status, conflict_count)."""
    if not user.exists():
        # User has the file deleted; treat as no merge needed (will be Commit-A decision).
        return ("clean", 0)
    if not base.exists():
        return ("clean", 0)
    work = scratch / "merge"
    work.mkdir(exist_ok=True)
    try:
        # git merge-file mutates the "current" file in place; use a copy.
        cur = work / "cur"
        shutil.copyfile(user, cur)
        b = work / "base"
        shutil.copyfile(base, b)
        n = work / "new"
        shutil.copyfile(new, n)
        result = subprocess.run(
            ["git", "merge-file", "--diff3", "-p", str(cur), str(b), str(n)],
            capture_output=True,
            text=True,
        )
        # exit 0 = clean; >0 = number of conflicts.
        if result.returncode == 0:
            return ("clean", 0)
        return ("conflict-predicted", result.returncode)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def emit_categories(args: argparse.Namespace, manifest: dict) -> dict[str, int]:
    counts = {"errors": 0, "warnings": 0, "prompts": 0, "conflicts": 0}
    old_files = list_files(args.old_tree)
    new_files = list_files(args.new_tree)
    user_files = list_files(args.user_tree)

    changed_or_new = new_files - old_files | {p for p in old_files & new_files
                                              if (args.old_tree / p).is_file()
                                              and (args.new_tree / p).is_file()
                                              and not filecmp.cmp(args.old_tree / p, args.new_tree / p, shallow=False)}
    deleted_in_template = old_files - new_files
    really_new = new_files - old_files

    # Sort path-events alphabetically.
    paths = sorted(changed_or_new | deleted_in_template)

    scratch = Path(tempfile.mkdtemp(prefix="awiki-plan-"))
    try:
        for rel in paths:
            strategy = manifest_parse.resolve_strategy(manifest, rel)

            if rel in deleted_in_template:
                locally_modified = "false"
                if rel in user_files:
                    if not (args.old_tree / rel).is_file() or \
                       not (args.user_tree / rel).is_file() or \
                       not filecmp.cmp(args.old_tree / rel, args.user_tree / rel, shallow=False):
                        locally_modified = "true"
                emit("deletion-in-template", rel, locally_modified)
                continue

            if strategy == "overwrite":
                emit("overwrite", rel, "")
            elif strategy == "preserve":
                emit("preserve", rel, "")
            elif strategy == "template_only":
                emit("template_only", rel, "")
            elif strategy in ("three_way", "attributes_merge"):
                status, conflicts = test_merge(
                    args.old_tree / rel, args.new_tree / rel, args.user_tree / rel, scratch
                )
                emit(strategy, rel, status, str(conflicts))
                if status == "conflict-predicted":
                    counts["conflicts"] += conflicts
            elif strategy == "prompt":
                if rel in really_new:
                    emit("new_file", rel, "prompt")
                    counts["prompts"] += 1
                else:
                    emit("overwrite", rel, "fallback")  # shouldn't normally happen
            else:
                emit("template_only", rel, f"unknown-strategy:{strategy}")
                counts["warnings"] += 1
    finally:
        shutil.rmtree(scratch, ignore_errors=True)

    return counts


def emit_migrations(args: argparse.Namespace, manifest: dict) -> int:
    """Emit migration lines for any NNNN-*.sh or .prompt.md in new_tree/migrations/
    that aren't covered by old_tree/migrations/. Returns count of new migrations."""
    new_mig_dir = args.new_tree / "migrations"
    old_mig_dir = args.old_tree / "migrations"
    if not new_mig_dir.is_dir():
        return 0
    new_mig_files = sorted(p.name for p in new_mig_dir.iterdir() if p.is_file())
    old_mig_files = set()
    if old_mig_dir.is_dir():
        old_mig_files = {p.name for p in old_mig_dir.iterdir() if p.is_file()}
    added = [n for n in new_mig_files if n not in old_mig_files]
    count = 0
    for fname in added:
        if fname == "README.md" or fname == ".gitkeep":
            continue
        full = new_mig_dir / fname
        if fname.endswith(".sh"):
            mid = fname[:-3]
            count += 1
            stat = subprocess.run(
                ["wc", "-l", str(full)],
                capture_output=True, text=True
            )
            lines = stat.stdout.split()[0] if stat.returncode == 0 else "0"
            emit("migration", mid, f"migrations/{fname}", f"{lines}-lines")
            sha = file_sha256(full)
            emit("migration-content", mid, f"sha256:{sha}", lines)
        elif fname.endswith(".prompt.md"):
            mid = fname[:-len(".prompt.md")]
            count += 1
            # Extract scope_glob + risk from frontmatter.
            text = full.read_text(encoding="utf-8")
            scope = ""
            risk = "medium"
            in_fm = False
            for line in text.splitlines():
                if line.strip() == "---":
                    in_fm = not in_fm
                    if not in_fm:
                        break
                    continue
                if in_fm:
                    if line.startswith("scope_glob:"):
                        scope = line.split(":", 1)[1].strip().strip('"').strip("'")
                    elif line.startswith("risk:"):
                        risk = line.split(":", 1)[1].strip()
            # Resolve scope to count (simple glob via Python).
            from fnmatch import fnmatch
            user_files = list_files(args.user_tree)
            matched = [f for f in user_files if fnmatch(f, scope)] if scope else []
            emit(
                "migration-prompt",
                mid,
                f"migrations/{fname}",
                scope,
                str(len(matched)),
                risk,
            )
    return count


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--old-tree", required=True, type=Path)
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--manifest", required=True, type=Path)
    p.add_argument("--commit-old", required=True)
    p.add_argument("--commit-new", required=True)
    args = p.parse_args()

    manifest = manifest_parse.load(args.manifest)
    schema = manifest.get("schema_version", 1)
    print(f"PLAN|header|{schema}|{args.commit_old}|{args.commit_new}")

    counts = emit_categories(args, manifest)
    counts["new_migrations"] = emit_migrations(args, manifest)

    print(
        f"PLAN|footer|errors={counts['errors']}|warnings={counts['warnings']}"
        f"|prompts={counts['prompts']}|conflicts={counts['conflicts']}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Bash wrapper**

Create `scripts/template-plan.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/_template_helpers/plan_emit.py" "$@"
```

`chmod +x scripts/template-plan.sh scripts/_template_helpers/plan_emit.py`.

- [ ] **Step 5: Run tests — verify passing**

```bash
bats tests/template-plan.bats
```

Expected: all 10 (4 escape + 6 plan) pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/template-plan.sh scripts/_template_helpers/plan_emit.py tests/template-plan.bats
git commit -m "feat: template-plan.sh emits PLAN|... lines (overwrite, three_way, migration, footer)"
```

---

## Task 4: `template-merge.sh` — apply 3-way merge with conflict surfacing

**Files:**
- Create: `scripts/template-merge.sh`
- Create: `tests/template-merge.bats`

- [ ] **Step 1: Write failing tests**

Create `tests/template-merge.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}

teardown() { rm -rf "$TMP"; }

@test "merge: clean 3-way (no user changes) -> exit 0, no markers" {
  printf 'a\nb\nc\n' > "$TMP/base"
  printf 'a\nb\nc\n' > "$TMP/cur"
  printf 'a\nB\nc\n' > "$TMP/new"
  run bash "$REPO_ROOT/scripts/template-merge.sh" "$TMP/cur" "$TMP/base" "$TMP/new"
  [ "$status" -eq 0 ]
  ! grep -q '<<<<<<<' "$TMP/cur"
  grep -q '^B$' "$TMP/cur"
}

@test "merge: conflict (both edited) -> nonzero, markers present" {
  printf 'a\nb\nc\n' > "$TMP/base"
  printf 'a\nUSER\nc\n' > "$TMP/cur"
  printf 'a\nNEW\nc\n' > "$TMP/new"
  run bash "$REPO_ROOT/scripts/template-merge.sh" "$TMP/cur" "$TMP/base" "$TMP/new"
  [ "$status" -ne 0 ]
  grep -q '<<<<<<<' "$TMP/cur"
}

@test "merge: missing base file -> error" {
  printf 'x\n' > "$TMP/cur"
  printf 'y\n' > "$TMP/new"
  run bash "$REPO_ROOT/scripts/template-merge.sh" "$TMP/cur" "$TMP/missing" "$TMP/new"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE 'base.*not found|missing'
}
```

- [ ] **Step 2: Run tests — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/template-merge.sh`:

```bash
#!/usr/bin/env bash
# Wraps `git merge-file --diff3` for three_way / attributes_merge strategies.
# Mutates <cur> in place. Returns 0 on clean merge, nonzero on conflicts.
set -euo pipefail

usage() {
  echo "usage: template-merge.sh <cur> <base> <new>" >&2
  exit 2
}

[[ $# -eq 3 ]] || usage
CUR="$1"; BASE="$2"; NEW="$3"

[[ -f "$BASE" ]] || { echo "base file not found: $BASE" >&2; exit 3; }
[[ -f "$CUR" ]]  || { echo "current file not found: $CUR" >&2; exit 3; }
[[ -f "$NEW" ]]  || { echo "new file not found: $NEW" >&2; exit 3; }

# git merge-file rewrites <cur> in place. Exit code = number of conflicts.
git merge-file --diff3 -L current -L base -L new "$CUR" "$BASE" "$NEW"
RC=$?
if [[ $RC -eq 0 ]]; then
  exit 0
fi
exit "$RC"
```

`chmod +x scripts/template-merge.sh`.

- [ ] **Step 4: Run tests — verify passing**

Expected: 3 of 3 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-merge.sh tests/template-merge.bats
git commit -m "feat: template-merge.sh wraps git merge-file --diff3 for three_way + attributes_merge"
```

---

## Task 5: `template-attr-audit.sh` — `git check-attr` diff helper

**Files:**
- Create: `scripts/template-attr-audit.sh`
- Create: `tests/template-attr-audit.bats`

- [ ] **Step 1: Write failing tests**

Create `tests/template-attr-audit.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
}

teardown() { rm -rf "$TMP"; }

@test "attr-audit: detects new filter=git-crypt pattern" {
  cd "$TMP"
  echo "secrets/foo" > tracked.txt
  git add tracked.txt
  git commit -q -m add
  printf '' > .gitattributes-old
  printf 'secrets/* filter=git-crypt diff=git-crypt\n' > .gitattributes-new
  run bash "$REPO_ROOT/scripts/template-attr-audit.sh" \
    --old .gitattributes-old --new .gitattributes-new --paths tracked.txt
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|attribute-change\|tracked.txt\|'
}

@test "attr-audit: no change -> no output" {
  cd "$TMP"
  echo content > tracked.txt
  git add tracked.txt
  git commit -q -m add
  printf 'tracked.txt diff=foo\n' > .gitattributes-old
  printf 'tracked.txt diff=foo\n' > .gitattributes-new
  run bash "$REPO_ROOT/scripts/template-attr-audit.sh" \
    --old .gitattributes-old --new .gitattributes-new --paths tracked.txt
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "attr-audit: filter=git-crypt added -> exit 1 (signals encryption flip)" {
  cd "$TMP"
  echo content > tracked.txt
  git add tracked.txt
  git commit -q -m add
  printf '' > .gitattributes-old
  printf 'tracked.txt filter=git-crypt diff=git-crypt\n' > .gitattributes-new
  run bash "$REPO_ROOT/scripts/template-attr-audit.sh" \
    --old .gitattributes-old --new .gitattributes-new --paths tracked.txt
  [ "$status" -eq 1 ]
}
```

- [ ] **Step 2: Run tests — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/template-attr-audit.sh`:

```bash
#!/usr/bin/env bash
# Diff `git check-attr` output between OLD and NEW .gitattributes for each path.
# Emits PLAN|attribute-change|... lines. Exit 1 if any filter=git-crypt added/removed.
set -euo pipefail

OLD=""
NEW=""
PATHS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --old) OLD="$2"; shift 2 ;;
    --new) NEW="$2"; shift 2 ;;
    --paths) shift; PATHS=("$@"); break ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

[[ -f "$OLD" ]] || { echo "old gitattributes not found" >&2; exit 2; }
[[ -f "$NEW" ]] || { echo "new gitattributes not found" >&2; exit 2; }
[[ ${#PATHS[@]} -gt 0 ]] || exit 0

ENCRYPTION_FLIPPED=0

for P in "${PATHS[@]}"; do
  OLD_ATTRS=$(GIT_ATTR_NOSYSTEM=1 git -c "core.attributesfile=$OLD" check-attr -a "$P" 2>/dev/null | sort | tr '\n' ';')
  NEW_ATTRS=$(GIT_ATTR_NOSYSTEM=1 git -c "core.attributesfile=$NEW" check-attr -a "$P" 2>/dev/null | sort | tr '\n' ';')

  if [[ "$OLD_ATTRS" != "$NEW_ATTRS" ]]; then
    OLD_ENC=$(echo "$OLD_ATTRS" | grep -c "filter: git-crypt" || true)
    NEW_ENC=$(echo "$NEW_ATTRS" | grep -c "filter: git-crypt" || true)
    if [[ "$OLD_ENC" != "$NEW_ENC" ]]; then
      ENCRYPTION_FLIPPED=1
    fi
    # URL-encode the attr strings minimally (replace pipe).
    OE=${OLD_ATTRS//|/%7C}
    NE=${NEW_ATTRS//|/%7C}
    PE=${P//|/%7C}
    echo "PLAN|attribute-change|${PE}|${OE}|${NE}"
  fi
done

[[ $ENCRYPTION_FLIPPED -eq 1 ]] && exit 1
exit 0
```

`chmod +x scripts/template-attr-audit.sh`.

- [ ] **Step 4: Run tests — verify passing**

Expected: 3 of 3 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-attr-audit.sh tests/template-attr-audit.bats
git commit -m "feat: template-attr-audit.sh detects .gitattributes filter=git-crypt flips"
```

---

## Task 6: `template-source-check.sh` — source-change comparator

**Files:**
- Create: `scripts/template-source-check.sh`
- Create: `tests/template-source-check.bats`

- [ ] **Step 1: Write failing tests**

Create `tests/template-source-check.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  PJ="$TMP/template.json"
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" \
    https://github.com/jmcarbo/awiki main 1.0.0 abc123 >/dev/null
}

teardown() { rm -rf "$TMP"; }

@test "source-check: matching source -> exit 0" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance "$PJ" --source https://github.com/jmcarbo/awiki
  [ "$status" -eq 0 ]
}

@test "source-check: different source -> exit 1 with halt message" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance "$PJ" --source https://github.com/evil/fork
  [ "$status" -eq 1 ]
  echo "$output" | grep -q "Source change detected"
  echo "$output" | grep -q "github.com/jmcarbo/awiki"
  echo "$output" | grep -q "github.com/evil/fork"
}

@test "source-check: --accept-source-change overrides" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance "$PJ" --source https://github.com/evil/fork --accept-source-change
  [ "$status" -eq 0 ]
}

@test "source-check: missing provenance -> exit 2" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance /nonexistent --source url
  [ "$status" -eq 2 ]
}
```

- [ ] **Step 2: Run tests — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/template-source-check.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PROVENANCE=""
SOURCE=""
ACCEPT=0

while [[ $# -gt 0 ]]; do
  case "$1" in
    --provenance) PROVENANCE="$2"; shift 2 ;;
    --source)     SOURCE="$2"; shift 2 ;;
    --accept-source-change) ACCEPT=1; shift ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

[[ -f "$PROVENANCE" ]] || { echo "provenance not found: $PROVENANCE" >&2; exit 2; }
[[ -n "$SOURCE" ]] || { echo "--source required" >&2; exit 2; }

PINNED=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PROVENANCE" repo)

if [[ "$PINNED" == "$SOURCE" ]]; then
  exit 0
fi

if [[ $ACCEPT -eq 1 ]]; then
  echo "info: source change accepted (pinned=$PINNED, new=$SOURCE)" >&2
  exit 0
fi

cat <<EOF >&2
Source change detected:
  pinned : $PINNED
  new    : $SOURCE
This will execute migrations and overwrite tracked files from the new source.
Re-run with --accept-source-change to proceed.
EOF
exit 1
```

`chmod +x scripts/template-source-check.sh`.

- [ ] **Step 4: Run tests — verify passing**

Expected: 4 of 4 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-source-check.sh tests/template-source-check.bats
git commit -m "feat: template-source-check.sh halts on --source pin mismatch"
```

---

## Task 7: Verify Phase 02 suite green together

**Files:** none — verification.

- [ ] **Step 1: Run all Phase 02 tests**

```bash
bats tests/template-plan.bats tests/template-merge.bats tests/template-attr-audit.bats tests/template-source-check.bats
```

Expected: all pass.

- [ ] **Step 2: Sanity-test `template-plan.sh` end-to-end on v0→v1 fixtures**

```bash
bash scripts/template-plan.sh \
  --old-tree tests/fixtures/template-update/v0 \
  --new-tree tests/fixtures/template-update/v1 \
  --user-tree tests/fixtures/template-update/v0 \
  --manifest tests/fixtures/template-update/v1/template.manifest.toml \
  --commit-old aaa --commit-new bbb
```

Expected: header line, an `overwrite` line for `scripts/new-helper.sh`, a `three_way|WIKI.md|clean` line, a `migration|0001-add-tagline|...` line, a `migration-content|...` line, a `footer` line.

- [ ] **Step 3: Update CHANGELOG**

```markdown
### Added
- `template-update` plan + merge layer: `template-plan.sh` (formal `PLAN|...` output), `template-merge.sh` (3-way wrapper), `template-attr-audit.sh` (`.gitattributes` change detector), `template-source-check.sh` (source-change halt) (Phase 02).
```

- [ ] **Step 4: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 02 plan+merge helpers"
```

---

## Phase 02 — Definition of done

- [ ] All 4 BATS test files green: plan, merge, attr-audit, source-check.
- [ ] `template-plan.sh` emits per-event PLAN lines per spec contract.
- [ ] `template-merge.sh` returns 0/N for clean/conflict counts.
- [ ] `template-attr-audit.sh` returns 1 if `filter=git-crypt` added or removed.
- [ ] `template-source-check.sh` halts on mismatch unless `--accept-source-change`.
- [ ] Escape helper produces `%7C %0A %25` for pipe/newline/percent.
- [ ] v1 fixture committed with one new file, one edited hybrid, one mechanical migration.
- [ ] CHANGELOG entry added.

Phase 03 builds the orchestrator wrapping these helpers.
