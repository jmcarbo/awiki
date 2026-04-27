#!/usr/bin/env bash
# Mermaid post-hook for mindmap synthesis pages. Invoked ONLY when
# ALLOW_PLUGIN_POST_HOOKS=1 (gated by synth.sh; this script does not check
# the flag itself — that's the orchestrator's job).
#
# Usage: synth-mindmap-validate.sh -- <synthesis-page-path>
#
# Strategy:
#   1. Extract the mermaid block from the generated region.
#   2. If `mmdc` (mermaid-cli) is on PATH, run it in --dry-run / --quiet mode.
#   3. Otherwise, fall back to a regex sanity check (block starts with
#      `mindmap`, balanced ``` fences, no obviously broken syntax).
# Exit 0 on pass, non-zero on fail.

set -euo pipefail

# Strip a leading -- (argv terminator) if present.
[[ "${1:-}" = "--" ]] && shift

PAGE="${1:?usage: synth-mindmap-validate.sh -- <page>}"
[[ -f "$PAGE" ]] || { echo "ERROR: page not found: $PAGE" >&2; exit 2; }

# Sentinel for the gate test (task 14.1) — written only when this hook actually runs.
if [[ -n "${AWIKI_REPO_ROOT:-}" ]]; then
  mkdir -p "$AWIKI_REPO_ROOT/.awiki"
  : > "$AWIKI_REPO_ROOT/.awiki/post-hook-ran"
fi

# Extract the mermaid block from the generated region.
TMP=$(mktemp); trap 'rm -f "$TMP"' EXIT
awk '
  /^<!-- BEGIN GENERATED .* -->$/ { in_region=1; next }
  /^<!-- END GENERATED -->$/      { in_region=0 }
  in_region && /^```mermaid$/     { in_block=1; next }
  in_region && in_block && /^```$/ { in_block=0; next }
  in_region && in_block           { print }
' "$PAGE" > "$TMP"

if [[ ! -s "$TMP" ]]; then
  echo "ERROR: no mermaid block found in generated region of $PAGE" >&2
  exit 3
fi

# 1) prefer mmdc dry-run if installed
if command -v mmdc >/dev/null 2>&1; then
  if mmdc -i "$TMP" -o /dev/null --quiet >/dev/null 2>&1; then
    exit 0
  else
    echo "ERROR: mmdc rejected mermaid block in $PAGE" >&2
    exit 4
  fi
fi

# 2) regex sanity fallback
first_line=$(head -1 "$TMP")
if [[ "$first_line" != mindmap* ]]; then
  echo "ERROR: mermaid block must start with 'mindmap' (got: $first_line)" >&2
  exit 5
fi
# Balance brace check — Mermaid mindmap uses (), [], {{}} for shapes.
open_count=$(tr -dc '([{' < "$TMP" | wc -c | tr -d ' ')
close_count=$(tr -dc ')]}' < "$TMP" | wc -c | tr -d ' ')
if [[ "$open_count" -ne "$close_count" ]]; then
  echo "ERROR: unbalanced shape brackets in mermaid block ($open_count open, $close_count close)" >&2
  exit 6
fi

exit 0
