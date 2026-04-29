# Go ops domain port Implementation Plan

> Use superpowers:subagent-driven-development.

**Goal:** port awiki ops verbs per
`docs/superpowers/specs/2026-04-29-go-ops-domain-design.md`.

**Bash oracle (~5000 LOC across ~20 scripts):**
`task-init`, `triage` (856 LOC), `action-{scan,recur}`, `agenda`,
`review-status`, `log-append`, `qmd-index`, `rename`, `delete-page`,
`update-catalog`, `build`, `serve`, `install-{hooks,qmd}`,
`encrypt-init`, `check-deps`, `wire-{awiki,qmd}-mcp`.

**Branch:** `feat/ops-domain`. Largest by verb count; split per
verb.

## Tasks

Order by size; small verbs first.

### Task 1: log-append + qmd-index + check-deps + rename + delete

Small wrappers, mostly fs + qmd exec. One commit per verb.

### Task 2: update-catalog

Modest catalog rewriter; consumes existing helpers.

### Task 3: action verbs (scan + recur + recur-dry)

Consume `internal/action.ParseLine` (already shipped).

### Task 4: agenda

Managed-region rebuild; consume `internal/region.ManagedReplace`.

### Task 5: review chain

`agenda` → `lint` → `review-status` chain.

### Task 6: triage

Largest verb (~856 LOC bash). Interactive walker. Port the
`--interactive` interactive loop OR ship `triage-apply <id> <outcome>`
non-interactive variant first and defer interactive UX.

### Task 7: build + serve

Hugo wrappers. Mirror `bash scripts/build.sh --full` arg shape.

### Task 8: bootstrap suite

`install-hooks`, `install-qmd`, `wire-{awiki,qmd}-mcp`. Mostly
idempotent fs setup.

### Task 9: encrypt-init

Stays bash (interactive `gpg`/`git-crypt`/`age`). Skip; document.

### Task 10: task-init

Idempotent enabler analogous to `data-init`.

### Task 11: deploy-build

Stays bash forever (site-specific). Skip.

### Task 12: regression + merge

## Risks

- Triage interactivity. Either port the TUI (tedious) or ship
  non-interactive path first.
- Agenda + review chain depends on lint + region helpers — exercise
  carefully.

## Out of scope

- New verbs.
- `task-layer-migrate-review.sh` (one-off; stays bash, deletes after
  migration window).
