#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/data" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_DATASET_INLINE_MAX_ROWS=500\nAWIKI_DATASET_INLINE_MAX_BYTES=51200\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "dataset new --from seeds inline rows from csv" {
  run bash scripts/dataset.sh new us-pop --format=csv --from=demo.csv
  [ "$status" -eq 0 ]
  [ -f content/datasets/us-pop.md ]
  run grep -E '^type: dataset$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
  run grep -E '^storage: inline$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
  run grep -E '^format: csv$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
  run grep -E '^rows: 3$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
  run grep -F "year,pop,country" content/datasets/us-pop.md
  [ "$status" -eq 0 ]
}

@test "dataset new without --from creates empty data block" {
  run bash scripts/dataset.sh new empty-set --format=csv
  [ "$status" -eq 0 ]
  [ -f content/datasets/empty-set.md ]
  run grep -E '^rows: 0$' content/datasets/empty-set.md
  [ "$status" -eq 0 ]
  run grep -A1 '## Data' content/datasets/empty-set.md
  [[ "$output" == *'```csv'* ]]
}

@test "dataset new refuses existing slug" {
  bash scripts/dataset.sh new us-pop --format=csv --from=demo.csv
  run bash scripts/dataset.sh new us-pop --format=csv
  [ "$status" -ne 0 ]
  [[ "$output" == *"already exists"* ]]
}

@test "dataset new rejects bad slug" {
  run bash scripts/dataset.sh new "Bad Slug" --format=csv
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid slug"* ]]
}

@test "dataset new rejects unknown format" {
  run bash scripts/dataset.sh new x --format=xlsx
  [ "$status" -ne 0 ]
  [[ "$output" == *"unsupported format"* ]]
}
