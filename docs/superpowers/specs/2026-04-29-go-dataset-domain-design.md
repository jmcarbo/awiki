# Go dataset domain design

Date: 2026-04-29

## Goal

Port the awiki dataset domain — `scripts/dataset.sh` (~298 LOC),
`scripts/data-init.sh` (~138 LOC), `scripts/lib/dataset-fm.sh` (~68
LOC) and `scripts/lib/dataset-rows.py` (~136 LOC) — into the `awiki`
Go binary.

Lint rules D1-D9 already live in `internal/lint/data/`; this slice
adds the orchestration verbs (`new`, `compact`, `validate`,
`data-init`) plus shared row helpers, and lifts row-load/coerce code
out of `internal/lint/data/rows.go` into a shared `internal/dataset/`
package the lint rules will consume.

## Non-goals

- Reimplementing externals (`git-crypt`, `age`). The `encryption` step
  of `data-init` stays bash because it requires interactive prompting
  and external CLIs.
- Removing bash files. The `scripts/dataset.sh` and `scripts/data-init.sh`
  shims stay until the cleanup slice.
- Changing user-visible contracts. Records (`DATASET|...`,
  `DATA-INIT|...`), exit codes, file shapes (frontmatter keys, `## Data`
  fence form, file under `data/<slug>.<fmt>`), thresholds.

## Compatibility contract

- Justfile recipes keep their names: `data-init`, `dataset-new`,
  `dataset-compact`, `dataset-validate`.
- Records byte-identical:
  - `DATASET|created <page> (rows=<N>)`
  - `DATASET|validated <slug> (rows=<actual>)`
  - `DATASET|compacted <slug> <inline-to-file|file-to-inline> (<target>, rows=<N>, bytes=<size>)`
  - `DATASET|<slug> already compact (...)`
  - `DATASET|ERROR|...`
  - `DATA-INIT|created <dir>`, `DATA-INIT|skip <dir>`,
    `DATA-INIT|appended <key>=<val>`, `DATA-INIT|restored …`,
    `DATA-INIT|WARN|…`
- Page on-disk shape: frontmatter keys (`title`, `date`, `last_updated`,
  `type: dataset`, `tags`, `storage`, `format`, `rows`, optional
  `columns`, optional `data_path`, `sources`, `draft`); `## Data` fence;
  `## Schema`, `## Provenance`, `## Related`, `## Sources` sections.
- Data file path: `data/<slug>.<fmt>` when `storage=file`.
- Compact thresholds from `.awiki/config`:
  `AWIKI_DATASET_INLINE_MAX_ROWS=500`, `AWIKI_DATASET_INLINE_MAX_BYTES=51200`.
- Exit codes: 0 on success; 1 on user error (missing slug, bad flag,
  missing page, validation failure, threshold breach refusing
  inline).

## Strategy

### CLI shape

Nested per the roadmap:

- `awiki dataset new <slug> [--format=<fmt>] [--from=<path>]`
- `awiki dataset compact <slug>`
- `awiki dataset validate <slug>`
- `awiki data-init` — flat one-shot.

### Sub-slice order

1. **Infra slice**: package skeleton, types, runner, fixture loader.
   Lift row-load/validate/sample helpers out of
   `internal/lint/data/rows.go` into `internal/dataset/rows.go` and
   re-export under `internal/lint/data` via aliases (zero behavior
   change for lint).
2. **`validate`** verb (smallest; reads page, refreshes `rows:`,
   optionally schema-validates).
3. **`compact`** verb (inline ↔ file move).
4. **`new`** verb (scaffold page; optional `--from=<path>` seeds rows).
5. **`data-init`** verb (idempotent dir/config/wiki-md setup; encryption
   step remains bash).

### Externals stay exec

- `git-crypt`, `age` — used by `data-init` encryption step.
  Implementation: keep that step bash; Go invokes the existing helper
  `_step_encryption` via `os/exec` (or simply leaves that one bash
  step as a residual recipe).

### Shared helpers

- `internal/dataset/rows.go` — `LoadRows`, `ValidateRows`, `SampleRows`,
  `CountRows` (lifted from lint).
- `internal/dataset/frontmatter.go` — `FmGet`, `FmSet`, `FmRemove`
  (port of `dataset-fm.sh`). Synth's `FmSetScalar` differs (only
  rewrites; never inserts) so this is a sibling implementation.
- `internal/dataset/fence.go` — `ExtractDataFence`, `ReplaceDataFence`,
  `RemoveDataFence`. Operate on `## Data` section markdown fence.
- `internal/dataset/scaffold.go` — `WriteScaffold` for new pages.
- `internal/dataset/config.go` — `InlineThresholds(repoRoot)` reads
  the two limits from `.awiki/config`.

## Architecture

### Package layout

```
cmd/awiki/main.go
internal/
  cli/dataset.go         # nested-group dispatch
  dataset/
    types.go             # Page, Storage, Format
    runner.go            # Runner with paths, config
    rows.go              # LoadRows / ValidateRows / SampleRows
    rows_test.go
    frontmatter.go       # FmGet/Set/Remove
    frontmatter_test.go
    fence.go             # ## Data fence ops
    fence_test.go
    scaffold.go          # WriteScaffold for `new`
    scaffold_test.go
    config.go            # InlineThresholds
    config_test.go
    new.go
    new_test.go
    compact.go
    compact_test.go
    validate.go
    validate_test.go
  lint/data/             # existing; type-aliases over internal/dataset/rows
internal/cli/init.go     # data-init flat verb
```

### Runner shape

```go
type Runner struct {
    RepoRoot   string
    ContentDir string
    DataDir    string         // <RepoRoot>/data
    Config     map[string]string
    Today      string         // injected for tests
}

// Threshold defaults: 500 rows, 51200 bytes.
func (r *Runner) InlineMaxRows() int { ... }
func (r *Runner) InlineMaxBytes() int { ... }
```

## Domain scope

### `validate`

1. Read `<ContentDir>/datasets/<slug>.md`. Missing → exit 1.
2. Read `storage`, `format`, optional `data_path`, optional `columns`.
3. If `storage=file`: load rows from `<RepoRoot>/<data_path>`.
   If `storage=inline`: load rows from the `## Data` fence.
4. If `columns` declared: schema-validate. Errors → exit 1 plus
   `DATASET|ERROR|<msg>` line per failure.
5. Update `rows:` to actual count via `FmSet`.
6. Stdout: `DATASET|validated <slug> (rows=<N>)`.

### `compact`

1. Read page; honor `storage`, `format`, optional `data_path`.
2. Compute current row count + byte size.
3. If `storage=inline` and (rows > MAX_ROWS or bytes > MAX_BYTES):
   move data to `<DataDir>/<slug>.<fmt>`. Update `storage=file`,
   add `data_path: data/<slug>.<fmt>`. Remove inline fence.
4. If `storage=file` and (rows ≤ MAX_ROWS and bytes ≤ MAX_BYTES):
   inline the file into the `## Data` fence. Set `storage=inline`,
   remove `data_path`, delete the file.
5. If `storage=inline` and over threshold but `--from` not provided
   and rows already external: error.
6. Stdout: `DATASET|compacted <slug> <direction> (<target>, rows=<N>, bytes=<B>)`
   or `DATASET|<slug> already compact (...)`.

### `new`

1. Validate slug pattern.
2. Build frontmatter scaffold.
3. If `--from=<path>` and storage would be inline: read file and embed
   under `## Data` fence (matching format).
4. If `--from=<path>` and exceeds inline threshold: copy file to
   `<DataDir>/<slug>.<fmt>`, set `storage=file`, `data_path=data/<slug>.<fmt>`.
5. Refuse overwrite (exit 1).
6. Write page atomically; emit
   `DATASET|created <page> (rows=<N>)`.

### `data-init`

Five idempotent steps, each printing `DATA-INIT|...`:

1. Create directories (`content/datasets`, `content/queries`,
   `content/charts`, `data`, `assets/charts`, `static/vendor/vega`)
   plus `.gitkeep` markers.
2. Vendor vega bundles via existing `scripts/lib/vendor-vega.sh`
   (call out via `os/exec`).
3. Patch `WIKI.md` `<!-- BEGIN data-layer -->` ... `<!-- END data-layer -->`
   region with the canonical template body. Delegates to existing
   bash helper or ports inline (template in
   `scripts/templates/wiki-data-layer.md`).
4. Append `.awiki/config` defaults (only if missing).
5. Encryption step **stays bash** — Go invokes the bash function via
   shell-out (or this step is preserved by leaving the user to run
   `bash scripts/data-init.sh` for the interactive prompt and Go
   short-circuits to skip it). Decision: Go runs steps 1-4; emits a
   one-line note pointing to `bash scripts/data-init.sh` for step 5
   the first time it's needed.

## Parity oracle

- Golden fixtures per verb under `tests/fixtures/dataset/<verb>/{input,
  args, expected_stdout, expected_files}`.
- `data-init` smoke: run on a fresh tempdir; assert created dirs,
  config keys, gitkeep markers; do NOT assert vega vendor download
  (network-dependent — skip when offline).
- Live wiki diff for each CRUD verb against
  `AWIKI_DATASET_LEGACY=1 bash scripts/dataset.sh ...`.

## Risks

- **dataset-rows.py format detection.** Python uses `csv.Sniffer`-
  style heuristic for DSV. Go must reproduce: try `,`, `;`, `|`,
  `\t` and pick the one with the most consistent column count
  across the first 5 rows. Mitigation: golden fixtures with each
  format.
- **JSON sample type fallback.** Python uses
  `json.dumps(..., default=str)`. Go: pre-coerce non-string scalar
  values to their string form before marshalling.
- **Threshold check ordering.** Bash compares rows BEFORE bytes. Go
  must match.
- **WIKI.md region patch.** The template is large; Go must use
  `internal/region.ManagedReplace` (already shipped).
- **Encryption step.** Cannot port without breaking interactive UX.
  Documented as a deferred recipe.

## Cleanup criteria

Standard. Per-verb sub-slice flips a verb in
`AWIKI_DATASET_GO_VERBS` analogous to synth's pattern.

## Open questions

- Encryption step disposition: keep as bash forever, or port with a
  non-interactive `--no-prompt` mode? Default: keep bash; future plan
  may add `--no-prompt` for CI.
- Shared `Frontmatter` package across synth + dataset domains? Synth
  uses bare scalar setter; dataset needs insert-if-missing. For now,
  separate implementations live in their domain packages. Potential
  future refactor: extract to `internal/frontmatter/` once a third
  consumer arrives.

## Out of scope

- Cleanup slice (delete bash + python).
- Chart and query domains — separate specs.
- MCP dataset handlers — separate binary.
