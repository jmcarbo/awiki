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
  "$REPO_ROOT/bin/awiki" dataset new --format=csv --from=trades.csv trades >/dev/null
  "$REPO_ROOT/bin/awiki" dataset new --format=csv --from=regions.csv regions >/dev/null
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

@test "engine runs SELECT against single dataset" {
  run bash scripts/lib/query-engine.sh run "SELECT COUNT(*) AS n FROM trades"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"n":5'* ]] || [[ "$output" == *'"n": 5'* ]]
}

@test "engine runs JOIN across two datasets" {
  run bash scripts/lib/query-engine.sh run \
    "SELECT r.name, COUNT(*) AS n FROM trades t JOIN regions r ON t.region_id=r.id GROUP BY 1 ORDER BY 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"name":"amer"'* ]] || [[ "$output" == *'"name": "amer"'* ]]
  [[ "$output" == *'"name":"emea"'* ]] || [[ "$output" == *'"name": "emea"'* ]]
}

@test "engine exits 3 for unknown dataset" {
  run bash scripts/lib/query-engine.sh run "SELECT * FROM ghost"
  [ "$status" -eq 3 ]
  [[ "$output" == *"unknown dataset: ghost"* ]]
}

@test "engine exits 9 for ATTACH" {
  run bash scripts/lib/query-engine.sh run "ATTACH 'foo.db'; SELECT 1"
  [ "$status" -eq 9 ]
  [[ "$output" == *"external attach not supported"* ]]
}

@test "engine emits stable result hash" {
  h1=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  h2=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  [ "$h1" = "$h2" ]
  [ -n "$h1" ]
}

@test "hash exits 3 for unknown dataset" {
  run bash scripts/lib/query-engine.sh hash "SELECT * FROM ghost ORDER BY 1"
  [ "$status" -eq 3 ]
  [[ "$output" == *"unknown dataset: ghost"* ]]
}

@test "hash exits 9 for ATTACH" {
  run bash scripts/lib/query-engine.sh hash "ATTACH 'foo.db'; SELECT 1"
  [ "$status" -eq 9 ]
}

@test "hash is whitespace + case insensitive" {
  h1=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  h2=$(bash scripts/lib/query-engine.sh hash "select   ID  from  TRADES order by id")
  [ "$h1" = "$h2" ]
}

@test "hash unaffected by frontmatter changes (only fence body matters)" {
  h1=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  # Touch last_updated in trades.md frontmatter
  python3 - <<'PY'
import re, pathlib
p = pathlib.Path("content/datasets/trades.md")
text = p.read_text()
text = re.sub(r'^last_updated:.*$', 'last_updated: 2099-01-01', text, count=1, flags=re.M)
p.write_text(text)
PY
  h2=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  [ "$h1" = "$h2" ]
}

@test "hash differs when fence body changes" {
  h1=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  # Append a row to the trades fence body via dataset compact path is heavy;
  # instead mutate the markdown body directly.
  python3 - <<'PY'
import re, pathlib
p = pathlib.Path("content/datasets/trades.md")
text = p.read_text()
text = re.sub(r"```csv\n(.*?)\n```", lambda m: "```csv\n" + m.group(1) + "\n6,gear,99.0,2\n```", text, count=1, flags=re.S)
p.write_text(text)
PY
  h2=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  [ "$h1" != "$h2" ]
}

@test "run with no SQL arg exits 1 with usage" {
  run bash scripts/lib/query-engine.sh run
  [ "$status" -eq 1 ]
  [[ "$output" == *"usage"* ]]
}
