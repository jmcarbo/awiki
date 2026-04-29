# Go template domain design

Date: 2026-04-29

## Goal

Port the awiki template-update domain to the Go binary under
`awiki template <verb>`. Largest single domain: 11 bash scripts +
12 Python helpers under `scripts/_template_helpers/` + 3 Markdown
payloads under `scripts/templates/`.

## Non-goals

- Reimplementing `git`, `gpg`, `git-crypt`. Stay exec / via existing
  adapters.
- Reworking the bootstrap step protocol.

## Compatibility contract

- Justfile: `template-init`, `template-update`, `template-status`,
  `template-gc`, `bootstrap-step`, `template-retrofit`. All names
  preserved.
- `template.manifest.toml` shape unchanged.
- `provenance.json` shape unchanged.

## Architecture (sketch)

```
internal/
  cli/template.go
  template/
    manifest.go           # parse template.manifest.toml
    plan.go               # plan emit (port plan_emit.py)
    state.go              # state machine (port state.py)
    sync.go               # sync (port sync.py)
    bootstrap_hash.go     # port bootstrap_hash.py
    bootstrap_replay.go   # port bootstrap_replay.py
    cache_rotate.go
    escape.go
    lint_template.go
    manifest_parse.go     # YAML/TOML manifest parser
    migration.go          # schema migrations
    preflight.go
    provenance.go
    init.go               # template-init verb
    update.go             # template-update verb (largest)
    status.go             # template-status verb
    gc.go                 # template-gc verb
    bootstrap.go          # bootstrap-step verb
    retrofit.go
    merge.go
  adapters/
    template_payloads.go  # embed.FS for scripts/templates/*.md
```

## Domain scope (verbs)

- `template-init`: clone template repo, write manifest, capture commit.
- `template-update`: refresh template, run plan, apply migrations.
  Driver around `_template_helpers/*.py` modules (12 of them).
- `template-status`: report freshness vs. upstream.
- `template-gc`: prune cached snapshots.
- `bootstrap-step <id>`: execute one bootstrap step.
- `template-retrofit`: backfill provenance for existing wikis.
- `template-merge`: merge a specific template path.

## Risks

- **Python algorithmic logic.** `_template_helpers/`'s 12 modules
  carry the heaviest non-trivial logic in the repo. Port with rich
  golden fixtures per module (one Go file per Python file).
- **Markdown payloads.** `scripts/templates/{wiki-data-layer,
  wiki-task-layer,wiki-weekly-review}.md` — embed via `embed.FS`.
- **Template repo cloning.** Use `go-git` per the roadmap.

## Suggested execution order

Largest in the synth-cleanup-window:

1. Adapters + payload embedding (Task 1)
2. `manifest`, `state`, `provenance`, `escape` helpers (Task 2)
3. `plan_emit`, `bootstrap_hash`, `bootstrap_replay` (Task 3)
4. `migration`, `cache_rotate`, `preflight`, `availability` (Task 4)
5. `sync`, `lint_template` (Task 5)
6. Verbs: `init`, `status`, `gc` (smallest first) — Task 6
7. `bootstrap-step`, `retrofit`, `merge` — Task 7
8. `update` (largest, depends on everything) — Task 8

## Out of scope

- Cross-template-version migration paths beyond what bash supports.
- Template authoring UX improvements.
