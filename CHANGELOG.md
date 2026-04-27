## [Unreleased]

### Added
- `template-update` foundation: manifest parser with glob-precedence resolver, `.awiki/config` parser, `template.json` provenance helper, bootstrap step content_hash util, `template-init.sh` (Phase 01).
- `template-update` plan + merge layer: `template-plan.sh` (formal `PLAN|...` output), `template-merge.sh` (3-way wrapper), `template-attr-audit.sh` (`.gitattributes` change detector), `template-source-check.sh` (source-change halt) (Phase 02).
- `template-update.sh` orchestrator skeleton with Phase 0a (preflight: dirty tree, branch, encryption, pending prompts, source-change), Phase 1 (fetch + ancestor auto-recovery + state file init), Phase 0b (post-fetch schema check + encryption recheck + optional `--verify-signature`) (Phase 03).
