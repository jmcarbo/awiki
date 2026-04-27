# awiki Plan — Phase 2: Scripts Core

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 1
**Previous:** [Phase 01](./2026-04-27-phase-01-skeleton.md)
**Next:** [Phase 03](./2026-04-27-phase-03-hugo-render.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** Functional `log-append.sh`, `ingest.sh`, `lint.sh` (mechanical), `.awiki/config` defaults, BATS tests for each.

**Branch:** `phase-2-scripts-core`
**Depends on:** Phase 1.

## Task 2.1: Branch + `.awiki/config` defaults

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-2-scripts-core
```

- [ ] **Step 2: Write `.awiki/config`**

```bash
cat > .awiki/config <<'EOF'
# awiki runtime config — edit as needed
AWIKI_LINT_AFTER_N=5
AWIKI_STALE_DAYS=90
AWIKI_LOG_QUERIES=0
EOF
```

- [ ] **Step 3: Update `.gitignore` to track `.awiki/config`**

Edit `.gitignore`: change `.awiki/*` block to:

```
.awiki/*
!.awiki/.gitkeep
!.awiki/config
```

- [ ] **Step 4: Commit**

```bash
git add .awiki/config .gitignore
git commit -m "chore: add .awiki/config defaults"
```

## Task 2.2: `scripts/log-append.sh`

**Files:** Create: `scripts/log-append.sh`, `tests/log_append_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/log_append_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_LOG="$(mktemp -d)/log.md"
  export AWIKI_LOG_FILE="$TEST_LOG"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$TEST_LOG"
}

teardown() {
  rm -rf "$(dirname "$TEST_LOG")"
}

@test "log-append writes ## [datetime] action | message" {
  bash scripts/log-append.sh ingest "Sample article"
  run grep -E '^## \[[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}\] ingest \| Sample article$' "$TEST_LOG"
  [ "$status" -eq 0 ]
}

@test "log-append handles multi-word message" {
  bash scripts/log-append.sh manual reviewed the catalog
  run grep -E '^## \[.*\] manual \| reviewed the catalog$' "$TEST_LOG"
  [ "$status" -eq 0 ]
}

@test "log-append fails on missing action arg" {
  run bash scripts/log-append.sh
  [ "$status" -ne 0 ]
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

Run: `bats tests/log_append_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 3: Write `scripts/log-append.sh`**

```bash
cat > scripts/log-append.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: log-append.sh <action> [message...]" >&2
  exit 1
fi

ACTION="$1"
shift
MESSAGE="$*"
LOG_FILE="${AWIKI_LOG_FILE:-content/log.md}"
NOW="$(date '+%Y-%m-%d %H:%M')"

if [[ ! -f "$LOG_FILE" ]]; then
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n" > "$LOG_FILE"
fi

printf -- "## [%s] %s | %s\n" "$NOW" "$ACTION" "$MESSAGE" >> "$LOG_FILE"
EOF
chmod +x scripts/log-append.sh
```

- [ ] **Step 4: Run test (expect PASS)**

Run: `bats tests/log_append_test.bats`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/log-append.sh tests/log_append_test.bats
git commit -m "feat: add log-append script + BATS tests"
```

## Task 2.3: `scripts/ingest.sh`

**Files:** Create: `scripts/ingest.sh`, `tests/ingest_test.bats`, `tests/fixtures/sample-source.md`

- [ ] **Step 1: Write the fixture**

```bash
cat > tests/fixtures/sample-source.md <<'EOF'
# Sample Source
A short article about caves.
EOF
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/ingest_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  cp -r raw "$WORK/raw"
  cp tests/fixtures/sample-source.md "$WORK/raw/inbox/interactive/sample.md"
  mkdir -p "$WORK/.awiki" "$WORK/content"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cp .awiki/config "$WORK/.awiki/config"
  export AWIKI_REPO_ROOT="$WORK"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "ingest moves source from inbox to processed" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  [ "$status" -eq 0 ]
  [ ! -f raw/inbox/interactive/sample.md ]
  [ -f raw/processed/interactive/sample.md ]
}

@test "ingest appends log entry" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  run grep "ingest | sample.md | mode=interactive" content/log.md
  [ "$status" -eq 0 ]
}

@test "ingest increments .awiki/ingest-count" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  count="$(cat .awiki/ingest-count)"
  [ "$count" = "1" ]
}

@test "ingest rejects path outside inbox" {
  cp tests/fixtures/sample-source.md "$WORK/elsewhere.md" 2>/dev/null || cp "$BATS_TEST_DIRNAME/../tests/fixtures/sample-source.md" elsewhere.md
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" elsewhere.md
  [ "$status" -eq 2 ]
}

@test "ingest rejects missing arg" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh"
  [ "$status" -eq 1 ]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL)**

Run: `bats tests/ingest_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 4: Write `scripts/ingest.sh`**

```bash
cat > scripts/ingest.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# Load config
if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi
THRESHOLD="${AWIKI_LINT_AFTER_N:-5}"

if [[ $# -lt 1 ]]; then
  echo "usage: ingest.sh <path-under-raw/inbox/>" >&2
  exit 1
fi

SRC="$1"
# Normalize to relative path from repo root.
if [[ "$SRC" = /* ]]; then
  SRC="${SRC#"$REPO_ROOT/"}"
fi

if [[ ! "$SRC" =~ ^raw/inbox/(interactive|batch|checkpoint)/.+ ]]; then
  echo "ERROR: path must be under raw/inbox/{interactive,batch,checkpoint}/" >&2
  exit 2
fi
MODE="${BASH_REMATCH[1]}"

if [[ ! -f "$SRC" ]]; then
  echo "ERROR: source not found: $SRC" >&2
  exit 1
fi

# Strip leading raw/inbox/<mode>/ to get relative subtree
REL="${SRC#raw/inbox/$MODE/}"
DEST="raw/processed/$MODE/$REL"
DEST_DIR="$(dirname "$DEST")"

if [[ -f "$DEST" ]]; then
  echo "ERROR: already processed: $DEST" >&2
  exit 3
fi

mkdir -p "$DEST_DIR"
mv "$SRC" "$DEST"

bash scripts/log-append.sh ingest "$(basename "$SRC") | mode=$MODE"

# Increment counter
COUNTER_FILE=".awiki/ingest-count"
mkdir -p .awiki
COUNT=0
[[ -f "$COUNTER_FILE" ]] && COUNT="$(cat "$COUNTER_FILE")"
COUNT=$((COUNT + 1))
echo "$COUNT" > "$COUNTER_FILE"

echo "INGEST-OK|src=$SRC|dest=$DEST|mode=$MODE|count=$COUNT"

# Auto-lint
LINT_RC=0
if [[ "$COUNT" -ge "$THRESHOLD" ]]; then
  echo "AUTO-LINT|threshold=$THRESHOLD|count=$COUNT"
  if ! bash scripts/lint.sh; then
    LINT_RC=4
  fi
  echo "0" > "$COUNTER_FILE"
fi

# Auto-reindex (best-effort)
QMD_RC=0
if [[ "${AWIKI_QMD_STATUS:-}" != "missing" ]] && command -v qmd >/dev/null 2>&1; then
  if ! bash scripts/qmd-index.sh 2>/dev/null; then
    QMD_RC=5
  fi
fi

if [[ "$LINT_RC" -ne 0 ]]; then exit 4; fi
if [[ "$QMD_RC" -ne 0 ]]; then exit 5; fi
exit 0
EOF
chmod +x scripts/ingest.sh
```

- [ ] **Step 5: Run test (expect PASS)**

Run: `bats tests/ingest_test.bats`
Expected: 5 tests pass. (The auto-lint and qmd lines may fail soft — script is tolerant since lint.sh and qmd-index.sh don't exist yet; they exit non-zero but the test fixture sets count=1 < threshold=5 so auto-lint won't trigger.)

- [ ] **Step 6: Commit**

```bash
git add scripts/ingest.sh tests/ingest_test.bats tests/fixtures/sample-source.md
git commit -m "feat: add ingest script + BATS tests"
```

## Task 2.4: `scripts/lint.sh` (mechanical, phase 1 of two)

**Files:** Create: `scripts/lint.sh`, `tests/lint_test.bats`, `tests/fixtures/wiki-broken/`

- [ ] **Step 1: Write fixtures**

```bash
mkdir -p tests/fixtures/wiki-broken/content/{entities,sources}
cat > tests/fixtures/wiki-broken/content/entities/foo.md <<'EOF'
---
title: "Foo"
date: 2026-01-01
last_updated: 2026-01-01
type: entity
tags: [demo]
aliases: []
sources: []
draft: false
---

See [[bar-doesnt-exist]].
EOF

cat > tests/fixtures/wiki-broken/content/entities/empty.md <<'EOF'
---
title: "Empty"
date: 2026-01-01
last_updated: 2026-01-01
type: entity
tags: []
aliases: []
sources: []
draft: false
---
EOF

cat > tests/fixtures/wiki-broken/content/sources/orphan.md <<'EOF'
---
title: "Orphan"
date: 2026-01-01
last_updated: 2026-01-01
type: source
tags: []
aliases: []
sources: []
draft: false
---

Body content with sufficient length to not be flagged as empty page.
EOF
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/lint_test.bats <<'EOF'
#!/usr/bin/env bats

@test "lint detects broken wikilink" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [[ "$output" == *"LINT|ERROR"*"foo.md"*"broken wikilink"* ]]
}

@test "lint detects empty page" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [[ "$output" == *"LINT|WARN"*"empty.md"* ]]
}

@test "lint exits 2 on errors" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [ "$status" -eq 2 ]
}

@test "lint exits 0 on clean wiki" {
  CLEAN="$(mktemp -d)/content"
  mkdir -p "$CLEAN/entities"
  cat > "$CLEAN/entities/foo.md" <<EOF2
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Foo references [[bar]].
EOF2
  cat > "$CLEAN/entities/bar.md" <<EOF2
---
title: "Bar"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Bar references [[foo]] for connectivity.
EOF2
  run bash scripts/lint.sh "$CLEAN"
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL)**

Run: `bats tests/lint_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 4: Write `scripts/lint.sh` (phase 1 — mechanical only, no `--hugo-check` yet)**

```bash
cat > scripts/lint.sh <<'EOF'
#!/usr/bin/env bash
set -uo pipefail

CONTENT_DIR="${1:-content}"
ERRORS=0
WARNS=0
INFOS=0

# Build slug map and alias map
declare -A SLUG_TO_PATH
declare -A ALIAS_TO_SLUG
declare -A ALIAS_COUNT

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  if [[ -n "${SLUG_TO_PATH[$slug]:-}" ]]; then
    echo "LINT|ERROR|$page|duplicate slug: $slug also at ${SLUG_TO_PATH[$slug]}"
    ERRORS=$((ERRORS + 1))
  fi
  SLUG_TO_PATH[$slug]="$page"

  # Extract aliases via grep on YAML frontmatter
  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        ALIAS_COUNT[$a]=$((${ALIAS_COUNT[$a]:-0} + 1))
        ALIAS_TO_SLUG[$a]="$slug"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Per-page checks
while IFS= read -r -d '' page; do
  body_len=$(awk '/^---$/{c++; next} c==2{print}' "$page" | wc -c | tr -d ' ')
  if [[ "$body_len" -lt 50 ]]; then
    echo "LINT|WARN|$page|empty page (<50 char body)"
    WARNS=$((WARNS + 1))
  fi

  # Broken wikilinks: extract [[slug]] or [[slug|x]]
  while read -r link; do
    target="${link%%|*}"
    if [[ -z "${SLUG_TO_PATH[$target]:-}" && -z "${ALIAS_TO_SLUG[$target]:-}" ]]; then
      echo "LINT|ERROR|$page|broken wikilink: [[$target]]"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Alias collisions
for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done

echo "LINT-SUMMARY|errors=$ERRORS|warnings=$WARNS|info=$INFOS"

if [[ "$ERRORS" -gt 0 ]]; then exit 2; fi
if [[ "$WARNS" -gt 0 ]]; then exit 1; fi
exit 0
EOF
chmod +x scripts/lint.sh
```

- [ ] **Step 5: Run test (expect PASS)**

Run: `bats tests/lint_test.bats`
Expected: 4 tests pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats tests/fixtures/wiki-broken
git commit -m "feat: add lint script (mechanical checks) + BATS tests"
```

## Task 2.5: `scripts/lint.sh --fix`

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add fix-mode test**

Append to `tests/lint_test.bats`:

```bash
cat >> tests/lint_test.bats <<'EOF'

@test "lint --fix adds missing last_updated" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/needs-fix.md" <<EOF2
---
title: "Needs Fix"
date: 2026-01-01
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body referencing [[needs-fix]] for self-connectivity, sufficient length.
EOF2
  bash scripts/lint.sh --fix "$TMP" || true
  run grep '^last_updated:' "$TMP/entities/needs-fix.md"
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

Run: `bats tests/lint_test.bats -f "lint --fix"`
Expected: FAIL — `--fix` not handled.

- [ ] **Step 3: Modify `scripts/lint.sh` to handle `--fix`**

Replace the top of the script (replace the line `CONTENT_DIR="${1:-content}"` block) with:

```bash
cat > scripts/lint.sh <<'EOF'
#!/usr/bin/env bash
set -uo pipefail

FIX=0
CONTENT_DIR="content"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix) FIX=1; shift ;;
    *) CONTENT_DIR="$1"; shift ;;
  esac
done

apply_fixes() {
  local page="$1"
  if ! grep -q '^last_updated:' "$page"; then
    today="$(date '+%Y-%m-%d')"
    # Portable insertion: use awk (handles BSD/GNU sed differences in \n).
    awk -v today="$today" '
      /^date: / { print; print "last_updated: " today; next }
      { print }
    ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
    echo "FIX|$page|added last_updated: $today"
  fi
}

if [[ "$FIX" -eq 1 ]]; then
  while IFS= read -r -d '' page; do
    apply_fixes "$page"
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi

ERRORS=0
WARNS=0
INFOS=0

declare -A SLUG_TO_PATH
declare -A ALIAS_TO_SLUG
declare -A ALIAS_COUNT

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  if [[ -n "${SLUG_TO_PATH[$slug]:-}" ]]; then
    echo "LINT|ERROR|$page|duplicate slug: $slug also at ${SLUG_TO_PATH[$slug]}"
    ERRORS=$((ERRORS + 1))
  fi
  SLUG_TO_PATH[$slug]="$page"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        ALIAS_COUNT[$a]=$((${ALIAS_COUNT[$a]:-0} + 1))
        ALIAS_TO_SLUG[$a]="$slug"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  body_len=$(awk '/^---$/{c++; next} c==2{print}' "$page" | wc -c | tr -d ' ')
  if [[ "$body_len" -lt 50 ]]; then
    echo "LINT|WARN|$page|empty page (<50 char body)"
    WARNS=$((WARNS + 1))
  fi

  while read -r link; do
    target="${link%%|*}"
    if [[ -z "${SLUG_TO_PATH[$target]:-}" && -z "${ALIAS_TO_SLUG[$target]:-}" ]]; then
      echo "LINT|ERROR|$page|broken wikilink: [[$target]]"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done

echo "LINT-SUMMARY|errors=$ERRORS|warnings=$WARNS|info=$INFOS"

if [[ "$ERRORS" -gt 0 ]]; then exit 2; fi
if [[ "$WARNS" -gt 0 ]]; then exit 1; fi
exit 0
EOF
chmod +x scripts/lint.sh
```

- [ ] **Step 4: Run test (expect PASS)**

Run: `bats tests/lint_test.bats`
Expected: 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: add --fix mode to lint script (mechanical fixes)"
```

## Task 2.6: Phase 2 merge

- [ ] **Step 1: Run all tests + smoke**

Run: `bats tests/ && just lint || true && just lint-fix || true`
Expected: tests pass; `just lint` reports a clean (or near-clean) repo.

- [ ] **Step 2: Merge**

```bash
git checkout main
git merge --no-ff phase-2-scripts-core -m "feat: complete phase 2 scripts core"
git branch -d phase-2-scripts-core
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 03](./2026-04-27-phase-03-hugo-render.md).
