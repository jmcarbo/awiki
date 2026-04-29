# Go ingest domain port Implementation Plan

> Use superpowers:subagent-driven-development.

**Goal:** port `awiki ingest <path>`, per-format workers, watchdog,
capture per `docs/superpowers/specs/2026-04-29-go-ingest-domain-design.md`.

**Bash oracle:** ~2000 LOC across `scripts/ingest.sh`,
`ingest-{xlsx,git,pdf,audio}.sh`, `watchdog.sh`, `capture.sh`,
`scripts/lib/{xlsx-extract.py,git-clone.sh,git-state.sh,git-config.sh,
ingest-git-transform.py}`.

**Branch:** `feat/ingest-domain`. Recommend splitting into multiple
PRs given size; each verb is a natural sub-slice.

## Tasks

### Task 1: skeleton + adapters

- Adapters: `internal/adapters/{pdftotext,whisper,xlsx2csv,fsnotify}.go` per the umbrella roadmap. `fsnotify` is native (`github.com/fsnotify/fsnotify`); rest exec.
- `internal/ingest/{types,runner}.go` with paths + adapter fields.
- Commit per adapter to keep diffs small.

### Task 2: capture verb

- `internal/ingest/capture.go` — append timestamped line to `content/inbox.md`. Smallest verb; ship first.
- Wire to flat verb `awiki capture`.

### Task 3: ingest <path> bookkeeping flow

- `internal/ingest/bookkeep.go` — auto-detects format by extension; routes to per-format worker; emits `AGENT-PROMPT|<single-string>` for steps 3-9 of WIKI.md §4.1.
- Wire flat verb `awiki ingest`.

### Task 4: ingest-xlsx

- `internal/ingest/xlsx/extract.go` — port `xlsx-extract.py` semantics (extract sheets to per-sheet markdown). Stay exec on `xlsx2csv` if simpler.
- Wire flat verb `awiki ingest-xlsx`.

### Task 5: ingest-git

- `internal/ingest/git/clone.go` + `state.go` + `transform.go` — port the trio. Use `go-git` for clone where straightforward; exec git for fetch/checkout if go-git friction.
- Port `ingest-git-transform.py` (269 LOC) — biggest python in this domain.
- Wire flat verb `awiki ingest-git`.

### Task 6: ingest-pdf, ingest-audio

- Both small wrappers shelling `pdftotext` / `whisper`.
- Wire flat verbs.

### Task 7: watchdog

- `internal/ingest/watchdog.go` — native `fsnotify` watcher on `raw/inbox/batch/`; quarantine to `_failed/` on ingest error.
- Polling fallback for environments without fsnotify support.
- Wire flat verb `awiki watchdog`.

### Task 8: regression + merge

Each of Tasks 2-7 should ship as its own commit; merge once all pass and bash-compare smokes are clean.

## Risks

- Watchdog cross-platform (Rename+Create ordering on macOS).
- xlsx2csv encoding edge cases.
- ingest-git-transform.py is large; split per-section if a single subagent struggles.

## Out of scope

- Cleanup slice (delete bash + python).
- Format-specific extraction quality improvements.
