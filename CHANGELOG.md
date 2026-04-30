## [1.3.0] - 2026-04-30

Ports the ingest domain to the `awiki` Go binary (slices 1-9, merged
2026-04-30) and removes the bash + Python originals (slice 10).
Ports the template + ops domains in full. Eliminates every remaining
bash shim. Five scripts stay under `scripts/` by design (deploy,
encrypt-init, vendor-vega, xlsx-extract, task-layer-migrate-review).
Closes the Go-port roadmap started in v1.2.0.

### Shim cleanup (2026-04-30)

Final pass: every remaining bash shim that delegated to the `awiki`
Go binary is gone. Five scripts stay under `scripts/` (`deploy-build.sh`,
`encrypt-init.sh`, `lib/vendor-vega.sh`, `lib/xlsx-extract.py`,
`task-layer-migrate-review.sh`) — each has a real reason to remain
(site-specific deploy step, interactive gpg/git-crypt, exec'd from
the Go vendor-vega adapter, exec'd from the Go xlsx ingest adapter,
or one-off migration).

#### Removed
- `scripts/lint.sh`, `scripts/chart.sh`, `scripts/query.sh` —
  pure shims that exec'd `awiki {lint, chart, query}`. Go callers
  (`internal/lint/engine.go`, `internal/adapters/{external,lint,ingest_lint}.go`)
  now invoke the `awiki` binary directly via the new
  `adapters.ResolveAwikiBin` helper.
- `scripts/lib/action-grammar.sh`, `scripts/lib/managed-region.sh` —
  no remaining sourcers once the three shims above were gone. The
  canonical grammar lives in `internal/action/grammar.go`; the
  managed-region read/write logic in `internal/region/`.
- `scripts/lib/lock.sh` and `tests/lock_test.bats` — covered by
  `internal/fsutil/lock.go` (timeout-30s, exit-7-on-contention).
  `scripts/task-layer-migrate-review.sh`, the only remaining bash
  caller, carries a ~10-line inlined `flock -x 9 ... 9>>.awiki/lock`
  copy.
- `scripts/synth-mindmap-validate.sh` — ported to a flat
  `awiki synth-mindmap-validate <page>` verb (`internal/synth/mindmap_validate.go`).
  Exit-code discipline preserved (1 usage, 2 missing page,
  3 no mermaid block, 4 mmdc rejected, 5 bad first line, 6 brace
  imbalance) plus the `.awiki/post-hook-ran` sentinel.

#### Changed
- `synthesis-plugins/mindmap.md` `post_hook` field flipped from
  `scripts/synth-mindmap-validate.sh` to `awiki synth-mindmap-validate`.
  The plugin runner (`internal/synth/posthook.go`) now accepts
  whitespace-separated commands as post-hooks; the adapter
  (`internal/adapters/posthook.go`) splits on spaces and forwards as
  `<prog> <args...> -- <page>`, preserving the bash-era argv
  convention for any external hooks left behind.
- The pre-commit hook template emitted by `awiki task-init` writes
  `awiki lint --alias-build-only` and `awiki scan` instead of
  `bash scripts/...sh` lines. Existing hooks left in place are not
  rewritten — re-run `awiki task-init` to refresh them.
- WIKI.md doc references to `scripts/lint.sh` / `scripts/task-init.sh`
  updated to `awiki lint` / `awiki task-init`.
- The CLI's repo-root inference (`internal/cli/cli.go:hasAwikiRoot`)
  uses `content/_index.md` as the awiki-root marker now that
  `scripts/lint.sh` is gone.

### Added
- **Ingest domain**: `awiki {ingest, ingest-pdf, ingest-audio,
  ingest-xlsx, ingest-git, capture, watchdog, ingest-batch-list,
  ingest-git-list}` — every ingest verb ported. Adapters: `Pdftotext`,
  `Whisper`, `Xlsx2csv`, `GitExt` (clone/init/fetch), `FSNotify`.
  Helpers: format auto-detect, AGENT-PROMPT byte-format pinned by
  golden fixture, watchdog with fsnotify + polling fallback, 500 ms
  debounce, quarantine to `raw/inbox/batch/_failed/` on ingest error,
  per-repo state JSON read/write under `.awiki/git-state/`, markdown
  transformer with frontmatter strip + link rewrite + image copy.

### Removed
- `scripts/ingest.sh` (bash bookkeep flow) — ported to `awiki ingest`.
- `scripts/ingest-pdf.sh`, `scripts/ingest-audio.sh`,
  `scripts/ingest-xlsx.sh` — ported to `awiki ingest-<format>`.
- `scripts/ingest-git.sh`, `scripts/lib/git-clone.sh`,
  `scripts/lib/git-state.sh`, `scripts/lib/git-config.sh`,
  `scripts/ingest-git-transform.py` — ported to
  `internal/ingest/git/{run,clone,state,config,transform}.go`.
- `scripts/watchdog.sh` — ported to `awiki watchdog` (fsnotify +
  polling fallback replace fswatch/inotifywait shellouts).
- `scripts/capture.sh` — ported to `awiki capture`.
- Corresponding bats files (`capture_test`, `ingest_pdf_test`,
  `ingest_test`, `ingest_xlsx_test`, `ingest_git_lib_test`,
  `ingest_git_test`, `ingest_git_transform_test`, `watchdog_test`,
  `watchdog_xlsx_test`) dropped — Go tests under
  `internal/ingest/...` cover the same scenarios.

### Changed
- Justfile recipes for `ingest`, `ingest-with-agent`, `ingest-xlsx`,
  `watchdog`, `ingest-git`, and `capture` now call `awiki` directly
  (no `bash scripts/...sh` shim layer). Recipe names, parameter
  shapes, and pass-through flags preserved.
- Cross-domain smoke tests (`tests/task_smoke_test.bats`,
  `tests/sample_wiki_smoke_test.bats`) updated to invoke the `awiki`
  binary directly in place of the deleted `scripts/capture.sh`.

### Caller impact
External consumers of `AGENT-PROMPT|...`, `INGEST-OK|...`,
`INGEST|...`, `WATCHDOG|...`, `XLSX-CONVERTED|...`, `XLSX-NEXT|...`,
`AUDIO-TRANSCRIBED|...`, `PDF-CONVERTED|...`, `OK|appended|...`, and
other ingest-domain stdout records: the Go ports preserve byte-for-byte
parity with the bash scripts they replaced (pinned by golden fixtures
in slices 2-9 and verified during the v1.2.0 release window). No
record format changes. The `--agent <cli>` and `--agent=<cli>` shell-
out semantics (stdin = AGENT-PROMPT line, exit status = agent's) are
unchanged.

### Notes
- `scripts/lib/xlsx-extract.py` retained — still exec'd by the Go
  xlsx adapter (`internal/adapters/xlsx2csv.go`) per the v1 pin
  (xlsx2csv stays exec; no `excelize/v2`).
- `scripts/lint.sh` was retained from the v1.2.0 release window only
  as a compatibility hop; it has now been removed in the Shim cleanup
  section above. `scripts/log-append.sh` / `scripts/qmd-index.sh`
  were already deleted in earlier domain ports.
- The `awiki` binary must be on `PATH` for the justfile recipes to
  resolve (build via `go build -o bin/awiki ./cmd/awiki` and add
  `./bin/` to `PATH`, or symlink to `/usr/local/bin/awiki`).

---

## [1.2.0] - 2026-04-29

Begins the awiki Go-port roadmap. Lifts shared helpers from
`internal/lint/` into reusable packages and ports four full domains
(synth, dataset, chart, query) to the `awiki` Go binary. Bash
scripts remain in place behind a per-domain shim and continue to
work via `AWIKI_<DOMAIN>_LEGACY=1` for safety. Cleanup slices
(deleting bash + python) gated on a release window.

### Added
- **Shared building blocks**: `internal/{fsutil,emit,region,action,
  config,adapters}` — atomic write, advisory lock, record formatters,
  region parsers (synth-flavor BEGIN-GENERATED + managed-region
  `<kind>:<id>`), action-line grammar, `.awiki/config` loader, typed
  adapter interfaces. Lint internals continue to consume these via
  type aliases.
- **Synth domain**: `awiki synth {list, resolve, refine, regen,
  accept-stage, finalize, new}` — all seven verbs ported. Adapters:
  `ExecPostHook`, `ExecQmd`, `ExecSynthGit`, `ExecSynthLint`. Helpers
  for plugin manifest parsing, scope resolution + hash, region
  clear/check/scope_hash update, frontmatter set/insert, hand-edit
  detection (git diff inside markers), prompt mustache rendering,
  page scaffold writer. Verb shim flips per-verb in
  `scripts/synth.sh`.
- **Dataset domain**: `awiki dataset {validate, compact, new}` plus
  `awiki data-init` (Go path opt-in via
  `AWIKI_DATA_INIT_GO=1`). Adapters/helpers: row loader (csv/tsv/dsv
  delimiter sniffing, json, topojson) lifted from
  `internal/lint/data` into `internal/dataset`, frontmatter
  set/insert (sibling to synth's replace-only), `## Data` fence
  ops, threshold loader. Lint package re-exports row helpers via
  thin wrappers; existing rules unchanged.
- **Chart domain**: `awiki chart {new, render, render-one}`.
  Adapters: `ExecVLConvert` (shells `vl-convert vl2svg`),
  `ExecVendorVega` (shells `vendor-vega.sh`). Helpers: fence
  extraction, dataset resolver mirroring `vl-resolve.py`, canonical
  (sorted-key) JSON SHA1 hash, sidecar writer with orphan cleanup,
  managed-region preview injection.
- **Query domain**: `awiki query {run, new, render, render-one,
  fence-render}`. `ExecDuckDB` adapter shells the `duckdb` CLI.
  Four python helpers ported: extract (awiki-query fences), format
  (markdown table render), resolve (`[[<slug>]]` → DuckDB
  `read_csv_auto`/`read_json_auto` calls), determinism (canonical
  hash for cache invalidation).
- **Specs and plans for remaining domains** (ingest, template, ops)
  under `docs/superpowers/{specs,plans}/`.

### Internal
- 150 commits ahead of v1.1.0 baseline. Go module bumped to
  `go 1.25` (driven by `golang.org/x/sys v0.43.0` minimum;
  `github.com/fsnotify/fsnotify` introduction deferred to ingest
  domain).
- Bats coverage 772/774 maintained (2 pre-existing failures unrelated
  to this slice: `data_layer_full_smoke_test:41` Hugo chart-render and
  `triage_threshold` rebuild).
- All four shipped domain ports verified byte-equivalent to bash
  output via `AWIKI_<DOMAIN>_LEGACY=1` smoke comparison on live
  wiki pages.

### Deferred
- Ingest, template, ops domain implementations — specs+plans queued.
- Cleanup slices (delete bash + python, flip justfile to call
  `awiki` directly) — gated on release window per umbrella roadmap.

---

## [1.1.0] - 2026-04-28

Adds the git-docs ingest pipeline (phase 20) and a watchdog daemon for the batch inbox. Users on v1.0.x can adopt v1.1.0 cleanly via `just template-update --apply`.

### Added
- **git-docs ingest** (`just ingest-git <spec>`, `scripts/ingest-git.sh`): clone or pull a documentation source repo, transform markdown (frontmatter strip, link → wikilink rewrite, image copy + path rewrite), generate a repo entity page, persist per-repo state JSON, lock per repo, batched log/catalog/qmd housekeeping. `--protect-edits` stages conflicts to `raw/inbox/checkpoint/` rather than overwriting user edits. Supports `--repo-name` override; emits exit codes 14/15 for missing-config / lock-busy. WIKI.md §4.7 documents the workflow. `markdown-it-py` is the new optional Python dep (`scripts/check-deps.sh`).
- **Watchdog** (`just watchdog`, `scripts/watchdog.sh`): foreground daemon that auto-ingests files landing in `raw/inbox/batch/`. Backends: `fswatch` (mac), `inotifywait` (linux), polling fallback. Size-stable wait before ingest; failed ingests quarantine to `raw/inbox/batch/_failed/`. Flags: `--catchup`, `--once`, `--agent`, `--backend`, `--poll-interval`, `--stable-checks`, `--stable-interval`. Single-instance pid lock at `.awiki/watchdog.pid`.
- **README**: expanded "Pulling template updates" section (status / dry-run / apply workflow, branch-per-phase model, pending-prompts pointer, recovery flags). New common-commands rows for `template-status`, `template-gc`, `watchdog`. Canonical clone URL.

### Fixed
- `task-init` smoke output no longer references unshipped phase 17/18/19 roadmap items. The pre-commit hook installer (which actually runs at `step_precommit`) is no longer described as "deferred."

### Internal
- Test suite at 578 cases. New: `tests/watchdog_test.bats` (8), git-docs coverage (51).

---

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
