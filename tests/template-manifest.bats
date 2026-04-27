#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
}

@test "manifest load: emits schema_version" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE 'schema_version=1'
}

@test "manifest load: emits template_version" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE 'template_version=0\.1\.0'
}

@test "manifest load: missing file -> nonzero exit" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load /nonexistent.toml
  [ "$status" -ne 0 ]
}
