#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: rename.sh <old-slug> <new-slug>" >&2; exit 1; }
OLD="$1"; NEW="$2"

OLD_PATH="$(find content -name "$OLD.md" -type f | head -1)"
[[ -n "$OLD_PATH" ]] || { echo "slug not found: $OLD" >&2; exit 2; }

NEW_PATH="$(dirname "$OLD_PATH")/$NEW.md"
[[ ! -e "$NEW_PATH" ]] || { echo "target exists: $NEW_PATH" >&2; exit 3; }

git mv "$OLD_PATH" "$NEW_PATH" 2>/dev/null || mv "$OLD_PATH" "$NEW_PATH"

# Update all wikilinks - portable across BSD/GNU sed
find content -name '*.md' -type f -print0 | while IFS= read -r -d '' f; do
  awk -v old="$OLD" -v new="$NEW" '
    {
      gsub("\\[\\[" old "\\]\\]", "[[" new "]]")
      gsub("\\[\\[" old "\\|", "[[" new "|")
    }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

bash scripts/log-append.sh rename "$OLD -> $NEW"
echo "RENAME-OK|old=$OLD|new=$NEW"
