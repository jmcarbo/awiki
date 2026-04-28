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
