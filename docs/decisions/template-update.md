# ADR: Template Update Mechanism

**Status:** Accepted
**Date:** 2026-04-27

## Context

The awiki repo is a template/scaffold cloned per domain. Users customize `WIKI.md` Identity, `hugo.toml`, `content/_index.md`, `.gitignore`, encryption setup, etc. As the template evolves (new scripts, MCP server fixes, schema changes), users need a way to pull updates without losing their customizations.

## Decision

Adopt a **detached-template + manifest** model:

- Bootstrapped repo records `.awiki/template.json` (pinned `commit`, `version`, `original_repo`).
- Updates fetch a fresh template, sync per-path strategy from `template.manifest.toml`, run versioned migrations, replay new bootstrap steps, write everything onto a dedicated `awiki-template-update/<sha>` branch.
- Default = dry-run; `--apply` mutates only on the dedicated branch with one commit per phase (Sync, Migrations, Bootstrap-steps, Provenance).
- `git merge-file --diff3` for hybrid files; user resolves conflicts via standard `<<<<<<<` markers.

## Alternatives considered

1. **Git remote + merge.** Rejected: merges noisy; user history mixes with template; encrypted repos shouldn't share history with public template.
2. **Copier / cruft.** Rejected: adds Python toolchain dependency; forces jinja restructure of all customizable files; awiki ethos is bash-first.

## Consequences

- Implementation is bash + Python helpers (no third-party dependencies beyond stdlib).
- Migration scripts and LLM prompts are trusted code — users review with `--print-migrations` and the in-flow user-confirm gate for prompts.
- Recovery flows (`--continue`, `--abort`, `--re-pin`) are first-class. `--non-interactive` enables CI.
- Rollout requires a retrofit script for repos bootstrapped before v1.0.0.

## See

- Spec: `docs/superpowers/specs/2026-04-27-template-update-design.md`
- Plans: `docs/superpowers/plans/2026-04-27-template-update-master-plan.md`
- User guide: `docs/template-update.md`
