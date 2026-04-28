#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/content/queries" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "Q1: missing sql fence flagged" {
  cat > content/queries/no-sql.md <<'MD'
---
type: query
out: foo
---
# x
MD
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q1|"* ]]
}

@test "Q2: unknown referenced dataset flagged" {
  cat > content/queries/bad-ref.md <<'MD'
---
type: query
out: foo
---
## SQL
```sql
SELECT * FROM ghost ORDER BY 1
```
MD
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q2|"* ]]
  [[ "$output" == *"ghost"* ]]
}

@test "Q1+Q2: clean query page lints clean" {
  bash scripts/query.sh new ok-q --out=ok-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/ok-q.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  run bash scripts/lint-query.sh
  [ "$status" -eq 0 ]
  [[ "$output" != *"|Q1|"* ]]
  [[ "$output" != *"|Q2|"* ]]
}

@test "Q3: NOW() flagged" {
  cat > content/queries/nd.md <<'MD'
---
type: query
out: foo
---
## SQL
```sql
SELECT NOW() AS t FROM trades ORDER BY t
```
MD
  # Need source dataset for Q2 to pass.
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q3|"* ]]
}

@test "Q4: stale sql_hash flagged" {
  bash scripts/query.sh new s-q --out=s-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/s-q.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  bash scripts/query.sh render-one s-q
  # Mutate SQL but do not re-render: sidecar is now stale.
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/s-q.md")
text = p.read_text().replace(
    "SELECT id FROM trades ORDER BY id",
    "SELECT id FROM trades WHERE id > 0 ORDER BY id",
)
p.write_text(text)
PY
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q4|"* ]]
}
