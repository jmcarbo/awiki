#!/usr/bin/env bash
# Source-only helper. Read/write per-repo state JSON for git-docs ingest.
# Functions:
#   awiki_git_state_path <repo_key>             → echoes .awiki/git-state/<repo_key>.json
#   awiki_git_state_load <repo_key>             → prints JSON; '{}' if missing
#   awiki_git_state_save <repo_key> <json>      → atomic write (tmp + rename)
#   awiki_git_state_validate <json>             → exits 0 if schema=1 and required keys present
#   awiki_git_state_validate_repo_key <repo_key> → exits 0 if repo_key is safe (no path traversal)

awiki_git_state_validate_repo_key() {
  local repo_key="$1"
  if [[ -z "$repo_key" ]]; then
    echo "ERROR: repo_key is empty" >&2
    return 1
  fi
  if [[ "$repo_key" =~ \.\. ]] || [[ "$repo_key" == /* ]] || [[ "$repo_key" =~ / ]]; then
    echo "ERROR: repo_key contains path-traversal chars: $repo_key" >&2
    return 1
  fi
  return 0
}

awiki_git_state_path() {
  local repo_key="$1"
  awiki_git_state_validate_repo_key "$repo_key" || return 1
  echo ".awiki/git-state/${repo_key}.json"
}

awiki_git_state_load() {
  local path
  path="$(awiki_git_state_path "$1")" || return 1
  if [[ -f "$path" ]]; then
    cat "$path"
  else
    echo "{}"
  fi
}

awiki_git_state_save() {
  local repo_key="$1" json="$2"
  awiki_git_state_validate_repo_key "$repo_key" || return 1
  local path
  path="$(awiki_git_state_path "$repo_key")" || return 1
  mkdir -p "$(dirname "$path")"
  local tmp="${path}.tmp.$$"
  printf '%s\n' "$json" > "$tmp"
  mv -f "$tmp" "$path"
}

awiki_git_state_validate() {
  local json="$1"
  if ! command -v python3 >/dev/null 2>&1; then
    echo "ERROR: python3 required for state validation" >&2
    return 1
  fi
  python3 - <<'PY' "$json"
import json, sys
try:
    obj = json.loads(sys.argv[1])
except Exception as e:
    print(f"ERROR: invalid JSON: {e}", file=sys.stderr); sys.exit(1)
if obj.get("schema") != 1:
    print("ERROR: schema must be 1", file=sys.stderr); sys.exit(1)
for key in ("repo_key","repo_name","url","default_branch","head_sha","ingested_at","private","files"):
    if key not in obj:
        print(f"ERROR: missing key: {key}", file=sys.stderr); sys.exit(1)
sys.exit(0)
PY
}
