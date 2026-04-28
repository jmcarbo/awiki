#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  mkdir -p "$WORK/content/concepts" "$WORK/content/datasets" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
  cat > content/concepts/cashflow.md <<'MD'
---
title: cashflow
type: concept
---

# cashflow

Some prose.

```sql awiki-query id="cf-by-cat"
SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1
```

More prose.
MD
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "fence-render populates managed region with markdown table" {
  run bash scripts/query.sh fence-render
  [ "$status" -eq 0 ]
  run grep -F "BEGIN query-result:cf-by-cat" content/concepts/cashflow.md
  [ "$status" -eq 0 ]
  run grep -F "END query-result:cf-by-cat" content/concepts/cashflow.md
  [ "$status" -eq 0 ]
  run grep -F "| category | total |" content/concepts/cashflow.md
  [ "$status" -eq 0 ]
  [ -f content/concepts/cashflow.queries.json ]
}

@test "fence-render is byte-identical on second run" {
  bash scripts/query.sh fence-render
  cp content/concepts/cashflow.md /tmp/cashflow-1.md
  bash scripts/query.sh fence-render
  diff /tmp/cashflow-1.md content/concepts/cashflow.md
}
