#!/usr/bin/env bash
set -uo pipefail

FIX=0
CONTENT_DIR="content"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix) FIX=1; shift ;;
    *) CONTENT_DIR="$1"; shift ;;
  esac
done

apply_fixes() {
  local page="$1"
  if ! grep -q '^last_updated:' "$page"; then
    local today
    today="$(date '+%Y-%m-%d')"
    # Portable insertion: use awk (handles BSD/GNU sed differences in \n).
    awk -v today="$today" '
      /^date: / { print; print "last_updated: " today; next }
      { print }
    ' "$page" > "$page.tmp"
    if cmp -s "$page.tmp" "$page"; then
      # awk pass was a no-op (no `date:` line to anchor on); leave file untouched.
      rm -f "$page.tmp"
    else
      mv "$page.tmp" "$page"
      echo "FIX|$page|added last_updated: $today"
    fi
  fi
}

if [[ "$FIX" -eq 1 ]]; then
  while IFS= read -r -d '' page; do
    apply_fixes "$page"
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi

ERRORS=0
WARNS=0
INFOS=0

declare -A SLUG_TO_PATH
declare -A ALIAS_TO_SLUG
declare -A ALIAS_COUNT

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  if [[ -n "${SLUG_TO_PATH[$slug]:-}" ]]; then
    echo "LINT|ERROR|$page|duplicate slug: $slug also at ${SLUG_TO_PATH[$slug]}"
    ERRORS=$((ERRORS + 1))
  fi
  SLUG_TO_PATH[$slug]="$page"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        ALIAS_COUNT[$a]=$((${ALIAS_COUNT[$a]:-0} + 1))
        ALIAS_TO_SLUG[$a]="$slug"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  body_len=$(awk '/^---$/{c++; next} c==2{print}' "$page" | wc -c | tr -d ' ')
  if [[ "$body_len" -lt 50 ]]; then
    echo "LINT|WARN|$page|empty page (<50 char body)"
    WARNS=$((WARNS + 1))
  fi

  # Parse the tags: line as a YAML list, check for an exact 'private' token.
  has_private_tag=0
  while IFS= read -r line; do
    if [[ "$line" =~ ^tags:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        token="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        if [[ "$token" == "private" ]]; then has_private_tag=1; fi
      done
    fi
  done < "$page"

  if [[ "$has_private_tag" -eq 1 && "$page" != *"/private/"* ]]; then
    echo "LINT|WARN|$page|private tag outside private path"
    WARNS=$((WARNS + 1))
  fi

  while read -r link; do
    target="${link%%|*}"
    if [[ -z "${SLUG_TO_PATH[$target]:-}" && -z "${ALIAS_TO_SLUG[$target]:-}" ]]; then
      echo "LINT|ERROR|$page|broken wikilink: [[$target]]"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done

# Orphan check: count inbound wikilinks per slug; warn if zero.
declare -A INBOUND
while IFS= read -r -d '' page; do
  while read -r link; do
    target="${link%%|*}"
    INBOUND[$target]=$((${INBOUND[$target]:-0} + 1))
    # Also credit the alias's resolved slug, if any
    resolved="${ALIAS_TO_SLUG[$target]:-}"
    if [[ -n "$resolved" ]]; then
      INBOUND[$resolved]=$((${INBOUND[$resolved]:-0} + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
  if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
    continue
  fi
  if [[ "${INBOUND[$slug]:-0}" -eq 0 ]]; then
    echo "LINT|INFO|$page|orphan: no inbound wikilinks"
    INFOS=$((INFOS + 1))
  fi
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Catalog cross-link check
CATALOG="$CONTENT_DIR/catalog.md"
if [[ -f "$CATALOG" ]]; then
  declare -A IN_CATALOG
  while read -r link; do
    target="${link%%|*}"
    IN_CATALOG[$target]=1
  done < <(grep -oE '\[\[[^]]+\]\]' "$CATALOG" | sed -E 's/^\[\[|\]\]$//g')

  while IFS= read -r -d '' page; do
    slug="$(basename "$page" .md)"
    [[ "$slug" =~ ^(_index|catalog|log)$ ]] && continue
    type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
    if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
      continue
    fi
    if [[ -z "${IN_CATALOG[$slug]:-}" ]]; then
      echo "LINT|WARN|$page|missing from catalog"
      WARNS=$((WARNS + 1))
    fi
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi

echo "LINT-SUMMARY|errors=$ERRORS|warnings=$WARNS|info=$INFOS"

if [[ "$ERRORS" -gt 0 ]]; then exit 2; fi
if [[ "$WARNS" -gt 0 ]]; then exit 1; fi
exit 0
