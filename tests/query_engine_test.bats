#!/usr/bin/env bats
bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  cp "$REPO_ROOT/tests/fixtures/query/regions.csv" "$WORK/regions.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/.awiki" "$WORK/.cache/duckdb"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
  bash scripts/dataset.sh new regions --format=csv --from=regions.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "query-extract writes fence body for csv dataset" {
  run python3 scripts/lib/query-extract.py \
    --slug=trades --datasets-dir=content/datasets --out=.cache/duckdb/trades.csv
  [ "$status" -eq 0 ]
  [ -f .cache/duckdb/trades.csv ]
  run grep -F "id,category,amount,region_id" .cache/duckdb/trades.csv
  [ "$status" -eq 0 ]
  # Trailing newline + 5 data rows + 1 header.
  run wc -l < .cache/duckdb/trades.csv
  [ "$output" = "       6" ] || [ "$output" = "6" ]
}

@test "query-extract fails on missing dataset" {
  run --separate-stderr python3 scripts/lib/query-extract.py \
    --slug=ghost --datasets-dir=content/datasets --out=.cache/duckdb/ghost.csv
  [ "$status" -eq 3 ]
  [[ "$stderr" == *"dataset not found"* ]]
}

@test "query-extract fails on absent fence" {
  cat > content/datasets/empty.md <<'MD'
---
type: dataset
storage: inline
format: csv
rows: 0
---
# empty
MD
  run --separate-stderr python3 scripts/lib/query-extract.py \
    --slug=empty --datasets-dir=content/datasets --out=.cache/duckdb/empty.csv
  [ "$status" -eq 4 ]
  [[ "$stderr" == *"no \`\`\`csv fence"* ]]
}

@test "query-extract follows storage:file -> data_path" {
  mkdir -p data
  printf 'id,name\n1,alpha\n2,beta\n' > data/big.csv
  cat > content/datasets/big.md <<'MD'
---
type: dataset
storage: file
format: csv
rows: 2
data_path: data/big.csv
---
# big
MD
  run python3 scripts/lib/query-extract.py \
    --slug=big --datasets-dir=content/datasets --out=.cache/duckdb/big.csv
  [ "$status" -eq 0 ]
  diff data/big.csv .cache/duckdb/big.csv
}
