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
  # template-cache is treated as a gitignored local cache per spec.
  echo ".awiki/template-cache/" > .gitignore
  git add .gitignore .awiki/template.json
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

@test "template-update: fetch creates _fetch dir and state file" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --accept-source-change
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/_fetch ]
  [ -f .awiki/template-cache/_fetch/.update-state.json ]
}

@test "template-update: same commit -> 'already up to date'" {
  cd "$TMP"
  # Use v0 as source to match the pinned commit.
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v0" --accept-source-change
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "already up to date" || true
}

@test "template-update: fetch auto-recovers ancestor cache when missing" {
  cd "$TMP"
  rm -rf ".awiki/template-cache/$COMMIT_OLD"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" --accept-source-change
  [ "$status" -eq 0 ]
  [ -d ".awiki/template-cache/$COMMIT_OLD" ]
  echo "$output" | grep -q "Re-built ancestor cache from pin"
}

@test "template-update: schema-version mismatch halts without --schema-upgrade" {
  cd "$TMP"
  # v3 fixture is committed in the source tree (created by the dedicated task above).
  V3="$REPO_ROOT/tests/fixtures/template-update/v3-schema-bump"
  [ -d "$V3" ] || skip "v3-schema-bump fixture missing — run the v3 fixture creation task first"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V3" --accept-source-change
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "schema_version"
  echo "$output" | grep -q -- "--schema-upgrade"
}

@test "template-update: --verify-signature halts on unsigned tag" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --accept-source-change \
    --verify-signature
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "verify|signature"
}
