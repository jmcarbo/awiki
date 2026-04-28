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
    resolve_out="$(python3 "$RESOLVE_PY" --spec="$tmp" --base-url=/ --repo-root="$REPO_ROOT" --src="$page" 2>/dev/null || true)"
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
