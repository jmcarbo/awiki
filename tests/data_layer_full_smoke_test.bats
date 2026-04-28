#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  command -v vl-convert >/dev/null 2>&1 || skip "vl-convert missing"
  command -v hugo >/dev/null 2>&1 || skip "hugo missing"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/layouts" "$WORK/layouts"
  cp -r "$REPO_ROOT/themes" "$WORK/themes"
  cp -r "$REPO_ROOT/static" "$WORK/static" 2>/dev/null || mkdir -p "$WORK/static"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/hugo.toml" "$WORK/hugo.toml"
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" "$WORK/demo.csv"
  mkdir -p "$WORK/content"
  printf -- "---\ntitle: WIKI\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "full smoke: data-init -> dataset-new -> chart-new -> charts-render -> build -> lint" {
  bash scripts/data-init.sh
  [ -d content/datasets ]
  [ -d content/charts ]
  [ -d static/vendor/vega ]

  bash scripts/dataset.sh new demo --format=csv --from=demo.csv
  [ -f content/datasets/demo.md ]

  bash scripts/chart.sh new demo-bar --data=demo
  [ -f content/charts/demo-bar.md ]

  bash scripts/chart.sh render
  [ -f assets/charts/demo-bar.svg ]
  [ -f assets/charts/demo-bar.json ]

  bash scripts/build.sh
  [ -d public ]
  out="$(find public -name 'demo-bar*' -type f -name '*.html' | head -1)"
  [[ -n "$out" ]]
  run grep -F 'class="vega-embed"' "$out"
  [ "$status" -eq 0 ]

  run bash scripts/lint.sh
  # No errors expected; warnings (e.g. D9 missing sources) are OK.
  [[ "$output" != *"|error|"* ]]
}
