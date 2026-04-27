#!/usr/bin/env bash
set -euo pipefail

# scripts/synth-export-anki.sh — convert a study-guide synthesis page's
# ## Flashcards block into an Anki .apkg file. Graceful skip (exit 0 with
# stderr note) when the optional `genanki` Python dependency is absent.

PAGE="${1:-}"
if [[ -z "$PAGE" || ! -f "$PAGE" ]]; then
  echo "usage: synth-export-anki.sh <path-to-study-guide-page.md>" >&2
  exit 1
fi

if ! python3 -c 'import genanki' 2>/dev/null; then
  echo "genanki not installed; skipping Anki export. Install: pip install genanki" >&2
  exit 0
fi

mkdir -p .awiki/exports
SLUG="$(basename "$PAGE" .md)"
OUT=".awiki/exports/${SLUG}.apkg"

# Extract the Flashcards section into a temp file. Each card is a Q:/A: pair
# separated by --- on its own line. Feed the temp file to a Python helper
# (NOT python3 -c) — never build a -c invocation by string-interpolating page
# content; flashcards may contain user-supplied adversarial strings.
CARDS_TMP="$(mktemp)"
trap 'rm -f "$CARDS_TMP"' EXIT

awk '
  /^## Flashcards[[:space:]]*$/ { in_block=1; next }
  in_block && /^## / { in_block=0 }
  in_block && /^<!-- END GENERATED/ { in_block=0 }
  in_block { print }
' "$PAGE" > "$CARDS_TMP"

SCRIPT_DIR="$(dirname "$0")"
python3 "$SCRIPT_DIR/synth-export-anki.py" -- "$SLUG" "$CARDS_TMP" "$OUT"
echo "EXPORTED|$OUT"
