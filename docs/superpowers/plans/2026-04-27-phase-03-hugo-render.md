# awiki Plan — Phase 3: Hugo Render

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 1
**Previous:** [Phase 02](./2026-04-27-phase-02-scripts-core.md)
**Next:** [Phase 04](./2026-04-27-phase-04-qmd-integration.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** Wikilink preprocessor (`build.sh`), live-watch wrapper (`serve.sh`), `hugo.toml`, `hugo-book` theme submodule, slug+alias maps, smoke render of fixture content.

**Branch:** `phase-3-hugo-render`

## Task 3.1: Spike — confirm preprocessing approach

Spike, time-boxed 4 hours. Output: `docs/decisions/wikilink-rendering.md`.

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-3-hugo-render
```

- [ ] **Step 2: Add `hugo-book` theme submodule**

```bash
git submodule add https://github.com/alex-shpak/hugo-book themes/hugo-book
git commit -m "chore: add hugo-book theme submodule"
```

- [ ] **Step 3: Write spike scratch**

Create `docs/decisions/wikilink-rendering.md` with answers to:
1. Does goldmark + hugo-book render `[[slug]]` natively? (verify by trying — write a minimal page).
2. If no, is preprocessing into `[<title>](/<section>/<slug>/)` sufficient? (test on 3 pages with title-from-frontmatter resolution).
3. Does Hugo's relative `--source` flag work with content/ outside the project root?

- [ ] **Step 4: Decide and document**

Spec says: preprocessing pipeline. Confirm with spike or pivot. If pivot, update Hugo Render section of spec before continuing.

- [ ] **Step 5: Commit**

```bash
git add docs/decisions/wikilink-rendering.md
git commit -m "docs: record wikilink rendering decision"
```

## Task 3.2: `hugo.toml`

**Files:** Create: `hugo.toml`

- [ ] **Step 1: Write `hugo.toml`**

```toml
baseURL = 'https://example.com/'
title = 'awiki'
theme = 'hugo-book'
contentDir = '.awiki/build-content'

[markup]
  [markup.goldmark]
    [markup.goldmark.renderer]
      unsafe = true
  [markup.tableOfContents]
    startLevel = 2
    endLevel = 4

[params]
  BookSection = 'docs'
```

Hugo is invoked from the repo root with `hugo --source .` (default). `contentDir` is relative to repo root and points at the preprocessor's output. `.awiki/build-content/` must exist before Hugo runs — `scripts/build.sh` creates it.

- [ ] **Step 2: Commit**

```bash
git add hugo.toml
git commit -m "feat: add hugo.toml pointing to preprocessed build dir"
```

## Task 3.3: `scripts/build.sh` — slug+alias maps

**Files:** Create: `scripts/build.sh`, `tests/build_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/build_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/content/entities" "$WORK/.awiki/maps"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: [Effoh]
sources: []
draft: false
---

Sees [[bar]].
E
  cat > "$WORK/content/entities/bar.md" <<E
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

Sees [[foo]] and [[Effoh|alias-display]].
E
  cd "$WORK"
}

teardown() {
  rm -rf "$WORK"
}

@test "build emits slug map" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh" --maps-only
  run cat .awiki/maps/slug-to-path.tsv
  [[ "$output" == *"foo	entities/foo.md"* ]]
  [[ "$output" == *"bar	entities/bar.md"* ]]
}

@test "build rewrites wikilinks in build-content" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/foo.md
  [[ "$output" == *"[Bar](/entities/bar/)"* ]]
}

@test "build resolves alias wikilink" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/bar.md
  [[ "$output" == *"[alias-display](/entities/foo/)"* ]]
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

Run: `bats tests/build_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 3: Write `scripts/build.sh`**

```bash
cat > scripts/build.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

MAPS_ONLY=0
[[ "${1:-}" == "--maps-only" ]] && MAPS_ONLY=1

CONTENT_DIR="content"
BUILD_DIR=".awiki/build-content"
MAPS_DIR=".awiki/maps"
mkdir -p "$MAPS_DIR" "$BUILD_DIR"

SLUG_MAP="$MAPS_DIR/slug-to-path.tsv"
ALIAS_MAP="$MAPS_DIR/alias-to-slug.tsv"
TITLE_MAP="$MAPS_DIR/slug-to-title.tsv"

: > "$SLUG_MAP"
: > "$ALIAS_MAP"
: > "$TITLE_MAP"

# Build maps
while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  slug="$(basename "$page" .md)"
  echo -e "$slug\t$rel" >> "$SLUG_MAP"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^title:[[:space:]]*\"?([^\"]+)\"?[[:space:]]*$ ]]; then
      title="${BASH_REMATCH[1]}"
      echo -e "$slug\t$title" >> "$TITLE_MAP"
    fi
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        echo -e "$a\t$slug" >> "$ALIAS_MAP"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

[[ "$MAPS_ONLY" -eq 1 ]] && exit 0

# Rewrite content into build-content
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

resolve_target() {
  local target="$1"
  local rel
  rel="$(awk -F'\t' -v s="$target" '$1==s{print $2; exit}' "$SLUG_MAP")"
  if [[ -z "$rel" ]]; then
    local resolved_slug
    resolved_slug="$(awk -F'\t' -v a="$target" '$1==a{print $2; exit}' "$ALIAS_MAP")"
    if [[ -n "$resolved_slug" ]]; then
      rel="$(awk -F'\t' -v s="$resolved_slug" '$1==s{print $2; exit}' "$SLUG_MAP")"
    fi
  fi
  echo "$rel"
}

resolve_title() {
  local slug="$1"
  awk -F'\t' -v s="$slug" '$1==s{print $2; exit}' "$TITLE_MAP"
}

while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  out="$BUILD_DIR/$rel"
  mkdir -p "$(dirname "$out")"

  python3 - "$page" "$out" "$SLUG_MAP" "$ALIAS_MAP" "$TITLE_MAP" <<'PY'
import re, sys, os
src, out, slug_map, alias_map, title_map = sys.argv[1:6]
def load(path):
    d = {}
    for line in open(path):
        line = line.rstrip('\n')
        if '\t' in line:
            k, v = line.split('\t', 1)
            d[k] = v
    return d
slugs = load(slug_map)
aliases = load(alias_map)
titles = load(title_map)
text = open(src).read()
def repl(m):
    inner = m.group(1)
    if '|' in inner:
        target, display = inner.split('|', 1)
    else:
        target = display = inner
    rel = slugs.get(target)
    resolved_slug = target
    if not rel:
        resolved_slug = aliases.get(target, '')
        rel = slugs.get(resolved_slug, '')
    if not rel:
        return m.group(0)
    section, _ = os.path.split(rel)
    if display == target:
        display = titles.get(resolved_slug, target)
    url = '/' + section + '/' + os.path.splitext(os.path.basename(rel))[0] + '/'
    return f'[{display}]({url})'
text = re.sub(r'\[\[([^\]]+)\]\]', repl, text)
open(out, 'w').write(text)
PY
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

echo "BUILD-OK|content=$CONTENT_DIR|build=$BUILD_DIR"
EOF
chmod +x scripts/build.sh
```

Note: depends on `python3` (universally available on macOS/Linux). Add to `check-deps.sh` in a follow-up commit.

- [ ] **Step 4: Run test (expect PASS)**

Run: `bats tests/build_test.bats`
Expected: 3 tests pass.

- [ ] **Step 5: Update `check-deps.sh`**

Add `check python3 "3.8" "brew install python" "apt install python3"` after the `check bats` line.

- [ ] **Step 6: Commit**

```bash
git add scripts/build.sh scripts/check-deps.sh tests/build_test.bats
git commit -m "feat: add build.sh wikilink preprocessor with slug+alias maps"
```

## Task 3.4: `scripts/serve.sh`

**Files:** Create: `scripts/serve.sh`

- [ ] **Step 1: Write `scripts/serve.sh`**

```bash
cat > scripts/serve.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

bash scripts/build.sh

if command -v entr >/dev/null 2>&1; then
  echo "WATCH|tool=entr"
  ( while true; do
      find content -name '*.md' | entr -d bash scripts/build.sh
    done ) &
elif command -v fswatch >/dev/null 2>&1; then
  echo "WATCH|tool=fswatch"
  ( fswatch -o content | while read -r _; do bash scripts/build.sh; done ) &
else
  echo "WATCH|tool=poll-1s"
  ( while true; do bash scripts/build.sh; sleep 1; done ) &
fi
WATCHER_PID=$!
trap "kill $WATCHER_PID 2>/dev/null || true" EXIT

hugo server --bind 0.0.0.0 --port 1313 -D
EOF
chmod +x scripts/serve.sh
```

- [ ] **Step 2: Smoke test**

Run: `just serve` for 5 seconds, verify Hugo binds (no crash). Ctrl-C.

- [ ] **Step 3: Commit**

```bash
git add scripts/serve.sh
git commit -m "feat: add serve script with watcher fallback"
```

## Task 3.5: Hugo build recipe + `_index.md`

**Files:** Modify: `scripts/build.sh` (extend), Create: `content/_index.md`

- [ ] **Step 1: Rewrite `scripts/build.sh` with proper arg parsing + `--full` flag**

Replace the entire file with:

```bash
cat > scripts/build.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

MAPS_ONLY=0
FULL=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --maps-only) MAPS_ONLY=1; shift ;;
    --full) FULL=1; shift ;;
    -*) echo "unknown flag: $1" >&2; exit 1 ;;
    *) echo "unexpected positional: $1" >&2; exit 1 ;;
  esac
done

if [[ "$MAPS_ONLY" -eq 1 && "$FULL" -eq 1 ]]; then
  echo "--maps-only and --full are mutually exclusive" >&2
  exit 1
fi

CONTENT_DIR="content"
BUILD_DIR=".awiki/build-content"
MAPS_DIR=".awiki/maps"
mkdir -p "$MAPS_DIR" "$BUILD_DIR"

SLUG_MAP="$MAPS_DIR/slug-to-path.tsv"
ALIAS_MAP="$MAPS_DIR/alias-to-slug.tsv"
TITLE_MAP="$MAPS_DIR/slug-to-title.tsv"

: > "$SLUG_MAP"
: > "$ALIAS_MAP"
: > "$TITLE_MAP"

while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  slug="$(basename "$page" .md)"
  printf '%s\t%s\n' "$slug" "$rel" >> "$SLUG_MAP"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^title:[[:space:]]*\"?([^\"]+)\"?[[:space:]]*$ ]]; then
      printf '%s\t%s\n' "$slug" "${BASH_REMATCH[1]}" >> "$TITLE_MAP"
    fi
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        printf '%s\t%s\n' "$a" "$slug" >> "$ALIAS_MAP"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

if [[ "$MAPS_ONLY" -eq 1 ]]; then
  echo "BUILD-OK|maps-only=1"
  exit 0
fi

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  out="$BUILD_DIR/$rel"
  mkdir -p "$(dirname "$out")"

  python3 - "$page" "$out" "$SLUG_MAP" "$ALIAS_MAP" "$TITLE_MAP" <<'PY'
import re, sys, os
src, out, slug_map, alias_map, title_map = sys.argv[1:6]
def load(path):
    d = {}
    for line in open(path):
        line = line.rstrip('\n')
        if '\t' in line:
            k, v = line.split('\t', 1)
            d[k] = v
    return d
slugs = load(slug_map)
aliases = load(alias_map)
titles = load(title_map)
text = open(src).read()
def repl(m):
    inner = m.group(1)
    if '|' in inner:
        target, display = inner.split('|', 1)
    else:
        target = display = inner
    rel = slugs.get(target)
    resolved_slug = target
    if not rel:
        resolved_slug = aliases.get(target, '')
        rel = slugs.get(resolved_slug, '')
    if not rel:
        return m.group(0)
    section, _ = os.path.split(rel)
    if display == target:
        display = titles.get(resolved_slug, target)
    url = '/' + section + '/' + os.path.splitext(os.path.basename(rel))[0] + '/'
    return f'[{display}]({url})'
text = re.sub(r'\[\[([^\]]+)\]\]', repl, text)
open(out, 'w').write(text)
PY
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

echo "BUILD-OK|content=$CONTENT_DIR|build=$BUILD_DIR"

if [[ "$FULL" -eq 1 ]]; then
  hugo --minify --destination public
  echo "HUGO-OK|out=public"
fi
EOF
chmod +x scripts/build.sh
```

This replaces the earlier task 3.3 version once `--full` lands. Hugo is invoked from repo root (no `--source` flag needed); `hugo.toml`'s `contentDir = '.awiki/build-content'` does the redirection.

- [ ] **Step 2: Update `_index.md` to be appropriate for the seeded version (already created in Phase 1; only edit if Phase 1 placeholder needs polish)**

```bash
cat > content/_index.md <<'EOF'
---
title: "awiki"
type: section-index
draft: false
---

# Welcome

This is your awiki. Browse:

- [[catalog]] — full content listing.
- [[log]] — chronological activity log.
- Sections: entities, concepts, topics, sources, synthesis.

Edit this page manually after BOOTSTRAP runs.
EOF
```

- [ ] **Step 3: Update `justfile` build recipe to pass `--full`**

Edit `justfile`, change the `build` recipe to:

```just
build:
    bash scripts/build.sh --full
```

- [ ] **Step 4: Smoke test**

Run: `just build && ls public/`
Expected: `public/index.html` exists.

- [ ] **Step 5: Commit**

```bash
git add scripts/build.sh content/_index.md justfile
git commit -m "feat: hugo build via build.sh --full; refine _index.md landing"
```

## Task 3.6: Phase 3 merge

- [ ] **Step 1: Run tests + smoke**

```bash
bats tests/
just build
just serve & sleep 3 && kill %1
```

- [ ] **Step 2: Merge**

```bash
git checkout main
git merge --no-ff phase-3-hugo-render -m "feat: complete phase 3 hugo render"
git branch -d phase-3-hugo-render
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 04](./2026-04-27-phase-04-qmd-integration.md).
