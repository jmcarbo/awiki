#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/data" "$WORK/.awiki"
  printf -- "AWIKI_DATASET_INLINE_MAX_ROWS=2\nAWIKI_DATASET_INLINE_MAX_BYTES=51200\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "compact inline -> file when over row threshold" {
  bash scripts/dataset.sh new us-pop --format=csv --from=demo.csv >/dev/null
  # Threshold is 2; demo.csv has 3 rows -> over.
  run bash scripts/dataset.sh compact us-pop
  [ "$status" -eq 0 ]
  [ -f data/us-pop.csv ]
  run grep -E '^storage: file$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
  run grep -E '^data_path: data/us-pop.csv$' content/datasets/us-pop.md
  [ "$status" -eq 0 ]
  # Inline data block is gone.
  run grep -F '```csv' content/datasets/us-pop.md
  [ "$status" -ne 0 ]
  # The ## Data heading is also gone.
  run grep -E '^## Data[[:space:]]*$' content/datasets/us-pop.md
  [ "$status" -ne 0 ]
}

@test "compact file -> inline when under threshold" {
  # Build a small file dataset by hand.
  printf 'a,b\n1,2\n' > data/tiny.csv
  cat > content/datasets/tiny.md <<'EOF'
---
title: "tiny"
type: dataset
storage: file
format: csv
rows: 1
data_path: data/tiny.csv
---

## Provenance
EOF
  run bash scripts/dataset.sh compact tiny
  [ "$status" -eq 0 ]
  run grep -E '^storage: inline$' content/datasets/tiny.md
  [ "$status" -eq 0 ]
  [ ! -f data/tiny.csv ]
  run grep -F '```csv' content/datasets/tiny.md
  [ "$status" -eq 0 ]
  # data_path frontmatter is removed.
  run grep -E '^data_path:' content/datasets/tiny.md
  [ "$status" -ne 0 ]
}

@test "compact is idempotent on already-compact inline (under threshold)" {
  printf 'a,b\n1,2\n' > small.csv
  bash scripts/dataset.sh new tiny --format=csv --from=small.csv >/dev/null
  run bash scripts/dataset.sh compact tiny
  [ "$status" -eq 0 ]
  run grep -E '^storage: inline$' content/datasets/tiny.md
  [ "$status" -eq 0 ]
}

@test "compact refuses to inline a file dataset that is over threshold" {
  cp demo.csv data/big.csv
  cat > content/datasets/big.md <<'EOF'
---
title: "big"
type: dataset
storage: file
format: csv
rows: 3
data_path: data/big.csv
---

## Provenance
EOF
  run bash scripts/dataset.sh compact big
  [ "$status" -ne 0 ]
  [[ "$output" == *"over threshold"* ]]
}

@test "compact refuses file -> inline when ## Provenance heading missing" {
  printf 'a,b\n1,2\n' > data/orphan.csv
  cat > content/datasets/orphan.md <<'EOF'
---
title: "orphan"
type: dataset
storage: file
format: csv
rows: 1
data_path: data/orphan.csv
---

## Some Other Section
EOF
  run bash scripts/dataset.sh compact orphan
  [ "$status" -ne 0 ]
  [[ "$output" == *"missing"* ]]
  [[ "$output" == *"Provenance"* ]]
  # Source data must still exist.
  [ -f data/orphan.csv ]
  # Frontmatter must be unchanged: storage still file, data_path still present.
  run grep -E '^storage: file$' content/datasets/orphan.md
  [ "$status" -eq 0 ]
  run grep -E '^data_path: data/orphan.csv$' content/datasets/orphan.md
  [ "$status" -eq 0 ]
}
