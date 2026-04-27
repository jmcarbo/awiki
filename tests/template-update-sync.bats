#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
}
teardown() { rm -rf "$TMP"; }

@test "sync apply-overwrite: copies file from new tree to user tree" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-overwrite \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh
  [ "$status" -eq 0 ]
  [ -f "$TMP/scripts/new-helper.sh" ]
  diff "$V1/scripts/new-helper.sh" "$TMP/scripts/new-helper.sh"
}
