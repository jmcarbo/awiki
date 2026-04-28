#!/usr/bin/env bash
# Source-only helper. Parse + validate .awiki/git-sources.yml entries.
# Functions:
#   awiki_git_config_validate_name <name>         → exit 0 if ^[a-z][a-z0-9-]*$
#   awiki_git_config_validate_paths <json-array>  → exit 0 if no '..' / leading '/'
#   awiki_git_config_get <yaml-path> <repo-name>  → JSON for the named repo
#                                                   (auto-flips private=true on SSH url)

awiki_git_config_validate_name() {
  local name="$1"
  if [[ "$name" =~ ^[a-z][a-z0-9-]*$ ]]; then
    return 0
  fi
  echo "ERROR: invalid repo name (need ^[a-z][a-z0-9-]*$): $name" >&2
  return 1
}

awiki_git_config_validate_paths() {
  local arr="$1"
  python3 - <<'PY' "$arr"
import json, sys
try:
    paths = json.loads(sys.argv[1])
except Exception as e:
    print(f"ERROR: invalid JSON: {e}", file=sys.stderr); sys.exit(1)
if not isinstance(paths, list):
    print("ERROR: expected JSON array", file=sys.stderr); sys.exit(1)
for p in paths:
    if not isinstance(p, str):
        print(f"ERROR: path entry must be string: {p!r}", file=sys.stderr); sys.exit(1)
    if ".." in p.split("/"):
        print(f"ERROR: '..' not allowed in path: {p}", file=sys.stderr); sys.exit(1)
    if p.startswith("/"):
        print(f"ERROR: leading '/' not allowed in path: {p}", file=sys.stderr); sys.exit(1)
sys.exit(0)
PY
}

awiki_git_config_get() {
  local yaml_path="$1" repo_name="$2"
  if [[ ! -f "$yaml_path" ]]; then
    echo "ERROR: config not found: $yaml_path" >&2
    return 12
  fi
  if ! python3 -c "import yaml" >/dev/null 2>&1; then
    echo "ERROR: pyyaml required to parse $yaml_path; install: pip3 install pyyaml" >&2
    return 12
  fi
  python3 - <<'PY' "$yaml_path" "$repo_name"
import json, re, sys, yaml
yaml_path, repo_name = sys.argv[1], sys.argv[2]
try:
    with open(yaml_path) as f:
        doc = yaml.safe_load(f)
except yaml.YAMLError as e:
    print(f"ERROR: yaml parse: {e}", file=sys.stderr); sys.exit(12)
if not isinstance(doc, dict) or doc.get("schema") != 1:
    print("ERROR: missing or wrong schema (need schema: 1)", file=sys.stderr); sys.exit(12)
repos = doc.get("repos") or []
match = next((r for r in repos if r.get("name") == repo_name), None)
if match is None:
    print(f"ERROR: no repo named: {repo_name}", file=sys.stderr); sys.exit(12)
name = match.get("name", "")
if not re.match(r"^[a-z][a-z0-9-]*$", name):
    print(f"ERROR: invalid name: {name}", file=sys.stderr); sys.exit(12)
url = match.get("url", "")
private = bool(match.get("private", False))
# SSH url auto-flips private — only when explicit private flag is not False-set.
# Spec §8: explicit private:false on SSH url → honor public + warning.
if url.startswith("git@") and "private" not in match:
    private = True
out = {
    "name": name,
    "url": url,
    "paths": match.get("paths", ["README.md", "docs/", "rfcs/", "adr/"]),
    "exclude": match.get("exclude", []),
    "private": private,
    "private_explicit": "private" in match,
    "protect_edits": bool(match.get("protect_edits", False)),
    "summarize": bool(match.get("summarize", False)),
    "default_branch": match.get("default_branch", ""),
}
print(json.dumps(out, separators=(',', ':')))
sys.exit(0)
PY
}
