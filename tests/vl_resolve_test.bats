#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content/datasets" "$WORK/data" "$WORK/.awiki"
  pushd "$WORK" >/dev/null
  # Build a file-storage dataset.
  cp "$REPO_ROOT/tests/fixtures/data-layer/demo.csv" data/demo.csv
  cat > content/datasets/demo.md <<'EOF'
---
title: "demo"
type: dataset
storage: file
format: csv
rows: 3
data_path: data/demo.csv
---

## Provenance
EOF
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "vl-resolve expands [[slug]] to data.url for file storage" {
  cp "$REPO_ROOT/tests/fixtures/data-layer/charts/inline-bar.json" spec.json
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"url":'*'/data/demo.csv'* ]]
  [[ "$output" != *'[[demo]]'* ]]
  [[ "$output" == *'"format"'*'"csv"'* ]]
}

@test "vl-resolve embeds values for inline storage" {
  cat > content/datasets/inline.md <<'EOF'
---
title: "inline"
type: dataset
storage: inline
format: csv
rows: 2
---

## Data

```csv
year,pop
2020,331
2021,333
```
EOF
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"name":"[[inline]]"},"encoding":{"x":{"field":"year"}}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"values"'* ]]
  [[ "$output" == *'"2020"'* ]]
}

@test "vl-resolve passes through raw data.url" {
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"url":"/some/path.csv"},"encoding":{}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"/some/path.csv"'* ]]
}

@test "vl-resolve passes through inline data.values" {
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"values":[{"a":1}]}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  [[ "$output" == *'"values"'* ]]
  [[ "$output" == *'"a"'* ]]
}

@test "vl-resolve fails on unknown slug" {
  cp "$REPO_ROOT/tests/fixtures/data-layer/charts/broken-resolver.json" spec.json
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -ne 0 ]
  [[ "$output" == *"RESOLVE|"*"no-such-slug"*"not_found"* ]]
}

@test "vl-resolve fails when slug points to non-dataset" {
  cat > content/datasets/notdata.md <<'EOF'
---
type: concept
---
EOF
  cat > spec.json <<'EOF'
{"mark":"bar","data":{"name":"[[notdata]]"}}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -ne 0 ]
  [[ "$output" == *"not_a_dataset"* ]]
}

@test "vl-resolve recurses into layer + hconcat + vconcat" {
  cat > spec.json <<'EOF'
{
  "hconcat": [
    {"mark":"bar","data":{"name":"[[demo]]"}},
    {"layer":[{"mark":"line","data":{"name":"[[demo]]"}}]}
  ]
}
EOF
  run python3 scripts/lib/vl-resolve.py --spec=spec.json --base-url=/
  [ "$status" -eq 0 ]
  # Two resolutions => two urls in output.
  count="$(grep -c '/data/demo.csv' <<<"$output")"
  [[ "$count" == "2" ]]
}
