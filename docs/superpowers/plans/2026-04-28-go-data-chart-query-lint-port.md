# Go Data Chart Query Lint Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace data, chart, and query lint shell/Python glue with Go-owned namespace packages while keeping true execution/render engines external.

**Architecture:** Add `internal/lint/data`, `internal/lint/chart`, and `internal/lint/query`. Data and chart can be ported directly from tested shell behavior; query starts with a contract reconciliation commit because `WIKI.md`, current tests, and `scripts/lint-query.sh` disagree on Q-code meanings.

**Tech Stack:** Go, Bats, CSV/TSV/JSON parsers, Vega-Lite JSON inspection, bounded external adapters for DuckDB/query hashing and vl-convert renderability checks when required.

---

## File Structure

- Create `internal/lint/data`: D1-D9 dataset lint and row validation.
- Create `internal/lint/chart`: C1-C9 and C-PRIV chart lint.
- Create `internal/lint/query`: Q1-Q5 and Q-PRIV query lint after contract reconciliation.
- Modify `internal/lint/engine.go`: register namespaces and remove legacy fallbacks one namespace at a time.
- Modify `WIKI.md` and `tests/lint_query_test.bats` together if the query Q-code contract changes.

## Task 1: Data D1-D5

- [ ] Add failing Go tests for missing `storage`, missing `format`, missing file data path, missing inline fence, mismatched inline fence format, and row schema validation.
- [ ] Implement dataset frontmatter parsing and inline/file data extraction.
- [ ] Implement CSV, TSV, DSV, JSON, and TopoJSON structural validation in Go.
- [ ] Run `go test ./internal/lint/data -count=1`.
- [ ] Commit `feat: port core dataset lint`.

## Task 2: Data D6-D9 and Fixes

- [ ] Add tests for inline threshold warnings, row-count mismatch, `--fix` row update, data path outside `data/`, and missing sources info.
- [ ] Implement D6-D9 and row-count fix behavior.
- [ ] Run `go test ./...`, `bats tests/lint_data_test.bats`, `bats tests/dataset_rows_test.bats`, and `bats tests/dataset_validate_test.bats`.
- [ ] Remove data from `deferredNamespaces()`.
- [ ] Commit `feat: run data lint in go`.

## Task 3: Chart C1-C7

- [ ] Add tests for malformed Vega-Lite JSON, missing visual keys, resolver failures, field-not-in-columns, missing chart fence, `chart_data` mismatch, `--fix` chart_data update, and missing SVG sidecar warnings.
- [ ] Implement chart fence extraction and JSON inspection.
- [ ] Port lint-only resolver behavior from `scripts/lib/vl-resolve.py`.
- [ ] Run `go test ./internal/lint/chart -count=1`.
- [ ] Commit `feat: port chart structural lint`.

## Task 4: Chart C8/C9/C-PRIV

- [ ] Add tests for high chart reference counts, hand-edited chart-preview regions, and non-private charts referencing private datasets.
- [ ] Implement aggregate C8, C9 managed-region warnings, and blocking C-PRIV diagnostics.
- [ ] Run `go test ./...`, `bats tests/lint_chart_test.bats`, `bats tests/vl_resolve_test.bats`, and `bats tests/hugo_render_chart_test.bats`.
- [ ] Remove chart from `deferredNamespaces()`.
- [ ] Commit `feat: run chart lint in go`.

## Task 5: Query Contract Reconciliation

- [ ] Add a documentation/test commit that chooses one Q-code contract.
- [ ] If preserving current shell behavior, update `WIKI.md` so Q1 means missing SQL fence, Q2 unknown referenced dataset, Q3 nondeterministic SQL, Q4 stale sidecar, Q5 managed-region tamper, and Q-PRIV advisory stub.
- [ ] If preserving current WIKI behavior, update `tests/lint_query_test.bats` and implementation expectations in the same commit.
- [ ] Run `bats tests/lint_query_test.bats`.
- [ ] Commit `docs: reconcile query lint contract`.

## Task 6: Query Q1-Q5/Q-PRIV

- [ ] Add tests for the reconciled Q1-Q5 contract and Q-PRIV stage behavior.
- [ ] Implement SQL fence parsing, source slug extraction, determinism checks, sidecar hash checks, and managed-region hash checks in Go.
- [ ] Keep DuckDB execution and full query rendering in external adapters only when a lint rule requires them.
- [ ] Run `go test ./...`, `bats tests/lint_query_test.bats`, `bats tests/query_cli_test.bats`, `bats tests/query_engine_test.bats`, and `node --test tests/query_node/*.test.mjs`.
- [ ] Remove query from `deferredNamespaces()`.
- [ ] Commit `feat: run query lint in go`.
