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

# --- S4: citation slug in scope ----------------------------------------------
# Parse the page's scope: block and resolve to a slug list using the
# wiki-root's slug-to-path.tsv. Every [[slug]] inside markers must be present.
synth_resolve_scope_slugs() {
  local page="$1"
  local content_dir="${2:-content}"
  local wiki_root="${content_dir%/content}"
  [[ "$wiki_root" = "$content_dir" ]] && wiki_root="$(dirname "$content_dir")"
  local map="$wiki_root/.awiki/maps/slug-to-path.tsv"

  # Read scope block from frontmatter.
  local scope_kind="" scope_val=""
  local in_fm=0 in_scope=0
  while IFS= read -r line; do
    if [[ "$line" == "---" ]]; then
      in_fm=$((in_fm + 1))
      continue
    fi
    [[ "$in_fm" -ne 1 ]] && continue
    if [[ "$line" =~ ^scope:[[:space:]]*$ ]]; then in_scope=1; continue; fi
    if [[ "$in_scope" -eq 1 ]]; then
      if [[ "$line" =~ ^[^[:space:]] ]]; then in_scope=0; continue; fi
      if [[ "$line" =~ ^[[:space:]]+tag:[[:space:]]+(.+)$ ]]; then
        scope_kind="tag"
        scope_val="${BASH_REMATCH[1]}"
      elif [[ "$line" =~ ^[[:space:]]+slugs:[[:space:]]+\[(.+)\]$ ]]; then
        scope_kind="slugs"
        scope_val="${BASH_REMATCH[1]}"
      elif [[ "$line" =~ ^[[:space:]]+query:[[:space:]]+(.+)$ ]]; then
        scope_kind="query"
        scope_val="${BASH_REMATCH[1]}"
      fi
    fi
  done < "$page"

  case "$scope_kind" in
    slugs)
      printf -- '%s' "$scope_val" | tr ',' '\n' | sed -E 's/^[[:space:]"'\'']+|[[:space:]"'\'']+$//g' | grep -v '^$' | sort -u
      ;;
    tag)
      # Walk the map; for each slug, check tags from the source file.
      [[ -f "$map" ]] || return 0
      local tag="$scope_val"
      while IFS=$'\t' read -r slug rel; do
        [[ -z "$slug" || -z "$rel" ]] && continue
        local p
        if [[ "$rel" = /* ]]; then p="$rel"; else p="$wiki_root/$rel"; fi
        [[ -f "$p" ]] || continue
        local tags_line
        tags_line=$(awk '/^tags:/{print; exit}' "$p")
        # tags: [memex, foo]
        if [[ "$tags_line" =~ \[(.*)\] ]]; then
          local raw="${BASH_REMATCH[1]}"
          local found=0
          local IFS_OLD="$IFS"
          IFS=','
          for t in $raw; do
            t="$(echo "$t" | sed -E 's/^[[:space:]"'\'']+|[[:space:]"'\'']+$//g')"
            if [[ "$t" == "$tag" ]]; then found=1; break; fi
          done
          IFS="$IFS_OLD"
          [[ "$found" -eq 1 ]] && echo "$slug"
        fi
      done < "$map" | sort -u
      ;;
    query)
      # Query-scope is non-deterministic; lint cannot verify in-scope without
      # running qmd. Emit a sentinel "" (empty) and let S4 short-circuit.
      return 0
      ;;
  esac
}

synth_check_s4() {
  local page="$1"
  local content_dir="${2:-content}"

  # If scope is query, skip S4 (cannot deterministically resolve).
  local scope_kind=""
  local in_fm=0 in_scope=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && in_fm=$((in_fm + 1)) && continue
    [[ "$in_fm" -ne 1 ]] && continue
    if [[ "$line" =~ ^scope:[[:space:]]*$ ]]; then in_scope=1; continue; fi
    if [[ "$in_scope" -eq 1 ]]; then
      if [[ "$line" =~ ^[^[:space:]] ]]; then in_scope=0; continue; fi
      [[ "$line" =~ ^[[:space:]]+tag:    ]] && scope_kind="tag"
      [[ "$line" =~ ^[[:space:]]+slugs:  ]] && scope_kind="slugs"
      [[ "$line" =~ ^[[:space:]]+query:  ]] && scope_kind="query"
    fi
  done < "$page"
  [[ "$scope_kind" = "query" ]] && return 0

  local resolved_slugs
  resolved_slugs="$(synth_resolve_scope_slugs "$page" "$content_dir")"
  [[ -n "$resolved_slugs" ]] || return 0

  local region
  region=$(awk '
    /^<!-- BEGIN GENERATED .* -->$/ { in_region=1; next }
    /^<!-- END GENERATED -->$/      { in_region=0 }
    in_region { print }
  ' "$page")

  local seen=" "
  local link target
  while IFS= read -r link; do
    target="${link%%|*}"
    [[ "$target" =~ ^[a-z0-9][a-z0-9-]*$ ]] || continue
    [[ "$seen" == *" $target "* ]] && continue
    seen="$seen$target "
    if ! grep -qxF "$target" <<<"$resolved_slugs"; then
      echo "LINT|ERROR|$page|S4: citation [[$target]] is out of scope"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[a-z0-9][a-z0-9|-]*\]\]' <<<"$region" | sed -E 's/^\[\[|\]\]$//g')
}

# --- S5: scope drift --------------------------------------------------------
# Recompute scope_hash from the page's scope: block; compare to the BEGIN
# marker's declared hash. Skip query-scoped pages (qmd is non-deterministic).
synth_check_s5() {
  local page="$1"
  local content_dir="${2:-content}"

  # Detect query scope; skip.
  local scope_kind=""
  local in_fm=0 in_scope=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && in_fm=$((in_fm + 1)) && continue
    [[ "$in_fm" -ne 1 ]] && continue
    if [[ "$line" =~ ^scope:[[:space:]]*$ ]]; then in_scope=1; continue; fi
    if [[ "$in_scope" -eq 1 ]]; then
      if [[ "$line" =~ ^[^[:space:]] ]]; then in_scope=0; continue; fi
      [[ "$line" =~ ^[[:space:]]+tag:    ]] && scope_kind="tag"
      [[ "$line" =~ ^[[:space:]]+slugs:  ]] && scope_kind="slugs"
      [[ "$line" =~ ^[[:space:]]+query:  ]] && scope_kind="query"
    fi
  done < "$page"
  [[ "$scope_kind" = "query" ]] && return 0

  local declared_hash
  declared_hash=$(grep -oE 'scope_hash=[a-f0-9]{6}' "$page" | head -1 | cut -d= -f2)
  [[ -n "$declared_hash" ]] || return 0

  local resolved_slugs
  resolved_slugs="$(synth_resolve_scope_slugs "$page" "$content_dir")"

  local current_hash
  current_hash=$(printf -- '%s\n' "$resolved_slugs" | sort -u | python3 scripts/lint-synth-hash.py)

  [[ -n "$current_hash" ]] || return 0
  if [[ "$declared_hash" != "$current_hash" ]]; then
    echo "LINT|WARN|$page|S5: scope drift (declared=$declared_hash current=$current_hash); consider regen"
    WARNS=$((WARNS + 1))
  fi
}

# --- S6: hand-edit inside markers -------------------------------------------
# Fires when frontmatter `last_updated` advanced past `last_generated` AND the
# working-tree diff vs HEAD intersects the BEGIN..END line range. Frontmatter-
# driven (not mtime); git-aware. Pages without a HEAD or untracked are skipped.
synth_check_s6() {
  local page="$1"
  local last_updated last_generated
  last_updated=$(awk -F': *' '/^---$/{c++} c==1 && /^last_updated:/{print $2; exit}' "$page" | tr -d '"')
  last_generated=$(awk -F': *' '/^---$/{c++} c==1 && /^last_generated:/{print $2; exit}' "$page" | tr -d '"')
  [[ -n "$last_updated" && -n "$last_generated" ]] || return 0
  # Compare lexicographically (ISO dates / timestamps are sort-safe).
  local lu_date lg_date
  lu_date="${last_updated:0:10}"
  lg_date="${last_generated:0:10}"
  [[ "$lu_date" > "$lg_date" ]] || return 0

  # Check git status: is page tracked? does HEAD have it?
  git ls-files --error-unmatch "$page" >/dev/null 2>&1 || return 0
  git show "HEAD:$page" >/dev/null 2>&1 || return 0

  # Compute begin/end line numbers in working tree.
  local begin_line end_line
  begin_line=$(grep -nE '^<!-- BEGIN GENERATED .* -->$' "$page" | head -1 | cut -d: -f1)
  end_line=$(grep -nE '^<!-- END GENERATED -->$' "$page" | head -1 | cut -d: -f1)
  [[ -n "$begin_line" && -n "$end_line" ]] || return 0

  # Diff hunks vs HEAD; check if any hunk's working-tree line range intersects
  # [begin_line+1, end_line-1]. Use unified diff with -U0.
  local diff_intersects=0
  local hunk start len hunk_end region_start region_end
  while IFS= read -r hunk; do
    [[ "$hunk" =~ ^@@\ -[0-9,]+\ \+([0-9]+)(,([0-9]+))?\ @@ ]] || continue
    start="${BASH_REMATCH[1]}"
    len="${BASH_REMATCH[3]:-1}"
    hunk_end=$((start + len - 1))
    region_start=$((begin_line + 1))
    region_end=$((end_line - 1))
    if [[ "$start" -le "$region_end" && "$hunk_end" -ge "$region_start" ]]; then
      diff_intersects=1
      break
    fi
  done < <(git diff -U0 -- "$page" 2>/dev/null | grep '^@@')

  if [[ "$diff_intersects" -eq 1 ]]; then
    echo "LINT|WARN|$page|S6: hand-edit inside generated region (last_updated=$last_updated > last_generated=$last_generated)"
    WARNS=$((WARNS + 1))
  fi
}

# --- S9: aggregate evidence-quote word cap ----------------------------------
synth_get_plugin_max_evidence_total_words() {
  local plugin="$1"
  local manifest="synthesis-plugins/$plugin.md"
  [[ -f "$manifest" ]] || { echo 500; return; }
  local v
  v=$(awk -F': *' '/^---$/{c++} c==1 && /^max_evidence_total_words:/{print $2; exit}' "$manifest" | tr -d '"')
  [[ -n "$v" ]] || v=500
  echo "$v"
}

synth_check_s9() {
  local page="$1"
  local plugin
  plugin=$(awk '/^plugin: /{print $2; exit}' "$page")
  [[ -n "$plugin" ]] || return 0
  local cap
  cap=$(synth_get_plugin_max_evidence_total_words "$plugin")

  local total
  total=$(awk '
    /^<!-- BEGIN GENERATED .* -->$/ { in_region=1; next }
    /^<!-- END GENERATED -->$/      { in_region=0 }
    in_region && /^>[[:space:]]+".+"[[:space:]]+—[[:space:]]+\[\[[a-z0-9-]+\]\]/ {
      # Extract the quote body between the first and last " in the line.
      line = $0
      sub(/^>[[:space:]]+"/, "", line)
      sub(/"[[:space:]]+—.*$/, "", line)
      n = split(line, w, /[[:space:]]+/)
      sum += n
    }
    END { print (sum+0) }
  ' "$page")

  if [[ "$total" -gt "$cap" ]]; then
    echo "LINT|ERROR|$page|S9: aggregate evidence words=$total exceeds plugin cap max_evidence_total_words=$cap"
    ERRORS=$((ERRORS + 1))
  fi
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
  synth_check_s4 "$page" "$content_dir"
  synth_check_s5 "$page" "$content_dir"
  synth_check_s6 "$page"
  synth_check_s9 "$page"
}

synth_lint_dir() {
  local content_dir="${1:-content}"
  local synth_dir="$content_dir/synthesis"
  [[ -d "$synth_dir" ]] || return 0
  while IFS= read -r -d '' page; do
    synth_lint_file "$page" "$content_dir"
  done < <(find "$synth_dir" -maxdepth 2 -name '*.md' -type f -print0)
}
