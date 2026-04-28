#!/usr/bin/env bash
# template-retrofit.sh — seed .awiki/template.json on a pre-v1 bootstrapped repo.
# Detects heuristic completion of bootstrap steps; runs template-init; optionally
# walks the user through marking missing steps as skipped.
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(pwd)"

REPO=""
REF="main"
VERSION=""
COMMIT=""
NON_INTERACTIVE=0
HEURISTICS_ONLY=0

usage() {
  cat <<EOF
usage: template-retrofit.sh --repo <url> --ref <ref> --version <v> --commit <sha>
                            [--non-interactive] [--heuristics-only]

Detects a bootstrapped repo without .awiki/template.json and seeds it.

Heuristics surfaced (best-effort):
  - .awiki/qmd-status                     -> install-qmd
  - .gitattributes references git-crypt   -> privacy=git-crypt
  - secrets/age-key.txt                   -> privacy=age
  - .mcp.json or .cursor/mcp.json contains awiki-server -> wire-awiki-mcp
EOF
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --commit) COMMIT="$2"; shift 2 ;;
    --non-interactive) NON_INTERACTIVE=1; shift ;;
    --heuristics-only) HEURISTICS_ONLY=1; shift ;;
    -h|--help) usage ;;
    *) echo "unknown arg: $1" >&2; usage ;;
  esac
done

PJ="$REPO_ROOT/.awiki/template.json"
if [[ -f "$PJ" ]]; then
  echo "info: $PJ already exists; nothing to retrofit"
  exit 0
fi

# Heuristics: which steps look already-done?
echo "Detecting already-completed bootstrap steps:"
HEURISTIC_HITS=()
if [[ -f "$REPO_ROOT/.awiki/qmd-status" ]]; then
  echo "  - install-qmd: yes (.awiki/qmd-status exists)"
  HEURISTIC_HITS+=("install-qmd")
fi
if [[ -f "$REPO_ROOT/.gitattributes" ]] && grep -q "filter=git-crypt" "$REPO_ROOT/.gitattributes" 2>/dev/null; then
  echo "  - privacy=git-crypt: yes (.gitattributes references git-crypt)"
  HEURISTIC_HITS+=("privacy")
fi
if [[ -f "$REPO_ROOT/secrets/age-key.txt" ]]; then
  echo "  - privacy=age: yes (secrets/age-key.txt exists)"
  HEURISTIC_HITS+=("privacy")
fi
if grep -q "awiki-server" "$REPO_ROOT/.mcp.json" 2>/dev/null \
   || grep -q "awiki-server" "$REPO_ROOT/.cursor/mcp.json" 2>/dev/null; then
  echo "  - wire-awiki-mcp: yes (mcp config references awiki-server)"
  HEURISTIC_HITS+=("wire-awiki-mcp")
fi
if [[ ${#HEURISTIC_HITS[@]} -eq 0 ]]; then
  echo "  (none detected)"
fi

if [[ $HEURISTICS_ONLY -eq 1 ]]; then
  exit 0
fi

[[ -z "$REPO" || -z "$VERSION" || -z "$COMMIT" ]] && usage

# Run template-init (records every BOOTSTRAP.md step ID as 'applied' with current content_hash).
bash "$SCRIPT_DIR/template-init.sh" --repo "$REPO" --ref "$REF" --version "$VERSION" --commit "$COMMIT"

# In interactive retrofit, walk the user through each step to confirm/correct status.
# (Heuristic hits are pre-filled "yes"; for unknown steps we prompt.)
if [[ $NON_INTERACTIVE -eq 0 ]]; then
  echo "Confirm each step status (y = applied / n = skipped):"
  STEPS=$(python3 "$SCRIPT_DIR/_template_helpers/bootstrap_hash.py" list "$REPO_ROOT/BOOTSTRAP.md")
  for SID in $STEPS; do
    HIT=""
    for h in "${HEURISTIC_HITS[@]:-}"; do
      [[ "$h" == "$SID" ]] && HIT=" (heuristic suggests yes)"
    done
    read -r -p "  $SID$HIT [y/n] " ans </dev/tty || ans="y"
    if [[ "$ans" =~ ^[Nn] ]]; then
      python3 - "$PJ" "$SID" <<'PY'
import json, sys
p, sid = sys.argv[1], sys.argv[2]
d = json.load(open(p))
for s in d.get('bootstrap_steps_done', []):
    if s.get('id') == sid:
        s['status'] = 'skipped'
        s['reason'] = 'retrofit: user said no'
        break
with open(p, 'w') as f:
    json.dump(d, f, indent=2)
    f.write('\n')
PY
    fi
  done
fi

echo "info: retrofit complete"
