# Data Layer Execution — Handoff

## State as of 2026-04-28 — Plan 1 COMPLETE

- **Branch:** `feat/data-layer` (in worktree `.worktrees/data-layer/`)
- **Latest commit:** `dc92696` (lint exit-code escalation + regression test)
- **Worktree:** `/Users/joanmarc/dailywork/celonis/awiki/.worktrees/data-layer/`

## Plan 1 — done (21/21 tasks + 4 polish fixes)

All datasets-layer tasks shipped:

- [x] **P1.T1** — `data-init.sh` skeleton + dirs + `.gitkeep`
- [x] **P1.T2** — config flag + threshold defaults
- [x] **P1.T3** — WIKI.md patch (byte-idempotent + malformed-marker guard)
- [x] **P1.T4** — `just data-init` recipe
- [x] **P1.T5** — `dataset-rows.py` (count/validate/sample, bool-rejected-for-int)
- [x] **P1.T6** — `dataset-fm.sh` (single-pass `fm_set`)
- [x] **P1.T7** — `dataset.sh new` (+ python3 preflight + numeric guard)
- [x] **P1.T8** — `dataset.sh validate` (+ numeric guard parity)
- [x] **P1.T9** — `dataset.sh compact` (fail-loud on missing `## Provenance`)
- [x] **P1.T10** — `just dataset-*` recipes
- [x] **P1.T11** — `lint-data.sh` D1-D4
- [x] **P1.T12** — D5 schema sample check
- [x] **P1.T13** — D6/D7/D8/D9 + `--fix`
- [x] **P1.T14** — wire `lint-data.sh` into `lint.sh`
- [x] **P1.T15** — MCP `list_datasets`
- [x] **P1.T16** — MCP `get_dataset` (path-traversal guard hardened)
- [x] **P1.T17** — register MCP tools in `index.js`
- [x] **P1.T18** — `docs/data-help.txt` + README data-layer section
- [x] **P1.T19** — `just data-help` recipe
- [x] **P1.T20** — check-deps `python3` advisory
- [x] **P1.T21** — end-to-end smoke test

Polish (post-T21):
- README smoke test step 2: `printf` instead of `echo "...\n..."`.
- WIKI.md template D9 row: matches implementation (frontmatter `sources:` check).
- `step_dirs`: `.gitkeep` restored on pre-existing dirs.
- `lint-data.sh` `_emit`: uppercases level → `lint.sh` tally now counts D-code errors → exit code escalates correctly.

## Test counts

- **68/68 BATS** (data-layer): data_init (16), dataset_rows (10), dataset_fm (7), dataset_new (7), dataset_compact (5), dataset_validate (5), lint_data (18), data_layer_smoke (1).
- **8/8 node:test** (MCP): list-datasets (3), get-dataset (5).

## Verification

```bash
cd /Users/joanmarc/dailywork/celonis/awiki/.worktrees/data-layer
bats tests/data_init_test.bats tests/dataset_rows_test.bats tests/dataset_fm_test.bats \
     tests/dataset_new_test.bats tests/dataset_compact_test.bats tests/dataset_validate_test.bats \
     tests/lint_data_test.bats tests/data_layer_smoke_test.bats
cd mcp/awiki-server && node --test test/list-datasets.test.mjs test/get-dataset.test.mjs
```

## Known carry-forward (not blocking, address before/during Plan 2)

- **M1**: D8 path-prefix doesn't normalize `./data/...` (false-positive D8).
- **M2**: `source <(grep ...)` config sourcing in `dataset.sh` + `lint-data.sh` is a code-execution sink. Replace with `awk -F=` parsing.
- **M3**: Frontmatter parsers duplicated across `dataset-fm.sh`, `dataset.sh::_columns_to_json`, `lint-data.sh::_columns_to_json_for_lint`, `list-datasets.js`, `get-dataset.js`. Extract shared helper before Plan 2 lands a third Python copy.
- **M4**: `get-dataset.js::parseCsv` doesn't handle quoted fields; Python helper does. Acceptable per plan.
- **M5**: `cmd_validate` EXIT trap clobbers any outer trap (currently safe, latent risk).
- **Test gaps in get-dataset.js**: bad_storage, no_data_fence, data_file_missing, symlink-escape, sibling-prefix.
- **`fm_get` quote stripping** is heuristic (strips leading OR trailing quote independently).

## Resume — Plan 2 (charts)

Plan: `docs/superpowers/plans/2026-04-28-data-layer-charts.md` (21 tasks).

Pre-flight before starting:
1. Install `vl-convert`: `cargo install vl-convert` OR pre-built from <https://github.com/vega/vl-convert/releases>. Without it, chart-render tests will skip.
2. Optional: address M3 (frontmatter parser extraction) so Plan 2 doesn't add a 6th copy.
3. Skim `docs/superpowers/specs/2026-04-28-data-layer-vega-lite-design.md` §5 (charts), §6.2 (C-codes), §7 (MCP — `list_charts`), §10.2 (BATS chart tests).

## Environment

- `flock` symlinked at `/opt/homebrew/bin/flock` (was missing — see `brew --prefix util-linux`).
- `python3`, `hugo`, `bats`, `just`, `node` all available.
- `vl-convert` NOT installed — required for Plan 2 chart-render tests.
