# Data Layer Execution — Handoff

## State as of 2026-04-28 — Plan 1 + Plan 2 COMPLETE

- **Branch:** `feat/data-layer` (in worktree `.worktrees/data-layer/`)
- **Latest commit:** `10d106c` (just chart-new recipe)
- **Worktree:** `/Users/joanmarc/dailywork/celonis/awiki/.worktrees/data-layer/`

## Plan 1 — done (datasets, 21/21 + 4 polish fixes)

See git log de6f522..d2d94d1 for the dataset commits.

## Plan 2 — done (charts, 21/21)

- [x] **P2.T1** — data-init chart dirs + git-crypt + obsidian flag (`57346ad`)
- [x] **P2.T2** — vendor-vega.sh + step_vendor_vega (`3acfcf5`)
- [x] **P2.T3** — vl-resolve.py wikilink expander (`9697f47`)
- [x] **P2.T4** — chart.sh new (`d9660ff`)
- [x] **P2.T5** — chart.sh render sidecar pipeline (`fc15440`)
- [x] **P2.T6** — chart.sh render-one (`9ccb9e2`)
- [x] **P2.T7** — Obsidian managed-region preview (`568e4df`)
- [x] **P2.T8** — lint-chart C1/C2/C3 (`cbcd29e`)
- [x] **P2.T9** — lint-chart C4/C5/C6 + --fix (`e27cd28`)
- [x] **P2.T10** — lint-chart C7/C8/C9 (`f11a246`)
- [x] **P2.T11** — lint-chart C-PRIV (`6054d01`)
- [x] **P2.T12** — wire lint-chart into lint.sh + escalate exit (`8a93320`)
- [x] **P2.T13** — Hugo render-hook for vega-lite fences (`93b2dd9`)
- [x] **P2.T14** — `{{< vega-lite >}}` shortcode (`043295d`)
- [x] **P2.T15** — just charts-render(*one) + build.sh integration (`daad7c0`)
- [x] **P2.T16** — MCP list_charts (`17ad991`)
- [x] **P2.T17** — check-deps vl-convert advisory (`2549239`)
- [x] **P2.T18** — chart docs in data-help.txt + README (`4da47ae`)
- [x] **P2.T19** — CI install vl-convert (`0a31aca`)
- [x] **P2.T20** — full data-layer smoke test (`8c685ab`)
- [x] **P2.T21** — just chart-new recipe (`10d106c`)

## Test counts (final)

- **117/117 BATS** (data-layer suite, including 4 hugo+vl-convert smoke tests that skip in this environment).
- **12/12 node:test** (MCP: list-datasets 3, get-dataset 5, list-charts 4).

Tests that skip when binary missing: chart_render (5), chart_obsidian_preview (3), hugo_render_chart (3), data_layer_full_smoke (1) — total 12 skips. Re-run with `vl-convert` + `hugo` installed to fully exercise.

## Verification

```bash
cd /Users/joanmarc/dailywork/celonis/awiki/.worktrees/data-layer
bats tests/data_init_test.bats tests/dataset_rows_test.bats tests/dataset_fm_test.bats \
     tests/dataset_new_test.bats tests/dataset_compact_test.bats tests/dataset_validate_test.bats \
     tests/lint_data_test.bats tests/data_layer_smoke_test.bats \
     tests/vl_resolve_test.bats tests/chart_new_test.bats tests/chart_render_test.bats \
     tests/chart_obsidian_preview_test.bats tests/lint_chart_test.bats \
     tests/hugo_render_chart_test.bats tests/data_layer_full_smoke_test.bats
cd mcp/awiki-server && node --test test/list-datasets.test.mjs test/get-dataset.test.mjs test/list-charts.test.mjs
```

## Plan 1 carry-forward (still applicable)

- **M1**: D8 path-prefix doesn't normalize `./data/...` (false-positive D8).
- **M2**: `source <(grep ...)` config sourcing in `dataset.sh` + `lint-data.sh` is a code-execution sink. Replace with `awk -F=` parsing.
- **M3**: Frontmatter parsers duplicated across dataset-fm.sh, dataset.sh::_columns_to_json, lint-data.sh::_columns_to_json_for_lint, lint-chart.sh::_columns_to_json_for_lint, list-datasets.js, get-dataset.js, list-charts.js, vl-resolve.py. Extract shared helpers (Python + JS).
- **M4**: `get-dataset.js::parseCsv` doesn't handle quoted fields; Python helper does.
- **Test gaps in get-dataset.js**: bad_storage, no_data_fence, data_file_missing, symlink-escape, sibling-prefix.

## Plan 2 known caveats

- `lint-chart.sh` uses `2>/dev/null` to capture `RESOLVE|...` from vl-resolve stdout (plan source had the redirect order wrong; fixed in implementation).
- `vendor-vega.sh` curl path needs network; CI uses `AWIKI_VEGA_VENDOR_LOCAL_DIR` for hermetic test runs.
- Hugo render-hook + shortcode require `hugo` ≥ 0.107 for `.Page.Scratch` and `findRESubmatch`.
- `_obsidian_preview_enabled` bash helper is dead code (plan-verbatim) — Python heredoc reads config directly. Acceptable.
- `parseScalar` in `list-charts.js` is unused (plan-verbatim).
- C7 fires per-fence when no sidecar exists. In CI without vl-convert the smoke test skips, so this is observable only after `chart.sh render` was expected to run but didn't.

## Environment

- `flock` symlinked at `/opt/homebrew/bin/flock`.
- `python3`, `hugo`, `bats`, `just`, `node`, `curl` all available.
- `vl-convert` NOT installed locally — chart-render tests skip gracefully.
  - Install: `cargo install vl-convert` OR pre-built from <https://github.com/vega/vl-convert/releases>.
  - CI workflow already includes the install step (T19).

## Both plans together

`feat/data-layer` ships as one feature: "datasets you can chart". Branch is ready for PR / merge once the user kicks off review.
