# Go awiki lint core design

Date: 2026-04-28

## Goal

Replace the awiki bash/Python/JavaScript command glue incrementally with one Go
executable, starting with the `lint` workflow. The first implementation slice
ships `awiki lint` as a strict-compatibility core lint engine while preserving
external domain tools behind explicit adapters.

This is a workflow-slice migration: ingest, synthesis, data, chart, task, build,
and template-update behavior will move later as separate verified slices. The
first slice focuses on the core wiki lint rules because every other workflow can
use them as a gate.

## Non-goals

- Reimplement `git`, `hugo`, `qmd`, `vl-convert`, `git-crypt`, `age`, or other
  domain tools inside Go.
- Port synth `S*`, data `D*`, chart `C*`, or task `T*` lint rules in the first
  slice.
- Change user-facing lint output, exit codes, or tested behavior as part of the
  first port.
- Remove `just` recipes immediately. They can remain as compatibility facades.

## Compatibility Contract

The first `awiki lint` slice is strict-compatible with the existing
`scripts/lint.sh` behavior where covered by tests and documented wiki rules.

Required compatibility:

- Flags: `--fix`, `--only=<name>`, `--file=<path>`, `--hugo-check`,
  `--alias-build-only`, and optional positional content directory.
- Records: `LINT|<level>|<file>|<message>` and `FIX|<file>|<message>`.
- Summary and exit behavior: exit `0` for clean lint, exit `2` when errors are
  emitted.
- Existing `just lint` and `just lint-fix` remain valid entrypoints.
- Existing Bats tests for core lint become the migration oracle.

Compatibility may be implemented with a temporary shell shim:

```text
just lint -> scripts/lint.sh -> awiki lint
```

or by updating `justfile` directly after tests prove the Go executable. During
the transition, compatibility matters more than command-surface cleanup.

## Architecture

The executable is named `awiki`.

Proposed package layout:

- `cmd/awiki` owns `main`, command registration, process exit mapping.
- `internal/cli` parses flags and dispatches subcommands.
- `internal/wiki` discovers pages, parses frontmatter, extracts wikilinks, and
  builds slug/alias maps.
- `internal/lint` runs rules, applies mechanical fixes, records diagnostics, and
  emits summaries.
- `internal/adapters` wraps external tools and legacy namespace lint during the
  migration.

The core lint engine should avoid shelling out for file traversal, markdown
inspection, YAML-ish frontmatter parsing, wikilink extraction, and fix
application. Those are awiki rules and should live in Go. External adapters are
reserved for real external engines such as Hugo and for temporary delegation to
legacy lint namespaces.

## Core Lint Scope

The first slice ports these rules and behaviors into Go:

- Markdown page discovery under the selected content directory.
- Frontmatter parsing for required wiki fields and page `type`.
- Slug uniqueness, with `_index.md` exempt from duplicate-slug errors.
- Alias map construction and unique alias resolution.
- Wikilink extraction from page bodies and broken-link diagnostics.
- Empty-page warnings.
- Orphan detection with system-page exemptions such as `section-index`.
- Catalog coverage warnings.
- Privacy warning when `tags: [private]` appears outside a private path.
- `--fix` insertion of `last_updated: <today>` immediately after `date:` when
  `last_updated:` is missing.
- No-op `--fix` behavior for pages without a `date:` anchor.
- `--hugo-check` through an adapter that invokes Hugo and converts failure into
  the existing lint record shape.
- `--alias-build-only` through the existing map-building path until map building
  is fully owned by Go.

## Deferred Namespace Lint

Feature-specific lint remains delegated during this slice:

- Synth `S*` rules and synth marker normalization.
- Data `D*` rules.
- Chart `C*` rules and `vl-convert` sidecar checks.
- Task `T*` rules and action grammar checks.

`awiki lint` should keep a delegation point for these namespaces so
`tests/lint_synth_test.bats` and related suites continue to pass while each
namespace is ported later. The long-term shape is separate Go rule packages
under `internal/lint/synth`, `internal/lint/data`, `internal/lint/chart`, and
`internal/lint/task`.

## Data Flow

1. Parse CLI flags into a `LintOptions` value.
2. If `--alias-build-only` is set, run the existing map-build adapter and exit
   with its status.
3. If `--only` targets a deferred namespace, call the legacy adapter.
4. Discover markdown pages from the selected content directory or `--file`.
5. Parse frontmatter and bodies into page records.
6. Build slug, alias, title, and inbound-link indexes.
7. Apply mechanical fixes when `--fix` is enabled.
8. Run core rules and collect diagnostics.
9. Run `--hugo-check` adapter when requested.
10. Emit records and summary in compatibility format.
11. Exit `2` when any error-level diagnostic exists, otherwise `0`.

## Error Handling

The Go command should fail closed for invalid CLI usage and unreadable required
inputs, but preserve lint-style diagnostics for invalid wiki content.

- Invalid flags or missing positional values: print a concise usage error and
  exit nonzero.
- Unreadable content directory: emit an error diagnostic and exit `2`.
- Malformed frontmatter: emit an error diagnostic for that page and continue
  scanning other pages.
- External adapter failure: map the failure to the existing lint record format
  where current behavior does so, especially for Hugo checks.
- Fix failures: emit an error diagnostic rather than partially rewriting files.

## Testing

Verification is existing-test driven.

Primary gate:

```sh
bats tests/lint_test.bats
```

Regression gate for delegated namespace behavior:

```sh
bats tests/lint_synth_test.bats
```

Expected additional Go tests:

- Frontmatter parser cases for required fields, arrays, and malformed blocks.
- Wikilink parser cases for `[[slug]]`, `[[slug|display]]`, aliases, and broken
  links.
- Slug/alias index tests, including `_index.md` duplicate exemption.
- Fix application tests for `last_updated:` insertion and no-op behavior without
  `date:`.
- Diagnostic summary and exit-code tests.

The first slice is complete only when existing lint tests pass through the Go
path and namespace-delegation tests still pass.

## Migration Sequence

1. Add Go module and `awiki lint` command skeleton.
2. Implement diagnostics, summary, and exit-code compatibility.
3. Port page discovery, frontmatter parsing, wikilink extraction, and indexes.
4. Port the core lint rules.
5. Port `--fix` for `last_updated:`.
6. Add adapters for `--hugo-check`, `--alias-build-only`, and deferred
   namespace lint.
7. Wire `scripts/lint.sh` or `justfile` to the Go executable once tests pass.
8. Keep scripts available until every lint namespace has moved to Go.

## Open Boundaries

The first slice intentionally leaves these boundaries explicit:

- `hugo` remains an external dependency for render checks.
- `qmd` remains an external search/index engine until the query workflow gets
  its own slice.
- Legacy synth/data/chart/task lint scripts remain callable until those
  namespaces are ported.
- The command name is `awiki`, but distribution and installer work are deferred
  until at least one complete workflow is running through Go.
