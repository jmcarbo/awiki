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

# Build the lead-paragraph + frontmatter snippet for the {{pages}} interpolation.
synth_pages_block() {
  local slugs="$1"
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    local path; path="$(synth_slug_to_path "$slug")"
    [[ -z "$path" ]] && continue
    local title; title="$(synth_fm_field "$path" "title")"
    local type;  type="$(synth_fm_field "$path" "type")"
    title="${title#\"}"; title="${title%\"}"
    # First non-blank non-frontmatter line as lead paragraph (truncated to ~200 chars).
    local lead
    lead="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==2 && NF>0 {print; exit}' "$path")"
    local lead_short="${lead:0:200}"
    printf -- '- [[%s]] (%s) — %s\n' "$slug" "$type" "$lead_short"
  done <<< "$slugs"
}

# Render the prompt bundle to stdout.
synth_emit_prompt() {
  local slugs="$1" scope_desc="$2" feedback="$3"
  local body; body="$(synth_plugin_extract_prompt)"
  # Interpolate {{scope_description}} (single-line replace).
  body="${body//\{\{scope_description\}\}/$scope_desc}"
  # Interpolate {{#pages}}...{{/pages}} block: replace whole tag-pair with
  # page rows. Only one such block in v1 plugin templates.
  local pages_rows pages_file
  pages_rows="$(synth_pages_block "$slugs")"
  pages_file="$(mktemp)"
  printf -- '%s\n' "$pages_rows" > "$pages_file"
  body="$(printf -- '%s\n' "$body" | awk -v rowsfile="$pages_file" '
    BEGIN{
      in_pages=0
      while ((getline line < rowsfile) > 0) { rows = rows line "\n" }
      close(rowsfile)
      sub(/\n$/, "", rows)
    }
    /\{\{#pages\}\}/ { in_pages=1; next }
    /\{\{\/pages\}\}/ { in_pages=0; if (rows != "") print rows; next }
    in_pages==0 { print }
  ')"
  rm -f "$pages_file"
  # Interpolate {{#feedback}}...{{/feedback}}.
  # Phase 13: feedback is empty on `new` and emits a stub note for plugin authors.
  if [[ -z "$feedback" ]]; then
    body="$(printf -- '%s\n' "$body" | awk '
      BEGIN{ in_fb=0 }
      /\{\{#feedback\}\}/ { in_fb=1; next }
      /\{\{\/feedback\}\}/ { in_fb=0; print "<!-- feedback channel arrives in phase 15 -->"; next }
      in_fb==0 { print }
    ')"
  else
    body="${body//\{\{feedback\}\}/$feedback}"
    body="$(printf -- '%s\n' "$body" | sed -e 's/{{#feedback}}//g' -e 's/{{\/feedback}}//g')"
  fi
  printf -- '%s\n' "$body"
}

# Write the synthesis-page scaffold (frontmatter + lead placeholder + ## Notes + markers).
synth_write_scaffold() {
  local page="$1" plugin="$2" scope_block="$3" scope_hash="$4" sources_yaml="$5" topic="$6"
  local today; today="$(date '+%Y-%m-%d')"
  mkdir -p "$(dirname "$page")"
  {
    echo '---'
    printf -- 'title: "%s — %s"\n' "$topic" "$plugin"
    printf -- 'date: %s\n' "$today"
    printf -- 'last_updated: %s\n' "$today"
    printf -- 'last_generated:\n'
    printf -- 'type: synthesis\n'
    printf -- 'plugin: %s\n' "$plugin"
    printf -- '%s' "$scope_block"
    printf -- 'tags: [%s, %s]\n' "$topic" "$plugin"
    printf -- 'aliases: []\n'
    printf -- '%s' "$sources_yaml"
    printf -- 'draft: false\n'
    echo '---'
    echo
    echo "Lead paragraph — written once by user/agent, NOT regenerated."
    echo
    echo "## Notes"
    echo
    echo "<!-- user notes; survives regen -->"
    echo
    printf -- '<!-- BEGIN GENERATED plugin=%s scope_hash=%s -->\n' "$plugin" "$scope_hash"
    echo
    echo '<!-- END GENERATED -->'
  } > "$page"
}

cmd_new() {
  local plugin="" topic="" scope_arg_kind="" scope_arg_val=""
  local exclude_tags="" min_last_updated="" types="" allow_private=0

  if [[ $# -lt 2 ]]; then
    EXIT_CODE=1 die "usage: synth.sh new <plugin> <topic-slug> (--tag=X|--slugs=a,b|--query=\"...\") [...]"
  fi
  plugin="$1"; topic="$2"; shift 2
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --tag=*)              scope_arg_kind="tag";   scope_arg_val="${1#--tag=}" ;;
      --slugs=*)            scope_arg_kind="slugs"; scope_arg_val="${1#--slugs=}" ;;
      --query=*)            scope_arg_kind="query"; scope_arg_val="${1#--query=}" ;;
      --exclude-tags=*)     exclude_tags="${1#--exclude-tags=}" ;;
      --min-last-updated=*) min_last_updated="${1#--min-last-updated=}" ;;
      --types=*)            types="${1#--types=}" ;;
      --allow-private)      allow_private=1 ;;
      --) shift; break ;;
      *) EXIT_CODE=1 die "unknown flag: $1" ;;
    esac
    shift
  done

  require_slug "$topic" "topic-slug"
  if ! synth_plugin_load "$plugin"; then EXIT_CODE=1 die "plugin load failed: $plugin"; fi
  [[ -n "$scope_arg_kind" ]] || { EXIT_CODE=1 die "must pass --tag, --slugs or --query"; }

  # Build scope state.
  SCOPE_TAG=""; SCOPE_SLUGS=""; SCOPE_QUERY=""
  SCOPE_EXCLUDE_TAGS="$exclude_tags"; SCOPE_MIN_LAST_UPDATED="$min_last_updated"; SCOPE_TYPES="$types"
  case "$scope_arg_kind" in
    tag)   SCOPE_TAG="$scope_arg_val" ;;
    slugs) SCOPE_SLUGS="$scope_arg_val" ;;
    query) SCOPE_QUERY="$scope_arg_val" ;;
  esac

  local target="$SYNTH_DIR/$topic-$plugin.md"
  if [[ "$target" == content/private/* ]]; then SCOPE_TARGET_PRIVATE=1; else SCOPE_TARGET_PRIVATE=0; fi

  # First pass: resolve WITHOUT private-stripping, so we can detect leakage.
  local saved_target_priv=$SCOPE_TARGET_PRIVATE
  SCOPE_TARGET_PRIVATE=1   # force "no privacy strip" so we see private slugs
  local raw_slugs; raw_slugs="$(synth_resolve_to_slugs)"
  SCOPE_TARGET_PRIVATE=$saved_target_priv

  local has_private=0
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    local path; path="$(synth_slug_to_path "$slug")"
    [[ -z "$path" ]] && continue
    local tags; tags="$(synth_fm_tags "$path")"
    if [[ " $tags " == *" private "* ]]; then has_private=1; fi
  done <<< "$raw_slugs"

  if [[ "$has_private" -eq 1 && "$SCOPE_TARGET_PRIVATE" -ne 1 && "$allow_private" -ne 1 ]]; then
    EXIT_CODE=2 die "private source in scope; either tag the synthesis page private and place under content/private/, or pass --allow-private"
  fi

  # Second pass: post-filter slug list (the canonical one used for scope_hash + sources:).
  local slugs; slugs="$(synth_resolve_to_slugs)"
  local nslugs; nslugs="$(printf -- '%s\n' "$slugs" | grep -c . || true)"

  if [[ -n "$SYNTH_PLUGIN_MIN_SOURCES" && "$nslugs" -lt "$SYNTH_PLUGIN_MIN_SOURCES" ]]; then
    EXIT_CODE=2 die "scope resolves to $nslugs sources; plugin requires min_sources=$SYNTH_PLUGIN_MIN_SOURCES"
  fi
  if [[ -n "$SYNTH_PLUGIN_MAX_SOURCES" && "$nslugs" -gt "$SYNTH_PLUGIN_MAX_SOURCES" ]]; then
    EXIT_CODE=2 die "scope resolves to $nslugs sources; plugin allows max_sources=$SYNTH_PLUGIN_MAX_SOURCES"
  fi

  if [[ -e "$target" ]]; then
    EXIT_CODE=3 die "target already exists: $target — use 'synth.sh regen' to refresh"
  fi

  local scope_hash; scope_hash="$(synth_scope_hash "$slugs")"

  # Build scope: YAML block.
  local scope_block; scope_block="scope:"$'\n'
  case "$scope_arg_kind" in
    tag)   scope_block+="  tag: $scope_arg_val"$'\n' ;;
    slugs) scope_block+="  slugs: [$scope_arg_val]"$'\n' ;;
    query) scope_block+="  query: \"$scope_arg_val\""$'\n' ;;
  esac
  [[ -n "$exclude_tags"     ]] && scope_block+="  exclude_tags: [$exclude_tags]"$'\n'
  [[ -n "$min_last_updated" ]] && scope_block+="  min_last_updated: $min_last_updated"$'\n'
  [[ -n "$types"            ]] && scope_block+="  types: [$types]"$'\n'

  # Build sources: list (empty on `new`; populated at finalize-time, per spec).
  local sources_yaml; sources_yaml="sources: []"$'\n'

  synth_write_scaffold "$target" "$plugin" "$scope_block" "$scope_hash" "$sources_yaml" "$topic"

  # Emit prompt bundle to stdout.
  local scope_desc
  case "$scope_arg_kind" in
    tag)   scope_desc="pages tagged '$scope_arg_val'" ;;
    slugs) scope_desc="explicit slug list ($nslugs pages)" ;;
    query) scope_desc="qmd query: $scope_arg_val" ;;
  esac
  synth_emit_prompt "$slugs" "$scope_desc" ""

  bash "$REPO_ROOT/scripts/log-append.sh" synth-scaffold -- "$plugin $topic"
  if [[ "$allow_private" -eq 1 && "$has_private" -eq 1 ]]; then
    bash "$REPO_ROOT/scripts/log-append.sh" synth-declassify -- "$topic sources=$nslugs"
  fi

  echo "SYNTH-NEW|target=$target|plugin=$plugin|sources=$nslugs|scope_hash=$scope_hash" >&2
}

# Validate marker integrity. Returns 0 on OK, prints reason and returns 5 on failure.
synth_check_markers() {
  local page="$1"
  local begins; begins="$(grep -c '^<!-- BEGIN GENERATED ' "$page" || true)"
  local ends;   ends="$(grep -c '^<!-- END GENERATED -->' "$page" || true)"
  if [[ "$begins" -ne 1 || "$ends" -ne 1 ]]; then
    echo "ERROR: marker integrity failure: BEGIN=$begins END=$ends" >&2
    return 5
  fi
  local b_line e_line
  b_line="$(grep -n '^<!-- BEGIN GENERATED ' "$page" | head -1 | cut -d: -f1)"
  e_line="$(grep -n '^<!-- END GENERATED -->' "$page" | head -1 | cut -d: -f1)"
  if [[ "$b_line" -gt "$e_line" ]]; then
    echo "ERROR: BEGIN marker after END marker" >&2
    return 5
  fi
  return 0
}

# Rewrite frontmatter scalars / list fields in-place.
synth_fm_set_scalar() {
  local page="$1" key="$2" value="$3"
  awk -v k="$key" -v v="$value" '
    BEGIN{c=0; done=0}
    /^---$/ {c++; print; next}
    c==1 && index($0, k":")==1 && done==0 { print k": "v; done=1; next }
    {print}
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

synth_fm_set_sources() {
  local page="$1" slugs="$2"
  local list="["
  local first=1
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if [[ $first -eq 1 ]]; then list+="\"[[$slug]]\""; first=0; else list+=", \"[[$slug]]\""; fi
  done <<< "$slugs"
  list+="]"
  awk -v v="$list" '
    BEGIN{c=0; done=0}
    /^---$/ {c++; print; next}
    c==1 && /^sources:/ && done==0 { print "sources: " v; done=1; next }
    {print}
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

synth_fm_set_scope_hash_in_marker() {
  local page="$1" hash="$2"
  awk -v h="$hash" '
    /^<!-- BEGIN GENERATED plugin=/ {
      sub(/scope_hash=[0-9a-f]+/, "scope_hash=" h)
    }
    {print}
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

cmd_finalize() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh finalize <slug>"; fi
  local slug="$1"; require_slug "$slug" "synthesis-page-slug"

  local live="$SYNTH_DIR/$slug.md"
  local staged="$STAGED_DIR/$slug.md"
  local target=""
  if [[ -f "$staged" ]]; then target="$staged"; else target="$live"; fi
  [[ -f "$target" ]] || { EXIT_CODE=1 die "synthesis page not found: $target"; }

  if ! synth_check_markers "$target"; then EXIT_CODE=5 die "marker integrity failed"; fi

  # Scoped lint (phase 13: only marker + required-sections + slug existence;
  # full S1-S9 lands in phase 14).
  if ! bash "$REPO_ROOT/scripts/lint.sh" --only=synth --file="$target" 2>/dev/null; then
    EXIT_CODE=6 die "scoped lint failed for $target"
  fi

  # Re-resolve scope, recompute hash, populate sources:.
  synth_read_scope "$target"
  if [[ "$target" == */private/* ]]; then SCOPE_TARGET_PRIVATE=1; else SCOPE_TARGET_PRIVATE=0; fi
  local slugs; slugs="$(synth_resolve_to_slugs)"
  local hash;  hash="$(synth_scope_hash "$slugs")"
  synth_fm_set_scope_hash_in_marker "$target" "$hash"
  synth_fm_set_sources "$target" "$slugs"
  local now_utc; now_utc="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  synth_fm_set_scalar "$target" "last_generated" "$now_utc"

  # If we operated on a staged file, leave it staged; user runs `accept-stage`.
  bash "$REPO_ROOT/scripts/log-append.sh" synth -- "$(synth_fm_field "$target" plugin) $slug"

  # Bump ingest counter (synthesis counts toward auto-lint cadence).
  mkdir -p .awiki
  local cnt=0; [[ -f .awiki/ingest-count ]] && cnt="$(cat .awiki/ingest-count)"
  echo "$((cnt + 1))" > .awiki/ingest-count

  echo "SYNTH-FINALIZE|target=$target|sources=$(printf -- '%s' "$slugs" | grep -c . || true)|scope_hash=$hash" >&2
}

# Detect hand-edits inside the marker region by diffing against HEAD.
# Returns 0 if no marker-region diff, 1 if intersecting diff found, 2 if untracked/no HEAD.
synth_handedit_check() {
  local page="$1"
  if ! git ls-files --error-unmatch -- "$page" >/dev/null 2>&1; then
    return 2  # untracked: treat as fresh
  fi
  if ! git rev-parse --verify HEAD >/dev/null 2>&1; then
    return 2  # no HEAD yet
  fi
  local head_tmp work_tmp; head_tmp="$(mktemp)"; work_tmp="$(mktemp)"
  git show "HEAD:$page" > "$head_tmp" 2>/dev/null || { rm -f "$head_tmp" "$work_tmp"; return 2; }
  cp "$page" "$work_tmp"

  # Extract the BEGIN..END block from each side.
  local extract='
    /^<!-- BEGIN GENERATED / { p=1 }
    p { print }
    /^<!-- END GENERATED -->/ { p=0 }
  '
  local h w
  h="$(awk "$extract" "$head_tmp")"
  w="$(awk "$extract" "$work_tmp")"
  rm -f "$head_tmp" "$work_tmp"
  if [[ "$h" != "$w" ]]; then return 1; fi
  return 0
}

# Rewrite the marker region of $page with empty body and updated scope_hash.
synth_clear_region() {
  local page="$1" plugin="$2" hash="$3"
  awk -v plugin="$plugin" -v hash="$hash" '
    BEGIN{ in_region=0 }
    /^<!-- BEGIN GENERATED / {
      print "<!-- BEGIN GENERATED plugin=" plugin " scope_hash=" hash " -->"
      print ""
      in_region=1; next
    }
    /^<!-- END GENERATED -->/ {
      print
      in_region=0; next
    }
    in_region==0 { print }
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

cmd_regen() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh regen <slug> [--force] [--stage]"; fi
  local slug="$1"; shift; require_slug "$slug" "synthesis-page-slug"

  local force=0 stage=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --force) force=1 ;;
      --stage) stage=1 ;;
      --) shift; break ;;
      *) EXIT_CODE=1 die "unknown flag: $1" ;;
    esac
    shift
  done

  local live="$SYNTH_DIR/$slug.md"
  [[ -f "$live" ]] || { EXIT_CODE=1 die "synthesis page not found: $live"; }

  local plugin; plugin="$(synth_fm_field "$live" "plugin")"
  [[ -n "$plugin" ]] || { EXIT_CODE=1 die "$live missing 'plugin:' frontmatter"; }
  if ! synth_plugin_load "$plugin"; then EXIT_CODE=1 die "plugin load failed: $plugin"; fi

  if [[ "$force" -eq 0 && "$stage" -eq 0 ]]; then
    set +e
    synth_handedit_check "$live"
    local rc=$?
    set -e
    case $rc in
      1) EXIT_CODE=4 die "hand-edit detected inside marker region; pass --force or --stage" ;;
      2) : ;;  # untracked or no HEAD; proceed
    esac
  fi

  synth_read_scope "$live"
  if [[ "$live" == */private/* ]]; then SCOPE_TARGET_PRIVATE=1; else SCOPE_TARGET_PRIVATE=0; fi

  # Re-run privacy fail-closed check (catches newly-private sources).
  local saved=$SCOPE_TARGET_PRIVATE
  SCOPE_TARGET_PRIVATE=1
  local raw_slugs; raw_slugs="$(synth_resolve_to_slugs)"
  SCOPE_TARGET_PRIVATE=$saved
  while IFS= read -r s; do
    [[ -z "$s" ]] && continue
    local sp; sp="$(synth_slug_to_path "$s")"
    [[ -z "$sp" ]] && continue
    local stags; stags="$(synth_fm_tags "$sp")"
    if [[ " $stags " == *" private "* && "$SCOPE_TARGET_PRIVATE" -ne 1 ]]; then
      EXIT_CODE=2 die "private source $s now in scope; tag the synthesis page private or pass --allow-private (regen)"
    fi
  done <<< "$raw_slugs"

  local slugs; slugs="$(synth_resolve_to_slugs)"
  local hash;  hash="$(synth_scope_hash "$slugs")"

  local target="$live"
  if [[ "$stage" -eq 1 ]]; then
    mkdir -p "$STAGED_DIR"
    target="$STAGED_DIR/$slug.md"
    cp "$live" "$target"
  fi

  synth_clear_region "$target" "$plugin" "$hash"

  # Emit prompt bundle (no feedback in phase 13).
  local scope_desc=""
  [[ -n "$SCOPE_TAG"   ]] && scope_desc="pages tagged '$SCOPE_TAG'"
  [[ -n "$SCOPE_SLUGS" ]] && scope_desc="explicit slug list"
  [[ -n "$SCOPE_QUERY" ]] && scope_desc="qmd query: $SCOPE_QUERY"
  synth_emit_prompt "$slugs" "$scope_desc" ""

  echo "SYNTH-REGEN|target=$target|stage=$stage|scope_hash=$hash" >&2
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
    new) cmd_new "$@" ;;
    finalize) cmd_finalize "$@" ;;
    regen) cmd_regen "$@" ;;
    accept-stage|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
    *) EXIT_CODE=1 die "unknown subcommand: $sub" ;;
  esac
}

main "$@"
