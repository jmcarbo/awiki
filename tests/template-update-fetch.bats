#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT_OLD=$(git rev-parse HEAD)
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m "post-init"
}

teardown() { rm -rf "$TMP"; }

@test "template-update: --help prints usage" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --help
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "usage:"
  echo "$output" | grep -q -- "--apply"
  echo "$output" | grep -q -- "--continue"
}

@test "template-update: dirty tree halts" {
  cd "$TMP"
  echo dirty > leftover.txt
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "dirty tree"
}

@test "template-update: on update branch halts" {
  cd "$TMP"
  git checkout -q -b awiki-template-update/foo
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "update branch|--continue"
}

@test "template-update: pending prompts halts" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo x > .awiki/pending-prompts/0001-leftover.md
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "pending"
}

@test "template-update: source mismatch halts without --accept-source-change" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --source /elsewhere
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "Source change detected"
}
