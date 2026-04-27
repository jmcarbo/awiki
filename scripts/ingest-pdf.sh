#!/usr/bin/env bash
set -euo pipefail

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
