#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi

PREVIEW_ROWS="${AWIKI_XLSX_PREVIEW_ROWS:-50}"
ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --preview-rows) PREVIEW_ROWS="$2"; shift 2 ;;
    --preview-rows=*) PREVIEW_ROWS="${1#--preview-rows=}"; shift ;;
    -h|--help)
      cat <<'EOF'
usage: ingest-xlsx.sh <path-under-raw/inbox/> [--preview-rows N]
EOF
      exit 0 ;;
    --) shift; ARGS+=("$@"); break ;;
    *) ARGS+=("$1"); shift ;;
  esac
done
set -- "${ARGS[@]}"
[[ $# -eq 1 ]] || { echo "XLSX-ERROR|reason=usage" >&2; exit 2; }
SRC="$1"

# Normalize to repo-relative
if [[ "$SRC" = /* ]]; then
  case "$SRC" in
    "$REPO_ROOT"/*) SRC="${SRC#"$REPO_ROOT/"}" ;;
    *) echo "XLSX-ERROR|reason=bad-path|src=$SRC" >&2; exit 2 ;;
  esac
fi

case "$SRC" in
  raw/inbox/*) ;;
  *) echo "XLSX-ERROR|reason=bad-path|src=$SRC" >&2; exit 2 ;;
esac

[[ -f "$SRC" ]] || { echo "XLSX-ERROR|reason=not-found|src=$SRC" >&2; exit 1; }

case "$SRC" in
  *.xlsx|*.xls|*.ods) ;;
  *) echo "XLSX-ERROR|reason=bad-ext|src=$SRC" >&2; exit 2 ;;
esac

# Preflight: python-calamine. Honour AWIKI_FAKE_MISSING for tests.
if [[ "${AWIKI_FAKE_MISSING:-}" == "python_calamine" ]] || \
   ! python3 -c "import python_calamine" >/dev/null 2>&1; then
  echo "XLSX-ERROR|reason=missing-dep|dep=python-calamine"
  echo "  install: pip3 install python-calamine" >&2
  exit 5
fi

BASE="$(basename "$SRC")"
STEM="${BASE%.*}"
SLUG="$(python3 "$SCRIPT_DIR/lib/xlsx-extract.py" --slugify "$STEM")"

ORIG_DIR="raw/processed/_originals/$SLUG"
mkdir -p "$ORIG_DIR"
cp "$SRC" "$ORIG_DIR/$BASE"

OUT_DIR="$(dirname "$SRC")"
CSV_DIR="$ORIG_DIR"

EXTRACT_RC=0
MANIFEST="$(python3 "$SCRIPT_DIR/lib/xlsx-extract.py" \
  --in "$SRC" \
  --out-dir "$OUT_DIR" \
  --csv-dir "$CSV_DIR" \
  --csv-rel "$ORIG_DIR" \
  --original-rel "$ORIG_DIR/$BASE" \
  --slug-prefix "$SLUG" \
  --preview-rows "$PREVIEW_ROWS")" || EXTRACT_RC=$?

if [[ "$EXTRACT_RC" -ne 0 ]]; then
  echo "XLSX-ERROR|reason=extract|src=$SRC|rc=$EXTRACT_RC" >&2
  bash "$SCRIPT_DIR/log-append.sh" xlsx "failed $BASE rc=$EXTRACT_RC" >/dev/null 2>&1 || true
  exit "$EXTRACT_RC"
fi

# Manifest is single-line JSON on stdout.
SHEETS=$(python3 -c "import json,sys; print(len(json.loads(sys.argv[1])['sheets']))" "$MANIFEST")
if [[ "$SHEETS" -eq 0 ]]; then
  echo "XLSX-EMPTY|workbook=$SLUG|src=$SRC"
  bash "$SCRIPT_DIR/log-append.sh" xlsx "empty $BASE" >/dev/null 2>&1 || true
  exit 6
fi

# Per-sheet log lines.
python3 - "$MANIFEST" <<'PY'
import json, os, sys
m = json.loads(sys.argv[1])
for s in m["sheets"]:
    md_rel = os.path.relpath(s["md"])
    csv_rel = os.path.relpath(s["csv"])
    print(f"XLSX-SHEET|slug={s['slug']}|rows={s['rows_total']}|md={md_rel}|csv={csv_rel}")
PY

rm "$SRC"

echo "XLSX-CONVERTED|in=$SRC|workbook=$SLUG|sheets=$SHEETS|orig=$ORIG_DIR/$BASE"
python3 - "$MANIFEST" <<'PY'
import json, os, sys
m = json.loads(sys.argv[1])
for s in m["sheets"]:
    print(f"XLSX-NEXT|{os.path.relpath(s['md'])}")
PY

bash "$SCRIPT_DIR/log-append.sh" xlsx "converted $BASE sheets=$SHEETS" >/dev/null 2>&1 || true
exit 0
