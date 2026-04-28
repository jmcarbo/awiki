#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/content/queries" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "query.sh new scaffolds type:query page" {
  run bash scripts/query.sh new top-trades --out=summary
  [ "$status" -eq 0 ]
  [ -f content/queries/top-trades.md ]
  run grep -E '^type: query$' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  run grep -E '^out: summary$' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  run grep -E '^deterministic: true$' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  run grep -F '```sql' content/queries/top-trades.md
  [ "$status" -eq 0 ]
}

@test "query.sh new rejects existing slug" {
  bash scripts/query.sh new dup --out=summary >/dev/null
  run bash scripts/query.sh new dup --out=summary
  [ "$status" -ne 0 ]
}
