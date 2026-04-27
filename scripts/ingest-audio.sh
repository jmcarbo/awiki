#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: ingest-audio.sh <audio-path-under-raw/inbox/>" >&2; exit 1; }
AUDIO="$1"

[[ -f "$AUDIO" ]] || { echo "not found: $AUDIO" >&2; exit 1; }
[[ "$AUDIO" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }
command -v whisper-cpp >/dev/null 2>&1 || { echo "Install whisper.cpp" >&2; exit 1; }
[[ -f models/ggml-base.en.bin ]] || { echo "missing models/ggml-base.en.bin (download from whisper.cpp releases)" >&2; exit 1; }

OUT="${AUDIO%.*}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$AUDIO" "$ORIG_DIR/$(basename "$AUDIO")"

whisper-cpp -m models/ggml-base.en.bin -otxt -f "$AUDIO"
mv "${AUDIO}.txt" "$OUT"

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [audio]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$AUDIO")" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$AUDIO")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$AUDIO"
echo "AUDIO-TRANSCRIBED|out=$OUT|orig=$ORIG_DIR/$(basename "$AUDIO")"
echo "Now run: just ingest $OUT"
