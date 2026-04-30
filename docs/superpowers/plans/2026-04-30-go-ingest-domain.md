# Go ingest domain port Implementation Plan

> Use superpowers:subagent-driven-development. Steps use `- [ ]`.

**Spec:** `docs/superpowers/specs/2026-04-30-go-ingest-slice-plan-design.md`
(refines `2026-04-29-go-ingest-domain-design.md`).

**Supersedes:** `docs/superpowers/plans/2026-04-29-go-ingest-domain.md`.
The earlier plan sketched 8 tasks; this plan locks the 10-PR slice
cadence the spec pins (infra → 3 leaves → 3 writers → 3 orchestrators →
cleanup) and matches the lint/synth/dataset/chart/query shipping pattern.

**Bash oracle:** ~2000 LOC across `scripts/ingest.sh`,
`ingest-{xlsx,git,pdf,audio}.sh`, `watchdog.sh`, `capture.sh`,
`scripts/lib/{xlsx-extract.py,git-clone.sh,git-state.sh,git-config.sh,
ingest-git-transform.py}`.

**Branch:** `feat/ingest-domain`. Each slice ships as its own PR off
this branch, merging back into `main` after sub-slice review.

**Pinned for v1 (per spec):**
- `xlsx2csv` stays exec; no `excelize/v2`.
- `git` stays exec for clone/fetch; no `go-git`.

## Slice 1: infra

- [ ] Create `internal/ingest/{types,runner}.go` skeleton (Format, Spec,
      Result, AgentPromptArgs, Runner with RepoRoot + adapter fields).
- [ ] Create `internal/cli/ingest.go` with flat dispatch returning
      `"verb not yet ported"` for every verb.
- [ ] Wire dispatch in `internal/cli/cli.go` switch (cases: `ingest`,
      `ingest-xlsx`, `ingest-git`, `ingest-pdf`, `ingest-audio`,
      `capture`, `watchdog`, `ingest-batch-list`, `ingest-git-list`).
- [ ] Add adapter stubs: `internal/adapters/{pdftotext,whisper,xlsx2csv,
      fsnotify,git_ext}.go` with interface only — no `os/exec` calls
      yet.
- [ ] Add fixture loader + golden differ helpers to
      `internal/testutil/` if missing (reuse synth/query/chart pattern).
- [ ] Green `go build ./...` and `go test ./internal/ingest/...`.
- [ ] Commit + open PR. Justfile + scripts unchanged. No shim wired.

## Slice 2: capture

- [ ] `internal/ingest/capture.go` — append timestamped line to
      `content/inbox.md`. Atomic write via `internal/fsutil`.
- [ ] Fixture: `tests/fixtures/ingest/capture/{input,args,
      expected_stdout,expected_files/}`.
- [ ] Test: `internal/ingest/capture_test.go` driving via fixture.
- [ ] Wire `awiki capture` in `internal/cli/ingest.go`.
- [ ] Edit `scripts/capture.sh` to shim once `bin/awiki capture` exists
      (mirror `scripts/synth.sh` shim shape).
- [ ] Run `bats tests/capture.bats` (if exists) against shim — assert
      green.
- [ ] Commit + PR.

## Slice 3: leaf listers

- [ ] `internal/ingest/list.go` — `ListBatch()` walks
      `raw/inbox/batch/` sorted; `ListGit()` lists
      `.awiki/git-state/*.json` slugs sorted.
- [ ] Fixtures: `tests/fixtures/ingest/{batch-list,git-list}/`.
- [ ] Tests for each.
- [ ] Wire `awiki ingest-batch-list` + `awiki ingest-git-list`.
- [ ] Justfile recipes already inline (`@find ...`, `@ls ...`); switch
      to `awiki` calls. Bash unchanged (these have no dedicated script).
- [ ] Commit + PR.

## Slice 4: ingest-pdf

- [ ] Implement `internal/adapters/pdftotext.go` (exec wrapper, struct
      fake-friendly).
- [ ] `internal/ingest/formats/pdf.go` — extract text via adapter, write
      `content/<slug>.md`, emit `INGEST|format=pdf|src=...|dst=...`.
- [ ] Fixture with adapter fake returning canned text:
      `tests/fixtures/ingest/pdf/`.
- [ ] Test pinning stdout + emitted files.
- [ ] Wire `awiki ingest-pdf` dispatch.
- [ ] `scripts/ingest-pdf.sh` becomes shim (mirror `synth.sh` pattern).
- [ ] Commit + PR.

## Slice 5: ingest-audio

- [ ] `internal/adapters/whisper.go` (exec wrapper).
- [ ] `internal/ingest/formats/audio.go` — same shape as pdf, emit
      `INGEST|format=audio|...`.
- [ ] Fixture with whisper fake: `tests/fixtures/ingest/audio/`.
- [ ] Test.
- [ ] Wire `awiki ingest-audio`.
- [ ] `scripts/ingest-audio.sh` shim.
- [ ] Commit + PR.

## Slice 6: ingest-xlsx

- [ ] `internal/adapters/xlsx2csv.go` — exec wrapper. Pass-through flags.
- [ ] `internal/ingest/formats/xlsx.go` — drive adapter, split per-sheet
      output to per-sheet markdown matching `lib/xlsx-extract.py` shape.
- [ ] Fixture with xlsx2csv fake + flag pass-through:
      `tests/fixtures/ingest/xlsx/`.
- [ ] Test.
- [ ] Wire `awiki ingest-xlsx`.
- [ ] `scripts/ingest-xlsx.sh` shim. Verify `--flags` pass-through.
- [ ] Commit + PR.

## Slice 7: ingest (orchestrator)

- [ ] `internal/ingest/bookkeep.go` — repo-root resolve, format detect,
      per-format dispatch (slices 4-6), archive source to
      `raw/inbox/_archive/`, log via `internal/ops.Log`, lint, qmd
      reindex, emit `AGENT-PROMPT|<string>`.
- [ ] Pin AGENT-PROMPT byte-format via golden fixture
      `tests/fixtures/ingest/ingest/`. Grep `scripts/ingest.sh:115` for
      exact form.
- [ ] Implement `--agent=<cli>` shell-out (stdin = AGENT-PROMPT line).
- [ ] Behavioral test for `--agent` with fake CLI script.
- [ ] Wire `awiki ingest <path>` + `awiki ingest --agent=<cli> <path>`.
- [ ] `scripts/ingest.sh` shim.
- [ ] Commit + PR.

## Slice 8: ingest-git

- [ ] Extend `internal/adapters/git_ext.go` with clone/init/fetch.
- [ ] Port `scripts/lib/git-state.sh` to
      `internal/ingest/git/state.go` (read/write
      `.awiki/git-state/<slug>.json`).
- [ ] Port `scripts/ingest-git-transform.py` (~269 LOC) to
      `internal/ingest/git/transform.go`. Split per-section commit if
      large.
- [ ] Driver in `internal/ingest/git/run.go` orchestrates
      clone/fetch → state → transform → bookkeep.
- [ ] Fixture with git fake: `tests/fixtures/ingest/git/`.
- [ ] Tests for state, transform, run.
- [ ] Wire `awiki ingest-git`.
- [ ] `scripts/ingest-git.sh` shim.
- [ ] Commit + PR.

## Slice 9: watchdog

- [ ] `internal/adapters/fsnotify.go` — native
      `github.com/fsnotify/fsnotify` wrapper.
- [ ] `internal/ingest/watchdog.go` — watch loop with 500 ms debounce,
      quarantine to `raw/inbox/batch/_failed/` on ingest error,
      `--catchup` initial drain, `--once` single-event mode for tests,
      `--poll-interval=2s` polling fallback.
- [ ] Behavioral test with synthetic events asserting:
      single-write triggers one ingest, rapid-multi-write triggers one
      ingest (debounce), failed ingest moves to `_failed/`.
- [ ] Emit `WATCHDOG|event=...|path=...[|err=...]`.
- [ ] Wire `awiki watchdog`.
- [ ] `scripts/watchdog.sh` shim.
- [ ] Commit + PR.

## Slice 10: cleanup

- [ ] Verify each verb has Go test parity. Audit bats coverage gaps and
      backfill before deletion.
- [ ] Confirm one release shipped with shims active and no parity bug
      filed.
- [ ] Delete: `scripts/{ingest,ingest-xlsx,ingest-git,ingest-pdf,
      ingest-audio,watchdog,capture}.sh`, `scripts/lib/{xlsx-extract,
      ingest-git-transform,git-clone,git-state,git-config}.{sh,py}`,
      `scripts/ingest-git-transform.py`.
- [ ] Update `justfile` recipes to call `awiki` directly (drop
      `bash scripts/...sh` prefix).
- [ ] Drop bats files now covered by Go tests.
- [ ] Update `CHANGELOG.md` with deletion + caller-impact note.
- [ ] Run `bats tests/` (remaining suites) — green.
- [ ] Run `go test ./...` — green.
- [ ] Commit + PR.

## Risks

- **Watchdog cross-platform.** macOS vs linux Rename+Create ordering.
  Mitigation: behavioral test with synthetic events; polling fallback.
- **AGENT-PROMPT drift.** External agents and hooks consume the record.
  Mitigation: golden fixture in slice 7; grep external repos before
  slice 10 deletion.
- **`ingest-git-transform.py` size.** 269 LOC of semantic transforms.
  Mitigation: rich golden fixtures in slice 8; split commit per-section.
- **xlsx flag pass-through drift.** Justfile uses `*flags` splat.
  Mitigation: per-flag fixture in slice 6.
- **Bats coverage gaps.** Some verbs may lack bats today. Mitigation:
  audit in slice 10 and backfill behavioral Go test before deletion,
  not after.

## Out of scope

- Cleanup slices for other domains (lint/synth/dataset/chart/query).
- Format-specific extraction quality improvements.
- MCP ingest handlers.
- New verbs beyond today's justfile.
