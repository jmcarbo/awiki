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

@test "--continue: no state file -> halt with helpful message" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "No update in progress"
}

@test "--continue: manual commits on update branch -> halt without --accept-manual-commits" {
  cd "$TMP"
  git checkout -q -b awiki-template-update/test
  mkdir -p .awiki/template-cache/_fetch
  echo '{
    "phase":"commit-a","status":"committed",
    "commit_old":"aaa","commit_new":"bbb",
    "branch":"awiki-template-update/test",
    "started_at":"2026-04-27T00:00:00Z",
    "last_completed_commit":"will-not-match",
    "applied_migrations_pending":[],"bootstrap_steps_pending":[],
    "deletions_user_decisions":{},"deleted_pending":[]
  }' > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "Manual commits"
}

@test "--continue: resumes mid-cycle past committed phases" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  TMP_BAD=$(mktemp -d)
  cp -R "$V1/." "$TMP_BAD/"
  cat > "$TMP_BAD/migrations/0099-fail.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0099-fail
# requires: agent=false
# touches: WIKI.md
# idempotent: yes
[[ "${1:-}" == "--dry-run" ]] && exit 0
exit 7
EOF
  chmod +x "$TMP_BAD/migrations/0099-fail.sh"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$TMP_BAD" --accept-source-change --apply --non-interactive
  [ "$status" -ne 0 ]
  # Commit A should have committed; Commit B halted at 0099.
  git log --format=%s -3 | grep -q "sync to"
  # Now skip the failing migration and continue.
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue --skip-migration 0099-fail --non-interactive
  [ "$status" -eq 0 ] || { echo "STATUS=$status"; echo "$output"; false; }
  rm -rf "$TMP_BAD"
}

@test "--continue --accept-manual-commits: bypasses HEAD check" {
  cd "$TMP"
  git checkout -q -b awiki-template-update/test
  mkdir -p .awiki/template-cache/_fetch
  CUR_HEAD=$(git rev-parse HEAD)
  echo "{
    \"phase\":\"commit-d\",\"status\":\"committed\",
    \"commit_old\":\"$CUR_HEAD\",\"commit_new\":\"$CUR_HEAD\",
    \"branch\":\"awiki-template-update/test\",
    \"started_at\":\"2026-04-27T00:00:00Z\",
    \"last_completed_commit\":\"$CUR_HEAD\",
    \"applied_migrations_pending\":[],\"bootstrap_steps_pending\":[],
    \"deletions_user_decisions\":{},\"deleted_pending\":[]
  }" > .awiki/template-cache/_fetch/.update-state.json
  # Make a manual commit.
  echo extra > extra.txt
  git add extra.txt && git -c user.email=a@b -c user.name=t commit -q -m manual
  run bash "$REPO_ROOT/scripts/template-update.sh" --continue --accept-manual-commits --non-interactive
  # Just assert the HEAD check did not block; downstream flow may exit 0 or non-zero
  # depending on resume past commit-d (no more work). We accept either as long as the
  # halt message is NOT emitted.
  ! echo "$output" | grep -q "Manual commits detected"
}
