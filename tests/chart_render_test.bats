#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts" "$WORK/data" "$WORK/assets/charts"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/data/demo.csv"
  cat > "$WORK/content/datasets/demo.md" <<'EOF'
---
type: dataset
storage: file
format: csv
rows: 3
data_path: data/demo.csv
---

## Provenance
EOF
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

# `vl-convert` is required for these tests; skip when missing.
need_vlconvert() {
  if ! command -v vl-convert >/dev/null 2>&1; then
    skip "vl-convert binary not installed"
  fi
}

@test "render produces SVG for inline fence" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

# intro

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
```
EOF
  mkdir -p content/concepts
  run bash scripts/chart.sh render
  [ "$status" -eq 0 ]
  [ -f assets/charts/intro-fig0.svg ]
  [ -f assets/charts/intro-fig0.svg.hash ]
}

@test "render is no-op when hash matches (idempotent)" {
  need_vlconvert
  cat > content/charts/demo-bar.md <<'EOF'
---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
```
EOF
  bash scripts/chart.sh render
  before="$(stat -f '%m' assets/charts/demo-bar.svg 2>/dev/null || stat -c '%Y' assets/charts/demo-bar.svg)"
  sleep 1
  bash scripts/chart.sh render
  after="$(stat -f '%m' assets/charts/demo-bar.svg 2>/dev/null || stat -c '%Y' assets/charts/demo-bar.svg)"
  [[ "$before" == "$after" ]]
}

@test "render cleans orphan sidecars" {
  need_vlconvert
  : > assets/charts/orphan-fig0.svg
  : > assets/charts/orphan-fig0.svg.hash
  bash scripts/chart.sh render
  [ ! -f assets/charts/orphan-fig0.svg ]
  [ ! -f assets/charts/orphan-fig0.svg.hash ]
}

@test "render skips with warning when vl-convert missing" {
  # Force vl-convert off by overriding PATH.
  PATH="/usr/bin:/bin" run env -i HOME="$HOME" PATH="/usr/bin:/bin" bash scripts/chart.sh render
  [[ "$output" == *"vl-convert"*"missing"* ]]
}

@test "render fails fast on broken resolver" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[no-such-slug]]"}}
```
EOF
  mkdir -p content/concepts
  run bash scripts/chart.sh render
  [ "$status" -ne 0 ]
  [[ "$output" == *"RESOLVE|"*"no-such-slug"* ]]
}

@test "render-one re-renders only the matching chart-id" {
  need_vlconvert
  mkdir -p content/concepts
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
```
EOF
  cat > content/concepts/other.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"line","data":{"name":"[[demo]]"}}
```
EOF
  bash scripts/chart.sh render
  rm assets/charts/intro-fig0.svg
  bash scripts/chart.sh render-one intro-fig0
  [ -f assets/charts/intro-fig0.svg ]
}

@test "render-one fails when chart-id unknown" {
  need_vlconvert
  run bash scripts/chart.sh render-one nope
  [ "$status" -ne 0 ]
  [[ "$output" == *"unknown chart"* ]]
}
