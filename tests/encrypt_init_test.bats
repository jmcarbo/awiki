#!/usr/bin/env bats

# Smoke tests for scripts/encrypt-init.sh.
# We avoid mutating the real repo; the script is run in throwaway temp git repos
# with a stub `scripts/log-append.sh` so it stays a self-contained smoke test.

setup() {
  REPO_ROOT="$(pwd)"
  WORK="$(mktemp -d)"
  cp "$REPO_ROOT/scripts/encrypt-init.sh" "$WORK/encrypt-init.sh"
  chmod +x "$WORK/encrypt-init.sh"
  mkdir -p "$WORK/scripts"
  # log-append.sh stub so the script can call it from cwd=$WORK
  cat > "$WORK/scripts/log-append.sh" <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
  chmod +x "$WORK/scripts/log-append.sh"
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

@test "encrypt-init exits 1 with helpful message when git-crypt missing" {
  if command -v git-crypt >/dev/null 2>&1; then
    skip "git-crypt is installed; cannot test missing-tool path"
  fi
  cd "$WORK"
  run bash encrypt-init.sh
  [ "$status" -eq 1 ]
  [[ "$output" == *"git-crypt not installed"* ]]
}

@test "encrypt-init --age exits 1 with helpful message when age missing" {
  if command -v age >/dev/null 2>&1; then
    skip "age is installed; cannot test missing-tool path"
  fi
  cd "$WORK"
  run bash encrypt-init.sh --age
  [ "$status" -eq 1 ]
  [[ "$output" == *"age not installed"* ]]
}

@test "encrypt-init --age generates keypair when age available" {
  if ! command -v age >/dev/null 2>&1; then
    skip "age not installed"
  fi
  cd "$WORK"
  run bash encrypt-init.sh --age
  [ "$status" -eq 0 ]
  [[ "$output" == *"ENCRYPT-OK|mode=age"* ]]
  [ -f "$WORK/secrets/age.key" ]
  [ -f "$WORK/secrets/age.pub" ]
  # Permissions on private key must be 600
  perms="$(stat -f %A "$WORK/secrets/age.key" 2>/dev/null || stat -c %a "$WORK/secrets/age.key")"
  [ "$perms" = "600" ]
}

@test "encrypt-init --age is idempotent when key already exists" {
  if ! command -v age >/dev/null 2>&1; then
    skip "age not installed"
  fi
  cd "$WORK"
  mkdir -p secrets
  # Pre-existing key sentinel
  echo "preexisting" > secrets/age.key
  run bash encrypt-init.sh --age
  [ "$status" -eq 0 ]
  [[ "$output" == *"age key already exists"* ]]
  # File was not overwritten
  run cat secrets/age.key
  [ "$output" = "preexisting" ]
}

@test "encrypt-init git-crypt initializes repo when git-crypt available" {
  if ! command -v git-crypt >/dev/null 2>&1; then
    skip "git-crypt not installed"
  fi
  cd "$WORK"
  git init -q
  git config user.email t@t.t
  git config user.name t
  cat > .gitignore <<'IGN'
content/private/**
raw/processed/private/**
raw/processed/_originals/**
IGN
  cat > .gitattributes <<'ATT'
# raw/processed/private/** filter=git-crypt diff=git-crypt
# raw/processed/_originals/** filter=git-crypt diff=git-crypt
# content/private/** filter=git-crypt diff=git-crypt
# secrets/** filter=git-crypt diff=git-crypt
ATT
  run bash encrypt-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"ENCRYPT-OK|mode=git-crypt"* ]]
  [ -f secrets/.git-crypt-key ]
  # Patterns activated (uncommented) in .gitattributes
  run grep -F 'content/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  # Private paths removed from .gitignore
  run grep -F 'content/private/**' .gitignore
  [ "$status" -ne 0 ]
}
