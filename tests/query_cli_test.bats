#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  cp "$REPO_ROOT/tests/fixtures/query/regions.csv" "$WORK/regions.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/content/queries" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  "$REPO_ROOT/bin/awiki" dataset new --format=csv --from=trades.csv trades >/dev/null
  "$REPO_ROOT/bin/awiki" dataset new --format=csv --from=regions.csv regions >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "query run prints rows as JSON" {
  run bash scripts/query.sh run "SELECT category, COUNT(*) AS n FROM trades GROUP BY 1 ORDER BY 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"category":"food"'* ]] || [[ "$output" == *'"category": "food"'* ]]
}

@test "query run exits 3 on unknown dataset" {
  run bash scripts/query.sh run "SELECT * FROM ghost"
  [ "$status" -eq 3 ]
}

@test "query run exits 5 on SQL parse error" {
  run bash scripts/query.sh run "SELEC bogus"
  [ "$status" -eq 5 ]
}

@test "query run --out=summary materializes dataset" {
  run bash scripts/query.sh run \
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1" \
    --out=summary
  [ "$status" -eq 0 ]
  [ -f content/datasets/summary.md ]
  run grep -E '^type: dataset$' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -E '^sources: \[trades\]' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -E '^query: sha256-' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -A3 '## Data' content/datasets/summary.md
  [[ "$output" == *'```csv'* ]]
  [[ "$output" == *'category,total'* ]]
}

@test "query run --out=existing rejects without --force" {
  run bash scripts/query.sh run \
    "SELECT 1 AS x" --out=trades
  [ "$status" -eq 7 ]
}

@test "query run --out=existing overwrites with --force" {
  run bash scripts/query.sh run \
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1" \
    --out=summary
  [ "$status" -eq 0 ]
  run bash scripts/query.sh run \
    "SELECT category, COUNT(*) AS n FROM trades GROUP BY 1 ORDER BY 1" \
    --out=summary --force
  [ "$status" -eq 0 ]
  run grep -F 'category,n' content/datasets/summary.md
  [ "$status" -eq 0 ]
}
