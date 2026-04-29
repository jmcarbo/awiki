# Go query domain design

Date: 2026-04-29

## Goal

Port `scripts/query.sh` plus python helpers
(`scripts/lib/query-engine.sh`, `query-extract.py`, `query-format.py`,
`query-resolve.py`, `query-determinism.py`) into the Go binary under
`awiki query {run, new, render, render-one, fence-render}`.
DuckDB stays exec.

## Non-goals

- Reimplementing DuckDB (CGO-heavy). Stays exec via `adapters.DuckDB`.
- Removing bash. Per-verb shim flip; cleanup gated on release.
- Changing user-visible record formats (`QUERY|...`) or output table
  shapes.

## Compatibility contract

- Justfile recipes: `query`, `query-new`, `query-render`,
  `query-render-one`, `query-fence-render`. Names + arg shapes
  preserved.
- Records: `QUERY|created`, `QUERY|materialized`, `QUERY|skip`,
  `QUERY|fence-render`, `QUERY|ERROR|...` byte-identical.
- File shapes: type:query pages with `query:` SQL frontmatter and
  optional `out:` materialization target. Inline `awiki-query`
  fences in any markdown.

## Architecture

```
internal/
  cli/query.go
  query/
    types.go               # QueryPage, Result, FenceSpec
    runner.go              # Runner with DuckDB adapter
    sql.go                 # template substitution: dataset/page refs
    sql_test.go
    determinism.go         # canonical sort + hash for caching
    determinism_test.go
    extract.go             # awiki-query fence parsing
    extract_test.go
    format.go              # result → markdown table
    format_test.go
    run.go                 # run verb (ad-hoc SQL)
    new.go                 # type:query scaffold + materialize target
    render.go              # rerun every materialized query
    render_one.go
    fence_render.go        # rerun every awiki-query fence in content/
```

## Domain scope (verbs)

- `awiki query run <SQL...> [--out=<slug>]` — execute SQL via DuckDB,
  print rows. With `--out`, materialize to a sibling type:dataset page.
- `awiki query new <slug> [--from=<sql>]` — scaffold type:query page.
- `awiki query render` — re-run every `type:query` page; refresh
  materialization target dataset.
- `awiki query render-one <slug>` — single-page regen.
- `awiki query fence-render` — re-run every inline `awiki-query` fence.

## External boundaries

- `adapters.DuckDB.Run(sql, format)` shells `duckdb -csv -c "<sql>"`.
- All template/canonicalization/hash logic ports to Go.

## Risks

- **DuckDB CSV escaping.** Output may need CSV-quote-stripping. Mirror
  the bash `query-format.py` pipeline.
- **Determinism across runs.** Bash `query-determinism.py` sorts rows
  by canonical key; replicate exactly.
- **Page wikilink rewriting.** SQL may reference `[[slug]]` resolved
  via the dataset registry. Mirror `query-resolve.py`.

## Out of scope

- DuckDB SQL grammar. Rely on DuckDB's parser.
- Optimization (parallel render).
- MCP query handlers.
