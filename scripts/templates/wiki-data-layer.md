<!-- BEGIN data-layer -->
## Data Layer (opt-in)

This block is managed by `scripts/data-init.sh`. To remove the layer,
delete everything between the BEGIN and END markers and run
`bash scripts/lint.sh` to surface broken references.

### Page kind: `dataset`

Page kind enum is extended with `dataset`. Frontmatter:

```yaml
---
type: dataset
storage: inline | file
format: csv | tsv | json | dsv | topojson
columns:                # optional; lint enforces types when present
  - { name: <col>, type: integer | number | string | boolean }
rows: <int>             # cached count, refreshed by dataset-validate
data_path: data/<slug>.<ext>   # only when storage=file
---
```

Body sections: lead paragraph, `## Schema`, `## Data` (only when
`storage: inline`), `## Provenance`, `## Related`, `## Sources`.

### Recipes

| Recipe | Purpose |
|--------|---------|
| `just data-init` | Scaffold (idempotent). |
| `just dataset-new <slug> [--format=csv] [--from=<path>]` | New dataset page. |
| `just dataset-compact <slug>` | Flip inline ↔ file at threshold. |
| `just dataset-validate <slug>` | Recount rows, schema-check. |

### Lint codes (datasets)

| Code | Level | Check |
|------|-------|-------|
| D1 | error | Missing `storage` / `format`. |
| D2 | error | `storage: file` but `data_path:` missing or file absent. |
| D3 | error | `storage: inline` but no fenced block under `## Data`. |
| D4 | error | `format` ≠ fence info-string. |
| D5 | error | Schema declared, sample row violates declared types. |
| D6 | warn | Inline dataset over `AWIKI_DATASET_INLINE_MAX_ROWS` / `_MAX_BYTES`. |
| D7 | warn | Cached `rows:` ≠ actual count. Auto-fixable. |
| D8 | warn | `data_path:` outside `data/`. |
| D9 | info | Dataset page has empty or absent `sources:` frontmatter. |

Chart subsystem ships in Plan 2 — `type: chart` and `vega-lite` rendering arrive there.
<!-- END data-layer -->
