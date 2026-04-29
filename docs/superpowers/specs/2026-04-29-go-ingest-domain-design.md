# Go ingest domain design

Date: 2026-04-29

## Goal

Port the awiki ingest pipeline to the Go binary:

- Bookkeeping flow: `awiki ingest <path>` (`scripts/ingest.sh`)
- Per-format extractors:
  - `awiki ingest-xlsx <path>` (`scripts/ingest-xlsx.sh` + `lib/xlsx-extract.py`)
  - `awiki ingest-git <spec>` (`scripts/ingest-git.sh` + `ingest-git-transform.py` + `lib/git-clone.sh` + `lib/git-state.sh`)
  - `awiki ingest-pdf <path>` (`scripts/ingest-pdf.sh`)
  - `awiki ingest-audio <path>` (`scripts/ingest-audio.sh`)
- `awiki watchdog [--catchup] [--once]` (`scripts/watchdog.sh`)
- `awiki capture <text>` (`scripts/capture.sh`)

## Non-goals

- Reimplementing `pdftotext`, `whisper`, `qmd`, `git-crypt`, `git`
  (mostly), `xlsx2csv`. Stay exec.
- Removing bash. Shim per verb.

## Compatibility contract

- Justfile: `ingest`, `ingest-with-agent`, `ingest-xlsx`, `ingest-git`,
  `ingest-batch-list`, `ingest-git-list`, `watchdog`, `capture`.
- Records byte-identical: `INGEST|*` (introduce only formats bash
  emits — verify with grep), `AGENT-PROMPT|<single-string>` (matches
  scripts/ingest.sh:115), `WATCHDOG|*`.

## Architecture (sketch)

```
internal/
  cli/ingest.go
  ingest/
    types.go
    runner.go
    bookkeep.go            # ingest <path> bookkeeping flow
    xlsx/                  # xlsx adapter
    git/                   # ingest-git transformer
    pdf/                   # ingest-pdf wrapper
    audio/                 # ingest-audio wrapper
    watchdog.go            # native fsnotify-based watcher
    capture.go             # capture <text> append flow
  adapters/
    fsnotify.go            # native ExecFSNotify uses github.com/fsnotify/fsnotify
    pdftotext.go
    whisper.go
    xlsx2csv.go
    git_ext.go             # extends ExecGit / SynthGit with clone/init
```

## Strategy

Per the umbrella roadmap:
- Native: `fsnotify` (cross-platform), `go-git` for ingest-git
  cloning where it earns its weight; otherwise exec git.
- Exec: `pdftotext`, `whisper`, `qmd`, `xlsx2csv`.

## Risks

- **watchdog cross-platform.** fsnotify Rename+Create ordering on
  macOS vs linux. Mitigation: behavioral test with synthetic events;
  keep polling fallback.
- **xlsx schema.** Python `openpyxl` extracts all sheets; Go port
  uses `xlsx2csv` exec OR `github.com/xuri/excelize/v2` library.
  Decision: stay exec for v1; revisit later.
- **Agent invocation.** `ingest-with-agent <path> <agent>` shells
  `claude` CLI. Stays exec.

## Verbs (high-level)

1. `ingest <path>`: detect format from extension, route to per-format
   worker, emit `AGENT-PROMPT|...` for steps 3-9 of WIKI.md §4.1.
2. `ingest <path> --agent=<cli>`: same but auto-runs the agent CLI.
3. `ingest-xlsx <path>`: extract sheets to per-sheet markdown,
   bookkeep, lint.
4. `ingest-git <spec>`: clone or refresh repo, transform per
   `ingest-git-transform.py`, write to `content/git/<repo>/...`.
5. `ingest-pdf <path>`: pdftotext extract, bookkeep.
6. `ingest-audio <path>`: whisper transcribe, bookkeep.
7. `watchdog`: watch `raw/inbox/batch/`, ingest new files; quarantine
   on failure to `_failed/`. Native fsnotify; fallback poll.
8. `capture <text>`: append to `content/inbox.md` with timestamp.

## Out of scope

- AI agent integration semantics.
- Format-specific extraction quality improvements.
- MCP ingest handlers.
