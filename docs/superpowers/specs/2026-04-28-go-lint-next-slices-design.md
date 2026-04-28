# Go lint next slices design

Date: 2026-04-28

## Goal

Complete the next lint migration after the Go core lint engine. The end state is
that `awiki lint` owns all wiki lint namespaces in Go, with shell/Python scripts
kept only as temporary compatibility adapters or as wrappers around real
external engines.

This spec covers four implementation slices:

1. Hugo-check cleanup.
2. Synth lint port.
3. Task lint port.
4. Data, chart, and query lint port.

The work should preserve the `WIKI.md` lint contract: agents run `just lint`,
read structured `LINT|...` output, group issues by theme, fix approved groups,
and log lint activity after changes.

## Non-goals

- Reimplement `hugo`, `duckdb`, `qmd`, `vl-convert`, `git`, `git-crypt`,
  `age`, or OS service managers in Go.
- Change the documented meaning of synth `S*`, task `T*`, data `D*`, chart
  `C*`, or query `Q*` rules except where this spec calls out a compatibility
  bug.
- Port ingest, build, template-update, dataset creation, chart rendering,
  query execution, or action capture workflows.
- Remove `just` recipes. They remain the user-facing workflow facade.
- Remove legacy scripts before every affected Bats suite is green through the
  Go path.

## Current State

The repository now has an `awiki` Go executable with these packages:

- `cmd/awiki` for process entry.
- `internal/cli` for command parsing.
- `internal/wiki` for page discovery, frontmatter, wikilinks, and indexes.
- `internal/lint` for core rules, diagnostics, fixes, and legacy delegation.
- `internal/adapters` for external commands.

`scripts/lint.sh` is a compatibility shim. When `bin/awiki` exists, it delegates
to `awiki lint`; `AWIKI_LINT_LEGACY=1` forces the old shell path. The Go core
already owns mechanical wiki lint and delegates deferred namespaces:

- `synth` to `scripts/lint-synth.sh`.
- `task` to task rules still in `scripts/lint.sh`.
- `data` to `scripts/lint-data.sh`.
- `chart` to `scripts/lint-chart.sh`.
- `query` to `scripts/lint-query.sh`.

The known post-merge gap is `tests/lint_test.bats` Hugo-check coverage. One
test fails because current root content emits warning diagnostics, so a
Hugo-specific render check is coupled to whole-wiki warning exit semantics.

## Compatibility Contract

The migrated Go lint command must continue to support:

- `just lint` and `just lint-fix`.
- `awiki lint [--fix] [--only=<namespace>] [--file=<path>] [--hugo-check]`.
- `--alias-build-only` until alias-map consumers no longer need it.
- Optional content directory positional argument.
- Records in both core and namespace shapes:
  - `LINT|<LEVEL>|<file>|<message>`
  - `LINT|<LEVEL>|<file>|<rule-code>|<message>`
  - `FIX|<file>|<message>`
- Exit `2` when error-level diagnostics exist.
- Exit `1` when only warning-level diagnostics exist.
- Exit `0` when no errors or warnings exist.

Go diagnostics should add an optional `Code` field rather than embedding rule
codes into message strings. Importers and emitters must accept both four-field
and five-field `LINT` records so existing scripts and tests remain compatible
during migration.

## Architecture

Extend `internal/lint` from a core-rules package into a namespace runner:

```text
internal/lint
  diagnostics, options, engine, registry
internal/lint/core
  current page/frontmatter/wikilink/catalog/privacy rules
internal/lint/synth
  S1-S9
internal/lint/task
  T1-T15
internal/lint/data
  D1-D9
internal/lint/chart
  C1-C9, C-PRIV
internal/lint/query
  Q1-Q5, Q-PRIV
```

Each namespace should expose a small rule-set boundary:

```go
type RuleSet interface {
    Name() string
    Run(context.Context, Options, *wiki.Index) (Collector, error)
}

type Fixer interface {
    Fix(context.Context, Options, *wiki.Index) (Collector, error)
}
```

The exact interface may evolve during implementation, but the boundary should
stay clear: namespace packages inspect parsed wiki state and emit diagnostics;
they should not own CLI parsing, process exits, or raw stdout formatting.

External execution remains centralized in `internal/adapters`. Namespace code
may call adapters only for explicit external engines or temporary compatibility
bridges, not for file traversal, Markdown parsing, YAML-like frontmatter
inspection, date math, rule-code formatting, or string normalization.

## Slice 1: Hugo-Check Cleanup

Purpose: restore a fully green core lint gate before deeper namespace ports.

The failing Hugo Bats should be fixed by separating renderability from
whole-wiki warning state. Normal lint should keep warning exit behavior; the
Hugo-specific test should not fail just because unrelated content has warning
diagnostics.

Implementation direction:

- Keep `Collector.ExitCode` semantics: errors return `2`, warnings return `1`.
- Keep `--hugo-check` as an adapter around Hugo.
- Update the Bats fixture or test invocation so the Hugo-check case runs
  against isolated clean content when it is asserting Hugo render success.
- Add Go tests that prove `runHugoCheck` maps missing Hugo to info, render
  failure to error, and success to no diagnostic.
- Do not suppress normal warnings globally to make this test pass.

Completion gate:

```sh
go test ./...
bats tests/lint_test.bats
```

## Slice 2: Synth Lint Port

Purpose: move synthesis rule enforcement from shell/Python helper scripts into
Go while preserving the `WIKI.md` S1-S9 contract.

Rules to port:

- S1 marker integrity.
- S2 required sections from plugin manifests.
- S3 evidence quote substring validation.
- S4 citation-in-scope validation.
- S5 scope drift warnings, with query-scoped pages exempt.
- S6 generated-region hand-edit warnings.
- S7 feedback-count metric, including warning when the count exceeds the
  documented threshold.
- S8 feedback-scope validation for wikilinks in `## Feedback` bullets.
- S9 aggregate evidence word cap.

The current helper behavior should become Go-owned, including Unicode NFC
normalization, zero-width stripping, hyphen-variant collapse, smart-quote
normalization, NBSP handling, whitespace collapse, wikilink-to-title rewrites,
fuzzy quote suggestions, scope hashing, and generated-region diff checks.

Suggested sub-slices:

1. Port marker parsing, generated-region extraction, required-section checks,
   and evidence-line parsing.
2. Port normalization and exact quote matching.
3. Port wikilink-to-title rewrites and fuzzy quote suggestions.
4. Port scope resolution and scope hashing.
5. Port generated-region diff checks and S9 word caps.
6. Port `--only=synth --fix` normalization so it edits only generated regions.

Privacy and safety requirements:

- Keep synth privacy fail-closed behavior for private sources feeding
  non-private targets.
- Keep plugin post-hooks out of lint execution.
- Keep marker repair and missing-section repair manual; `--fix` only performs
  documented safe normalization.

Completion gate:

```sh
go test ./...
bats tests/lint_synth_test.bats
bats tests/lint_synth_fuzzy_test.bats
bats tests/synth_e2e_test.bats
```

The legacy synth adapter can be removed only after these gates pass through the
Go implementation.

## Slice 3: Task Lint Port

Purpose: move task layer T1-T15 diagnostics into Go without changing task
workflow behavior.

The task-lint port must not trust stale shell-generated task maps as its only
source of truth. Lint should build the action index it needs from current page
content in Go, then compare or write map artifacts only where compatibility
requires them:

- `.awiki/maps/actions.tsv`
- `.awiki/maps/actions-rejected.tsv`

This means the lint slice includes the action grammar parser needed for T-rule
evaluation, but still keeps action capture, recurrence emission, agenda
generation, and standalone `action-scan.sh` command migration out of scope.

Rules to port:

- T1-T15 exactly as documented by `WIKI.md` and the task-layer tests.
- Action ID shape and uniqueness.
- Rejected-action surfacing.
- Context/project alias resolution.
- Managed-region integrity.
- Agenda action consistency.
- Review-date and stale-date warnings.
- Continuation-line and chained-action edge cases.
- Map freshness: if compatibility maps are present and disagree with the
  current in-memory scan, lint must refresh them under `--fix` or emit a clear
  diagnostic instead of silently using stale data.

Go should own date parsing and comparisons. The port must not depend on BSD or
GNU `date` behavior.

Fix behavior should remain conservative:

- Port only fixes already proven safe by tests.
- Skip fixes when action lines involve continuations or ambiguous grammar.
- Emit diagnostics instead of rewriting when the intended target is ambiguous.

Completion gate:

```sh
go test ./...
bats tests/lint_task_test.bats
bats tests/action_grammar_test.bats
bats tests/action_scan_test.bats
bats tests/agenda_test.bats
bats tests/review_status_test.bats
```

The action scanner itself can remain shell-backed until a later workflow slice.

## Slice 4: Data, Chart, and Query Lint Port

Purpose: move the remaining data-oriented lint namespaces into Go while keeping
actual data/query/render engines external.

These namespaces should be ported after synth and task because they share more
external-tool boundaries and fixture-heavy behavior.

### Data

Move `D1-D9` from `scripts/lint-data.sh` into `internal/lint/data`.

Go should own:

- Dataset frontmatter parsing and validation.
- Inline/file storage checks.
- Declared path and slug checks.
- Row-count and schema sanity checks that can be done with standard parsers.
- CSV, TSV, and JSON structural validation currently covered by
  `scripts/lib/dataset-rows.py`.

External engines remain out of scope. Dataset creation and compaction commands
stay in their current scripts until a separate workflow migration.

Gate:

```sh
go test ./...
bats tests/lint_data_test.bats
bats tests/dataset_rows_test.bats
bats tests/dataset_validate_test.bats
```

### Chart

Move `C1-C9` and `C-PRIV` from `scripts/lint-chart.sh` into
`internal/lint/chart`.

Go should own:

- Chart page frontmatter checks.
- Vega-Lite JSON syntax checks.
- Dataset reference resolution.
- Chart sidecar path checks.
- Privacy diagnostics for charts built from private data.
- Resolver behavior currently covered by `scripts/lib/vl-resolve.py` where it
  is lint-only.

`vl-convert` remains an external rendering engine and should stay behind an
adapter when lint needs to check renderability.

Gate:

```sh
go test ./...
bats tests/lint_chart_test.bats
bats tests/vl_resolve_test.bats
bats tests/hugo_render_chart_test.bats
```

### Query

Move `Q1-Q5` and `Q-PRIV` from `scripts/lint-query.sh` into
`internal/lint/query`.

Go should own:

- Query page frontmatter checks.
- SQL fence and output fence structural checks.
- Source slug extraction sufficient for lint diagnostics.
- Determinism rule checks currently covered by query lint.
- Privacy diagnostics when private datasets or private sources feed
  non-private query pages.

DuckDB execution, qmd indexing, and any full query-engine run remain external.
If a lint rule genuinely needs to execute SQL, it must do so through an
explicit adapter with bounded input and clear failure diagnostics.

Before porting query lint, reconcile the current source-of-truth mismatch:
`WIKI.md` describes Q1-Q5 as frontmatter, output freshness, source existence,
determinism, and managed-region checks, while the current shell linter and Bats
tests use Q1 for SQL-fence presence, Q2 for unknown referenced datasets, Q3 for
determinism, Q4 for sidecar staleness, and Q5 for managed-region tamper checks.
The implementation plan must choose one contract, update the other docs/tests in
the same slice, and preserve compatibility messages where existing tests depend
on them.

Q-PRIV also needs an explicit stage decision. Current behavior is a no-op stub;
the WIKI describes an advisory warning today and mandatory privacy floor later.
This Go lint slice should implement the documented advisory warning only if the
tests and WIKI are updated together. It must not silently turn Q-PRIV into a
hard error in the lint port.

Gate:

```sh
go test ./...
bats tests/lint_query_test.bats
bats tests/query_cli_test.bats
bats tests/query_engine_test.bats
node --test tests/query_node/*.test.mjs
```

## Migration Sequence

Recommended order:

1. Fix Hugo-check test isolation and add Go adapter tests.
2. Add optional diagnostic rule codes and five-field record import/export.
3. Add the lint namespace registry while leaving all deferred adapters in place.
4. Port synth S1/S2/S7/S8 simple structural checks.
5. Port synth S3/S4/S5/S6/S9 hard checks and synth `--fix`.
6. Remove synth legacy delegation after synth gates pass.
7. Port task T1-T7/T14/T15 structural and map-backed checks.
8. Port task T8-T13 date/review checks and safe fixes.
9. Remove task legacy delegation after task gates pass.
10. Port data D1-D9 and remove data legacy delegation.
11. Port chart C1-C9/C-PRIV and remove chart legacy delegation.
12. Port query Q1-Q5/Q-PRIV and remove query legacy delegation.
13. Run the full lint namespace gate.
14. Keep `AWIKI_LINT_LEGACY=1` for one compatibility cycle if scripts remain
    in-tree; otherwise remove the shim and document the Go-only lint path.

Full lint namespace gate:

```sh
go test ./...
bats tests/lint_test.bats
bats tests/lint_synth_test.bats
bats tests/lint_synth_fuzzy_test.bats
bats tests/lint_task_test.bats
bats tests/lint_data_test.bats
bats tests/lint_chart_test.bats
bats tests/lint_query_test.bats
```

## Error Handling

Lint should fail closed for schema and privacy risks:

- Private-source declassification remains an error unless the documented
  workflow explicitly allows it.
- Query/chart pages that expose private data outside private paths emit the
  namespace privacy diagnostics required by the current documented stage:
  chart privacy remains blocking, query privacy remains advisory until the
  Stage 3 privacy floor is implemented.
- Malformed frontmatter, broken generated markers, invalid manifests, invalid
  JSON, invalid SQL fences, and unreadable referenced files emit diagnostics
  rather than panicking or aborting the whole run.
- External adapter failures become lint diagnostics when the current shell lint
  behavior treats them as content problems.
- Invalid CLI usage remains a CLI error rather than a lint record.

## Testing Strategy

Use existing Bats tests as behavioral oracles and add Go unit tests around the
logic that was previously hidden inside shell or Python helpers.

Expected Go test focus:

- Diagnostic record compatibility, including optional rule codes.
- Namespace registry dispatch and `--only` filtering.
- Synth generated-region parsing, quote normalization, scope hashing, and word
  caps.
- Task map parsing, date parsing, alias resolution, and safe-fix refusal.
- Data frontmatter parsing and row validation.
- Chart JSON parsing, dataset reference resolution, and privacy checks.
- Query fence parsing, source extraction, determinism checks, and privacy
  checks.

Each namespace should be deleted from `deferredNamespaces()` only in the same
commit that proves its Bats suite passes through the Go path.

## Acceptance Criteria

The migration is complete when:

- `scripts/lint.sh` no longer needs to delegate lint namespaces to shell or
  Python scripts during normal operation.
- `awiki lint --only=synth`, `--only=task`, `--only=data`, `--only=chart`, and
  `--only=query` are Go-owned.
- Existing lint Bats suites pass without `AWIKI_LINT_LEGACY=1`.
- `just lint` remains the documented user command.
- Privacy checks remain fail-closed.
- Real external engines are still explicit adapters, not hidden shell
  dependencies.
