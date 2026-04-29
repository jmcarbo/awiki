# Go chart domain port Implementation Plan

> Use superpowers:subagent-driven-development.

**Goal:** port `awiki chart {new, render, render-one}` per
`docs/superpowers/specs/2026-04-29-go-chart-domain-design.md`.

**Bash oracle:** `scripts/chart.sh`, `scripts/lib/vendor-vega.sh`,
`scripts/lib/vl-resolve.py`. Existing Go in `internal/lint/chart/`
already implements rule logic; this slice adds CRUD + render verbs.

**Branch:** `feat/chart-domain` in `.worktrees/chart-domain`.

## Tasks

### Task 1: VLConvert + VendorVega adapters + skeleton

- Create: `internal/adapters/vlconvert.go` + test (`ExecVLConvert.RenderSVG(spec, out)` shells `vl-convert vl2svg --input <spec> --output <out>`).
- Create: `internal/adapters/vendorvega.go` + test (`ExecVendorVega.Ensure(repoRoot)` shells `bash scripts/lib/vendor-vega.sh`; no-op when files present).
- Create: `internal/chart/{types,runner}.go`. Runner has `RepoRoot`, `ContentDir`, `AssetsDir` (=<RepoRoot>/assets/charts), `VLConvert adapters.VLConvert`, `VendorVega adapters.VendorVega`.
- Tests: compile-time conformance for both adapters.
- Commit: `feat: add chart adapters and runner`.

### Task 2: fence extract + dataset resolve

- Create: `internal/chart/extract.go` — `ExtractFences(text string, slug string) []ChartFence`. Walks markdown; finds ` ```vega-lite ``` ` fences; assigns `ChartID` per scheme (single-fence type:chart → `<slug>`; else `<slug>-fig<idx>`).
- Create: `internal/chart/resolve.go` — `ResolveSpec(repoRoot string, baseURL string, src string, spec map[string]any) (map[string]any, error)`. Mirrors `vl-resolve.py`: recursively walks `data` keys; `data.name = "[[<slug>]]"` → look up `<ContentDir>/datasets/<slug>.md`; if `storage=file` → `{url, format}`; if `storage=inline` → `{values}` parsed from inline fence per format.
- Tests: extract from page with 1 fence, page with 3 fences, `type:chart` ID scheme; resolve with file storage, inline csv, inline json, missing dataset error.
- Commit: `feat: add chart fence extraction and dataset resolver`.

### Task 3: hash + sidecar

- Create: `internal/chart/hash.go` — `Hash(spec map[string]any) string` returns SHA1 hex of canonical JSON (sorted keys recursively).
- Create: `internal/chart/sidecar.go` — `WriteSidecars(dir, id string, resolvedSpec map[string]any, svg []byte) error`; `ReadHash(dir, id string) string`; `WriteFailed(dir, id, errMsg string)`; `RemoveOrphans(dir string, knownIDs map[string]bool) []string` (returns removed paths).
- Tests: golden hash for fixture spec; orphan removal lists matching `*.svg`/`*.svg.hash`/`*.svg.failed`.
- Commit: `feat: add chart hash and sidecar helpers`.

### Task 4: managed-region preview

- Create: `internal/chart/preview.go` — `InjectPreview(pageText string, chartID string, repoRoot string) string` uses `internal/region.ManagedReplace` with `kind="chart-preview"`, `id=<chartID>`, body matching bash:
  ```
  ![<chartID>](../../assets/charts/<chartID>.svg)
  ```
- `RemovePreview(pageText string, chartID string) string` strips the region (use `region.ManagedReplace` with empty body and follow-up cleanup OR a small regex-based remove).
- Tests: insert when absent, refresh when present, remove when chart deleted.
- Commit: `feat: add chart preview region helper`.

### Task 5: new verb

- Create: `internal/chart/new.go` (Runner.New) + test.
- Bash oracle (`cmd_new` in `chart.sh`): scaffold page with `chart_engine: vega-lite`, `chart_data: "[[<dataset-slug>]]"`, single skeleton vega-lite fence. Refuse overwrite. Stdout `CHART|created <path>`.
- Wire CLI dispatcher (`internal/cli/chart.go`); shim flip in `scripts/chart.sh` with `AWIKI_CHART_GO_VERBS=(new)`.
- Commit: `feat: port chart new verb`.

### Task 6: render verb

- Create: `internal/chart/render.go` (Runner.Render) + test.
- Walks `<ContentDir>` for `.md` files (skip `_failed`, hidden). Per page: extract fences → for each fence, resolve → hash → if `<asset>.hash` matches: emit `CHART|skip <id> (hash match)` and continue; else write resolved JSON sidecar, exec `vl-convert`, write SVG + hash on success (emit `CHART|rendered <id>`) or write `.failed` and emit `CHART|RENDER|<id>|<200-char-trunc>`. Inject `chart-preview` managed region per success.
- After main loop: unless `--keep-orphans`, scan `assets/charts/` for sidecars whose chart-id is no longer present; delete and emit `CHART|removed orphan <file>` per removal. Also strip `chart-preview` managed regions for orphan IDs.
- Commit: `feat: port chart render verb`.

### Task 7: render-one verb

- Create: `internal/chart/render_one.go` (Runner.RenderOne) + test.
- Locate fence by chart-id across all pages; render that one. Same skip/hash semantics.
- Commit: `feat: port chart render-one verb`.

### Task 8: final regression + merge

- `go test -race ./...` clean.
- `just test` 772/774 (preserved).
- Smoke each verb against live wiki, diff against bash. Hash output bash uses `sha1sum` — match Go SHA1 hex digest.
- Merge `feat/chart-domain` → main with `--no-ff`.

## Self-review

- All 3 verbs map to tasks; helpers separated for reuse.
- `vl-convert` and `vendor-vega.sh` stay external.
- Hash determinism: canonical JSON via sorted-keys marshaler.
- Managed-region body byte-equivalent to bash injection.
