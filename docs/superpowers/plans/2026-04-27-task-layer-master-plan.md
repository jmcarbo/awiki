# awiki Task Layer Implementation Plan — Master

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement each phase task-by-task. Per-phase plans use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build the awiki task layer — an opt-in personal-action-management feature on top of awiki v1, supporting capture, seven-outcome triage, project / context page types, inline-checkbox actions with key:value tail metadata, scanner-built agenda views, recurrence, weekly review, all schema-enforced and MCP-exposed.

**Spec:** [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md)

**Master v1 plan (depends on):** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)

**Architecture:** Action lines are inline checkboxes on project / context pages — single source of truth. Capture inbox at `content/inbox.md` for typed thoughts; existing `raw/inbox/interactive/` for file-shaped capture. A shared bash grammar library (`scripts/lib/action-grammar.sh`) is sourced by scanner, lint, recur, and triage so the parser cannot drift. Scanner emits `actions.tsv` (canonical) + `actions-rejected.tsv` (lint consumes). Agenda pages under `content/agenda/` are derived artifacts — managed-region markers, atomic-rename writes, regenerated on cadence. Privacy filter excludes actions on private origin files (path-based, frontmatter-tag-based, or wikilink-target-based). Concurrency via `flock -x .awiki/lock` (advisory) for every mutating script and MCP handler. Seven-outcome triage MCP tool with bash-fallback `triage.sh --interactive`. Recurrence chains use `^<base>~<n>` ID namespace (or `^<base>__<n>` if phase-17 spike falls back). Weekly review surfaced via structured `REVIEW|` lines + JSON-shaped MCP `review_status()`.

**Tech Stack:** bash 4+, `flock` (util-linux on Linux, Homebrew `util-linux` on macOS), just 1.13+, hugo 0.120+ extended, hugo-book theme, bats-core 1.10+, python3 3.8+, Node 20+ with `@modelcontextprotocol/sdk` (phase 18 only). Reuses awiki v1 phases 1-3, 6, 8 infrastructure.

---

## Phase Index

Each phase ships independently with its own branch, tests, and merge gate. Phase 16 is the foundation; 17 builds the scanner + agenda + mechanical lint; 18 adds triage + recurrence + MCP tools + semantic lint; 19 closes the loop with review tooling, encrypt-init integration, sample-wiki, and README.

| # | Phase | Plan file | Depends on |
|---|---|---|---|
| 16 | Schema + scaffold | [`phase-16-task-schema-scaffold`](./2026-04-27-phase-16-task-schema-scaffold.md) | v1 phases 1-3, 6 |
| 17 | Scanner + agenda + mechanical lint | [`phase-17-task-scanner-agenda`](./2026-04-27-phase-17-task-scanner-agenda.md) | 16 |
| 18a | Triage + recurrence + semantic lint (bash) | [`phase-18a-task-triage-recur-bash`](./2026-04-27-phase-18a-task-triage-recur-bash.md) | 16, 17 |
| 18b | MCP server extension (Node) | [`phase-18b-task-mcp-server`](./2026-04-27-phase-18b-task-mcp-server.md) | 16, 17, 18a, v1 phase 8 |
| 19 | Review + encryption + polish + samples | [`phase-19-task-review-polish`](./2026-04-27-phase-19-task-review-polish.md) | 18a, 18b |

Phase 18 is split because the bash side (triage.sh, action-recur.sh, lint T8-T13, log-append poisoning) and the Node side (MCP server +7 tools) are independent codebases with separate test harnesses. Merge order is 18a → 18b (the MCP layer shells out to triage.sh / action-recur.sh and depends on the lint extensions).

Phases 16-19 are governed by [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md).

## Recommended ordering

- **Strict dependency order:** 16 → 17 → 18a → 18b → 19. Each phase's tests depend on the prior phase's deliverables.
- **No parallelism within the task layer.** The shared grammar library (16) is foundational; the scanner (17) reads what triage (18a) writes; the MCP server (18b) shells out to 18a's triage/recur scripts; the review tooling (19) reads what 17 + 18a + 18b produce. Parallel branch merges produce surprising lint regressions.
- The whole task layer is opt-in for end users — none of the 16-19 deliverables fire unless the user runs `just task-init`. So merge of phase 16 alone is safe to ship even before 17-19 land.

## How to use

1. Read the [spec](../specs/2026-04-27-task-layer-design.md) once. Pay attention to "Concurrency & Atomicity", "Inline Action Grammar", and the seven triage outcomes.
2. Pick a phase. Open its plan file.
3. Create the branch declared at the top of the phase plan.
4. Walk the tasks top-to-bottom, ticking checkboxes as you go.
5. At the end of the phase, run `bats tests/ && just lint` and merge.

## Cross-phase rules

- **Spike absorption.** Phase 17 opens with a 1-day spike covering (a) Obsidian Tasks plugin Dataview-mode interop, (b) `~`-separator block-ID round-trip in Obsidian and Hugo. If `~` round-trip fails, the spike falls back to `__` and patches T14 + spec inline before the rest of phase 17 proceeds. Phase 18's recur-emit code reads the active separator from a constant exported by phase 17's spike, so the choice ripples cleanly.
- **Cumulative test gate.** Every phase merge requires `bats tests/ && just lint` clean on the merged result. The optional pre-commit hook (installed by `task-init` step 6 in phase 19) enforces this locally for users who opt in.
- **Shared grammar library is canonical.** Every consumer of action-line syntax (scanner, lint, recur, triage) sources `scripts/lib/action-grammar.sh`. New consumers MUST source it; do not re-implement the regex set.
- **Lock-acquisition is mandatory.** Every mutating script and every mutating MCP handler acquires `flock -x .awiki/lock` via `scripts/lib/lock.sh`. Read-only paths take `flock -s`. No bypass.
- **No deferrals.** Anything unfinished in a phase stays in the same phase. Do not push work to a later phase or to a v2 bucket. The five phase plans listed (16, 17, 18a, 18b, 19) are the entire task layer.

## Self-Review Checklist (run after completing all phases)

- [ ] Every spec section has at least one task implementing it (cross-check spec section list against the five phase plans: 16, 17, 18a, 18b, 19).
- [ ] No "TBD", "TODO", "implement later" anywhere across the five per-phase plans.
- [ ] Every consumer of action-line syntax sources `scripts/lib/action-grammar.sh` — no duplicate regex anywhere.
- [ ] Every mutating script and every mutating MCP handler acquires the lock.
- [ ] All seven triage outcomes have a tested side-effect path in both `triage.sh` and the MCP `triage_apply` tool.
- [ ] All 15 lint rules (T1-T15) have a test fixture firing them and a fixture proving they stay silent on legitimate content.
- [ ] Block-ID format consistent: 8-char base32 mint with collision-retry; user-typed `^[a-z0-9]{3,16}$`; recurrence chain `^<base>~<n>` (or `__<n>` per phase-17 spike outcome).
- [ ] All scripts use `#!/usr/bin/env bash` and `set -euo pipefail`.
- [ ] All recipes match script signatures (incl. variadic quoting note for `capture *text`).
- [ ] Phase dependencies stated and respected.
- [ ] Commit messages use Conventional Commits. No co-author trailers. No emojis.
- [ ] `bats tests/ && just lint` clean on `main` after phase 19 merge.
- [ ] Smoke test from spec §"Manual smoke test" runs end-to-end against `examples/sample-wiki/`.

## Execution Notes

- **Recommended runner:** subagent-driven-development. Dispatch one subagent per task; review between tasks. The grammar library + MCP server tasks especially benefit from review checkpoints because their outputs ripple downstream.
- **No parallel batches.** Strict 16 → 17 → 18 → 19. Use a single feature branch per phase; merge to main between phases.
- **Worktree hygiene:** if executing alongside other awiki work, use `git worktree add` so the task layer's `content/` mutations don't collide with concurrent v1 fixes.

## Per-phase deliverable summary

| # | Phase | Headline deliverables |
|---|-------|------------------------|
| 16 | Schema + scaffold | `task-init.sh` (per-step idempotent), `lib/action-grammar.sh` + `lib/lock.sh`, `capture.sh` with full sanitization, page-type enum extension, system pages, `task-init` + `capture` justfile recipes, dep-check `flock`, BATS for init / capture / grammar / lock. |
| 17 | Scanner + agenda | Spike (Obsidian Tasks Dataview + `~` round-trip with `__` fallback), `action-scan.sh` (with `source_kind` privacy column + inbox-line ID synthesis + alias-map private-target detection), `agenda.sh` (5 managed-region pages, atomic rename, privacy filter with redacted placeholder, `last_updated` rewrite), justfile `agenda` + `scan`, lint T1-T7 + T14 + T15, `--fix` for date / key / id (8-char retry), BATS for scan / agenda / lint. |
| 18a | Triage + recur + semantic lint (bash) | `triage.sh` (CLI + `--interactive` per-item-atomic with regex-validated prompts), `action-recur.sh` (clamp arithmetic, separator-from-spike, ≥200 refuse-to-emit), `log-append.sh` poisoning protection, lint T8-T13 (T8 exempts `_loose` / `_someday`), BATS for triage / recur / log-poisoning. |
| 18b | MCP server (Node) | MCP server +7 tools (`capture`, `triage_inbox`, `triage_apply`, `list_actions`, `rebuild_agenda`, `review_status` stub, `mark_review_done` stub) with full sanitization + path-resolution guard + lock acquisition + `sanitizations_applied[]` + `stale_id` re-verify. JS lib split: `sanitize-capture.js`, `inbox-id.js`, `triage-validate.js`, `path-guard.js`, `lock.js`. BATS `mcp_task_test.bats`. |
| 19 | Review + polish | `review-status.sh` emitting structured `REVIEW|...` lines, MCP `review_status()` full impl, MCP `mark_review_done()` writing `.awiki/last-review` + appending `review-log.md`, justfile `review`, WIKI.md weekly-review subsection, pre-commit hook installer (idempotent, marker-tagged), `encrypt-init` extension auto-including `content/inbox.md` + `content/agenda/**` when `AWIKI_TASK_LAYER=on`, README task-layer section, `docs/just-help.txt` extension, sample-wiki extended, BATS for review / mark-done / encrypt-init / sample-smoke. Full smoke test green. |

## Definition of done (entire task layer)

- All 5 phase plans (16, 17, 18a, 18b, 19) merged to `main`.
- `bats tests/ && just lint` clean.
- Spec smoke test from "Manual smoke test (added to README task-layer section)" runs end-to-end against `examples/sample-wiki/`.
- Sample wiki at `examples/sample-wiki/` ships `content/inbox.md`, `content/projects/{renovate-kitchen,q3-launch}.md`, `content/contexts/{phone,errands,computer,home}.md`, populated `content/agenda/*.md`, and a `content/agenda/review-log.md` entry — all hand-curated examples that pass lint.
- README documents the manual smoke test + the methodology one-paragraph note (capture / clarify / organize / reflect / engage; trademark name avoided).
- `encrypt-init` covers task-layer pages by default for new wikis with `AWIKI_TASK_LAYER=on`.
- The optional pre-commit hook installer works idempotently (marker-tagged so re-runs skip).
- Master plan checklist (above) all checked.
