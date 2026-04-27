#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

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
  rel="${page#"$CONTENT_DIR"/}"
  slug="$(basename "$page" .md)"
  printf '%s\t%s\n' "$slug" "$rel" >> "$SLUG_MAP"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^title:[[:space:]]*\"?([^\"]+)\"?[[:space:]]*$ ]]; then
      title="${BASH_REMATCH[1]}"
      printf '%s\t%s\n' "$slug" "$title" >> "$TITLE_MAP"
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

[[ "$MAPS_ONLY" -eq 1 ]] && exit 0

# Rewrite content into build-content
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

while IFS= read -r -d '' page; do
  rel="${page#"$CONTENT_DIR"/}"
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
