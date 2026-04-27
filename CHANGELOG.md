## [Unreleased]

### Added
- `template-update` foundation: manifest parser with glob-precedence resolver, `.awiki/config` parser, `template.json` provenance helper, bootstrap step content_hash util, `template-init.sh` (Phase 01).
- `template-update` plan + merge layer: `template-plan.sh` (formal `PLAN|...` output), `template-merge.sh` (3-way wrapper), `template-attr-audit.sh` (`.gitattributes` change detector), `template-source-check.sh` (source-change halt) (Phase 02).
- `template-update.sh` orchestrator skeleton with Phase 0a (preflight: dirty tree, branch, encryption, pending prompts, source-change), Phase 1 (fetch + ancestor auto-recovery + state file init), Phase 0b (post-fetch schema check + encryption recheck + optional `--verify-signature`) (Phase 03).
- `template-update.sh` Phase 1.5 (schema-upgrade with update branch + Commit 0) and Phase 2 (plan emit, `--print-migrations` body framing, scratch-merge cleanup) (Phase 04).
- `template-update.sh` Phase 3 Commit A: sync per-strategy (overwrite, three_way, attributes_merge with `--accept-attribute-changes` gate, new_file prompts, deletions with locally-modified prompt), conflict-marker halt, branch creation if needed (Phase 05).
- Migration runner with stripped env + `touches:` enforcement + `.awiki/` ban + `--skip-migration <id>`.
- LLM prompt staging into `.awiki/pending-prompts/` with `scope_glob` enforcement (blocks secrets/, .awiki/, .git/, themes/) and `risk` metadata; `risk: high` under `--non-interactive` auto-declined and recorded as skipped (Phase 06).
