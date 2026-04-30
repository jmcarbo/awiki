<!-- BEGIN data-layer -->
## Data Layer (opt-in)

This block is managed by `awiki data-init`. To remove the layer,
delete everything between the BEGIN and END markers and run
`awiki lint` to surface broken references.

### Page kinds

- `dataset` — structured rows with optional schema. See Plan 1 docs.
- `chart` — Vega-Lite spec, embeddable via shortcode or `![[slug]]`.

### Inline charts

Drop a fenced block in any markdown page:

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

The resolver expands `data.name: "[[slug]]"` to a dataset URL (file
storage) or `values: [...]` (inline storage). Raw `data.url` and inline
`data.values` pass through.

Sidecar SVGs render below the fence inside `<!-- BEGIN chart-preview:<id> -->` /
`<!-- END chart-preview:<id> -->` markers for plain-Obsidian preview.
Suppress with `AWIKI_CHART_OBSIDIAN_PREVIEW=off`.

### Recipes

| Recipe | Purpose |
|--------|---------|
| `just data-init` | Scaffold (idempotent). |
| `just dataset-new <slug> [--format=csv] [--from=<path>]` | New dataset page. |
| `just dataset-compact <slug>` | Flip inline ↔ file at threshold. |
| `just dataset-validate <slug>` | Recount rows + schema-check. |
| `just chart-new <slug> --data=<dataset-slug>` | Scaffold a chart page. |
| `just charts-render` | Walk all charts, regen stale SVGs. |
| `just charts-render-one <chart-id>` | Single chart fast iteration. |

### Lint codes (data layer)

D1–D9 — datasets (see Plan 1 docs).

C1 error — fence body fails JSON parse.
C2 error — spec missing `mark` / `layer` / `hconcat` / `vconcat` / `facet`.
C3 error — `data.name: "[[slug]]"` resolves to non-existent / non-dataset page.
C4 error — spec field not in target dataset's `columns:` (when both declared).
C5 error — `type: chart` page with `chart_engine: vega-lite` has no fence.
C6 warn — `chart_data:` frontmatter pointer disagrees with body. `--fix`-able.
C7 warn — sidecar `assets/charts/<id>.svg` missing or hash-stale.
C8 info — chart references same dataset >5 times across the wiki.
C9 warn — hand-edit detected inside `<!-- BEGIN chart-preview:* -->`.
C-PRIV error — chart in non-private page references private dataset.

### MCP tools

- `list_datasets()` — Plan 1.
- `get_dataset(slug)` — Plan 1.
- `list_charts()` — every fence + every `type: chart` page.
<!-- END data-layer -->
