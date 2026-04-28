#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

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

# Build slug/alias/title maps via a single python3 invocation.
# Skips _index.md files (section indexes) so they don't pollute slug/alias/title maps.
# Uses PyYAML when present; falls back to a state-machine parser that handles
# single+double quoted strings and escaped quotes for the title and alias-list cases.
python3 - "$CONTENT_DIR" "$SLUG_MAP" "$ALIAS_MAP" "$TITLE_MAP" <<'PY'
import os, sys

content_dir, slug_map_path, alias_map_path, title_map_path = sys.argv[1:5]

try:
    import yaml  # type: ignore
    HAVE_YAML = True
except ImportError:
    HAVE_YAML = False


def parse_flow_list(s):
    """Parse a YAML flow-style list `[a, "b", 'c, with comma', "d \"esc\""]`.
    Returns list of unquoted string values. State-machine respects single/double
    quotes and backslash escapes inside double quotes."""
    s = s.strip()
    if not (s.startswith('[') and s.endswith(']')):
        return []
    inner = s[1:-1]
    items = []
    buf = []
    state = 'top'  # top | dq | sq
    i = 0
    while i < len(inner):
        c = inner[i]
        if state == 'top':
            if c == ',':
                items.append(''.join(buf).strip())
                buf = []
            elif c == '"':
                state = 'dq'
                buf.append(c)
            elif c == "'":
                state = 'sq'
                buf.append(c)
            else:
                buf.append(c)
        elif state == 'dq':
            buf.append(c)
            if c == '\\' and i + 1 < len(inner):
                buf.append(inner[i + 1])
                i += 2
                continue
            if c == '"':
                state = 'top'
        elif state == 'sq':
            buf.append(c)
            if c == "'":
                # YAML single-quoted strings escape ' as ''
                if i + 1 < len(inner) and inner[i + 1] == "'":
                    buf.append(inner[i + 1])
                    i += 2
                    continue
                state = 'top'
        i += 1
    tail = ''.join(buf).strip()
    if tail:
        items.append(tail)
    out = []
    for raw in items:
        v = raw.strip()
        if not v:
            continue
        if len(v) >= 2 and v[0] == '"' and v[-1] == '"':
            # decode \" and \\
            inner_v = v[1:-1]
            decoded = []
            j = 0
            while j < len(inner_v):
                ch = inner_v[j]
                if ch == '\\' and j + 1 < len(inner_v):
                    decoded.append(inner_v[j + 1])
                    j += 2
                    continue
                decoded.append(ch)
                j += 1
            out.append(''.join(decoded))
        elif len(v) >= 2 and v[0] == "'" and v[-1] == "'":
            out.append(v[1:-1].replace("''", "'"))
        else:
            out.append(v)
    return out


def parse_scalar(v):
    """Strip a single layer of YAML quoting from a scalar value."""
    v = v.strip()
    if len(v) >= 2 and v[0] == '"' and v[-1] == '"':
        inner_v = v[1:-1]
        decoded = []
        j = 0
        while j < len(inner_v):
            ch = inner_v[j]
            if ch == '\\' and j + 1 < len(inner_v):
                decoded.append(inner_v[j + 1])
                j += 2
                continue
            decoded.append(ch)
            j += 1
        return ''.join(decoded)
    if len(v) >= 2 and v[0] == "'" and v[-1] == "'":
        return v[1:-1].replace("''", "'")
    return v


def extract_frontmatter(text):
    """Return raw frontmatter string (between leading --- markers) or None."""
    if not text.startswith('---'):
        return None
    # Find closing --- on its own line.
    lines = text.splitlines()
    if not lines or lines[0].strip() != '---':
        return None
    for idx in range(1, len(lines)):
        if lines[idx].strip() == '---':
            return '\n'.join(lines[1:idx])
    return None


def parse_fm(text):
    fm = extract_frontmatter(text)
    if fm is None:
        return {}
    if HAVE_YAML:
        try:
            data = yaml.safe_load(fm) or {}
            if isinstance(data, dict):
                return data
        except Exception:
            pass
    # Fallback: only extract `title:` and `aliases:` (the keys we actually need).
    out = {}
    for line in fm.splitlines():
        if line.startswith('title:'):
            out['title'] = parse_scalar(line[len('title:'):].strip())
        elif line.startswith('aliases:'):
            rest = line[len('aliases:'):].strip()
            if rest.startswith('['):
                out['aliases'] = parse_flow_list(rest)
            else:
                out['aliases'] = []
    return out


slug_lines = []
alias_lines = []
title_lines = []

for root, _dirs, files in os.walk(content_dir):
    for name in sorted(files):
        if not name.endswith('.md'):
            continue
        if name == '_index.md':
            continue
        path = os.path.join(root, name)
        rel = os.path.relpath(path, content_dir)
        slug = name[:-3]  # strip .md
        slug_lines.append(f'{slug}\t{rel}\n')
        try:
            with open(path, 'r', encoding='utf-8') as f:
                text = f.read()
        except OSError:
            continue
        fm = parse_fm(text)
        title = fm.get('title')
        if isinstance(title, str) and title:
            title_lines.append(f'{slug}\t{title}\n')
        aliases = fm.get('aliases') or []
        if isinstance(aliases, list):
            for a in aliases:
                if not isinstance(a, str):
                    continue
                a = a.strip()
                if not a:
                    continue
                alias_lines.append(f'{a}\t{slug}\n')

with open(slug_map_path, 'w', encoding='utf-8') as f:
    f.writelines(slug_lines)
with open(alias_map_path, 'w', encoding='utf-8') as f:
    f.writelines(alias_lines)
with open(title_map_path, 'w', encoding='utf-8') as f:
    f.writelines(title_lines)
PY

if [[ "$MAPS_ONLY" -eq 1 ]]; then
  echo "BUILD-OK|maps-only=1"
  exit 0
fi

# Atomic rebuild: write to a sibling temp dir then swap into place. This
# avoids races where `hugo server`'s file watcher trips over a transient
# missing `_index.md` while the rebuild is mid-flight (poll-1s mode is
# the worst offender; entr/fswatch are also vulnerable on big builds).
BUILD_TMP="$BUILD_DIR.tmp"
rm -rf "$BUILD_TMP"
mkdir -p "$BUILD_TMP"

while IFS= read -r -d '' page; do
  rel="${page#"$CONTENT_DIR"/}"
  out="$BUILD_TMP/$rel"
  mkdir -p "$(dirname "$out")"

  python3 - "$page" "$out" "$SLUG_MAP" "$ALIAS_MAP" "$TITLE_MAP" <<'PY'
import re, sys, os
src, out, slug_map, alias_map, title_map = sys.argv[1:6]

def load(path):
    d = {}
    with open(path, 'r', encoding='utf-8') as f:
        for line in f:
            line = line.rstrip('\n')
            if '\t' in line:
                k, v = line.split('\t', 1)
                d[k] = v
    return d

slugs = load(slug_map)
aliases = load(alias_map)
titles = load(title_map)

with open(src, 'r', encoding='utf-8') as f:
    text = f.read()


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
    base = os.path.splitext(os.path.basename(rel))[0]
    url = '/' + (section + '/' if section else '') + base + '/'
    return f'[{display}]({url})'


WIKILINK_RE = re.compile(r'\[\[([^\]]+)\]\]')


def rewrite_outside_code(s):
    """Apply WIKILINK_RE.sub only to segments that are NOT inside fenced code
    blocks or inline code spans. Fenced blocks are delimited by lines whose
    first non-space chars are ``` or ~~~ (with optional info string). Inline
    code is delimited by matched runs of backticks on the same line."""
    out = []
    lines = s.split('\n')
    in_fence = False
    fence_char = ''
    fence_len = 0
    fence_re = re.compile(r'^(\s*)(`{3,}|~{3,})(.*)$')
    for line in lines:
        m = fence_re.match(line)
        if not in_fence and m:
            in_fence = True
            fence_char = m.group(2)[0]
            fence_len = len(m.group(2))
            out.append(line)
            continue
        if in_fence:
            # Closing fence: same char, length >= opening, and nothing after the ticks but whitespace.
            if m and m.group(2)[0] == fence_char and len(m.group(2)) >= fence_len and m.group(3).strip() == '':
                in_fence = False
            out.append(line)
            continue
        # Outside fences: split inline code spans, only rewrite non-code parts.
        out.append(rewrite_inline(line))
    return '\n'.join(out)


def rewrite_inline(line):
    """Walk the line; when we hit a run of N backticks, find the matching
    closing run of N backticks and skip the span. Outside spans, run wikilink
    substitution."""
    parts = []
    i = 0
    n = len(line)
    buf = []
    while i < n:
        if line[i] == '`':
            # flush buffer with substitution
            if buf:
                parts.append(WIKILINK_RE.sub(repl, ''.join(buf)))
                buf = []
            # measure tick run
            j = i
            while j < n and line[j] == '`':
                j += 1
            run_len = j - i
            tick = '`' * run_len
            # find matching closing run
            close = line.find(tick, j)
            if close == -1:
                # unmatched: treat rest as literal (no wikilink rewrite)
                parts.append(line[i:])
                i = n
                break
            parts.append(line[i:close + run_len])
            i = close + run_len
        else:
            buf.append(line[i])
            i += 1
    if buf:
        parts.append(WIKILINK_RE.sub(repl, ''.join(buf)))
    return ''.join(parts)


text = rewrite_outside_code(text)

with open(out, 'w', encoding='utf-8') as f:
    f.write(text)
PY
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Atomic swap: replace BUILD_DIR with BUILD_TMP in two cheap rename ops.
# rename(2) is atomic per directory entry; the brief window where
# BUILD_DIR doesn't exist is sub-millisecond and shorter than any
# watcher's poll interval.
BUILD_OLD="$BUILD_DIR.old.$$"
if [[ -d "$BUILD_DIR" ]]; then
  mv "$BUILD_DIR" "$BUILD_OLD"
fi
mv "$BUILD_TMP" "$BUILD_DIR"
rm -rf "$BUILD_OLD" 2>/dev/null || true

echo "BUILD-OK|content=$CONTENT_DIR|build=$BUILD_DIR"

if [[ "$FULL" -eq 1 ]]; then
  # Render Vega-Lite chart sidecars before Hugo runs (no-op if data layer off
  # or vl-convert missing).
  if [[ -f .awiki/config ]] && grep -q '^AWIKI_DATA_LAYER=on' .awiki/config; then
    bash scripts/query.sh render || echo "BUILD|WARN|query-render returned non-zero"
    bash scripts/query.sh fence-render || echo "BUILD|WARN|query-fence-render returned non-zero"
    bash scripts/chart.sh render || echo "BUILD|WARN|charts-render returned non-zero"
  fi
  hugo --minify --destination public
  echo "HUGO-OK|out=public"
fi
