#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts"
  cat > "$WORK/content/datasets/demo.md" <<'EOF'
---
title: "demo"
type: dataset
storage: inline
format: csv
rows: 0
---

## Data

```csv
a,b
```
EOF
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "chart new --data scaffolds a type:chart page with skeleton fence" {
  run bash scripts/chart.sh new demo-bar --data=demo
  [ "$status" -eq 0 ]
  [ -f content/charts/demo-bar.md ]
  run grep -E '^type: chart$' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -E '^chart_engine: vega-lite$' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -F 'chart_data: "[[demo]]"' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -F '```vega-lite' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
  run grep -F '"name": "[[demo]]"' content/charts/demo-bar.md
  [ "$status" -eq 0 ]
}

@test "chart new --data refuses unknown dataset slug" {
  run bash scripts/chart.sh new x --data=missing
  [ "$status" -ne 0 ]
  [[ "$output" == *"dataset not found"* ]]
}

@test "chart new refuses existing slug" {
  bash scripts/chart.sh new demo-bar --data=demo
  run bash scripts/chart.sh new demo-bar --data=demo
  [ "$status" -ne 0 ]
  [[ "$output" == *"already exists"* ]]
}

@test "just chart-new wrapper works" {
  run just chart-new demo-bar --data=demo
  [ "$status" -eq 0 ]
  [ -f content/charts/demo-bar.md ]
}
