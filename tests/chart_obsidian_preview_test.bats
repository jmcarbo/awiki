#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/content/charts" "$WORK/content/concepts" "$WORK/data" "$WORK/assets/charts" "$WORK/.awiki"
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
  printf 'AWIKI_CHART_OBSIDIAN_PREVIEW=on\n' > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }
need_vlconvert() {
  command -v vl-convert >/dev/null 2>&1 || skip "vl-convert missing"
}

@test "render injects chart-preview managed region below fence" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

# intro

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
```

paragraph after.
EOF
  bash scripts/chart.sh render
  run grep -F '<!-- BEGIN chart-preview:intro-fig0 -->' content/concepts/intro.md
  [ "$status" -eq 0 ]
  run grep -F '<!-- END chart-preview:intro-fig0 -->' content/concepts/intro.md
  [ "$status" -eq 0 ]
  run grep -F 'assets/charts/intro-fig0.svg' content/concepts/intro.md
  [ "$status" -eq 0 ]
  # Region must come AFTER the closing fence and BEFORE the next paragraph.
  run grep -B0 -A2 -F '<!-- BEGIN chart-preview' content/concepts/intro.md
  [[ "$output" == *"![intro-fig0]"* ]]
}

@test "AWIKI_CHART_OBSIDIAN_PREVIEW=off leaves managed region empty" {
  need_vlconvert
  printf 'AWIKI_CHART_OBSIDIAN_PREVIEW=off\n' > .awiki/config
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
```
EOF
  bash scripts/chart.sh render
  run grep -F '<!-- BEGIN chart-preview:intro-fig0 -->' content/concepts/intro.md
  [ "$status" -eq 0 ]
  # Body between markers should be empty.
  run python3 - <<'PY'
import re, sys
txt = open("content/concepts/intro.md").read()
m = re.search(r"<!-- BEGIN chart-preview:intro-fig0 -->\n(.*?)\n<!-- END chart-preview:intro-fig0 -->", txt, re.S)
assert m, "no markers"
body = m.group(1).strip()
assert body == "", f"expected empty, got {body!r}"
print("ok")
PY
  [[ "$output" == "ok" ]]
}

@test "render is idempotent on managed region (no duplicate insertion)" {
  need_vlconvert
  cat > content/concepts/intro.md <<'EOF'
---
type: concept
---

```vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
```
EOF
  bash scripts/chart.sh render
  bash scripts/chart.sh render
  run grep -c -F '<!-- BEGIN chart-preview:intro-fig0 -->' content/concepts/intro.md
  [[ "$output" == "1" ]]
}
