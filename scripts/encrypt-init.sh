#!/usr/bin/env bash
set -euo pipefail

MODE="git-crypt"
if [[ "${1:-}" == "--age" ]]; then MODE="age"; fi

case "$MODE" in
  git-crypt)
    if ! command -v git-crypt >/dev/null 2>&1; then
      echo "git-crypt not installed. Install: brew install git-crypt (mac) / apt install git-crypt (linux)" >&2
      exit 1
    fi
    if [[ -d .git/git-crypt ]]; then
      echo "git-crypt already initialized" >&2
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

    bash scripts/log-append.sh encrypt "git-crypt initialized; key at secrets/.git-crypt-key"
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

    bash scripts/log-append.sh encrypt "age keypair generated"
    echo "ENCRYPT-OK|mode=age|key=secrets/age.key|pub=secrets/age.pub"
    echo "Encrypt sensitive files: age -R secrets/age.pub -o file.age file"
    echo "Decrypt: age -d -i secrets/age.key file.age"
    ;;
esac
