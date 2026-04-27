#!/usr/bin/env bats

# Test wiring:
# - sources lint-synth.sh in a subshell against the fixture wiki under
#   tests/fixtures/wiki-synth/, which phase 13 populated with the
#   memex source set (smart-quote / NBSP / ZWSP / hyphen / multi-paragraph
#   variants per spec).
# - Each test runs `lint.sh --only=synth --file=<path>` once the --only
#   flag is wired in task 14.9; until then, tests source lint-synth.sh
#   directly.

FIXTURES="tests/fixtures/wiki-synth"

run_synth_lint_file() {
  local page="$1"
  bash -c "
    source scripts/lint-synth.sh
    ERRORS=0; WARNS=0; INFOS=0
    synth_lint_file '$page' '$FIXTURES/content' 2>&1
    echo LINT-SUMMARY-RC=\$ERRORS
  "
}

@test "S1: double BEGIN marker → error" {
  run run_synth_lint_file "$FIXTURES/synthesis/s1-double-begin.md"
  [[ "$output" == *"LINT|ERROR"*"S1"*"BEGIN"* ]]
  [[ "$output" == *"LINT-SUMMARY-RC=1"* || "$output" == *"LINT-SUMMARY-RC=2"* ]]
}
