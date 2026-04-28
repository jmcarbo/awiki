# Go Task Lint Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace task T1-T15 lint in `scripts/lint.sh` with Go-owned task lint that scans current page content directly.

**Architecture:** Add `internal/lint/task` with an action grammar parser, in-memory action index, managed-region checks, date checks, and conservative fix behavior. Keep capture, recurrence emission, agenda rendering, and standalone `action-scan.sh` migration out of this plan.

**Tech Stack:** Go, Bats, existing task fixtures, UTC date parsing.

---

## File Structure

- Create `internal/lint/task/parser.go`: action-line parsing and tail metadata.
- Create `internal/lint/task/index.go`: current-content action index and context/project lookup.
- Create `internal/lint/task/rules.go`: T1-T15 diagnostics.
- Create `internal/lint/task/fix.go`: safe date normalization and ID minting.
- Create `internal/lint/task/maps.go`: compatibility map comparison/refresh behavior.
- Modify `internal/lint/engine.go`: register task namespace and remove task legacy fallback after gates pass.

## Task 1: Parser and T1/T3/T4/T14/T15

- [ ] Add parser tests for status markers, known tail keys, bad dates, block-ID shape, and indented continuations.
- [ ] Implement parser and diagnostics for T1, T3, T4, T14, and T15.
- [ ] Run `go test ./internal/lint/task -count=1`.
- [ ] Commit `feat: parse task actions in go`.

## Task 2: Index and T2/T5/T6

- [ ] Add tests using `tests/fixtures/wiki-task-good` and `tests/fixtures/wiki-task-broken`.
- [ ] Implement duplicate ID checks for T2, including recurrence chain instances.
- [ ] Implement `[?]` without `wait:` as T5.
- [ ] Implement `@context` alias resolution as T6 while ignoring prose and code blocks.
- [ ] Run `go test ./internal/lint/task -count=1`.
- [ ] Commit `feat: port task action index lint`.

## Task 3: Managed Regions and Review Checks

- [ ] Add tests for T7, T8, T9, T10, T11, T12, and T13.
- [ ] Implement agenda managed-region hand-edit detection.
- [ ] Implement active-project no-next-action warnings and context-unused info.
- [ ] Implement UTC overdue and stale-review date checks.
- [ ] Implement recurrence-chain warn/error thresholds.
- [ ] Run `go test ./internal/lint/task -count=1`.
- [ ] Commit `feat: port task review lint`.

## Task 4: Safe Fixes and Map Freshness

- [ ] Add tests for date normalization, ID minting, idempotence, continuation refusal, and stale map behavior.
- [ ] Implement safe fixes only for tested single-line actions.
- [ ] Refresh compatibility maps under `--fix` when current scan differs, or emit a diagnostic without `--fix`.
- [ ] Run `go test ./...`, `bats tests/lint_task_test.bats`, `bats tests/action_grammar_test.bats`, `bats tests/action_scan_test.bats`, `bats tests/agenda_test.bats`, and `bats tests/review_status_test.bats`.
- [ ] Remove task from `deferredNamespaces()`.
- [ ] Commit `feat: run task lint in go`.
