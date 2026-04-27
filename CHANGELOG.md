## [1.0.0] - 2026-04-27

First release with the template-update mechanism. Repos bootstrapped from this version (or later) get the clean update path. Pre-v1 repos use `just template-retrofit` to seed `.awiki/template.json` once.

### Added
- Template update mechanism: detached-template + manifest model with per-path strategies (`overwrite`/`preserve`/`three_way`/`attributes_merge`/`template_only`), versioned migrations (mechanical bash + LLM prompt), three-way merge for hybrid files, dedicated update branch with phase-per-commit (Sync, Migrations, Bootstrap-steps, Provenance), state file for `--continue`/`--abort` recovery, multi-machine ancestor cache auto-recovery, `--re-pin` rollback, `--rerun-bootstrap-step` for dangerous steps, `--gc` cache cleanup, `--non-interactive` for CI.
- Trust model: pinned `repo`/`original_repo`, `--accept-source-change` gate, stripped env for migrations, `touches:` post-run enforcement, `scope_glob` enforcement for LLM prompts, optional `--verify-signature` GPG check.
- Lint additions for manifest dupes, migration headers, aged pending prompts, repo/original_repo drift, and update-availability detection via `_check-stamp`.
- BOOTSTRAP step IDs (HTML comment markers) + new `template-init` step seeding `.awiki/template.json` and `.awiki/template-cache/<commit>/`.
- Retrofit script (`scripts/template-retrofit.sh`) for pre-v1 repos with heuristic suggestions for already-completed steps.
- `scripts/template-step.sh` alias for `--rerun-bootstrap-step`.
- BATS test suite (155 tests) covering happy / security / recovery / edge cases including retrofit.
- CI E2E workflow (`tests/template-update-e2e.sh` + `.github/workflows/template-update-e2e.yml`).
- Docs: `docs/template-update.md`, `docs/decisions/template-update.md` (ADR), `migrations/README.md` (author guide), README mention, WIKI.md §14 agent-facing pending-prompts workflow.

### Migration

If your wiki was bootstrapped before this release, run `just template-retrofit` once to seed `.awiki/template.json` with content_hashes for completed bootstrap steps. From then on, regular `just template-update` works.

### Phase log (development history)
- `template-update` foundation: manifest parser with glob-precedence resolver, `.awiki/config` parser, `template.json` provenance helper, bootstrap step content_hash util, `template-init.sh` (Phase 01).
- `template-update` plan + merge layer: `template-plan.sh` (formal `PLAN|...` output), `template-merge.sh` (3-way wrapper), `template-attr-audit.sh` (`.gitattributes` change detector), `template-source-check.sh` (source-change halt) (Phase 02).
- `template-update.sh` orchestrator skeleton with Phase 0a (preflight: dirty tree, branch, encryption, pending prompts, source-change), Phase 1 (fetch + ancestor auto-recovery + state file init), Phase 0b (post-fetch schema check + encryption recheck + optional `--verify-signature`) (Phase 03).
- `template-update.sh` Phase 1.5 (schema-upgrade with update branch + Commit 0) and Phase 2 (plan emit, `--print-migrations` body framing, scratch-merge cleanup) (Phase 04).
- `template-update.sh` Phase 3 Commit A: sync per-strategy (overwrite, three_way, attributes_merge with `--accept-attribute-changes` gate, new_file prompts, deletions with locally-modified prompt), conflict-marker halt, branch creation if needed (Phase 05).
- Migration runner with stripped env + `touches:` enforcement + `.awiki/` ban + `--skip-migration <id>`.
- LLM prompt staging into `.awiki/pending-prompts/` with `scope_glob` enforcement (blocks secrets/, .awiki/, .git/, themes/) and `risk` metadata; `risk: high` under `--non-interactive` auto-declined and recorded as skipped (Phase 06).
- `template-update.sh` Phase 3 Commit C (bootstrap step replay with `content_hash` tracking + dangerous-step skip + env-stripped body execution) and Commit D (single `template.json` write — consolidates pending entries from state file into `applied_migrations[]`, `bootstrap_steps_done[]`, `deleted[]`; cache rotation keeping current + previous SHA dirs only; state-file deletion; final summary print). `--persist-source` updates `repo` only; `original_repo` is immutable (Phase 07).
- Recovery flows: `--abort` (deletes update branch + `_fetch/`, restores default branch); `--continue` (resumes from state file with HEAD validation, `--accept-manual-commits` bypass, `git reset --hard` on `started`-status phases); `--re-pin <commit>` (validates commit upstream, drops orphan cache, rebuilds target ancestor cache, refuses on pending-prompts or in-progress state); `--rerun-bootstrap-step <id>` (dedicated `awiki-template-update/rerun-<id>-<ts>` branch, env-stripped body replay, literal-heredoc Python `content_hash` update, refuses if state file/pending prompts/existing update branch); `--gc` (rotates cache to current + previous); `--status` (pin/version/repo/original_repo drift/pending prompts/in-progress phase) (Phase 08).
- Production `template.manifest.toml` at repo root with full strategy + bootstrap config (template_version=1.0.0, dangerous=[theme, wire-qmd-mcp, wire-awiki-mcp, install-qmd]).
- `BOOTSTRAP.md` step markers (`<!-- bootstrap-step: <id> -->`) and new `template-init` step (Step 12) calling `template-init.sh`; bidirectional parity with manifest `bootstrap.ordered_steps`.
- Lint additions wired into `scripts/lint.sh` via `scripts/_template_helpers/lint_template.py`: manifest dupe globs, migration filename pattern + required headers + blocked `touches:`, LLM prompt frontmatter (id/requires/scope_glob/risk) + risk enum + blocked `scope_glob`, aged pending prompts (>14d), repo/original_repo drift, pinned-commit-not-in-cache, orphan `_fetch/.update-state.json` and `_scratch-merge/`, pending-prompt mtime drift vs `applied_migrations[]`, prompt body resolved-scope leak.
- Update-availability check `scripts/_template_helpers/availability.py` (`_check-stamp` 7-day cadence + `AWIKI_NO_TEMPLATE_CHECK` opt-out + `.awiki/config.no_template_check=true`); silent on network failures; emits `LINT|info|template|...` when upstream is ahead of pin.
- Justfile recipes: `template-update`, `template-status`, `template-gc`, `template-retrofit`, `bootstrap-step` (Phase 09).
