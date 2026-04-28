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
  [[ "$output" == *"LINT|ERROR|content/datasets/bad.md|D1|"* ]]
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

@test "D5 fires when declared columns reject row data" {
  cat > content/datasets/bad.md <<'EOF'
---
title: "bad"
type: dataset
storage: inline
format: csv
rows: 1
columns:
  - { name: a, type: integer }
  - { name: b, type: integer }
---

## Data

```csv
a,b
not-a-number,7
```
EOF
  run bash scripts/lint-data.sh
  [[ "$output" == *"|D5|"* ]]
  [[ "$output" == *"col=a"* ]]
}

@test "D5 quiet when no columns declared (Q3=b)" {
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
not-a-number,maybe
```
EOF
  run bash scripts/lint-data.sh
  [[ "$output" != *"|D5|"* ]]
}

@test "D5 honors sample window (first 50 + last 10)" {
  # 200 rows, only the 100th is bad: lint should miss it (only validate via dataset-validate).
  {
    echo "a,b"
    for i in $(seq 1 99); do echo "$i,1"; done
    echo "BAD,1"
    for i in $(seq 101 200); do echo "$i,1"; done
  } > /tmp/big.csv
  cat > content/datasets/big.md <<'EOF'
---
title: "big"
type: dataset
storage: file
format: csv
rows: 200
columns:
  - { name: a, type: integer }
  - { name: b, type: integer }
data_path: /tmp/big.csv
---

## Provenance
EOF
  run bash scripts/lint-data.sh
  [[ "$output" != *"|D5|"* ]]
  rm -f /tmp/big.csv
}

@test "D6 warns when inline rows exceed AWIKI_DATASET_INLINE_MAX_ROWS" {
  printf 'AWIKI_DATASET_INLINE_MAX_ROWS=2\nAWIKI_DATASET_INLINE_MAX_BYTES=51200\n' > .awiki/config
  cat > content/datasets/big.md <<'EOF'
---
title: "big"
type: dataset
storage: inline
format: csv
rows: 3
---

## Data

```csv
a,b
1,2
3,4
5,6
```
EOF
  run bash scripts/lint-data.sh
  [[ "$output" == *"|D6|"* ]]
}

@test "D7 warns when cached rows != actual" {
  cat > content/datasets/stale.md <<'EOF'
---
title: "stale"
type: dataset
storage: inline
format: csv
rows: 99
---

## Data

```csv
a,b
1,2
```
EOF
  run bash scripts/lint-data.sh
  [[ "$output" == *"|D7|"* ]]
}

@test "D7 --fix updates cached rows" {
  cat > content/datasets/stale.md <<'EOF'
---
title: "stale"
type: dataset
storage: inline
format: csv
rows: 99
---

## Data

```csv
a,b
1,2
```
EOF
  run bash scripts/lint-data.sh --fix
  run grep -E '^rows: 1$' content/datasets/stale.md
  [ "$status" -eq 0 ]
}

@test "D8 warns when data_path is outside data/" {
  cp /dev/null /tmp/escape.csv
  printf 'a\n' > /tmp/escape.csv
  cat > content/datasets/escape.md <<'EOF'
---
title: "escape"
type: dataset
storage: file
format: csv
rows: 0
data_path: /tmp/escape.csv
---

## Provenance
EOF
  run bash scripts/lint-data.sh
  [[ "$output" == *"|D8|"* ]]
  rm -f /tmp/escape.csv
}

@test "D9 info-level warning when sources is empty" {
  cat > content/datasets/orphan.md <<'EOF'
---
title: "orphan"
type: dataset
storage: inline
format: csv
rows: 0
sources: []
---

## Data

```csv
a
```

## Sources
EOF
  run bash scripts/lint-data.sh
  [[ "$output" == *"|D9|"* ]]
}

@test "lint.sh --only=data delegates to lint-data.sh" {
  cat > content/datasets/bad.md <<'EOF'
---
type: dataset
storage: inline
---

## Data
EOF
  run bash scripts/lint.sh --only=data
  [[ "$output" == *"|D1|"* ]] || [[ "$output" == *"|D3|"* ]]
}

@test "lint.sh default mode includes D-codes" {
  cat > content/datasets/bad.md <<'EOF'
---
type: dataset
storage: inline
---

## Data
EOF
  run bash scripts/lint.sh
  [[ "$output" == *"|D"*"|"* ]]
}

@test "lint.sh exits non-zero when D-code errors fire" {
  cat > content/datasets/bad.md <<'EOF'
---
type: dataset
format: csv
---
EOF
  run bash scripts/lint.sh
  # Errors should escalate exit code.
  [ "$status" -ne 0 ]
  [[ "$output" == *"|D1|"* ]]
}
