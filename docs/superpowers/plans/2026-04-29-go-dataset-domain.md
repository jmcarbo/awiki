# Go dataset domain port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development.

**Goal:** Port `awiki dataset {new, compact, validate}` plus
`awiki data-init` flat verb. Lift row-load helpers out of
`internal/lint/data/rows.go` so the new `internal/dataset/` package
hosts them, with the lint package consuming via aliases.

**Spec:** `docs/superpowers/specs/2026-04-29-go-dataset-domain-design.md`

**Bash oracle:** `scripts/dataset.sh`, `scripts/data-init.sh`,
`scripts/lib/dataset-fm.sh`, `scripts/lib/dataset-rows.py`

**Branch:** `feat/dataset-domain` in `.worktrees/dataset-domain`.

## Tasks

### Task 1: package skeleton + lift rows + frontmatter helpers

**Files:**
- Create: `internal/dataset/types.go`, `runner.go`, `rows.go`, `rows_test.go`,
  `frontmatter.go`, `frontmatter_test.go`, `fence.go`, `fence_test.go`,
  `config.go`, `config_test.go`
- Modify: `internal/lint/data/rows.go` — alias the moved helpers

The bash row-load lives in `scripts/lib/dataset-rows.py`. The Go port
already exists in `internal/lint/data/rows.go`. Move the row-load
functions into `internal/dataset/rows.go` as exported types and have
`internal/lint/data/rows.go` import + re-export under their existing
names.

`internal/dataset/types.go`:

```go
package dataset

type Storage string

const (
    StorageInline Storage = "inline"
    StorageFile   Storage = "file"
)

type Format string

const (
    FormatCSV      Format = "csv"
    FormatTSV      Format = "tsv"
    FormatDSV      Format = "dsv"
    FormatJSON     Format = "json"
    FormatTopoJSON Format = "topojson"
)

// Page describes a parsed dataset page.
type Page struct {
    Slug        string
    Path        string
    Storage     Storage
    Format      Format
    Rows        int
    DataPath    string   // relative path under repo
    Columns     []Column // parsed from frontmatter when present
    Frontmatter map[string]string
}

type Column struct {
    Name string
    Type string // "string" | "integer" | "number" | "boolean"
}
```

`internal/dataset/rows.go`: export `LoadRows(format Format, path string) ([][]string, error)`,
`ValidateRows(rows [][]string, columns []Column) []ValidationError`,
`SampleRows(rows [][]string, n int) [][]string`,
`CountRows(format Format, path string) (int, error)`. Lift the bodies
from `internal/lint/data/rows.go` verbatim where possible. Tests cover
csv, tsv, dsv (with delimiter sniffing), json, topojson, schema
validation hits/misses, sample sizes.

`internal/dataset/frontmatter.go`: implement `FmGet(text, key) (string,
bool)`, `FmSet(text, key, value) string` (insert before closing `---`
if missing), `FmRemove(text, key) string`. Tests cover insert, replace,
remove, idempotency, no-frontmatter no-op.

`internal/dataset/fence.go`: implement
`ExtractDataFence(text string) (rows []byte, format Format, ok bool)`,
`ReplaceDataFence(text string, format Format, rows []byte) string`,
`RemoveDataFence(text string) string`. Tests cover empty fence,
existing fence, no `## Data` heading, mixed-format detection.

`internal/dataset/config.go`: `InlineThresholds(repoRoot)` returns
`(maxRows, maxBytes int)` from `.awiki/config` keys
`AWIKI_DATASET_INLINE_MAX_ROWS` / `AWIKI_DATASET_INLINE_MAX_BYTES`,
defaults 500 / 51200.

`internal/dataset/runner.go`:

```go
type Runner struct {
    RepoRoot   string
    ContentDir string
    DataDir    string
    Config     map[string]string
    Today      string
}

func (r *Runner) DatasetDir() string { /* <ContentDir>/datasets */ }
```

`internal/lint/data/rows.go` now becomes:

```go
package data

import "awiki/internal/dataset"

// re-exports preserving the existing internal API
func loadRows(format string, path string) ([][]string, error) {
    return dataset.LoadRows(dataset.Format(format), path)
}
// (similarly for the other helpers)
```

Adjust call sites in `internal/lint/data/rules.go` if needed. All lint
tests must continue to pass.

Verify:
```
go test ./...
```
Commit: `feat: lift dataset row helpers and add internal/dataset skeleton`.

### Task 2: validate verb

**Files:**
- Create: `internal/dataset/validate.go`, `validate_test.go`
- Create: `internal/cli/dataset.go` — nested dispatcher with
  `case "validate"`; other verbs return "not yet ported".
- Modify: `internal/cli/cli.go` — register `dataset` subcommand.
- Modify: `scripts/dataset.sh` — add Go shim head (per synth pattern)
  with `AWIKI_DATASET_GO_VERBS=(validate)`.

Implementation outline:

```go
// (*Runner).Validate(slug string, stdout, stderr io.Writer) error
// 1. Read content/datasets/<slug>.md
// 2. Parse frontmatter
// 3. Load rows from inline fence or data_path
// 4. Schema-validate if columns present (DATASET|ERROR|... per failure)
// 5. FmSet rows: <count>; AtomicWrite back
// 6. fmt.Fprintf(stdout, "DATASET|validated %s (rows=%d)\n", slug, n)
```

Tests: csv inline OK, csv inline schema fail, json file storage, missing
page, bad data_path.

Verify against bash:
```
bash scripts/dataset.sh validate <slug>   # AWIKI_DATASET_LEGACY=1
./bin/awiki dataset validate <slug>
```

### Task 3: compact verb

**Files:**
- Create: `internal/dataset/compact.go`, `compact_test.go`
- Modify: `internal/cli/dataset.go` (wire `compact`)
- Modify: `scripts/dataset.sh` (add `compact` to GO_VERBS)

Implementation: read page, decide direction (inline→file when over
threshold; file→inline when under), perform the move, update
frontmatter (`storage`, optional `data_path`), atomic write.
Stdout records per spec.

Tests: inline-to-file (threshold breach), file-to-inline (under
threshold), already-compact no-op, refuses inline above threshold.

### Task 4: new verb

**Files:**
- Create: `internal/dataset/scaffold.go`, `scaffold_test.go`,
  `new.go`, `new_test.go`
- Modify: `internal/cli/dataset.go` (wire `new`)
- Modify: `scripts/dataset.sh` (add `new` to GO_VERBS)

`scaffold.go`: `WriteScaffold(in ScaffoldInput) []byte` — produces
the page bytes byte-equivalent to the bash heredoc.
`ScaffoldInput`: `Slug, Format, Today, Storage, Rows, DataPath` plus
optional inline data bytes.

`new.go`: parse args, validate slug, choose storage based on `--from`
and threshold, write page (refusing overwrite), copy file to
`<DataDir>/<slug>.<fmt>` if storage=file.

Tests: minimal slug (no --from), --from inline, --from file (over
threshold), refuse overwrite.

### Task 5: data-init flat verb

**Files:**
- Create: `internal/cli/init.go` — flat `data-init` verb dispatch
- Create: `internal/dataset/init.go`, `init_test.go`
- Modify: `internal/cli/cli.go` — add `case "data-init"` flat verb
- Modify: `scripts/data-init.sh` — Go shim head; bash continues to
  handle the interactive encryption step when invoked under
  `AWIKI_DATA_INIT_LEGACY=1` or when Go skips it.

Steps to implement (idempotent):

1. `EnsureDirs(r)` — create `content/datasets`, `content/queries`,
   `content/charts`, `data`, `assets/charts`, `static/vendor/vega` plus
   `.gitkeep` markers when absent. Emit
   `DATA-INIT|created <dir>` or `DATA-INIT|skip <dir>` to stdout.
2. `EnsureVegaVendor(r)` — call `bash scripts/lib/vendor-vega.sh` via
   `os/exec`. Skip when files exist.
3. `EnsureWikiMd(r)` — patch `WIKI.md` between `<!-- BEGIN data-layer -->`
   and `<!-- END data-layer -->` markers using
   `region.ManagedReplace`. Body comes from
   `scripts/templates/wiki-data-layer.md`.
4. `EnsureConfig(r)` — append default kv pairs to `.awiki/config`
   when missing (`AWIKI_DATA_LAYER=on`,
   `AWIKI_DATASET_INLINE_MAX_ROWS=500`,
   `AWIKI_DATASET_INLINE_MAX_BYTES=51200`).
5. **Encryption step**: do NOT port. Emit
   `DATA-INIT|encryption-skipped|run 'bash scripts/data-init.sh' for interactive setup`
   to stderr.

Tests: fresh tempdir, assert all four steps; second run is no-op.

### Task 6: final regression + merge

- `go test -race ./...` clean
- `just test` 774/774
- Smoke each verb against a real dataset page (validate/compact;
  new is destructive — use a unique slug)
- Merge `feat/dataset-domain` to main with `--no-ff`.

## Self-review

- Spec coverage: every verb mapped to a task.
- Type consistency: `Storage`, `Format`, `Page`, `Column`, `Runner`
  reused across files.
- No placeholders.
- Lint test parity preserved via the lift-and-alias pattern in Task 1.
