#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}

teardown() { rm -rf "$TMP"; }

@test "merge: clean 3-way (no user changes) -> exit 0, no markers" {
  printf 'a\nb\nc\n' > "$TMP/base"
  printf 'a\nb\nc\n' > "$TMP/cur"
  printf 'a\nB\nc\n' > "$TMP/new"
  run bash "$REPO_ROOT/scripts/template-merge.sh" "$TMP/cur" "$TMP/base" "$TMP/new"
  [ "$status" -eq 0 ]
  ! grep -q '<<<<<<<' "$TMP/cur"
  grep -q '^B$' "$TMP/cur"
}

@test "merge: conflict (both edited) -> nonzero, markers present" {
  printf 'a\nb\nc\n' > "$TMP/base"
  printf 'a\nUSER\nc\n' > "$TMP/cur"
  printf 'a\nNEW\nc\n' > "$TMP/new"
  run bash "$REPO_ROOT/scripts/template-merge.sh" "$TMP/cur" "$TMP/base" "$TMP/new"
  [ "$status" -ne 0 ]
  grep -q '<<<<<<<' "$TMP/cur"
}

@test "merge: missing base file -> error" {
  printf 'x\n' > "$TMP/cur"
  printf 'y\n' > "$TMP/new"
  run bash "$REPO_ROOT/scripts/template-merge.sh" "$TMP/cur" "$TMP/missing" "$TMP/new"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE 'base.*not found|missing'
}
