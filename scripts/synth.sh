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
    new|regen|accept-stage|finalize|resolve|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
    *) EXIT_CODE=1 die "unknown subcommand: $sub" ;;
  esac
}

main "$@"
