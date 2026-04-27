#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  echo ".awiki/template-cache/" > .gitignore
  git add .gitignore .awiki/template.json
  git -c user.email=a@b -c user.name=t commit -q -m "post-init"
}
teardown() { rm -rf "$TMP"; }

@test "--abort: deletes branch + _fetch dir; main untouched" {
  cd "$TMP"
  # Simulate mid-update: create _fetch + state + branch.
  mkdir -p .awiki/template-cache/_fetch
  echo '{"phase":"commit-a","status":"started","branch":"awiki-template-update/zzz","commit_old":"x","commit_new":"y","started_at":"2026-04-27T00:00:00Z","last_completed_commit":null,"applied_migrations_pending":[],"bootstrap_steps_pending":[],"deletions_user_decisions":{},"deleted_pending":[]}' \
    > .awiki/template-cache/_fetch/.update-state.json
  git checkout -q -b awiki-template-update/zzz
  run bash "$REPO_ROOT/scripts/template-update.sh" --abort
  [ "$status" -eq 0 ]
  ! git rev-parse --verify --quiet "refs/heads/awiki-template-update/zzz" >/dev/null
  [ ! -d .awiki/template-cache/_fetch ]
  [ "$(git rev-parse --abbrev-ref HEAD)" = "main" ]
}

@test "--abort: no in-progress update -> no-op exit 0" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --abort
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "no in-progress update"
}
