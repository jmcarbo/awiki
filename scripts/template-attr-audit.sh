#!/usr/bin/env bash
# Diff `git check-attr` output between OLD and NEW .gitattributes for each path.
# Emits PLAN|attribute-change|... lines. Exit 1 if any filter=git-crypt added/removed.
set -euo pipefail

OLD=""
NEW=""
PATHS=()

while [[ $# -gt 0 ]]; do
  case "$1" in
    --old) OLD="$2"; shift 2 ;;
    --new) NEW="$2"; shift 2 ;;
    --paths) shift; PATHS=("$@"); break ;;
    *) echo "unknown arg: $1" >&2; exit 2 ;;
  esac
done

[[ -f "$OLD" ]] || { echo "old gitattributes not found" >&2; exit 2; }
[[ -f "$NEW" ]] || { echo "new gitattributes not found" >&2; exit 2; }
[[ ${#PATHS[@]} -gt 0 ]] || exit 0

ENCRYPTION_FLIPPED=0

for P in "${PATHS[@]}"; do
  OLD_ATTRS=$(GIT_ATTR_NOSYSTEM=1 git -c "core.attributesfile=$OLD" check-attr -a "$P" 2>/dev/null | sort | tr '\n' ';')
  NEW_ATTRS=$(GIT_ATTR_NOSYSTEM=1 git -c "core.attributesfile=$NEW" check-attr -a "$P" 2>/dev/null | sort | tr '\n' ';')

  if [[ "$OLD_ATTRS" != "$NEW_ATTRS" ]]; then
    OLD_ENC=$(echo "$OLD_ATTRS" | grep -c "filter: git-crypt" || true)
    NEW_ENC=$(echo "$NEW_ATTRS" | grep -c "filter: git-crypt" || true)
    if [[ "$OLD_ENC" != "$NEW_ENC" ]]; then
      ENCRYPTION_FLIPPED=1
    fi
    # URL-encode the attr strings minimally (replace pipe).
    OE=${OLD_ATTRS//|/%7C}
    NE=${NEW_ATTRS//|/%7C}
    PE=${P//|/%7C}
    echo "PLAN|attribute-change|${PE}|${OE}|${NE}"
  fi
done

[[ $ENCRYPTION_FLIPPED -eq 1 ]] && exit 1
exit 0
