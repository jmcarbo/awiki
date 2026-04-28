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
    command -v python3 >/dev/null 2>&1 || die "python3 required for --from row counting"
    rows="$(python3 "$ROWS_PY" count --format="$format" --file="$from")"
    [[ "$rows" =~ ^[0-9]+$ ]] || die "row counter returned non-numeric: '$rows'"
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

# Extract the inline ```<format>``` block from a dataset page to stdout.
_extract_inline() {
  local page="$1" format="$2"
  awk -v fmt="$format" '
    /^## Data[[:space:]]*$/ { in_data=1; next }
    in_data && match($0, "^```" fmt "[[:space:]]*$") { in_block=1; next }
    in_block && /^```[[:space:]]*$/ { in_block=0; in_data=0; next }
    in_block { print }
  ' "$page"
}

# Extract `columns:` YAML block to a JSON file. Empty file if no columns.
_columns_to_json() {
  local page="$1" out="$2"
  python3 - "$page" "$out" <<'PY'
import re, sys, json
page, out = sys.argv[1], sys.argv[2]
with open(page) as f:
    txt = f.read()
m = re.search(r"^---\s*\n(.*?)\n---\s*$", txt, re.M | re.S)
if not m:
    open(out, "w").write("[]"); sys.exit(0)
fm = m.group(1)
# Crude columns parser: lines like `  - { name: x, type: y }` or block style.
cols = []
in_cols = False
for line in fm.splitlines():
    if line.startswith("columns:"):
        in_cols = True; continue
    if in_cols:
        if line and not line.startswith((" ", "\t")):
            in_cols = False
            continue
        item = line.strip()
        if not item.startswith("-"):
            continue
        body = item[1:].strip()
        if body.startswith("{") and body.endswith("}"):
            inner = body[1:-1]
            entry = {}
            for part in inner.split(","):
                if ":" not in part:
                    continue
                k, v = part.split(":", 1)
                entry[k.strip()] = v.strip().strip('"').strip("'")
            cols.append(entry)
open(out, "w").write(json.dumps(cols))
PY
}

cmd_validate() {
  local slug=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --) shift; break ;;
      -*) die "unknown flag: $1" ;;
      *) slug="$1" ;;
    esac
    shift
  done
  [[ -n "$slug" ]] || die "usage: dataset.sh validate <slug>"
  local page="$DATASETS_DIR/$slug.md"
  [[ -f "$page" ]] || die "$page not found"

  local storage format
  storage="$(fm_get "$page" storage)"
  format="$(fm_get "$page" format)"
  [[ -n "$storage" ]] || die "missing storage in $page"
  [[ -n "$format" ]] || die "missing format in $page"

  local data_file
  if [[ "$storage" == "file" ]]; then
    data_file="$(fm_get "$page" data_path)"
    [[ -f "$data_file" ]] || die "data_path not found: $data_file"
  else
    data_file="$(mktemp)"
    _extract_inline "$page" "$format" > "$data_file"
    trap "rm -f $data_file" EXIT
  fi

  # Validate against schema if present.
  local schema_file
  schema_file="$(mktemp)"
  _columns_to_json "$page" "$schema_file"
  if [[ "$(cat "$schema_file")" != "[]" ]]; then
    if ! python3 "$ROWS_PY" validate --format="$format" --file="$data_file" --schema="$schema_file"; then
      rm -f "$schema_file"
      die "schema validation failed for $slug"
    fi
  fi
  rm -f "$schema_file"

  # Refresh rows count.
  local actual
  actual="$(python3 "$ROWS_PY" count --format="$format" --file="$data_file")"
  [[ "$actual" =~ ^[0-9]+$ ]] || die "row counter returned non-numeric: '$actual'"
  fm_set "$page" rows "$actual"
  note "validated $slug (rows=$actual)"
}

cmd_compact() { die "compact not implemented yet (Task 9)"; }

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
