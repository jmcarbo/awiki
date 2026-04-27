#!/usr/bin/env bash
# Wraps `git merge-file --diff3` for three_way / attributes_merge strategies.
# Mutates <cur> in place. Returns 0 on clean merge, nonzero on conflicts.
set -euo pipefail

usage() {
  echo "usage: template-merge.sh <cur> <base> <new>" >&2
  exit 2
}

[[ $# -eq 3 ]] || usage
CUR="$1"; BASE="$2"; NEW="$3"

[[ -f "$BASE" ]] || { echo "base file not found: $BASE" >&2; exit 3; }
[[ -f "$CUR" ]]  || { echo "current file not found: $CUR" >&2; exit 3; }
[[ -f "$NEW" ]]  || { echo "new file not found: $NEW" >&2; exit 3; }

# git merge-file rewrites <cur> in place. Exit code = number of conflicts.
set +e
git merge-file --diff3 -L current -L base -L new "$CUR" "$BASE" "$NEW"
RC=$?
set -e
if [[ $RC -eq 0 ]]; then
  exit 0
fi
exit "$RC"
