# Template Update Mechanism — Master Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement each phase task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a manifest-driven update mechanism so repositories that bootstrapped from awiki can pull template updates without losing per-domain customizations.

**Architecture:** Detached-template + manifest. Bootstrapped repo records template provenance in `.awiki/template.json`; `just template-update` fetches a fresh template, syncs by per-path strategy, runs versioned migrations, prompts for new bootstrap steps, and writes everything onto a dedicated update branch. Default = dry-run. State persisted between phases for crash recovery.

**Tech Stack:** bash 4+, `git merge-file` (3-way), `git-crypt` (encryption preflight), TOML (manifest), JSON (provenance + state), BATS (tests).

**Spec:** [`docs/superpowers/specs/2026-04-27-template-update-design.md`](../specs/2026-04-27-template-update-design.md)

---

## Phase plans

| # | File                                                                                       | Scope                                                                  |
|---|--------------------------------------------------------------------------------------------|------------------------------------------------------------------------|
| 1 | [tu-phase-01-manifest-config-provenance.md](2026-04-27-tu-phase-01-manifest-config-provenance.md) | Manifest parser (glob precedence), `.awiki/config`, `template-init.sh`, `template.json` writer, content_hash, `original_repo`. |
| 2 | [tu-phase-02-plan-merge-helpers.md](2026-04-27-tu-phase-02-plan-merge-helpers.md)                 | `template-plan.sh`, `template-merge.sh`, `template-attr-audit.sh`, `template-source-check.sh`. |
| 3 | [tu-phase-03-preflight-fetch.md](2026-04-27-tu-phase-03-preflight-fetch.md)                       | Orchestrator Phase 0a (pre-fetch), Phase 1 (fetch + ancestor auto-recovery), Phase 0b (post-fetch checks + signature verification). |
| 4 | [tu-phase-04-schema-upgrade-plan-emit.md](2026-04-27-tu-phase-04-schema-upgrade-plan-emit.md)     | Phase 1.5 schema-upgrade flow (Commit 0); Phase 2 plan emit (formal `PLAN|...` output).         |
| 5 | [tu-phase-05-commit-a-sync.md](2026-04-27-tu-phase-05-commit-a-sync.md)                           | Phase 3 Commit A (sync: overwrite, three_way, attributes_merge, new_files, deletions).          |
| 6 | [tu-phase-06-commit-b-migrations.md](2026-04-27-tu-phase-06-commit-b-migrations.md)               | Phase 3 Commit B (mechanical migrations with stripped env + `touches:` enforcement + LLM staging with scope_glob + risk handling). |
| 7 | [tu-phase-07-commit-c-d-bootstrap-provenance.md](2026-04-27-tu-phase-07-commit-c-d-bootstrap-provenance.md) | Phase 3 Commit C (bootstrap-steps + content_hash + dangerous-step skip) and Commit D (single template.json write + cache rotation). |
| 8 | [tu-phase-08-recovery-flows.md](2026-04-27-tu-phase-08-recovery-flows.md)                         | `--continue`, `--abort`, `--re-pin`, `--rerun-bootstrap-step`, `--gc`, `--non-interactive`.    |
| 9 | [tu-phase-09-bootstrap-lint-availability.md](2026-04-27-tu-phase-09-bootstrap-lint-availability.md) | BOOTSTRAP step IDs + new `template-init` step; lint additions; update-availability check.       |
|10 | [tu-phase-10-retrofit-tests-docs.md](2026-04-27-tu-phase-10-retrofit-tests-docs.md)               | `template-retrofit.sh` for pre-v1 repos; full BATS matrix; CI E2E; WIKI.md workflow; docs.      |

---

## Dependencies

```
Phase 1 ──► Phase 2 ──► Phase 3 ──► Phase 4 ──► Phase 5 ──► Phase 6 ──► Phase 7
                                                                            │
                                                                            ▼
                                                                         Phase 8
                                                                            │
                                                                            ▼
                                  Phase 9  ◄────────── (parallel-eligible after Phase 1)
                                  Phase 10 ◄────────── (after Phase 8)
```

- **Phase 1** is the foundation — every later phase depends on the manifest parser, `template.json` writer, and `template-init.sh`.
- **Phases 2–8** are sequential per orchestrator phase boundaries.
- **Phase 9** (BOOTSTRAP step IDs + lint) can begin in parallel after Phase 1 lands; the update-availability check needs Phase 1 too.
- **Phase 10** (retrofit, tests, docs) needs Phase 8 complete (recovery flows are the most-tested surface).

## Execution discipline

- **TDD strict.** Each task = write failing test → run + observe failure → minimal implementation → run + observe pass → commit.
- **One concept per commit.** No bundling refactors with feature work.
- **Run tests after every task, not just end-of-phase.**
- **No `--no-verify` on commits.** Lint failures = fix them.
- **Branch per phase.** `feat/template-update-phase-NN`. PR each phase to main with summary.
- **Gating between phases.** Code review of each phase before starting next (subagent-driven-development surfaces this naturally).

## Test fixtures

Phase 10 creates `tests/fixtures/template-update/` with:
- `template-v0/` — minimal template at "old" pin.
- `template-v1/` — same template + one new script + one mechanical migration + one bootstrap step.
- `template-v2/` — `template-v1` + LLM prompt migration + dangerous-step content change.
- `template-v3-schema-bump/` — same content as v1 but `schema_version=2` + `schema-1-to-2.sh`.
- `bootstrapped-fresh/` — repo bootstrapped from `template-v0`, ready to update.
- `bootstrapped-customized/` — bootstrapped + user added wiki pages, edited hybrid files.

Earlier phases ship targeted fixtures as needed.

## Definition of done (master)

- All 10 phase plans complete and merged.
- `just template-update --status` works on a freshly bootstrapped repo.
- E2E test (Phase 10) passes: bootstrap from `template-v0` → mutate to `template-v1` → run update → verify diff matches expected.
- `examples/sample-wiki/` updated to a v1.0.0-bootstrapped state.
- Lint clean across awiki repo with new rules active.
- Docs published: `docs/template-update.md`, ADR, `migrations/README.md`, README mention.

## Tag

After Phase 10 merges and full E2E passes, tag awiki release `v1.0.0`. This is the floor version: only repos bootstrapped from `>=v1.0.0` get the clean update path. Older repos use the retrofit script.
