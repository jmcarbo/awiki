#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  pushd "$WORK" >/dev/null
}
teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "dataset-rows count csv returns 3" {
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" count --format=csv --file=demo.csv
  [ "$status" -eq 0 ]
  [[ "$output" == "3" ]]
}

@test "dataset-rows count tsv returns 3" {
  printf 'year\tpop\tcountry\n2020\t331\tUS\n2021\t333\tUS\n2022\t335\tUS\n' > demo.tsv
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" count --format=tsv --file=demo.tsv
  [[ "$output" == "3" ]]
}

@test "dataset-rows count json (array) returns 3" {
  printf '[{"a":1},{"a":2},{"a":3}]' > demo.json
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" count --format=json --file=demo.json
  [[ "$output" == "3" ]]
}

@test "dataset-rows count topojson treats objects as 1 row" {
  printf '{"type":"Topology","objects":{}}' > demo.topojson
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" count --format=topojson --file=demo.topojson
  [[ "$output" == "1" ]]
}

@test "dataset-rows validate csv with passing schema returns 0" {
  cat > schema.json <<'EOF'
[{"name":"year","type":"integer"},{"name":"pop","type":"number"},{"name":"country","type":"string"}]
EOF
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" validate --format=csv --file=demo.csv --schema=schema.json
  [ "$status" -eq 0 ]
}

@test "dataset-rows validate csv flags wrong type" {
  # pop=331 is integer-shaped so passes; force a number-only check
  printf 'year,pop,country\n2020,3.5,US\n' > bad.csv
  cat > schema.json <<'EOF'
[{"name":"year","type":"integer"},{"name":"pop","type":"integer"},{"name":"country","type":"string"}]
EOF
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" validate --format=csv --file=bad.csv --schema=schema.json
  [ "$status" -ne 0 ]
  [[ "$output" == *"row=1"*"col=pop"*"want=integer"* ]]
}

@test "dataset-rows sample csv first 2 returns json array" {
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" sample --format=csv --file=demo.csv --n=2
  [ "$status" -eq 0 ]
  [[ "$output" == *'"year"'*'"2020"'*'"2021"'* ]]
}

@test "dataset-rows count empty csv returns 0" {
  printf 'year,pop,country\n' > empty.csv
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" count --format=csv --file=empty.csv
  [[ "$output" == "0" ]]
}

@test "dataset-rows validate json rejects bool for integer column" {
  printf '[{"a":true},{"a":1}]' > bools.json
  cat > schema.json <<'EOF'
[{"name":"a","type":"integer"}]
EOF
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" validate --format=json --file=bools.json --schema=schema.json
  [ "$status" -ne 0 ]
  [[ "$output" == *"row=1"*"col=a"*"want=integer"* ]]
}

@test "dataset-rows count dsv (semicolon) returns 3" {
  printf 'year;pop;country\n2020;331;US\n2021;333;US\n2022;335;US\n' > demo.dsv
  run python3 "$REPO_ROOT/scripts/lib/dataset-rows.py" count --format=dsv --file=demo.dsv
  [ "$status" -eq 0 ]
  [[ "$output" == "3" ]]
}
