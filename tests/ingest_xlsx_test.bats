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

@test "xlsx-extract --infer-headers returns header strings when row is all strings" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-headers '["region","amount","date"]'
  [ "$status" -eq 0 ]
  [ "$output" = '["region", "amount", "date"]' ]
}

@test "xlsx-extract --infer-headers synthesises col_N when first row not all strings" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-headers '[1,2,3]'
  [ "$status" -eq 0 ]
  [ "$output" = '["col_1", "col_2", "col_3"]' ]
}

@test "xlsx-extract --infer-headers synthesises col_N when any cell empty" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-headers '["region","",""]'
  [ "$status" -eq 0 ]
  [ "$output" = '["col_1", "col_2", "col_3"]' ]
}

@test "xlsx-extract --infer-type classifies column samples" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[1,2,3]'
  [ "$status" -eq 0 ]
  [ "$output" = "number" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '["a","b"]'
  [ "$status" -eq 0 ]
  [ "$output" = "text" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[true,false]'
  [ "$status" -eq 0 ]
  [ "$output" = "bool" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[1,"a"]'
  [ "$status" -eq 0 ]
  [ "$output" = "mixed" ]
}

@test "xlsx-extract --infer-type returns text for all-None and empty samples" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[null,null]'
  [ "$status" -eq 0 ]
  [ "$output" = "text" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[]'
  [ "$status" -eq 0 ]
  [ "$output" = "text" ]
}

@test "xlsx-extract single-sheet writes csv + md with frontmatter and preview table" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/single-sheet.xlsx \
    --out-dir out \
    --csv-dir csv \
    --csv-rel raw/processed/_originals/single-sheet \
    --original-rel raw/processed/_originals/single-sheet/single-sheet.xlsx \
    --slug-prefix single-sheet \
    --preview-rows 10
  [ "$status" -eq 0 ]
  [ -f out/single-sheet--sales.md ]
  [ -f csv/single-sheet--sales.csv ]
  # Frontmatter has rows_total=3
  grep -E '^rows_total: 3$' out/single-sheet--sales.md
  # Frontmatter has csv path written verbatim from --csv-rel
  grep -F 'csv: raw/processed/_originals/single-sheet/single-sheet--sales.csv' out/single-sheet--sales.md
  # Frontmatter columns list preserves original strings
  grep -E '^columns: \["region", "amount", "date"\]$' out/single-sheet--sales.md
  # CSV body has the data row
  grep -F 'EMEA,100,2026-01-01' csv/single-sheet--sales.csv
  # Manifest emitted on stdout (single line of JSON)
  echo "$output" | python3 -c "import sys,json; m=json.load(sys.stdin); assert m['workbook_slug']=='single-sheet'; assert len(m['sheets'])==1; assert m['sheets'][0]['rows_total']==3"
}
