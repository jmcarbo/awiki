# Go query domain port Implementation Plan

> Use superpowers:subagent-driven-development.

**Goal:** port `awiki query {run, new, render, render-one, fence-render}`
per `docs/superpowers/specs/2026-04-29-go-query-domain-design.md`.
DuckDB stays exec.

**Bash oracle:** `scripts/query.sh` + `scripts/lib/query-engine.sh`,
`query-extract.py`, `query-format.py`, `query-resolve.py`,
`query-determinism.py`. Total ~667 LOC.

**Branch:** `feat/query-domain` in `.worktrees/query-domain`.

## Tasks

### Task 1: DuckDB adapter + skeleton

- Create `internal/adapters/duckdb.go` — `ExecDuckDB.Run(ctx, sql, format string)` shells `duckdb -<format> -c "<sql>"` (or `duckdb -json` etc.). Test: compile-time conformance (existing `DuckDB` interface in `interfaces.go` has the right shape).
- Create `internal/query/{types,runner}.go`. `Runner{RepoRoot, ContentDir, DataDir, DuckDB adapters.DuckDB, Today string}`. Types: `QueryPage{Slug, SQL, OutSlug string}`, `QueryResult{Columns []string, Rows [][]string}`.
- Commit: `feat: add query adapter and runner skeleton`.

### Task 2: helpers (extract + format + resolve + determinism)

Port the four python helpers verbatim into `internal/query/`:

- `extract.go` — port `query-extract.py`: parse `awiki-query` fences from markdown. `ExtractFences(text string) []FenceSpec` with FenceSpec carrying `SQL string` plus optional `Out string` directive on the fence info-line (e.g. ` ```awiki-query out=mydataset `).
- `format.go` — port `query-format.py`: `FormatTable(result QueryResult) string` returns markdown-table representation of rows.
- `resolve.go` — port `query-resolve.py`: rewrite `[[<slug>]]` references in SQL into DuckDB-readable file paths (e.g. `read_csv_auto('data/<slug>.csv')`) by reading dataset frontmatter via `internal/dataset.FmGet`.
- `determinism.go` — port `query-determinism.py`: canonicalize result (sort by all columns, normalize whitespace/types) and hash; used to detect stale materialization.
- Tests per file mirroring the python edge cases.
- Commit: `feat: add query helpers (extract, format, resolve, determinism)`.

### Task 3: run verb (ad-hoc SQL)

- Create `internal/query/run.go` — `Runner.Run(ctx, sql string, outSlug string, stdout io.Writer) error`. Resolve wikilinks; exec DuckDB; format result; emit to stdout (or materialize to a sibling type:dataset page if `outSlug != ""`).
- CLI dispatcher `internal/cli/query.go` with case "run".
- Wire `case "query"` in `internal/cli/cli.go`. Shim head in `scripts/query.sh` with `AWIKI_QUERY_GO_VERBS=(run)`.
- Tests: simple SELECT against fake DuckDB; with-out materializes a dataset page.
- Commit: `feat: port query run verb`.

### Task 4: new verb

- `Runner.New(slug string, fromSQL string, stdout io.Writer) error`. Scaffold type:query page with frontmatter + SQL fence.
- Tests: golden scaffold; refuse overwrite.
- Add `new` to GO_VERBS.
- Commit: `feat: port query new verb`.

### Task 5: render + render-one

- `Runner.Render(ctx)` walks all type:query pages; for each: resolve, exec DuckDB, determinism-hash, skip if unchanged, materialize sibling dataset page on change. Emit `QUERY|materialized <slug>` / `QUERY|skip <slug>` records.
- `Runner.RenderOne(ctx, slug string)` — single-page variant.
- Tests + GO_VERBS additions.
- Commit: `feat: port query render and render-one verbs`.

### Task 6: fence-render verb

- `Runner.FenceRender(ctx)` walks `<ContentDir>` for any markdown with `awiki-query` fences; replaces fence body with rendered table via managed-region (use `internal/region.ManagedReplace` with `kind="awiki-query"`, `id=<fence-hash>`).
- Tests + GO_VERBS=(run new render render-one fence-render).
- Commit: `feat: port query fence-render verb`.

### Task 7: regression + merge

- `go test -race ./...` clean
- `just test` pass-rate maintained
- Merge `feat/query-domain` → main

## Self-review

- DuckDB stays exec; Go never embeds.
- Determinism hash matches `query-determinism.py` output (rich golden fixture).
- Wikilink rewrite resolves both `inline` and `file` storage.
- Verbs flip in shim incrementally.
