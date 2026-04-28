---
title: "XLSX (and XLS/ODS) Batch Ingest"
date: 2026-04-28
status: design
---

# XLSX Batch Ingest — Design

## Goal

Drop spreadsheet files (`.xlsx`, `.xls`, `.ods`) into `raw/inbox/`. They flow through the existing ingest pipeline, becoming one `source` page **per sheet** plus per-sheet CSV artifacts. Both manual (`just ingest-xlsx <path>`) and auto-via-watchdog paths are supported, mirroring the existing `ingest-pdf.sh` / `ingest-audio.sh` pattern.

## Non-goals

- Direct creation of `content/datasets/*.md` entries. CSVs are written so the agent (or a follow-up `just dataset new --from=...`) can promote sheets into datasets later.
- Formula evaluation tuning. We trust calamine's evaluated cell values.
- Image / chart extraction from workbooks.
- Refactoring `ingest-pdf.sh` / `ingest-audio.sh` into a generic preconvert dispatcher (deferred until a third format converges).

## Architecture

Three new units, two extensions:

**New**
- `scripts/lib/xlsx-extract.py` — pure extractor. Reads workbook via `python-calamine`. Writes per-sheet CSV + per-sheet MD with frontmatter. Stdout = JSON manifest line consumed by the bash wrapper.
- `scripts/ingest-xlsx.sh` — bash wrapper. Validates path, archives original, calls extractor, emits log lines. Single source of filesystem-layout truth.
- `just ingest-xlsx <path>` recipe.

**Extended**
- `scripts/watchdog.sh` — extension dispatch in `process_file`: `.xlsx|.xls|.ods` → call `ingest-xlsx.sh` instead of `ingest.sh`. Derived `.md` files re-enter the watch loop.
- `scripts/check-deps.sh` — register `python-calamine` as optional, with install hint.

**Boundaries**
- Extractor is pure-Python, knows nothing about awiki paths beyond what bash passes in. Testable standalone.
- Bash wrapper owns filesystem layout (where originals/CSVs/MDs go).
- Watchdog only learns "binary X needs preconversion to Y"; no per-format logic inside.

## Components

### `scripts/lib/xlsx-extract.py`

CLI:
```
xlsx-extract.py --in <xlsx>
                --out-dir <dir>           # md destination (filesystem path)
                --csv-dir <dir>           # csv destination (filesystem path)
                --csv-rel <repo-rel>      # csv path encoded in frontmatter (e.g. raw/processed/_originals/foo)
                --original-rel <repo-rel> # archived original path encoded in frontmatter
                --slug-prefix <slug>      # workbook slug
                [--preview-rows N]        # default 50
```

The wrapper passes repo-relative paths in `--csv-rel` and `--original-rel` so the extractor can write provenance into frontmatter without doing repo-root resolution itself.

Behavior:
- Loads via `python_calamine.CalamineWorkbook.from_path(...)`.
- For each sheet:
  - Skip if hidden (`sheet.visible == False`) → log `XLSX-SKIP|sheet=<name>|reason=hidden`.
  - Skip if empty (zero non-blank rows after trim) → log `XLSX-SKIP|sheet=<name>|reason=empty`.
  - Compute `sheet_slug = "<workbook>--<sheet>"`, kebab-cased: lowercase, non-`[a-z0-9-]` → `-`, collapse repeats, strip leading/trailing `-`.
  - Resolve in-workbook collisions by `-2`, `-3`, ... suffix.
  - Write full CSV → `<csv-dir>/<sheet_slug>.csv` (UTF-8, `csv.writer`, RFC4180 quoting).
  - Write MD → `<out-dir>/<sheet_slug>.md` with frontmatter:
    ```yaml
    title: "<workbook> — <sheet>"
    date: <today>
    last_updated: <today>
    type: source
    tags: [xlsx]
    aliases: []
    sources: []
    workbook: <basename-with-ext>
    sheet: <original sheet name>
    rows_total: N
    rows_preview: min(N, preview_rows)
    columns: [<header strings>]
    csv: raw/processed/_originals/<workbook_slug>/<sheet_slug>.csv
    original: raw/processed/_originals/<workbook_slug>/<basename-with-ext>
    draft: false
    ```
  - Body sections:
    - `## Preview` — markdown table of first `preview_rows` data rows (header + body).
    - `## Schema` — bulleted column list with inferred type (`text` | `number` | `date` | `bool` | `mixed`) sampled from the column.
    - `## Notes` — empty placeholder for the agent to populate during the WIKI.md §4.1 ingest workflow.
- Header inference: first row treated as headers when all cells are non-empty strings; otherwise synthesize `col_1, col_2, ...`. Headers slugified for CSV (preserve original strings in the `columns:` frontmatter list).
- Atomicity: writes go to `<out-dir>/.xlsx-extract-<pid>/` first; renamed into place only after every sheet succeeds. On any failure, the temp dir is removed.
- Stdout: single JSON line on success — `{"workbook_slug": "...", "sheets": [{"name": ..., "slug": ..., "rows_total": N, "rows_preview": M, "csv": "...", "md": "..."}, ...]}`.
- Exit codes: `0` ok, `2` invalid input args, `3` calamine parse error, `4` IO write failure.

### `scripts/ingest-xlsx.sh`

Args: `<path-under-raw/inbox/>`, optional `--preview-rows N`.

Flow:
1. Validate extension ∈ `{xlsx, xls, ods}`. On miss → `XLSX-ERROR|reason=bad-ext`, exit 2.
2. Validate file exists, path under `raw/inbox/`. Exit 1 / 2 respectively.
3. Preflight: `python3 -c "import python_calamine"` — on failure print `XLSX-ERROR|reason=missing-dep` + install hint, exit 5. Honors `AWIKI_FAKE_MISSING=python_calamine` (forces the failure path) to keep the test hook consistent with `check-deps.sh`.
4. Compute `WORKBOOK_SLUG` from basename (sans extension), kebab-cased.
5. `ORIG_DIR=raw/processed/_originals/$WORKBOOK_SLUG`. Copy original there preserving filename.
6. `OUT_DIR`: same dir as source `.xlsx` (so watchdog re-detects md siblings).
   `CSV_DIR`: `$ORIG_DIR`.
7. Invoke extractor; capture stdout JSON manifest.
8. Parse manifest with inline `python3 -c`; emit one `XLSX-SHEET|slug=...|rows=...|md=...|csv=...` line per sheet.
9. If `sheets == []` → `XLSX-EMPTY|workbook=<slug>`, exit 6, **do not delete source** (so the user notices).
10. Remove source `.xlsx` after extractor exits 0.
11. Final lines:
    ```
    XLSX-CONVERTED|in=<src>|workbook=<slug>|sheets=N|orig=<orig-path>
    XLSX-NEXT|<md-path>          # one per sheet
    ```
12. `bash scripts/log-append.sh xlsx "converted <basename> sheets=N"` on success; same prefix with `failed` on terminal error.

`AWIKI_XLSX_PREVIEW_ROWS` env (default `50`) is read and passed through unless `--preview-rows` overrides.

### `scripts/watchdog.sh` change

New helper `dispatch_path "$path"` switches on extension:
- `.xlsx | .xls | .ods` → `bash "$SCRIPT_DIR/ingest-xlsx.sh" "$path"`.
- default → `run_ingest "$path"`.

`process_file` calls `dispatch_path` instead of `run_ingest`. Failure → existing `quarantine` flow moves the binary to `raw/inbox/batch/_failed/`.

`AWIKI_XLSX_PREVIEW_ROWS` propagates from process env (no new flag needed).

### `scripts/check-deps.sh`

Append after the existing `pyyaml` block:
```bash
if command -v python3 >/dev/null 2>&1 && python3 -c "import python_calamine" >/dev/null 2>&1; then
  echo "OK|python-calamine (optional)"
else
  echo "OPTIONAL-MISSING|python-calamine"
  echo "  install: pip3 install python-calamine" >&2
fi
```

### `justfile`

```make
ingest-xlsx path *flags:
    bash scripts/ingest-xlsx.sh {{path}} {{flags}}
```

Place under the existing `# === ingest ===` section, beside `ingest-with-agent`.

## Data flow

### Manual
```
user drops foo.xlsx anywhere under raw/inbox/
  └─ just ingest-xlsx raw/inbox/.../foo.xlsx
       ├─ archive raw/processed/_originals/foo/foo.xlsx
       ├─ extractor:
       │    └─ for each visible non-empty sheet:
       │         ├─ raw/processed/_originals/foo/foo--sales.csv
       │         └─ <same-dir-as-foo.xlsx>/foo--sales.md
       ├─ rm raw/inbox/.../foo.xlsx
       └─ prints XLSX-NEXT|<md> per sheet
  └─ user runs `just ingest <md>` per sheet (or batch via watchdog).
```

### Auto via watchdog
```
user drops foo.xlsx into raw/inbox/batch/
  └─ watchdog detects, waits for stable size
  └─ dispatch_path: ext .xlsx → ingest-xlsx.sh
       └─ writes foo--sales.md, foo--inventory.md into raw/inbox/batch/
       └─ archives original, removes raw/inbox/batch/foo.xlsx
  └─ watchdog next cycle:
       └─ detects foo--sales.md (text) → run_ingest → ingest.sh
            └─ moves to raw/processed/batch/foo--sales.md
            └─ emits AGENT-PROMPT
       └─ same for foo--inventory.md
  └─ agent processes each per WIKI.md §4.1 steps 3-9
```

### Re-entry safety

- Once original `.xlsx` is removed, no chance of re-extraction.
- Generated `.md` siblings re-enter the same watchdog loop and flow through standard `ingest.sh`.
- No loops; existing `SEEN[]` guards + `ingest.sh` "already processed" exit 3 prevent double-move.

### Slug collisions across workbooks

If two workbooks both produce slug `q1--sales`, the second extraction's `.md` reaches `ingest.sh`, which exits 3 (`already processed`). Watchdog quarantines the second `.md` to `_failed/`. The user resolves by renaming one workbook before retrying. Acceptable.

## Error handling

| Failure | Where | Response | Wrapper exit |
|---|---|---|---|
| `python-calamine` not installed | bash preflight | `XLSX-ERROR\|reason=missing-dep` + install hint | 5 |
| Corrupt/locked workbook | calamine raises | `XLSX-ERROR\|reason=parse\|file=...\|err=<short>` | 3 |
| Path not under `raw/inbox/` | wrapper validation | `XLSX-ERROR\|reason=bad-path` | 2 |
| Wrong extension | wrapper | `XLSX-ERROR\|reason=bad-ext` | 2 |
| Missing source | wrapper | `XLSX-ERROR\|reason=not-found` | 1 |
| All sheets empty/hidden | extractor returns empty manifest | `XLSX-EMPTY\|workbook=<slug>`; source preserved | 6 |
| IO write failure (full disk, perms) | extractor | `XLSX-ERROR\|reason=io\|path=...` | 4 |
| Slug collision in same workbook | extractor | suffix `-2`, `-3`; logs `XLSX-DUP\|slug=...\|resolved=...` | n/a |
| Slug collision across workbooks | downstream `ingest.sh` exit 3 | watchdog quarantines `.md` to `_failed/` | n/a |

**Atomicity:** extractor writes to a temp dir, only renames into place after all sheets succeed. Any mid-flight failure removes the temp dir; original `.xlsx` is preserved until extractor exits 0.

**Logging:** all messages use `XLSX|...` prefix. `log-append.sh xlsx "<msg>"` on terminal success/failure; matches project convention (`INGEST|`, `WATCHDOG|`, `LINT|`, `DATASET|`).

## Testing

### `tests/ingest_xlsx.bats` (shell-level)

Setup: temp repo with `raw/inbox/batch/`, fixture xlsx files committed under `tests/fixtures/xlsx/` (≤5KB each).

Cases:
1. Single-sheet `.xlsx` → md + csv produced, frontmatter has `rows_total`, original archived, source removed, exit 0.
2. Multi-sheet `.xlsx` → N md + N csv, slugs unique, all under expected dirs.
3. Hidden sheet skipped, empty sheet skipped, log lines present.
4. All-hidden workbook → `XLSX-EMPTY`, wrapper exit 6, source NOT removed.
5. Bad extension (`.txt`) → exit 2.
6. Path outside `raw/inbox/` → exit 2.
7. Missing `python-calamine` (mock via `AWIKI_FAKE_MISSING=python_calamine`) → exit 5.
8. Corrupt xlsx (truncated bytes) → exit 3, source NOT removed.
9. `--preview-rows=10` honored: md table has ≤10 rows, csv has full rows.
10. Slug-collision within workbook (two sheets kebab to same slug) → second gets `-2` suffix.
11. Atomicity: simulated IO error mid-extract → temp dir cleaned, no partial files in out-dir.

### `tests/watchdog_xlsx.bats` (integration)

Use existing `--once --catchup --backend=poll` mode.
- Drop `.xlsx` → assert `WATCHDOG|detect`, `XLSX-CONVERTED`, then `WATCHDOG|detect` for each generated `.md`, then `INGEST-OK` for each.
- Drop corrupt `.xlsx` → assert quarantined to `_failed/`.
- `AWIKI_XLSX_PREVIEW_ROWS=5` env propagated end-to-end.

### `tests/check_deps_test.bats`

Extend with `python-calamine` optional check (OK + OPTIONAL-MISSING branches via `AWIKI_FAKE_MISSING=python_calamine`).

### `scripts/lib/xlsx-extract-test.py` (Python unit)

- Hidden detection.
- Slug kebab-casing.
- CSV RFC4180 quoting (cells with `,`, `"`, embedded newlines).
- Header inference (first row = headers; numeric-only or empty header → `col_1`, `col_2`, ...).
- Frontmatter YAML round-trip parses.

### Fixtures

Committed under `tests/fixtures/xlsx/`:
- `single-sheet.xlsx`
- `multi-sheet-with-hidden.xlsx`
- `empty-and-hidden.xlsx`
- `collision-sheet-names.xlsx`
- `corrupt.xlsx` (truncated bytes)

A `tests/fixtures/xlsx/_generate.py` script lives alongside, regenerable by humans when fixtures need refresh. Not run in CI.

### CI gate

`python-calamine` is optional. BATS tests `bats_skip` when `python3 -c "import python_calamine"` fails, matching the `pdftotext` test pattern. CI without the dep still passes.

## Open questions

None.
