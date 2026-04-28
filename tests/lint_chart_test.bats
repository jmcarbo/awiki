#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts" "$WORK/content/concepts" "$WORK/.awiki"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "C1 fires on malformed JSON in fence" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{ this is not json
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C1|"* ]]
}

@test "C2 fires when spec lacks mark/layer/concat/facet" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"data": {"name": "[[demo]]"}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C2|"* ]]
}

@test "C3 fires on missing dataset" {
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[no-such]]"}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C3|"* ]]
  [[ "$output" == *"no-such"* ]]
}

@test "no C-codes on a clean inline chart" {
  cat > content/datasets/demo.md <<'EOF'
---
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
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" != *"|C1|"* ]]
  [[ "$output" != *"|C2|"* ]]
  [[ "$output" != *"|C3|"* ]]
}

@test "C4 fires when spec field not in dataset columns" {
  cat > content/datasets/typed.md <<'EOF'
---
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
1,2
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[typed]]"},"encoding":{"x":{"field":"missing"}}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C4|"* ]]
  [[ "$output" == *"missing"* ]]
}

@test "C5 fires on type:chart page with no fence" {
  cat > content/charts/empty.md <<'EOF'
---
type: chart
chart_engine: vega-lite
---

# empty chart page
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C5|"* ]]
}

@test "C6 fires when chart_data disagrees with body data.name" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/datasets/other.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/charts/c.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

```vega-lite
{"mark":"bar","data":{"name":"[[other]]"},"encoding":{"x":{"field":"a"}}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C6|"* ]]
}

@test "C6 --fix updates chart_data to match body" {
  cat > content/datasets/demo.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/datasets/other.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a
```
EOF
  cat > content/charts/c.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

```vega-lite
{"mark":"bar","data":{"name":"[[other]]"},"encoding":{"x":{"field":"a"}}}
```
EOF
  run bash scripts/lint-chart.sh --fix
  run grep -F 'chart_data: "[[other]]"' content/charts/c.md
  [ "$status" -eq 0 ]
}
