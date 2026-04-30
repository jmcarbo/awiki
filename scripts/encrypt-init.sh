#!/usr/bin/env bash
set -euo pipefail

MODE="git-crypt"
if [[ "${1:-}" == "--age" ]]; then MODE="age"; fi

# Phase 19 helper: when .awiki/config carries AWIKI_TASK_LAYER=on, ensure
# content/inbox.md and content/agenda/** are covered by git-crypt patterns
# in .gitattributes. Idempotent — re-runs detect existing patterns and
# skip. The helper is called both on fresh init AND on re-runs of an
# already-initialized repo (the latter so users who ran encrypt-init
# before phase 19 can re-run it after enabling the task layer to extend
# coverage).
add_task_layer_patterns_if_enabled() {
  if [[ ! -f .awiki/config ]] || ! grep -qE '^AWIKI_TASK_LAYER=on$' .awiki/config; then
    return 0
  fi
  if [[ ! -f .gitattributes ]]; then
    : > .gitattributes
  fi
  local pat
  for pat in "content/inbox.md" "content/agenda/**"; do
    if ! grep -qE "^$(printf '%s' "$pat" | sed 's/[].[\*\^\$\/]/\\&/g') filter=git-crypt diff=git-crypt\$" .gitattributes; then
      printf '%s filter=git-crypt diff=git-crypt\n' "$pat" >> .gitattributes
      echo "encrypt-init: added pattern $pat (task-layer)"
    fi
  done
}

case "$MODE" in
  git-crypt)
    if ! command -v git-crypt >/dev/null 2>&1; then
      echo "git-crypt not installed. Install: brew install git-crypt (mac) / apt install git-crypt (linux)" >&2
      exit 1
    fi
    if [[ -d .git/git-crypt ]]; then
      echo "git-crypt already initialized" >&2
      # Even on re-run, extend coverage if the task layer is now on.
      add_task_layer_patterns_if_enabled
      exit 0
    fi
    git-crypt init

    # Activate patterns in .gitattributes (portable across BSD/GNU sed via awk)
    awk '
      /^# raw\/processed\/private\// { sub(/^# /, ""); print; next }
      /^# raw\/processed\/_originals\// { sub(/^# /, ""); print; next }
      /^# content\/private\// { sub(/^# /, ""); print; next }
      /^# secrets\// { sub(/^# /, ""); print; next }
      { print }
    ' .gitattributes > .gitattributes.tmp && mv .gitattributes.tmp .gitattributes

    # Atomically remove the gitignore lines for private paths so encrypted commits work
    awk '
      /^content\/private\/\*\*$/ { next }
      /^raw\/processed\/private\/\*\*$/ { next }
      /^raw\/processed\/_originals\/\*\*$/ { next }
      { print }
    ' .gitignore > .gitignore.tmp && mv .gitignore.tmp .gitignore

    mkdir -p secrets
    git-crypt export-key secrets/.git-crypt-key
    chmod 600 secrets/.git-crypt-key

    add_task_layer_patterns_if_enabled

    awiki log encrypt "git-crypt initialized; key at secrets/.git-crypt-key" 2>/dev/null || true
    echo "ENCRYPT-OK|mode=git-crypt|key=secrets/.git-crypt-key"
    echo "STORE the key securely (password manager). Anyone with this key can read encrypted paths."
    ;;
  age)
    if ! command -v age >/dev/null 2>&1; then
      echo "age not installed. Install: brew install age / apt install age" >&2
      exit 1
    fi
    mkdir -p secrets
    if [[ -f secrets/age.key ]]; then
      echo "age key already exists at secrets/age.key" >&2
      exit 0
    fi
    age-keygen -o secrets/age.key
    grep '^# public key:' secrets/age.key | sed 's/^# public key: //' > secrets/age.pub
    chmod 600 secrets/age.key

    awiki log encrypt "age keypair generated" 2>/dev/null || true
    echo "ENCRYPT-OK|mode=age|key=secrets/age.key|pub=secrets/age.pub"
    echo "Encrypt sensitive files: age -R secrets/age.pub -o file.age file"
    echo "Decrypt: age -d -i secrets/age.key file.age"
    ;;
esac
