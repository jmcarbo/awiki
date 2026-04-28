#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/data" "$WORK/.awiki"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new us-pop --format=csv --from=demo.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "validate refreshes rows count when stale" {
  # Manually corrupt rows: count.
  sed -i.bak 's/^rows: 3$/rows: 99/' content/datasets/us-pop.md
  run bash scripts/dataset.sh validate us-pop
  [ "$status" -eq 0 ]
  run grep -E '^rows: 3$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
}

@test "validate is no-op when rows already match" {
  run bash scripts/dataset.sh validate us-pop
  [ "$status" -eq 0 ]
  run grep -E '^rows: 3$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
}

@test "validate fails when declared schema rejects data" {
  # Inject bad columns: schema (year as boolean — won't coerce).
  cat > content/datasets/us-pop.md <<'EOF'
---
title: "us-pop"
type: dataset
storage: inline
format: csv
rows: 3
columns:
  - { name: year, type: boolean }
  - { name: pop, type: number }
  - { name: country, type: string }
---

## Data

```csv
year,pop,country
2020,331,US
2021,333,US
2022,335,US
```
EOF
  run bash scripts/dataset.sh validate us-pop
  [ "$status" -ne 0 ]
  [[ "$output" == *"col=year"* ]]
}

@test "validate file storage variant updates rows from file" {
  # Manually flip to file storage for this test.
  cp demo.csv data/us-pop.csv
  cat > content/datasets/us-pop.md <<'EOF'
---
title: "us-pop"
type: dataset
storage: file
format: csv
rows: 0
data_path: data/us-pop.csv
---

## Provenance
EOF
  run bash scripts/dataset.sh validate us-pop
  [ "$status" -eq 0 ]
  run grep -E '^rows: 3$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
}

@test "validate refuses unknown slug" {
  run bash scripts/dataset.sh validate nope
  [ "$status" -ne 0 ]
  [[ "$output" == *"not found"* ]]
}
