# Go ingest domain — slice plan design

Date: 2026-04-30

## Relation to prior design

This document refines the 2026-04-29 ingest domain design
(`2026-04-29-go-ingest-domain-design.md`) into an executable slice plan
and pins two open questions that were left implicit in the prior spec.
The high-level goal, non-goals, compatibility contract, and verb list
from the prior spec stand unchanged. Read both docs together; this one
supersedes the prior spec only where they disagree.

## Goal

Produce the per-PR slicing, package layout, error/exit-code contract,
and test harness so the ingest port can execute slice-by-slice without
further design rounds. Terminal state of this design is one umbrella
implementation plan (created by the writing-plans skill) that lists
every slice from infra through cleanup.

## Non-goals

- Reimplementing externals: `pdftotext`, `whisper`, `qmd`, `xlsx2csv`,
  `git`, `git-crypt`. Stay exec.
- Porting `scripts/ingest-with-agent` agent invocation semantics. The
  Go binary still shells the configured agent CLI.
- Per-format extraction quality improvements. Byte-for-byte parity with
  the bash output, modulo timestamps and absolute paths.
- Changing the AGENT-PROMPT contract.

## Pinned open questions

The 2026-04-29 spec called these out as decisions to make. Pinned now:

- **xlsx implementation.** Stay exec `xlsx2csv` for v1. No `excelize/v2`.
  Revisit only if a concrete extraction need surfaces that the bash
  pipeline cannot serve.
- **git clone.** Stay exec `git` for v1. No `go-git`. Revisit only if
  the `ingest-git` transformer materially benefits from in-process
  Git operations (e.g. shallow clone tuning, incremental fetch).

Both pins favor lower-risk parity. The roadmap explicitly allows
revisiting either when a concrete need arises.

## Architecture

### Package layout

```
internal/
  cli/ingest.go              # flat dispatch
  ingest/
    types.go                 # Format, Spec, Result, AgentPromptArgs
    runner.go                # Runner struct (deps + RepoRoot)
    bookkeep.go              # archive source, log, lint, reindex,
                             # emit AGENT-PROMPT
    capture.go               # capture <text>
    watchdog.go              # fsnotify + poll fallback
    formats/
      xlsx.go                # → adapters.Xlsx2csv
      git.go                 # → adapters.Git (clone) + transformer
      pdf.go                 # → adapters.Pdftotext
      audio.go               # → adapters.Whisper
adapters/
  fsnotify.go                # native ExecFSNotify wraps
                             # github.com/fsnotify/fsnotify
  pdftotext.go
  whisper.go
  xlsx2csv.go
  git_ext.go                 # extends ExecGit / ExecSynthGit with
                             # clone/init/fetch
```

### CLI dispatch

Flat verbs (matches roadmap "flat for one-shots" rule):

- `awiki ingest <path>` — bookkeeping flow; auto-detects format and
  routes to per-format worker for extraction, then bookkeep + emit
  AGENT-PROMPT.
- `awiki ingest --agent=<cli> <path>` — same plus shell `<cli>` with
  AGENT-PROMPT on stdin.
- `awiki ingest-xlsx <path> [flags]`
- `awiki ingest-git <spec> [flags]`
- `awiki ingest-pdf <path>`
- `awiki ingest-audio <path>`
- `awiki capture <text>`
- `awiki watchdog [--catchup] [--once] [--poll-interval=2s]`
- `awiki ingest-batch-list`
- `awiki ingest-git-list`

## Slice plan

Ten PRs total. Each row is one PR. Order: infra → cheap leaves → writers
→ orchestrators → cleanup. Matches the lint/synth/dataset/chart/query
pattern shipped to date.

| # | Slice | Verbs | Notes |
|---|-------|-------|-------|
| 1 | infra | — | `internal/ingest/` skeleton, `types.go`, `runner.go`, dispatch returns "verb not yet ported" for every verb. Fixture loader + golden differ wired. Justfile + scripts unchanged. Shim is wired in slice 2 (first verb sub-slice), not here. |
| 2 | leaf | `capture` | Smallest verb. Smoke-tests the `bookkeep`+`Log` helpers and the fixture harness. Wires the `capture.sh` shim. |
| 3 | leaf | `ingest-batch-list`, `ingest-git-list` | Pure fs-scan listers. No external adapters. |
| 4 | writer | `ingest-pdf` | First exec adapter (pdftotext). Establishes the adapter-fake pattern reused by audio/xlsx. |
| 5 | writer | `ingest-audio` | Same shape as pdf; whisper adapter. |
| 6 | writer | `ingest-xlsx` | xlsx2csv adapter. Sheet-per-page layout matches `lib/xlsx-extract.py` output. |
| 7 | orchestrator | `ingest` (incl. `--agent=`) | Defines AGENT-PROMPT byte contract via golden fixture. Auto-detects format and dispatches to slices 4-6. |
| 8 | orchestrator | `ingest-git` | Exec git clone via adapter; ports `ingest-git-transform.py` to Go. Largest single port. |
| 9 | orchestrator | `watchdog` | fsnotify watch + poll fallback. Behavioral test only. |
| 10 | cleanup | — | Delete bash + py for the domain. Replace shim entries with direct `awiki <verb>` in justfile. Drop bats files now covered by Go tests. |

### Per-slice acceptance criteria

Every verb sub-slice (slices 2-9) must:

1. Implement the verb under `internal/ingest/`.
2. Add golden fixture(s) under `tests/fixtures/ingest/<verb>/` with
   `input/`, `args`, `expected_stdout`, `expected_files/`.
3. Add Go test pinning the new path.
4. Wire dispatch in `internal/cli/ingest.go`. Delete the corresponding
   bash branch from the shim in the same PR.
5. Justfile recipe unchanged.

The cleanup slice (#10) merges only when:

- Every verb is Go-native and Go-tested.
- Bats coverage matched by Go tests.
- One release has shipped with the shims active and no parity bug
  filed.

## Data flow

### `ingest <path>`

1. Resolve repo root (`AWIKI_REPO_ROOT` env or `git rev-parse
   --show-toplevel`).
2. Detect format by extension:
   - `.xlsx` → xlsx
   - `.pdf` → pdf
   - `.m4a`, `.mp3`, `.wav` → audio
   - `.git-spec` (or `--git-spec` flag) → git
   - else → generic markdown ingest
3. Per-format worker writes intermediate artifacts under `content/...`
   and emits `INGEST|format=X|src=Y|dst=Z`.
4. Bookkeep: archive source to `raw/inbox/_archive/`, log via
   `internal/ops.Log`, run lint, run qmd reindex (adapter).
5. Emit `AGENT-PROMPT|<single-string>` byte-identical to the bash
   form at `scripts/ingest.sh:115`.
6. If `--agent=<cli>` was passed, shell `<cli>` with the AGENT-PROMPT
   line on stdin and exit with the agent's status.

### `watchdog`

- fsnotify watcher on `raw/inbox/batch/`.
- On `Create`+`Write` settle (debounce 500 ms), invoke
  `ingest <path>` in-process.
- Failure → move source to `raw/inbox/batch/_failed/`, log
  `WATCHDOG|event=fail|path=...|err=...`, continue.
- `--catchup` drains existing files at startup before entering the
  watch loop.
- `--once` processes one event then exits (used by tests).
- `--poll-interval=2s` enables the polling fallback (used when
  fsnotify init fails or on platforms where it is unreliable).

### Records (byte-identical with bash)

- `INGEST|format|src|dst` — verify exact field order with `grep`
  against the bash script before each verb sub-slice.
- `AGENT-PROMPT|<string>` — single line, single string. The string
  itself is the WIKI.md §4.1 steps 3-9 prompt; format pinned by
  golden fixture in slice 7.
- `WATCHDOG|event|path[|err]`

## Error handling

Exit codes match bash:

- `0` — success.
- `1` — user error (bad path, missing file, invalid format).
- `2` — extractor failure (pdftotext / whisper / xlsx2csv / git
  non-zero).
- `3` — bookkeeping or post-ingest lint failure.

All adapter errors wrap with `fmt.Errorf("ingest %s: %w", verb, err)`.
No panics. Watchdog never aborts the watch loop on per-file failure;
it quarantines and continues.

When extraction succeeds but bookkeeping fails (lint, log, archive),
emit `INGEST|partial|...` and return non-zero. Source stays in inbox
so the user can re-ingest after fixing the underlying issue.

## Testing

### Per-verb harness

| Verb | Type | Fixture path |
|------|------|--------------|
| `capture` | golden | `tests/fixtures/ingest/capture/` |
| `ingest-batch-list` | golden | `tests/fixtures/ingest/batch-list/` |
| `ingest-git-list` | golden | `tests/fixtures/ingest/git-list/` |
| `ingest-pdf` | golden + adapter fake | `tests/fixtures/ingest/pdf/` |
| `ingest-audio` | golden + adapter fake | `tests/fixtures/ingest/audio/` |
| `ingest-xlsx` | golden + adapter fake | `tests/fixtures/ingest/xlsx/` |
| `ingest` | golden | `tests/fixtures/ingest/ingest/` (per-format) |
| `ingest-git` | golden + adapter fake | `tests/fixtures/ingest/git/` |
| `watchdog` | behavioral | `internal/ingest/watchdog_test.go` |

Each golden fixture has `input/`, `args`, `expected_stdout`,
`expected_files/`. The diff normalizes timestamps and absolute paths
only.

### Adapter fakes

Adapter fakes live in `internal/testutil/` and implement the same
interfaces the production exec adapters do. Tests inject the fake into
the `Runner`. No `os/exec` outside `internal/adapters/`.

### Bats parity

Existing `tests/*.bats` stays as a parallel oracle until slice 10
(cleanup). Go tests duplicate coverage first; bats files for the
ingest domain delete last.

## Risks and mitigations

- **fsnotify cross-platform.** macOS vs linux differ on
  `Rename`+`Create` ordering. Mitigation: behavioral test with
  synthetic events; keep polling fallback.
- **AGENT-PROMPT drift.** External agents and hooks consume this
  record. Mitigation: golden fixture in slice 7, byte-match; grep
  external repos before deleting bash in slice 10.
- **`ingest-git-transform.py` algorithmic logic.** Carries semantic
  rules for content layout. Mitigation: rich golden fixtures in slice
  8; port one transformer at a time inside the slice.
- **Watchdog debounce.** 500 ms debounce may miss bursty writes.
  Mitigation: behavioral test asserts both single-write and
  rapid-multi-write produce one ingest call.
- **Hidden flag drift.** `ingest-xlsx` accepts pass-through flags
  (`*flags` in justfile). Mitigation: per-flag test fixture in
  slice 6.

## Out of scope

- AI agent integration semantics beyond `--agent=<cli>` shell.
- Format-specific extraction quality improvements.
- MCP ingest handlers (separate binary).
- New verbs not present in the justfile today.
- Reorganizing `WIKI.md §4.1` ingest workflow.
