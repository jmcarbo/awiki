#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# shellcheck source=lib/lock.sh
source "$SCRIPT_DIR/lib/lock.sh"
# shellcheck source=lib/git-clone.sh
source "$SCRIPT_DIR/lib/git-clone.sh"
# shellcheck source=lib/git-state.sh
source "$SCRIPT_DIR/lib/git-state.sh"
# shellcheck source=lib/git-config.sh
source "$SCRIPT_DIR/lib/git-config.sh"

usage() {
  cat >&2 <<USAGE
usage: ingest-git.sh <repo-spec> [flags]

  <repo-spec>          local path | https URL | git@ URL | config alias
  --paths=<csv>        override include paths (default README.md,docs/,rfcs/,adr/)
  --private            force private routing
  --protect-edits      stage conflicts under raw/inbox/checkpoint/.staged/
  --summarize          (reserved; not implemented v1)
  --dry-run            print plan + exit 0; no writes
  --repo-name=<name>   override derived name for slug prefix + entity page
USAGE
}

if [[ $# -lt 1 ]]; then
  usage; exit 1
fi

SPEC="$1"; shift
PATHS_OVERRIDE=""
PRIVATE_FLAG=""
PROTECT_EDITS=""
SUMMARIZE=""
DRY_RUN=""
REPO_NAME_OVERRIDE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --paths=*)        PATHS_OVERRIDE="${1#--paths=}";;
    --private)        PRIVATE_FLAG=1;;
    --protect-edits)  PROTECT_EDITS=1;;
    --summarize)      SUMMARIZE=1;;
    --dry-run)        DRY_RUN=1;;
    --repo-name=*)    REPO_NAME_OVERRIDE="${1#--repo-name=}";;
    -h|--help)        usage; exit 0;;
    *) echo "ERROR: unknown flag: $1" >&2; usage; exit 1;;
  esac
  shift
done

REPO_KEY="$(awiki_git_clone_repo_key "$SPEC")"
REPO_NAME="${REPO_NAME_OVERRIDE:-${REPO_KEY#local-}}"

if awiki_git_clone_is_ssh "$SPEC"; then
  PRIVATE_FLAG=1
fi

# Lock dir
mkdir -p .awiki/lock
LOCK_FILE=".awiki/lock/git-${REPO_KEY}"
: > "$LOCK_FILE.flockfile"

# Per-repo flock (separate from .awiki/lock single-file used by lib/lock.sh).
exec 9>"$LOCK_FILE.flockfile"
if ! flock -w "$AWIKI_LOCK_TIMEOUT_USER" -x 9; then
  echo "ERROR: lock acquire timeout for $REPO_KEY" >&2
  exit 13
fi

# Resolve checkout
RESOLVED="$(awiki_git_clone_resolve "$SPEC" "$REPO_KEY")" || {
  rc=$?; echo "ERROR: resolve failed (rc=$rc)" >&2; exit "$rc";
}
IFS='|' read -r CHECKOUT HEAD_SHA DEFAULT_BRANCH <<<"$RESOLVED"

# Determine include paths
DEFAULT_PATHS="README.md,docs/,rfcs/,adr/"
PATHS="${PATHS_OVERRIDE:-$DEFAULT_PATHS}"

# Walk repo: list every *.md / *.mdx, then filter by paths + exclude vendored
mapfile -t ALL_MD < <(git -C "$CHECKOUT" ls-tree -r HEAD --name-only 2>/dev/null | grep -E '\.(md|mdx)$' | LC_ALL=C sort)

filter_paths() {
  local rel="$1"
  case "$rel" in
    node_modules/*|*/node_modules/*|vendor/*|*/vendor/*|.git/*|*/.git/*) return 1;;
  esac
  IFS=',' read -ra ENTRIES <<<"$PATHS"
  for entry in "${ENTRIES[@]}"; do
    if [[ -z "$entry" ]]; then continue; fi
    if [[ "$entry" == */ ]]; then
      [[ "$rel" == "${entry}"* ]] && return 0
    else
      [[ "$rel" == "$entry" ]] && return 0
    fi
  done
  return 1
}

CURRENT_FILES=()
for rel in "${ALL_MD[@]}"; do
  if filter_paths "$rel"; then
    CURRENT_FILES+=("$rel")
  fi
done

# Build map repo-relpath → blob_sha
declare -A CURRENT_BLOB
for rel in "${CURRENT_FILES[@]}"; do
  CURRENT_BLOB["$rel"]="$(git -C "$CHECKOUT" rev-parse "HEAD:$rel" 2>/dev/null || echo "")"
done

# Load prior state
PRIOR_JSON="$(awiki_git_state_load "$REPO_KEY" 2>/dev/null || echo '{}')"
declare -A PRIOR_BLOB PRIOR_SLUG
while IFS=$'\t' read -r prel pblob pslug; do
  [[ -z "$prel" ]] && continue
  PRIOR_BLOB["$prel"]="$pblob"
  PRIOR_SLUG["$prel"]="$pslug"
done < <(printf '%s\n' "$PRIOR_JSON" | python3 -c '
import json, sys
try:
    obj = json.loads(sys.stdin.read() or "{}")
except Exception:
    obj = {}
for rel, info in (obj.get("files") or {}).items():
    blob = info.get("blob_sha", "")
    slug = info.get("slug", "")
    print(rel + "\t" + blob + "\t" + slug)
')

# Diff
ADDED=()
MODIFIED=()
UNCHANGED=()
REMOVED=()
for rel in "${CURRENT_FILES[@]}"; do
  pblob="${PRIOR_BLOB[$rel]:-}"
  cblob="${CURRENT_BLOB[$rel]}"
  if [[ -z "$pblob" ]]; then
    ADDED+=("$rel")
  elif [[ "$pblob" != "$cblob" ]]; then
    MODIFIED+=("$rel")
  else
    UNCHANGED+=("$rel")
  fi
done
for prel in "${!PRIOR_BLOB[@]}"; do
  if [[ -z "${CURRENT_BLOB[$prel]:-}" ]]; then
    REMOVED+=("$prel")
  fi
done

flatten_slug() {
  local rel="$1"
  local stem="${rel%.md}"
  stem="${stem%.mdx}"
  stem="${stem,,}"
  stem="${stem//\//-}"
  echo "git-${REPO_NAME}-${stem}"
}

declare -A SLUG_MAP
for rel in "${CURRENT_FILES[@]}"; do
  SLUG_MAP["$rel"]="$(flatten_slug "$rel")"
done

declare -A SEEN_SLUG
for rel in "${CURRENT_FILES[@]}"; do
  s="${SLUG_MAP[$rel]}"
  if [[ -n "${SEEN_SLUG[$s]:-}" ]]; then
    echo "ERROR: slug collision: $s ← ${SEEN_SLUG[$s]} and $rel" >&2
    exit 14
  fi
  SEEN_SLUG[$s]="$rel"
done

SLUG_MAP_JSON="$(python3 -c '
import json, sys
m = {}
for line in sys.stdin:
    line = line.rstrip("\n")
    if not line: continue
    rel, slug = line.split("\t", 1)
    m[rel] = slug
print(json.dumps(m))
' < <(for r in "${!SLUG_MAP[@]}"; do printf '%s\t%s\n' "$r" "${SLUG_MAP[$r]}"; done))"

if [[ -n "${PRIVATE_FLAG:-}" ]]; then
  OUT_SOURCES="content/private/sources"
  OUT_ENTITIES="content/private/entities"
  ASSET_OUT_DIR="content/private/sources/_assets/git-${REPO_NAME}"
else
  OUT_SOURCES="content/sources"
  OUT_ENTITIES="content/entities"
  ASSET_OUT_DIR="content/sources/_assets/git-${REPO_NAME}"
fi
mkdir -p "$OUT_SOURCES" "$OUT_ENTITIES" "$ASSET_OUT_DIR"

ENTITY_PATH="$OUT_ENTITIES/repo-${REPO_NAME}.md"
if [[ -f "$ENTITY_PATH" ]]; then
  existing_url="$(awk -F': ' '/^git_url: /{print $2; exit}' "$ENTITY_PATH" || true)"
  if [[ -n "$existing_url" && "$existing_url" != "$SPEC" && -z "${REPO_NAME_OVERRIDE:-}" ]]; then
    echo "ERROR: repo entity $ENTITY_PATH already exists with git_url=$existing_url; pass --repo-name=<override>" >&2
    exit 15
  fi
fi

if [[ -n "$DRY_RUN" ]]; then
  echo "PLAN|spec=$SPEC|repo_key=$REPO_KEY|repo_name=$REPO_NAME|checkout=$CHECKOUT|head=$HEAD_SHA|branch=$DEFAULT_BRANCH|private=${PRIVATE_FLAG:-0}"
  echo "PLAN|added=${#ADDED[@]}|modified=${#MODIFIED[@]}|removed=${#REMOVED[@]}|unchanged=${#UNCHANGED[@]}"
  for f in "${ADDED[@]}";    do echo "PLAN|add|$f"; done
  for f in "${MODIFIED[@]}"; do echo "PLAN|mod|$f"; done
  for f in "${REMOVED[@]}";  do echo "PLAN|rem|$f"; done
  exit 0
fi

TO_WRITE=("${ADDED[@]+"${ADDED[@]}"}" "${MODIFIED[@]+"${MODIFIED[@]}"}")
WRITE_OK=0
WRITE_FAIL=0
PRIVATE_ARG=""
[[ -n "${PRIVATE_FLAG:-}" ]] && PRIVATE_ARG="--private"

for rel in "${TO_WRITE[@]}"; do
  slug="${SLUG_MAP[$rel]}"
  out="$OUT_SOURCES/${slug}.md"
  if printf '%s' "$SLUG_MAP_JSON" | python3 "$SCRIPT_DIR/ingest-git-transform.py" \
        --in "$CHECKOUT/$rel" --out "$out" \
        --repo-key "$REPO_KEY" --repo-name "$REPO_NAME" --repo-relpath "$rel" \
        --git-url "$SPEC" --git-blob-sha "${CURRENT_BLOB[$rel]}" \
        --asset-out-dir "$ASSET_OUT_DIR" \
        --upstream-root "$CHECKOUT" \
        $PRIVATE_ARG ; then
    WRITE_OK=$((WRITE_OK+1))
  else
    WRITE_FAIL=$((WRITE_FAIL+1))
    echo "FAIL|transform|$rel" >&2
  fi
done

# Move derived pages for removed upstream files to graveyard
if [[ "${#REMOVED[@]}" -gt 0 ]]; then
  graveyard="raw/_originals/git/${REPO_KEY}"
  mkdir -p "$graveyard"
  for prel in "${REMOVED[@]}"; do
    slug="${PRIOR_SLUG[$prel]}"
    [[ -z "$slug" ]] && continue
    src="$OUT_SOURCES/${slug}.md"
    if [[ -f "$src" ]]; then
      mv -f "$src" "$graveyard/${slug}.md"
      echo "REMOVED|$prel|→|$graveyard/${slug}.md"
    fi
  done
fi

TODAY="$(date -u +%F)"
NOW_ISO="$(date -u +%FT%TZ)"

# Persist state JSON so next run can diff correctly (schema §7.2)
NEW_FILES_JSON="$(AWIKI_NOW_ISO="$NOW_ISO" python3 -c '
import json, sys, os
files = {}
now_iso = os.environ.get("AWIKI_NOW_ISO", "")
for line in sys.stdin:
    line = line.rstrip("\n")
    if not line: continue
    rel, blob, slug, out = line.split("\t")
    files[rel] = {"blob_sha": blob, "slug": slug, "out_path": out, "last_ingested": now_iso}
print(json.dumps(files))
' < <(
  for rel in "${CURRENT_FILES[@]}"; do
    slug="${SLUG_MAP[$rel]}"
    printf "%s\t%s\t%s\t%s\n" "$rel" "${CURRENT_BLOB[$rel]}" "$slug" "$OUT_SOURCES/${slug}.md"
  done
))"

STATE_JSON="$(python3 -c '
import json, sys
files = json.loads(sys.argv[1])
out = {
    "schema": 1,
    "repo_key": sys.argv[2],
    "repo_name": sys.argv[3],
    "url": sys.argv[4],
    "default_branch": sys.argv[5],
    "head_sha": sys.argv[6],
    "ingested_at": sys.argv[7],
    "private": sys.argv[8] == "1",
    "files": files,
}
print(json.dumps(out, indent=2))
' "$NEW_FILES_JSON" "$REPO_KEY" "$REPO_NAME" "$SPEC" "$DEFAULT_BRANCH" "$HEAD_SHA" "$NOW_ISO" "${PRIVATE_FLAG:-0}")"

awiki_git_state_save "$REPO_KEY" "$STATE_JSON"

{
  echo "---"
  echo "title: \"${REPO_NAME}\""
  echo "date: ${TODAY}"
  echo "last_updated: ${TODAY}"
  echo "type: entity"
  if [[ -n "${PRIVATE_FLAG:-}" ]]; then
    echo "tags: [git, repo, private]"
  else
    echo "tags: [git, repo]"
  fi
  echo "aliases: [${REPO_NAME}]"
  echo "git_url: ${SPEC}"
  echo "git_default_branch: ${DEFAULT_BRANCH}"
  echo "git_sha: ${HEAD_SHA}"
  echo "last_ingested: ${NOW_ISO}"
  echo "---"
  echo
  echo "Repository \`${REPO_NAME}\` ingested from \`${SPEC}\`."
  echo
  echo "## Sources"
  echo
  for rel in "${CURRENT_FILES[@]}"; do
    echo "- [[${SLUG_MAP[$rel]}]]"
  done
} > "$ENTITY_PATH.tmp.$$"
mv -f "$ENTITY_PATH.tmp.$$" "$ENTITY_PATH"

echo "OK|repo_key=$REPO_KEY|added=${#ADDED[@]}|modified=${#MODIFIED[@]}|removed=${#REMOVED[@]}|written=$WRITE_OK|failed=$WRITE_FAIL"

if [[ "$WRITE_FAIL" -gt 0 ]]; then exit 2; fi
