#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  # Build a small upstream repo so re-pin can validate commits.
  UPSTREAM=$(mktemp -d)
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." "$UPSTREAM/"
  git -C "$UPSTREAM" init -q -b main
  git -C "$UPSTREAM" add -A
  git -C "$UPSTREAM" -c user.email=a@b -c user.name=t commit -q -m v0
  UPSTREAM_HEAD=$(git -C "$UPSTREAM" rev-parse HEAD)
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$UPSTREAM" \
    --ref main --version 0.1.0 --commit "$UPSTREAM_HEAD" >/dev/null
  echo ".awiki/template-cache/" > .gitignore
  git add .gitignore .awiki/template.json
  git -c user.email=a@b -c user.name=t commit -q -m "post-init"
  export UPSTREAM UPSTREAM_HEAD
}
teardown() { rm -rf "$TMP" "$UPSTREAM"; }

@test "--re-pin: refuses if pending-prompts present" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo body > .awiki/pending-prompts/0001-x.md
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin "$UPSTREAM_HEAD"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "pending"
}

@test "--re-pin: refuses if state file present" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/_fetch
  echo '{}' > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin "$UPSTREAM_HEAD"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "in progress"
}

@test "--re-pin: writes new commit to template.json" {
  cd "$TMP"
  # Add a second commit upstream we can re-pin to.
  echo extra > "$UPSTREAM/EXTRA.md"
  git -C "$UPSTREAM" add EXTRA.md
  git -C "$UPSTREAM" -c user.email=a@b -c user.name=t commit -q -m extra
  TARGET=$(git -C "$UPSTREAM" rev-parse HEAD)
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin "$TARGET"
  [ "$status" -eq 0 ] || { echo "STATUS=$status"; echo "$output"; false; }
  PIN_COMMIT=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json commit)
  [ "$PIN_COMMIT" = "$TARGET" ]
  # Target cache rebuilt.
  [ -d ".awiki/template-cache/$TARGET" ]
}

@test "--re-pin: invalid commit rejected when original_repo reachable" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --re-pin "0000000000000000000000000000000000000000"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "not resolvable|cannot reach"
}

@test "repo=\"none\" disables update with informational exit" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-provenance.sh" set .awiki/template.json repo none >/dev/null
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "updates disabled"
}
