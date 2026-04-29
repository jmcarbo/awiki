# Go chart domain design

Date: 2026-04-29

## Goal

Port `scripts/chart.sh` (~269 LOC), `scripts/lib/vendor-vega.sh` (~49
LOC), and `scripts/lib/vl-resolve.py` (~124 LOC) into the Go binary
under `awiki chart {new, render, render-one}`. `vl-convert` and the
vega CDN downloads stay external.

## Non-goals

- Reimplementing `vl-convert` (Rust SVG renderer). Stays exec.
- Reimplementing CDN-fetch in `vendor-vega.sh`. Called via `os/exec`.
- Removing bash. The `scripts/chart.sh` shim flips per verb; bash
  body kept for legacy fallback until cleanup slice.

## Compatibility contract

- Justfile recipes unchanged: `chart-new`, `charts-render`,
  `charts-render-one`.
- Records byte-identical: `CHART|created <path>`,
  `CHART|rendered <id>`, `CHART|skip <id> (hash match)`,
  `CHART|removed orphan <file>`,
  `CHART|RENDER|<id>|<error-excerpt>`, `CHART|ERROR|<msg>`.
- Sidecar paths under `assets/charts/<chart-id>.{svg,json,hash,failed}`
  preserved.
- Chart-preview managed regions:
  `<!-- BEGIN chart-preview:<id> -->` / `<!-- END chart-preview:<id> -->`.
- Chart ID scheme: `<slug>` for type:chart pages with one fence,
  else `<slug>-fig<idx>`.

## Architecture

```
internal/
  cli/chart.go                  # nested dispatcher
  chart/
    types.go                    # ChartFence, Spec, ChartID
    runner.go
    extract.go                  # parse vega-lite fences from pages
    extract_test.go
    resolve.go                  # vl-resolve port (dataset substitution)
    resolve_test.go
    hash.go                     # SHA1 of resolved spec for skip-check
    sidecar.go                  # write SVG/JSON/HASH; cleanup orphans
    sidecar_test.go
    preview.go                  # ManagedRegion injection per chart
    preview_test.go
    new.go                      # chart-new verb
    new_test.go
    render.go                   # walk all pages
    render_test.go
    render_one.go
    render_one_test.go
  adapters/
    vlconvert.go                # ExecVLConvert
    vendorvega.go               # ExecVendorVega (shells the bash helper)
```

The existing `internal/lint/chart` (rules C1-C9, C-PRIV) consumes
`internal/chart` for fence extraction and dataset resolution where
possible (lift behind aliases — same pattern as dataset row helpers).

## Domain scope

### `new`

CLI: `awiki chart new <slug> --data=<dataset-slug>`. Validates slug
pattern; refuses overwrite at `<ContentDir>/charts/<slug>.md`; writes
scaffold (frontmatter + skeleton vega-lite fence). Stdout
`CHART|created <path>`.

### `render`

CLI: `awiki chart render [--keep-orphans]`.

1. Walk `<ContentDir>` for `.md` files.
2. Extract every `vega-lite` fence per page; assign chart-id.
3. For each: resolve via `internal/chart/resolve.go` (dataset
   substitution mirroring `vl-resolve.py`).
4. SHA1 of resolved spec. If `<sidecar>.hash` matches, emit
   `CHART|skip <id> (hash match)` and continue.
5. Write resolved JSON sidecar; invoke `adapters.VLConvert.RenderSVG`;
   on success write `.svg` and `.hash`; on failure write `.failed`
   (truncated 200 chars) and emit `CHART|RENDER|<id>|...`.
6. `internal/chart/preview.go` injects/refreshes managed-region
   preview block in the source page.
7. Unless `--keep-orphans`, walk `assets/charts/`; remove sidecars
   for chart-ids no longer present and purge dangling preview regions
   (managed-region remove).

### `render-one`

CLI: `awiki chart render-one <chart-id>`. Locate the fence by ID
across all pages; render only that one. Same skip/hash logic.

## External boundaries

- `adapters.VLConvert.RenderSVG(specPath, outPath)` shells
  `vl-convert vl2svg --input <specPath> --output <outPath>`.
- `adapters.VendorVega.Ensure(repoRoot)` shells
  `bash <repoRoot>/scripts/lib/vendor-vega.sh`. Called by
  `data-init` (already wired) and as a no-op precondition for
  `render`.

## Risks

- **Fence extraction edge cases.** Pages with multiple fences,
  fences inside other code blocks (rare). Mitigation: golden
  fixture per shape.
- **Dataset resolution `[[slug]]` recursion.** `vl-resolve.py`
  walks dict/list trees recursively. Go port must match. Test on
  layered/concat specs.
- **Managed-region preview path.** Bash injects with Obsidian-
  preview-friendly markdown image syntax. Use existing
  `internal/region.ManagedReplace` and reproduce body byte-equivalent.
- **Hash determinism.** SHA1 of `json.Marshal(spec)` may differ from
  bash `python -m json.tool | sha1sum` due to key ordering. The
  resolved JSON cache file is written by both; ensure marshaling
  uses sorted keys (`json.MarshalIndent` with custom encoder OR
  pre-sort recursively).

## Cleanup

Standard. Verbs ported one-by-one; shim has `AWIKI_CHART_GO_VERBS`
array.

## Open questions

- Should `vl-convert` missing be fatal in render or just skip with a
  warn? Bash is no-op-with-warn; mirror that.
- Should `render` parallelize? Bash is serial. Keep serial in port;
  optimize later.

## Out of scope

- Vega CDN URL bumps; that's a `vendor-vega.sh` concern.
- Chart lint rule changes; lint already shipped.
