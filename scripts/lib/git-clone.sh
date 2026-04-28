#!/usr/bin/env bash
# Source-only helper. Resolve <repo-spec> to a local checkout for ingest-git.sh.

awiki_git_clone_is_ssh() {
  [[ "${1:-}" == git@* ]]
}

awiki_git_clone_repo_key() {
  local spec="${1:-}"
  if [[ -z "$spec" ]]; then
    echo "ERROR: repo_key needs spec" >&2; return 1
  fi
  # Local path branch
  if [[ -d "$spec" || "$spec" = /* || "$spec" = ./* || "$spec" = ../* ]]; then
    local rp
    rp="$(cd "$spec" 2>/dev/null && pwd || true)"
    if [[ -z "$rp" ]]; then rp="$spec"; fi
    echo "local-$(basename "$rp")"
    return 0
  fi
  # file:// URL
  if [[ "$spec" =~ ^file:// ]]; then
    local p="${spec#file://}"
    echo "local-$(basename "$p" .git)"
    return 0
  fi
  # https / http URL: https://host/owner/repo(.git)
  if [[ "$spec" =~ ^https?://([^/]+)/(.+)$ ]]; then
    local host="${BASH_REMATCH[1]}" rest="${BASH_REMATCH[2]}"
    rest="${rest%.git}"
    rest="${rest//\//-}"
    host="${host//./-}"
    echo "${host}-${rest}"
    return 0
  fi
  # ssh URL: git@host:owner/repo(.git)
  if [[ "$spec" =~ ^git@([^:]+):(.+)$ ]]; then
    local host="${BASH_REMATCH[1]}" rest="${BASH_REMATCH[2]}"
    rest="${rest%.git}"
    rest="${rest//\//-}"
    host="${host//./-}"
    echo "${host}-${rest}"
    return 0
  fi
  echo "ERROR: unrecognized repo spec: $spec" >&2
  return 1
}

awiki_git_clone_resolve() {
  local spec="${1:-}" repo_key="${2:-}"
  if [[ -z "$spec" || -z "$repo_key" ]]; then
    echo "ERROR: resolve needs <spec> <repo_key>" >&2; return 1
  fi

  local checkout=""
  if [[ -d "$spec" ]]; then
    checkout="$(cd "$spec" && pwd)"
  elif [[ "$spec" =~ ^file:// ]]; then
    checkout="${spec#file://}"
  else
    # remote URL — clone or fetch under raw/_git-cache/
    checkout="raw/_git-cache/${repo_key}"
    if [[ ! -d "$checkout/.git" ]]; then
      mkdir -p raw/_git-cache
      git clone --filter=blob:none --quiet "$spec" "$checkout" || return 10
    else
      git -C "$checkout" fetch --quiet --prune origin || return 11
      local default_branch
      default_branch="$(git -C "$checkout" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||' || echo main)"
      git -C "$checkout" reset --quiet --hard "origin/${default_branch}" || return 11
    fi
  fi

  local sha
  sha="$(git -C "$checkout" rev-parse HEAD 2>/dev/null)" || return 11
  local branch
  branch="$(git -C "$checkout" rev-parse --abbrev-ref HEAD 2>/dev/null)" || branch="HEAD"
  if [[ "$branch" = "HEAD" ]]; then
    branch="$(git -C "$checkout" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||' || echo main)"
  fi
  printf '%s|%s|%s\n' "$checkout" "$sha" "$branch"
}
