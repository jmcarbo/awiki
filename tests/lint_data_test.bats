#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/data" "$WORK/.awiki"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

_write_fm() { # _write_fm <slug> <fm-body> <body>
  local slug="$1" fm="$2" body="${3:-}"
  cat > "content/datasets/$slug.md" <<EOF
---
$fm
---

$body
EOF
}

@test "D1 fires when storage missing" {
  _write_fm bad "title: bad
type: dataset
format: csv
rows: 0"
  run bash scripts/lint-data.sh
  [[ "$output" == *"LINT|error|content/datasets/bad.md|D1|"* ]]
}

@test "D1 fires when format missing" {
  _write_fm bad "title: bad
type: dataset
storage: inline
rows: 0"
  run bash scripts/lint-data.sh
  [[ "$output" == *"D1|"*"format"* ]]
}

@test "D2 fires when storage=file but data_path missing" {
  _write_fm bad "title: bad
type: dataset
storage: file
format: csv
rows: 0"
  run bash scripts/lint-data.sh
  [[ "$output" == *"D2|"* ]]
}

@test "D2 fires when storage=file but data file absent" {
  _write_fm bad "title: bad
type: dataset
storage: file
format: csv
rows: 0
data_path: data/missing.csv"
  run bash scripts/lint-data.sh
  [[ "$output" == *"D2|"* ]]
}

@test "D3 fires when storage=inline but no fenced block" {
  _write_fm bad "title: bad
type: dataset
storage: inline
format: csv
rows: 0" "## Data

(no fence)
"
  run bash scripts/lint-data.sh
  [[ "$output" == *"D3|"* ]]
}

@test "D4 fires when format != fence info-string" {
  _write_fm bad "title: bad
type: dataset
storage: inline
format: csv
rows: 1" "## Data

\`\`\`tsv
a\tb
1\t2
\`\`\`
"
  run bash scripts/lint-data.sh
  [[ "$output" == *"D4|"* ]]
}

@test "no D-codes on a clean dataset" {
  cat > content/datasets/good.md <<'EOF'
---
title: "good"
type: dataset
storage: inline
format: csv
rows: 1
---

## Data

```csv
a,b
1,2
```
EOF
  run bash scripts/lint-data.sh
  [ "$status" -eq 0 ]
  [[ "$output" != *"|D1|"* ]]
  [[ "$output" != *"|D2|"* ]]
  [[ "$output" != *"|D3|"* ]]
  [[ "$output" != *"|D4|"* ]]
}
