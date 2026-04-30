#!/usr/bin/env bash
set -euo pipefail

# Slice 6 of the Go ingest port: when bin/awiki is built and the user
# has not opted into the legacy bash path, exec the Go implementation.
# Mirrors scripts/ingest-audio.sh:9-16. Set AWIKI_INGEST_XLSX_LEGACY=1
# to force the original bash path (used by the bats oracle and parity
# smoke tests until cleanup in slice 10).
AWIKI_INGEST_XLSX_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AWIKI_INGEST_XLSX_REPO_ROOT="$(cd "$AWIKI_INGEST_XLSX_SCRIPT_DIR/.." && pwd)"
AWIKI_INGEST_XLSX_GO_BIN="$AWIKI_INGEST_XLSX_REPO_ROOT/bin/awiki"

if [[ "${AWIKI_INGEST_XLSX_LEGACY:-0}" != "1" && -x "$AWIKI_INGEST_XLSX_GO_BIN" ]]; then
  cd "$AWIKI_INGEST_XLSX_REPO_ROOT" || exit 1
  exec "$AWIKI_INGEST_XLSX_GO_BIN" ingest-xlsx "$@"
fi

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

# Strip a leading `./` so callers can supply tab-completed forms.
SRC="${SRC#./}"

# Reject any traversal segment so `raw/inbox/../etc/passwd` does not
# satisfy the textual `raw/inbox/*` gate below.
case "/$SRC/" in
  */../*) echo "XLSX-ERROR|reason=bad-path|src=$SRC" >&2; exit 2 ;;
esac

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
  echo "XLSX-ERROR|reason=missing-dep|dep=python-calamine" >&2
  echo "  install: pip3 install python-calamine" >&2
  exit 5
fi

BASE="$(basename "$SRC")"
STEM="${BASE%.*}"
SLUG="$(python3 "$SCRIPT_DIR/lib/xlsx-extract.py" --slugify "$STEM")"
[[ -n "$SLUG" ]] || { echo "XLSX-ERROR|reason=bad-slug|src=$SRC|stem=$STEM" >&2; exit 2; }

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
SHEETS_RC=0
SHEETS=$(python3 -c "import json,sys; print(len(json.loads(sys.argv[1])['sheets']))" "$MANIFEST") || SHEETS_RC=$?
if [[ "$SHEETS_RC" -ne 0 ]]; then
  echo "XLSX-ERROR|reason=bad-manifest|src=$SRC|rc=$SHEETS_RC" >&2
  bash "$SCRIPT_DIR/log-append.sh" xlsx "bad-manifest $BASE rc=$SHEETS_RC" >/dev/null 2>&1 || true
  exit "$SHEETS_RC"
fi
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
