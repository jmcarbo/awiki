# awiki Plan — Phase 6: Section Indexes + Catalog v2

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 1, 2, 3 (modifies `scripts/lint.sh` from phase 2)
**Previous:** [Phase 05](./2026-04-27-phase-05-encryption.md)
**Next:** [Phase 07](./2026-04-27-phase-07-rename-delete.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** Hugo `page-list` shortcode, BOOTSTRAP scaffolds section indexes, lint exemptions for system pages, catalog cross-link checks.

**Branch:** `phase-6-section-indexes`

## Task 6.1: Branch + Hugo shortcode

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-6-section-indexes
```

- [ ] **Step 2: Create `layouts/shortcodes/page-list.html`**

```bash
mkdir -p layouts/shortcodes
cat > layouts/shortcodes/page-list.html <<'EOF'
{{ $section := .Get "section" | default .Page.Section }}
<ul>
{{ range where (where .Site.RegularPages "Section" $section) "Type" "ne" "section-index" }}
  <li><a href="{{ .RelPermalink }}">{{ .Title }}</a> — {{ .Summary }}</li>
{{ end }}
</ul>
EOF
```

- [ ] **Step 3: Commit**

```bash
git add layouts/shortcodes/page-list.html
git commit -m "feat: add Hugo page-list shortcode"
```

## Task 6.2: Section index scaffolds

- [ ] **Step 1: Create one `_index.md` per section**

```bash
for section in entities concepts topics sources synthesis; do
  cat > "content/$section/_index.md" <<EOF
---
title: "$(echo $section | sed 's/.*/\u&/')"
type: section-index
draft: false
---

Pages tagged \`$section\`.

{{< page-list section="$section" >}}
EOF
done
```

- [ ] **Step 2: Commit**

```bash
git add content/*/_index.md
git commit -m "feat: scaffold section index pages with page-list shortcode"
```

## Task 6.3: Lint exemption for system pages

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add test for orphan check + system-page exemption (single combined test)**

```bash
cat >> tests/lint_test.bats <<'EOF'

@test "lint flags orphan entity" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/island.md" <<E
---
title: "Island"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body content with sufficient length, but no inbound wikilinks anywhere.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|INFO"*"island.md"*"orphan"* ]]
}

@test "lint exempts section-index from orphan check" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/_index.md" <<E
---
title: "Entities"
type: section-index
draft: false
---

Section landing.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" != *"LINT|INFO"*"_index.md"*"orphan"* ]]
}
EOF
```

- [ ] **Step 2: Add orphan check + system-page exemption to `scripts/lint.sh`**

`scripts/lint.sh` was created in phase 2. Open it and find this anchor block (the alias-collision loop near the bottom):

```bash
for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done
```

Insert the orphan-check block IMMEDIATELY AFTER that `done` line and BEFORE the `echo "LINT-SUMMARY|..."` line:

```bash
# Orphan check: count inbound wikilinks per slug; warn if zero.
declare -A INBOUND
while IFS= read -r -d '' page; do
  while read -r link; do
    target="${link%%|*}"
    INBOUND[$target]=$((${INBOUND[$target]:-0} + 1))
    # Also credit the alias's resolved slug, if any
    resolved="${ALIAS_TO_SLUG[$target]:-}"
    if [[ -n "$resolved" ]]; then
      INBOUND[$resolved]=$((${INBOUND[$resolved]:-0} + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
  if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
    continue
  fi
  if [[ "${INBOUND[$slug]:-0}" -eq 0 ]]; then
    echo "LINT|INFO|$page|orphan: no inbound wikilinks"
    INFOS=$((INFOS + 1))
  fi
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
```

- [ ] **Step 3: Run tests (PASS expected)**

```bash
bats tests/lint_test.bats
```

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: lint orphan check with system-page exemption"
```

## Task 6.4: Catalog cross-link check

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add test**

```bash
cat >> tests/lint_test.bats <<'EOF'

@test "lint warns on page missing from catalog" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/catalog.md" <<E
---
title: "Catalog"
type: catalog
---

# Catalog
E
  cat > "$TMP/entities/uncatalogued.md" <<E
---
title: "Uncatalogued"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Uncatalogued [[uncatalogued]] self-reference for connectivity.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|WARN"*"uncatalogued.md"*"missing from catalog"* ]]
}
EOF
```

- [ ] **Step 2: Add catalog cross-link check to lint.sh**

Insert before `LINT-SUMMARY`:

```bash
# Catalog cross-link check
CATALOG="$CONTENT_DIR/catalog.md"
if [[ -f "$CATALOG" ]]; then
  declare -A IN_CATALOG
  while read -r link; do
    target="${link%%|*}"
    IN_CATALOG[$target]=1
  done < <(grep -oE '\[\[[^]]+\]\]' "$CATALOG" | sed -E 's/^\[\[|\]\]$//g')

  while IFS= read -r -d '' page; do
    slug="$(basename "$page" .md)"
    [[ "$slug" =~ ^(_index|catalog|log)$ ]] && continue
    type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
    if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
      continue
    fi
    if [[ -z "${IN_CATALOG[$slug]:-}" ]]; then
      echo "LINT|WARN|$page|missing from catalog"
      WARNS=$((WARNS + 1))
    fi
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi
```

- [ ] **Step 3: Run tests + commit**

```bash
bats tests/lint_test.bats
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: lint warns on pages missing from catalog"
```

## Task 6.5: `scripts/update-catalog.sh`

**Files:** Create: `scripts/update-catalog.sh`, `tests/update_catalog_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/update_catalog_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities"
  cat > "$WORK/content/catalog.md" <<E
---
title: "Catalog"
type: catalog
---

# Catalog
E
  cat > "$WORK/content/entities/foo.md" <<E
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

Body referencing [[foo]] for connectivity, sufficient length.
E
  cd "$WORK"
}
teardown() { rm -rf "$WORK"; }

@test "update-catalog inserts entity entry" {
  bash "$BATS_TEST_DIRNAME/../scripts/update-catalog.sh"
  run grep -F '[[foo]]' content/catalog.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Write `scripts/update-catalog.sh`**

```bash
cat > scripts/update-catalog.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

CATALOG="content/catalog.md"
[[ -f "$CATALOG" ]] || { echo "no catalog at $CATALOG" >&2; exit 1; }

python3 - "$CATALOG" content <<'PY'
import os, sys, re, glob, collections
catalog_path, content_dir = sys.argv[1:3]

groups = collections.OrderedDict([
    ("entity", "Entities"),
    ("concept", "Concepts"),
    ("topic", "Topics"),
    ("source", "Sources"),
    ("synthesis", "Synthesis"),
    ("deck", "Synthesis"),
    ("chart", "Synthesis"),
    ("canvas", "Synthesis"),
])

entries = collections.defaultdict(list)
for path in sorted(glob.glob(os.path.join(content_dir, "**", "*.md"), recursive=True)):
    rel = os.path.relpath(path, content_dir)
    slug = os.path.splitext(os.path.basename(path))[0]
    if slug in ("_index", "catalog", "log"):
        continue
    fm = {}
    with open(path) as f:
        text = f.read()
    m = re.match(r"---\n(.*?)\n---", text, re.S)
    if not m:
        continue
    for line in m.group(1).splitlines():
        if ":" in line:
            k, v = line.split(":", 1)
            fm[k.strip()] = v.strip().strip('"')
    t = fm.get("type", "")
    if t in ("log", "catalog", "section-index"):
        continue
    section = groups.get(t, "Misc")
    title = fm.get("title", slug)
    entries[section].append(f"- [[{slug}]] — {title}")

# Read existing catalog body up to first ##; replace below with our generated sections.
with open(catalog_path) as f:
    full = f.read()
m = re.split(r"^## ", full, count=1, flags=re.M)
prelude = m[0].rstrip() + "\n\n"
out = prelude
for section in ("Entities", "Concepts", "Topics", "Sources", "Synthesis", "Misc"):
    if entries.get(section):
        out += f"## {section}\n\n"
        out += "\n".join(entries[section]) + "\n\n"
with open(catalog_path, "w") as f:
    f.write(out.rstrip() + "\n")
PY

echo "CATALOG-OK"
EOF
chmod +x scripts/update-catalog.sh
```

- [ ] **Step 3: Run test + commit**

```bash
bats tests/update_catalog_test.bats
git add scripts/update-catalog.sh tests/update_catalog_test.bats
git commit -m "feat: add update-catalog script (rebuilds content/catalog.md from frontmatter)"
```

## Task 6.6: Phase 6 merge

```bash
git checkout main
git merge --no-ff phase-6-section-indexes -m "feat: complete phase 6 section indexes + catalog v2"
git branch -d phase-6-section-indexes
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 07](./2026-04-27-phase-07-rename-delete.md).
