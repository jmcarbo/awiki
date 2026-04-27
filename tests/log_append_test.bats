#!/usr/bin/env bats

setup() {
  TEST_LOG="$(mktemp -d)/log.md"
  export AWIKI_LOG_FILE="$TEST_LOG"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$TEST_LOG"
}

teardown() {
  rm -rf "$(dirname "$TEST_LOG")"
}

@test "log-append writes ## [datetime] action | message" {
  bash scripts/log-append.sh ingest "Sample article"
  run grep -E '^## \[[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}\] ingest \| Sample article$' "$TEST_LOG"
  [ "$status" -eq 0 ]
}

@test "log-append handles multi-word message" {
  bash scripts/log-append.sh manual reviewed the catalog
  run grep -E '^## \[.*\] manual \| reviewed the catalog$' "$TEST_LOG"
  [ "$status" -eq 0 ]
}

@test "log-append fails on missing action arg" {
  run bash scripts/log-append.sh
  [ "$status" -ne 0 ]
}
