#!/usr/bin/env bash
set -euo pipefail

# scripts/dataset.sh — manage awiki datasets.
# Subcommands: new, compact, validate.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

# shellcheck source=/dev/null
source "$REPO_ROOT/scripts/lib/dataset-fm.sh"

ROWS_PY="$REPO_ROOT/scripts/lib/dataset-rows.py"
DATASETS_DIR="content/datasets"
DATA_DIR="data"
SLUG_RE='^[a-z0-9][a-z0-9-]*$'
FORMAT_RE='^(csv|tsv|json|dsv|topojson)$'

note() { echo "DATASET|$*"; }
die() { echo "DATASET|ERROR|$*" >&2; exit 1; }

cmd_new() {
  local slug="" format="csv" from=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --format=*) format="${1#--format=}" ;;
      --from=*) from="${1#--from=}" ;;
      --) shift; break ;;
      -*) die "unknown flag: $1" ;;
      *) slug="$1" ;;
    esac
    shift
  done
  [[ -n "$slug" ]] || die "missing <slug>"
  [[ "$slug" =~ $SLUG_RE ]] || die "invalid slug: $slug (must match $SLUG_RE)"
  [[ "$format" =~ $FORMAT_RE ]] || die "unsupported format: $format"
  local page="$DATASETS_DIR/$slug.md"
  [[ ! -e "$page" ]] || die "$page already exists"
  mkdir -p "$DATASETS_DIR"

  local today
  today="$(date '+%Y-%m-%d')"
  local rows=0
  local body=""
  if [[ -n "$from" ]]; then
    [[ -f "$from" ]] || die "source file not found: $from"
    rows="$(python3 "$ROWS_PY" count --format="$format" --file="$from")"
    body="$(cat "$from")"
  fi

  {
    printf -- '---\n'
    printf -- 'title: "%s"\n' "$slug"
    printf -- 'date: %s\n' "$today"
    printf -- 'last_updated: %s\n' "$today"
    printf -- 'type: dataset\n'
    printf -- 'tags: []\n'
    printf -- 'storage: inline\n'
    printf -- 'format: %s\n' "$format"
    printf -- 'rows: %s\n' "$rows"
    printf -- 'sources: []\n'
    printf -- 'draft: false\n'
    printf -- '---\n\n'
    printf -- '# %s\n\n' "$slug"
    printf -- '<!-- one-paragraph description here -->\n\n'
    printf -- '## Schema\n\n'
    printf -- '<!-- describe each column here -->\n\n'
    printf -- '## Data\n'
    printf -- '```%s\n' "$format"
    if [[ -n "$body" ]]; then
      printf -- '%s\n' "$body"
    fi
    printf -- '```\n\n'
    printf -- '## Provenance\n\n'
    printf -- '<!-- where these rows came from -->\n\n'
    printf -- '## Related\n\n'
    printf -- '## Sources\n'
  } > "$page"
  note "created $page (rows=$rows)"
}

cmd_compact() { die "compact not implemented yet (Task 9)"; }
cmd_validate() { die "validate not implemented yet (Task 8)"; }

main() {
  [[ $# -ge 1 ]] || die "usage: dataset.sh <new|compact|validate> [args...]"
  local cmd="$1"; shift
  case "$cmd" in
    new) cmd_new "$@" ;;
    compact) cmd_compact "$@" ;;
    validate) cmd_validate "$@" ;;
    *) die "unknown subcommand: $cmd" ;;
  esac
}

main "$@"
