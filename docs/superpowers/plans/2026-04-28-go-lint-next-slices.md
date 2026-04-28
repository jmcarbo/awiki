# Go Lint Next Slices Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Migrate awiki lint from shell-delegated namespace checks to Go-owned lint slices without weakening existing lint, privacy, or test contracts.

**Architecture:** Keep `cmd/awiki` and `internal/cli` as the command surface, evolve `internal/lint` into a namespace registry, and move each deferred namespace into focused packages under `internal/lint/<namespace>`. External engines such as Hugo, DuckDB, qmd, and vl-convert remain explicit adapters in `internal/adapters`.

**Tech Stack:** Go, Bats, shell compatibility shims, existing awiki Markdown/YAML-ish parsers, Node test runner for query-node tests.

---

## File Structure

- Modify `internal/lint/diagnostic.go`: add optional rule-code support and five-field `LINT` records.
- Modify `internal/lint/engine.go`: preserve exit behavior, import four- and five-field records, route namespaces through a registry as packages land.
- Modify `internal/lint/*_test.go`: cover diagnostics, external-record import, and Hugo-check adapter behavior.
- Modify `tests/lint_test.bats`: isolate Hugo-check success from unrelated whole-wiki warnings.
- Create later `internal/lint/synth`: S1-S9 synth rules and safe synth fixes.
- Create later `internal/lint/task`: Go action parser and T1-T15 task rules.
- Create later `internal/lint/data`: D1-D9 dataset lint.
- Create later `internal/lint/chart`: C1-C9 and C-PRIV chart lint.
- Create later `internal/lint/query`: Q1-Q5 and Q-PRIV query lint after resolving doc/test contract mismatch.

## Task 1: Hugo-Check Cleanup

**Files:**
- Modify: `tests/lint_test.bats`
- Modify: `internal/lint/engine.go`
- Test: `internal/lint/engine_test.go`

- [ ] **Step 1: Verify the existing red test**

Run:

```sh
go build -o bin/awiki ./cmd/awiki
bats tests/lint_test.bats
```

Expected: one Hugo-check test fails because warning diagnostics affect the command status or output for an otherwise renderable fixture.

- [ ] **Step 2: Add a Go test for Hugo adapter status mapping**

Add table coverage in `internal/lint/engine_test.go` that calls `Run` with `HugoCheck: true`, a custom runner, and a clean temporary wiki. Cover success, missing Hugo (`127`), and render failure.

- [ ] **Step 3: Run the Go test to verify it fails if behavior is missing**

Run:

```sh
go test ./internal/lint -run 'TestRunHugoCheck' -count=1
```

Expected: fail until the test harness or implementation exposes the required behavior.

- [ ] **Step 4: Make the minimal implementation/test-fixture change**

Keep `Collector.ExitCode` unchanged. If the existing Go behavior is already correct, update only `tests/lint_test.bats` so the Hugo success case runs in an isolated clean fixture and asserts renderability rather than whole-repo warning state.

- [ ] **Step 5: Verify the slice**

Run:

```sh
go test ./...
bats tests/lint_test.bats
```

Expected: both pass.

- [ ] **Step 6: Commit**

```sh
git add internal/lint/engine.go internal/lint/engine_test.go tests/lint_test.bats
git commit -m "fix: isolate hugo lint check"
```

## Task 2: Diagnostic Rule-Code Compatibility

**Files:**
- Modify: `internal/lint/diagnostic.go`
- Modify: `internal/lint/diagnostic_test.go`
- Modify: `internal/lint/engine.go`
- Modify: `internal/lint/engine_test.go`

- [ ] **Step 1: Add failing diagnostic record tests**

Add tests that assert `Diagnostic{Level: Error, File: "x.md", Code: "D1", Message: "missing storage"}.Record()` emits `LINT|ERROR|x.md|D1|missing storage`, while diagnostics without `Code` keep the current four-field format.

- [ ] **Step 2: Add failing import tests**

Add tests for `importExternalRecords` that import both `LINT|ERROR|x.md|bad` and `LINT|ERROR|x.md|D1|bad` into `Collector.Diagnostics`.

- [ ] **Step 3: Implement optional `Code`**

Add `Code string` to `Diagnostic`, update `Record`, and update `importDiagnostic` to split both four- and five-field records.

- [ ] **Step 4: Verify**

Run:

```sh
go test ./internal/lint -count=1
go test ./...
```

Expected: pass.

- [ ] **Step 5: Commit**

```sh
git add internal/lint/diagnostic.go internal/lint/diagnostic_test.go internal/lint/engine.go internal/lint/engine_test.go
git commit -m "feat: support lint rule codes"
```

## Task 3: Namespace Registry Skeleton

**Files:**
- Create: `internal/lint/registry.go`
- Create: `internal/lint/registry_test.go`
- Modify: `internal/lint/engine.go`

- [ ] **Step 1: Add failing registry dispatch tests**

Add a test that registers a fake namespace and proves `Run` dispatches `--only=<namespace>` through the registry before falling back to legacy adapters.

- [ ] **Step 2: Implement minimal registry**

Create `RuleSet` and `Fixer` interfaces and a private default registry. Keep all current deferred namespace behavior unchanged until real namespace packages are added.

- [ ] **Step 3: Verify no behavior drift**

Run:

```sh
go test ./...
bats tests/lint_test.bats
bats tests/lint_synth_test.bats
```

Expected: pass, except for any documented pre-existing non-slice failure if encountered.

- [ ] **Step 4: Commit**

```sh
git add internal/lint/registry.go internal/lint/registry_test.go internal/lint/engine.go
git commit -m "feat: add lint namespace registry"
```

## Task 4: Synth Lint Port Plan Breakout

**Files:**
- Create: `docs/superpowers/plans/2026-04-28-go-synth-lint-port.md`

- [ ] **Step 1: Write the synth-specific implementation plan**

Decompose S1-S9 into smaller TDD tasks. Include exact fixtures from `tests/fixtures/wiki-synth`, exact Go files under `internal/lint/synth`, and the Bats gates:

```sh
go test ./...
bats tests/lint_synth_test.bats
bats tests/lint_synth_fuzzy_test.bats
bats tests/synth_e2e_test.bats
```

- [ ] **Step 2: Commit**

```sh
git add docs/superpowers/plans/2026-04-28-go-synth-lint-port.md
git commit -m "docs: plan go synth lint port"
```

## Task 5: Task/Data/Chart/Query Breakout Plans

**Files:**
- Create: `docs/superpowers/plans/2026-04-28-go-task-lint-port.md`
- Create: `docs/superpowers/plans/2026-04-28-go-data-chart-query-lint-port.md`

- [ ] **Step 1: Write the task-specific plan**

Include the Go action parser, in-memory action index, stale-map safeguards, T1-T15 tests, and conservative fix behavior.

- [ ] **Step 2: Write the data/chart/query plan**

Include D1-D9, C1-C9/C-PRIV, and Q1-Q5/Q-PRIV. Resolve the query contract mismatch before any query code changes.

- [ ] **Step 3: Commit**

```sh
git add docs/superpowers/plans/2026-04-28-go-task-lint-port.md docs/superpowers/plans/2026-04-28-go-data-chart-query-lint-port.md
git commit -m "docs: plan remaining go lint namespace ports"
```
