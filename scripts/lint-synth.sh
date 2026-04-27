#!/usr/bin/env bash
# lint-synth.sh — synth-namespaced lint rules. SOURCED by scripts/lint.sh.
# Contributes to ERRORS / WARNS / INFOS counters defined by the caller.
#
# Public entry points:
#   synth_lint_file <synth-page-path> <content-dir>
#   synth_lint_dir <content-dir>
#
# Rules:
#   S1 marker integrity (error)
#   S2 required sections present (error)        — task 14.4
#   S3 evidence quote substring (error)         — task 14.5
#   S4 citation slug in scope (error)           — task 14.6
#   S5 scope drift (warning, skip query-scope)  — task 14.7
#   S6 hand-edit inside markers (warning)       — task 14.8
#   S9 aggregate evidence words (error)         — task 14.10
#
# S7/S8 are deferred to phase 15.

# Guard: only re-source ok if functions already defined.
if declare -F synth_lint_file >/dev/null 2>&1; then
  return 0
fi

# --- S1: marker integrity ----------------------------------------------------
synth_check_s1() {
  local page="$1"
  local begin_count end_count
  begin_count=$(grep -cE '^<!-- BEGIN GENERATED .* -->$' "$page" || true)
  end_count=$(grep -cE '^<!-- END GENERATED -->$' "$page" || true)

  if [[ "$begin_count" -eq 0 ]]; then
    echo "LINT|ERROR|$page|S1: missing BEGIN GENERATED marker"
    ERRORS=$((ERRORS + 1))
    return 1
  fi
  if [[ "$end_count" -eq 0 ]]; then
    echo "LINT|ERROR|$page|S1: missing END GENERATED marker"
    ERRORS=$((ERRORS + 1))
    return 1
  fi
  if [[ "$begin_count" -gt 1 ]]; then
    echo "LINT|ERROR|$page|S1: duplicate BEGIN GENERATED marker (found $begin_count)"
    ERRORS=$((ERRORS + 1))
    return 1
  fi
  if [[ "$end_count" -gt 1 ]]; then
    echo "LINT|ERROR|$page|S1: duplicate END GENERATED marker (found $end_count)"
    ERRORS=$((ERRORS + 1))
    return 1
  fi

  local begin_line end_line
  begin_line=$(grep -nE '^<!-- BEGIN GENERATED .* -->$' "$page" | head -1 | cut -d: -f1)
  end_line=$(grep -nE '^<!-- END GENERATED -->$' "$page" | head -1 | cut -d: -f1)
  if [[ -z "$begin_line" || -z "$end_line" || "$begin_line" -ge "$end_line" ]]; then
    echo "LINT|ERROR|$page|S1: BEGIN must precede END (begin=$begin_line end=$end_line)"
    ERRORS=$((ERRORS + 1))
    return 1
  fi
  return 0
}

# --- S2: required sections present ------------------------------------------
# Reads plugin manifest's required_sections list (YAML array, single-file form
# only — directory form not supported in v1) and asserts each heading appears
# between BEGIN and END markers.
synth_get_plugin_required_sections() {
  local plugin="$1"
  local manifest="synthesis-plugins/$plugin.md"
  [[ -f "$manifest" ]] || return 1

  # Extract YAML frontmatter, look for required_sections: list.
  awk '
    /^---$/ { c++; next }
    c==1 && /^required_sections:/ { in_list=1; next }
    c==1 && in_list && /^[a-z_]+:/ { in_list=0 }
    c==1 && in_list && /^[[:space:]]*-[[:space:]]/ {
      sub(/^[[:space:]]*-[[:space:]]*/, "")
      gsub(/^"|"$/, "")
      print
    }
    c>=2 { exit }
  ' "$manifest"
}

synth_check_s2() {
  local page="$1"
  local plugin
  plugin=$(awk '/^plugin: /{print $2; exit}' "$page")
  [[ -n "$plugin" ]] || return 0

  local required
  required=$(synth_get_plugin_required_sections "$plugin") || {
    echo "LINT|ERROR|$page|S2: cannot read manifest for plugin $plugin"
    ERRORS=$((ERRORS + 1))
    return 1
  }
  [[ -n "$required" ]] || return 0

  # Slice between markers.
  local region
  region=$(awk '
    /^<!-- BEGIN GENERATED .* -->$/ { in_region=1; next }
    /^<!-- END GENERATED -->$/      { in_region=0 }
    in_region { print }
  ' "$page")

  while IFS= read -r heading; do
    [[ -z "$heading" ]] && continue
    if ! grep -qxF "$heading" <<<"$region"; then
      echo "LINT|ERROR|$page|S2: required section missing: $heading"
      ERRORS=$((ERRORS + 1))
    fi
  done <<<"$required"
}

# --- S3: evidence quote substring -------------------------------------------
# Resolve a slug to a source path via <wiki-root>/.awiki/maps/slug-to-path.tsv.
# Wiki root is derived from content_dir (assumes content_dir = <wiki-root>/content).
synth_resolve_slug() {
  local slug="$1"
  local content_dir="${2:-content}"
  local wiki_root="${content_dir%/content}"
  [[ "$wiki_root" = "$content_dir" ]] && wiki_root="$(dirname "$content_dir")"
  local map="$wiki_root/.awiki/maps/slug-to-path.tsv"
  if [[ -f "$map" ]]; then
    local rel
    rel=$(awk -F'\t' -v s="$slug" '$1==s{print $2; exit}' "$map")
    if [[ -n "$rel" ]]; then
      if [[ "$rel" = /* ]]; then
        echo "$rel"
      else
        echo "$wiki_root/$rel"
      fi
      return
    fi
  fi
  # Fallback: name-based lookup under the content dir.
  find "$content_dir" -type f -name "$slug.md" 2>/dev/null | head -1
}

synth_check_s3() {
  local page="$1"
  local content_dir="${2:-content}"
  local wiki_root="${content_dir%/content}"
  [[ "$wiki_root" = "$content_dir" ]] && wiki_root="$(dirname "$content_dir")"
  local map="$wiki_root/.awiki/maps/slug-to-path.tsv"

  # Slice the generated region.
  local region_file
  region_file=$(mktemp)
  awk '
    /^<!-- BEGIN GENERATED .* -->$/ { in_region=1; next }
    /^<!-- END GENERATED -->$/      { in_region=0 }
    in_region { print }
  ' "$page" > "$region_file"

  # Iterate evidence quote lines: > "..." — [[slug]]
  # The em-dash is U+2014 LITERAL — match exactly.
  local line slug quote_raw
  while IFS= read -r line; do
    # Capture the quote body and slug. Match an em-dash literal.
    if [[ "$line" =~ ^\>[[:space:]]+\"(.+)\"[[:space:]]+—[[:space:]]+\[\[([a-z0-9][a-z0-9-]*)\]\][[:space:]]*$ ]]; then
      quote_raw="${BASH_REMATCH[1]}"
      slug="${BASH_REMATCH[2]}"
    else
      continue
    fi

    local source_path
    source_path=$(synth_resolve_slug "$slug" "$content_dir")
    if [[ -z "$source_path" || ! -f "$source_path" ]]; then
      echo "LINT|ERROR|$page|S3: evidence cites unresolvable slug: $slug"
      ERRORS=$((ERRORS + 1))
      continue
    fi

    # Materialize quote to a tmpfile (no argv text interpolation).
    local quote_tmp rewrite_tmp norm_quote_tmp norm_source_tmp
    quote_tmp=$(mktemp); rewrite_tmp=$(mktemp)
    norm_quote_tmp=$(mktemp); norm_source_tmp=$(mktemp)
    printf '%s' "$quote_raw" > "$quote_tmp"

    # Rewrite [[other-slug]] inside the quote to title text.
    if [[ -f "$map" ]]; then
      python3 scripts/lint-synth-rewrite-wikilinks.py "--map=$map" -- "$quote_tmp" > "$rewrite_tmp"
    else
      python3 scripts/lint-synth-rewrite-wikilinks.py -- "$quote_tmp" > "$rewrite_tmp"
    fi

    # Normalize.
    python3 scripts/lint-synth-normalize.py -- --quote  "$rewrite_tmp"  > "$norm_quote_tmp"
    python3 scripts/lint-synth-normalize.py -- --source "$source_path" > "$norm_source_tmp"

    if grep -F -q -f "$norm_quote_tmp" "$norm_source_tmp"; then
      :  # OK
    else
      local suggestion
      suggestion=$(python3 scripts/lint-synth-fuzzy.py -- "$source_path" "$norm_quote_tmp" 2>/dev/null || true)
      if [[ -n "$suggestion" ]]; then
        echo "LINT|ERROR|$page|S3: evidence quote not found in [[$slug]] (suggestion: $suggestion)"
      else
        echo "LINT|ERROR|$page|S3: evidence quote not found in [[$slug]] (hallucinated or paraphrased; no close match)"
      fi
      ERRORS=$((ERRORS + 1))
    fi

    rm -f "$quote_tmp" "$rewrite_tmp" "$norm_quote_tmp" "$norm_source_tmp"
  done < "$region_file"

  rm -f "$region_file"
}

# --- entry points ------------------------------------------------------------
# synth_lint_file: lint a single synthesis page. Caller passes the page path
# and the wiki content directory (used by S3/S4 for slug resolution).
synth_lint_file() {
  local page="$1"
  local content_dir="${2:-content}"

  # Only act on files with `type: synthesis` AND `plugin: <name>` frontmatter.
  if ! head -40 "$page" | grep -q '^type: synthesis$'; then
    return 0
  fi
  if ! head -40 "$page" | grep -qE '^plugin: [a-z][a-z0-9-]*$'; then
    return 0
  fi

  synth_check_s1 "$page" || return 0  # bail on broken markers — downstream rules need them
  synth_check_s2 "$page"
  synth_check_s3 "$page" "$content_dir"
  # synth_check_s4..S9 added in subsequent tasks.
}

synth_lint_dir() {
  local content_dir="${1:-content}"
  local synth_dir="$content_dir/synthesis"
  [[ -d "$synth_dir" ]] || return 0
  while IFS= read -r -d '' page; do
    synth_lint_file "$page" "$content_dir"
  done < <(find "$synth_dir" -maxdepth 2 -name '*.md' -type f -print0)
}
