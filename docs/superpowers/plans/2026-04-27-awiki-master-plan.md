# awiki Implementation Plan — Master

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement each phase task-by-task. Per-phase plans use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build awiki, a domain-agnostic, multi-agent template repository for LLM-maintained personal wikis.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)

**Architecture:** Three layers — immutable raw sources, LLM-owned Hugo `content/` markdown wiki, and a `WIKI.md` schema. Thin bash scripts handle bookkeeping (ingest, lint, log, qmd index, encrypt). A `justfile` exposes recipes. qmd (qntx-labs fork) provides hybrid search; index-first retrieval with grep fallback. Bootstrap is agent-driven via `BOOTSTRAP.md`. v1 ships in 12 phases.

**Tech Stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only), entr or fswatch (phase 3 watcher), pdftotext / whisper-cpp (phase 11 optional).

---

## Phase Index

Each phase ships independently with its own branch, tests, and merge gate. Phases 1-2 are foundational; 3-7 are mostly parallelizable after their declared dependencies; 8-12 layer on top.

| # | Phase | Plan file | Depends on |
|---|---|---|---|
| 1 | Skeleton | [`phase-01-skeleton`](./2026-04-27-phase-01-skeleton.md) | — |
| 2 | Scripts core | [`phase-02-scripts-core`](./2026-04-27-phase-02-scripts-core.md) | 1 |
| 3 | Hugo render | [`phase-03-hugo-render`](./2026-04-27-phase-03-hugo-render.md) | 1 |
| 4 | qmd integration | [`phase-04-qmd-integration`](./2026-04-27-phase-04-qmd-integration.md) | 1 |
| 5 | Encryption | [`phase-05-encryption`](./2026-04-27-phase-05-encryption.md) | 1, 2 |
| 6 | Section indexes + catalog v2 | [`phase-06-section-indexes`](./2026-04-27-phase-06-section-indexes.md) | 1, 2, 3 |
| 7 | Slug rename, deletion, alias resolution | [`phase-07-rename-delete`](./2026-04-27-phase-07-rename-delete.md) | 2, 3 |
| 8 | MCP wiki-ops server | [`phase-08-mcp-server`](./2026-04-27-phase-08-mcp-server.md) | 2, 4, 6 |
| 9 | Scheduled lint configs | [`phase-09-scheduled-lint`](./2026-04-27-phase-09-scheduled-lint.md) | 2 |
| 10 | Auto-deploy templates | [`phase-10-auto-deploy`](./2026-04-27-phase-10-auto-deploy.md) | 3, 5 |
| 11 | Multimodal ingest helpers | [`phase-11-multimodal-ingest`](./2026-04-27-phase-11-multimodal-ingest.md) | 2, 5 |
| 12 | Polish & examples | [`phase-12-polish-examples`](./2026-04-27-phase-12-polish-examples.md) | all prior |

## Recommended ordering

- Strict dependency order: 1 → 2 → 3 → 4 → 5 → 6 → 7 → 8 → 9 → 10 → 11 → 12.
- Parallelizable batches once 1+2 land:
  - Batch A: 3, 4, 5, 9 (no cross-dependencies after 1-2).
  - Batch B (after batch A merged): 6 (needs 3), 7 (needs 2+3), 11 (needs 2+5).
  - Batch C: 8 (needs 2+4+6), 10 (needs 3+5).
  - Final: 12.

## How to use

1. Read the [spec](../specs/2026-04-27-llm-wiki-scaffold-design.md) once.
2. Pick a phase. Open its plan file.
3. Create the branch declared at the top of the phase plan.
4. Walk the tasks top-to-bottom, ticking checkboxes as you go.
5. At the end of the phase, run `bats tests/ && just lint` and merge.

## Cross-phase rules

- **Spike absorption.** Phases 3 (Hugo wikilinks) and 4 (qmd build) each open with a spike task. If the spike outcome contradicts the spec, update the spec FIRST, then revise affected tasks before continuing.
- **Cumulative test gate.** Every phase merge requires `just test && just lint` clean on `main`.
- **No skipped TDD.** Every phase's first script-shipping task writes a BATS test before the implementation.
- **No deferrals.** Anything unfinished in a phase stays in the same phase. Do not push work to a later phase or to a v2 bucket.

## Self-Review Checklist (run after completing all phases)

- [ ] Every spec section has at least one task implementing it.
- [ ] No "TBD", "TODO", "implement later" anywhere across the per-phase plans.
- [ ] Type/method/property names consistent across phases (e.g. `LINT|<level>|<file>|<msg>` everywhere).
- [ ] Every code block compiles or is valid syntax for its language.
- [ ] Test code precedes implementation in every TDD task.
- [ ] All scripts use `#!/usr/bin/env bash` and `set -euo pipefail`.
- [ ] All recipes match script signatures (incl. variadic quoting).
- [ ] Phase dependencies stated and respected.
- [ ] Commit messages use Conventional Commits.

## Execution Notes

- **Recommended runner:** subagent-driven-development. Dispatch one subagent per task; review between tasks.
- **Parallelism:** see "Recommended ordering" above. Use git worktrees for parallel batches to avoid shared-state collisions.
- **Phase merge:** prefer `git merge --no-ff phase-N-...` to preserve the phase boundary in history.
- **Tag at end:** after phase 12 merges, tag `v1.0.0`.
