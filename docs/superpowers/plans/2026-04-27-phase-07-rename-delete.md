# awiki Plan — Phase 7: Slug Rename, Deletion, Alias Resolution

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 2, 3
**Previous:** [Phase 06](./2026-04-27-phase-06-section-indexes.md)
**Next:** [Phase 08](./2026-04-27-phase-08-mcp-server.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** `rename.sh`, `delete-page.sh`. Alias-collision lint rule (already shipped phase 2/3).

**Branch:** `phase-7-rename-delete`

## Task 7.0: Branch

- [ ] **Step 1: Create branch**

```bash
git checkout -b phase-7-rename-delete
```

## Task 7.1: `scripts/rename.sh`

**Files:** Create: `scripts/rename.sh`, `tests/rename_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/rename_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities" "$WORK/scripts"
  # rename.sh shells `bash scripts/log-append.sh ...` from cwd; mirror that script.
  cp "$REPO_ROOT/scripts/log-append.sh" "$WORK/scripts/log-append.sh"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
type: entity
---

Body.
E
  cat > "$WORK/content/entities/bar.md" <<E
---
title: "Bar"
type: entity
---

References [[foo]].
E
  cd "$WORK"
}

teardown() { rm -rf "$WORK"; }

@test "rename moves file and updates wikilinks" {
  bash "$REPO_ROOT/scripts/rename.sh" foo foo-renamed
  [ -f content/entities/foo-renamed.md ]
  [ ! -f content/entities/foo.md ]
  run grep -F '[[foo-renamed]]' content/entities/bar.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Write `scripts/rename.sh`**

```bash
cat > scripts/rename.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: rename.sh <old-slug> <new-slug>" >&2; exit 1; }
OLD="$1"; NEW="$2"

OLD_PATH="$(find content -name "$OLD.md" -type f | head -1)"
[[ -n "$OLD_PATH" ]] || { echo "slug not found: $OLD" >&2; exit 2; }

NEW_PATH="$(dirname "$OLD_PATH")/$NEW.md"
[[ ! -e "$NEW_PATH" ]] || { echo "target exists: $NEW_PATH" >&2; exit 3; }

git mv "$OLD_PATH" "$NEW_PATH" 2>/dev/null || mv "$OLD_PATH" "$NEW_PATH"

# Update all wikilinks — portable across BSD/GNU sed
find content -name '*.md' -type f -print0 | while IFS= read -r -d '' f; do
  awk -v old="$OLD" -v new="$NEW" '
    {
      gsub("\\[\\[" old "\\]\\]", "[[" new "]]")
      gsub("\\[\\[" old "\\|", "[[" new "|")
    }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

bash scripts/log-append.sh rename "$OLD -> $NEW"
echo "RENAME-OK|old=$OLD|new=$NEW"
EOF
chmod +x scripts/rename.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/rename.sh tests/rename_test.bats
git commit -m "feat: add rename script (updates all wikilinks)"
```

## Task 7.2: `scripts/delete-page.sh`

**Files:** Create: `scripts/delete-page.sh`, `tests/delete_page_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/delete_page_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities" "$WORK/scripts"
  cp "$REPO_ROOT/scripts/log-append.sh" "$WORK/scripts/log-append.sh"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
type: entity
---

Body.
E
  cat > "$WORK/content/entities/bar.md" <<E
---
title: "Bar"
type: entity
---

References [[foo]].
E
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cd "$WORK"
}
teardown() { rm -rf "$WORK"; }

@test "delete-page removes file" {
  bash "$REPO_ROOT/scripts/delete-page.sh" foo
  [ ! -f content/entities/foo.md ]
}

@test "delete-page marks wikilinks as broken" {
  bash "$REPO_ROOT/scripts/delete-page.sh" foo
  run grep -F 'broken: was [[foo]]' content/entities/bar.md
  [ "$status" -eq 0 ]
}

@test "delete-page rejects unknown slug" {
  run bash "$REPO_ROOT/scripts/delete-page.sh" nonexistent
  [ "$status" -eq 2 ]
}
EOF
```

- [ ] **Step 2: Write `scripts/delete-page.sh`**

```bash
cat > scripts/delete-page.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: delete-page.sh <slug>" >&2; exit 1; }
SLUG="$1"

PAGE="$(find content -name "$SLUG.md" -type f | head -1)"
[[ -n "$PAGE" ]] || { echo "slug not found: $SLUG" >&2; exit 2; }

git rm "$PAGE" 2>/dev/null || rm "$PAGE"

# Replace wikilinks with broken markers — portable across BSD/GNU sed
find content -name '*.md' -type f -print0 | while IFS= read -r -d '' f; do
  awk -v slug="$SLUG" '
    {
      gsub("\\[\\[" slug "\\]\\]", "<!-- broken: was [[" slug "]] -->" slug)
      gsub("\\[\\[" slug "\\|", "<!-- broken: was [[" slug "|... -->" slug "|")
    }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

bash scripts/log-append.sh delete "$SLUG (removed; wikilinks marked broken for review)"
echo "DELETE-OK|slug=$SLUG"
EOF
chmod +x scripts/delete-page.sh
```

- [ ] **Step 3: Run test (PASS)**

```bash
bats tests/delete_page_test.bats
```

- [ ] **Step 4: Commit**

```bash
git add scripts/delete-page.sh tests/delete_page_test.bats
git commit -m "feat: add delete-page script (marks broken wikilinks) + BATS test"
```

## Task 7.3: Phase 7 merge

```bash
bats tests/
git checkout main
git merge --no-ff phase-7-rename-delete -m "feat: complete phase 7 rename/delete/alias"
git branch -d phase-7-rename-delete
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 08](./2026-04-27-phase-08-mcp-server.md).
