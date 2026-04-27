#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  PJ="$TMP/template.json"
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" \
    https://github.com/jmcarbo/awiki main 1.0.0 abc123 >/dev/null
}

teardown() { rm -rf "$TMP"; }

@test "source-check: matching source -> exit 0" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance "$PJ" --source https://github.com/jmcarbo/awiki
  [ "$status" -eq 0 ]
}

@test "source-check: different source -> exit 1 with halt message" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance "$PJ" --source https://github.com/evil/fork
  [ "$status" -eq 1 ]
  echo "$output" | grep -q "Source change detected"
  echo "$output" | grep -q "github.com/jmcarbo/awiki"
  echo "$output" | grep -q "github.com/evil/fork"
}

@test "source-check: --accept-source-change overrides" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance "$PJ" --source https://github.com/evil/fork --accept-source-change
  [ "$status" -eq 0 ]
}

@test "source-check: missing provenance -> exit 2" {
  run bash "$REPO_ROOT/scripts/template-source-check.sh" \
    --provenance /nonexistent --source url
  [ "$status" -eq 2 ]
}
