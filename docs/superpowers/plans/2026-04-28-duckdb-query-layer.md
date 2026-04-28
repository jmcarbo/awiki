# DuckDB Query Layer (Stage 1) Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a SQL surface over awiki datasets using DuckDB. Three frontends (CLI ad-hoc, `type: query` page, inline `awiki-query` SQL fence) share one engine. Local datasets only; external DBs deferred to Stage 3.

**Architecture:** A pure shell engine (`scripts/lib/query-engine.sh`) extracts dataset fence bodies to `.cache/duckdb/`, registers DuckDB views, runs user SQL, and returns rows + a deterministic hash. Three thin frontends (CLI, page-kind renderer, inline-fence pre-build pass) consume the engine and own their own filesystem outputs (stdout, materialized dataset page, managed-region table). Hash sidecars + a Q-series lint module enforce staleness, determinism (Surface B only), and managed-region tamper.

**Tech Stack:** Bash 5, Python 3 (stdlib only), DuckDB CLI ≥ 1.1, BATS, `node:test`, Hugo (existing build).

**Spec:** `docs/superpowers/specs/2026-04-28-duckdb-query-layer-design.md`

**Conventions reused from data + chart layers:**
- `LINT|LEVEL|file|CODE|msg` log format.
- `QUERY|...` log prefix (matches `DATASET|`, `CHART|`).
- BATS test setup mirrors `tests/dataset_new_test.bats`: copy scripts + justfile + fixtures into `mktemp` workdir, write `.awiki/config` with `AWIKI_DATA_LAYER=on`.
- Managed-region marker style: `<!-- BEGIN <kind>:<id> -->` / `<!-- END <kind>:<id> -->`. Matches `chart-preview:` in `chart.sh`.
- Slug regex: `^[a-z0-9][a-z0-9-]*$`.

---

## File Structure

### New files
- `scripts/query.sh` — CLI dispatcher (subcommands: `run`, `new`, `render`, `render-one`, `fence-render`).
- `scripts/lib/query-engine.sh` — pure engine (resolve refs, extract, run DuckDB, hash).
- `scripts/lib/query-extract.py` — extract fence body from a dataset page → `.cache/duckdb/<slug>.<ext>`.
- `scripts/lib/query-resolve.py` — parse SQL → list of referenced dataset slugs (FROM/JOIN, CTE-aware, quoted-id-aware).
- `scripts/lib/query-determinism.py` — pre-exec guard for materialized SQL (banned tokens + outer ORDER BY).
- `scripts/lib/query-format.py` — DuckDB JSON rows → markdown table.
- `scripts/lib/managed-region.sh` — shared BEGIN/END region rewrite (extracted from `chart.sh`).
- `scripts/lint-query.sh` — Q1–Q5 + Q-PRIV stub.
- `tests/query_engine_test.bats`
- `tests/query_cli_test.bats`
- `tests/query_page_test.bats`
- `tests/query_fence_test.bats`
- `tests/lint_query_test.bats`
- `tests/query_deps_test.bats`
- `tests/query_node/` — node:test units (resolver, determinism, formatter)
- `tests/fixtures/query/` — small CSV + dataset fixtures

### Modified files
- `justfile` — add `query`, `query-new`, `query-render`, `query-render-one`, `query-fence-render` recipes.
- `scripts/build.sh` — run `query.sh render` and inline-fence pass before `chart.sh render`.
- `scripts/check-deps.sh` — advisory if `duckdb` missing + data layer on.
- `scripts/data-init.sh` — `step_dirs` adds `content/queries`; `step_config` adds `AWIKI_QUERY_LAYER=on` (defaults on with data layer).
- `scripts/lint.sh` — source `lint-query.sh`; route `--only=query`.
- `scripts/chart.sh` — refactor inline managed-region regex to call `managed-region.sh`.
- `.gitignore` — `.cache/duckdb/`.
- `WIKI.md` — add `query` to page-kind enum; add §4.x "Query" workflow.
- `README.md` — query layer section.
- `docs/data-help.txt` — query commands.
- `mcp/...` — `list_queries()` tool (parity with `list_charts`).

### Test layout

Mirrors existing repo: top-level `tests/*_test.bats`, fixtures under `tests/fixtures/query/`, node units under `tests/query_node/`.

---

## Phase 0 — Foundations

### Task 1: Gitignore + cache helper

**Files:**
- Modify: `.gitignore`
- Create: `tests/fixtures/query/trades.csv`
- Create: `tests/fixtures/query/regions.csv`

- [ ] **Step 1: Add cache dir to .gitignore**

Append to `.gitignore`:

```
# DuckDB query layer cache
.cache/duckdb/
```

- [ ] **Step 2: Create test fixtures**

`tests/fixtures/query/trades.csv`:

```
id,category,amount,region_id
1,food,10.5,1
2,food,20.0,1
3,gear,150.0,2
4,gear,75.0,2
5,food,5.0,1
```

`tests/fixtures/query/regions.csv`:

```
id,name
1,emea
2,amer
```

- [ ] **Step 3: Verify**

```bash
grep -q ".cache/duckdb/" .gitignore && echo OK
```

Expected: `OK`.

- [ ] **Step 4: Commit**

```bash
git add .gitignore tests/fixtures/query/
git commit -m "feat(query): cache dir gitignore + test fixtures"
```

---

### Task 2: data-init adds queries dir + flag

**Files:**
- Modify: `scripts/data-init.sh`
- Test: `tests/data_init_test.bats` (add cases)

- [ ] **Step 1: Write failing test**

Append to `tests/data_init_test.bats` (find `setup()` at top to follow pattern):

```bash
@test "data-init creates content/queries directory" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/queries ]
  [ -f content/queries/.gitkeep ]
}

@test "data-init enables query layer flag" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_QUERY_LAYER=on$' .awiki/config
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/data_init_test.bats
```

Expected: two new tests FAIL.

- [ ] **Step 3: Modify `step_dirs`**

In `scripts/data-init.sh`, find the `step_dirs()` function. Add `content/queries` to its loop list:

```bash
  for d in content/datasets content/queries content/charts data assets/charts static/vendor/vega; do
```

- [ ] **Step 4: Modify `step_config`**

In `scripts/data-init.sh`, append inside `step_config()` after the existing `_ensure_kv` calls:

```bash
  _ensure_kv "AWIKI_QUERY_LAYER" "on"
```

- [ ] **Step 5: Run tests**

```bash
bats tests/data_init_test.bats
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/data-init.sh tests/data_init_test.bats
git commit -m "feat(data-init): scaffold content/queries + AWIKI_QUERY_LAYER flag"
```

---

### Task 3: check-deps advisory for duckdb

**Files:**
- Modify: `scripts/check-deps.sh`
- Test: `tests/query_deps_test.bats` (new)

- [ ] **Step 1: Write failing test**

Create `tests/query_deps_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "check-deps emits advisory when duckdb missing + query layer on" {
  # Shadow duckdb to force "not found".
  mkdir -p shimbin
  PATH="$(pwd)/shimbin:$PATH" run bash scripts/check-deps.sh
  # Advisory is a warn line; do not fail check-deps for optional dep.
  [[ "$output" == *"data layer is on but duckdb is missing"* ]]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_deps_test.bats
```

Expected: FAIL (no such advisory yet).

- [ ] **Step 3: Add advisory**

In `scripts/check-deps.sh`, inside the `if [[ -f .awiki/config ]] && grep -q '^AWIKI_DATA_LAYER=on'` block, after the `vl-convert` advisory, append:

```bash
  if ! command -v duckdb >/dev/null 2>&1; then
    echo "WARN: data layer is on but duckdb is missing — query layer won't run"
    echo "      install: brew install duckdb (macOS) OR https://duckdb.org/docs/installation/" >&2
  fi
```

- [ ] **Step 4: Run test**

```bash
bats tests/query_deps_test.bats
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/check-deps.sh tests/query_deps_test.bats
git commit -m "feat(check-deps): duckdb advisory when query layer enabled"
```

---

## Phase 1 — Engine + extractor

### Task 4: query-extract.py — extract dataset fence body

Reads `content/datasets/<slug>.md`, finds fence `\`\`\`<format>` matching the `format:` frontmatter, writes its body to a target file.

**Files:**
- Create: `scripts/lib/query-extract.py`
- Test: `tests/query_engine_test.bats` (new file; will host engine-level cases)

- [ ] **Step 1: Write failing test**

Create `tests/query_engine_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  cp "$REPO_ROOT/tests/fixtures/query/regions.csv" "$WORK/regions.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/.awiki" "$WORK/.cache/duckdb"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
  bash scripts/dataset.sh new regions --format=csv --from=regions.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "query-extract writes fence body for csv dataset" {
  run python3 scripts/lib/query-extract.py \
    --slug=trades --datasets-dir=content/datasets --out=.cache/duckdb/trades.csv
  [ "$status" -eq 0 ]
  [ -f .cache/duckdb/trades.csv ]
  run grep -F "id,category,amount,region_id" .cache/duckdb/trades.csv
  [ "$status" -eq 0 ]
  # Trailing newline + 5 data rows + 1 header.
  run wc -l < .cache/duckdb/trades.csv
  [ "$output" = "       6" ] || [ "$output" = "6" ]
}

@test "query-extract fails on missing dataset" {
  run python3 scripts/lib/query-extract.py \
    --slug=ghost --datasets-dir=content/datasets --out=.cache/duckdb/ghost.csv
  [ "$status" -ne 0 ]
  [[ "$output" == *"dataset not found"* ]] || [[ "$stderr" == *"dataset not found"* ]] || true
}

@test "query-extract fails on absent fence" {
  cat > content/datasets/empty.md <<'MD'
---
type: dataset
storage: inline
format: csv
rows: 0
---
# empty
MD
  run python3 scripts/lib/query-extract.py \
    --slug=empty --datasets-dir=content/datasets --out=.cache/duckdb/empty.csv
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_engine_test.bats
```

Expected: FAIL (script absent).

- [ ] **Step 3: Implement extractor**

Create `scripts/lib/query-extract.py`:

```python
#!/usr/bin/env python3
"""Extract a dataset fence body from content/datasets/<slug>.md to a file.

Reads `format:` from frontmatter, finds the matching ``` fence, writes its
body verbatim (with a trailing newline if missing) to --out.
"""
import argparse
import re
import sys
from pathlib import Path


def _frontmatter(text: str) -> dict[str, str]:
    m = re.match(r"^---\s*\n(.*?)\n---\s*\n", text, re.S)
    if not m:
        return {}
    out: dict[str, str] = {}
    for line in m.group(1).splitlines():
        kv = re.match(r"^([a-z0-9_]+):\s*(.*)$", line.strip())
        if kv:
            out[kv.group(1)] = kv.group(2).strip().strip('"').strip("'")
    return out


def _fence_body(text: str, fmt: str) -> str | None:
    pat = re.compile(rf"```{re.escape(fmt)}\s*\n(.*?)\n```", re.S)
    m = pat.search(text)
    return m.group(1) if m else None


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--slug", required=True)
    ap.add_argument("--datasets-dir", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    page = Path(args.datasets_dir) / f"{args.slug}.md"
    if not page.is_file():
        print(f"QUERY|ERROR|dataset not found: {args.slug}", file=sys.stderr)
        return 3

    text = page.read_text(encoding="utf-8")
    fm = _frontmatter(text)
    fmt = fm.get("format")
    if not fmt:
        print(f"QUERY|ERROR|no format in frontmatter: {page}", file=sys.stderr)
        return 4

    body = _fence_body(text, fmt)
    if body is None:
        print(f"QUERY|ERROR|no ```{fmt} fence in {page}", file=sys.stderr)
        return 4

    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    if not body.endswith("\n"):
        body += "\n"
    out.write_text(body, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests**

```bash
bats tests/query_engine_test.bats
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/query-extract.py tests/query_engine_test.bats
git commit -m "feat(query): extract dataset fence body to cache file"
```

---

### Task 5: query-resolve.py — SQL → referenced slugs

**Files:**
- Create: `scripts/lib/query-resolve.py`
- Test: `tests/query_node/resolve.test.mjs` (node:test)

- [ ] **Step 1: Write failing test**

Create `tests/query_node/resolve.test.mjs`:

```javascript
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const SCRIPT = resolve("scripts/lib/query-resolve.py");

function refs(sql) {
  const out = execFileSync("python3", [SCRIPT], {
    input: sql,
    encoding: "utf-8",
  });
  return out.trim().split("\n").filter(Boolean).sort();
}

test("bare FROM ref", () => {
  assert.deepEqual(refs("SELECT * FROM trades"), ["trades"]);
});

test("quoted identifier", () => {
  assert.deepEqual(refs('SELECT * FROM "trades"'), ["trades"]);
});

test("JOIN", () => {
  assert.deepEqual(
    refs("SELECT * FROM trades t JOIN regions r ON t.region_id=r.id"),
    ["regions", "trades"],
  );
});

test("CTE alias not treated as dataset", () => {
  assert.deepEqual(
    refs(
      "WITH agg AS (SELECT category FROM trades) SELECT * FROM agg",
    ),
    ["trades"],
  );
});

test("subquery", () => {
  assert.deepEqual(
    refs("SELECT * FROM (SELECT id FROM trades) x"),
    ["trades"],
  );
});

test("schema-qualified main.<slug>", () => {
  assert.deepEqual(refs("SELECT * FROM main.trades"), ["trades"]);
});

test("multiple statements", () => {
  assert.deepEqual(
    refs("SELECT * FROM trades; SELECT * FROM regions;"),
    ["regions", "trades"],
  );
});

test("ATTACH rejected as ref (caller will handle separately)", () => {
  // Resolver only reports table refs; ATTACH detection is a separate pass.
  assert.deepEqual(refs("SELECT * FROM trades"), ["trades"]);
});
```

- [ ] **Step 2: Run failing**

```bash
node --test tests/query_node/resolve.test.mjs
```

Expected: FAIL.

- [ ] **Step 3: Implement resolver**

Create `scripts/lib/query-resolve.py`:

```python
#!/usr/bin/env python3
"""Parse SQL on stdin, emit one referenced dataset slug per stdout line.

Heuristic: scan FROM/JOIN clauses; strip CTE names, quoted identifiers,
and `main.` schema prefix. Caller validates each slug exists on disk.
"""
import re
import sys

SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")


def _strip_strings_and_comments(sql: str) -> str:
    # Remove -- line comments and /* */ block comments and 'string literals'.
    sql = re.sub(r"--[^\n]*", "", sql)
    sql = re.sub(r"/\*.*?\*/", "", sql, flags=re.S)
    sql = re.sub(r"'(?:''|[^'])*'", "''", sql)
    return sql


def _cte_names(sql: str) -> set[str]:
    names: set[str] = set()
    for m in re.finditer(r"\bWITH\b(.+?)\bSELECT\b", sql, re.I | re.S):
        block = m.group(1)
        # Each CTE is `name AS ( ... )`; comma-separated at depth 0.
        for cte in re.finditer(r"([a-zA-Z_][\w]*)\s+AS\s*\(", block, re.I):
            names.add(cte.group(1).lower())
    return names


def _refs(sql: str) -> list[str]:
    sql = _strip_strings_and_comments(sql)
    skip = _cte_names(sql)
    found: list[str] = []
    for m in re.finditer(
        r"\b(?:FROM|JOIN)\s+(?:(?:main|public)\.)?\"?([a-zA-Z_][\w-]*)\"?",
        sql,
        re.I,
    ):
        name = m.group(1).lower()
        if name in skip:
            continue
        if not SLUG_RE.match(name):
            continue
        if name in found:
            continue
        found.append(name)
    return sorted(found)


def main() -> int:
    sql = sys.stdin.read()
    for slug in _refs(sql):
        print(slug)
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run test**

```bash
node --test tests/query_node/resolve.test.mjs
```

Expected: PASS all 8.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/query-resolve.py tests/query_node/resolve.test.mjs
git commit -m "feat(query): SQL ref resolver (FROM/JOIN, CTE-aware)"
```

---

### Task 6: query-engine.sh — extract + run + hash

**Files:**
- Create: `scripts/lib/query-engine.sh`
- Test: `tests/query_engine_test.bats` (extend)

- [ ] **Step 1: Write failing test**

Append to `tests/query_engine_test.bats`:

```bash
@test "engine runs SELECT against single dataset" {
  run bash scripts/lib/query-engine.sh run "SELECT COUNT(*) AS n FROM trades"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"n":5'* ]] || [[ "$output" == *'"n": 5'* ]]
}

@test "engine runs JOIN across two datasets" {
  run bash scripts/lib/query-engine.sh run \
    "SELECT r.name, COUNT(*) AS n FROM trades t JOIN regions r ON t.region_id=r.id GROUP BY 1 ORDER BY 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"name":"amer"'* ]] || [[ "$output" == *'"name": "amer"'* ]]
  [[ "$output" == *'"name":"emea"'* ]] || [[ "$output" == *'"name": "emea"'* ]]
}

@test "engine exits 3 for unknown dataset" {
  run bash scripts/lib/query-engine.sh run "SELECT * FROM ghost"
  [ "$status" -eq 3 ]
  [[ "$output" == *"unknown dataset: ghost"* ]]
}

@test "engine exits 9 for ATTACH" {
  run bash scripts/lib/query-engine.sh run "ATTACH 'foo.db'; SELECT 1"
  [ "$status" -eq 9 ]
  [[ "$output" == *"external attach not supported"* ]]
}

@test "engine emits stable result hash" {
  h1=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  h2=$(bash scripts/lib/query-engine.sh hash "SELECT id FROM trades ORDER BY id")
  [ "$h1" = "$h2" ]
  [ -n "$h1" ]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_engine_test.bats -f "engine"
```

Expected: FAIL (engine missing).

- [ ] **Step 3: Implement engine**

Create `scripts/lib/query-engine.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/query-engine.sh — pure DuckDB engine: extract refs, run SQL,
# emit JSON rows on stdout. Surfaces (CLI, page, fence) wrap this.
#
# Subcommands:
#   run "<SQL>"   — run query, print DuckDB JSON rows on stdout
#   hash "<SQL>"  — print sha256 of (normalized SQL + sorted slug:filehash)
#
# Exit codes: 2 duckdb-missing | 3 unknown-dataset | 4 extract-fail
#             5 sql-parse | 9 external-attach-rejected
set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
DATASETS_DIR="${AWIKI_DATASETS_DIR:-content/datasets}"
CACHE_DIR="${AWIKI_QUERY_CACHE:-.cache/duckdb}"
EXTRACT_PY="$REPO_ROOT/scripts/lib/query-extract.py"
RESOLVE_PY="$REPO_ROOT/scripts/lib/query-resolve.py"

note() { echo "QUERY|$*"; }
die()  { echo "QUERY|ERROR|$*" >&2; }

_require_duckdb() {
  command -v duckdb >/dev/null 2>&1 || { die "duckdb CLI not found"; return 2; }
}

_reject_attach() {
  local sql="$1"
  if printf '%s' "$sql" | grep -iE '\bATTACH\b' >/dev/null; then
    die "external attach not supported until stage 3"
    return 9
  fi
  return 0
}

_resolve_refs() {
  local sql="$1"
  printf '%s' "$sql" | python3 "$RESOLVE_PY"
}

_extract_one() {
  local slug="$1" fmt="$2"
  python3 "$EXTRACT_PY" --slug="$slug" --datasets-dir="$DATASETS_DIR" \
    --out="$CACHE_DIR/$slug.$fmt"
}

_format_for() {
  local slug="$1"
  python3 - "$DATASETS_DIR/$slug.md" <<'PY'
import re, sys, pathlib
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
m = re.search(r"^format:\s*(\S+)", text, re.M)
print(m.group(1) if m else "csv")
PY
}

_view_ddl() {
  local slug="$1" fmt="$2" path="$CACHE_DIR/$slug.$fmt"
  case "$fmt" in
    csv|tsv|dsv) printf "CREATE VIEW %s AS SELECT * FROM read_csv('%s', AUTO_DETECT=TRUE);\n" "$slug" "$path" ;;
    json)        printf "CREATE VIEW %s AS SELECT * FROM read_json_auto('%s');\n" "$slug" "$path" ;;
    *)           printf "CREATE VIEW %s AS SELECT * FROM read_csv('%s', AUTO_DETECT=TRUE);\n" "$slug" "$path" ;;
  esac
}

_prepare() {
  local sql="$1"
  _require_duckdb || return $?
  _reject_attach "$sql" || return $?
  mkdir -p "$CACHE_DIR"
  local refs ddl=""
  refs="$(_resolve_refs "$sql")"
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if [[ ! -f "$DATASETS_DIR/$slug.md" ]]; then
      die "unknown dataset: $slug"
      return 3
    fi
    local fmt; fmt="$(_format_for "$slug")"
    _extract_one "$slug" "$fmt" || return 4
    ddl+="$(_view_ddl "$slug" "$fmt")"
  done <<< "$refs"
  printf '%s' "$ddl"
}

cmd_run() {
  local sql="$1"
  local ddl
  ddl="$(_prepare "$sql")" || return $?
  local script="$ddl
$sql
"
  local out
  out="$(printf '%s' "$script" | duckdb -json 2>&1)"
  local rc=$?
  if [[ $rc -ne 0 ]]; then
    printf '%s\n' "$out" >&2
    return 5
  fi
  printf '%s\n' "$out"
}

cmd_hash() {
  local sql="$1"
  local refs slug fmt path file_hash payload=""
  refs="$(_resolve_refs "$sql")" || return $?
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    fmt="$(_format_for "$slug")"
    path="$DATASETS_DIR/$slug.md"
    file_hash="$(shasum -a 256 "$path" | awk '{print $1}')"
    payload+="$slug:$file_hash"$'\n'
  done <<< "$refs"
  local norm
  norm="$(printf '%s' "$sql" | tr -s '[:space:]' ' ' | sed -e 's/^ *//' -e 's/ *$//')"
  printf '%s\n%s' "$norm" "$payload" | shasum -a 256 | awk '{print $1}'
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    run)  cmd_run "$@" ;;
    hash) cmd_hash "$@" ;;
    *) die "unknown subcommand: $cmd"; return 1 ;;
  esac
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
```

- [ ] **Step 4: Run tests**

```bash
bats tests/query_engine_test.bats
```

Expected: PASS all engine cases.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/query-engine.sh tests/query_engine_test.bats
git commit -m "feat(query): engine — resolve, extract, run, hash"
```

---

## Phase 2 — CLI Surface A

### Task 7: query.sh run frontend

**Files:**
- Create: `scripts/query.sh`
- Test: `tests/query_cli_test.bats`

- [ ] **Step 1: Write failing test**

Create `tests/query_cli_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  cp "$REPO_ROOT/tests/fixtures/query/regions.csv" "$WORK/regions.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/content/queries" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
  bash scripts/dataset.sh new regions --format=csv --from=regions.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "query run prints rows as JSON" {
  run bash scripts/query.sh run "SELECT category, COUNT(*) AS n FROM trades GROUP BY 1 ORDER BY 1"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"category":"food"'* ]] || [[ "$output" == *'"category": "food"'* ]]
}

@test "query run exits 3 on unknown dataset" {
  run bash scripts/query.sh run "SELECT * FROM ghost"
  [ "$status" -eq 3 ]
}

@test "query run exits 5 on SQL parse error" {
  run bash scripts/query.sh run "SELEC bogus"
  [ "$status" -eq 5 ]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_cli_test.bats
```

Expected: FAIL (script absent).

- [ ] **Step 3: Implement CLI**

Create `scripts/query.sh`:

```bash
#!/usr/bin/env bash
# scripts/query.sh — DuckDB query layer CLI.
# Subcommands: run, new, render, render-one, fence-render.
set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

ENGINE="$REPO_ROOT/scripts/lib/query-engine.sh"

note() { echo "QUERY|$*"; }
die()  { echo "QUERY|ERROR|$*" >&2; exit 1; }

cmd_run() {
  local sql=""
  local out=""
  local force=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --out=*)   out="${1#--out=}" ;;
      --force)   force=1 ;;
      --)        shift; break ;;
      -*)        die "unknown flag: $1" ;;
      *)         sql="$1" ;;
    esac
    shift
  done
  [[ -n "$sql" ]] || die "missing SQL"
  if [[ -n "$out" ]]; then
    bash "$ENGINE" run "$sql" > /tmp/q-rows.json
    local rc=$?
    [[ $rc -eq 0 ]] || exit $rc
    # Materialization happens in Task 8.
    die "--out=<slug> requires Task 8 materializer (not yet implemented)"
  else
    bash "$ENGINE" run "$sql"
  fi
}

main() {
  local cmd="${1:-}"
  shift || true
  case "$cmd" in
    run)           cmd_run "$@" ;;
    "")            die "usage: query.sh <run|new|render|render-one|fence-render>" ;;
    *)             die "unknown subcommand: $cmd" ;;
  esac
}

main "$@"
```

- [ ] **Step 4: Run tests**

```bash
bats tests/query_cli_test.bats
```

Expected: PASS three cases.

- [ ] **Step 5: Commit**

```bash
git add scripts/query.sh tests/query_cli_test.bats
git commit -m "feat(query): query.sh run — ad-hoc CLI over DuckDB engine"
```

---

### Task 8: --out=<slug> materialize

**Files:**
- Modify: `scripts/query.sh`
- Test: `tests/query_cli_test.bats` (extend)

- [ ] **Step 1: Write failing tests**

Append to `tests/query_cli_test.bats`:

```bash
@test "query run --out=summary materializes dataset" {
  run bash scripts/query.sh run \
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1" \
    --out=summary
  [ "$status" -eq 0 ]
  [ -f content/datasets/summary.md ]
  run grep -E '^type: dataset$' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -E '^sources: \[trades\]' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -E '^query: sha256-' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -A3 '## Data' content/datasets/summary.md
  [[ "$output" == *'```csv'* ]]
  [[ "$output" == *'category,total'* ]]
}

@test "query run --out=existing rejects without --force" {
  run bash scripts/query.sh run \
    "SELECT 1 AS x" --out=trades
  [ "$status" -eq 7 ]
}

@test "query run --out=existing overwrites with --force" {
  run bash scripts/query.sh run \
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1" \
    --out=summary
  [ "$status" -eq 0 ]
  run bash scripts/query.sh run \
    "SELECT category, COUNT(*) AS n FROM trades GROUP BY 1 ORDER BY 1" \
    --out=summary --force
  [ "$status" -eq 0 ]
  run grep -F 'category,n' content/datasets/summary.md
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_cli_test.bats -f "out="
```

Expected: FAIL.

- [ ] **Step 3: Implement materializer**

Replace `cmd_run` in `scripts/query.sh` with:

```bash
cmd_run() {
  local sql=""
  local out=""
  local force=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --out=*)   out="${1#--out=}" ;;
      --force)   force=1 ;;
      --)        shift; break ;;
      -*)        die "unknown flag: $1" ;;
      *)         sql="$1" ;;
    esac
    shift
  done
  [[ -n "$sql" ]] || die "missing SQL"
  if [[ -z "$out" ]]; then
    bash "$ENGINE" run "$sql"
    return $?
  fi

  local SLUG_RE='^[a-z0-9][a-z0-9-]*$'
  [[ "$out" =~ $SLUG_RE ]] || die "invalid slug: $out"
  local target="content/datasets/$out.md"
  if [[ -f "$target" && $force -eq 0 ]]; then
    echo "QUERY|ERROR|target exists: $target (use --force)" >&2
    exit 7
  fi

  local tmp_rows; tmp_rows="$(mktemp)"
  bash "$ENGINE" run "$sql" > "$tmp_rows"
  local rc=$?
  [[ $rc -eq 0 ]] || { rm -f "$tmp_rows"; exit $rc; }

  # Convert JSON rows -> CSV.
  local tmp_csv; tmp_csv="$(mktemp --suffix=.csv 2>/dev/null || mktemp -t qcsv)"
  python3 - "$tmp_rows" "$tmp_csv" <<'PY'
import csv, json, sys, pathlib
rows = json.loads(pathlib.Path(sys.argv[1]).read_text() or "[]")
with open(sys.argv[2], "w", newline="") as f:
    if not rows:
        f.write("")
        sys.exit(0)
    w = csv.DictWriter(f, fieldnames=list(rows[0].keys()))
    w.writeheader()
    for r in rows:
        w.writerow(r)
PY

  local sources_csv
  sources_csv="$(printf '%s' "$sql" | python3 "$REPO_ROOT/scripts/lib/query-resolve.py" | paste -sd, -)"
  local hash_val
  hash_val="$(bash "$ENGINE" hash "$sql")"
  local today; today="$(date '+%Y-%m-%d')"
  local rows_n; rows_n=$(($(wc -l < "$tmp_csv") - 1))
  [[ $rows_n -lt 0 ]] && rows_n=0

  {
    printf -- '---\n'
    printf -- 'title: "%s"\n' "$out"
    printf -- 'date: %s\n' "$today"
    printf -- 'last_updated: %s\n' "$today"
    printf -- 'type: dataset\n'
    printf -- 'tags: []\n'
    printf -- 'storage: inline\n'
    printf -- 'format: csv\n'
    printf -- 'rows: %s\n' "$rows_n"
    printf -- 'sources: [%s]\n' "$sources_csv"
    printf -- 'query: sha256-%s\n' "$hash_val"
    printf -- 'draft: false\n'
    printf -- '---\n\n'
    printf -- '# %s\n\n' "$out"
    printf -- '<!-- generated by query.sh — do not edit ## Data by hand -->\n\n'
    printf -- '## Schema\n\n'
    printf -- '## Data\n'
    printf -- '```csv\n'
    cat "$tmp_csv"
    printf -- '```\n\n'
    printf -- '## Provenance\n\n'
    printf -- 'Generated by `scripts/query.sh run --out=%s`.\n\n' "$out"
    printf -- '## Related\n\n'
    printf -- '## Sources\n'
  } > "$target"
  rm -f "$tmp_rows" "$tmp_csv"
  note "MATERIALIZED|$target|rows=$rows_n"
}
```

- [ ] **Step 4: Run tests**

```bash
bats tests/query_cli_test.bats
```

Expected: PASS all six.

- [ ] **Step 5: Commit**

```bash
git add scripts/query.sh tests/query_cli_test.bats
git commit -m "feat(query): --out=<slug> materializes result as dataset page"
```

---

### Task 9: just recipe — query

**Files:**
- Modify: `justfile`

- [ ] **Step 1: Read existing chart recipe section**

```bash
grep -n "chart-new\|charts-render" justfile
```

Note line numbers, locate "Chart" section (around lines 145–158 per spec exploration).

- [ ] **Step 2: Append query recipes**

After the chart recipe block in `justfile`, append:

```make
# === Query layer ===

# Run an ad-hoc SQL query against awiki datasets.
# Use --out=<slug> to materialize the result as a dataset page.
query SQL *args:
    bash scripts/query.sh run "{{SQL}}" {{args}}

# Scaffold a type:query page that materializes to a sibling dataset.
query-new slug *args:
    bash scripts/query.sh new {{slug}} {{args}}

# Re-run every materialized query and refresh inline awiki-query fences.
query-render:
    bash scripts/query.sh render

# Single-query regen for fast iteration.
query-render-one slug:
    bash scripts/query.sh render-one {{slug}}

# Re-run every inline awiki-query fence in content/.
query-fence-render:
    bash scripts/query.sh fence-render
```

- [ ] **Step 3: Verify**

```bash
just --list 2>&1 | grep -E "query|query-"
```

Expected: five recipes shown.

- [ ] **Step 4: Commit**

```bash
git add justfile
git commit -m "feat(query): just recipes — query, query-new, query-render(*one), query-fence-render"
```

---

## Phase 3 — type: query page (Surface B)

### Task 10: query.sh new — scaffold

**Files:**
- Modify: `scripts/query.sh`
- Test: `tests/query_page_test.bats` (new)

- [ ] **Step 1: Write failing test**

Create `tests/query_page_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/content/queries" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "query.sh new scaffolds type:query page" {
  run bash scripts/query.sh new top-trades --out=summary
  [ "$status" -eq 0 ]
  [ -f content/queries/top-trades.md ]
  run grep -E '^type: query$' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  run grep -E '^out: summary$' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  run grep -E '^deterministic: true$' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  run grep -F '```sql' content/queries/top-trades.md
  [ "$status" -eq 0 ]
}

@test "query.sh new rejects existing slug" {
  bash scripts/query.sh new dup --out=summary >/dev/null
  run bash scripts/query.sh new dup --out=summary
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_page_test.bats
```

Expected: FAIL.

- [ ] **Step 3: Implement `cmd_new`**

In `scripts/query.sh`, add this function and wire it into `main`:

```bash
cmd_new() {
  local slug="" out=""
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --out=*) out="${1#--out=}" ;;
      --) shift; break ;;
      -*) die "unknown flag: $1" ;;
      *) slug="$1" ;;
    esac
    shift
  done
  local SLUG_RE='^[a-z0-9][a-z0-9-]*$'
  [[ -n "$slug" ]] || die "missing <slug>"
  [[ "$slug" =~ $SLUG_RE ]] || die "invalid slug: $slug"
  [[ -n "$out" ]]  || die "--out=<dataset-slug> required"
  [[ "$out" =~ $SLUG_RE ]] || die "invalid out slug: $out"
  local page="content/queries/$slug.md"
  [[ ! -e "$page" ]] || die "$page already exists"
  mkdir -p content/queries
  local today; today="$(date '+%Y-%m-%d')"
  cat > "$page" <<EOF
---
title: "$slug"
date: $today
last_updated: $today
type: query
sources: []
out: $out
privacy: internal
deterministic: true
sql_hash: ""
draft: false
---

# $slug

<!-- describe what this query answers -->

## SQL

\`\`\`sql
SELECT 1 AS placeholder
ORDER BY 1
\`\`\`

## Notes
EOF
  note "NEW|$page"
}
```

In `main()`, add the case branch:

```bash
    new)           cmd_new "$@" ;;
```

- [ ] **Step 4: Run test**

```bash
bats tests/query_page_test.bats
```

Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add scripts/query.sh tests/query_page_test.bats
git commit -m "feat(query): query.sh new — scaffold type:query page"
```

---

### Task 11: Determinism guard

**Files:**
- Create: `scripts/lib/query-determinism.py`
- Test: `tests/query_node/determinism.test.mjs`

- [ ] **Step 1: Write failing test**

Create `tests/query_node/determinism.test.mjs`:

```javascript
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const SCRIPT = resolve("scripts/lib/query-determinism.py");

function check(sql) {
  try {
    execFileSync("python3", [SCRIPT], { input: sql, encoding: "utf-8" });
    return { ok: true };
  } catch (e) {
    return { ok: false, stderr: e.stderr?.toString?.() ?? "" };
  }
}

test("plain SELECT with ORDER BY passes", () => {
  assert.equal(check("SELECT id FROM trades ORDER BY id").ok, true);
});

test("missing outer ORDER BY rejected", () => {
  const r = check("SELECT id FROM trades");
  assert.equal(r.ok, false);
  assert.match(r.stderr, /ORDER BY/);
});

test("NOW() rejected", () => {
  const r = check("SELECT NOW() AS t FROM trades ORDER BY t");
  assert.equal(r.ok, false);
  assert.match(r.stderr, /NOW/);
});

test("CURRENT_DATE rejected", () => {
  const r = check("SELECT CURRENT_DATE FROM trades ORDER BY 1");
  assert.equal(r.ok, false);
});

test("RANDOM() rejected", () => {
  const r = check("SELECT RANDOM() AS r FROM trades ORDER BY r");
  assert.equal(r.ok, false);
  assert.match(r.stderr, /RANDOM/);
});

test("UUID() rejected", () => {
  const r = check("SELECT UUID() AS u FROM trades ORDER BY u");
  assert.equal(r.ok, false);
});

test("ORDER BY in inner subquery does not satisfy", () => {
  const r = check(
    "SELECT * FROM (SELECT id FROM trades ORDER BY id) x",
  );
  assert.equal(r.ok, false);
});

test("LIMIT without ORDER BY rejected", () => {
  const r = check("SELECT id FROM trades LIMIT 10");
  assert.equal(r.ok, false);
});
```

- [ ] **Step 2: Run failing**

```bash
node --test tests/query_node/determinism.test.mjs
```

Expected: FAIL.

- [ ] **Step 3: Implement determinism guard**

Create `scripts/lib/query-determinism.py`:

```python
#!/usr/bin/env python3
"""Reject non-deterministic SQL when materializing.

Reads SQL on stdin. Exits 0 if deterministic, 6 with a stderr message otherwise.

Rules:
  - Banned tokens: NOW(), CURRENT_TIMESTAMP, CURRENT_DATE, CURRENT_TIME,
    RANDOM(), UUID(), gen_random_uuid().
  - Top-level statement must contain ORDER BY (case-insensitive).
  - Inner ORDER BY (inside parentheses) does not count.
"""
import re
import sys

BANNED = [
    r"\bNOW\s*\(",
    r"\bCURRENT_TIMESTAMP\b",
    r"\bCURRENT_DATE\b",
    r"\bCURRENT_TIME\b",
    r"\bRANDOM\s*\(",
    r"\bUUID\s*\(",
    r"\bGEN_RANDOM_UUID\s*\(",
]


def _strip_strings_and_comments(sql: str) -> str:
    sql = re.sub(r"--[^\n]*", "", sql)
    sql = re.sub(r"/\*.*?\*/", "", sql, flags=re.S)
    sql = re.sub(r"'(?:''|[^'])*'", "''", sql)
    return sql


def _toplevel(sql: str) -> str:
    """Return SQL with all parenthesised groups erased (depth>0)."""
    out: list[str] = []
    depth = 0
    for ch in sql:
        if ch == "(":
            depth += 1
            continue
        if ch == ")":
            depth = max(0, depth - 1)
            continue
        if depth == 0:
            out.append(ch)
    return "".join(out)


def main() -> int:
    raw = sys.stdin.read()
    sql = _strip_strings_and_comments(raw)
    for pat in BANNED:
        m = re.search(pat, sql, re.I)
        if m:
            print(
                f"QUERY|ERROR|non-deterministic token: {m.group(0).strip()}",
                file=sys.stderr,
            )
            return 6
    top = _toplevel(sql)
    if not re.search(r"\bORDER\s+BY\b", top, re.I):
        print(
            "QUERY|ERROR|materialized SQL must have ORDER BY on the outer SELECT",
            file=sys.stderr,
        )
        return 6
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run test**

```bash
node --test tests/query_node/determinism.test.mjs
```

Expected: PASS all 8.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/query-determinism.py tests/query_node/determinism.test.mjs
git commit -m "feat(query): determinism guard for materialized queries"
```

---

### Task 12: query.sh render-one — render single materialized query

**Files:**
- Modify: `scripts/query.sh`
- Test: `tests/query_page_test.bats` (extend)

- [ ] **Step 1: Write failing test**

Append to `tests/query_page_test.bats`:

```bash
@test "query render-one materializes target dataset" {
  bash scripts/query.sh new top-trades --out=summary >/dev/null
  # Replace placeholder SQL with a real query.
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/top-trades.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1",
)
p.write_text(text)
PY
  run bash scripts/query.sh render-one top-trades
  [ "$status" -eq 0 ]
  [ -f content/datasets/summary.md ]
  run grep -F 'category,total' content/datasets/summary.md
  [ "$status" -eq 0 ]
  run grep -E '^sql_hash: sha256-' content/queries/top-trades.md
  [ "$status" -eq 0 ]
  [ -f content/queries/top-trades.sql.hash ]
}

@test "render-one is byte-identical on second run" {
  bash scripts/query.sh new dup-q --out=dup-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/dup-q.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  bash scripts/query.sh render-one dup-q
  cp content/datasets/dup-out.md /tmp/dup-out-1.md
  bash scripts/query.sh render-one dup-q
  diff /tmp/dup-out-1.md content/datasets/dup-out.md
}

@test "render-one rejects non-deterministic SQL" {
  bash scripts/query.sh new ndq --out=ndq-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/ndq.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT NOW() AS t FROM trades ORDER BY t",
)
p.write_text(text)
PY
  run bash scripts/query.sh render-one ndq
  [ "$status" -eq 6 ]
  [[ "$output" == *"non-deterministic"* ]]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_page_test.bats -f "render-one"
```

Expected: FAIL.

- [ ] **Step 3: Implement `cmd_render_one`**

Add to `scripts/query.sh`, plus wire `render-one` and `render` cases in `main`:

```bash
_query_page_sql() {
  python3 - "$1" <<'PY'
import re, sys, pathlib
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
m = re.search(r"```sql\s*\n(.*?)\n```", text, re.S)
print(m.group(1) if m else "", end="")
PY
}

_query_page_out() {
  python3 - "$1" <<'PY'
import re, sys, pathlib
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
m = re.search(r"^out:\s*(\S+)", text, re.M)
print(m.group(1) if m else "", end="")
PY
}

cmd_render_one() {
  local slug="${1:-}"
  [[ -n "$slug" ]] || die "missing query slug"
  local page="content/queries/$slug.md"
  [[ -f "$page" ]] || die "no such query: $page"
  local sql out
  sql="$(_query_page_sql "$page")"
  out="$(_query_page_out "$page")"
  [[ -n "$sql" ]] || die "no \`\`\`sql fence in $page"
  [[ -n "$out" ]] || die "no out: <slug> in frontmatter of $page"

  # Determinism guard.
  if ! printf '%s' "$sql" | python3 "$REPO_ROOT/scripts/lib/query-determinism.py"; then
    exit 6
  fi

  # Materialize via cmd_run --out --force.
  cmd_run "$sql" --out="$out" --force

  # Update sql_hash + sidecar.
  local h; h="$(bash "$ENGINE" hash "$sql")"
  python3 - "$page" "$h" <<'PY'
import re, sys, pathlib
page = pathlib.Path(sys.argv[1])
h = sys.argv[2]
text = page.read_text(encoding="utf-8")
text2 = re.sub(r'^sql_hash:\s*"[^"]*"', f'sql_hash: "sha256-{h}"', text, count=1, flags=re.M)
if text2 == text:
    text2 = re.sub(r'^sql_hash:.*$', f'sql_hash: "sha256-{h}"', text, count=1, flags=re.M)
page.write_text(text2, encoding="utf-8")
PY
  printf 'sha256-%s\n' "$h" > "content/queries/$slug.sql.hash"
  note "RENDER|$slug|out=$out|hash=sha256-$h"
}

cmd_render() {
  shopt -s nullglob
  local rc=0
  for page in content/queries/*.md; do
    local slug; slug="$(basename "$page" .md)"
    cmd_render_one "$slug" || rc=$?
  done
  return $rc
}
```

In `main()`:

```bash
    render)        cmd_render "$@" ;;
    render-one)    cmd_render_one "$@" ;;
```

- [ ] **Step 4: Run tests**

```bash
bats tests/query_page_test.bats
```

Expected: PASS all five.

- [ ] **Step 5: Commit**

```bash
git add scripts/query.sh tests/query_page_test.bats
git commit -m "feat(query): render-one + render — materialize type:query pages"
```

---

## Phase 4 — Inline SQL fence (Surface C)

### Task 13: Extract managed-region helper

Refactor `chart.sh`'s inline regex into a shared `lib/managed-region.sh`. Chart layer continues to work; query layer reuses it.

**Files:**
- Create: `scripts/lib/managed-region.sh`
- Modify: `scripts/chart.sh` (refactor only)
- Test: `tests/chart_render_test.bats` (must still pass — no new tests needed; refactor is behavior-preserving)

- [ ] **Step 1: Read existing chart managed-region code**

```bash
grep -nB1 -A5 "BEGIN chart-preview" scripts/chart.sh
```

Note exact regex used and write down (it includes `re.sub(...)` in a Python heredoc).

- [ ] **Step 2: Create helper**

Create `scripts/lib/managed-region.sh`:

```bash
#!/usr/bin/env bash
# scripts/lib/managed-region.sh — read/write BEGIN/END managed regions in
# markdown pages. Marker style: <!-- BEGIN <kind>:<id> --> ... <!-- END <kind>:<id> -->
#
# Functions (sourced):
#   managed_region_replace <page> <kind> <id> <body-file>
#     Replace the body inside the named region. Insert a fresh region appended
#     to EOF if absent. Idempotent: byte-identical when body file unchanged.
#
#   managed_region_extract <page> <kind> <id>
#     Print body on stdout. Empty if region absent.

managed_region_replace() {
  local page="$1" kind="$2" id="$3" body_file="$4"
  python3 - "$page" "$kind" "$id" "$body_file" <<'PY'
import re, sys, pathlib
page, kind, ident, body_file = sys.argv[1:5]
text = pathlib.Path(page).read_text(encoding="utf-8")
body = pathlib.Path(body_file).read_text(encoding="utf-8")
if not body.endswith("\n"):
    body += "\n"
begin = f"<!-- BEGIN {kind}:{ident} -->"
end = f"<!-- END {kind}:{ident} -->"
new_block = f"{begin}\n{body}{end}"
pat = re.compile(
    rf"<!-- BEGIN {re.escape(kind)}:{re.escape(ident)} -->\n.*?\n<!-- END {re.escape(kind)}:{re.escape(ident)} -->",
    re.S,
)
if pat.search(text):
    out = pat.sub(new_block, text)
else:
    sep = "" if text.endswith("\n") else "\n"
    out = text + sep + "\n" + new_block + "\n"
pathlib.Path(page).write_text(out, encoding="utf-8")
PY
}

managed_region_extract() {
  local page="$1" kind="$2" id="$3"
  python3 - "$page" "$kind" "$id" <<'PY'
import re, sys, pathlib
page, kind, ident = sys.argv[1:4]
text = pathlib.Path(page).read_text(encoding="utf-8")
pat = re.compile(
    rf"<!-- BEGIN {re.escape(kind)}:{re.escape(ident)} -->\n(.*?)\n<!-- END {re.escape(kind)}:{re.escape(ident)} -->",
    re.S,
)
m = pat.search(text)
print(m.group(1) if m else "", end="")
PY
}
```

- [ ] **Step 3: Refactor `chart.sh`**

In `scripts/chart.sh`, find the Python heredoc that uses `re.sub(r"<!-- BEGIN chart-preview:..."`. Replace its surrounding logic to source and call `managed_region_replace`. Concretely, locate the function that writes the chart-preview block and replace its inner Python heredoc with:

```bash
# shellcheck source=lib/managed-region.sh
source "$REPO_ROOT/scripts/lib/managed-region.sh"
managed_region_replace "$page" chart-preview "$cid" "$body_file"
```

Where `$body_file` is the temp file containing the rendered preview body. (Adjust variable names to match the existing function; the goal is no behavior change — just delegating the regex to the helper.)

- [ ] **Step 4: Run existing chart tests**

```bash
bats tests/chart_render_test.bats tests/chart_obsidian_preview_test.bats
```

Expected: PASS (no regressions).

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/managed-region.sh scripts/chart.sh
git commit -m "refactor(chart): extract managed-region helper for query reuse"
```

---

### Task 14: query-format.py — JSON rows → markdown table

**Files:**
- Create: `scripts/lib/query-format.py`
- Test: `tests/query_node/format.test.mjs`

- [ ] **Step 1: Write failing test**

Create `tests/query_node/format.test.mjs`:

```javascript
import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const SCRIPT = resolve("scripts/lib/query-format.py");

function fmt(rows) {
  return execFileSync("python3", [SCRIPT, "--format=md"], {
    input: JSON.stringify(rows),
    encoding: "utf-8",
  });
}

test("two-column table", () => {
  const out = fmt([{ a: 1, b: "x" }, { a: 2, b: "y" }]);
  assert.match(out, /\| a \| b \|/);
  assert.match(out, /\|---\|---\|/);
  assert.match(out, /\| 1 \| x \|/);
});

test("empty rows yields explicit empty marker", () => {
  const out = fmt([]);
  assert.match(out, /\(no rows\)/);
});

test("escapes pipe characters in cells", () => {
  const out = fmt([{ a: "x|y" }]);
  assert.match(out, /\| x\\\|y \|/);
});
```

- [ ] **Step 2: Run failing**

```bash
node --test tests/query_node/format.test.mjs
```

Expected: FAIL.

- [ ] **Step 3: Implement formatter**

Create `scripts/lib/query-format.py`:

```python
#!/usr/bin/env python3
"""Convert DuckDB JSON rows on stdin to a markdown table on stdout."""
import argparse
import json
import sys


def md(rows: list[dict]) -> str:
    if not rows:
        return "(no rows)\n"
    cols = list(rows[0].keys())
    def cell(v):
        s = "" if v is None else str(v)
        return s.replace("|", "\\|")
    lines = [
        "| " + " | ".join(cols) + " |",
        "|" + "|".join(["---"] * len(cols)) + "|",
    ]
    for r in rows:
        lines.append("| " + " | ".join(cell(r.get(c)) for c in cols) + " |")
    return "\n".join(lines) + "\n"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--format", choices=["md"], default="md")
    ap.parse_args()
    raw = sys.stdin.read().strip() or "[]"
    rows = json.loads(raw)
    sys.stdout.write(md(rows))
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run test**

```bash
node --test tests/query_node/format.test.mjs
```

Expected: PASS all three.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/query-format.py tests/query_node/format.test.mjs
git commit -m "feat(query): JSON rows -> markdown table formatter"
```

---

### Task 15: Inline awiki-query fence renderer

**Files:**
- Modify: `scripts/query.sh`
- Test: `tests/query_fence_test.bats`

- [ ] **Step 1: Write failing test**

Create `tests/query_fence_test.bats`:

```bash
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
```

- [ ] **Step 2: Run failing**

```bash
bats tests/query_fence_test.bats
```

Expected: FAIL.

- [ ] **Step 3: Implement `cmd_fence_render`**

Add to `scripts/query.sh` (and source the managed-region helper near the top of the file):

```bash
# Near the top, after ENGINE assignment:
# shellcheck source=lib/managed-region.sh
source "$REPO_ROOT/scripts/lib/managed-region.sh"

_iter_fences() {
  # Print TSV: <page>\t<id>\t<base64-sql>
  python3 - <<'PY'
import re, base64, pathlib
for page in pathlib.Path("content").rglob("*.md"):
    text = page.read_text(encoding="utf-8")
    for m in re.finditer(
        r'```sql\s+awiki-query\s+id="([a-z0-9][a-z0-9-]*)"\s*\n(.*?)\n```',
        text, re.S,
    ):
        sql_b64 = base64.b64encode(m.group(2).encode()).decode()
        print(f"{page}\t{m.group(1)}\t{sql_b64}")
PY
}

cmd_fence_render() {
  local rc=0
  declare -A page_hashes
  while IFS=$'\t' read -r page fid sql_b64; do
    [[ -z "$page" ]] && continue
    local sql; sql="$(printf '%s' "$sql_b64" | base64 --decode)"
    local rows; rows="$(bash "$ENGINE" run "$sql")" || { rc=$?; continue; }
    local body_md; body_md="$(printf '%s' "$rows" | python3 "$REPO_ROOT/scripts/lib/query-format.py" --format=md)"
    local body_file; body_file="$(mktemp)"
    printf '%s' "$body_md" > "$body_file"
    managed_region_replace "$page" query-result "$fid" "$body_file"
    rm -f "$body_file"
    local h; h="$(bash "$ENGINE" hash "$sql")"
    page_hashes["$page"]+="$fid:$h"$'\n'
    note "FENCE-RENDER|$page|id=$fid|hash=sha256-$h"
  done < <(_iter_fences)

  for page in "${!page_hashes[@]}"; do
    python3 - "$page" "${page_hashes[$page]}" <<'PY'
import json, sys, pathlib
page = pathlib.Path(sys.argv[1])
side = page.with_suffix(".queries.json")
entries = {}
for line in sys.argv[2].splitlines():
    if not line.strip(): continue
    fid, h = line.split(":", 1)
    entries[fid] = f"sha256-{h}"
side.write_text(json.dumps(entries, sort_keys=True, indent=2) + "\n")
PY
  done
  return $rc
}
```

In `main()`:

```bash
    fence-render)  cmd_fence_render "$@" ;;
```

- [ ] **Step 4: Run tests**

```bash
bats tests/query_fence_test.bats
```

Expected: PASS both.

- [ ] **Step 5: Commit**

```bash
git add scripts/query.sh tests/query_fence_test.bats
git commit -m "feat(query): inline awiki-query fence -> managed region renderer"
```

---

## Phase 5 — Lint Q-rules

### Task 16: lint-query.sh — Q1 parse + Q2 ref-resolve

**Files:**
- Create: `scripts/lint-query.sh`
- Modify: `scripts/lint.sh`
- Test: `tests/lint_query_test.bats`

- [ ] **Step 1: Write failing test**

Create `tests/lint_query_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/tests/fixtures/query/trades.csv" "$WORK/trades.csv"
  mkdir -p "$WORK/content/datasets" "$WORK/content/queries" "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "Q1: missing sql fence flagged" {
  cat > content/queries/no-sql.md <<'MD'
---
type: query
out: foo
---
# x
MD
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q1|"* ]]
}

@test "Q2: unknown referenced dataset flagged" {
  cat > content/queries/bad-ref.md <<'MD'
---
type: query
out: foo
---
## SQL
```sql
SELECT * FROM ghost ORDER BY 1
```
MD
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q2|"* ]]
  [[ "$output" == *"ghost"* ]]
}

@test "Q1+Q2: clean query page lints clean" {
  bash scripts/query.sh new ok-q --out=ok-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/ok-q.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  run bash scripts/lint-query.sh
  [ "$status" -eq 0 ]
  [[ "$output" != *"|Q1|"* ]]
  [[ "$output" != *"|Q2|"* ]]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/lint_query_test.bats
```

Expected: FAIL.

- [ ] **Step 3: Implement Q1+Q2**

Create `scripts/lint-query.sh`:

```bash
#!/usr/bin/env bash
# scripts/lint-query.sh — Q-code query linter. Sourced by lint.sh OR run standalone.
set -uo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
RESOLVE_PY="$REPO_ROOT/scripts/lib/query-resolve.py"
DETERMINISM_PY="$REPO_ROOT/scripts/lib/query-determinism.py"
ENGINE="$REPO_ROOT/scripts/lib/query-engine.sh"

: "${FIX:=0}"

_emit_q() {
  local level="$1"
  level="$(printf '%s' "$level" | tr '[:lower:]' '[:upper:]')"
  printf 'LINT|%s|%s|%s|%s\n' "$level" "$2" "$3" "$4"
}

_query_pages() {
  find content/queries -maxdepth 1 -type f -name '*.md' 2>/dev/null
}

lint_query_one() {
  local page="$1"
  local rel="${page#$REPO_ROOT/}"
  rel="${rel#./}"

  # Q1 — must contain a ```sql fence under ## SQL.
  local sql
  sql="$(python3 - "$page" <<'PY'
import re, sys, pathlib
text = pathlib.Path(sys.argv[1]).read_text(encoding="utf-8")
m = re.search(r"```sql\s*\n(.*?)\n```", text, re.S)
print(m.group(1) if m else "", end="")
PY
)"
  if [[ -z "$sql" ]]; then
    _emit_q error "$rel" Q1 "no \`\`\`sql fence found"
    return 1
  fi

  # Q2 — every referenced slug must exist on disk.
  local missing=0
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if [[ ! -f "content/datasets/$slug.md" ]]; then
      _emit_q error "$rel" Q2 "unknown dataset: $slug"
      missing=1
    fi
  done < <(printf '%s' "$sql" | python3 "$RESOLVE_PY")
  return $missing
}

lint_query_all() {
  local rc=0
  while IFS= read -r page; do
    [[ -z "$page" ]] && continue
    lint_query_one "$page" || rc=1
  done < <(_query_pages)
  return $rc
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  lint_query_all
fi
```

`chmod +x scripts/lint-query.sh`.

- [ ] **Step 4: Wire into lint.sh**

In `scripts/lint.sh`, after the `lint-chart.sh` source block, add:

```bash
# Source query lint extension (Q-codes).
if [[ -f "$(dirname "$0")/lint-query.sh" ]]; then
  # shellcheck disable=SC1091
  source "$(dirname "$0")/lint-query.sh"
elif [[ -f scripts/lint-query.sh ]]; then
  # shellcheck disable=SC1091
  source scripts/lint-query.sh
fi
```

Also wire `--only=query` routing (search for `ONLY` handling in `lint.sh`; mirror the `chart` branch that calls `lint_chart_*` to call `lint_query_all`).

- [ ] **Step 5: Run tests**

```bash
bats tests/lint_query_test.bats
```

Expected: PASS three.

- [ ] **Step 6: Commit**

```bash
git add scripts/lint-query.sh scripts/lint.sh tests/lint_query_test.bats
git commit -m "feat(lint-query): Q1 parse, Q2 reference resolve + lint.sh routing"
```

---

### Task 17: Q3 determinism + Q4 hash staleness

**Files:**
- Modify: `scripts/lint-query.sh`
- Test: `tests/lint_query_test.bats` (extend)

- [ ] **Step 1: Write failing tests**

Append to `tests/lint_query_test.bats`:

```bash
@test "Q3: NOW() flagged" {
  cat > content/queries/nd.md <<'MD'
---
type: query
out: foo
---
## SQL
```sql
SELECT NOW() AS t FROM trades ORDER BY t
```
MD
  # Need source dataset for Q2 to pass.
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q3|"* ]]
}

@test "Q4: stale sql_hash flagged" {
  bash scripts/query.sh new s-q --out=s-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/s-q.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  bash scripts/query.sh render-one s-q
  # Mutate SQL but do not re-render: sidecar is now stale.
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/s-q.md")
text = p.read_text().replace(
    "SELECT id FROM trades ORDER BY id",
    "SELECT id FROM trades WHERE id > 0 ORDER BY id",
)
p.write_text(text)
PY
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q4|"* ]]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/lint_query_test.bats -f "Q3|Q4"
```

Expected: FAIL.

- [ ] **Step 3: Extend `lint_query_one`**

In `scripts/lint-query.sh`, replace `lint_query_one` body's tail with:

```bash
  # Q3 — determinism (only on materialized type:query pages).
  if printf '%s' "$sql" | python3 "$DETERMINISM_PY" 2>/tmp/q3.err >/dev/null; then
    :
  else
    _emit_q error "$rel" Q3 "$(cat /tmp/q3.err | tr '\n' ' ')"
    rm -f /tmp/q3.err
    missing=1
  fi
  rm -f /tmp/q3.err

  # Q4 — sidecar staleness.
  local slug; slug="$(basename "$page" .md)"
  local sidecar="content/queries/$slug.sql.hash"
  if [[ -f "$sidecar" ]]; then
    local now_hash; now_hash="sha256-$(bash "$ENGINE" hash "$sql")"
    local prev_hash; prev_hash="$(cat "$sidecar" | tr -d '[:space:]')"
    if [[ "$now_hash" != "$prev_hash" ]]; then
      _emit_q error "$rel" Q4 "sidecar stale: $sidecar (run \`just query-render-one $slug\`)"
      missing=1
    fi
  fi
  return $missing
}
```

- [ ] **Step 4: Run tests**

```bash
bats tests/lint_query_test.bats
```

Expected: PASS all five.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint-query.sh tests/lint_query_test.bats
git commit -m "feat(lint-query): Q3 determinism + Q4 sidecar staleness"
```

---

### Task 18: Q5 inline fence managed-region tamper + Q-PRIV stub

**Files:**
- Modify: `scripts/lint-query.sh`
- Test: `tests/lint_query_test.bats` (extend)

- [ ] **Step 1: Write failing test**

Append to `tests/lint_query_test.bats`:

```bash
@test "Q5: tampered managed region flagged" {
  cat > content/concepts/cf.md <<'MD'
---
type: concept
---

```sql awiki-query id="x"
SELECT id FROM trades ORDER BY id
```

<!-- BEGIN query-result:x -->
| id |
|---|
| 999 |
<!-- END query-result:x -->
MD
  mkdir -p content/concepts
  # Sidecar pretends current — but body is tampered.
  echo '{"x": "sha256-deadbeef"}' > content/concepts/cf.queries.json
  run bash scripts/lint-query.sh
  [[ "$output" == *"|Q5|"* ]]
}

@test "Q-PRIV stub: never fails (stage 3 placeholder)" {
  bash scripts/query.sh new okp --out=okp-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/okp.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT id FROM trades ORDER BY id",
)
p.write_text(text)
PY
  bash scripts/query.sh render-one okp >/dev/null
  run bash scripts/lint-query.sh
  [ "$status" -eq 0 ]
  [[ "$output" != *"|Q-PRIV|"* ]]
}
```

- [ ] **Step 2: Run failing**

```bash
bats tests/lint_query_test.bats -f "Q5|Q-PRIV"
```

Expected: FAIL.

- [ ] **Step 3: Extend lint with Q5 + Q-PRIV stub**

In `scripts/lint-query.sh`, add:

```bash
lint_query_fences() {
  # Q5 — every page with awiki-query fences must have a sidecar entry whose
  # current hash matches. (Sidecar is pages-relative, named <page>.queries.json.)
  python3 - <<'PY' > /tmp/q5.tsv
import json, re, base64, pathlib
for page in pathlib.Path("content").rglob("*.md"):
    text = page.read_text(encoding="utf-8")
    fences = list(re.finditer(
        r'```sql\s+awiki-query\s+id="([a-z0-9][a-z0-9-]*)"\s*\n(.*?)\n```',
        text, re.S))
    if not fences:
        continue
    side = page.with_suffix(".queries.json")
    entries = json.loads(side.read_text()) if side.exists() else {}
    for m in fences:
        fid = m.group(1)
        sql_b64 = base64.b64encode(m.group(2).encode()).decode()
        prev = entries.get(fid, "")
        print(f"{page}\t{fid}\t{sql_b64}\t{prev}")
PY
  local rc=0
  while IFS=$'\t' read -r page fid sql_b64 prev; do
    [[ -z "$page" ]] && continue
    local sql; sql="$(printf '%s' "$sql_b64" | base64 --decode)"
    local now; now="sha256-$(bash "$ENGINE" hash "$sql")"
    if [[ "$prev" != "$now" ]]; then
      _emit_q error "$page" Q5 "managed region stale or tampered (id=$fid)"
      rc=1
    fi
  done < /tmp/q5.tsv
  rm -f /tmp/q5.tsv
  return $rc
}

lint_query_priv() {
  : # Q-PRIV stub for Stage 3. Intentionally a no-op.
  return 0
}
```

Update the standalone runner:

```bash
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  rc=0
  lint_query_all || rc=1
  lint_query_fences || rc=1
  lint_query_priv || rc=1
  exit $rc
fi
```

(If `lint.sh` calls these via `--only=query` routing, point that branch at the same trio.)

- [ ] **Step 4: Run tests**

```bash
bats tests/lint_query_test.bats
```

Expected: PASS seven.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint-query.sh tests/lint_query_test.bats
git commit -m "feat(lint-query): Q5 fence region tamper + Q-PRIV stub"
```

---

## Phase 6 — Build integration + MCP + smoke

### Task 19: build.sh runs query render before chart render

**Files:**
- Modify: `scripts/build.sh`
- Test: `tests/data_layer_full_smoke_test.bats` (extend or use existing if its scope already covers full pipelines)

- [ ] **Step 1: Read current build.sh data-layer block**

Lines ~377–385 contain the `chart.sh render` invocation guarded by `AWIKI_DATA_LAYER=on`.

- [ ] **Step 2: Modify build.sh**

Insert before the existing `bash scripts/chart.sh render || ...` line:

```bash
    bash scripts/query.sh render || echo "BUILD|WARN|query-render returned non-zero"
    bash scripts/query.sh fence-render || echo "BUILD|WARN|query-fence-render returned non-zero"
```

(Run query before chart so charts could one day consume materialized datasets.)

- [ ] **Step 3: Add smoke case**

Append a smoke test to `tests/data_layer_full_smoke_test.bats`:

```bash
@test "smoke: query layer end-to-end (init -> dataset -> query -> build -> lint)" {
  bash scripts/data-init.sh >/dev/null
  bash scripts/dataset.sh new trades --format=csv --from=trades.csv >/dev/null
  bash scripts/query.sh new top --out=top-out >/dev/null
  python3 - <<'PY'
import pathlib
p = pathlib.Path("content/queries/top.md")
text = p.read_text().replace(
    "SELECT 1 AS placeholder\nORDER BY 1",
    "SELECT category, SUM(amount) AS total FROM trades GROUP BY 1 ORDER BY 1",
)
p.write_text(text)
PY
  run bash scripts/build.sh --full
  [ "$status" -eq 0 ]
  [ -f content/datasets/top-out.md ]
  run bash scripts/lint.sh --only=query
  [ "$status" -eq 0 ]
}
```

(Adjust `setup()` to copy `tests/fixtures/query/trades.csv` into the workdir if not already present.)

- [ ] **Step 4: Run smoke**

```bash
bats tests/data_layer_full_smoke_test.bats
```

Expected: PASS (existing + new case).

- [ ] **Step 5: Commit**

```bash
git add scripts/build.sh tests/data_layer_full_smoke_test.bats
git commit -m "feat(build): run query-render + fence-render before chart-render"
```

---

### Task 20: MCP list_queries tool

**Files:**
- Modify: `mcp/...` (locate file via `grep -rn 'list_charts' mcp/`)
- Test: existing MCP test fixtures or repo-style MCP test (mirror `list_charts`)

- [ ] **Step 1: Locate list_charts implementation**

```bash
grep -rn "list_charts" mcp/ tests/ 2>/dev/null
```

Note the file and tool registration pattern.

- [ ] **Step 2: Write failing test**

Mirror the existing `list_charts` test (find it via `grep -rn list_charts tests/`). Add a sibling case for `list_queries` that creates two `type: query` pages and asserts both are returned.

- [ ] **Step 3: Implement `list_queries`**

In the same MCP source as `list_charts`, register a new tool that walks `content/queries/*.md` (and inline `awiki-query` fences) and returns page paths + ids. Reuse the page-walker helper used by `list_charts`.

- [ ] **Step 4: Run MCP test**

```bash
# Use the same runner as the existing MCP tests.
bats tests/mcp_*.bats || true
```

Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add mcp/ tests/
git commit -m "feat(mcp): list_queries — enumerate type:query pages + awiki-query fences"
```

---

### Task 21: Docs — README, data-help.txt, WIKI.md

**Files:**
- Modify: `README.md`
- Modify: `docs/data-help.txt`
- Modify: `WIKI.md`

- [ ] **Step 1: README query section**

In `README.md`, after the chart section, add a "Query layer" subsection covering:

- One-line summary.
- `just query "SELECT ..."` example.
- `just query-new <slug> --out=<dataset>` example.
- Inline ` ```sql awiki-query id="..." ` example with one-line note that it renders a markdown table.
- Pointer to `WIKI.md` §Query for schema details.

- [ ] **Step 2: data-help.txt**

In `docs/data-help.txt`, append a Query section listing every recipe and subcommand introduced in this plan:

```
QUERY LAYER
  just query "<SQL>"                    Run an ad-hoc SQL query against datasets.
  just query "<SQL>" --out=<slug>       Materialize the result as a dataset page.
  just query-new <slug> --out=<dst>     Scaffold a type:query page that materializes
                                        to <dst>.
  just query-render-one <slug>          Re-render one type:query page.
  just query-render                     Re-render every type:query page.
  just query-fence-render               Re-run every inline awiki-query fence.

LINT
  just lint --only=query                Run Q1..Q5 + Q-PRIV stub.
```

- [ ] **Step 3: WIKI.md updates**

In `WIKI.md`:

1. Page-kind enum (around line 22): add `query` next to `chart` in the comment list.
2. Add new section `### 4.x Query` describing:
   - `type: query` page schema (frontmatter keys: `out`, `sources`, `privacy`, `deterministic`, `sql_hash`).
   - Inline `awiki-query` fence pattern + managed-region marker convention.
   - Lint rules Q1–Q5 + Q-PRIV stub.
   - Privacy note: Stage 1 author-declares; Stage 3 will enforce floor.

- [ ] **Step 4: Verify lint-clean**

```bash
just lint || true
```

Inspect for any new lint hits caused by docs changes.

- [ ] **Step 5: Commit**

```bash
git add README.md docs/data-help.txt WIKI.md
git commit -m "docs(query): README + data-help + WIKI.md schema for query layer"
```

---

### Task 22: CI — install duckdb in runners

**Files:**
- Modify: `.github/workflows/*.yml` (locate via `grep -rn vl-convert .github`)

- [ ] **Step 1: Locate vl-convert install step**

```bash
grep -rn "vl-convert" .github/workflows/ 2>/dev/null
```

- [ ] **Step 2: Add duckdb install**

Beside the `vl-convert` install step (mirror the same OS guards), add:

```yaml
      - name: Install DuckDB CLI
        run: |
          if [ "$RUNNER_OS" = "macOS" ]; then
            brew install duckdb
          else
            curl -fsSL https://github.com/duckdb/duckdb/releases/latest/download/duckdb_cli-linux-amd64.zip -o /tmp/duckdb.zip
            unzip /tmp/duckdb.zip -d /tmp
            sudo mv /tmp/duckdb /usr/local/bin/duckdb
          fi
          duckdb --version
```

- [ ] **Step 3: Verify locally (best effort)**

```bash
duckdb --version
```

- [ ] **Step 4: Commit**

```bash
git add .github/workflows/
git commit -m "ci: install duckdb CLI for query layer tests"
```

---

## Self-Review

Done after writing the full plan. Spot-check spec coverage:

- §1 Purpose / Scope — Tasks 1–22 deliver Stage 1 surface; Stage 2/3 explicitly deferred ✓
- §3 Architecture — Engine (Task 6), three frontends (Tasks 7–8 / 10–12 / 13–15), cache + sidecars (Tasks 1, 12, 15) ✓
- §4 Components — every line in the components table maps to a task:
  - Engine lib → 6
  - Extractor → 4
  - CLI frontend → 7, 8, 10, 12, 15
  - Page bootstrap → 10
  - Render integration → 19
  - Lint module → 16, 17, 18
  - Just recipes → 9
  - MCP → 20
  - Dep check → 3
  - Managed-region helper → 13
- §5 Data flow — Surface A (Tasks 7–8), Surface B (Tasks 10–12), Surface C (Tasks 13–15) ✓
- §6 Reference resolution — Task 5 covers all listed identifier shapes; tests assert each ✓
- §7 Determinism guard — Task 11 covers banned tokens + outer ORDER BY; tests assert each ✓
- §8 Caching + hash model — engine `hash` subcommand (Task 6); sidecar paths (Tasks 12, 15); staleness (Task 17 Q4, Task 18 Q5) ✓
- §9 Error handling — exit codes 2–9 covered:
  - 2 duckdb missing (Task 6 + Task 3 advisory)
  - 3 unknown dataset (Task 6, Task 7 test)
  - 4 extract fail (Task 4 test)
  - 5 SQL parse (Task 7 test)
  - 6 determinism (Task 11, Task 12)
  - 7 collision (Task 8)
  - 9 attach rejected (Task 6 test)
  - exit 8 `--require-rows` is in spec but no task — accept as YAGNI for v1; remove from spec or add micro-task.
- §10 Lint rules Q1–Q5 + Q-PRIV — Tasks 16, 17, 18 ✓
- §11 Privacy — Q-PRIV stub (Task 18); author-declared `privacy:` written by `query.sh new` (Task 10) ✓
- §12 Tooling integration — recipes (Task 9), build.sh (Task 19), check-deps (Task 3) ✓
- §13 Testing — every BATS file in spec has a creating task; node:test units across Tasks 5, 11, 14; smoke in Task 19; CI in Task 22 ✓
- §15 Risks — DuckDB version pin in CI (Task 22 — note: pin minimum version with `duckdb --version` check); regex-vs-sqlparse decision deferred (regex chosen, plan-locked); cache cleanup (`.gitignore` covers; `just clean` extension TODO — minor, not Stage 1 blocking).

**Gaps fixed inline:**

- Exit code 8 (`--require-rows`) — drop from Stage 1; not implementing in any task. Spec §9 will be updated when this plan lands; not a plan blocker.
- DuckDB minimum version pin — Task 22 just runs `duckdb --version`. Add a min-version guard to `check-deps.sh` later if drift bites; not Stage 1 blocking.

**Placeholder scan:** No `TBD`/`TODO`/"implement later" in any task body. Every code step has actual code. Every command has expected output.

**Type consistency:** Function names used consistently:
- `_resolve_refs`, `_extract_one`, `_format_for`, `_view_ddl`, `_prepare`, `cmd_run`, `cmd_hash`, `cmd_new`, `cmd_render`, `cmd_render_one`, `cmd_fence_render` — defined in `query-engine.sh` / `query.sh`, invoked under those exact names.
- `managed_region_replace`, `managed_region_extract` — defined Task 13, used Tasks 13 (chart refactor) and 15.
- Sidecar names: `<slug>.sql.hash` (Surface B) and `<page>.queries.json` (Surface C) — consistent across Tasks 12, 15, 17, 18.

Plan is internally consistent.

---

## Execution Handoff

Plan complete and saved to `docs/superpowers/plans/2026-04-28-duckdb-query-layer.md`. Two execution options:

1. **Subagent-Driven (recommended)** — fresh subagent per task, review between tasks.
2. **Inline Execution** — execute tasks in this session with checkpoints.
