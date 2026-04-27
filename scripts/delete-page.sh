#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: delete-page.sh <slug>" >&2; exit 1; }
SLUG="$1"

PAGE="$(find content -name "$SLUG.md" -type f | head -1)"
[[ -n "$PAGE" ]] || { echo "slug not found: $SLUG" >&2; exit 2; }

git rm "$PAGE" 2>/dev/null || rm "$PAGE"

# Replace wikilinks with broken markers - portable across BSD/GNU sed
find content -name '*.md' -type f -print0 | while IFS= read -r -d '' f; do
  awk -v slug="$SLUG" '
    {
      gsub("\\[\\[" slug "\\]\\]", "<!-- broken: was [[" slug "]] -->" slug)
      gsub("\\[\\[" slug "\\|", "<!-- broken: was [[" slug "|... -->" slug "|")
    }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

bash scripts/log-append.sh delete "$SLUG (removed; wikilinks marked broken for review)"
echo "DELETE-OK|slug=$SLUG"
