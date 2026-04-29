# Go port roadmap design

Date: 2026-04-29

## Goal

Move every awiki bash and Python command currently shelled from the
`justfile` into the single `awiki` Go executable, so the justfile becomes a
thin alias layer (or disappears) and the binary is the one source of truth
for wiki workflows.

The migration is **domain-grouped**, executed under a per-domain
**infra-slice + verb-sub-slice** pattern that mirrors how the lint port
shipped (`go-awiki-lint-core` → `go-lint-next-slices`). This document is
the umbrella roadmap; each domain ships its own design and plan when its
turn arrives.

## Non-goals

- Reimplementing externals: `hugo`, `qmd`, `git-crypt`, `age`, `vl-convert`,
  `duckdb`, `pdftotext`, `whisper`, `gpg`, `bats`. `fswatch`/`inotifywait`
  are replaced by the pure-Go `fsnotify`; `git` is replaced by `go-git`
  where it earns its weight, otherwise stays exec.
- Changing user-visible contracts. Stdout records (`LINT|...`, `FIX|...`,
  `AGENT-PROMPT|...`, `INGEST|...`, `REVIEW|...`, `RECUR|...`), exit codes,
  and on-disk file shapes (frontmatter keys, managed-region markers,
  dataset/chart/synth page bodies) stay byte-identical.
- Removing justfile or bash scripts before parity is proven and one release
  has shipped with the shim active.
- Cross-domain refactors. Each domain ports under the existing schema in
  `WIKI.md`.
- Per-verb Go signatures, fixture lists, and oracle bats cases. Those
  belong in the per-domain spec written when the domain's turn arrives.
- Migration of `mcp/` (separate binary) and Hugo themes/layouts.

## Strategy

### Domain order (locked)

1. `synth`
2. `dataset`
3. `chart`
4. `query`
5. `ingest`
6. `template`
7. `ops` (task layer, build/serve/deploy, bootstrap/install)

The order builds on lint domain familiarity (the lint port already touched
synth/dataset/chart/query rule logic), keeps ingest until externals are
well understood, and finishes with the long tail of small ops verbs.

### CLI shape (hybrid)

Multi-verb domains take nested subcommands; one-shots stay flat.

- Nested: `awiki synth <verb>`, `awiki dataset <verb>`, `awiki chart
  <verb>`, `awiki query <verb>`, `awiki ingest <verb>`, `awiki template
  <verb>`.
- Flat: `awiki lint`, `awiki agenda`, `awiki triage`, `awiki capture`,
  `awiki recur`, `awiki recur-dry`, `awiki review`, `awiki scan`,
  `awiki rename`, `awiki delete`, `awiki log`, `awiki reindex`,
  `awiki build`, `awiki serve`, `awiki check-deps`,
  `awiki install-hooks`, `awiki install-qmd`, `awiki encrypt-init`,
  `awiki watchdog`, `awiki bootstrap-step`, `awiki data-init`,
  `awiki task-init`, plus the per-format ingest workers above.

`awiki review` ports the existing chain (`agenda` → `lint` →
`review-status`), not a single shell call. `data-init` and `task-init`
stay flat (instead of `awiki dataset init` / `awiki ops task-init`) to
preserve the current `just data-init` / `just task-init` caller
contract; this is a compat carve-out, not an oversight.

### Compatibility strategy: shim then delete

Per domain, follow the lint pattern:

1. Bash script (e.g. `scripts/synth.sh`) becomes a thin shim that execs the
   Go subcommand once a verb is ported.
2. Justfile recipes keep their current name and arg shape, calling the
   shim.
3. Once every verb in a domain is Go-native and one release has shipped
   without parity bugs, the cleanup slice deletes the bash script and
   updates the justfile to call `awiki` directly.

### External tool boundary

`os/exec` lives only in `internal/adapters/`. Domain packages depend on
typed adapter interfaces, not on `exec.Command`.

- Pure-Go replacements where they pay off: `fsnotify` (watchdog),
  `go-git` (ingest-git, git-state).
- Stay exec: `hugo`, `qmd`, `duckdb`, `vl-convert`, `pdftotext`, `whisper`,
  `gpg`, `git-crypt`, `age`. These are heavy, well-maintained, and bring
  CGO or large dependency trees that do not justify a Go rewrite.

### Parity oracle (golden + behavioral)

- **Golden** for emitters: lint, synth, dataset, chart, query, ingest,
  template. Fixture wiki under `tests/fixtures/<domain>/<verb>/` with
  `input/`, `args`, `expected_stdout`, `expected_files/`. The diff
  normalizes timestamps and absolute paths only.
- **Behavioral** for orchestrators: watchdog, triage, agenda, action-recur,
  build, serve, bootstrap. Tests assert externally observable effects
  (events emitted, regions rewritten, processes exit clean, idempotency
  holds), not byte equality.

The existing `tests/*.bats` stays as a parallel oracle until each domain's
cleanup slice. Go tests duplicate coverage first; bats files for that
domain delete last.

## Slice anatomy (per domain)

Each domain ships under one umbrella spec as one infra slice plus N verb
sub-slices, plus a cleanup slice.

### Slice 0: lint cleanup (already queued)

The lint port shipped without the cleanup slice. Bash and Python lint
files still live under `scripts/` (`lint.sh`, `lint-synth.sh`,
`lint-data.sh`, `lint-chart.sh`, `lint-query.sh`, and five
`lint-synth-*.py` helpers). Slice 0 of this roadmap deletes those after
confirming Go test coverage matches, in line with the standard cleanup
slice (see "Cleanup criteria"). It is not one of the seven domains
listed below.

### Infra slice (first PR per domain)

- Add `internal/<domain>/` package skeleton, types, fixture loader, golden
  test harness wiring.
- Register nested command in `internal/cli`. Dispatch returns
  "verb not yet ported" for any verb not in the first sub-slice.
- Bash scripts and the justfile are unchanged. The shim is wired in the
  **first verb sub-slice**, not here, because there is nothing to delegate
  to until a verb actually exists in Go.

### Verb sub-slice (each subsequent PR)

Order within domain: cheap leaf verbs first (`list`, `resolve`, `status`),
then write verbs (`new`, `compact`, `render-one`), then orchestrators
(`finalize`, `render`, `update`).

Each sub-slice:

1. Implement Go logic under `internal/<domain>/`.
2. Add golden fixture(s) under `tests/fixtures/<domain>/<verb>/`.
3. Wire dispatch in `internal/cli`. Delete the corresponding bash branch
   from the shim in the same PR.
4. Add Go test (or port the relevant bats test) that pins the new path.
5. Justfile recipe unchanged. Caller contract preserved.

### Cleanup slice (last PR per domain)

- Delete bash and Python files for the domain.
- Replace shim with direct `awiki <domain> <verb>` calls in the justfile.
- Drop bats tests now covered by Go tests.

### Per-domain spec

Each domain's spec is written when its turn arrives at
`docs/superpowers/specs/YYYY-MM-DD-go-<domain>-design.md`. It defines:

- Verb-to-Go-signature mapping.
- Fixture inputs and expected outputs.
- External tool boundaries for the domain.
- Parity oracle: which bats cases are golden.
- Cleanup criteria.

## Architecture

### Package layout

```
cmd/awiki/main.go
internal/
  cli/             # flag parsing + dispatch (exists; grows nested groups)
  wiki/            # page model, frontmatter, slug/alias maps (exists)
  lint/            # already shipped
  adapters/        # exec wrappers + native fs/git
    hugo.go
    qmd.go
    duckdb.go
    vlconvert.go
    pdftotext.go
    whisper.go
    git.go         # exec + go-git
    gpg.go
    fsnotify.go    # native watch
  synth/
  dataset/
  chart/
  query/
  ingest/
    xlsx/
    git/
    pdf/
    audio/
    watchdog.go
  template/
  ops/
    agenda.go
    triage.go
    action.go      # scan + recur
    log.go
    index.go       # qmd-index wrapper
    rename.go
    delete.go
    build.go
    serve.go
    deploy.go
    bootstrap.go   # install-hooks, install-qmd, encrypt-init,
                   # check-deps, wire-mcp
  fsutil/          # atomic write, advisory lock, repo-root resolution
  region/          # managed-region begin/end markers
  config/          # .awiki/config loader
  emit/            # shared record formats
  testutil/        # fixture loader, golden differ
```

### Shared building blocks (extracted before first domain port)

These are pulled out before `synth` so every domain consumes one
implementation. The extraction is a **lint refactor slice** (no behavior
change): the helpers already exist inside `internal/lint/` (atomic write,
lock, managed-region, record formats) and get lifted into the new shared
packages, then lint imports them. This slice precedes the synth infra
slice and ships green-field for everyone after lint.

- `fsutil` ports `scripts/lib/lock.sh` and the atomic-write helpers
  already used by lint.
- `region` ports `scripts/lib/managed-region.sh`. Synth, agenda, query,
  and dataset all depend on this.
- `action` ports `scripts/lib/action-grammar.sh`. The single most
  cross-cutting bash library: sourced by `action-recur.sh`,
  `action-scan.sh`, `triage.sh`, `task-layer-migrate-review.sh`,
  `lint.sh`, and `ingest-git.sh`. It owns action-line tokenization and
  must be shared, not re-implemented per domain.
- `config` reads `.awiki/config`.
- `emit` owns the `LINT|`, `FIX|`, `AGENT-PROMPT|`, `INGEST|`, `REVIEW|`,
  `RECUR|` record formats so domain code never builds the strings by
  hand.
- `adapters` is the only package allowed to call `os/exec`. Domain
  packages take an adapter interface and unit tests inject fakes.

Each domain package depends only on `wiki`, `fsutil`, `region`, `action`,
`config`, `emit`, `adapters`, and `testutil`. There are no cross-domain
imports.

## Domain scope

### 1. synth

Bash/Python: `synth.sh`, `synth-plugin-load.sh`,
`synth-mindmap-validate.sh`, `synth-export-anki.sh`,
`synth-export-anki.py`.

CLI: `awiki synth {new, regen, finalize, accept-stage, refine, list,
resolve}`. Anki export and mindmap validation become flat verbs:
`awiki synth-export-anki`, `awiki synth-mindmap-validate` (or nested
`awiki synth export-anki`, `awiki synth mindmap-validate` — pin in the
synth spec).

External boundary: plugin loader keeps the existing plugin protocol
(plugins remain external scripts) but dispatch lives in Go.
`synth-export-anki.py` carries algorithmic logic (Anki package
generation) and is treated as Python algorithmic logic for the risks
section: rich golden fixtures required.

### 2. dataset

Bash/Python: `dataset.sh`, `data-init.sh`, `scripts/lib/dataset-fm.sh`,
`scripts/lib/dataset-rows.py`.

CLI: `awiki dataset {new, compact, validate}`, plus flat
`awiki data-init`.

### 3. chart

Bash/Python: `chart.sh`, `scripts/lib/vendor-vega.sh`,
`scripts/lib/vl-resolve.py`.

CLI: `awiki chart {new, render, render-one}`. `vl-convert` stays exec.

### 4. query

Bash/Python: `query.sh`, `scripts/lib/query-engine.sh`,
`scripts/lib/query-extract.py`, `scripts/lib/query-format.py`,
`scripts/lib/query-resolve.py`, `scripts/lib/query-determinism.py`.

CLI: `awiki query {run, new, render, render-one, fence-render}`. `duckdb`
stays exec.

### 5. ingest

Bash/Python: `ingest.sh`, `ingest-xlsx.sh`,
`scripts/lib/xlsx-extract.py`, `ingest-git.sh`,
`ingest-git-transform.py`, `scripts/lib/git-clone.sh`,
`scripts/lib/git-state.sh`, `ingest-pdf.sh`, `ingest-audio.sh`,
`watchdog.sh`, `capture.sh`.

CLI maps the existing justfile recipes as **flat verbs** (matches the
"flat verbs for one-shots" rule and avoids the path-vs-subverb
ambiguity that nested ingest dispatch would create):

- `awiki ingest <path>` — bookkeeping flow, mirrors `just ingest <path>`.
  Auto-detects format by extension and delegates to the per-format
  workers below for extraction, then runs the bookkeeping step
  (`AGENT-PROMPT|...` for steps 3-9 of `WIKI.md §4.1`).
- `awiki ingest --agent=<cli> <path>` — agent-driven variant, mirrors
  `just ingest-with-agent`.
- `awiki ingest-xlsx <path>`, `awiki ingest-git <spec>`,
  `awiki ingest-pdf <path>`, `awiki ingest-audio <path>` — per-format
  workers, mirror the existing format-specific recipes. Each worker is
  format extraction only; bookkeeping is invoked separately by
  `awiki ingest <path>`.
- `awiki watchdog`, `awiki capture` — flat.
- `awiki ingest-batch-list`, `awiki ingest-git-list` — flat listers.

Native: `fsnotify`, `go-git`. Exec: `pdftotext`, `whisper`, `qmd`. The
ingest spec pins which python helpers (`xlsx-extract.py`,
`ingest-git-transform.py`) port as Go versus stay exec-via-adapter.

### 6. template

Bash: `template-init.sh`, `template-update.sh` (~971 LOC, mostly a
driver around `scripts/_template_helpers/`), `template-step.sh`,
`template-retrofit.sh`, `template-merge.sh`, `template-plan.sh`,
`template-provenance.sh`, `template-source-check.sh`,
`template-manifest.sh`, `template-attr-audit.sh`, `template-config.sh`.

Python helpers under `scripts/_template_helpers/` (12 modules:
`availability.py`, `bootstrap_hash.py`, `bootstrap_replay.py`,
`cache_rotate.py`, `escape.py`, `lint_template.py`, `manifest_parse.py`,
`migration.py`, `plan_emit.py`, `preflight.py`, `provenance.py`,
`state.py`, `sync.py`) carry the heaviest algorithmic logic in the
domain. The template port is dominated by porting these helpers, not by
porting the bash drivers; the per-domain spec must enumerate each
helper's parity oracle.

Markdown payloads under `scripts/templates/` (`wiki-data-layer.md`,
`wiki-task-layer.md`, `wiki-weekly-review.md`) are user-visible
artifacts. The template spec pins whether they are embedded into the
binary (via `embed.FS`) or kept as repo files referenced by path.

CLI: `awiki template {init, update, status, gc, retrofit, merge, plan,
provenance, source-check, manifest, attr-audit, config}` plus
`awiki bootstrap-step <id>` flat.

### 7. ops

Bash/Python: `task-init.sh`, `triage.sh` (~856 LOC), `action-scan.sh`,
`action-recur.sh`, `agenda.sh`, `review-status.sh`, `log-append.sh`,
`qmd-index.sh`, `rename.sh`, `delete-page.sh`, `update-catalog.sh`,
`build.sh`, `serve.sh`, `deploy-build.sh`, `install-hooks.sh`,
`install-qmd.sh`, `encrypt-init.sh`, `check-deps.sh`,
`wire-awiki-mcp.sh`, `wire-qmd-mcp.sh`.

`update-catalog.sh` is invoked by both `ingest-git.sh` and the agenda
chain. It ports as part of `ops` (verb: `awiki update-catalog` flat),
and `ingest-git`'s adapter calls the Go function directly rather than
re-implementing.

`task-layer-migrate-review.sh` is a one-off migration tool, not a
recurring verb. Disposition: keep as bash, do not port. It will be
deleted in the ops cleanup slice once the migration window has passed
(no users depend on it being callable from Go).

CLI: mostly flat verbs (see CLI shape above). Some scripts may stay bash
forever (see Open questions).

## Compatibility contract (per domain)

Locked before the infra slice merges:

- Justfile recipes keep their current name and argument shape.
- Emitted records (`LINT|`, `FIX|`, `AGENT-PROMPT|`, `INGEST|`, `REVIEW|`,
  `RECUR|`, etc.) are **per-record byte-identical** to the bash output.
  Where bash emits records in non-deterministic order (parallel
  ingest, multi-page lint), parity is asserted on the sorted record set
  using a per-domain stable sort key, not on raw output order.
- Written file shapes (frontmatter keys, managed-region markers,
  dataset/chart sidecars, synth page bodies, template manifests) are
  byte-identical, modulo timestamps and absolute paths normalized by
  the golden differ.
- Exit codes match.
- Existing bats tests continue to pass against the shim during the
  transition.

## Cleanup criteria (per domain)

The cleanup slice may merge only when:

- Every verb in the domain is ported and Go-tested.
- Bats coverage is matched by Go tests or migrated.
- One release has shipped with the shim active and no parity bug filed.
- All bash and Python files in the domain are deleted in the cleanup
  PR.
- The justfile is updated to call `awiki` directly.

## Risks and mitigations

- **Bash drift mid-port.** Active development on the bash domain while it
  is being ported makes the spec stale. Mitigation: freeze the domain on
  `feat/go-<domain>` and rebase weekly.
- **Hidden contracts.** Records (notably `AGENT-PROMPT|`) are consumed by
  hooks and agents that do not live in this repo. Mitigation: grep
  external repos and personal config for record prefixes before deleting;
  pin the format in the per-domain spec.
- **Shell quirks unportable verbatim.** `set -euo pipefail`, process
  substitution, here-docs interacting with editors. Mitigation: reproduce
  semantics with `errgroup` and scoped temp dirs; behavioral tests catch
  divergence.
- **Python algorithmic logic.** `lint-synth-fuzzy.py`,
  `query-determinism.py`, `xlsx-extract.py`, `ingest-git-transform.py`
  carry numeric or semantic logic. Mitigation: rich golden fixtures; port
  one verb at a time so divergence localizes.
- **Watchdog cross-platform.** `fswatch` (mac) vs `inotifywait` (linux)
  vs polling. `fsnotify` covers both but has edge cases on
  `Rename`+`Create` ordering. Mitigation: behavioral test with synthetic
  events; keep polling fallback.
- **CGO temptation.** `go-duckdb`, `git2go` pull CGO. Stay pure Go:
  `go-git` is pure; `duckdb` stays exec.
- **Caller regression.** Some users invoke scripts directly
  (`bash scripts/synth.sh ...`). The shim preserves this; the existing
  `CHANGELOG.md` records the deletion in the cleanup slice's release
  notes.
- **Bats coverage gaps.** The cleanup criterion "bats coverage matched
  by Go tests" is trivially true for any verb without a bats test today.
  Mitigation: each per-domain spec audits existing bats files and lists
  any verb without coverage; those verbs get a new behavioral test
  before the verb sub-slice merges, not after.

## Open questions (defer to per-domain spec)

- **Synth plugin loader.** Likely keep plugin protocol (plugins stay
  external scripts), port the loader to Go.
- **Encryption (`encrypt-init.sh`).** Wraps `git-crypt`/`age`. Pure
  adapter or skip the port (rare lifecycle command, low value)?
- **Deploy (`deploy-build.sh`).** Site-specific. Likely keep as bash
  forever.
- **Test runner (`bats`).** Keep until Go test coverage matches; do not
  port.

## Justfile passthroughs (not migrated)

A handful of justfile recipes are thin passthroughs to external tools and
are not part of this port. They stay as one-line recipes even after the
cleanup slices land:

- `just status`, `just commit` — git wrappers.
- `just search <q>` — `qmd` passthrough.
- `just test` — `bats` runner.
- `just help` — prints `docs/just-help.txt`.
- `just init` — agent prompt, no-op shell.

The justfile may also retain these as the only remaining recipes once all
domains have completed cleanup.

## Out of scope of this roadmap

- Per-verb Go signatures (each domain spec defines).
- Migration of `mcp/` server (separate Go binary already exists).
- Hugo themes and layouts.
- Refactoring `WIKI.md` schema.
