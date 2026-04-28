#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  command -v hugo >/dev/null 2>&1 || skip "hugo missing"
  command -v vl-convert >/dev/null 2>&1 || skip "vl-convert missing"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/layouts" "$WORK/layouts"
  cp -r "$REPO_ROOT/static" "$WORK/static" 2>/dev/null || mkdir -p "$WORK/static"
  cp -r "$REPO_ROOT/themes" "$WORK/themes"
  cp "$REPO_ROOT/hugo.toml" "$WORK/hugo.toml"
  mkdir -p "$WORK/content/datasets" "$WORK/content/concepts" "$WORK/data" "$WORK/.awiki/build-content"
  printf 'AWIKI_DATA_LAYER=on\n' > "$WORK/.awiki/config"
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
  cat > "$WORK/content/concepts/intro.md" <<'EOF'
---
title: "intro"
type: concept
---

# intro

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"},"encoding":{"x":{"field":"year"}}}
```
EOF
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "hugo build emits vega-embed div for vega-lite fence" {
  bash scripts/build.sh --full
  [ -f public/concepts/intro/index.html ] || [ -f public/concepts/intro.html ]
  out="$(find public -path '*intro*' -name '*.html' -type f | head -1)"
  run grep -E 'class="?vega-embed' "$out"
  [ "$status" -eq 0 ]
  run grep -F '"url":' "$out"
  [ "$status" -eq 0 ]
}

@test "hugo build injects vega-embed.min.js only on chart pages" {
  bash scripts/build.sh --full
  out="$(find public -path '*intro*' -name '*.html' -type f | head -1)"
  run grep -F 'vega-embed.min.js' "$out"
  [ "$status" -eq 0 ]
  # A page without charts must NOT get the script.
  cat > content/concepts/plain.md <<'EOF'
---
type: concept
---
plain page
EOF
  bash scripts/build.sh --full
  out="$(find public -path '*plain*' -name '*.html' -type f | head -1)"
  run grep -F 'vega-embed.min.js' "$out"
  [ "$status" -ne 0 ]
}

@test "shortcode embeds chart-page spec on another page" {
  mkdir -p content/charts
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
  cat > content/concepts/uses.md <<'EOF'
---
type: concept
title: "uses"
---

# uses

{{< vega-lite "demo-bar" >}}
EOF
  bash scripts/build.sh --full
  out="$(find public -path '*uses*' -name '*.html' -type f | head -1)"
  run grep -E 'class="?vega-embed' "$out"
  [ "$status" -eq 0 ]
  run grep -E 'id="?demo-bar"?' "$out"
  [ "$status" -eq 0 ]
}
