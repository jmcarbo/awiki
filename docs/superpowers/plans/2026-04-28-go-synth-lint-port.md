# Go Synth Lint Port Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace `scripts/lint-synth.sh` and its Python lint helpers with Go-owned S1-S9 synthesis lint.

**Architecture:** Add `internal/lint/synth` as a focused namespace package registered through the lint namespace registry. Keep synthesis generation scripts unchanged; only lint and safe synth-fix behavior move to Go.

**Tech Stack:** Go, Bats, Markdown region parsing, Unicode normalization, git diff adapter for S6.

---

## File Structure

- Create `internal/lint/synth/region.go`: generated-region and feedback-section parsing.
- Create `internal/lint/synth/normalize.go`: S3 normalization helpers.
- Create `internal/lint/synth/scope.go`: scope resolution and scope hash checks.
- Create `internal/lint/synth/rules.go`: S1-S9 diagnostics.
- Create `internal/lint/synth/fix.go`: generated-region-only S3 normalization fixes.
- Create `internal/lint/synth/*_test.go`: unit coverage for each helper and rule group.
- Modify `internal/lint/engine.go`: register synth namespace and remove synth from legacy fallback after gates pass.
- Test with `tests/lint_synth_test.bats`, `tests/lint_synth_fuzzy_test.bats`, and `tests/synth_e2e_test.bats`.

## Task 1: Region Parser

- [ ] Add failing tests for `ParseGeneratedRegion` covering exactly one BEGIN/END, double BEGIN, missing END, and BEGIN after END.
- [ ] Implement `ParseGeneratedRegion` returning marker positions, generated body, and diagnostics data.
- [ ] Run `go test ./internal/lint/synth -run TestParseGeneratedRegion -count=1`.
- [ ] Commit `feat: parse synth generated regions`.

## Task 2: S1/S2/S7/S8 Structural Rules

- [ ] Add tests using fixtures from `tests/fixtures/wiki-synth/content/synthesis`.
- [ ] Implement S1 marker integrity.
- [ ] Implement plugin manifest required-section loading for S2.
- [ ] Implement S7 `feedback_count=N` info plus warning above 20.
- [ ] Implement S8 out-of-scope feedback wikilink warnings.
- [ ] Run `go test ./internal/lint/synth -count=1` and `bats tests/lint_synth_test.bats`.
- [ ] Commit `feat: port structural synth lint rules`.

## Task 3: S3 Quote Normalization

- [ ] Add table tests for smart quotes, NFC/NFD, NBSP, zero-width spaces, hyphen variants, whitespace collapse, multi-paragraph quotes, and wikilink-to-title rewrite.
- [ ] Implement normalization without shelling to Python.
- [ ] Implement exact evidence substring validation.
- [ ] Run `go test ./internal/lint/synth -run 'TestNormalize|TestS3' -count=1`.
- [ ] Commit `feat: port synth evidence quote matching`.

## Task 4: Fuzzy Suggestions

- [ ] Add failing tests matching `tests/lint_synth_fuzzy_test.bats` expectations.
- [ ] Implement a bounded fuzzy suggestion over normalized source text.
- [ ] Run `go test ./internal/lint/synth -run TestFuzzy -count=1` and `bats tests/lint_synth_fuzzy_test.bats`.
- [ ] Commit `feat: add synth quote suggestions`.

## Task 5: S4/S5 Scope Rules

- [ ] Add tests for tag, explicit slug, and query scope descriptors.
- [ ] Implement resolved-scope citation checks for S4.
- [ ] Implement scope hash recomputation for S5 and skip query-scoped pages.
- [ ] Run `go test ./internal/lint/synth -run 'TestScope|TestS4|TestS5' -count=1`.
- [ ] Commit `feat: port synth scope lint`.

## Task 6: S6/S9 and Fixes

- [ ] Add tests for generated-region diff detection and evidence word caps.
- [ ] Implement S6 using an adapter around `git diff` only for the diff source.
- [ ] Implement S9 aggregate evidence word counts.
- [ ] Implement `--only=synth --fix` normalization limited to generated regions.
- [ ] Run `go test ./...`, `bats tests/lint_synth_test.bats`, `bats tests/lint_synth_fuzzy_test.bats`, and `bats tests/synth_e2e_test.bats`.
- [ ] Remove synth from `deferredNamespaces()`.
- [ ] Commit `feat: run synth lint in go`.
