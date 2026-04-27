#!/usr/bin/env bash
set -euo pipefail

# Refuse to publish if encryption is configured but tree is locked.
if [[ -d .git/git-crypt ]]; then
  if ! git-crypt status -e 2>/dev/null | head -1 | grep -q 'encrypted:'; then
    echo "ERROR: git-crypt initialized but tree appears locked. Set GIT_CRYPT_KEY secret and unlock before deploy." >&2
    exit 1
  fi
fi

bash scripts/build.sh --full
