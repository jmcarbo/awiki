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
    rm -f "$tmp"
  done < <(_extract_fences_for_lint "$page")
}

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

lint_chart_all() {
  if [[ ! -d "$REPO_ROOT/content" ]]; then return 0; fi
  while IFS= read -r -d '' page; do
    grep -q '^```vega-lite' "$page" || continue
    lint_chart_one "$page"
  done < <(find "$REPO_ROOT/content" -type f -name '*.md' -print0)
  lint_chart_pages
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  lint_chart_all
fi
