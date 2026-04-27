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
  run run_synth_lint_file "$FIXTURES/content/synthesis/s1-double-begin.md"
  [[ "$output" == *"LINT|ERROR"*"S1"*"BEGIN"* ]]
  [[ "$output" == *"LINT-SUMMARY-RC=1"* || "$output" == *"LINT-SUMMARY-RC=2"* ]]
}

@test "S2: missing ## Evidence in briefing → error" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s2-missing-evidence.md"
  [[ "$output" == *"LINT|ERROR"*"S2"*"Evidence"* ]]
}

@test "S3: smart-quote variant matches after normalization" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-smart-quote.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: NFC vs NFD variant matches after normalization" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-nfc-vs-nfd.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: NBSP-spaced quote matches after normalization" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-nbsp.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: em-dash inside quote body matches after normalization" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-em-dash.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: ZWSP-injected source matches after normalization" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-zwsp.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: multi-paragraph quote matches after collapse" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-multi-paragraph.md"
  [[ "$output" != *"LINT|ERROR"*"S3"* ]]
}

@test "S3: hallucinated quote → error with fuzzy suggestion" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s3-hallucination.md"
  [[ "$output" == *"LINT|ERROR"*"S3"*"hallucinat"* || "$output" == *"LINT|ERROR"*"S3"*"not found"* ]]
  [[ "$output" == *"suggestion:"* ]]
}

@test "S4: out-of-scope citation → error" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s4-out-of-scope.md"
  [[ "$output" == *"LINT|ERROR"*"S4"*"some-other-slug-not-in-scope"* ]]
}

@test "S5: tag-scope drift → warning" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s5-drift.md"
  [[ "$output" == *"LINT|WARN"*"S5"*"scope drift"* ]]
}

@test "S5: query-scope is skipped (no warning regardless of hash)" {
  run run_synth_lint_file "$FIXTURES/content/synthesis/s5-query-scope.md"
  [[ "$output" != *"LINT|WARN"*"S5"* ]]
  [[ "$output" != *"LINT|ERROR"*"S5"* ]]
}

s6_setup_repo() {
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/synthesis" "$WORK/synthesis-plugins" "$WORK/scripts"
  cp scripts/lint-synth*.sh scripts/lint-synth*.py "$WORK/scripts/"
  cp synthesis-plugins/briefing.md "$WORK/synthesis-plugins/"
  REPO_TOP="$(pwd)"
  pushd "$WORK" >/dev/null
  git init -q
  git config user.email t@t
  git config user.name t
}

s6_teardown_repo() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "S6: marker edit + last_updated bump → warning" {
  s6_setup_repo
  cat > content/synthesis/s6-page.md <<'P'
---
title: "S6"
date: 2026-04-01
last_updated: 2026-04-01
last_generated: 2026-04-15T00:00:00Z
type: synthesis
plugin: briefing
scope:
  slugs: [foo]
sources: ["[[foo]]"]
draft: false
---

Lead.

<!-- BEGIN GENERATED plugin=briefing scope_hash=a3f9c2 -->

## TL;DR
- foo claim [[foo]]

## Key Findings
- foo finding [[foo]]

## Open Questions
- q?

## Evidence
> "x" — [[foo]]

<!-- END GENERATED -->
P
  git add . && git commit -qm init

  # Edit inside markers AND bump last_updated.
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/synthesis/s6-page.md")
t = p.read_text()
t = t.replace("foo claim", "foo CLAIM-EDITED")
t = t.replace("last_updated: 2026-04-01", "last_updated: 2026-05-01")
p.write_text(t)
PY

  run bash -c "source scripts/lint-synth.sh; ERRORS=0;WARNS=0;INFOS=0; synth_check_s6 content/synthesis/s6-page.md"
  s6_teardown_repo
  [[ "$output" == *"LINT|WARN"*"S6"* ]]
}

@test "lint.sh --only=synth --file=<path> runs only synth rules" {
  run bash scripts/lint.sh --only=synth --file="$FIXTURES/content/synthesis/s2-missing-evidence.md" "$FIXTURES/content"
  [[ "$output" == *"LINT|ERROR"*"S2"* ]]
  # Mechanical lint rules from phase 2 (broken wikilink, etc.) should NOT fire here.
  [[ "$output" != *"broken wikilink"* ]]
}

@test "lint.sh --only=synth (no --file) walks content/synthesis/ in fixture root" {
  run bash scripts/lint.sh --only=synth "$FIXTURES/content"
  [[ "$output" == *"LINT-SUMMARY"* ]]
}

@test "S6: feedback-only edit + last_updated bump → no warning" {
  s6_setup_repo
  cat > content/synthesis/s6-page.md <<'P'
---
title: "S6"
date: 2026-04-01
last_updated: 2026-04-01
last_generated: 2026-04-15T00:00:00Z
type: synthesis
plugin: briefing
scope:
  slugs: [foo]
sources: ["[[foo]]"]
draft: false
---

Lead.

## Feedback

- earlier note

<!-- BEGIN GENERATED plugin=briefing scope_hash=a3f9c2 -->

## TL;DR
- foo claim [[foo]]

## Evidence
> "x" — [[foo]]

<!-- END GENERATED -->
P
  git add . && git commit -qm init

  # Edit ONLY the Feedback section AND bump last_updated.
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/synthesis/s6-page.md")
t = p.read_text()
t = t.replace("- earlier note", "- earlier note\n- a new feedback bullet")
t = t.replace("last_updated: 2026-04-01", "last_updated: 2026-05-01")
p.write_text(t)
PY

  run bash -c "source scripts/lint-synth.sh; ERRORS=0;WARNS=0;INFOS=0; synth_check_s6 content/synthesis/s6-page.md"
  s6_teardown_repo
  [[ "$output" != *"LINT|WARN"*"S6"* ]]
}
