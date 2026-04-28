#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
  cat > sample.md <<'EOF'
---
title: "demo"
type: dataset
storage: inline
format: csv
rows: 3
---

# demo

Body.
EOF
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "fm_get reads scalar fields" {
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  run fm_get sample.md storage
  [[ "$output" == "inline" ]]
  run fm_get sample.md format
  [[ "$output" == "csv" ]]
  run fm_get sample.md rows
  [[ "$output" == "3" ]]
}

@test "fm_get returns empty when key absent" {
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  run fm_get sample.md data_path
  [[ "$output" == "" ]]
}

@test "fm_set updates an existing scalar" {
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  fm_set sample.md rows 42
  run fm_get sample.md rows
  [[ "$output" == "42" ]]
}

@test "fm_set adds a new scalar before the closing ---" {
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  fm_set sample.md data_path "data/demo.csv"
  run fm_get sample.md data_path
  [[ "$output" == "data/demo.csv" ]]
  # Body is preserved.
  run grep -F "Body." sample.md
  [ "$status" -eq 0 ]
}

@test "fm_remove drops a key" {
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  fm_remove sample.md rows
  run fm_get sample.md rows
  [[ "$output" == "" ]]
}

@test "fm_get unwraps quoted scalar (title)" {
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  run fm_get sample.md title
  [[ "$output" == "demo" ]]
}

@test "fm_set on existing empty-valued key updates without duplicating" {
  cat > empty.md <<'EOF'
---
title: "demo"
empty_key:
storage: inline
---

Body.
EOF
  source "$REPO_ROOT/scripts/lib/dataset-fm.sh"
  fm_set empty.md empty_key now-set
  run grep -c '^empty_key:' empty.md
  [[ "$output" == "1" ]]
  run fm_get empty.md empty_key
  [[ "$output" == "now-set" ]]
}
