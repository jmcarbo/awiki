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

@test "S2: missing ## Evidence in briefing → error" {
  run run_synth_lint_file "$FIXTURES/synthesis/s2-missing-evidence.md"
  [[ "$output" == *"LINT|ERROR"*"S2"*"Evidence"* ]]
}

@test "S3: smart-quote variant matches after normalization" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-smart-quote.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: NFC vs NFD variant matches after normalization" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-nfc-vs-nfd.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: NBSP-spaced quote matches after normalization" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-nbsp.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: em-dash inside quote body matches after normalization" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-em-dash.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: ZWSP-injected source matches after normalization" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-zwsp.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: multi-paragraph quote matches after collapse" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-multi-paragraph.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: hallucinated quote → error with fuzzy suggestion" {
  run run_synth_lint_file "$FIXTURES/synthesis/s3-hallucination.md"
  [[ "$output" == *"LINT|ERROR"*"S3"*"hallucinat"* || "$output" == *"LINT|ERROR"*"S3"*"not found"* ]]
  [[ "$output" == *"suggestion:"* ]]
}

@test "S4: out-of-scope citation → error" {
  run run_synth_lint_file "$FIXTURES/synthesis/s4-out-of-scope.md"
  [[ "$output" == *"LINT|ERROR"*"S4"*"some-other-slug-not-in-scope"* ]]
}

@test "S5: tag-scope drift → warning" {
  run run_synth_lint_file "$FIXTURES/synthesis/s5-drift.md"
  [[ "$output" == *"LINT|WARN"*"S5"*"scope drift"* ]]
}

@test "S5: query-scope is skipped (no warning regardless of hash)" {
  run run_synth_lint_file "$FIXTURES/synthesis/s5-query-scope.md"
  [[ "$output" != *"LINT|WARN"*"S5"* ]]
  [[ "$output" != *"LINT|ERROR"*"S5"* ]]
}
