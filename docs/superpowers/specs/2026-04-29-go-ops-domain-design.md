# Go ops domain design

Date: 2026-04-29

## Goal

Port the long tail of awiki one-shot operational commands to Go.
~20 verbs covering task-layer, build/serve/deploy, bootstrap/install.

## Non-goals

- Reimplementing `hugo`, `qmd`, `gpg`, `git-crypt`, `age`, `bats`.
  Stay exec.
- Porting `scripts/deploy-build.sh` to Go (site-specific; stays bash
  forever).
- Porting `scripts/task-layer-migrate-review.sh` (one-off migration;
  stays bash, deletes after migration window).

## Compatibility contract

- Justfile recipes preserved: `agenda`, `triage`, `triage-apply`,
  `capture`, `recur`, `recur-dry`, `review`, `scan`, `task-init`,
  `rename`, `delete`, `log`, `reindex`, `build`, `serve`,
  `check-deps`, `install-hooks`, `install-qmd`, `encrypt-init`.
- Records byte-identical: `REVIEW|...`, `RECUR|...`, etc.

## Architecture (sketch)

```
internal/
  cli/ops.go             # flat dispatch for one-shot verbs
  ops/
    agenda.go
    triage.go            # 856-LOC bash; biggest verb in this domain
    action.go            # scan + recur (action-grammar consumer)
    log.go
    index.go             # qmd-index wrapper
    rename.go
    delete.go
    catalog.go           # update-catalog
    build.go
    serve.go
    bootstrap.go         # install-hooks, install-qmd, encrypt-init,
                         # check-deps, wire-mcp
    review.go            # review chain (agenda + lint + review-status)
```

## Strategy

**Per-verb sub-slices.** Most verbs are small; a few are large
(`triage` ~856 LOC, `agenda` orchestration). Order by size:

1. **Adapters + helpers** (rename, delete, log, index, catalog,
   check-deps) — small wrappers over qmd / fs.
2. **Action verbs** (scan, recur, recur-dry) — consume
   `internal/action.ParseLine` (already shipped).
3. **Agenda + review** — managed-region rebuild; chain orchestration.
4. **Triage** — interactive walker; large; behavioral tests.
5. **Build + serve** — exec `hugo`; native flag handling.
6. **Bootstrap suite** — install-hooks/install-qmd/encrypt-init/
   wire-mcp. Mostly idempotent file-system setup; encrypt-init keeps
   bash for interactivity.
7. **Task-init** — flat verb (counterpart to dataset's data-init).

## Risks

- **`triage` interactivity.** Bash uses `read` for prompts. Go needs
  an analogous TUI loop or a non-interactive `--apply` mode that
  matches `triage-apply` recipe semantics.
- **`agenda` managed regions.** Multiple regions per page; consume
  `internal/region.ManagedReplace`.
- **`build` Hugo flags.** Mirror `bash scripts/build.sh --full`
  argument shape.

## Out of scope

- New verbs not in justfile today.
- MCP ops handlers.
- Restructuring bash/awk inside scripts that won't port.
