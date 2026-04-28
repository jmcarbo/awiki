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
