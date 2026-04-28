#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  echo ".awiki/template-cache/" > .gitignore
  git add .gitignore .awiki/template.json
  git -c user.email=a@b -c user.name=t commit -q -m "post-init"
}
teardown() { rm -rf "$TMP"; }

@test "--gc: keeps current + immediate previous, removes older orphans" {
  cd "$TMP"
  # 3 orphan dirs at very old mtimes; current cache stays newest.
  mkdir -p .awiki/template-cache/orphan1 .awiki/template-cache/orphan2 .awiki/template-cache/orphan3
  touch -t 200001011200 .awiki/template-cache/orphan1
  touch -t 200001021200 .awiki/template-cache/orphan2
  touch -t 200001031200 .awiki/template-cache/orphan3
  run bash "$REPO_ROOT/scripts/template-update.sh" --gc
  [ "$status" -eq 0 ]
  CUR=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json commit)
  # Current cache preserved.
  [ -d ".awiki/template-cache/$CUR" ]
  # The two oldest orphans removed; only the newest (orphan3) preserved as "previous".
  [ ! -d .awiki/template-cache/orphan1 ]
  [ ! -d .awiki/template-cache/orphan2 ]
  [ -d .awiki/template-cache/orphan3 ]
  echo "$output" | grep -q "cache GC complete"
}

@test "--status: prints pin + version + repo" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --status
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "version:"
  echo "$output" | grep -q "commit:"
  echo "$output" | grep -q "repo:"
}

@test "--status: notes pending prompts" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo x > .awiki/pending-prompts/0001-x.md
  run bash "$REPO_ROOT/scripts/template-update.sh" --status
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "pending prompts"
}

@test "--status: notes in-progress phase" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/_fetch
  echo '{"phase":"commit-b","status":"started","commit_old":"x","commit_new":"y","branch":"awiki-template-update/abc","started_at":"2026-04-27T00:00:00Z","last_completed_commit":null,"applied_migrations_pending":[],"bootstrap_steps_pending":[],"deletions_user_decisions":{},"deleted_pending":[]}' \
    > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --status
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "in-progress update"
  echo "$output" | grep -q "phase=commit-b"
}
