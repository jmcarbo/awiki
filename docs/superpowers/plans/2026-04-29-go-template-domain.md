# Go template domain port Implementation Plan

> Use superpowers:subagent-driven-development.

**Goal:** port template-update domain per
`docs/superpowers/specs/2026-04-29-go-template-domain-design.md`.

**Bash + Python oracle (~3000 LOC):**
- `scripts/template-{init,update,step,retrofit,merge,plan,provenance,
  source-check,manifest,attr-audit,config}.sh`
- `scripts/_template_helpers/{availability,bootstrap_hash,bootstrap_replay,
  cache_rotate,escape,lint_template,manifest_parse,migration,plan_emit,
  preflight,provenance,state,sync}.py` (12 modules)
- `scripts/templates/{wiki-data-layer,wiki-task-layer,wiki-weekly-review}.md`

**Branch:** `feat/template-domain`. Largest domain — split into
many sub-slices.

## Tasks

### Task 1: payload embedding + manifest parsing

- `internal/template/payloads.go` with `embed.FS` for the three Markdown templates.
- `internal/template/manifest.go` — port `manifest_parse.py`. Read `template.manifest.toml`.

### Task 2: state + provenance

- `internal/template/state.go` — port `state.py`.
- `internal/template/provenance.go` — port `provenance.py`.

### Task 3: bootstrap hash + replay

- `internal/template/bootstrap_hash.go` — port `bootstrap_hash.py`.
- `internal/template/bootstrap_replay.go` — port `bootstrap_replay.py`.

### Task 4: cache + preflight + availability

- `internal/template/{cache_rotate,preflight,availability,escape}.go`.

### Task 5: sync + plan_emit + lint_template + migration

- `internal/template/{sync,plan_emit,lint_template,migration}.go`.

### Task 6: small verbs (init, status, gc, source-check, manifest, attr-audit, config, plan, provenance)

Each is a thin driver around the helpers above. Wire CLI dispatcher
`internal/cli/template.go` per-verb.

### Task 7: bootstrap-step verb

Driver around `bootstrap_replay`.

### Task 8: retrofit + merge verbs

`template-retrofit`, `template-merge` — drivers consuming the helpers.

### Task 9: update verb (largest)

Final driver tying all helpers together. Big verb; comprehensive
golden fixtures required.

### Task 10: regression + merge

## Risks

- 12 algorithmic Python modules — each needs rich golden fixtures.
- `template-update.sh` (971 LOC) is mostly a driver, but the
  contract is precise.
- `embed.FS` for Markdown payloads must preserve byte-equivalence.
- Cross-template-version migrations are sensitive.

## Strategy

This domain is too large for any single subagent to complete reliably.
Recommend allocating multiple sessions, one helper module per
session, with bash-comparison verification each time.

## Out of scope

- Adding new template features.
- Migrating wiki schema beyond bash supports.
