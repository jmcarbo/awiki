#!/usr/bin/env bash
# Source-only helper. Defines:
#   synth_plugin_load <name>      → sets SYNTH_PLUGIN_* vars; returns 0/1
#   synth_plugin_list_all         → echoes one plugin name per line
#   synth_plugin_extract_prompt   → echoes prompt body to stdout for last loaded plugin
# Caller is responsible for set -e discipline.

SYNTH_PLUGIN_NAME_REGEX='^[a-z][a-z0-9-]*$'

synth_plugin_load() {
  local name="$1"
  local plugins_dir="${AWIKI_SYNTH_PLUGINS_DIR:-synthesis-plugins}"

  # Validate name regex BEFORE any directory scan — defeats traversal attempts.
  if ! [[ "$name" =~ $SYNTH_PLUGIN_NAME_REGEX ]]; then
    echo "ERROR: invalid plugin name '$name' (must match $SYNTH_PLUGIN_NAME_REGEX)" >&2
    return 1
  fi

  local single_file="$plugins_dir/$name.md"
  local dir_form="$plugins_dir/$name/plugin.yaml"

  if [[ -f "$single_file" && -f "$dir_form" ]]; then
    echo "ERROR: plugin '$name' has both single-file and directory form; remove one" >&2
    return 1
  fi
  if [[ -f "$dir_form" ]]; then
    echo "ERROR: directory-form plugins not yet supported (phase 14); use single-file <name>.md" >&2
    return 1
  fi
  if [[ ! -f "$single_file" ]]; then
    echo "ERROR: plugin not found: $name (looked at $single_file)" >&2
    return 1
  fi

  # Reset state from prior loads.
  SYNTH_PLUGIN_NAME=""; SYNTH_PLUGIN_DESCRIPTION=""; SYNTH_PLUGIN_VERSION="1"
  SYNTH_PLUGIN_OUTPUT_TYPE=""; SYNTH_PLUGIN_OUTPUT_SUBTYPE=""
  SYNTH_PLUGIN_MIN_SOURCES=""; SYNTH_PLUGIN_MAX_SOURCES=""
  SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS="500"
  SYNTH_PLUGIN_REQUIRED_SECTIONS=""
  SYNTH_PLUGIN_POST_HOOK=""
  SYNTH_PLUGIN_RENDER=""
  SYNTH_PLUGIN_PATH="$single_file"

  # Extract frontmatter (between first two `---` lines).
  local fm
  fm="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==1 {print}' "$single_file")"

  # Scalar fields via awk (the manifest shape is small, fixed, and our own).
  SYNTH_PLUGIN_NAME="$(printf -- '%s\n' "$fm" | awk -F': ' '/^name: /{print $2; exit}')"
  SYNTH_PLUGIN_DESCRIPTION="$(printf -- '%s\n' "$fm" | awk '/^description: /{sub(/^description: /,""); print; exit}')"
  SYNTH_PLUGIN_VERSION="$(printf -- '%s\n' "$fm" | awk -F': ' '/^version: /{print $2; exit}')"
  SYNTH_PLUGIN_OUTPUT_TYPE="$(printf -- '%s\n' "$fm" | awk -F': ' '/^output_type: /{print $2; exit}')"
  SYNTH_PLUGIN_OUTPUT_SUBTYPE="$(printf -- '%s\n' "$fm" | awk -F': ' '/^output_subtype: /{print $2; exit}')"
  SYNTH_PLUGIN_MIN_SOURCES="$(printf -- '%s\n' "$fm" | awk -F': ' '/^min_sources: /{print $2; exit}')"
  SYNTH_PLUGIN_MAX_SOURCES="$(printf -- '%s\n' "$fm" | awk -F': ' '/^max_sources: /{print $2; exit}')"
  SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS="$(printf -- '%s\n' "$fm" | awk -F': ' '/^max_evidence_total_words: /{print $2; exit}')"
  SYNTH_PLUGIN_POST_HOOK="$(printf -- '%s\n' "$fm" | awk -F': ' '/^post_hook: /{print $2; exit}')"
  SYNTH_PLUGIN_RENDER="$(printf -- '%s\n' "$fm" | awk -F': ' '/^render: /{print $2; exit}')"

  # required_sections is a YAML list. Two accepted forms: inline `[a, b]` or block `- "a"`.
  SYNTH_PLUGIN_REQUIRED_SECTIONS="$(printf -- '%s\n' "$fm" | awk '
    BEGIN{ in_list=0 }
    /^required_sections:[[:space:]]*\[/ {
      line=$0; sub(/^required_sections:[[:space:]]*\[/,"",line); sub(/\][[:space:]]*$/,"",line)
      n=split(line, parts, ",")
      for (i=1; i<=n; i++) {
        gsub(/^[[:space:]]*"?|"?[[:space:]]*$/, "", parts[i])
        gsub(/^[[:space:]]*'\''?|'\''?[[:space:]]*$/, "", parts[i])
        if (parts[i] != "") print parts[i]
      }
      next
    }
    /^required_sections:[[:space:]]*$/ { in_list=1; next }
    in_list==1 && /^[^[:space:]-]/ { in_list=0 }
    in_list==1 && /^[[:space:]]+-[[:space:]]+/ {
      line=$0
      sub(/^[[:space:]]+-[[:space:]]+/, "", line)
      gsub(/^["'\'']|["'\'']$/, "", line)
      print line
    }
  ')"

  # Defaults
  [[ -z "$SYNTH_PLUGIN_VERSION" ]] && SYNTH_PLUGIN_VERSION="1"
  [[ -z "$SYNTH_PLUGIN_OUTPUT_SUBTYPE" ]] && SYNTH_PLUGIN_OUTPUT_SUBTYPE="$SYNTH_PLUGIN_NAME"
  [[ -z "$SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS" ]] && SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS="500"
  [[ "$SYNTH_PLUGIN_POST_HOOK" == "null" ]] && SYNTH_PLUGIN_POST_HOOK=""

  # Required-field validation.
  for f in NAME DESCRIPTION OUTPUT_TYPE MIN_SOURCES; do
    local var="SYNTH_PLUGIN_$f"
    if [[ -z "${!var:-}" ]]; then
      echo "ERROR: plugin manifest $name missing required field: $(echo "$f" | tr 'A-Z_' 'a-z-' | tr '_' '-')" >&2
      return 1
    fi
  done
  if [[ -z "$SYNTH_PLUGIN_REQUIRED_SECTIONS" ]]; then
    echo "ERROR: plugin manifest $name missing required field: required_sections" >&2
    return 1
  fi
  if [[ "$SYNTH_PLUGIN_NAME" != "$name" ]]; then
    echo "ERROR: plugin filename ($name.md) does not match manifest name ($SYNTH_PLUGIN_NAME)" >&2
    return 1
  fi

  return 0
}

synth_plugin_extract_prompt() {
  # Echo body after second `---` line. Caller must have run synth_plugin_load first.
  if [[ -z "${SYNTH_PLUGIN_PATH:-}" ]]; then
    echo "ERROR: no plugin loaded" >&2
    return 1
  fi
  awk 'BEGIN{c=0} /^---$/ {c++; next} c>=2 {print}' "$SYNTH_PLUGIN_PATH"
}

synth_plugin_list_all() {
  local plugins_dir="${AWIKI_SYNTH_PLUGINS_DIR:-synthesis-plugins}"
  if [[ ! -d "$plugins_dir" ]]; then return 0; fi
  find "$plugins_dir" -maxdepth 1 -type f -name '*.md' -print0 \
    | xargs -0 -n1 basename \
    | sed 's/\.md$//' \
    | sort
}
