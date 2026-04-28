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
