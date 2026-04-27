#!/usr/bin/env bash
set -euo pipefail

# scripts/synth.sh — orchestrator for awiki synthesis pages.
# Subcommands: new | regen | accept-stage | finalize | list | resolve | refine

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

# shellcheck disable=SC1091
source "$REPO_ROOT/scripts/synth-plugin-load.sh"

PLUGINS_DIR="${AWIKI_SYNTH_PLUGINS_DIR:-synthesis-plugins}"
SYNTH_DIR="${AWIKI_SYNTH_DIR:-content/synthesis}"
STAGED_DIR="$SYNTH_DIR/.staged"
SLUG_MAP="${AWIKI_SLUG_MAP:-.awiki/maps/slug-to-path.tsv}"

SLUG_REGEX='^[a-z0-9][a-z0-9-]*$'
PLUGIN_NAME_REGEX='^[a-z][a-z0-9-]*$'

die() { echo "ERROR: $*" >&2; exit "${EXIT_CODE:-1}"; }
log() { echo "$*" >&2; }

require_slug() {
  local s="$1" label="${2:-slug}"
  if ! [[ "$s" =~ $SLUG_REGEX ]]; then
    EXIT_CODE=1 die "invalid $label: '$s' (must match $SLUG_REGEX, no leading hyphen)"
  fi
}

cmd_list() {
  local count=0
  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    if synth_plugin_load "$name" 2>/dev/null; then
      printf -- "%s\t%s\t%s\t%s\n" \
        "$SYNTH_PLUGIN_NAME" \
        "$SYNTH_PLUGIN_OUTPUT_TYPE" \
        "$SYNTH_PLUGIN_OUTPUT_SUBTYPE" \
        "$SYNTH_PLUGIN_DESCRIPTION"
      count=$((count + 1))
    else
      log "skipping malformed plugin: $name"
    fi
  done < <(synth_plugin_list_all)
  if [[ "$count" -eq 0 ]]; then
    EXIT_CODE=1 die "no parseable plugins under $PLUGINS_DIR"
  fi
}

# Read scope: block from a synthesis page's frontmatter.
# Sets SCOPE_TAG, SCOPE_SLUGS, SCOPE_QUERY, SCOPE_EXCLUDE_TAGS, SCOPE_MIN_LAST_UPDATED, SCOPE_TYPES.
synth_read_scope() {
  local page="$1"
  SCOPE_TAG=""; SCOPE_SLUGS=""; SCOPE_QUERY=""
  SCOPE_EXCLUDE_TAGS=""; SCOPE_MIN_LAST_UPDATED=""; SCOPE_TYPES=""
  local fm
  fm="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==1 {print}' "$page")"
  # Read scope: block (we expect block-form keys: scope: + indented children).
  # Flat parse: indented lines starting with two spaces under the line "scope:".
  local in_scope=0
  while IFS= read -r line; do
    if [[ "$line" =~ ^scope:[[:space:]]*$ ]]; then in_scope=1; continue; fi
    if [[ "$in_scope" -eq 1 ]]; then
      if [[ "$line" =~ ^[^[:space:]] ]]; then in_scope=0; continue; fi
      case "$line" in
        *"  tag:"*)    SCOPE_TAG="${line#*tag:}"; SCOPE_TAG="${SCOPE_TAG# }" ;;
        *"  slugs:"*)  SCOPE_SLUGS="${line#*slugs:}"; SCOPE_SLUGS="${SCOPE_SLUGS# }"; SCOPE_SLUGS="${SCOPE_SLUGS#[}"; SCOPE_SLUGS="${SCOPE_SLUGS%]}" ;;
        *"  query:"*)  SCOPE_QUERY="${line#*query:}"; SCOPE_QUERY="${SCOPE_QUERY# }"; SCOPE_QUERY="${SCOPE_QUERY#\"}"; SCOPE_QUERY="${SCOPE_QUERY%\"}" ;;
        *"  exclude_tags:"*) SCOPE_EXCLUDE_TAGS="${line#*exclude_tags:}"; SCOPE_EXCLUDE_TAGS="${SCOPE_EXCLUDE_TAGS# }"; SCOPE_EXCLUDE_TAGS="${SCOPE_EXCLUDE_TAGS#[}"; SCOPE_EXCLUDE_TAGS="${SCOPE_EXCLUDE_TAGS%]}" ;;
        *"  min_last_updated:"*) SCOPE_MIN_LAST_UPDATED="${line#*min_last_updated:}"; SCOPE_MIN_LAST_UPDATED="${SCOPE_MIN_LAST_UPDATED# }" ;;
        *"  types:"*) SCOPE_TYPES="${line#*types:}"; SCOPE_TYPES="${SCOPE_TYPES# }"; SCOPE_TYPES="${SCOPE_TYPES#[}"; SCOPE_TYPES="${SCOPE_TYPES%]}" ;;
      esac
    fi
  done <<< "$fm"
}

# Returns frontmatter scalar value for a single page.
# Usage: synth_fm_field <page> <field>
synth_fm_field() {
  awk -v field="$2" '
    BEGIN{c=0}
    /^---$/ {c++; next}
    c==1 && $1==(field":") { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }
  ' "$1"
}

# Echo space-separated tag list (lowered, no quotes/brackets).
synth_fm_tags() {
  local raw
  raw="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==1 && /^tags:/ {sub(/^tags:[[:space:]]*/,""); print; exit}' "$1")"
  raw="${raw#[}"; raw="${raw%]}"
  printf -- '%s' "$raw" | tr ',' ' ' | tr -d '"' | tr -d "'" | tr -s ' '
}

# Return path for a slug via the slug-to-path map; fall back to find.
synth_slug_to_path() {
  local slug="$1"
  if [[ -f "$SLUG_MAP" ]]; then
    awk -F '\t' -v s="$slug" '$1==s {print $2; exit}' "$SLUG_MAP"
    return
  fi
  find "${AWIKI_CONTENT_DIR:-content}" -name "$slug.md" -type f -print -quit
}

# Resolve scope to a sorted, post-filter slug list, one per line.
# Inputs: $SCOPE_*, plus $SCOPE_TARGET_PRIVATE (1 if target is under content/private/).
synth_resolve_to_slugs() {
  local content_root="${AWIKI_CONTENT_DIR:-content}"
  local found
  found="$(mktemp)"

  if [[ -n "$SCOPE_SLUGS" ]]; then
    printf -- '%s' "$SCOPE_SLUGS" | tr ',' '\n' | sed -E 's/^[[:space:]"'\''-]+|[[:space:]"'\'']+$//g' | grep -v '^$' >> "$found"
  elif [[ -n "$SCOPE_TAG" ]]; then
    while IFS= read -r -d '' page; do
      local tags; tags="$(synth_fm_tags "$page")"
      if [[ " $tags " == *" $SCOPE_TAG "* ]]; then
        basename "$page" .md >> "$found"
      fi
    done < <(find "$content_root" -name '*.md' -type f -print0)
  elif [[ -n "$SCOPE_QUERY" ]]; then
    if ! command -v qmd >/dev/null 2>&1; then
      EXIT_CODE=2 die "scope.query requires qmd; phase-15 grep fallback not yet shipped"
    fi
    qmd search -- "$SCOPE_QUERY" 2>/dev/null \
      | awk '{print $1}' \
      | grep -E "$SLUG_REGEX" >> "$found" || true
  else
    EXIT_CODE=2 die "scope must include exactly one of: tag, slugs, query"
  fi

  # Filter: exclude_tags
  if [[ -n "$SCOPE_EXCLUDE_TAGS" ]]; then
    local excl
    excl="$(printf -- '%s' "$SCOPE_EXCLUDE_TAGS" | tr ',' ' ' | tr -d '"' | tr -d "'" | tr -s ' ')"
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local tags; tags="$(synth_fm_tags "$path")"
      local skip=0
      for ex in $excl; do
        [[ -z "$ex" ]] && continue
        if [[ " $tags " == *" $ex "* ]]; then skip=1; break; fi
      done
      [[ "$skip" -eq 0 ]] && echo "$slug" >> "$kept"
    done < "$found"
    mv "$kept" "$found"
  fi

  # Filter: min_last_updated (lex compare on YYYY-MM-DD)
  if [[ -n "$SCOPE_MIN_LAST_UPDATED" ]]; then
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local lu; lu="$(synth_fm_field "$path" "last_updated")"
      if [[ "$lu" > "$SCOPE_MIN_LAST_UPDATED" || "$lu" == "$SCOPE_MIN_LAST_UPDATED" ]]; then
        echo "$slug" >> "$kept"
      fi
    done < "$found"
    mv "$kept" "$found"
  fi

  # Filter: types
  if [[ -n "$SCOPE_TYPES" ]]; then
    local types
    types="$(printf -- '%s' "$SCOPE_TYPES" | tr ',' ' ' | tr -d '"' | tr -d "'" | tr -s ' ')"
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local t; t="$(synth_fm_field "$path" "type")"
      for want in $types; do
        if [[ "$t" == "$want" ]]; then echo "$slug" >> "$kept"; break; fi
      done
    done < "$found"
    mv "$kept" "$found"
  fi

  # Privacy filter: drop slugs tagged `private` if target is not private.
  # The fail-closed enforcement happens in cmd_new; here we only emit the
  # post-filter slug list so resolve/regen can compute scope_hash deterministically.
  if [[ "${SCOPE_TARGET_PRIVATE:-0}" -ne 1 ]]; then
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local tags; tags="$(synth_fm_tags "$path")"
      [[ " $tags " != *" private "* ]] && echo "$slug" >> "$kept"
    done < "$found"
    mv "$kept" "$found"
  fi

  # Sort + dedupe ASCII-ascending (post-filter ordering is load-bearing for scope_hash).
  sort -u "$found"
  rm -f "$found"
}

# scope_hash = first 6 hex chars of sha256(sorted-slugs-newline-joined)
synth_scope_hash() {
  local slugs="$1"
  printf -- '%s\n' "$slugs" | tr -d '\r' | shasum -a 256 | awk '{print substr($1,1,6)}'
}

cmd_resolve() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh resolve <slug>"; fi
  local slug="$1"; require_slug "$slug" "synthesis-page-slug"
  local page="$SYNTH_DIR/$slug.md"
  [[ -f "$page" ]] || { EXIT_CODE=2 die "synthesis page not found: $page"; }

  synth_read_scope "$page"
  local target_path; target_path="$page"
  if [[ "$target_path" == *"/private/"* || "$target_path" == content/private/* ]]; then
    SCOPE_TARGET_PRIVATE=1
  else
    SCOPE_TARGET_PRIVATE=0
  fi
  synth_resolve_to_slugs
}

main() {
  if [[ $# -lt 1 ]]; then
    cat <<USAGE
usage: synth.sh <subcommand> [args...]
subcommands: new | regen | accept-stage | finalize | list | resolve | refine
USAGE
    exit 1
  fi
  local sub="$1"; shift
  case "$sub" in
    list) cmd_list "$@" ;;
    resolve) cmd_resolve "$@" ;;
    new|regen|accept-stage|finalize|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
    *) EXIT_CODE=1 die "unknown subcommand: $sub" ;;
  esac
}

main "$@"
