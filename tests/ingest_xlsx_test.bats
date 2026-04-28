#!/usr/bin/env bats

setup() {
  command -v python3 >/dev/null 2>&1 || skip "python3 not installed"
  python3 -c "import python_calamine" 2>/dev/null || skip "python-calamine not installed"
  WORK="$(mktemp -d)/repo"
  REPO="$BATS_TEST_DIRNAME/.."
  mkdir -p "$WORK/raw/inbox/batch" "$WORK/raw/inbox/interactive" "$WORK/raw/processed/_originals"
  cp "$REPO/tests/fixtures/xlsx/"*.xlsx "$WORK/raw/inbox/batch/" 2>/dev/null || true
  cd "$WORK"
}
teardown() { cd - >/dev/null; rm -rf "$WORK"; }

@test "xlsx-extract --slugify produces kebab-case" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --slugify "Q1 Report — Sales!"
  [ "$status" -eq 0 ]
  [ "$output" = "q1-report-sales" ]
}

@test "xlsx-extract --slugify trims leading and trailing dashes" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --slugify "---abc---"
  [ "$status" -eq 0 ]
  [ "$output" = "abc" ]
}

@test "xlsx-extract --help prints usage" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"--in"* ]]
  [[ "$output" == *"--out-dir"* ]]
  [[ "$output" == *"--csv-dir"* ]]
}
