# Data Layer + Vega-Lite Charts — Design

- **Date:** 2026-04-28
- **Status:** Draft, awaiting user review
- **Scope:** SP1 (datasets-as-pages) + SP2 (Vega-Lite renderer), shipping together as one user-visible feature ("datasets you can chart")
- **Deferred:** SP3 (typed frontmatter on non-dataset pages), SP4 (external data refresh)

## 1. Goal

Add an opt-in **data layer** to awiki that lets a wiki host structured datasets as first-class pages and render Vega-Lite charts that read from them. Charts must work in both Hugo (static-site output) and Obsidian (live editor preview), driven from the same markdown source.

The feature is opt-in via `just data-init`, mirroring the existing task layer pattern. A wiki without datasets remains unchanged.

## 2. Decisions locked during brainstorm

| # | Decision |
|---|----------|
| Q1 | Storage: dataset rows live inline in the page body OR in a sibling file under `data/`. Threshold-driven; lint warns when inline exceeds limits; flip via explicit `just dataset-compact` (no silent rewrites). |
| Q2 | Formats: CSV, TSV, JSON, DSV, topojson — passed through to Vega-Lite, no awiki-side parsing. |
| Q3 | Schema (`columns:`) is optional; when present, lint enforces strict types. |
| Q4 | Chart data binding: wikilink (`data.name: "[[slug]]"`), raw path (`data.url: "..."`), and inline values all allowed. |
| Q5 | Chart specs live as inline ```vega-lite``` fences AND as dedicated `type: chart` pages. |
| Q6 | Obsidian: dual path — community plugin if installed, else SVG sidecar fallback rendered next to the fence. |
| Q7 | Hugo: vega-embed JS vendored under `static/vendor/vega/`, lazy-loaded only on pages containing a chart. |
| Q8 | Sidecar tooling: `vl-convert` Rust binary; regen via `just charts-render` (manual) and automatically in `just build`. |
| Q9 | Threshold flip mechanism: hard threshold + lint warn (D6); manual `just dataset-compact <slug>` to apply. |
| Q10 | MCP surface: read-only — `list_datasets`, `get_dataset`, `list_charts`. No write/query MCP tools in this spec. |

## 3. Architecture

Two new opt-in subsystems on existing awiki, enabled together by `just data-init`. The init script:

- Creates `data/`, `content/datasets/`, `content/charts/`, `assets/charts/`, `static/vendor/vega/`.
- Patches `WIKI.md` between `<!-- BEGIN data-layer -->` / `<!-- END data-layer -->` markers (new "Data Layer" section; numbering chosen at apply time to avoid collision with existing §10/§14 and the task-layer block).
- Sets `AWIKI_DATA_LAYER=on` in `.awiki/config`.
- Prompts to extend git-crypt patterns to `data/**`, `data/private/**`, `assets/charts/private/**` if encryption is configured.
- Vendors `vega.min.js`, `vega-lite.min.js`, `vega-embed.min.js` into `static/vendor/vega/` (one-time download from a pinned version listed in `template.manifest.toml`).
- Per-step idempotent (mirrors `task-init.sh`).

### 3.1 Layout

```
content/datasets/<slug>.md          # dataset metadata pages
content/charts/<slug>.md            # named chart pages (optional, embeddable)
data/<slug>.<ext>                   # row data when storage=file
data/private/<slug>.<ext>           # private rows (git-crypt)
static/vendor/vega/                 # vendored vega bundle
assets/charts/<chart-id>.svg        # generated sidecar previews
assets/charts/private/<chart-id>.svg
scripts/dataset.sh                  # create / compact / validate
scripts/chart.sh                    # render / regen sidecars
scripts/lint-data.sh                # D-codes
scripts/lint-chart.sh               # C-codes
scripts/lib/vl-resolve.py           # spec wikilink → URL rewriter
layouts/shortcodes/vega-lite.html   # for `type: chart` page embeds
layouts/_default/_markup/render-codeblock-vega-lite.html  # fence interceptor
mcp/awiki-server/tools/data.py      # list_datasets / get_dataset / list_charts
```

### 3.2 Render pipeline

1. **Hugo build:** the codeblock render-hook intercepts ```vega-lite``` fences. It runs `vl-resolve.py` over the spec to expand `[[slug]]` references, emits `<div class="vega-embed" data-spec="...">` plus a lazy `<script>` tag (only on pages whose front-matter or scanned content contains a chart). The `{{< vega-lite >}}` shortcode does the same for embedded `type: chart` pages.
2. **Pre-render (sidecar generation):** `just charts-render` walks all chart sources, computes `sha1(spec_after_resolve)`, looks up `assets/charts/<chart-id>.svg`, calls `vl-convert vl2svg` if missing or stale. `just build` invokes `charts-render` before `hugo`.
3. **Obsidian:** the same fenced ```vega-lite``` block. With a community plugin installed (e.g. `obsidian-vega-lite`), Obsidian renders live. Without a plugin, `charts-render` injects a managed-region preview block immediately below the fence:

   ```
   <!-- BEGIN chart-preview:<chart-id> -->
   ![<chart-id>](../assets/charts/<chart-id>.svg)
   <!-- END chart-preview:<chart-id> -->
   ```

   The image embed renders in Obsidian's reading view (and in any plain markdown viewer). Hugo's render-hook hides the managed region from the published HTML to avoid double-rendering. Plugin users who want to suppress the redundant preview set `AWIKI_CHART_OBSIDIAN_PREVIEW=off` in `.awiki/config`; `charts-render` then leaves the markers empty. Lint flags hand-edits inside `<!-- BEGIN chart-preview:* -->` markers (regen-only, mirrors agenda-managed-regions in the task layer).

### 3.3 Wikilink resolver

Single Python module (`scripts/lib/vl-resolve.py`), reused by Hugo render-hook and by `chart-render`:

- Input: parsed Vega-Lite spec (JSON dict).
- Behavior:
  - `data.name: "[[slug]]"` → look up `content/datasets/<slug>.md`. If `storage: file`, replace with `{"url": "<baseURL>/data/<slug>.<ext>", "format": {"type": <format>}}`. If `storage: inline`, parse the fenced block from `## Data` and replace with `{"values": [...]}` (parsed via Python's csv/json modules).
  - `data.url: "..."` → pass through unchanged.
  - `data.values: [...]` → pass through unchanged.
  - Multiple data references (`layer`, `hconcat`, `vconcat`, `repeat`) — recurse over the spec tree.
- Failure modes:
  - Slug missing or page is not `type: dataset` → exit non-zero with `RESOLVE|<page>|<slug>|<reason>`. Render-hook turns this into a Hugo build failure; lint surfaces it as C3.
- Output: rewritten spec as JSON to stdout (or to a temp file for vl-convert).

## 4. Dataset subsystem

### 4.1 Page schema

Frontmatter for `type: dataset`:

```yaml
---
title: "..."
date: 2026-04-28
last_updated: 2026-04-28
type: dataset
tags: [...]
storage: inline | file        # required, set by dataset.sh
format: csv | tsv | json | dsv | topojson
columns:                      # optional schema (Q3=b)
  - { name: year, type: integer }
  - { name: pop, type: number }
  - { name: country, type: string, enum: [...] }   # optional
rows: 247                     # cached count, recomputed on compact/validate
data_path: "data/<slug>.csv"  # only when storage=file
sources: ["[[s-foo]]"]
draft: false
---
```

Body:

- Lead paragraph (≤100 words).
- `## Schema` — human-readable column descriptions, mirrors `columns:`.
- `## Data` — fenced block in the declared format when `storage: inline`; absent when `storage: file`.
- `## Provenance` — origin (URL, ingestion date, manual entry).
- `## Related` — wikilinks.
- `## Sources` — mirrors frontmatter.

### 4.2 Lifecycle

| Recipe | Behavior |
|--------|----------|
| `just dataset-new <slug> [--format=csv] [--from=<path>]` | Scaffolds page; optionally seeds rows from a local file. Refuses if slug exists. |
| `just dataset-compact <slug>` | Flips `inline` ↔ `file` based on threshold. Idempotent: no-op when already compact. Refuses to inline-compact a file dataset that exceeds threshold. |
| `just dataset-validate <slug>` | Re-counts rows, type-checks against `columns:` if declared, updates `rows:` field. |

### 4.3 Threshold

Configurable in `.awiki/config`:

- `AWIKI_DATASET_INLINE_MAX_ROWS=500`
- `AWIKI_DATASET_INLINE_MAX_BYTES=51200`

Lint emits D6 warn when exceeded; never auto-rewrites.

### 4.4 Privacy

Standard awiki rule. `tags: [private]` requires `content/datasets/private/<slug>.md` AND `data/private/<slug>.<ext>` (when `storage: file`). git-crypt patterns extended on `data-init` (with consent) or on the next `encrypt-init`.

## 5. Chart subsystem

### 5.1 Inline fence (Surface 1)

````markdown
```vega-lite
{
  "mark": "bar",
  "data": {"name": "[[us-pop-by-state]]"},
  "encoding": {
    "x": {"field": "state", "type": "nominal"},
    "y": {"field": "pop", "type": "quantitative"}
  }
}
```
````

Allowed in any markdown page. Resolver handles the `[[slug]]` reference at build time. Sidecar id: `<page-slug>-fig<N>` where `N` is the zero-based fence index ordered by source byte offset of the opening ```` ```vega-lite ```` fence in the markdown file (deterministic across renderers).

### 5.2 Chart page (Surface 2)

Frontmatter for `type: chart`:

```yaml
---
type: chart
chart_engine: vega-lite           # discriminator; future: matplotlib, mermaid
chart_data: "[[us-pop-by-state]]" # optional cached pointer for catalog/lint (lint C6 keeps it in sync)
sources: [...]
---
```

Body holds one ```vega-lite``` fence. Embedded elsewhere via `{{< chart "us-pop-bar" >}}` (Hugo) or transclusion `![[us-pop-bar]]` (Obsidian, plugin-dependent). Sidecar id: just `<slug>`.

### 5.3 Sidecar generation

`just charts-render`:

1. Walk `content/**/*.md`, extract every ```vega-lite``` fence + every `type: chart` page.
2. For each, run resolver → get canonical spec → compute `sha1(canonical_spec)`.
3. If `assets/charts/<chart-id>.svg` missing or its hash sidecar (`<chart-id>.svg.hash`) differs → call `vl-convert vl2svg` → write SVG + hash.
4. Clean orphans: any `assets/charts/<chart-id>.svg`, `<chart-id>.svg.hash`, `<chart-id>.svg.failed`, AND any in-page `<!-- BEGIN chart-preview:<chart-id> -->`...`<!-- END chart-preview:<chart-id> -->` block whose `<chart-id>` no longer matches a current chart is removed, unless `--keep-orphans`.

`just charts-render-one <chart-id>` operates on a single chart for fast iteration.

`just build` invokes `charts-render` before `hugo`.

## 6. Lint codes

### 6.1 D-codes (datasets)

| Code | Level | Check |
|------|-------|-------|
| D1 | error | `type: dataset` missing required frontmatter (`storage`, `format`). |
| D2 | error | `storage: file` but `data_path:` missing or file not on disk. |
| D3 | error | `storage: inline` but no fenced block under `## Data`. |
| D4 | error | `format` in frontmatter ≠ fence info-string. |
| D5 | error | `columns:` declared but data row violates declared types. Sampled (first 50 + last 10 rows) at lint time; full check via `just dataset-validate`. |
| D6 | warn | Inline dataset exceeds `AWIKI_DATASET_INLINE_MAX_ROWS` or `_MAX_BYTES`. Suggests `dataset-compact`. |
| D7 | warn | `rows:` frontmatter stale vs actual count. Auto-fixable via `just lint-fix`. |
| D8 | warn | `data_path:` outside `data/` (or `data/private/`). |
| D9 | info | Dataset page has zero `## Sources` entries (provenance missing). |

### 6.2 C-codes (charts)

| Code | Level | Check |
|------|-------|-------|
| C1 | error | ```vega-lite``` fence body fails JSON parse. |
| C2 | error | Spec missing required keys (`mark` or one of `layer`/`hconcat`/`vconcat`/`facet`). |
| C3 | error | `data.name: "[[slug]]"` resolves to non-existent slug or non-`dataset` page. |
| C4 | error | Spec references a field not in target dataset's `columns:` (only when both sides declare). |
| C5 | error | `type: chart` page with `chart_engine: vega-lite` has zero ```vega-lite``` fences. |
| C6 | warn | `chart_data:` frontmatter pointer disagrees with body's `data.name`. Auto-fixable. |
| C7 | warn | Sidecar `assets/charts/<chart-id>.svg` missing or hash-stale. Suggests `charts-render`. |
| C8 | info | Chart references same dataset >5 times across the wiki (suggest a `type: chart` page for reuse). |
| C9 | warn | Hand-edit detected inside a `<!-- BEGIN chart-preview:<chart-id> -->` managed region. Re-run `charts-render` to restore. Mirrors task-layer T7. |
| C-PRIV | error | Chart in a non-private page references a dataset with `tags: [private]`. Mirrors existing privacy rule. |

### 6.3 Integration

- `scripts/lint.sh` calls `lint-data.sh` and `lint-chart.sh` after existing checks.
- `--only=data`, `--only=chart` filters supported.
- `--fix` handles D7 (recount rows) and C6 (sync chart_data pointer). Other auto-fixes deferred.

## 7. MCP tools (read-only)

Added to `mcp/awiki-server/`:

```
list_datasets() → { datasets: [{slug, format, storage, rows, columns?, tags, last_updated}], errors: [] }
```

```
get_dataset(slug) → {
  slug, frontmatter, schema?,
  sample_rows,         # first 20
  total_rows,
  source_paths,
  errors?              # structured payload, not MCP-level error
}
```

- `slug` matches `^[a-z0-9][a-z0-9-]*$`.
- Sample rows read from inline fence or sidecar file. `data_path` realpath-checked against `data/` to defeat path traversal.

```
list_charts() → { charts: [{slug?, page, fence_index?, engine, data_ref, sidecar_path?}], errors: [] }
```

Walks all markdown for ```vega-lite``` fences + `type: chart` pages. `slug` set only for dedicated chart pages; inline charts identified by `(page, fence_index)` tuple.

Argument validation, regex rules, and realpath checks match existing synth tools.

## 8. Just recipes

| Recipe | Purpose |
|--------|---------|
| `just data-init` | Scaffold data layer, opt-in. Per-step idempotent. |
| `just dataset-new <slug> [--format=csv] [--from=<path>]` | Scaffold a dataset page; optionally seed from a local file. |
| `just dataset-compact <slug>` | Flip inline ↔ file at threshold. Idempotent. |
| `just dataset-validate <slug>` | Re-count rows + type-check schema. |
| `just charts-render` | Walk all charts, regen stale `assets/charts/*.svg`. |
| `just charts-render-one <chart-id>` | Single-chart regen. |
| `just data-help` | Per-recipe usage / output / when notes. Lives at `docs/data-help.txt`. |

`just build` augmented to call `charts-render` first. `just check-deps` adds `vl-convert` as optional, warns when missing AND `AWIKI_DATA_LAYER=on`.

## 9. Error handling + edge cases

### 9.1 Resolver failures

- Unknown slug → C3 lint error AND build fails (render-hook returns non-zero, Hugo aborts).
- Slug resolves to non-dataset → C3 with reason `not-a-dataset`.
- Resolver crash mid-build → emits `<div class="vega-error">` placeholder + stderr log; never silently renders an empty chart.

### 9.2 Sidecar staleness

- Hash mismatch → C7 warn; regen at `charts-render`.
- Missing `vl-convert` → `charts-render` exits non-zero with install instructions; build proceeds without sidecars (Obsidian-with-plugin still renders live; without plugin, raw fence is shown).
- vl-convert non-zero exit (malformed spec post-resolve) → emits `RENDER|<chart-id>|<stderr>`; writes `<chart-id>.svg.failed` placeholder so subsequent runs retry only on hash change.

### 9.3 Privacy

- Lint D8 warns on `data_path` outside `data/`.
- Lint C-PRIV blocks publishing private data via a non-private chart.
- `data-init` git-crypt extension is opt-in. If declined and a private dataset exists, lint warns on every `data-init` run thereafter.

### 9.4 Threshold / compaction

- `dataset-compact` is idempotent.
- inline → file: writes `data/<slug>.<ext>`, removes `## Data` block, sets `storage: file`, sets `data_path:`. Single commit's worth of changes.
- file → inline: refuses if file >threshold; otherwise embeds rows, deletes `data_path:`, removes file.

### 9.5 Schema drift

- New column in data, declared schema → D5 error (user updates schema or removes column).
- Type widening (integer → number) → D5 error; no auto-widen.
- Schema removed → opaque dataset, no D5 ever fires (matches Q3=b).

### 9.6 Render-time edge cases

- Multiple fences per page → stable `<page-slug>-fig<N>` ids, zero-indexed by document order. Re-ordering renames sidecars; orphan cleanup removes the old SVGs.
- Fence inside synthesis BEGIN/END markers → allowed; C-codes don't interfere with S-codes.
- Fence in private page → sidecar lands in `assets/charts/private/`; git-crypt pattern covers it.

### 9.7 Engine forward-compat

- `chart_engine: vega-lite` is the only engine in this spec. Any other value is ignored by the render-hook so future engines (matplotlib, mermaid, …) can wire their own paths without a breaking change.

## 10. Testing

### 10.1 BATS suite additions

| File | Coverage |
|------|----------|
| `tests/data_init_test.bats` | Idempotence; WIKI.md patch; config flag; git-crypt extension; second-run no-op. |
| `tests/dataset_new_test.bats` | Scaffold; `--from=` seeds; format detection; refuses existing slug. |
| `tests/dataset_compact_test.bats` | inline → file, file → inline, idempotent, refuse-when-over-threshold both directions. |
| `tests/dataset_validate_test.bats` | Row count update; schema pass + fail per type; no-schema clean. |
| `tests/lint_data_test.bats` | One test per D-code; pass + fail fixtures; `--fix` on D7. |
| `tests/lint_chart_test.bats` | One test per C-code; pass + fail fixtures; `--fix` on C6. |
| `tests/chart_render_test.bats` | SVG produced when vl-convert present; skipped + warn when absent; hash-stable no-op; orphan cleanup of `.svg` + `.hash` + `.failed` + in-page managed-region; `AWIKI_CHART_OBSIDIAN_PREVIEW=off` leaves managed region empty. |
| `tests/chart_resolver_test.bats` | `[[slug]]` → URL (file storage); `[[slug]]` → values (inline); raw URL passthrough; inline values passthrough; broken slug error. |
| `tests/mcp_data_test.bats` | `list_datasets`, `get_dataset` happy path + missing slug + path-traversal attempt; `list_charts` enumeration. |
| `tests/hugo_render_test.bats` | Build fixture wiki; output HTML has `<div class="vega-embed">`; lazy script tag injected only on chart pages. |
| `tests/privacy_data_test.bats` | Private dataset under `data/private/`; non-private chart referencing it → C-PRIV. |

### 10.2 Fixtures

- `tests/fixtures/data-layer/` — sample wiki with 3 datasets (csv inline, csv file, topojson file), 4 charts (inline-bar, embedded-chart-page, broken-resolver, private-cross-ref).
- Golden sidecars: stored under `tests/fixtures/data-layer/expected/*.svg`. If vl-convert version drift breaks byte compare, switch to schema/structural compare.

### 10.3 Manual smoke test (in README)

1. `just data-init` → config flag, dirs, WIKI.md patch.
2. `just dataset-new demo --format=csv --from=tests/fixtures/demo.csv` → page exists, rows seeded.
3. Edit a concept page, add ```vega-lite``` fence with `data.name: "[[demo]]"`.
4. `just charts-render` → SVG appears in `assets/charts/`.
5. `just build` → output HTML contains `<div class="vega-embed">` + lazy script tag.
6. `just lint` → clean.

### 10.4 CI

- `.github/workflows/awiki-ci.yml.example` adds a `vl-convert` install step (pinned release URL).
- BATS suite runs on every push (already exists).

## 11. Out of scope (deferred specs)

- **SP3** — typed frontmatter on non-dataset pages (records-as-pages query layer).
- **SP4** — external data refresh / scheduled materialization.
- DuckDB query layer over datasets.
- Non-Vega-Lite chart engines.
- LLM-assisted dataset construction from sources (could be a synthesis plugin later).
