# Data Layer — Charts Implementation Plan (Plan 2 of 2)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the chart half of the awiki data layer: Vega-Lite specs in inline ```vega-lite``` fences and dedicated `type: chart` pages, rendered live in Hugo and as SVG sidecars in Obsidian, with C-code lint rules and a `list_charts` MCP tool.

**Architecture:** A Python resolver (`vl-resolve.py`) is the single source of truth for `[[slug]]` → URL expansion; both the Hugo render-hook and the sidecar generator call it. A vendored vega-embed bundle is lazy-loaded by Hugo only on pages that contain a chart. For Obsidian, `chart.sh render` writes an `<!-- BEGIN chart-preview:<id> -->` managed region below each fence containing an image embed pointing at the SVG sidecar; users with the `obsidian-vega-lite` plugin (or Obsidian Charts) ignore the managed region in favor of live render.

**Tech Stack:** Hugo 0.120+ extended (render-hooks, shortcodes), Python 3.9+ (`vl-resolve.py`), `vl-convert` 1.x Rust binary (sidecar generator), Node.js 20+ (MCP server), bats-core 1.10+, just.

**Source spec:** `docs/superpowers/specs/2026-04-28-data-layer-vega-lite-design.md`. Implements §3.2 (render pipeline), §3.3 (resolver), §5 (chart subsystem), §6.2 (C-codes), §7 (`list_charts`), §8 (chart recipes), §9 (error handling), §10.1 (chart BATS), §10.3 (smoke), §10.4 (CI).

**Depends on:** Plan 1 (`2026-04-28-data-layer-datasets.md`). All datasets-as-pages infrastructure must be in place before starting Plan 2. The vega bundle and chart directories are added by extending `data-init.sh`, so wikis on Plan 1 only get the chart subsystem on a re-run of `just data-init` after Plan 2 lands.

---

## File Structure

**New files:**

```
scripts/lib/vl-resolve.py
scripts/lib/vendor-vega.sh
scripts/chart.sh
scripts/lint-chart.sh
layouts/_default/_markup/render-codeblock-vega-lite.html
layouts/shortcodes/vega-lite.html
mcp/awiki-server/lib/list-charts.js
mcp/awiki-server/test/list-charts.test.mjs
tests/vl_resolve_test.bats
tests/chart_render_test.bats
tests/chart_obsidian_preview_test.bats
tests/lint_chart_test.bats
tests/hugo_render_chart_test.bats
tests/data_layer_full_smoke_test.bats
tests/fixtures/data-layer/charts/inline-bar.json
tests/fixtures/data-layer/charts/broken-resolver.json
docs/data-help.txt                # extended (already exists from Plan 1)
.github/workflows/awiki-ci.yml.example   # extended
```

**Modified files:**

```
scripts/data-init.sh           # add chart dirs, vega bundle, git-crypt extension
scripts/templates/wiki-data-layer.md   # extend with chart docs (extends Plan 1 stub)
scripts/lint.sh                # source lint-chart.sh, --only=chart
scripts/build.sh               # invoke charts-render before hugo
scripts/check-deps.sh          # vl-convert advisory
justfile                       # charts-render, charts-render-one
mcp/awiki-server/index.js      # register list_charts
README.md                      # extend data-layer section with charts
```

`vl-resolve.py` and `lint-chart.sh` carry the bulk of the new logic. `chart.sh` is thin — orchestrates resolver + vl-convert + managed-region writer. The render-hook is a small Hugo template that calls a Hugo `partial` rendering the embed div + lazy script tag.

---

## Task 1: Extend `data-init.sh` with chart dirs + git-crypt patterns

**Files:**
- Modify: `scripts/data-init.sh`
- Modify: `tests/data_init_test.bats`
- Modify: `scripts/templates/wiki-data-layer.md`

- [ ] **Step 1: Add failing tests**

Append to `tests/data_init_test.bats`:

```bash
@test "data-init creates content/charts/ and assets/charts/ and static/vendor/vega/" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/charts ]
  [ -d assets/charts ]
  [ -d static/vendor/vega ]
}

@test "data-init appends AWIKI_CHART_OBSIDIAN_PREVIEW=on default" {
  run bash scripts/data-init.sh
  run grep -E '^AWIKI_CHART_OBSIDIAN_PREVIEW=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init extends git-crypt patterns when .gitattributes has git-crypt section" {
  printf '*.secret filter=git-crypt diff=git-crypt\n' > .gitattributes
  run bash scripts/data-init.sh <<<"y"
  run grep -F 'data/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  run grep -F 'assets/charts/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
}

@test "data-init does not duplicate git-crypt patterns on re-run" {
  printf '*.secret filter=git-crypt diff=git-crypt\n' > .gitattributes
  bash scripts/data-init.sh <<<"y"
  bash scripts/data-init.sh <<<"y"
  run grep -c -F 'data/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [[ "$output" == "1" ]]
}

@test "data-init skips git-crypt extension when no git-crypt section" {
  printf '*.txt text\n' > .gitattributes
  run bash scripts/data-init.sh
  run grep -F 'data/private/**' .gitattributes
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/data_init_test.bats
```

Expected: 5 new fail.

- [ ] **Step 3: Extend `step_dirs` and `step_config` in `scripts/data-init.sh`**

Replace `step_dirs`:

```bash
step_dirs() {
  for d in content/datasets content/charts data assets/charts static/vendor/vega; do
    if [[ ! -d "$d" ]]; then
      mkdir -p "$d"
      note "created $d"
    else
      note "skip $d (exists)"
    fi
  done
}
```

Add to `step_config` after the existing `_ensure_kv` calls:

```bash
  _ensure_kv "AWIKI_CHART_OBSIDIAN_PREVIEW" "on"
```

Add a new step `step_encryption`:

```bash
step_encryption() {
  local ga=".gitattributes"
  if [[ ! -f "$ga" ]]; then
    note "skip encryption (no .gitattributes)"
    return 0
  fi
  if ! grep -q 'filter=git-crypt diff=git-crypt' "$ga"; then
    note "skip encryption (no git-crypt section)"
    return 0
  fi
  local need_data=0 need_charts=0
  grep -q -F 'data/private/** filter=git-crypt diff=git-crypt' "$ga" || need_data=1
  grep -q -F 'assets/charts/private/** filter=git-crypt diff=git-crypt' "$ga" || need_charts=1
  if [[ $need_data -eq 0 && $need_charts -eq 0 ]]; then
    note "skip encryption (already covered)"
    return 0
  fi
  echo "Add 'data/private/**' and 'assets/charts/private/**' to git-crypt patterns? (Y/n) "
  local reply
  read -r reply || reply=""
  case "$reply" in
    n|N|no|NO) note "user declined git-crypt extension"; return 0 ;;
  esac
  [[ $need_data -eq 1 ]]   && printf -- '\ndata/private/** filter=git-crypt diff=git-crypt\n' >> "$ga"
  [[ $need_charts -eq 1 ]] && printf -- 'assets/charts/private/** filter=git-crypt diff=git-crypt\n' >> "$ga"
  note "added git-crypt patterns for data + charts"
}
```

Update `main()`:

```bash
main() {
  note "start"
  step_dirs
  step_wiki_md
  step_encryption
  step_config
  note "done"
}
```

- [ ] **Step 4: Update `scripts/templates/wiki-data-layer.md`**

Replace the existing template body with the chart-aware version (Plan 1 stub gets superseded):

````markdown
<!-- BEGIN data-layer -->
## Data Layer (opt-in)

This block is managed by `scripts/data-init.sh`. To remove the layer,
delete everything between the BEGIN and END markers and run
`bash scripts/lint.sh` to surface broken references.

### Page kinds

- `dataset` — structured rows with optional schema. See Plan 1 docs.
- `chart` — Vega-Lite spec, embeddable via shortcode or `![[slug]]`.

### Inline charts

Drop a fenced block in any markdown page:

```vega-lite
{
  "mark": "bar",
  "data": {"name": "[[us-pop-by-state]]"},
  "encoding": {
    "x": {"field": "state", "type": "nominal"},
    "y": {"field": "pop", "type": "quantitative"}
  }
}
```

The resolver expands `data.name: "[[slug]]"` to a dataset URL (file
storage) or `values: [...]` (inline storage). Raw `data.url` and inline
`data.values` pass through.

Sidecar SVGs render below the fence inside `<!-- BEGIN chart-preview:<id> -->` /
`<!-- END chart-preview:<id> -->` markers for plain-Obsidian preview.
Suppress with `AWIKI_CHART_OBSIDIAN_PREVIEW=off`.

### Recipes

| Recipe | Purpose |
|--------|---------|
| `just data-init` | Scaffold (idempotent). |
| `just dataset-new <slug> [--format=csv] [--from=<path>]` | New dataset page. |
| `just dataset-compact <slug>` | Flip inline ↔ file at threshold. |
| `just dataset-validate <slug>` | Recount rows + schema-check. |
| `just chart-new <slug> --data=<dataset-slug>` | Scaffold a chart page. |
| `just charts-render` | Walk all charts, regen stale SVGs. |
| `just charts-render-one <chart-id>` | Single chart fast iteration. |

### Lint codes (data layer)

D1–D9 — datasets (see Plan 1 docs).

C1 error — fence body fails JSON parse.
C2 error — spec missing `mark` / `layer` / `hconcat` / `vconcat` / `facet`.
C3 error — `data.name: "[[slug]]"` resolves to non-existent / non-dataset page.
C4 error — spec field not in target dataset's `columns:` (when both declared).
C5 error — `type: chart` page with `chart_engine: vega-lite` has no fence.
C6 warn — `chart_data:` frontmatter pointer disagrees with body. `--fix`-able.
C7 warn — sidecar `assets/charts/<id>.svg` missing or hash-stale.
C8 info — chart references same dataset >5 times across the wiki.
C9 warn — hand-edit detected inside `<!-- BEGIN chart-preview:* -->`.
C-PRIV error — chart in non-private page references private dataset.

### MCP tools

- `list_datasets()` — Plan 1.
- `get_dataset(slug)` — Plan 1.
- `list_charts()` — every fence + every `type: chart` page.
<!-- END data-layer -->
````

(`step_wiki_md` from Plan 1 already refreshes the block on re-run; no change to that step.)

- [ ] **Step 5: Run, verify pass**

```bash
bats tests/data_init_test.bats
```

Expected: all pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/data-init.sh scripts/templates/wiki-data-layer.md tests/data_init_test.bats
git commit -m "feat(data-init): chart dirs + git-crypt extension + obsidian-preview flag"
```

---

## Task 2: `scripts/lib/vendor-vega.sh` + invoke from `data-init.sh`

**Files:**
- Create: `scripts/lib/vendor-vega.sh`
- Modify: `scripts/data-init.sh`
- Modify: `tests/data_init_test.bats`

The vendored bundle is fetched once by `data-init.sh`. Pinned versions are written into `scripts/lib/vendor-vega.sh` so renaming/upgrade is traceable.

- [ ] **Step 1: Add failing test**

Append to `tests/data_init_test.bats`:

```bash
@test "data-init drops vendored vega bundle in static/vendor/vega when missing" {
  # Simulate cached bundle (no network in CI sandbox).
  export AWIKI_VEGA_VENDOR_LOCAL_DIR="$REPO_ROOT/tests/fixtures/data-layer/vendor-vega-cache"
  mkdir -p "$AWIKI_VEGA_VENDOR_LOCAL_DIR"
  printf 'vega.min.js' > "$AWIKI_VEGA_VENDOR_LOCAL_DIR/vega.min.js"
  printf 'vega-lite.min.js' > "$AWIKI_VEGA_VENDOR_LOCAL_DIR/vega-lite.min.js"
  printf 'vega-embed.min.js' > "$AWIKI_VEGA_VENDOR_LOCAL_DIR/vega-embed.min.js"
  REPO_ROOT="$REPO_ROOT" AWIKI_VEGA_VENDOR_LOCAL_DIR="$AWIKI_VEGA_VENDOR_LOCAL_DIR" \
    run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -f static/vendor/vega/vega.min.js ]
  [ -f static/vendor/vega/vega-lite.min.js ]
  [ -f static/vendor/vega/vega-embed.min.js ]
  rm -rf "$AWIKI_VEGA_VENDOR_LOCAL_DIR"
}

@test "data-init skips vega bundle download when files already exist" {
  mkdir -p static/vendor/vega
  printf 'cached' > static/vendor/vega/vega.min.js
  printf 'cached' > static/vendor/vega/vega-lite.min.js
  printf 'cached' > static/vendor/vega/vega-embed.min.js
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run cat static/vendor/vega/vega.min.js
  [[ "$output" == "cached" ]]
}
```

- [ ] **Step 2: Create `scripts/lib/vendor-vega.sh`**

```bash
#!/usr/bin/env bash
# scripts/lib/vendor-vega.sh — fetch vendored vega + vega-lite + vega-embed.
# Pinned versions; bump together. Re-run is idempotent.

set -euo pipefail

VEGA_VERSION="5.30.0"
VEGA_LITE_VERSION="5.21.0"
VEGA_EMBED_VERSION="6.27.0"
OUT_DIR="${1:-static/vendor/vega}"
mkdir -p "$OUT_DIR"

CDN_BASE="https://cdn.jsdelivr.net/npm"

_fetch() {
  local url="$1" dest="$2"
  if [[ -f "$dest" ]]; then
    echo "VENDOR-VEGA|skip $dest (exists)"
    return 0
  fi
  if [[ -n "${AWIKI_VEGA_VENDOR_LOCAL_DIR:-}" ]]; then
    local name
    name="$(basename "$dest")"
    local local_src="$AWIKI_VEGA_VENDOR_LOCAL_DIR/$name"
    if [[ -f "$local_src" ]]; then
      cp "$local_src" "$dest"
      echo "VENDOR-VEGA|copied $name from local cache"
      return 0
    fi
  fi
  if ! command -v curl >/dev/null 2>&1; then
    echo "VENDOR-VEGA|ERROR curl missing; cannot fetch $url" >&2
    return 1
  fi
  curl --fail --silent --show-error -L "$url" -o "$dest"
  echo "VENDOR-VEGA|fetched $dest"
}

_fetch "$CDN_BASE/vega@$VEGA_VERSION/build/vega.min.js"             "$OUT_DIR/vega.min.js"
_fetch "$CDN_BASE/vega-lite@$VEGA_LITE_VERSION/build/vega-lite.min.js" "$OUT_DIR/vega-lite.min.js"
_fetch "$CDN_BASE/vega-embed@$VEGA_EMBED_VERSION/build/vega-embed.min.js" "$OUT_DIR/vega-embed.min.js"

cat > "$OUT_DIR/VERSIONS.txt" <<EOF
vega=$VEGA_VERSION
vega-lite=$VEGA_LITE_VERSION
vega-embed=$VEGA_EMBED_VERSION
fetched=$(date -u '+%Y-%m-%dT%H:%M:%SZ')
EOF
echo "VENDOR-VEGA|done"
```

- [ ] **Step 3: Wire into `step_dirs` of `data-init.sh`**

Add a new step after `step_dirs`:

```bash
step_vendor_vega() {
  if [[ -f "static/vendor/vega/vega.min.js" \
     && -f "static/vendor/vega/vega-lite.min.js" \
     && -f "static/vendor/vega/vega-embed.min.js" ]]; then
    note "skip vega vendor (all files present)"
    return 0
  fi
  if [[ ! -x scripts/lib/vendor-vega.sh ]]; then
    chmod +x scripts/lib/vendor-vega.sh
  fi
  bash scripts/lib/vendor-vega.sh static/vendor/vega || warn "vega vendor failed"
}
```

Update `main()`:

```bash
main() {
  note "start"
  step_dirs
  step_vendor_vega
  step_wiki_md
  step_encryption
  step_config
  note "done"
}
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/data_init_test.bats
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/vendor-vega.sh scripts/data-init.sh tests/data_init_test.bats
git commit -m "feat(data-init): vendor vega + vega-lite + vega-embed (idempotent)"
```

---

## Task 3: `vl-resolve.py` — wikilink expander

**Files:**
- Create: `scripts/lib/vl-resolve.py`
- Create: `tests/vl_resolve_test.bats`
- Create: `tests/fixtures/data-layer/charts/inline-bar.json`
- Create: `tests/fixtures/data-layer/charts/broken-resolver.json`

The resolver is the spine of chart rendering. Used by Hugo render-hook AND `chart.sh render`. Single source of truth for `[[slug]]` → URL/values rewriting.

- [ ] **Step 1: Create fixtures**

`tests/fixtures/data-layer/charts/inline-bar.json`:

```json
{
  "mark": "bar",
  "data": {"name": "[[demo]]"},
  "encoding": {
    "x": {"field": "year", "type": "ordinal"},
    "y": {"field": "pop", "type": "quantitative"}
  }
}
```

`tests/fixtures/data-layer/charts/broken-resolver.json`:

```json
{
  "mark": "bar",
  "data": {"name": "[[no-such-slug]]"},
  "encoding": {"x": {"field": "year"}}
}
```

- [ ] **Step 2: Write failing tests**

`tests/vl_resolve_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/data" "$WORK/.awiki"
  pushd "$WORK" >/dev/null
  # Build a file-storage dataset.
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" data/demo.csv
  cat > content/datasets/demo.md <<'EOF'
---
title: "demo"
type: dataset
storage: file
format: csv
rows: 3
data_path: data/demo.csv
---

## Provenance
EOF
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "vl-resolve expands [[slug]] to data.url for file storage" {
  cp "$REPO_ROOT/tests/fixtures/data-layer/charts/inline-bar.json" spec.json
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"url":'*'/data/demo.csv'* ]]
  [[ "$output" != *'[[demo]]'* ]]
  [[ "$output" == *'"format"'*'"csv"'* ]]
}

@test "vl-resolve embeds values for inline storage" {
  cat > content/datasets/inline.md <<'EOF'
---
title: "inline"
type: dataset
storage: inline
format: csv
rows: 2
---

## Data

```csv
year,pop
2020,331
2021,333
```
EOF
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"name":"[[inline]]"},"encoding":{"x":{"field":"year"}}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"values"'* ]]
  [[ "$output" == *'"2020"'* ]]
}

@test "vl-resolve passes through raw data.url" {
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"url":"/some/path.csv"},"encoding":{}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"/some/path.csv"'* ]]
}

@test "vl-resolve passes through inline data.values" {
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"values":[{"a":1}]}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"values"'* ]]
  [[ "$output" == *'"a"'* ]]
}

@test "vl-resolve fails on unknown slug" {
  cp "$REPO_ROOT/tests/fixtures/data-layer/charts/broken-resolver.json" spec.json
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -ne 0 ]
  [[ "$output" == *"RESOLVE|"*"no-such-slug"*"not_found"* ]]
}

@test "vl-resolve fails when slug points to non-dataset" {
  cat > content/concepts/notdata.md <<'EOF'
---
type: concept
---
EOF
  mkdir -p content/concepts
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"name":"[[notdata]]"}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -ne 0 ]
  [[ "$output" == *"not_a_dataset"* ]]
}

@test "vl-resolve recurses into layer + hconcat + vconcat" {
  cat > spec.json <<'EOF'
{
  "hconcat": [
    {"mark":"bar","data":{"name":"[[demo]]"}},
    {"layer":[{"mark":"line","data":{"name":"[[demo]]"}}]}
  ]
}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  # Two resolutions => two urls in output.
  count="$(grep -c '/data/demo.csv' <<<"$output")"
  [[ "$count" == "2" ]]
}
```

- [ ] **Step 3: Run, verify failures**

```bash
bats tests/vl_resolve_test.bats
```

Expected: all fail.

- [ ] **Step 4: Implement `scripts/lib/vl-resolve.py`**

```python
#!/usr/bin/env python3
"""Resolve `[[slug]]` references in a Vega-Lite spec to dataset URLs / values.

Usage:
  vl-resolve.py --spec=<path> --base-url=<base> [--repo-root=<dir>]

Exits non-zero on resolution failure with a `RESOLVE|<src>|<slug>|<reason>`
line on stdout (so callers can capture it).
"""
import argparse
import csv
import json
import re
import sys
from pathlib import Path

DATA_NAME_RE = re.compile(r"^\[\[([a-z0-9][a-z0-9-]*)\]\]$")
FRONTMATTER_RE = re.compile(r"^---\s*\n(.*?)\n---\s*$", re.M | re.S)


def _read_dataset_page(repo_root: Path, slug: str):
    page = repo_root / "content" / "datasets" / f"{slug}.md"
    if not page.exists():
        return None
    text = page.read_text(encoding="utf-8")
    m = FRONTMATTER_RE.search(text)
    if not m:
        return {"_no_fm": True}
    fm = {}
    for line in m.group(1).splitlines():
        kv = re.match(r"^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$", line)
        if not kv:
            continue
        k, raw = kv.group(1), kv.group(2).strip()
        if raw.startswith('"') and raw.endswith('"'):
            raw = raw[1:-1]
        fm[k] = raw
    fm["_text"] = text
    return fm


def _extract_inline(text: str, fmt: str) -> str:
    fence = re.compile(r"## Data[\s\S]*?```" + re.escape(fmt) + r"\s*\n(.*?)\n```", re.S)
    m = fence.search(text)
    return m.group(1) if m else ""


def _parse_csv(body: str, delim: str):
    return list(csv.DictReader(body.splitlines(), delimiter=delim))


def _to_values(body: str, fmt: str):
    if fmt == "csv":
        return _parse_csv(body, ",")
    if fmt == "tsv":
        return _parse_csv(body, "\t")
    if fmt == "dsv":
        first = body.splitlines()[0] if body else ""
        d = next((c for c in (",", ";", "|", "\t") if c in first), ",")
        return _parse_csv(body, d)
    if fmt == "json":
        d = json.loads(body)
        return d if isinstance(d, list) else [d]
    if fmt == "topojson":
        return [json.loads(body)]
    return []


def _resolve_data_block(data: dict, repo_root: Path, base_url: str, src: str):
    name = data.get("name") if isinstance(data, dict) else None
    if not isinstance(name, str):
        return data  # pass through `url`, `values`, or already-resolved blocks.
    m = DATA_NAME_RE.match(name)
    if not m:
        return data  # `name` exists but isn't `[[slug]]` syntax — leave to Vega-Lite.
    slug = m.group(1)
    fm = _read_dataset_page(repo_root, slug)
    if fm is None:
        print(f"RESOLVE|{src}|{slug}|not_found", flush=True)
        sys.exit(2)
    if fm.get("_no_fm") or fm.get("type") != "dataset":
        print(f"RESOLVE|{src}|{slug}|not_a_dataset", flush=True)
        sys.exit(3)
    storage = fm.get("storage")
    fmt = fm.get("format")
    if storage == "file":
        path = fm.get("data_path", f"data/{slug}.{fmt}")
        url = base_url.rstrip("/") + "/" + path.lstrip("/")
        return {"url": url, "format": {"type": fmt}}
    if storage == "inline":
        body = _extract_inline(fm["_text"], fmt)
        return {"values": _to_values(body, fmt)}
    print(f"RESOLVE|{src}|{slug}|bad_storage", flush=True)
    sys.exit(4)


def _walk(node, repo_root: Path, base_url: str, src: str):
    if isinstance(node, dict):
        if "data" in node and isinstance(node["data"], dict):
            node["data"] = _resolve_data_block(node["data"], repo_root, base_url, src)
        for k, v in list(node.items()):
            node[k] = _walk(v, repo_root, base_url, src)
        return node
    if isinstance(node, list):
        return [_walk(item, repo_root, base_url, src) for item in node]
    return node


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--spec", required=True)
    p.add_argument("--base-url", default="/")
    p.add_argument("--repo-root", default=".")
    p.add_argument("--src", default="(unknown)", help="Source page path for diagnostics.")
    args = p.parse_args()

    repo_root = Path(args.repo_root).resolve()
    spec = json.loads(Path(args.spec).read_text(encoding="utf-8"))
    resolved = _walk(spec, repo_root, args.base_url, args.src)
    print(json.dumps(resolved, indent=2))


if __name__ == "__main__":
    main()
```

- [ ] **Step 5: Run, verify pass**

```bash
bats tests/vl_resolve_test.bats
```

Expected: 7 tests pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/lib/vl-resolve.py tests/vl_resolve_test.bats tests/fixtures/data-layer/charts/
git commit -m "feat(data): vl-resolve.py — wikilink-to-url expander for vega-lite specs"
```

---

## Task 4: `chart.sh new` — scaffold `type: chart` page

**Files:**
- Create: `scripts/chart.sh`
- Create: `tests/chart_new_test.bats`

- [ ] **Step 1: Write failing tests**

`tests/chart_new_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts"
  cat > "$WORK/content/datasets/demo.md" <<'EOF'
---
title: "demo"
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a,b
```
EOF
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "chart new --data scaffolds a type:chart page with skeleton fence" {
  run bash scripts/chart.sh new demo-bar --data=demo
  [ "$status" -eq 0 ]
  [ -f content/charts/demo-bar.md ]
  run grep -E '^type: chart$' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -E '^chart_engine: vega-lite$' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -F 'chart_data: "[[demo]]"' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -F '```vega-lite' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -F '"name": "[[demo]]"' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
}

@test "chart new --data refuses unknown dataset slug" {
  run bash scripts/chart.sh new x --data=missing
  [ "$status" -ne 0 ]
  [[ "$output" == *"dataset not found"* ]]
}

@test "chart new refuses existing slug" {
  bash scripts/chart.sh new demo-bar --data=demo
  run bash scripts/chart.sh new demo-bar --data=demo
  [ "$status" -ne 0 ]
  [[ "$output" == *"already exists"* ]]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/chart_new_test.bats
```

Expected: all fail.

- [ ] **Step 3: Implement `scripts/chart.sh`**

```bash
#!/usr/bin/env bash
set -euo pipefail

# scripts/chart.sh — manage Vega-Lite chart pages and rendered SVG sidecars.
# Subcommands: new, render, render-one.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

CHARTS_DIR="content/charts"
DATASETS_DIR="content/datasets"
ASSETS_DIR="assets/charts"
RESOLVE_PY="$REPO_ROOT/scripts/lib/vl-resolve.py"
SLUG_RE='^[a-z0-9][a-z0-9-]*$'

note() { echo "CHART|$*"; }
die() { echo "CHART|ERROR|$*" >&2; exit 1; }

cmd_new() {
  local slug="" data=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --data=*) data="${1#--data=}" ;;
      --) shift; break ;;
      -*) die "unknown flag: $1" ;;
      *) slug="$1" ;;
    esac
    shift
  done
  [[ -n "$slug" ]] || die "missing <slug>"
  [[ "$slug" =~ $SLUG_RE ]] || die "invalid slug: $slug"
  [[ -n "$data" ]] || die "--data=<dataset-slug> required"
  [[ "$data" =~ $SLUG_RE ]] || die "invalid dataset slug: $data"
  local page="$CHARTS_DIR/$slug.md"
  [[ ! -e "$page" ]] || die "$page already exists"
  [[ -f "$DATASETS_DIR/$data.md" ]] || die "dataset not found: $data"
  mkdir -p "$CHARTS_DIR"

  local today
  today="$(date '+%Y-%m-%d')"
  cat > "$page" <<EOF
---
title: "$slug"
date: $today
last_updated: $today
type: chart
chart_engine: vega-lite
chart_data: "[[$data]]"
sources: ["[[$data]]"]
draft: false
---

# $slug

<!-- one-paragraph description here -->

\`\`\`vega-lite
{
  "mark": "bar",
  "data": {"name": "[[$data]]"},
  "encoding": {
    "x": {"field": "<x-field>", "type": "nominal"},
    "y": {"field": "<y-field>", "type": "quantitative"}
  }
}
\`\`\`

## Notes

## Related
EOF
  note "created $page"
}

cmd_render() { die "render not implemented yet (Task 5)"; }
cmd_render_one() { die "render-one not implemented yet (Task 6)"; }

main() {
  [[ $# -ge 1 ]] || die "usage: chart.sh <new|render|render-one> [args...]"
  local cmd="$1"; shift
  case "$cmd" in
    new) cmd_new "$@" ;;
    render) cmd_render "$@" ;;
    render-one) cmd_render_one "$@" ;;
    *) die "unknown subcommand: $cmd" ;;
  esac
}

main "$@"
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/chart_new_test.bats
```

Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/chart.sh tests/chart_new_test.bats
git commit -m "feat(chart): new — scaffold type:chart page with skeleton spec"
```

---

## Task 5: `chart.sh render` — sidecar pipeline

**Files:**
- Modify: `scripts/chart.sh`
- Create: `tests/chart_render_test.bats`

`render` walks every `vega-lite` fence + every `type: chart` page, computes a stable chart-id, runs the resolver, hashes, calls `vl-convert` if stale, writes `<chart-id>.svg` + `<chart-id>.svg.hash`.

- [ ] **Step 1: Write failing tests**

`tests/chart_render_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts" "$WORK/data" "$WORK/assets/charts"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/data/demo.csv"
  cat > "$WORK/content/datasets/demo.md" <<'EOF'
---
type: dataset
storage: file
format: csv
rows: 3
data_path: data/demo.csv
---

## Provenance
EOF
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

# `vl-convert` is required for these tests; skip when missing.
need_vlconvert() {
  if ! command -v vl-convert >/dev/null 2>&1; then
    skip "vl-convert binary not installed"
  fi
}

@test "render produces SVG for inline fence" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

# intro

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
\`\`\`
EOF
  mkdir -p content/concepts
  run bash scripts/chart.sh render
  [ "$status" -eq 0 ]
  [ -f assets/charts/intro-fig0.svg ]
  [ -f assets/charts/intro-fig0.svg.hash ]
}

@test "render is no-op when hash matches (idempotent)" {
  need_vlconvert
  cat > content/charts/demo-bar.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
\`\`\`
EOF
  bash scripts/chart.sh render
  before="$(stat -f '%m' assets/charts/demo-bar.svg 2>/dev/null || stat -c '%Y' assets/charts/demo-bar.svg)"
  sleep 1
  bash scripts/chart.sh render
  after="$(stat -f '%m' assets/charts/demo-bar.svg 2>/dev/null || stat -c '%Y' assets/charts/demo-bar.svg)"
  [[ "$before" == "$after" ]]
}

@test "render cleans orphan sidecars" {
  need_vlconvert
  : > assets/charts/orphan-fig0.svg
  : > assets/charts/orphan-fig0.svg.hash
  bash scripts/chart.sh render
  [ ! -f assets/charts/orphan-fig0.svg ]
  [ ! -f assets/charts/orphan-fig0.svg.hash ]
}

@test "render skips with warning when vl-convert missing" {
  # Force vl-convert off by overriding PATH.
  PATH="/usr/bin:/bin" run env -i HOME="$HOME" PATH="/usr/bin:/bin" bash scripts/chart.sh render
  [[ "$output" == *"vl-convert"*"missing"* ]]
}

@test "render fails fast on broken resolver" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[no-such-slug]]"}}
\`\`\`
EOF
  mkdir -p content/concepts
  run bash scripts/chart.sh render
  [ "$status" -ne 0 ]
  [[ "$output" == *"RESOLVE|"*"no-such-slug"* ]]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/chart_render_test.bats
```

Expected: most fail (or skip if vl-convert missing).

- [ ] **Step 3: Implement `cmd_render`**

Replace the stub in `scripts/chart.sh`:

```bash
# Helper: extract every ```vega-lite``` fence body from a markdown page.
# Emits TSV: <chart-id>\t<base64-spec> (base64 encoding preserves JSON
# backslash escapes that printf %b would otherwise mangle).
_extract_fences() {
  local page="$1"
  python3 - "$page" <<'PY'
import sys, re, base64
from pathlib import Path
page = sys.argv[1]
text = Path(page).read_text(encoding="utf-8")
slug = Path(page).stem
fences = list(re.finditer(r"```vega-lite\s*\n(.*?)\n```", text, re.S))
fm_match = re.search(r"^---\s*\n(.*?)\n---\s*$", text, re.M | re.S)
is_chart_page = fm_match and re.search(r"^type:\s*chart\s*$", fm_match.group(1), re.M)
for idx, m in enumerate(fences):
    cid = slug if (is_chart_page and len(fences) == 1) else f"{slug}-fig{idx}"
    body_b64 = base64.b64encode(m.group(1).encode("utf-8")).decode("ascii")
    print(cid + "\t" + body_b64)
PY
}

_walk_charts() { # emit `<page>\t<chart-id>\t<spec-tmpfile>` per chart
  local page chart_id body_b64
  while IFS= read -r -d '' page; do
    while IFS=$'\t' read -r chart_id body_b64; do
      [[ -z "$chart_id" ]] && continue
      local tmp
      tmp="$(mktemp)"
      printf '%s' "$body_b64" | base64 --decode > "$tmp"
      printf '%s\t%s\t%s\n' "$page" "$chart_id" "$tmp"
    done < <(_extract_fences "$page")
  done < <(find content -type f -name '*.md' -print0)
}

_render_one() {
  local page="$1" chart_id="$2" spec_tmp="$3"
  local resolved hash existing_hash sidecar
  sidecar="$ASSETS_DIR/$chart_id.svg"
  resolved="$(python3 "$RESOLVE_PY" --spec="$spec_tmp" --base-url=/ --repo-root="$REPO_ROOT" --src="$page")"
  if [[ $? -ne 0 ]]; then
    return 2
  fi
  hash="$(printf '%s' "$resolved" | shasum -a 1 | awk '{print $1}')"
  existing_hash=""
  [[ -f "$sidecar.hash" ]] && existing_hash="$(cat "$sidecar.hash")"
  if [[ -f "$sidecar" && "$hash" == "$existing_hash" ]]; then
    note "skip $chart_id (hash match)"
    return 0
  fi
  local resolved_tmp
  resolved_tmp="$(mktemp)"
  printf '%s' "$resolved" > "$resolved_tmp"
  if vl-convert vl2svg --input "$resolved_tmp" --output "$sidecar" 2>/tmp/vlc.err; then
    printf '%s' "$hash" > "$sidecar.hash"
    rm -f "$sidecar.failed"
    note "rendered $chart_id"
  else
    printf '%s\n' "$(cat /tmp/vlc.err)" > "$sidecar.failed"
    note "RENDER|$chart_id|$(cat /tmp/vlc.err | head -c 200)"
  fi
  rm -f "$resolved_tmp"
}

cmd_render() {
  local keep_orphans=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --keep-orphans) keep_orphans=1; shift ;;
      *) shift ;;
    esac
  done
  if ! command -v vl-convert >/dev/null 2>&1; then
    note "vl-convert missing — skipping sidecar render (install via cargo or pre-built release)"
    return 0
  fi
  mkdir -p "$ASSETS_DIR"
  local current_ids=()
  local rc=0
  while IFS=$'\t' read -r page chart_id spec_tmp; do
    [[ -z "$chart_id" ]] && continue
    if ! _render_one "$page" "$chart_id" "$spec_tmp"; then
      rc=$?
    fi
    current_ids+=("$chart_id")
    rm -f "$spec_tmp"
  done < <(_walk_charts)

  if [[ $keep_orphans -eq 0 ]]; then
    while IFS= read -r -d '' f; do
      local base
      base="$(basename "$f")"
      base="${base%.svg}"
      base="${base%.hash}"
      base="${base%.failed}"
      local found=0
      for id in "${current_ids[@]:-}"; do
        [[ "$id" == "$base" ]] && { found=1; break; }
      done
      [[ $found -eq 0 ]] && rm -f "$f" && note "removed orphan $f"
    done < <(find "$ASSETS_DIR" -type f \( -name '*.svg' -o -name '*.svg.hash' -o -name '*.svg.failed' \) -print0)
  fi
  return $rc
}
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/chart_render_test.bats
```

Expected: tests pass (skip if vl-convert missing).

- [ ] **Step 5: Commit**

```bash
git add scripts/chart.sh tests/chart_render_test.bats
git commit -m "feat(chart): render — resolve + vl-convert sidecar pipeline (idempotent)"
```

---

## Task 6: `chart.sh render-one`

**Files:**
- Modify: `scripts/chart.sh`
- Modify: `tests/chart_render_test.bats`

- [ ] **Step 1: Add failing test**

Append to `tests/chart_render_test.bats`:

```bash
@test "render-one re-renders only the matching chart-id" {
  need_vlconvert
  mkdir -p content/concepts
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
EOF
  cat > content/concepts/other.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"line","data":{"name":"[[demo]]"}}
\`\`\`
EOF
  bash scripts/chart.sh render
  rm assets/charts/intro-fig0.svg
  bash scripts/chart.sh render-one intro-fig0
  [ -f assets/charts/intro-fig0.svg ]
}

@test "render-one fails when chart-id unknown" {
  need_vlconvert
  run bash scripts/chart.sh render-one nope
  [ "$status" -ne 0 ]
  [[ "$output" == *"unknown chart"* ]]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/chart_render_test.bats -f "render-one"
```

Expected: 2 fail.

- [ ] **Step 3: Implement `cmd_render_one`**

Replace stub in `scripts/chart.sh`:

```bash
cmd_render_one() {
  local target="${1:-}"
  [[ -n "$target" ]] || die "usage: chart.sh render-one <chart-id>"
  if ! command -v vl-convert >/dev/null 2>&1; then
    die "vl-convert missing"
  fi
  mkdir -p "$ASSETS_DIR"
  local found=0
  while IFS=$'\t' read -r page chart_id spec_tmp; do
    if [[ "$chart_id" == "$target" ]]; then
      found=1
      _render_one "$page" "$chart_id" "$spec_tmp" || exit $?
    fi
    rm -f "$spec_tmp"
  done < <(_walk_charts)
  [[ $found -eq 1 ]] || die "unknown chart: $target"
}
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/chart_render_test.bats
```

Expected: all pass (or skip).

- [ ] **Step 5: Commit**

```bash
git add scripts/chart.sh tests/chart_render_test.bats
git commit -m "feat(chart): render-one for fast single-chart iteration"
```

---

## Task 7: Obsidian managed-region preview block

**Files:**
- Modify: `scripts/chart.sh`
- Create: `tests/chart_obsidian_preview_test.bats`

After a successful sidecar write, `_render_one` must update the source page so an Obsidian-without-plugin reader sees an image preview below the fence. Plugin users honor `AWIKI_CHART_OBSIDIAN_PREVIEW=off` to suppress.

- [ ] **Step 1: Write failing tests**

`tests/chart_obsidian_preview_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts" "$WORK/content/concepts" "$WORK/data" "$WORK/assets/charts" "$WORK/.awiki"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/data/demo.csv"
  cat > "$WORK/content/datasets/demo.md" <<'EOF'
---
type: dataset
storage: file
format: csv
rows: 3
data_path: data/demo.csv
---

## Provenance
EOF
  printf 'AWIKI_CHART_OBSIDIAN_PREVIEW=on\n' > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }
need_vlconvert() {
  command -v vl-convert >/dev/null 2>&1 || skip "vl-convert missing"
}

@test "render injects chart-preview managed region below fence" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

# intro

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`

paragraph after.
EOF
  bash scripts/chart.sh render
  run grep -F '<!-- BEGIN chart-preview:intro-fig0 -->' content/concepts/intro.md
  [ "$status" -eq 0 ]
  run grep -F '<!-- END chart-preview:intro-fig0 -->' content/concepts/intro.md
  [ "$status" -eq 0 ]
  run grep -F 'assets/charts/intro-fig0.svg' content/concepts/intro.md
  [ "$status" -eq 0 ]
  # Region must come AFTER the closing fence and BEFORE the next paragraph.
  run grep -B0 -A2 -F '<!-- BEGIN chart-preview' content/concepts/intro.md
  [[ "$output" == *"![intro-fig0]"* ]]
}

@test "AWIKI_CHART_OBSIDIAN_PREVIEW=off leaves managed region empty" {
  need_vlconvert
  printf 'AWIKI_CHART_OBSIDIAN_PREVIEW=off\n' > .awiki/config
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
EOF
  bash scripts/chart.sh render
  run grep -F '<!-- BEGIN chart-preview:intro-fig0 -->' content/concepts/intro.md
  [ "$status" -eq 0 ]
  # Body between markers should be empty.
  run python3 - <<'PY'
import re, sys
txt = open("content/concepts/intro.md").read()
m = re.search(r"<!-- BEGIN chart-preview:intro-fig0 -->\n(.*?)\n<!-- END chart-preview:intro-fig0 -->", txt, re.S)
assert m, "no markers"
body = m.group(1).strip()
assert body == "", f"expected empty, got {body!r}"
print("ok")
PY
  [[ "$output" == "ok" ]]
}

@test "render is idempotent on managed region (no duplicate insertion)" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
EOF
  bash scripts/chart.sh render
  bash scripts/chart.sh render
  run grep -c -F '<!-- BEGIN chart-preview:intro-fig0 -->' content/concepts/intro.md
  [[ "$output" == "1" ]]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/chart_obsidian_preview_test.bats
```

- [ ] **Step 3: Implement preview-injection helper in `scripts/chart.sh`**

Add helper before `_render_one`:

```bash
_obsidian_preview_enabled() {
  local cfg="$REPO_ROOT/.awiki/config"
  if [[ -f "$cfg" ]] && grep -q '^AWIKI_CHART_OBSIDIAN_PREVIEW=off' "$cfg"; then
    return 1
  fi
  return 0
}

_inject_preview() {
  local page="$1" chart_id="$2"
  python3 - "$page" "$chart_id" "$ASSETS_DIR" "$REPO_ROOT" <<'PY'
import re, sys, os
page, cid, assets_dir, repo_root = sys.argv[1:5]
text = open(page).read()
begin = f"<!-- BEGIN chart-preview:{cid} -->"
end = f"<!-- END chart-preview:{cid} -->"

# Pick body (or empty if AWIKI_CHART_OBSIDIAN_PREVIEW=off).
preview_on = True
cfg = os.path.join(repo_root, ".awiki", "config")
if os.path.exists(cfg):
    with open(cfg) as f:
        if any(line.strip() == "AWIKI_CHART_OBSIDIAN_PREVIEW=off" for line in f):
            preview_on = False

if preview_on:
    # Path relative to the markdown file's directory.
    page_dir = os.path.dirname(os.path.relpath(page, repo_root))
    abs_sidecar = os.path.join(assets_dir, f"{cid}.svg")
    rel = os.path.relpath(abs_sidecar, os.path.join(repo_root, page_dir))
    body = f"![{cid}]({rel})"
else:
    body = ""

block = f"{begin}\n{body}\n{end}"

if begin in text:
    new = re.sub(re.escape(begin) + r"\n.*?\n" + re.escape(end), block, text, count=1, flags=re.S)
else:
    # Insert after the closing fence of THIS chart-id (need to find which).
    # We find the Nth ```vega-lite``` fence where N matches the chart-id suffix.
    # For chart pages, the fence is unique on page.
    if "-fig" in cid and cid.rsplit("-fig", 1)[1].isdigit():
        idx = int(cid.rsplit("-fig", 1)[1])
    else:
        idx = 0
    pattern = re.compile(r"(```vega-lite\s*\n.*?\n```)", re.S)
    matches = list(pattern.finditer(text))
    if idx >= len(matches):
        new = text  # nothing to anchor to
    else:
        m = matches[idx]
        insert_at = m.end()
        new = text[:insert_at] + "\n\n" + block + text[insert_at:]
open(page, "w").write(new)
PY
}
```

In `_render_one`, after a successful `vl-convert` write, call `_inject_preview "$page" "$chart_id"`. Specifically, replace the success path:

```bash
  if vl-convert vl2svg --input "$resolved_tmp" --output "$sidecar" 2>/tmp/vlc.err; then
    printf '%s' "$hash" > "$sidecar.hash"
    rm -f "$sidecar.failed"
    _inject_preview "$page" "$chart_id"
    note "rendered $chart_id"
```

Also extend `cmd_render`'s orphan-cleanup pass to remove stale managed regions. After the file orphan-cleanup loop, add:

```bash
  # Remove orphan managed regions inside content pages.
  while IFS= read -r -d '' page; do
    python3 - "$page" "${current_ids[@]:-_NONE_}" <<'PY'
import re, sys, os
page = sys.argv[1]
ids = set(sys.argv[2:])
text = open(page).read()
def _keep(m):
    cid = m.group(1)
    if cid in ids: return m.group(0)
    return ""
new = re.sub(r"<!-- BEGIN chart-preview:([a-z0-9][a-z0-9-]*) -->\n.*?\n<!-- END chart-preview:\1 -->", _keep, text, flags=re.S)
if new != text: open(page, "w").write(new)
PY
  done < <(find content -type f -name '*.md' -print0)
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/chart_obsidian_preview_test.bats
```

Expected: all pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/chart.sh tests/chart_obsidian_preview_test.bats
git commit -m "feat(chart): obsidian managed-region preview (toggleable + idempotent)"
```

---

## Task 8: `lint-chart.sh` C1, C2, C3 (parse + structure + resolver)

**Files:**
- Create: `scripts/lint-chart.sh`
- Create: `tests/lint_chart_test.bats`

- [ ] **Step 1: Write failing tests**

`tests/lint_chart_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts" "$WORK/content/concepts" "$WORK/.awiki"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "C1 fires on malformed JSON in fence" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{ this is not json
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C1|"* ]]
}

@test "C2 fires when spec lacks mark/layer/concat/facet" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"data": {"name": "[[demo]]"}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C2|"* ]]
}

@test "C3 fires on missing dataset" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[no-such]]"}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C3|"* ]]
  [[ "$output" == *"no-such"* ]]
}

@test "no C-codes on a clean inline chart" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 1
---

## Data

```csv
a,b
1,2
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" != *"|C1|"* ]]
  [[ "$output" != *"|C2|"* ]]
  [[ "$output" != *"|C3|"* ]]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/lint_chart_test.bats
```

- [ ] **Step 3: Implement `scripts/lint-chart.sh`**

```bash
#!/usr/bin/env bash
# scripts/lint-chart.sh — C-code chart linter. Sourced by lint.sh OR run standalone.

set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RESOLVE_PY="$REPO_ROOT/scripts/lib/vl-resolve.py"

: "${FIX:=0}"
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  while [[ ${1:-} == --* ]]; do
    case "$1" in
      --fix) FIX=1; shift ;;
      --) shift; break ;;
      *) shift ;;
    esac
  done
fi

_emit_c() { printf 'LINT|%s|%s|%s|%s\n' "$1" "$2" "$3" "$4"; }

_extract_fences_for_lint() { # <page> -> emits TSV: <chart-id>\t<base64-spec>
  python3 - "$1" <<'PY'
import sys, re, base64
from pathlib import Path
text = Path(sys.argv[1]).read_text(encoding="utf-8")
slug = Path(sys.argv[1]).stem
fm_match = re.search(r"^---\s*\n(.*?)\n---\s*$", text, re.M | re.S)
is_chart = fm_match and re.search(r"^type:\s*chart\s*$", fm_match.group(1), re.M)
fences = list(re.finditer(r"```vega-lite\s*\n(.*?)\n```", text, re.S))
for idx, m in enumerate(fences):
    cid = slug if (is_chart and len(fences) == 1) else f"{slug}-fig{idx}"
    body_b64 = base64.b64encode(m.group(1).encode("utf-8")).decode("ascii")
    print(f"{cid}\t{body_b64}")
PY
}

lint_chart_one() {
  local page="$1"
  local rel="${page#$REPO_ROOT/}"
  while IFS=$'\t' read -r chart_id body_b64; do
    [[ -z "$chart_id" ]] && continue
    local tmp
    tmp="$(mktemp)"
    printf '%s' "$body_b64" | base64 --decode > "$tmp"
    # C1 — JSON parse.
    if ! python3 -c "import json,sys; json.load(open(sys.argv[1]))" "$tmp" 2>/dev/null; then
      _emit_c error "$rel" C1 "fence body fails JSON parse (chart=$chart_id)"
      rm -f "$tmp"; continue
    fi
    # C2 — required keys.
    if ! python3 - "$tmp" <<'PY' >/dev/null 2>&1
import json,sys
spec = json.load(open(sys.argv[1]))
for k in ("mark","layer","hconcat","vconcat","facet","repeat"):
    if k in spec: sys.exit(0)
sys.exit(1)
PY
    then
      _emit_c error "$rel" C2 "spec missing mark/layer/hconcat/vconcat/facet/repeat (chart=$chart_id)"
    fi
    # C3 — resolver dry-run.
    local resolve_out
    resolve_out="$(python3 "$RESOLVE_PY" --spec="$tmp" --base-url=/ --repo-root="$REPO_ROOT" --src="$page" 2>&1 >/dev/null || true)"
    if [[ "$resolve_out" == *"RESOLVE|"* ]]; then
      while IFS= read -r line; do
        [[ "$line" == "RESOLVE|"* ]] || continue
        _emit_c error "$rel" C3 "$line"
      done <<<"$resolve_out"
    fi
    rm -f "$tmp"
  done < <(_extract_fences_for_lint "$page")
}

lint_chart_all() {
  if [[ ! -d "$REPO_ROOT/content" ]]; then return 0; fi
  while IFS= read -r -d '' page; do
    grep -q '^```vega-lite' "$page" || continue
    lint_chart_one "$page"
  done < <(find "$REPO_ROOT/content" -type f -name '*.md' -print0)
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  lint_chart_all
fi
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/lint_chart_test.bats
```

Expected: 4 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint-chart.sh tests/lint_chart_test.bats
git commit -m "feat(lint-chart): C1 parse, C2 structure, C3 resolver"
```

---

## Task 9: C4 (field check), C5 (chart page sanity), C6 (chart_data sync) + `--fix`

**Files:**
- Modify: `scripts/lint-chart.sh`
- Modify: `tests/lint_chart_test.bats`

- [ ] **Step 1: Add failing tests**

Append to `tests/lint_chart_test.bats`:

```bash
@test "C4 fires when spec field not in dataset columns" {
  cat > content/datasets/typed.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 1
columns:
  - { name: a, type: integer }
  - { name: b, type: integer }
---

## Data

```csv
a,b
1,2
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[typed]]"},"encoding":{"x":{"field":"missing"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C4|"* ]]
  [[ "$output" == *"missing"* ]]
}

@test "C5 fires on type:chart page with no fence" {
  cat > content/charts/empty.md <<'EOF'
---
type: chart
chart_engine: vega-lite
---

# empty chart page
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C5|"* ]]
}

@test "C6 fires when chart_data disagrees with body data.name" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/datasets/other.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/charts/c.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[other]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C6|"* ]]
}

@test "C6 --fix updates chart_data to match body" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/datasets/other.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/charts/c.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[other]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh --fix
  run grep -F 'chart_data: "[[other]]"' content/charts/c.md
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/lint_chart_test.bats
```

- [ ] **Step 3: Extend `lint-chart.sh`**

Inside the per-fence loop after C3, add:

```bash
    # C4 — field-in-columns check (only if both sides declare).
    python3 - "$tmp" "$REPO_ROOT" "$rel" "$chart_id" <<'PY' || true
import json, re, sys, os
spec_path, repo_root, rel, cid = sys.argv[1:5]
spec = json.load(open(spec_path))
data = spec.get("data") or {}
name = data.get("name", "")
m = re.match(r"^\[\[([a-z0-9][a-z0-9-]*)\]\]$", name)
if not m: sys.exit(0)
ds = os.path.join(repo_root, "content", "datasets", m.group(1) + ".md")
if not os.path.exists(ds): sys.exit(0)
text = open(ds).read()
fm = re.search(r"^---\s*\n(.*?)\n---\s*$", text, re.M | re.S)
if not fm: sys.exit(0)
cols = []
in_cols = False
for line in fm.group(1).splitlines():
    if line.startswith("columns:"): in_cols = True; continue
    if in_cols:
        if line and not line.startswith((" ", "\t")):
            in_cols = False; continue
        it = line.strip()
        if it.startswith("-"):
            body = it[1:].strip()
            if body.startswith("{") and body.endswith("}"):
                inner = body[1:-1]
                for part in inner.split(","):
                    if ":" in part:
                        k, v = part.split(":", 1)
                        if k.strip() == "name":
                            cols.append(v.strip().strip('"').strip("'"))
if not cols: sys.exit(0)

def fields(node):
    out = []
    if isinstance(node, dict):
        if isinstance(node.get("encoding"), dict):
            for ch, enc in node["encoding"].items():
                if isinstance(enc, dict) and isinstance(enc.get("field"), str):
                    out.append(enc["field"])
        for v in node.values():
            out.extend(fields(v))
    if isinstance(node, list):
        for v in node:
            out.extend(fields(v))
    return out

bad = [f for f in fields(spec) if f not in cols]
for f in bad:
    print(f"LINT|error|{rel}|C4|field '{f}' not in [[{m.group(1)}]] columns (chart={cid})")
PY
```

After the per-fence loop, walk chart pages for C5/C6:

```bash
lint_chart_pages() {
  local pages_dir="$REPO_ROOT/content/charts"
  [[ -d "$pages_dir" ]] || return 0
  while IFS= read -r -d '' page; do
    local rel="${page#$REPO_ROOT/}"
    # C5 — chart page must have at least one vega-lite fence when chart_engine=vega-lite.
    if grep -q '^chart_engine: *vega-lite' "$page" && ! grep -q '^```vega-lite' "$page"; then
      _emit_c error "$rel" C5 "chart_engine=vega-lite but no vega-lite fence"
      continue
    fi
    # C6 — chart_data vs body data.name.
    local declared body
    declared="$(awk '/^chart_data:/ { sub(/^chart_data: */, ""); gsub(/^"|"$/, ""); print; exit }' "$page")"
    body="$(python3 - "$page" <<'PY' 2>/dev/null || true
import re, json, sys
text = open(sys.argv[1]).read()
m = re.search(r"```vega-lite\s*\n(.*?)\n```", text, re.S)
if not m: sys.exit(0)
try:
    spec = json.loads(m.group(1))
except Exception:
    sys.exit(0)
data = spec.get("data") or {}
name = data.get("name", "")
if name: print(name)
PY
)"
    if [[ -n "$declared" && -n "$body" && "$declared" != "$body" ]]; then
      if [[ $FIX -eq 1 ]]; then
        sed -i.bak "s|^chart_data: .*|chart_data: \"$body\"|" "$page" && rm -f "$page.bak"
        _emit_c info "$rel" FIX "chart_data: $declared -> $body"
      else
        _emit_c warn "$rel" C6 "chart_data $declared disagrees with body $body"
      fi
    fi
  done < <(find "$pages_dir" -type f -name '*.md' -print0)
}
```

Update `lint_chart_all` to call `lint_chart_pages`:

```bash
lint_chart_all() {
  if [[ ! -d "$REPO_ROOT/content" ]]; then return 0; fi
  while IFS= read -r -d '' page; do
    grep -q '^```vega-lite' "$page" || continue
    lint_chart_one "$page"
  done < <(find "$REPO_ROOT/content" -type f -name '*.md' -print0)
  lint_chart_pages
}
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/lint_chart_test.bats
```

- [ ] **Step 5: Commit**

```bash
git add scripts/lint-chart.sh tests/lint_chart_test.bats
git commit -m "feat(lint-chart): C4 field, C5 empty chart page, C6 chart_data sync (+ --fix)"
```

---

## Task 10: C7 (sidecar staleness), C8 (info reuse), C9 (managed-region tamper)

**Files:**
- Modify: `scripts/lint-chart.sh`
- Modify: `tests/lint_chart_test.bats`

- [ ] **Step 1: Add failing tests**

Append:

```bash
@test "C7 warns when sidecar missing" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C7|"* ]]
}

@test "C8 info when same dataset referenced >5 times" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  for i in 1 2 3 4 5 6; do
    cat > content/concepts/p$i.md <<EOF
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  done
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C8|"* ]]
}

@test "C9 warns on hand-edit inside chart-preview managed region" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`

<!-- BEGIN chart-preview:p-fig0 -->
hand-written content here
extra line
<!-- END chart-preview:p-fig0 -->
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C9|"* ]]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/lint_chart_test.bats
```

- [ ] **Step 3: Extend `lint-chart.sh`**

Add to the per-fence loop in `lint_chart_one`, after C4:

```bash
    # C7 — sidecar presence.
    local sidecar="$REPO_ROOT/assets/charts/$chart_id.svg"
    if [[ ! -f "$sidecar" ]]; then
      _emit_c warn "$rel" C7 "sidecar missing for $chart_id (run charts-render)"
    fi
```

After `lint_chart_pages`, add a separate pass for C8 and C9:

```bash
lint_chart_aggregate() {
  # C8 — count [[<slug>]] references in vega-lite fences across all pages.
  python3 - "$REPO_ROOT" <<'PY' || true
import re, sys, os
root = sys.argv[1]
counts = {}
for dirpath, _, files in os.walk(os.path.join(root, "content")):
    for fn in files:
        if not fn.endswith(".md"): continue
        path = os.path.join(dirpath, fn)
        text = open(path).read()
        for fence in re.finditer(r"```vega-lite\s*\n(.*?)\n```", text, re.S):
            for m in re.finditer(r'"name"\s*:\s*"\[\[([a-z0-9][a-z0-9-]*)\]\]"', fence.group(1)):
                counts[m.group(1)] = counts.get(m.group(1), 0) + 1
for slug, n in counts.items():
    if n > 5:
        print(f"LINT|info|content/datasets/{slug}.md|C8|referenced {n} times across charts (consider type:chart page)")
PY
  # C9 — hand-edit inside chart-preview region.
  while IFS= read -r -d '' page; do
    local rel="${page#$REPO_ROOT/}"
    python3 - "$page" "$rel" "$REPO_ROOT" <<'PY' || true
import re, sys, os
page, rel, repo_root = sys.argv[1:4]
text = open(page).read()
for m in re.finditer(r"<!-- BEGIN chart-preview:([a-z0-9][a-z0-9-]*) -->\n(.*?)\n<!-- END chart-preview:\1 -->", text, re.S):
    cid = m.group(1)
    body = m.group(2).strip()
    # Allowed body: empty, or `![<cid>](<path-to-svg>)`.
    if body == "": continue
    expected_re = re.compile(r"^!\[" + re.escape(cid) + r"\]\([^)]+\)$")
    if expected_re.match(body): continue
    print(f"LINT|warn|{rel}|C9|hand-edit detected inside chart-preview:{cid} (re-run charts-render)")
PY
  done < <(find "$REPO_ROOT/content" -type f -name '*.md' -print0)
}
```

Update `lint_chart_all`:

```bash
lint_chart_all() {
  if [[ ! -d "$REPO_ROOT/content" ]]; then return 0; fi
  while IFS= read -r -d '' page; do
    grep -q '^```vega-lite' "$page" || continue
    lint_chart_one "$page"
  done < <(find "$REPO_ROOT/content" -type f -name '*.md' -print0)
  lint_chart_pages
  lint_chart_aggregate
}
```

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/lint_chart_test.bats
```

- [ ] **Step 5: Commit**

```bash
git add scripts/lint-chart.sh tests/lint_chart_test.bats
git commit -m "feat(lint-chart): C7 sidecar staleness, C8 reuse-info, C9 managed-region tamper"
```

---

## Task 11: C-PRIV (cross-privacy chart references)

**Files:**
- Modify: `scripts/lint-chart.sh`
- Modify: `tests/lint_chart_test.bats`

- [ ] **Step 1: Add failing test**

```bash
@test "C-PRIV blocks non-private chart referencing private dataset" {
  mkdir -p content/datasets/private
  cat > content/datasets/private/secret.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
tags: [private]
---

## Data

```csv
a
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[secret]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C-PRIV|"* ]]
}

@test "C-PRIV quiet when both sides private" {
  mkdir -p content/datasets/private content/private
  cat > content/datasets/private/secret.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
tags: [private]
---

## Data

```csv
a
```
EOF
  cat > content/private/p.md <<'EOF'
---
type: concept
tags: [private]
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[secret]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" != *"|C-PRIV|"* ]]
}
```

- [ ] **Step 2: Run, verify failures**

- [ ] **Step 3: Extend `lint-chart.sh`**

Add to the per-fence loop, after C7:

```bash
    # C-PRIV — non-private page referencing a tagged-private dataset.
    python3 - "$tmp" "$REPO_ROOT" "$page" "$rel" "$chart_id" <<'PY' || true
import json, re, sys, os
spec_path, repo_root, page, rel, cid = sys.argv[1:6]
spec = json.load(open(spec_path))

def collect_names(node, out):
    if isinstance(node, dict):
        d = node.get("data")
        if isinstance(d, dict) and isinstance(d.get("name"), str):
            m = re.match(r"^\[\[([a-z0-9][a-z0-9-]*)\]\]$", d["name"])
            if m: out.append(m.group(1))
        for v in node.values():
            collect_names(v, out)
    if isinstance(node, list):
        for v in node: collect_names(v, out)
names = []
collect_names(spec, names)
if not names: sys.exit(0)

# Is the host page private?
def is_private(text):
    fm = re.search(r"^---\s*\n(.*?)\n---\s*$", text, re.M | re.S)
    if not fm: return False
    return bool(re.search(r"^tags:\s*\[.*\bprivate\b.*\]", fm.group(1), re.M))

host_text = open(page).read()
host_private = is_private(host_text) or "/private/" in page

for slug in names:
    # Find the dataset page (private/ subdir or root datasets/).
    candidates = [
        os.path.join(repo_root, "content", "datasets", f"{slug}.md"),
        os.path.join(repo_root, "content", "datasets", "private", f"{slug}.md"),
    ]
    ds_path = next((c for c in candidates if os.path.exists(c)), None)
    if not ds_path: continue
    ds_text = open(ds_path).read()
    if is_private(ds_text) or "/private/" in ds_path:
        if not host_private:
            print(f"LINT|error|{rel}|C-PRIV|chart references private dataset [[{slug}]] from non-private page (chart={cid})")
PY
```

- [ ] **Step 4: Run, verify pass**

- [ ] **Step 5: Commit**

```bash
git add scripts/lint-chart.sh tests/lint_chart_test.bats
git commit -m "feat(lint-chart): C-PRIV cross-privacy guard"
```

---

## Task 12: Wire `lint-chart.sh` into `lint.sh`

**Files:**
- Modify: `scripts/lint.sh`
- Modify: `tests/lint_chart_test.bats`

- [ ] **Step 1: Add failing test**

```bash
@test "lint.sh --only=chart routes to lint-chart" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{ broken
\`\`\`
EOF
  run bash scripts/lint.sh --only=chart
  [[ "$output" == *"|C1|"* ]]
}

@test "lint.sh default mode includes C-codes" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{ broken
\`\`\`
EOF
  run bash scripts/lint.sh
  [[ "$output" == *"|C"*"|"* ]]
}
```

- [ ] **Step 2: Modify `scripts/lint.sh`**

Beside the existing `lint-data.sh` source block, add:

```bash
if [[ -f "$(dirname "$0")/lint-chart.sh" ]]; then
  # shellcheck disable=SC1091
  source "$(dirname "$0")/lint-chart.sh"
elif [[ -f scripts/lint-chart.sh ]]; then
  # shellcheck disable=SC1091
  source scripts/lint-chart.sh
fi
```

In the `--only` dispatch:

```bash
if [[ "$ONLY" == "chart" ]]; then
  lint_chart_all
  exit 0
fi
```

In the default flow (next to `lint_data_all` from Plan 1):

```bash
[[ -d "$CONTENT_DIR" ]] && lint_chart_all || true
```

- [ ] **Step 3: Run, verify pass**

```bash
bats tests/lint_chart_test.bats
```

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_chart_test.bats
git commit -m "feat(lint): route --only=chart to lint-chart.sh"
```

---

## Task 13: Hugo render-hook for ```vega-lite``` fences

**Files:**
- Create: `layouts/_default/_markup/render-codeblock-vega-lite.html`
- Create: `tests/hugo_render_chart_test.bats`

- [ ] **Step 1: Write failing test**

`tests/hugo_render_chart_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  command -v hugo >/dev/null 2>&1 || skip "hugo missing"
  command -v vl-convert >/dev/null 2>&1 || skip "vl-convert missing"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/layouts" "$WORK/layouts"
  cp -r "$REPO_ROOT/static" "$WORK/static" 2>/dev/null || mkdir -p "$WORK/static"
  cp -r "$REPO_ROOT/themes" "$WORK/themes"
  cp "$REPO_ROOT/hugo.toml" "$WORK/hugo.toml"
  mkdir -p "$WORK/content/datasets" "$WORK/content/concepts" "$WORK/data" "$WORK/.awiki/build-content"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/data/demo.csv"
  cat > "$WORK/content/datasets/demo.md" <<'EOF'
---
type: dataset
storage: file
format: csv
rows: 3
data_path: data/demo.csv
---

## Provenance
EOF
  cat > "$WORK/content/concepts/intro.md" <<'EOF'
---
title: "intro"
type: concept
---

# intro

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
\`\`\`
EOF
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "hugo build emits vega-embed div for vega-lite fence" {
  bash scripts/build.sh
  [ -f public/concepts/intro/index.html ] || [ -f public/concepts/intro.html ]
  out="$(find public -name 'intro*' -type f -name '*.html' | head -1)"
  run grep -F 'class="vega-embed"' "$out"
  [ "$status" -eq 0 ]
  run grep -F '"url":' "$out"
  [ "$status" -eq 0 ]
}

@test "hugo build injects vega-embed.min.js only on chart pages" {
  bash scripts/build.sh
  out="$(find public -name 'intro*' -type f -name '*.html' | head -1)"
  run grep -F 'vega-embed.min.js' "$out"
  [ "$status" -eq 0 ]
  # A page without charts must NOT get the script.
  cat > content/concepts/plain.md <<'EOF'
---
type: concept
---
plain page
EOF
  bash scripts/build.sh
  out="$(find public -name 'plain*' -type f -name '*.html' | head -1)"
  run grep -F 'vega-embed.min.js' "$out"
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run, verify failures**

```bash
bats tests/hugo_render_chart_test.bats
```

Expected: skip if hugo/vl-convert absent; otherwise fail.

- [ ] **Step 3: Implement the render-hook**

The hook emits the embed div PLUS the script tags inline on the first chart of each page (guarded by Hugo Scratch so we don't duplicate them when a page has multiple charts). This avoids clobbering theme `baseof.html`. Pages without a vega-lite fence never trigger the hook, so the bundle is only loaded where it's needed (lazy per Q7).

`layouts/_default/_markup/render-codeblock-vega-lite.html`:

```go-html-template
{{- $spec := .Inner -}}
{{- $idx  := .Ordinal -}}
{{- $slug := .Page.File.BaseFileName | default "page" -}}
{{- $isChartPage := eq .Page.Type "chart" -}}
{{- $cid := cond $isChartPage $slug (printf "%s-fig%d" $slug $idx) -}}
{{- $cachePath := printf "assets/charts/%s.json" $cid -}}
{{- $resolved := $spec -}}
{{- if fileExists $cachePath -}}
  {{- $resolved = readFile $cachePath -}}
{{- end -}}
<div class="vega-embed" id="{{ $cid }}" data-spec='{{ $resolved | safeJS }}'></div>
{{- if not (.Page.Scratch.Get "vega-embed-loaded") -}}
  {{- .Page.Scratch.Set "vega-embed-loaded" true -}}
<script src="{{ "/vendor/vega/vega.min.js" | relURL }}"></script>
<script src="{{ "/vendor/vega/vega-lite.min.js" | relURL }}"></script>
<script src="{{ "/vendor/vega/vega-embed.min.js" | relURL }}"></script>
<script>
(function(){
  function hydrate(){
    document.querySelectorAll('.vega-embed[data-spec]').forEach(function(el){
      if (el.dataset.hydrated) return;
      el.dataset.hydrated = '1';
      try {
        var spec = JSON.parse(el.getAttribute('data-spec'));
        vegaEmbed('#' + el.id, spec, {actions: false}).catch(function(err){
          el.innerHTML = '<pre class="vega-error">' + (err && err.message || err) + '</pre>';
        });
      } catch (e) {
        el.innerHTML = '<pre class="vega-error">' + e.message + '</pre>';
      }
    });
  }
  if (document.readyState === 'loading') {
    document.addEventListener('DOMContentLoaded', hydrate);
  } else { hydrate(); }
})();
</script>
{{- end -}}
```

This requires `chart.sh render` to also write `assets/charts/<cid>.json` (the resolved spec). Update `_render_one` in `chart.sh` — add this line right after `printf '%s' "$hash" > "$sidecar.hash"` in the success branch:

```bash
    # Cache the resolved spec next to the SVG for the Hugo render-hook.
    printf '%s' "$resolved" > "$ASSETS_DIR/$chart_id.json"
```

No `baseof.html` override, no partial — the theme is untouched.

- [ ] **Step 4: Run, verify pass**

```bash
bats tests/hugo_render_chart_test.bats
```

- [ ] **Step 5: Commit**

```bash
git add layouts/_default/_markup/render-codeblock-vega-lite.html scripts/chart.sh tests/hugo_render_chart_test.bats
git commit -m "feat(hugo): render-hook for vega-lite fences with lazy inline scripts"
```

---

## Task 14: `{{< vega-lite >}}` shortcode for `type: chart` page embeds

**Files:**
- Create: `layouts/shortcodes/vega-lite.html`
- Modify: `tests/hugo_render_chart_test.bats`

Allows `{{< vega-lite "demo-bar" >}}` to embed a `type: chart` page's spec inline on another page.

- [ ] **Step 1: Add failing test**

```bash
@test "shortcode embeds chart-page spec on another page" {
  cat > content/charts/demo-bar.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
\`\`\`
EOF
  cat > content/concepts/uses.md <<'EOF'
---
type: concept
title: "uses"
---

# uses

{{< vega-lite "demo-bar" >}}
EOF
  bash scripts/build.sh
  out="$(find public -name 'uses*' -type f -name '*.html' | head -1)"
  run grep -F 'class="vega-embed"' "$out"
  [ "$status" -eq 0 ]
  run grep -F 'id="demo-bar"' "$out"
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 2: Implement `layouts/shortcodes/vega-lite.html`**

```go-html-template
{{- $slug := .Get 0 -}}
{{- $cachePath := printf "assets/charts/%s.json" $slug -}}
{{- $resolved := "" -}}
{{- if fileExists $cachePath -}}
  {{- $resolved = readFile $cachePath -}}
{{- else -}}
  {{- /* Fall back to the source page's raw spec. */ -}}
  {{- $page := site.GetPage (printf "/charts/%s" $slug) -}}
  {{- with $page -}}
    {{- $body := .RawContent -}}
    {{- $re := `(?s)\x60\x60\x60vega-lite\s*\n(.*?)\n\x60\x60\x60` -}}
    {{- $matches := findRE $re $body 1 -}}
    {{- if $matches -}}
      {{- $resolved = index (findRESubmatch $re $body 1) 0 1 -}}
    {{- end -}}
  {{- end -}}
{{- end -}}
<div class="vega-embed" id="{{ $slug }}" data-spec='{{ $resolved | safeJS }}'></div>
{{- .Page.Scratch.Set "has-vega-chart" true -}}
```

(Hugo's `findRESubmatch` returns nested arrays; if the version in use doesn't support it, fall back to the cached path-only path. The cached `assets/charts/<slug>.json` is the primary supply.)

- [ ] **Step 3: Run, verify pass**

- [ ] **Step 4: Commit**

```bash
git add layouts/shortcodes/vega-lite.html tests/hugo_render_chart_test.bats
git commit -m "feat(hugo): {{< vega-lite >}} shortcode for chart-page embeds"
```

---

## Task 15: `just charts-render` + `just charts-render-one` + `just build` integration

**Files:**
- Modify: `justfile`
- Modify: `scripts/build.sh`

- [ ] **Step 1: Add failing tests**

Append to `tests/chart_render_test.bats`:

```bash
@test "just charts-render runs chart.sh render" {
  need_vlconvert
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
EOF
  mkdir -p content/concepts
  run just charts-render
  [ "$status" -eq 0 ]
  [ -f assets/charts/p-fig0.svg ]
}

@test "just charts-render-one runs chart.sh render-one" {
  need_vlconvert
  run just charts-render-one nope
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Add recipes**

Append to `justfile`:

```just
# Walk every vega-lite fence + type:chart page; regen stale SVG sidecars
# under assets/charts/. Uses scripts/lib/vendor-vega.sh if vendored bundle
# absent. Requires the `vl-convert` Rust binary.
charts-render:
    bash scripts/chart.sh render

# Single-chart regen for fast iteration. Argument is the chart-id
# (`<page-slug>-fig<N>` for inline charts, `<slug>` for type:chart pages).
charts-render-one chart_id:
    bash scripts/chart.sh render-one {{chart_id}}
```

- [ ] **Step 3: Wire into `scripts/build.sh`**

In `scripts/build.sh`, after the maps build but before `hugo`, add:

```bash
# Render Vega-Lite chart sidecars before Hugo runs (no-op if data layer off
# or vl-convert missing).
if [[ -f .awiki/config ]] && grep -q '^AWIKI_DATA_LAYER=on' .awiki/config; then
  bash scripts/chart.sh render || echo "BUILD|WARN|charts-render returned non-zero"
fi
```

- [ ] **Step 4: Run, verify pass**

- [ ] **Step 5: Commit**

```bash
git add justfile scripts/build.sh tests/chart_render_test.bats
git commit -m "feat(chart): just charts-render(*one) + build.sh integration"
```

---

## Task 16: MCP `list_charts`

**Files:**
- Create: `mcp/awiki-server/lib/list-charts.js`
- Create: `mcp/awiki-server/test/list-charts.test.mjs`
- Modify: `mcp/awiki-server/index.js`

- [ ] **Step 1: Write failing test**

`mcp/awiki-server/test/list-charts.test.mjs`:

```javascript
import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { listCharts } from "../lib/list-charts.js";

function setup() {
  const root = mkdtempSync(join(tmpdir(), "awiki-charts-"));
  mkdirSync(join(root, "content", "charts"), { recursive: true });
  mkdirSync(join(root, "content", "concepts"), { recursive: true });
  mkdirSync(join(root, "assets", "charts"), { recursive: true });
  return root;
}

test("listCharts returns empty on empty repo", () => {
  const root = setup();
  try {
    const r = listCharts(root);
    assert.deepEqual(r.charts, []);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("listCharts finds inline fences with fence_index", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "concepts", "p.md"), `---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`

\`\`\`vega-lite
{"mark":"line","data":{"name":"[[demo2]]"}}
\`\`\`
`);
    const r = listCharts(root);
    assert.equal(r.charts.length, 2);
    assert.equal(r.charts[0].fence_index, 0);
    assert.equal(r.charts[1].fence_index, 1);
    assert.equal(r.charts[0].engine, "vega-lite");
    assert.equal(r.charts[0].data_ref, "[[demo]]");
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("listCharts finds type:chart pages with slug", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "charts", "demo-bar.md"), `---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
`);
    const r = listCharts(root);
    assert.equal(r.charts.length, 1);
    assert.equal(r.charts[0].slug, "demo-bar");
    assert.equal(r.charts[0].fence_index, undefined);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("listCharts links sidecar_path when SVG exists", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "charts", "demo.md"), `---
type: chart
chart_engine: vega-lite
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
`);
    writeFileSync(join(root, "assets", "charts", "demo.svg"), "<svg/>");
    const r = listCharts(root);
    assert.equal(r.charts[0].sidecar_path, "assets/charts/demo.svg");
  } finally { rmSync(root, { recursive: true, force: true }); }
});
```

- [ ] **Step 2: Implement `mcp/awiki-server/lib/list-charts.js`**

```javascript
import { readdirSync, readFileSync, statSync, existsSync } from "node:fs";
import { join, relative, sep } from "node:path";

const FRONTMATTER_RE = /^---\s*\n([\s\S]*?)\n---\s*$/m;
const FENCE_RE = /```vega-lite\s*\n([\s\S]*?)\n```/g;
const NAME_RE = /"name"\s*:\s*"(\[\[[a-z0-9][a-z0-9-]*\]\])"/;

function* walk(dir) {
  let entries;
  try { entries = readdirSync(dir); } catch { return; }
  for (const e of entries) {
    const p = join(dir, e);
    let st;
    try { st = statSync(p); } catch { continue; }
    if (st.isDirectory()) yield* walk(p);
    else if (e.endsWith(".md")) yield p;
  }
}

function parseScalar(fm, key) {
  const re = new RegExp("^" + key + ":\\s*(.*)$", "m");
  const m = re.exec(fm);
  if (!m) return null;
  let v = m[1].trim();
  if (v.startsWith('"') && v.endsWith('"')) v = v.slice(1, -1);
  return v;
}

export function listCharts(repoRoot) {
  const charts = [];
  const errors = [];
  const contentDir = join(repoRoot, "content");
  if (!existsSync(contentDir)) return { charts, errors };

  for (const path of walk(contentDir)) {
    let text;
    try { text = readFileSync(path, "utf8"); } catch (e) {
      errors.push({ path, reason: String(e.message || e) });
      continue;
    }
    const fmMatch = FRONTMATTER_RE.exec(text);
    const fm = fmMatch ? fmMatch[1] : "";
    const isChartPage = /^type:\s*chart\s*$/m.test(fm);
    const fences = [...text.matchAll(FENCE_RE)];
    const slug = path.split(sep).pop().slice(0, -3);
    const rel = relative(repoRoot, path);

    fences.forEach((m, idx) => {
      const body = m[1];
      const refMatch = NAME_RE.exec(body);
      const dataRef = refMatch ? refMatch[1] : null;
      const isSingleChartPage = isChartPage && fences.length === 1;
      const cid = isSingleChartPage ? slug : `${slug}-fig${idx}`;
      const sidecarRel = `assets/charts/${cid}.svg`;
      const sidecarAbs = join(repoRoot, sidecarRel);
      const entry = {
        page: rel,
        engine: "vega-lite",
        data_ref: dataRef,
      };
      if (isSingleChartPage) {
        entry.slug = slug;
      } else {
        entry.fence_index = idx;
      }
      if (existsSync(sidecarAbs)) entry.sidecar_path = sidecarRel;
      charts.push(entry);
    });
  }
  return { charts, errors };
}
```

- [ ] **Step 3: Register in `index.js`**

In imports:

```javascript
import { listCharts } from "./lib/list-charts.js";
```

In `ListToolsRequestSchema` handler:

```javascript
{
  name: "list_charts",
  description: "Enumerate every vega-lite fence + every type:chart page.",
  inputSchema: { type: "object", properties: {}, additionalProperties: false },
},
```

In `CallToolRequestSchema` switch:

```javascript
case "list_charts": {
  const result = listCharts(REPO_ROOT);
  return { content: [{ type: "text", text: JSON.stringify(result) }] };
}
```

- [ ] **Step 4: Run, verify pass**

```bash
cd mcp/awiki-server && node --test test/list-charts.test.mjs
```

- [ ] **Step 5: Commit**

```bash
git add mcp/awiki-server/lib/list-charts.js mcp/awiki-server/test/list-charts.test.mjs mcp/awiki-server/index.js
git commit -m "feat(mcp): list_charts — enumerate fences + type:chart pages"
```

---

## Task 17: `check-deps.sh` vl-convert advisory

**Files:**
- Modify: `scripts/check-deps.sh`

- [ ] **Step 1: Add the warning**

In `scripts/check-deps.sh`, near the existing data-layer python3 advisory (Plan 1 Task 20), add:

```bash
if [[ -f .awiki/config ]] && grep -q '^AWIKI_DATA_LAYER=on' .awiki/config; then
  if ! command -v vl-convert >/dev/null 2>&1; then
    echo "WARN: data layer is on but vl-convert is missing — chart sidecars won't render"
    echo "      install: cargo install vl-convert OR download from https://github.com/vega/vl-convert/releases"
  fi
fi
```

- [ ] **Step 2: Manual verify**

```bash
bash scripts/check-deps.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/check-deps.sh
git commit -m "feat(check-deps): vl-convert advisory when data layer on"
```

---

## Task 18: Extend `docs/data-help.txt` + README with chart docs

**Files:**
- Modify: `docs/data-help.txt`
- Modify: `README.md`

- [ ] **Step 1: Append to `docs/data-help.txt`**

```
=== charts ===

just chart-new <slug> --data=<dataset-slug>
  Scaffold a content/charts/<slug>.md (type: chart) with a skeleton
  vega-lite spec referencing the named dataset.

just charts-render
  Walk every ```vega-lite``` fence + every type:chart page. For each:
    - resolve [[slug]] data references (scripts/lib/vl-resolve.py)
    - hash the resolved spec
    - skip if assets/charts/<id>.svg + .hash match
    - else call vl-convert vl2svg, write SVG + hash + cached resolved JSON
    - update Obsidian managed-region preview block in source page
  Cleans orphan sidecars + managed regions.

just charts-render-one <chart-id>
  Single-chart regen. <chart-id> is `<page-slug>-fig<N>` for inline charts
  or just `<slug>` for type:chart pages.

Hugo render-hook: ```vega-lite``` blocks render as <div class="vega-embed">,
hydrated by lazy-loaded vendored vega-embed.min.js.

Obsidian: install obsidian-vega-lite (or Obsidian Charts) to render live.
Without a plugin, the managed-region SVG sidecar shows in reading view.
Suppress sidecar injection: AWIKI_CHART_OBSIDIAN_PREVIEW=off in .awiki/config.

MCP tool: list_charts() -> { charts, errors }
```

- [ ] **Step 2: Append to README.md**

After the existing data-layer smoke section, add:

```markdown
### Charts (Plan 2)

Embed Vega-Lite charts in any page:

\`\`\`markdown
\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{...}}
\`\`\`
\`\`\`

The `[[demo]]` reference is resolved at build time to a URL (file storage)
or inline values (inline storage). Hugo renders interactive charts via
vendored vega-embed; Obsidian renders live with `obsidian-vega-lite` (or
Obsidian Charts) installed, otherwise via SVG sidecar previews.

Run `just charts-render` to regenerate sidecars; `just build` invokes it
automatically. Per-recipe usage: `just data-help`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/data-help.txt README.md
git commit -m "docs(data): chart docs in data-help.txt + README"
```

---

## Task 19: CI — vl-convert install step

**Files:**
- Modify: `.github/workflows/awiki-ci.yml.example` (or `scheduled/github-action.yml.example`)

- [ ] **Step 1: Inspect existing example**

```bash
cat .github/workflows/awiki-ci.yml.example 2>/dev/null || cat scheduled/github-action.yml.example
```

- [ ] **Step 2: Add a setup step**

Insert before the "Run tests" step:

```yaml
- name: Install vl-convert
  run: |
    set -e
    VERSION=v1.7.0
    curl -L "https://github.com/vega/vl-convert/releases/download/${VERSION}/vl-convert-x86_64-unknown-linux-gnu.tar.gz" \
      | tar xz -C /usr/local/bin --strip-components=1 vl-convert-x86_64-unknown-linux-gnu/vl-convert
    chmod +x /usr/local/bin/vl-convert
    vl-convert --version
```

- [ ] **Step 3: Commit**

```bash
git add scheduled/github-action.yml.example
git commit -m "ci: install vl-convert for chart-render tests"
```

---

## Task 20: End-to-end smoke test

**Files:**
- Create: `tests/data_layer_full_smoke_test.bats`

- [ ] **Step 1: Write the smoke test**

`tests/data_layer_full_smoke_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  command -v vl-convert >/dev/null 2>&1 || skip "vl-convert missing"
  command -v hugo >/dev/null 2>&1 || skip "hugo missing"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/layouts" "$WORK/layouts"
  cp -r "$REPO_ROOT/themes" "$WORK/themes"
  cp -r "$REPO_ROOT/static" "$WORK/static" 2>/dev/null || mkdir -p "$WORK/static"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/hugo.toml" "$WORK/hugo.toml"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  mkdir -p "$WORK/content"
  printf -- "---\ntitle: WIKI\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "full smoke: data-init -> dataset-new -> chart-new -> charts-render -> build -> lint" {
  bash scripts/data-init.sh
  [ -d content/datasets ]
  [ -d content/charts ]
  [ -d static/vendor/vega ]

  bash scripts/dataset.sh new demo --format=csv --from=demo.csv
  [ -f content/datasets/demo.md ]

  bash scripts/chart.sh new demo-bar --data=demo
  [ -f content/charts/demo-bar.md ]

  bash scripts/chart.sh render
  [ -f assets/charts/demo-bar.svg ]
  [ -f assets/charts/demo-bar.json ]

  bash scripts/build.sh
  [ -d public ]
  out="$(find public -name 'demo-bar*' -type f -name '*.html' | head -1)"
  [[ -n "$out" ]]
  run grep -F 'class="vega-embed"' "$out"
  [ "$status" -eq 0 ]

  run bash scripts/lint.sh
  # No errors expected; warnings (e.g. D9 missing sources) are OK.
  [[ "$output" != *"|error|"* ]]
}
```

- [ ] **Step 2: Run, verify pass**

```bash
bats tests/data_layer_full_smoke_test.bats
```

(Skips if vl-convert / hugo absent.)

- [ ] **Step 3: Commit**

```bash
git add tests/data_layer_full_smoke_test.bats
git commit -m "test(data): end-to-end full smoke (init -> chart -> hugo build -> lint)"
```

---

## Task 21: `just chart-new` recipe + `data-help` recipe update

**Files:**
- Modify: `justfile`
- Modify: `tests/chart_new_test.bats`

- [ ] **Step 1: Add failing test**

```bash
@test "just chart-new wrapper works" {
  run just chart-new demo-bar --data=demo
  [ "$status" -eq 0 ]
  [ -f content/charts/demo-bar.md ]
}
```

- [ ] **Step 2: Add recipe**

Append to `justfile`:

```just
# Scaffold a type:chart page that references an existing dataset.
chart-new slug *args:
    bash scripts/chart.sh new {{slug}} {{args}}
```

- [ ] **Step 3: Run, verify pass**

- [ ] **Step 4: Commit**

```bash
git add justfile tests/chart_new_test.bats
git commit -m "feat(chart): just chart-new recipe"
```

---

## Done — Plan 2 deliverables

After all 21 tasks, on top of Plan 1, the wiki has:

- Vendored vega + vega-lite + vega-embed bundle (lazy-loaded by Hugo).
- `[[slug]]` resolver shared by Hugo render-hook + sidecar pipeline.
- `type: chart` page kind with embedded `{{< vega-lite >}}` shortcode.
- `just chart-new`, `just charts-render`, `just charts-render-one`.
- `lint-chart.sh` with C1–C9 + C-PRIV codes, wired into `lint.sh --only=chart` + `--fix` for C6.
- Obsidian managed-region preview block (toggleable via `AWIKI_CHART_OBSIDIAN_PREVIEW`).
- MCP tool `list_charts` registered.
- `check-deps.sh` advisory for missing `vl-convert`.
- CI install step for `vl-convert`.
- README + `docs/data-help.txt` extended.
- End-to-end smoke covering full pipeline.

Both Plan 1 and Plan 2 ship as one feature branch (`feat/data-layer`) and merge together as a single user-visible feature ("datasets you can chart").
