#!/usr/bin/env bats

# Phase 18a poisoning protection for scripts/log-append.sh.
# Caller-supplied substrings (action and message) MUST be neutralized before
# being written to content/log.md:
#   '|'  -> '_'  (would break the field separator)
#   '\n' -> ' '  (would break the line terminator)
#   '\r' -> ' '  (CR alone or before LF is a line terminator under some readers)
# Internal-only fields (timestamp) are not touched.

setup() {
  TEST_LOG="$(mktemp -d)/log.md"
  export AWIKI_LOG_FILE="$TEST_LOG"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$TEST_LOG"
}

teardown() {
  rm -rf "$(dirname "$TEST_LOG")"
}

@test "log-append neutralizes pipe in caller message" {
  run bash scripts/log-append.sh triage "act | _loose | phantom"
  [ "$status" -eq 0 ]
  # Exactly one log line must have been appended.
  appended=$(grep -c '^## \[' "$TEST_LOG")
  [ "$appended" -eq 1 ]
  # Header format: "## [datetime] action | message". With a clean action and
  # a sanitized message, exactly one '|' must remain on the line (the field
  # separator). The two pipes inside the message must be replaced with '_'.
  line=$(grep '^## \[' "$TEST_LOG")
  pipe_count=$(awk -F'|' '{print NF-1}' <<<"$line")
  [ "$pipe_count" -eq 1 ]
  echo "$line" | grep -q 'act _ _loose _ phantom'
}

@test "log-append neutralizes embedded LF in caller message" {
  run bash scripts/log-append.sh triage $'first\nsecond'
  [ "$status" -eq 0 ]
  # The log file must still contain exactly one appended header line.
  appended=$(grep -c '^## \[' "$TEST_LOG")
  [ "$appended" -eq 1 ]
  grep -q 'first second' "$TEST_LOG"
  # And no orphan "second" line on its own (which would indicate a real LF
  # made it through).
  ! grep -qE '^second$' "$TEST_LOG"
}

@test "log-append neutralizes embedded CR in caller message" {
  run bash scripts/log-append.sh triage $'first\rsecond'
  [ "$status" -eq 0 ]
  appended=$(grep -c '^## \[' "$TEST_LOG")
  [ "$appended" -eq 1 ]
  grep -q 'first second' "$TEST_LOG"
  # No raw CR byte should survive in the log.
  ! grep -qU $'\r' "$TEST_LOG"
}

@test "log-append leaves clean message untouched" {
  run bash scripts/log-append.sh triage "do-now / dentist / phone"
  [ "$status" -eq 0 ]
  grep -q 'do-now / dentist / phone' "$TEST_LOG"
}

@test "log-append also sanitizes the action argument" {
  run bash scripts/log-append.sh $'evil|action' "msg"
  [ "$status" -eq 0 ]
  appended=$(grep -c '^## \[' "$TEST_LOG")
  [ "$appended" -eq 1 ]
  line=$(grep '^## \[' "$TEST_LOG")
  # Pipe in the action must be replaced with '_'; only the field separator
  # pipe between action and message remains.
  pipe_count=$(awk -F'|' '{print NF-1}' <<<"$line")
  [ "$pipe_count" -eq 1 ]
  echo "$line" | grep -q 'evil_action'
}
