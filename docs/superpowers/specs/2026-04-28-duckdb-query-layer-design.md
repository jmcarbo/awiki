# DuckDB Query Layer — Stage 1 Design

**Date:** 2026-04-28
**Status:** Draft (brainstorm-approved, awaiting spec review)
**Scope:** Stage 1 only. Stages 2–3 sketched as future work; not specced here.

## 1. Purpose

Add a SQL surface over awiki datasets using DuckDB. Three entry points share one engine. Local datasets only in Stage 1; external databases (Postgres et al.) and inline-page SQL fences are deferred — but design seams are reserved.

Stage 1 ships:
- DuckDB engine wrapper.
- CLI: `scripts/query.sh` + `just query` recipes.
- New page kind `query` (`type: query`) — SQL fence whose result materializes as a sibling dataset page.
- Inline `awiki-query` SQL fence on any markdown page — managed region holds rendered markdown table.
- Hash sidecar + staleness lint (Q-rules), determinism guard for materialized queries.

Out of Stage 1 (deferred):
- External DB attach + `connection` page kind + `.env` cred handling.
- Privacy floor enforcement (rule registered as no-op stub in Stage 1).

## 2. Goals / Non-goals

### Goals
- One engine, three frontends. No duplication of resolve/extract/run logic.
- Reuse data-layer conventions: dataset pages remain source of truth (markdown fence with `csv|tsv|json|dsv|topojson`).
- Reuse chart-layer patterns: managed regions, sidecar hash, `LINT|*` exit-code escalation.
- Reproducible diffs: materialized queries are byte-identical on re-run when inputs unchanged.

### Non-goals
- Live remote queries, external attaches.
- Privacy class enforcement on result rows (Stage 3).
- DuckDB extensions or custom table functions.
- Replacing the existing fence-based dataset format.

## 3. Architecture

```
+------------------------------------------+
| User SQL (3 surfaces)                    |
|  - CLI: scripts/query.sh / just query    |
|  - type: query page (materialize)        |
|  - ```sql awiki-query fence (any page)   |
+--------------------+---------------------+
                     |
                     v
+------------------------------------------+
| scripts/query.sh                         |
|  - parse args, route by surface          |
|  - call lib/query-engine.sh              |
+--------------------+---------------------+
                     |
                     v
+------------------------------------------+
| scripts/lib/query-engine.sh              |
|  1. Resolve referenced dataset slugs     |
|     in SQL (FROM/JOIN regex; CTE-aware)  |
|  2. query-extract.py:                    |
|     md fence -> .cache/duckdb/<slug>.<ext>|
|  3. Build DuckDB script:                 |
|       CREATE VIEW <slug> AS read_csv(...)|
|       <user SQL>                         |
|  4. Run via duckdb CLI -> JSON or CSV    |
|  5. Return result + hash to caller       |
+--------------------+---------------------+
                     |
                     v
+------------------------------------------+
| Output sinks (per surface)               |
|  CLI:        stdout (or --out=<slug>)    |
|  type:query: materialize dataset page    |
|  sql fence:  managed region (md table)   |
+------------------------------------------+
```

Engine = pure shell lib. Frontends own filesystem I/O for pages and managed regions. Engine writes only to `.cache/duckdb/` (gitignored) plus an explicit `--out` target when invoked.

## 4. Components

| Component | Path | Purpose |
|-----------|------|---------|
| Engine lib | `scripts/lib/query-engine.sh` | Resolve refs, drive extract, run DuckDB, hash result |
| Extractor | `scripts/lib/query-extract.py` | Pull fenced data block from `content/datasets/<slug>.md` to `.cache/duckdb/<slug>.<ext>` |
| Frontend (CLI) | `scripts/query.sh` | Subcommands: `run`, `new`, `page`, `fence`, `--out=<slug>` |
| Page bootstrap | `scripts/lib/query-page.sh` | Scaffold new `type: query` page |
| Render integration | `scripts/build.sh` (extension) | Re-run materialized queries + fences during `just build`; refresh managed regions |
| Managed-region helper | `scripts/lib/managed-region.sh` (new) | Extract shared BEGIN/END region rewrite from `chart.sh`; reused by query layer |
| Lint module | `scripts/lint-query.sh` | Q1–Q5 rules. Q-PRIV registered as no-op stub for Stage 3 |
| Just recipes | `justfile` | `query`, `query-new`, `query-render`, `query-render-one` |
| MCP tool | `mcp/...` | `list_queries()` only in Stage 1 (parity with `list_charts`) |
| Dep check | `scripts/check-deps.sh` | Advisory if `duckdb` CLI absent + data layer enabled |

Boundaries:
- Engine is pure: SQL in, result + hash out. No filesystem writes outside `.cache/duckdb/` and the explicit `--out` target.
- Frontends own page + managed-region I/O. Engine never parses markdown.
- Extractor reusable by future chart layer SQL pre-shape (seam left, not implemented).

## 5. Data flow

### 5.1 Surface A — CLI ad-hoc

```
$ scripts/query.sh run "SELECT category, SUM(amount) FROM trades GROUP BY 1"
1. Parse SQL -> referenced slugs: {trades}
2. Extract content/datasets/trades.md -> .cache/duckdb/trades.csv
3. duckdb -c "
     CREATE VIEW trades AS SELECT * FROM read_csv('.cache/duckdb/trades.csv', AUTO_DETECT=TRUE);
     <user SQL>
   " -json
4. stdout: [{"category":"x","sum":...}, ...]
```

With `--out=summary`: step 4 writes `content/datasets/summary.md` via the existing `dataset.sh new`-style flow. Frontmatter auto-populated:
- `sources: [trades]`
- `query: <sha256>` (of normalized SQL)
- `format: csv` (default; `--format=` overrides)

### 5.2 Surface B — `type: query` page (materialized)

```markdown
---
title: top trades
type: query
sources: [trades]
out: top-trades-summary
privacy: internal
deterministic: true
sql_hash: sha256-...
---

## SQL
```sql
SELECT category, SUM(amount) AS total
FROM trades
GROUP BY 1
ORDER BY 1
```
```

On `just query-render-one top-trades`:
1. Read SQL fence.
2. **Determinism guard**: regex pre-pass rejects `NOW()`, `CURRENT_*`, `RANDOM()`, `UUID()`, `gen_random_uuid()`, and absence of an outer `ORDER BY` on the top-level SELECT. Exit 6 on violation.
3. Run engine -> result rows.
4. Materialize to `content/datasets/top-trades-summary.md`. Only the managed `## Data` region is overwritten; other sections (description, schema, provenance) are preserved.
5. Update `sql_hash` frontmatter on the query page.
6. Write `content/queries/top-trades.sql.hash` sidecar (for staleness lint).

### 5.3 Surface C — inline SQL fence on any page

```markdown
... prose ...

```sql awiki-query id="cf-by-month"
SELECT month, SUM(amount) FROM trades GROUP BY 1 ORDER BY 1
```

<!-- BEGIN query-result:cf-by-month -->
| month | sum    |
|-------|--------|
| ...   | ...    |
<!-- END query-result:cf-by-month -->
```

On render (pre-Hugo pass):
1. Walk pages for ` ```sql awiki-query id="..."` fences.
2. Engine runs query.
3. The fence-rewrite step replaces the `BEGIN query-result:<id>` / `END query-result:<id>` block with a markdown table. Marker format mirrors chart layer (`BEGIN chart-preview:<id>` in `scripts/chart.sh`). The regex is small and currently inlined in `chart.sh`; this work should extract the shared logic to a new `scripts/lib/managed-region.sh` and have both layers consume it.
4. Hash sidecar `<page>.queries.json` keyed by fence id (hash of SQL + source dataset hashes).

**Output format for inline fence: markdown table only.**
- One artifact, one diff, one staleness check.
- Charts cannot consume it. If the user wants chart-able results, use Surface B (materialize) or Surface A (`--out=`).
- Stage 2 may add `out=<slug>` attribute on the fence; design seam reserved.

## 6. SQL ↔ dataset reference resolution

- Top-level approach: regex sniff on `FROM` and `JOIN` tokens, plus CTE name extraction so CTE aliases are not treated as datasets.
- Quoted identifiers respected (`FROM "trades"` resolves to slug `trades`).
- Subqueries traversed.
- Unknown identifier → engine exits 3 (`unknown dataset`) before invoking DuckDB. (DuckDB's own error is less helpful and would fire after the cache write step.)
- Tests must cover: bare ref, quoted ref, CTE alias, JOIN, subquery, schema-qualified-but-local (`main.trades`).

## 7. Determinism guard (Surface B only)

Inline fences are exploratory; allow non-determinism with a lint warning, not a hard error. Materialized queries (Surface B) commit results to git, so:

Reject (exit 6):
- Tokens: `NOW()`, `CURRENT_TIMESTAMP`, `CURRENT_DATE`, `CURRENT_TIME`, `RANDOM()`, `UUID()`, `gen_random_uuid()`.
- Top-level SELECT lacking an `ORDER BY`. (Multi-row results without `ORDER BY` are non-deterministic between DuckDB versions.)
- `LIMIT` without `ORDER BY` on the top-level.

Allowed:
- Deterministic functions over deterministic inputs.
- `ORDER BY` on inner subqueries does not satisfy the rule. Outer query must order.

Implementation: a regex/AST pre-pass in `query-engine.sh` (regex first; if false-positive-prone, escalate to a small Python AST helper using `sqlparse`. Decision deferred to plan: pick whichever passes the test matrix with less code).

## 8. Caching + hash model

- `.cache/duckdb/<slug>.<ext>`: extracted dataset bodies. Mtime + content hash compared against `content/datasets/<slug>.md` fence body. Stale → re-extract.
- Result hashing: `sha256(normalized_sql + sorted_inputs(<slug>:<dataset_hash>))`.
  - Normalization: trim, collapse whitespace, lowercase keywords.
- Sidecar paths:
  - Surface B: `content/queries/<slug>.sql.hash` — single value, the result hash above.
  - Surface C: `<page-path>.queries.json` — `{ "<fence-id>": "<hash>" }`.
- Stale-lint (Q4): if recomputed hash != sidecar value, fail lint with the existing `LINT|...` exit-code escalation.

## 9. Error handling

| Failure | Detection | Behavior |
|---------|-----------|----------|
| `duckdb` CLI missing | `check-deps.sh` advisory; engine probe at run | Exit 2; `QUERY\|ERROR\|duckdb CLI not found — install: brew install duckdb` |
| Referenced slug not found | Engine resolve step, before exec | Exit 3; `QUERY\|ERROR\|unknown dataset: <slug>` |
| Fence extraction fails (malformed) | `query-extract.py` | Exit 4; point to dataset page + line |
| SQL parse error | DuckDB stderr | Pass through verbatim; exit 5 |
| Determinism violation (Surface B) | Pre-exec regex/AST pass | Exit 6; `QUERY\|ERROR\|non-deterministic: NOW() at line N` |
| Materialize target collision | Compare existing target's storage/owner FM | Exit 7 unless `--force`; never silently overwrite human-authored data |
| Hash sidecar mismatch (stale) | `lint-query.sh` Q4 | Lint exit-code escalation (matches chart C7) |
| Cache directory absent | Engine | Auto-create `.cache/duckdb/` |
| `--require-rows` and zero rows | CLI flag | Exit 8 |
| External DB attempt (`ATTACH '...'`) | Engine pre-pass | Exit 9; `QUERY\|ERROR\|external attach not supported until stage 3` |

All log lines use the `QUERY|...` prefix (matches `DATASET|`, `CHART|`).

Privacy floor lint: registered as Q-PRIV no-op stub in Stage 1. Real implementation lands with Stage 3.

## 10. Lint rules (Q-series)

| Rule | Description |
|------|-------------|
| Q1 | Parse: `type: query` page must have one `sql` fence in the `## SQL` section |
| Q2 | Reference resolve: every dataset slug in SQL exists at `content/datasets/<slug>.md` |
| Q3 | Determinism (Surface B only): rejects banned tokens / missing outer ORDER BY |
| Q4 | Hash sidecar staleness (Surface B + C) |
| Q5 | Managed-region tamper (Surface C; mirror of chart C9) |
| Q-PRIV | Stub no-op for Stage 3 (rule registered, returns clean) |

Routing: `scripts/lint.sh --only=query` dispatches to `scripts/lint-query.sh`. Exit-code escalation matches chart layer.

## 11. Privacy + provenance (Stage 1 minimum)

- Materialized dataset (`--out=<slug>` or Surface B) auto-populates frontmatter:
  - `sources: [<input-slugs>]`
  - `query: <result-hash>`
  - `last_updated: <today>`
- `privacy:` field is **not** auto-set in Stage 1. Author writes it. Q-PRIV stub will enforce floor in Stage 3.
- Inline managed regions inherit the host page's privacy class transitively (no extra metadata).

## 12. Tooling integration

- `justfile` recipes:
  - `query SQL`            → CLI run
  - `query-new SLUG`       → scaffold `type: query` page
  - `query-render`         → render all materialized queries + fences
  - `query-render-one X`   → fast single-target iteration
- `scripts/build.sh`: hook query-render before chart-render (charts may consume materialized datasets in the future).
- `scripts/check-deps.sh`: advisory line `DEPS|advisory|duckdb missing — install for query layer` when data layer is enabled.

## 13. Testing

Mirror existing data/chart test layout (BATS + `node:test`).

### BATS — `tests/query/`
- `test-query-cli.bats`
  - `run` returns rows for valid SQL
  - `run` exit 3 on unknown slug
  - `run` exit 5 on SQL parse error
  - `run` exit 2 when `duckdb` absent (mock PATH)
  - `--out=<slug>` materializes dataset; row count + frontmatter `sources` populated
  - `--out=<slug>` rejects collision without `--force`
- `test-query-page.bats`
  - `query.sh new <slug>` scaffolds query page with required FM
  - render produces `content/datasets/<out-slug>.md` deterministically (run twice = identical)
  - determinism guard rejects `NOW()`, `RANDOM()`, missing outer `ORDER BY`
  - hash sidecar written; second render unchanged → no diff
- `test-query-fence.bats`
  - inline ` ```sql awiki-query id="x"` populates managed region with markdown table
  - re-run idempotent (managed region only)
  - tampered region detected (lint Q5)
  - missing END marker → lint error
- `test-lint-query.bats`
  - Q1 parse, Q2 reference, Q3 determinism, Q4 hash staleness, Q5 region tamper, Q-PRIV stub no-op
- `test-query-deps.bats`
  - `check-deps.sh` advisory line when data layer on + `duckdb` absent

### node:test — `tests/query-node/`
- engine resolver: SQL → slug list (FROM/JOIN regex; quoted identifiers; CTEs; subqueries)
- determinism guard: token coverage + outer-ORDER-BY logic
- managed-region writer: idempotent, byte-identical second pass

### e2e smoke (extend `tests/data-smoke.bats`)
- Init repo → create dataset → create query page → render → hugo build → lint → all green
- Inline fence on a concept page → render → hugo build → managed region present + lint clean

### CI
- Install `duckdb` CLI (brew on macOS, apt on linux runners). Reuse the `vl-convert` install pattern.

Coverage target: each Q-rule has at least one BATS test, matching the chart layer's bar.

## 14. Future stages (out of scope, sketched only)

- **Stage 2** — Inline fence enrichment: `out=<slug>` attribute promotes inline fence into a materialized sibling dataset; charts can then consume it. Reuses Stage 1 engine.
- **Stage 3** — External DB connections: new `type: connection` page kind, `.env` for secrets, `ATTACH` whitelist gated by connection registry. Privacy floor lint goes from stub to enforcing rule. `connection_fingerprint` enters the result hash.

## 15. Risks + mitigations

| Risk | Mitigation |
|------|------------|
| DuckDB CLI version drift changes JSON output | Pin a minimum version in `check-deps.sh`; CI uses pinned version |
| Regex-based SQL ref resolver mis-parses tricky SQL | Test matrix covers known shapes; escalate to `sqlparse` in plan if matrix fails |
| Inline fences cause noisy diffs on every build | Hash sidecar gates re-write — managed region only changes when result hash changes |
| User commits secrets in SQL (e.g., hardcoded URL) | Stage 1 forbids `ATTACH`; secrets surface arrives in Stage 3 with explicit cred model |
| `.cache/duckdb/` size growth | `.gitignore` covers it; doc `just clean` recipe extension to wipe it |

## 16. Open questions deferred to plan (not blocking spec)

- Regex vs `sqlparse` for SQL parsing — pick based on test-matrix pass rate.
- Exact result-hash JSON shape for Surface C sidecar (`.queries.json`) — single key vs sorted.
- Whether to colocate query pages under `content/queries/` (proposed) or under `content/datasets/queries/` — proposing `content/queries/` to avoid dataset orphan-check confusion.
