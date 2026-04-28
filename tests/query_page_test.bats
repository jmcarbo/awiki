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

@test "query render-one materializes target dataset" {
  bash scripts/query.sh new top-trades --out=summary >/dev/null
  # Replace placeholder SQL with a real query.
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/top-trades.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1",
)
p.write_text(text)
PY
  run bash scripts/query.sh render-one top-trades
  [ "$status" -eq 0 ]
  [ -f content/datasets/summary.md ]
  run grep -F 'category,total' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -E '^sql_hash: sha256-' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  [ -f content/queries/top-trades.sql.hash ]
}

@test "render-one is byte-identical on second run" {
  bash scripts/query.sh new dup-q --out=dup-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/dup-q.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  bash scripts/query.sh render-one dup-q
  cp content/datasets/dup-out.md /tmp/dup-out-1.md
  bash scripts/query.sh render-one dup-q
  diff /tmp/dup-out-1.md content/datasets/dup-out.md
}

@test "render-one rejects non-deterministic SQL" {
  bash scripts/query.sh new ndq --out=ndq-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/ndq.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT NOW() AS t FROM trades ORDER BY t",
)
p.write_text(text)
PY
  run bash scripts/query.sh render-one ndq
  [ "$status" -eq 6 ]
  [[ "$output" == *"non-deterministic"* ]]
}
