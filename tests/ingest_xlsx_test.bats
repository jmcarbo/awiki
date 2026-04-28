#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

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

@test "xlsx-extract multi-sheet skips hidden and processes visible non-empty" {
  mkdir -p out csv
  # --separate-stderr keeps XLSX-SKIP lines out of $output so json.load works.
  run --separate-stderr python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/multi-sheet-with-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel raw/processed/_originals/multi --original-rel raw/processed/_originals/multi/multi-sheet-with-hidden.xlsx \
    --slug-prefix multi
  [ "$status" -eq 0 ]
  [ -f out/multi--sales.md ]
  [ -f out/multi--inventory.md ]
  [ ! -f out/multi--hiddenscratch.md ]
  [[ "$stderr" == *"XLSX-SKIP|sheet=HiddenScratch|reason=hidden"* ]]
  echo "$output" | python3 -c "import sys,json; m=json.load(sys.stdin); names=[s['name'] for s in m['sheets']]; assert names==['Sales','Inventory'], names"
}

@test "xlsx-extract empty-and-hidden workbook returns empty manifest" {
  mkdir -p out csv
  run --separate-stderr python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/empty-and-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix x
  [ "$status" -eq 0 ]
  [[ "$stderr" == *"XLSX-SKIP|sheet=Empty|reason=empty"* ]]
  [[ "$stderr" == *"XLSX-SKIP|sheet=AlsoHidden|reason=hidden"* ]]
  echo "$output" | python3 -c "import sys,json; m=json.load(sys.stdin); assert m['sheets']==[]"
}

@test "xlsx-extract collision sheet names get -2 suffix" {
  mkdir -p out csv
  run --separate-stderr python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/collision-sheet-names.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix wb
  [ "$status" -eq 0 ]
  [ -f out/wb--sales-data.md ]
  [ -f out/wb--sales-data-2.md ]
  [[ "$stderr" == *"XLSX-DUP|slug=wb--sales-data|resolved=wb--sales-data-2"* ]]
}

@test "xlsx-extract corrupt workbook exits 3" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/corrupt.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix bad
  [ "$status" -eq 3 ]
}

@test "xlsx-extract on injected mid-run failure leaves no partial files" {
  mkdir -p out csv
  # AWIKI_XLSX_FORCE_FAIL_AFTER=1 raises after the 1st sheet's rename completes
  # so the rollback path that unlinks already-renamed final files is exercised.
  # multi-sheet-with-hidden.xlsx has 2 visible sheets (Sales, Inventory).
  run --separate-stderr env AWIKI_XLSX_FORCE_FAIL_AFTER=1 python3 \
    "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/multi-sheet-with-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix multi
  [ "$status" -eq 4 ]
  [[ "$stderr" == *"XLSX-ERROR|reason=io|err=forced failure"* ]]
  # No partial md/csv files in destinations, no leftover hidden temp dirs.
  [ -z "$(ls -A out)" ]
  [ -z "$(ls -A csv)" ]
}

@test "xlsx-extract pipes/newlines in error message are escaped" {
  # Sanity check: trigger the same rollback and assert stderr has no
  # unescaped pipe inside the err= field that would confuse a downstream
  # parser splitting on '|'.
  mkdir -p out csv
  run --separate-stderr env AWIKI_XLSX_FORCE_FAIL_AFTER=1 python3 \
    "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/multi-sheet-with-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix multi
  [ "$status" -eq 4 ]
  # The error line should split into exactly three fields when cut on '|'.
  fields=$(echo "$stderr" | grep "^XLSX-ERROR" | awk -F'|' '{print NF}')
  [ "$fields" = "3" ]
}

@test "ingest-xlsx archives original, removes source, prints XLSX-CONVERTED" {
  cp raw/inbox/batch/single-sheet.xlsx raw/inbox/interactive/single-sheet.xlsx
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/interactive/single-sheet.xlsx
  [ "$status" -eq 0 ]
  [ -f raw/processed/_originals/single-sheet/single-sheet.xlsx ]
  [ -f raw/processed/_originals/single-sheet/single-sheet--sales.csv ]
  [ -f raw/inbox/interactive/single-sheet--sales.md ]
  [ ! -f raw/inbox/interactive/single-sheet.xlsx ]
  [[ "$output" == *"XLSX-CONVERTED|"* ]]
  [[ "$output" == *"XLSX-NEXT|raw/inbox/interactive/single-sheet--sales.md"* ]]
}

@test "ingest-xlsx rejects bad extension" {
  echo "x" > raw/inbox/interactive/notes.txt
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/interactive/notes.txt
  [ "$status" -eq 2 ]
  [[ "$output" == *"reason=bad-ext"* ]]
}

@test "ingest-xlsx rejects path outside raw/inbox/" {
  cp raw/inbox/batch/single-sheet.xlsx /tmp/outside-$$.xlsx
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" "/tmp/outside-$$.xlsx"
  [ "$status" -eq 2 ]
  [[ "$output" == *"reason=bad-path"* ]]
  rm -f /tmp/outside-$$.xlsx
}

@test "ingest-xlsx exits 5 with hint when python-calamine is faked-missing" {
  run env AWIKI_FAKE_MISSING=python_calamine bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/batch/single-sheet.xlsx
  [ "$status" -eq 5 ]
  [[ "$output" == *"reason=missing-dep"* ]]
  [[ "$output" == *"pip3 install python-calamine"* ]]
}

@test "ingest-xlsx empty workbook exits 6 and preserves source" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/batch/empty-and-hidden.xlsx
  [ "$status" -eq 6 ]
  [[ "$output" == *"XLSX-EMPTY|"* ]]
  [ -f raw/inbox/batch/empty-and-hidden.xlsx ]
}

@test "ingest-xlsx corrupt workbook exits 3 and preserves source" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/batch/corrupt.xlsx
  [ "$status" -eq 3 ]
  [ -f raw/inbox/batch/corrupt.xlsx ]
}

@test "ingest-xlsx --preview-rows propagated" {
  cp raw/inbox/batch/single-sheet.xlsx raw/inbox/interactive/single-sheet.xlsx
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/interactive/single-sheet.xlsx --preview-rows=1
  [ "$status" -eq 0 ]
  grep -E '^rows_preview: 1$' raw/inbox/interactive/single-sheet--sales.md
}
