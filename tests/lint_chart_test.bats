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

@test "C7 warns when sidecar missing" {
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
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C7|"* ]]
}

@test "C8 info when same dataset referenced >5 times" {
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
  for i in 1 2 3 4 5 6; do
    cat > content/concepts/p$i.md <<EOF
---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
\`\`\`
EOF
  done
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C8|"* ]]
}

@test "C9 warns on hand-edit inside chart-preview managed region" {
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
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"a"}}}
```

<!-- BEGIN chart-preview:p-fig0 -->
hand-written content here
extra line
<!-- END chart-preview:p-fig0 -->
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C9|"* ]]
}

@test "C-PRIV blocks non-private chart referencing private dataset" {
  mkdir -p content/datasets/private
  cat > content/datasets/private/secret.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
tags: [private]
---

## Data

```csv
a
```
EOF
  cat > content/concepts/p.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[secret]]"},"encoding":{"x":{"field":"a"}}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" == *"|C-PRIV|"* ]]
}

@test "C-PRIV quiet when both sides private" {
  mkdir -p content/datasets/private content/private
  cat > content/datasets/private/secret.md <<'EOF'
---
type: dataset
storage: inline
format: csv
rows: 0
tags: [private]
---

## Data

```csv
a
```
EOF
  cat > content/private/p.md <<'EOF'
---
type: concept
tags: [private]
---

```vega-lite
{"mark":"bar","data":{"name":"[[secret]]"},"encoding":{"x":{"field":"a"}}}
```
EOF
  run bash scripts/lint-chart.sh
  [[ "$output" != *"|C-PRIV|"* ]]
}
