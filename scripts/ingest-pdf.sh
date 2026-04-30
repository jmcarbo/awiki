#!/usr/bin/env bash
set -euo pipefail

# Slice 4 of the Go ingest port: when bin/awiki is built and the user
# has not opted into the legacy bash path, exec the Go implementation.
# Mirrors scripts/capture.sh:14-21. Set AWIKI_INGEST_PDF_LEGACY=1 to
# force the original bash path (used by the bats oracle and parity
# smoke tests until cleanup in slice 10).
AWIKI_INGEST_PDF_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AWIKI_INGEST_PDF_REPO_ROOT="$(cd "$AWIKI_INGEST_PDF_SCRIPT_DIR/.." && pwd)"
AWIKI_INGEST_PDF_GO_BIN="$AWIKI_INGEST_PDF_REPO_ROOT/bin/awiki"

if [[ "${AWIKI_INGEST_PDF_LEGACY:-0}" != "1" && -x "$AWIKI_INGEST_PDF_GO_BIN" ]]; then
  cd "$AWIKI_INGEST_PDF_REPO_ROOT" || exit 1
  exec "$AWIKI_INGEST_PDF_GO_BIN" ingest-pdf "$@"
fi

[[ $# -eq 1 ]] || { echo "usage: ingest-pdf.sh <pdf-path-under-raw/inbox/>" >&2; exit 1; }
PDF="$1"

[[ "$PDF" =~ \.pdf$ ]] || { echo "not a pdf" >&2; exit 1; }
[[ -f "$PDF" ]] || { echo "not found: $PDF" >&2; exit 1; }
[[ "$PDF" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }

OUT="${PDF%.pdf}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$PDF" "$ORIG_DIR/$(basename "$PDF")"

if command -v pdftotext >/dev/null 2>&1; then
  pdftotext -layout "$PDF" - > "$OUT"
elif command -v marker >/dev/null 2>&1; then
  marker "$PDF" -o "$OUT"
else
  echo "Need pdftotext or marker installed" >&2
  exit 1
fi

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [pdf]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$PDF" .pdf)" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$PDF")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$PDF"

echo "PDF-CONVERTED|in=$PDF|out=$OUT|orig=$ORIG_DIR/$(basename "$PDF")"
echo "Now run: just ingest $OUT"
