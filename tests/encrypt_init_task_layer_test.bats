#!/usr/bin/env bats

# Tests for the phase-19 extension to scripts/encrypt-init.sh: when
# .awiki/config carries AWIKI_TASK_LAYER=on, encrypt-init should add
# content/inbox.md and content/agenda/** to .gitattributes git-crypt
# patterns. Idempotent re-runs do not duplicate.
#
# git-crypt is stubbed via PATH so the test is self-contained.

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/projects" "$WORK/content/agenda" "$WORK/content/private" "$WORK/.awiki" "$WORK/secrets"
  cd "$WORK"
  git init -q
  git config user.email t@t.t
  git config user.name t

  # Seed a phase-5-style .gitattributes (commented patterns the script
  # uncomments on first run) and matching .gitignore entries.
  cat > .gitattributes <<'ATT'
# raw/processed/private/** filter=git-crypt diff=git-crypt
# raw/processed/_originals/** filter=git-crypt diff=git-crypt
# content/private/** filter=git-crypt diff=git-crypt
# secrets/** filter=git-crypt diff=git-crypt
ATT
  cat > .gitignore <<'IGN'
content/private/**
raw/processed/private/**
raw/processed/_originals/**
IGN

  # Stub log-append.sh so the script's call is a no-op.
  cat > scripts/log-append.sh <<'STUB'
#!/usr/bin/env bash
exit 0
STUB
  chmod +x scripts/log-append.sh

  # Stub git-crypt on PATH.
  STUB_DIR="$(mktemp -d)"
  cat > "$STUB_DIR/git-crypt" <<'STUB'
#!/usr/bin/env bash
case "$1" in
  init)
    # simulate `.git/git-crypt` directory creation
    mkdir -p .git/git-crypt
    exit 0 ;;
  add-gpg-user) exit 0 ;;
  status)       echo "encrypted: yes" ;;
  export-key)   shift; touch "$1"; exit 0 ;;
  *)            exit 0 ;;
esac
STUB
  chmod +x "$STUB_DIR/git-crypt"
  export PATH="$STUB_DIR:$PATH"
  STUB_DIR_KEEP="$STUB_DIR"
}

teardown() {
  if [[ -n "${STUB_DIR_KEEP:-}" && -d "$STUB_DIR_KEEP" ]]; then
    rm -rf "$STUB_DIR_KEEP"
  fi
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

@test "encrypt-init covers inbox.md and agenda/** when AWIKI_TASK_LAYER=on" {
  printf '%s\n' "AWIKI_TASK_LAYER=on" > .awiki/config

  run bash scripts/encrypt-init.sh
  [ "$status" -eq 0 ]

  run grep -F 'content/inbox.md filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  run grep -F 'content/agenda/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
}

@test "encrypt-init does NOT cover inbox/agenda when AWIKI_TASK_LAYER is unset" {
  : > .awiki/config

  run bash scripts/encrypt-init.sh
  [ "$status" -eq 0 ]

  run grep -F 'content/inbox.md filter=git-crypt' .gitattributes
  [ "$status" -ne 0 ]
  run grep -F 'content/agenda/** filter=git-crypt' .gitattributes
  [ "$status" -ne 0 ]
}

@test "encrypt-init re-run is idempotent (no duplicate patterns)" {
  printf '%s\n' "AWIKI_TASK_LAYER=on" > .awiki/config

  bash scripts/encrypt-init.sh
  bash scripts/encrypt-init.sh

  run grep -c -F 'content/inbox.md filter=git-crypt' .gitattributes
  [[ "$output" == "1" ]]
  run grep -c -F 'content/agenda/** filter=git-crypt' .gitattributes
  [[ "$output" == "1" ]]
}

@test "encrypt-init re-run after task-layer enable adds patterns to existing repo" {
  # Simulate: encrypt-init was run once before task-layer was enabled.
  : > .awiki/config
  bash scripts/encrypt-init.sh

  # User now enables the task layer, then re-runs encrypt-init.
  printf '%s\n' "AWIKI_TASK_LAYER=on" > .awiki/config
  run bash scripts/encrypt-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"already initialized"* ]] || [[ "$output" == *"task-layer"* ]]

  run grep -F 'content/inbox.md filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  run grep -F 'content/agenda/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
}
