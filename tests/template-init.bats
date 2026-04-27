#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
  TMP=$(mktemp -d)
  # Mock a "bootstrapped" repo: copy fixture into TMP as a git repo at a known SHA.
  cp -R "$FIXTURE/." "$TMP/"
  cd "$TMP"
  git init -q
  git add -A
  git -c user.email=test@example.com -c user.name=Test commit -q -m "init"
  COMMIT=$(git rev-parse HEAD)
}

teardown() { rm -rf "$TMP"; }

@test "template-init: writes .awiki/template.json with current commit" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo https://github.com/jmcarbo/awiki \
    --ref main \
    --version 1.0.0 \
    --commit "$COMMIT"
  [ "$status" -eq 0 ]
  [ -f .awiki/template.json ]
  PJ_COMMIT=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json commit)
  [ "$PJ_COMMIT" = "$COMMIT" ]
}

@test "template-init: snapshots template tree to template-cache/<commit>/" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  [ -d ".awiki/template-cache/$COMMIT" ]
  [ -f ".awiki/template-cache/$COMMIT/template.manifest.toml" ]
}

@test "template-init: records bootstrap_steps_done from BOOTSTRAP.md with content_hash" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  STEPS=$(bash "$REPO_ROOT/scripts/template-provenance.sh" list-steps .awiki/template.json)
  echo "$STEPS" | grep -qE '^dep-check applied sha256:'
  echo "$STEPS" | grep -qE '^domain applied sha256:'
  echo "$STEPS" | grep -qE '^stage-commit applied sha256:'
}

@test "template-init: idempotent (re-run rebuilds cache, doesn't error)" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  run bash "$REPO_ROOT/scripts/template-init.sh" --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  [ "$status" -eq 0 ]
}

@test "template-init: missing required arg fails" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-init.sh" --repo url
  [ "$status" -ne 0 ]
}
