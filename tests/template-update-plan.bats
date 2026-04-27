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
  echo ".awiki/template-cache/" > .gitignore
  git add .gitignore .awiki/template.json
  git -c user.email=a@b -c user.name=t commit -q -m "post-init"
}
teardown() { rm -rf "$TMP"; }

@test "phase 2: dry-run prints PLAN lines and exits 0 without applying" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" --accept-source-change
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|header\|'
  echo "$output" | grep -qE '^PLAN\|footer\|'
  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  [ "$CUR_BRANCH" = "main" ]
}

@test "phase 2: --print-migrations adds migration-body framing" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --accept-source-change --print-migrations
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|migration-body\|0001-add-tagline\|begin'
  echo "$output" | grep -qE '^PLAN\|migration-body\|0001-add-tagline\|end'
}

@test "phase 2: scratch-merge dir is removed after plan" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" --accept-source-change
  [ ! -d ".awiki/template-cache/_fetch/_scratch-merge" ]
}
