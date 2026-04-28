#!/usr/bin/env bash
# scripts/lint-data.sh — D-code dataset linter.
# Sourced by lint.sh OR run standalone. Exits 0 always; emits LINT|<level>|<file>|<code>|<msg>.

set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DATASETS_DIR="$REPO_ROOT/content/datasets"
ROWS_PY="$REPO_ROOT/scripts/lib/dataset-rows.py"

# shellcheck source=/dev/null
[[ -f "$REPO_ROOT/scripts/lib/dataset-fm.sh" ]] && source "$REPO_ROOT/scripts/lib/dataset-fm.sh"

# When sourced from lint.sh, FIX may already be set by the caller.
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

_emit() { # _emit <level> <file> <code> <msg>
  local level="$1"
  # Uppercase to match lint.sh tally pattern (LINT|ERROR|... and LINT|WARN|...).
  level="$(printf '%s' "$level" | tr '[:lower:]' '[:upper:]')"
  printf 'LINT|%s|%s|%s|%s\n' "$level" "$2" "$3" "$4"
}

_fence_info() { # _fence_info <page>: print info-string of the fence directly under ## Data, or empty
  awk '
    /^## Data[[:space:]]*$/ { in_data=1; next }
    in_data && /^```/ {
      info=$0; sub(/^```/, "", info); sub(/[[:space:]]+$/, "", info); print info; exit
    }
  ' "$1"
}

_extract_inline_to_tmp() { # <page> <format>
  local page="$1" format="$2" out
  out="$(mktemp)"
  awk -v fmt="$format" '
    /^## Data[[:space:]]*$/ { in_data=1; next }
    in_data && match($0, "^```" fmt "[[:space:]]*$") { in_block=1; next }
    in_block && /^```[[:space:]]*$/ { exit }
    in_block { print }
  ' "$page" > "$out"
  echo "$out"
}

_columns_to_json_for_lint() {
  local page="$1" out="$2"
  python3 - "$page" "$out" <<'PY'
import re, sys, json
page, out = sys.argv[1], sys.argv[2]
txt = open(page).read()
m = re.search(r"^---\s*\n(.*?)\n---\s*$", txt, re.M | re.S)
cols=[]
if m:
    in_cols=False
    for line in m.group(1).splitlines():
        if line.startswith("columns:"):
            in_cols=True; continue
        if in_cols:
            if line and not line.startswith((" ", "\t")):
                in_cols=False; continue
            it=line.strip()
            if not it.startswith("-"): continue
            body=it[1:].strip()
            if body.startswith("{") and body.endswith("}"):
                inner=body[1:-1]; entry={}
                for part in inner.split(","):
                    if ":" not in part: continue
                    k,v=part.split(":",1)
                    entry[k.strip()]=v.strip().strip("\"").strip("'")
                cols.append(entry)
open(out,"w").write(json.dumps(cols))
PY
}

_lint_d5() { # <page> <format> <data-file>
  local page="$1" format="$2" data="$3"
  local rel="${page#$REPO_ROOT/}"
  local schema sample
  schema="$(mktemp)"
  _columns_to_json_for_lint "$page" "$schema"
  if [[ "$(cat "$schema")" == "[]" ]]; then rm -f "$schema"; return; fi
  # Build a sample of first 50 + last 10 rows to a temp file in the same format.
  sample="$(mktemp).$format"
  python3 - "$data" "$format" "$sample" <<'PY'
import csv, json, sys
src, fmt, dst = sys.argv[1:]
def open_csv(p, d):
    with open(p, newline="") as f: return list(csv.DictReader(f, delimiter=d))
def write_csv(rows, p, d):
    if not rows: open(p,"w").write(""); return
    with open(p,"w",newline="") as f:
        w=csv.DictWriter(f, fieldnames=list(rows[0].keys()), delimiter=d)
        w.writeheader(); w.writerows(rows)
if fmt=="csv": rows=open_csv(src, ","); write_csv(rows[:50]+rows[-10:], dst, ",")
elif fmt=="tsv": rows=open_csv(src, "\t"); write_csv(rows[:50]+rows[-10:], dst, "\t")
elif fmt=="dsv":
    first=open(src).readline()
    d=next((c for c in (",",";","|","\t") if c in first), ",")
    rows=open_csv(src, d); write_csv(rows[:50]+rows[-10:], dst, d)
elif fmt=="json":
    data=json.load(open(src))
    if isinstance(data, list): json.dump(data[:50]+data[-10:], open(dst,"w"))
    else: json.dump(data, open(dst,"w"))
elif fmt=="topojson":
    json.dump(json.load(open(src)), open(dst,"w"))
PY
  local out
  out="$(python3 "$ROWS_PY" validate --format="$format" --file="$sample" --schema="$schema" 2>&1)"
  if (( $? != 0 )); then
    while IFS= read -r line; do
      _emit error "$rel" D5 "$line"
    done <<<"$out"
  fi
  rm -f "$schema" "$sample"
}

lint_data_one() {
  local page="$1"
  local rel="${page#$REPO_ROOT/}"
  local storage format data_path
  storage="$(fm_get "$page" storage || true)"
  format="$(fm_get "$page" format || true)"
  data_path="$(fm_get "$page" data_path || true)"

  if [[ -z "$storage" ]]; then _emit error "$rel" D1 "missing storage"; return; fi
  if [[ -z "$format"  ]]; then _emit error "$rel" D1 "missing format";  return; fi

  local data_file=""
  if [[ "$storage" == "file" ]]; then
    if [[ -z "$data_path" ]]; then _emit error "$rel" D2 "storage=file but data_path missing"; return; fi
    local resolved
    if [[ "$data_path" = /* ]]; then resolved="$data_path"; else resolved="$REPO_ROOT/$data_path"; fi
    if [[ ! -f "$resolved" ]]; then _emit error "$rel" D2 "data file absent: $data_path"; return; fi
    data_file="$resolved"
  fi
  if [[ "$storage" == "inline" ]]; then
    local info
    info="$(_fence_info "$page")"
    if [[ -z "$info" ]]; then _emit error "$rel" D3 "storage=inline but no fenced block under ## Data"; return; fi
    if [[ "$info" != "$format" ]]; then _emit error "$rel" D4 "frontmatter format=$format != fence info=$info"; fi
    data_file="$(_extract_inline_to_tmp "$page" "$format")"
  fi
  if [[ -n "$data_file" && -s "$data_file" ]]; then
    _lint_d5 "$page" "$format" "$data_file"
  fi

  # Thresholds (D6).
  local cfg_rows=500 cfg_bytes=51200
  if [[ -f "$REPO_ROOT/.awiki/config" ]]; then
    # shellcheck disable=SC1091
    source <(grep -E '^AWIKI_DATASET_INLINE_MAX_(ROWS|BYTES)=' "$REPO_ROOT/.awiki/config" || true)
    cfg_rows="${AWIKI_DATASET_INLINE_MAX_ROWS:-500}"
    cfg_bytes="${AWIKI_DATASET_INLINE_MAX_BYTES:-51200}"
  fi

  if [[ -n "$data_file" && -s "$data_file" ]]; then
    local actual_rows
    actual_rows="$(python3 "$ROWS_PY" count --format="$format" --file="$data_file" 2>/dev/null || echo 0)"
    local declared_rows
    declared_rows="$(fm_get "$page" rows || echo 0)"
    declared_rows="${declared_rows:-0}"

    if [[ "$storage" == "inline" ]]; then
      local size
      size="$(wc -c < "$data_file")"
      if (( actual_rows > cfg_rows )) || (( size > cfg_bytes )); then
        _emit warn "$rel" D6 "inline dataset over threshold (rows=$actual_rows bytes=$size); run dataset-compact"
      fi
    fi

    if [[ "$declared_rows" != "$actual_rows" ]]; then
      if [[ $FIX -eq 1 ]]; then
        fm_set "$page" rows "$actual_rows"
        _emit info "$rel" FIX "rows: $declared_rows -> $actual_rows"
      else
        _emit warn "$rel" D7 "declared rows=$declared_rows but actual=$actual_rows (run with --fix)"
      fi
    fi
  fi

  # D8 — data_path inside data/ tree.
  if [[ "$storage" == "file" && -n "$data_path" ]]; then
    case "$data_path" in
      data/*|data/private/*) ;;
      *) _emit warn "$rel" D8 "data_path outside data/: $data_path" ;;
    esac
  fi

  # D9 — provenance check.
  if grep -q -E '^sources: *\[\] *$' "$page" || ! grep -q -E '^sources:' "$page"; then
    _emit info "$rel" D9 "no sources declared"
  fi

  [[ "$storage" == "inline" && -n "$data_file" ]] && rm -f "$data_file"
}

lint_data_all() {
  local rc=0
  if [[ ! -d "$DATASETS_DIR" ]]; then return 0; fi
  while IFS= read -r -d '' page; do
    lint_data_one "$page"
  done < <(find "$DATASETS_DIR" -type f -name '*.md' -print0)
  return $rc
}

# Run as standalone when invoked directly.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  lint_data_all
fi
