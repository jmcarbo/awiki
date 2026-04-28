#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/tests/fixtures" "$WORK/tests-fixtures" 2>/dev/null || true
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  mkdir -p "$WORK/content"
  printf -- "---\ntitle: WIKI\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "smoke: full data layer flow (init -> new -> lint -> validate)" {
  bash scripts/data-init.sh
  [ -d content/datasets ]
  [ -d data ]

  bash scripts/dataset.sh new us-pop --format=csv --from=demo.csv
  [ -f content/datasets/us-pop.md ]
  run grep -E '^rows: 3$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]

  run bash scripts/lint-data.sh
  [[ "$output" != *"|D1|"* ]]
  [[ "$output" != *"|D2|"* ]]
  [[ "$output" != *"|D3|"* ]]
  [[ "$output" != *"|D4|"* ]]
  [[ "$output" != *"|D5|"* ]]

  run bash scripts/dataset.sh validate us-pop
  [ "$status" -eq 0 ]
}
