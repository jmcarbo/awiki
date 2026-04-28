# XLSX Batch Ingest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `.xlsx` / `.xls` / `.ods` ingest to `raw/inbox/`, mirroring `ingest-pdf.sh` / `ingest-audio.sh`. One source page is produced per sheet plus per-sheet CSVs, both via manual `just ingest-xlsx <path>` and auto-via-`watchdog.sh`.

**Architecture:** A pure-Python extractor (`scripts/lib/xlsx-extract.py`, calamine-backed) writes per-sheet markdown + CSV under control of a thin bash wrapper (`scripts/ingest-xlsx.sh`) that owns filesystem layout. `scripts/watchdog.sh` gains an extension-dispatch step so dropping a workbook into `raw/inbox/batch/` triggers extraction; the resulting `.md` siblings re-enter the standard ingest loop.

**Tech Stack:** Python 3.8+, [`python-calamine`](https://pypi.org/project/python-calamine/) (optional runtime dep, declared in `scripts/check-deps.sh`), bash 4+, BATS for tests, openpyxl (dev-only, used by the fixture generator script — never invoked in CI).

**Deviation from spec:** spec mentioned `scripts/lib/xlsx-extract-test.py` for Python-side unit tests. Project convention is BATS-only for Python scripts (see `tests/ingest_git_transform_test.bats`), so all tests live in `tests/ingest_xlsx_test.bats` and `tests/watchdog_xlsx_test.bats`. No standalone Python unittest file.

---

## File Map

**Create**
- `scripts/lib/xlsx-extract.py` — pure extractor (calamine, csv, json, argparse)
- `scripts/ingest-xlsx.sh` — bash wrapper
- `tests/fixtures/xlsx/_generate.py` — openpyxl fixture builder (dev-only)
- `tests/fixtures/xlsx/single-sheet.xlsx` — committed fixture (built by `_generate.py`)
- `tests/fixtures/xlsx/multi-sheet-with-hidden.xlsx`
- `tests/fixtures/xlsx/empty-and-hidden.xlsx`
- `tests/fixtures/xlsx/collision-sheet-names.xlsx`
- `tests/fixtures/xlsx/corrupt.xlsx` — truncated bytes
- `tests/ingest_xlsx_test.bats` — extractor + wrapper tests
- `tests/watchdog_xlsx_test.bats` — auto-pipeline integration test

**Modify**
- `scripts/watchdog.sh` — add `dispatch_path()` helper, switch `process_file` to use it
- `scripts/check-deps.sh` — register `python-calamine` as optional
- `tests/check_deps_test.bats` — add coverage for the new optional dep
- `justfile` — new `ingest-xlsx` recipe under `# === ingest ===`

---

## Task 1: Register `python-calamine` as an optional dep

**Files:**
- Modify: `scripts/check-deps.sh`
- Test: `tests/check_deps_test.bats`

- [ ] **Step 1: Write the failing test**

Append to `tests/check_deps_test.bats`:

```bash
@test "check-deps reports python-calamine status" {
  run bash "$BATS_TEST_DIRNAME/../scripts/check-deps.sh"
  echo "$output" | grep -E "^(OK\|python-calamine|OPTIONAL-MISSING\|python-calamine)" >/dev/null
}

@test "check-deps prints install hint when python-calamine missing" {
  run env AWIKI_FAKE_MISSING=python_calamine bash "$BATS_TEST_DIRNAME/../scripts/check-deps.sh"
  [[ "$output" == *"OPTIONAL-MISSING|python-calamine"* ]]
  [[ "$output" == *"pip3 install python-calamine"* ]]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run:
```bash
bats tests/check_deps_test.bats -f "python-calamine"
```

Expected: both new tests FAIL (no `python-calamine` lines in output yet).

- [ ] **Step 3: Implement the check**

Append to `scripts/check-deps.sh` after the existing `pyyaml` block (around the trailing optional-Python section):

```bash
# Optional Python module: python-calamine (used by scripts/ingest-xlsx.sh).
if [[ -n "${AWIKI_FAKE_MISSING:-}" && "$AWIKI_FAKE_MISSING" == "python_calamine" ]]; then
  echo "OPTIONAL-MISSING|python-calamine"
  echo "  install: pip3 install python-calamine" >&2
elif command -v python3 >/dev/null 2>&1 && python3 -c "import python_calamine" >/dev/null 2>&1; then
  echo "OK|python-calamine (optional)"
else
  echo "OPTIONAL-MISSING|python-calamine"
  echo "  install: pip3 install python-calamine" >&2
fi
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/check_deps_test.bats -f "python-calamine"`
Expected: both PASS. Then run the full file: `bats tests/check_deps_test.bats` — expect all pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/check-deps.sh tests/check_deps_test.bats
git commit -m "feat(deps): register python-calamine as optional dep"
```

---

## Task 2: Fixture generator + materialize fixtures

**Files:**
- Create: `tests/fixtures/xlsx/_generate.py`
- Create: `tests/fixtures/xlsx/single-sheet.xlsx`
- Create: `tests/fixtures/xlsx/multi-sheet-with-hidden.xlsx`
- Create: `tests/fixtures/xlsx/empty-and-hidden.xlsx`
- Create: `tests/fixtures/xlsx/collision-sheet-names.xlsx`
- Create: `tests/fixtures/xlsx/corrupt.xlsx`

This task produces committed binary fixtures used by every later task. Run the generator once locally (requires `openpyxl`), then commit the resulting `.xlsx` files.

- [ ] **Step 1: Install openpyxl locally (one-time, dev only)**

Run: `pip3 install openpyxl`

- [ ] **Step 2: Write the generator**

Create `tests/fixtures/xlsx/_generate.py`:

```python
#!/usr/bin/env python3
"""
Materialize xlsx fixtures used by tests/ingest_xlsx_test.bats and
tests/watchdog_xlsx_test.bats. Run manually after editing — fixtures
are committed binaries; CI does not regenerate them.

Requires: pip3 install openpyxl
"""
from __future__ import annotations
from pathlib import Path
from openpyxl import Workbook

FIXTURES = Path(__file__).resolve().parent


def write_single_sheet() -> None:
    wb = Workbook()
    ws = wb.active
    ws.title = "Sales"
    ws.append(["region", "amount", "date"])
    ws.append(["EMEA", 100, "2026-01-01"])
    ws.append(["AMER", 250, "2026-01-02"])
    ws.append(["APAC", 75, "2026-01-03"])
    wb.save(FIXTURES / "single-sheet.xlsx")


def write_multi_with_hidden() -> None:
    wb = Workbook()
    ws1 = wb.active
    ws1.title = "Sales"
    ws1.append(["region", "amount"])
    ws1.append(["EMEA", 100])
    ws2 = wb.create_sheet("Inventory")
    ws2.append(["sku", "qty"])
    ws2.append(["A1", 12])
    ws2.append(["B2", 7])
    ws3 = wb.create_sheet("HiddenScratch")
    ws3.append(["x", "y"])
    ws3.append([1, 2])
    ws3.sheet_state = "hidden"
    wb.save(FIXTURES / "multi-sheet-with-hidden.xlsx")


def write_empty_and_hidden() -> None:
    wb = Workbook()
    wb.active.title = "Empty"
    extra = wb.create_sheet("AlsoHidden")
    extra.append(["only", "header"])
    extra.sheet_state = "hidden"
    wb.save(FIXTURES / "empty-and-hidden.xlsx")


def write_collision() -> None:
    # Two sheets that kebab-slugify to the same string within the workbook.
    wb = Workbook()
    ws1 = wb.active
    ws1.title = "Sales Data"
    ws1.append(["region", "amount"])
    ws1.append(["EMEA", 100])
    ws2 = wb.create_sheet("sales-data")
    ws2.append(["region", "amount"])
    ws2.append(["AMER", 250])
    wb.save(FIXTURES / "collision-sheet-names.xlsx")


def write_corrupt() -> None:
    # Truncated zip header — calamine fails to parse.
    (FIXTURES / "corrupt.xlsx").write_bytes(b"PK\x03\x04not-a-real-xlsx")


def main() -> None:
    write_single_sheet()
    write_multi_with_hidden()
    write_empty_and_hidden()
    write_collision()
    write_corrupt()
    print("OK|fixtures regenerated under", FIXTURES)


if __name__ == "__main__":
    main()
```

- [ ] **Step 3: Run the generator**

Run: `python3 tests/fixtures/xlsx/_generate.py`
Expected: prints `OK|fixtures regenerated under .../tests/fixtures/xlsx`. Five `.xlsx` files exist:

```bash
ls tests/fixtures/xlsx/*.xlsx
```

Expected: 5 files (single-sheet, multi-sheet-with-hidden, empty-and-hidden, collision-sheet-names, corrupt).

- [ ] **Step 4: Sanity-check via calamine (optional, requires `pip3 install python-calamine`)**

```bash
python3 -c "
from python_calamine import CalamineWorkbook
wb = CalamineWorkbook.from_path('tests/fixtures/xlsx/multi-sheet-with-hidden.xlsx')
for s in wb.sheet_names:
    print(s, wb.get_sheet_by_name(s).visible)
"
```

Expected output includes `Sales True`, `Inventory True`, `HiddenScratch False`.

- [ ] **Step 5: Commit**

```bash
git add tests/fixtures/xlsx/_generate.py tests/fixtures/xlsx/*.xlsx
git commit -m "test(xlsx): commit ingest-xlsx fixtures and openpyxl generator"
```

---

## Task 3: `xlsx-extract.py` skeleton — slug helper + CLI scaffolding

**Files:**
- Create: `scripts/lib/xlsx-extract.py`
- Test: `tests/ingest_xlsx_test.bats`

The slug helper is exercised through the CLI to match project test conventions. This task lays down the file scaffold and the slug logic before any sheet work.

- [ ] **Step 1: Write the failing test**

Create `tests/ingest_xlsx_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  command -v python3 >/dev/null 2>&1 || skip "python3 not installed"
  python3 -c "import python_calamine" 2>/dev/null || skip "python-calamine not installed"
  WORK="$(mktemp -d)/repo"
  REPO="$BATS_TEST_DIRNAME/.."
  mkdir -p "$WORK/raw/inbox/batch" "$WORK/raw/inbox/interactive" "$WORK/raw/processed/_originals"
  cp "$REPO/tests/fixtures/xlsx/"*.xlsx "$WORK/raw/inbox/batch/" 2>/dev/null || true
  cd "$WORK"
}
teardown() { cd - >/dev/null; rm -rf "$WORK"; }

@test "xlsx-extract --slugify produces kebab-case" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --slugify "Q1 Report — Sales!"
  [ "$status" -eq 0 ]
  [ "$output" = "q1-report-sales" ]
}

@test "xlsx-extract --slugify trims leading and trailing dashes" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --slugify "---abc---"
  [ "$status" -eq 0 ]
  [ "$output" = "abc" ]
}

@test "xlsx-extract --help prints usage" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"--in"* ]]
  [[ "$output" == *"--out-dir"* ]]
  [[ "$output" == *"--csv-dir"* ]]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bats tests/ingest_xlsx_test.bats`
Expected: all FAIL (file does not exist yet).

- [ ] **Step 3: Implement the skeleton**

Create `scripts/lib/xlsx-extract.py`:

```python
#!/usr/bin/env python3
"""
Extract a workbook into per-sheet CSV + per-sheet markdown source pages.
Invoked by scripts/ingest-xlsx.sh. Pure: knows nothing about awiki paths
beyond what the bash wrapper passes in.
"""
from __future__ import annotations

import argparse
import re
import sys


def slugify(text: str) -> str:
    """Lowercase, collapse non [a-z0-9-] to '-', strip leading/trailing '-'."""
    s = text.lower()
    s = re.sub(r"[^a-z0-9-]+", "-", s)
    s = re.sub(r"-+", "-", s)
    return s.strip("-")


def build_parser() -> argparse.ArgumentParser:
    p = argparse.ArgumentParser(
        prog="xlsx-extract",
        description="Extract xlsx/xls/ods into per-sheet CSV + markdown.",
    )
    p.add_argument("--in", dest="in_path", help="Path to source workbook")
    p.add_argument("--out-dir", help="Filesystem dir for per-sheet .md output")
    p.add_argument("--csv-dir", help="Filesystem dir for per-sheet .csv output")
    p.add_argument("--csv-rel", help="Repo-relative csv dir, written into frontmatter")
    p.add_argument("--original-rel", help="Repo-relative archived-original path, written into frontmatter")
    p.add_argument("--slug-prefix", help="Workbook slug used to namespace sheet slugs")
    p.add_argument("--preview-rows", type=int, default=50, help="Rows shown in md preview table")
    p.add_argument("--slugify", metavar="TEXT", help="Print kebab-cased slug of TEXT and exit")
    return p


def main(argv: list[str]) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.slugify is not None:
        print(slugify(args.slugify))
        return 0
    parser.print_help(sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/ingest_xlsx_test.bats`
Expected: 3 PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/xlsx-extract.py tests/ingest_xlsx_test.bats
git commit -m "feat(xlsx): scaffold xlsx-extract.py with slugify helper"
```

---

## Task 4: Header inference + cell type inference

**Files:**
- Modify: `scripts/lib/xlsx-extract.py`
- Test: `tests/ingest_xlsx_test.bats`

These pure helpers feed CSV + markdown writers. Exposed via internal CLI hooks for testability.

- [ ] **Step 1: Write the failing tests**

Append to `tests/ingest_xlsx_test.bats`:

```bash
@test "xlsx-extract --infer-headers returns header strings when row is all strings" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-headers '["region","amount","date"]'
  [ "$status" -eq 0 ]
  [ "$output" = '["region", "amount", "date"]' ]
}

@test "xlsx-extract --infer-headers synthesises col_N when first row not all strings" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-headers '[1,2,3]'
  [ "$status" -eq 0 ]
  [ "$output" = '["col_1", "col_2", "col_3"]' ]
}

@test "xlsx-extract --infer-headers synthesises col_N when any cell empty" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-headers '["region","",""]'
  [ "$status" -eq 0 ]
  [ "$output" = '["col_1", "col_2", "col_3"]' ]
}

@test "xlsx-extract --infer-type classifies column samples" {
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[1,2,3]'
  [ "$output" = "number" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '["a","b"]'
  [ "$output" = "text" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[true,false]'
  [ "$output" = "bool" ]
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" --infer-type '[1,"a"]'
  [ "$output" = "mixed" ]
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bats tests/ingest_xlsx_test.bats -f "infer"`
Expected: all FAIL (`unrecognized arguments: --infer-headers`).

- [ ] **Step 3: Implement the helpers**

Edit `scripts/lib/xlsx-extract.py` — add helpers above `build_parser`:

```python
import json


def infer_headers(first_row: list) -> list[str]:
    """Return first_row when every cell is a non-empty string; else col_1..col_N."""
    if first_row and all(isinstance(c, str) and c.strip() for c in first_row):
        return [c for c in first_row]
    return [f"col_{i + 1}" for i in range(len(first_row))]


def infer_type(samples: list) -> str:
    """One of text|number|date|bool|mixed based on a column sample."""
    if not samples:
        return "text"
    seen: set[str] = set()
    for v in samples:
        if isinstance(v, bool):
            seen.add("bool")
        elif isinstance(v, (int, float)):
            seen.add("number")
        elif isinstance(v, str):
            seen.add("text")
        else:  # datetime, date, time, etc.
            seen.add("date")
        if len(seen) > 1:
            return "mixed"
    return next(iter(seen))
```

Add CLI hooks inside `main()` before `parser.print_help`:

```python
    if args.infer_headers is not None:
        print(json.dumps(infer_headers(json.loads(args.infer_headers))))
        return 0
    if args.infer_type is not None:
        print(infer_type(json.loads(args.infer_type)))
        return 0
```

Add to `build_parser()`:

```python
    p.add_argument("--infer-headers", metavar="JSON", help="Print headers inferred from a JSON list and exit")
    p.add_argument("--infer-type", metavar="JSON", help="Print type inferred from a JSON sample list and exit")
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/ingest_xlsx_test.bats -f "infer"`
Expected: 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/xlsx-extract.py tests/ingest_xlsx_test.bats
git commit -m "feat(xlsx): header + cell-type inference helpers"
```

---

## Task 5: Per-sheet extraction — happy-path single sheet end-to-end

**Files:**
- Modify: `scripts/lib/xlsx-extract.py`
- Test: `tests/ingest_xlsx_test.bats`

This is the meat of the extractor. Iterates visible non-empty sheets, writes one `.csv` and one `.md` per sheet (still without atomicity — added in the next task).

- [ ] **Step 1: Write the failing test**

Append to `tests/ingest_xlsx_test.bats`:

```bash
@test "xlsx-extract single-sheet writes csv + md with frontmatter and preview table" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/single-sheet.xlsx \
    --out-dir out \
    --csv-dir csv \
    --csv-rel raw/processed/_originals/single-sheet \
    --original-rel raw/processed/_originals/single-sheet/single-sheet.xlsx \
    --slug-prefix single-sheet \
    --preview-rows 10
  [ "$status" -eq 0 ]
  [ -f out/single-sheet--sales.md ]
  [ -f csv/single-sheet--sales.csv ]
  # Frontmatter has rows_total=3
  grep -E '^rows_total: 3$' out/single-sheet--sales.md
  # Frontmatter has csv path written verbatim from --csv-rel
  grep -F 'csv: raw/processed/_originals/single-sheet/single-sheet--sales.csv' out/single-sheet--sales.md
  # Frontmatter columns list preserves original strings
  grep -E '^columns: \["region", "amount", "date"\]$' out/single-sheet--sales.md
  # CSV body has the data row
  grep -F 'EMEA,100,2026-01-01' csv/single-sheet--sales.csv
  # Manifest emitted on stdout (single line of JSON)
  echo "$output" | python3 -c "import sys,json; m=json.load(sys.stdin); assert m['workbook_slug']=='single-sheet'; assert len(m['sheets'])==1; assert m['sheets'][0]['rows_total']==3"
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bats tests/ingest_xlsx_test.bats -f "single-sheet writes"`
Expected: FAIL — no extraction logic yet.

- [ ] **Step 3: Implement extraction**

Edit `scripts/lib/xlsx-extract.py`:

Add imports:

```python
import csv
import os
import sys
from datetime import date
from pathlib import Path
```

Add the extraction core above `main()`:

```python
def _stringify_cell(v) -> str:
    if v is None:
        return ""
    if isinstance(v, bool):
        return "true" if v else "false"
    return str(v)


def _md_table(headers: list[str], rows: list[list]) -> str:
    if not rows:
        return "(empty)\n"
    sep = "| " + " | ".join(headers) + " |\n"
    sep += "|" + "|".join("---" for _ in headers) + "|\n"
    for r in rows:
        cells = [_stringify_cell(c).replace("|", r"\|").replace("\n", " ") for c in r]
        sep += "| " + " | ".join(cells) + " |\n"
    return sep


def _frontmatter(
    *, title: str, today: str, workbook: str, sheet_name: str,
    rows_total: int, rows_preview: int, columns: list[str],
    csv_rel: str, original_rel: str,
) -> str:
    cols_json = json.dumps(columns)
    return (
        "---\n"
        f"title: \"{title}\"\n"
        f"date: {today}\n"
        f"last_updated: {today}\n"
        "type: source\n"
        "tags: [xlsx]\n"
        "aliases: []\n"
        "sources: []\n"
        f"workbook: {workbook}\n"
        f"sheet: \"{sheet_name}\"\n"
        f"rows_total: {rows_total}\n"
        f"rows_preview: {rows_preview}\n"
        f"columns: {cols_json}\n"
        f"csv: {csv_rel}\n"
        f"original: {original_rel}\n"
        "draft: false\n"
        "---\n\n"
    )


def _resolve_collision(slug: str, taken: set[str]) -> str:
    if slug not in taken:
        taken.add(slug)
        return slug
    i = 2
    while f"{slug}-{i}" in taken:
        i += 1
    resolved = f"{slug}-{i}"
    taken.add(resolved)
    return resolved


def _iter_sheet_rows(sheet) -> list[list]:
    """Return rows as list-of-lists, trimming trailing all-empty rows."""
    rows = sheet.to_python()
    while rows and all(c is None or (isinstance(c, str) and not c.strip()) for c in rows[-1]):
        rows.pop()
    return rows


def extract(args) -> int:
    # imported lazily so --help works without dep
    from python_calamine import CalamineWorkbook, SheetVisibleEnum
    in_path = Path(args.in_path)
    out_dir = Path(args.out_dir)
    csv_dir = Path(args.csv_dir)
    out_dir.mkdir(parents=True, exist_ok=True)
    csv_dir.mkdir(parents=True, exist_ok=True)

    try:
        wb = CalamineWorkbook.from_path(str(in_path))
    except Exception as e:  # parse error
        print(f"XLSX-ERROR|reason=parse|file={in_path}|err={e}", file=sys.stderr)
        return 3

    today = date.today().isoformat()
    sheets_out: list[dict] = []
    taken: set[str] = set()

    # Visibility lives on wb.sheets_metadata, NOT on CalamineSheet itself.
    # Filter Hidden + VeryHidden the same way (anything not Visible is skipped).
    for meta in wb.sheets_metadata:
        sheet_name = meta.name
        if meta.visible != SheetVisibleEnum.Visible:
            print(f"XLSX-SKIP|sheet={sheet_name}|reason=hidden", file=sys.stderr)
            continue
        sheet = wb.get_sheet_by_name(sheet_name)
        rows = _iter_sheet_rows(sheet)
        if not rows:
            print(f"XLSX-SKIP|sheet={sheet_name}|reason=empty", file=sys.stderr)
            continue

        headers = infer_headers(rows[0])
        original_first_row_was_headers = headers == [str(c) for c in rows[0]]
        data_rows = rows[1:] if original_first_row_was_headers else rows
        rows_total = len(data_rows)

        sheet_slug_raw = f"{args.slug_prefix}--{slugify(sheet_name)}"
        sheet_slug = _resolve_collision(sheet_slug_raw, taken)
        if sheet_slug != sheet_slug_raw:
            print(f"XLSX-DUP|slug={sheet_slug_raw}|resolved={sheet_slug}", file=sys.stderr)

        csv_path = csv_dir / f"{sheet_slug}.csv"
        with csv_path.open("w", encoding="utf-8", newline="") as f:
            writer = csv.writer(f)
            writer.writerow(headers)
            for r in data_rows:
                writer.writerow([_stringify_cell(c) for c in r])

        preview_rows = data_rows[: args.preview_rows]
        rows_preview = len(preview_rows)

        # Per-column type inference, sampling first 50 non-null cells.
        col_types: list[str] = []
        for ci in range(len(headers)):
            sample = []
            for r in data_rows[:50]:
                if ci < len(r) and r[ci] is not None and r[ci] != "":
                    sample.append(r[ci])
            col_types.append(infer_type(sample))

        csv_rel = f"{args.csv_rel.rstrip('/')}/{sheet_slug}.csv"
        fm = _frontmatter(
            title=f"{args.slug_prefix} — {sheet_name}",
            today=today,
            workbook=in_path.name,
            sheet_name=sheet_name,
            rows_total=rows_total,
            rows_preview=rows_preview,
            columns=headers,
            csv_rel=csv_rel,
            original_rel=args.original_rel,
        )
        body = "## Preview\n\n" + _md_table(headers, preview_rows) + "\n"
        body += "## Schema\n\n"
        for h, t in zip(headers, col_types):
            body += f"- `{h}` — {t}\n"
        body += "\n## Notes\n\n"
        md_path = out_dir / f"{sheet_slug}.md"
        md_path.write_text(fm + body, encoding="utf-8")

        sheets_out.append({
            "name": sheet_name,
            "slug": sheet_slug,
            "rows_total": rows_total,
            "rows_preview": rows_preview,
            "csv": str(csv_path),
            "md": str(md_path),
        })

    manifest = {"workbook_slug": args.slug_prefix, "sheets": sheets_out}
    print(json.dumps(manifest))
    return 0
```

In `main()`, route `--in` to `extract`:

```python
    if args.in_path:
        return extract(args)
```

Place this branch BEFORE `parser.print_help` and AFTER the `slugify`/`infer-*` short-circuits.

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/ingest_xlsx_test.bats -f "single-sheet writes"`
Expected: PASS.

Then run the full file: `bats tests/ingest_xlsx_test.bats`
Expected: every test so far PASSes.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/xlsx-extract.py tests/ingest_xlsx_test.bats
git commit -m "feat(xlsx): per-sheet csv+md extraction (single-sheet path)"
```

---

## Task 6: Multi-sheet, hidden-sheet, empty-sheet handling

**Files:**
- Test: `tests/ingest_xlsx_test.bats`

The implementation already supports these via `_iter_sheet_rows` + `getattr(sheet, "visible", True)`. This task locks in coverage with fixture-backed tests.

- [ ] **Step 1: Write the tests**

Append to `tests/ingest_xlsx_test.bats`:

```bash
@test "xlsx-extract multi-sheet skips hidden and processes visible non-empty" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/multi-sheet-with-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel raw/processed/_originals/multi --original-rel raw/processed/_originals/multi/multi-sheet-with-hidden.xlsx \
    --slug-prefix multi
  [ "$status" -eq 0 ]
  [ -f out/multi--sales.md ]
  [ -f out/multi--inventory.md ]
  [ ! -f out/multi--hiddenscratch.md ]
  echo "$output" | python3 -c "import sys,json; m=json.load(sys.stdin); names=[s['name'] for s in m['sheets']]; assert names==['Sales','Inventory'], names"
}

@test "xlsx-extract empty-and-hidden workbook returns empty manifest" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/empty-and-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix x
  [ "$status" -eq 0 ]
  echo "$output" | python3 -c "import sys,json; m=json.load(sys.stdin); assert m['sheets']==[]"
}

@test "xlsx-extract collision sheet names get -2 suffix" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/collision-sheet-names.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix wb
  [ "$status" -eq 0 ]
  [ -f out/wb--sales-data.md ]
  [ -f out/wb--sales-data-2.md ]
}

@test "xlsx-extract corrupt workbook exits 3" {
  mkdir -p out csv
  run python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/corrupt.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix bad
  [ "$status" -eq 3 ]
}
```

- [ ] **Step 2: Run tests to verify they pass (no impl change)**

Run: `bats tests/ingest_xlsx_test.bats`
Expected: all PASS. If any fails, the failure points to a specific gap; fix in `scripts/lib/xlsx-extract.py` before continuing.

- [ ] **Step 3: Commit**

```bash
git add tests/ingest_xlsx_test.bats
git commit -m "test(xlsx): cover multi-sheet, hidden, empty, collision, corrupt"
```

---

## Task 7: Atomicity — temp dir + rename

**Files:**
- Modify: `scripts/lib/xlsx-extract.py`
- Test: `tests/ingest_xlsx_test.bats`

Wrap extraction so partial output is impossible: write all sheets into a per-PID temp dir under each destination, then rename into place.

- [ ] **Step 1: Write the failing test**

Append to `tests/ingest_xlsx_test.bats`:

```bash
@test "xlsx-extract on injected mid-run failure leaves no partial files" {
  mkdir -p out csv
  # AWIKI_XLSX_FORCE_FAIL_AFTER=1 must abort after writing the 1st sheet — extractor honours it for tests.
  run env AWIKI_XLSX_FORCE_FAIL_AFTER=1 python3 "$BATS_TEST_DIRNAME/../scripts/lib/xlsx-extract.py" \
    --in raw/inbox/batch/multi-sheet-with-hidden.xlsx \
    --out-dir out --csv-dir csv \
    --csv-rel rel --original-rel rel/orig.xlsx --slug-prefix multi
  [ "$status" -eq 4 ]
  # No partial md/csv files in destinations, no leftover temp dirs.
  [ -z "$(ls -A out)" ]
  [ -z "$(ls -A csv)" ]
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bats tests/ingest_xlsx_test.bats -f "atomicity\|partial files"`
Expected: FAIL — no failure injection support yet, files written directly.

- [ ] **Step 3: Implement atomicity**

Edit `scripts/lib/xlsx-extract.py`:

Add to imports:

```python
import shutil
import tempfile
```

Replace the body of `extract` after the `wb = CalamineWorkbook.from_path(...)` call with a temp-dir wrapper. The cleanest delta: introduce two staging dirs, write into them, rename per-file at the end.

Concretely, inside `extract`:

```python
    out_dir.mkdir(parents=True, exist_ok=True)
    csv_dir.mkdir(parents=True, exist_ok=True)
    stage_md = Path(tempfile.mkdtemp(prefix=".xlsx-extract-", dir=str(out_dir)))
    stage_csv = Path(tempfile.mkdtemp(prefix=".xlsx-extract-", dir=str(csv_dir)))

    try:
        # ... existing per-sheet loop, but write csv_path / md_path into stage_csv / stage_md
        # After the loop, atomically rename each staged file into its final dest.
        force_fail = os.environ.get("AWIKI_XLSX_FORCE_FAIL_AFTER")
        for idx, s in enumerate(sheets_out):
            if force_fail is not None and idx + 1 == int(force_fail):
                raise IOError("forced failure for atomicity test")

        for s in sheets_out:
            staged_md = Path(s["md"])  # currently in stage_md
            staged_csv = Path(s["csv"])  # currently in stage_csv
            final_md = out_dir / staged_md.name
            final_csv = csv_dir / staged_csv.name
            os.replace(staged_md, final_md)
            os.replace(staged_csv, final_csv)
            s["md"] = str(final_md)
            s["csv"] = str(final_csv)
    except Exception as e:
        shutil.rmtree(stage_md, ignore_errors=True)
        shutil.rmtree(stage_csv, ignore_errors=True)
        # Best-effort: clear any final files we already renamed.
        for s in sheets_out:
            for p in (Path(s["md"]), Path(s["csv"])):
                if p.exists() and p.parent in (out_dir, csv_dir):
                    p.unlink(missing_ok=True)
        print(f"XLSX-ERROR|reason=io|err={e}", file=sys.stderr)
        return 4
    finally:
        shutil.rmtree(stage_md, ignore_errors=True)
        shutil.rmtree(stage_csv, ignore_errors=True)
```

Update the per-sheet loop to write into `stage_md` / `stage_csv`:

```python
        csv_path = stage_csv / f"{sheet_slug}.csv"
        # ... write csv writer ...
        md_path = stage_md / f"{sheet_slug}.md"
        # ... write md ...
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/ingest_xlsx_test.bats`
Expected: every test PASSes including the new atomicity case.

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/xlsx-extract.py tests/ingest_xlsx_test.bats
git commit -m "feat(xlsx): atomic extraction via per-pid stage dirs"
```

---

## Task 8: `scripts/ingest-xlsx.sh` — bash wrapper

**Files:**
- Create: `scripts/ingest-xlsx.sh`
- Test: `tests/ingest_xlsx_test.bats`

The wrapper validates input, archives the original, calls the extractor, parses the JSON manifest, and emits log lines.

- [ ] **Step 1: Write the failing tests**

Append to `tests/ingest_xlsx_test.bats`:

```bash
@test "ingest-xlsx archives original, removes source, prints XLSX-CONVERTED" {
  cp raw/inbox/batch/single-sheet.xlsx raw/inbox/interactive/single-sheet.xlsx
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/interactive/single-sheet.xlsx
  [ "$status" -eq 0 ]
  [ -f raw/processed/_originals/single-sheet/single-sheet.xlsx ]
  [ -f raw/processed/_originals/single-sheet/single-sheet--sales.csv ]
  [ -f raw/inbox/interactive/single-sheet--sales.md ]
  [ ! -f raw/inbox/interactive/single-sheet.xlsx ]
  [[ "$output" == *"XLSX-CONVERTED|"* ]]
  [[ "$output" == *"XLSX-NEXT|raw/inbox/interactive/single-sheet--sales.md"* ]]
}

@test "ingest-xlsx rejects bad extension" {
  echo "x" > raw/inbox/interactive/notes.txt
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/interactive/notes.txt
  [ "$status" -eq 2 ]
  [[ "$output" == *"reason=bad-ext"* ]]
}

@test "ingest-xlsx rejects path outside raw/inbox/" {
  cp raw/inbox/batch/single-sheet.xlsx /tmp/outside-$$.xlsx
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" "/tmp/outside-$$.xlsx"
  [ "$status" -eq 2 ]
  [[ "$output" == *"reason=bad-path"* ]]
  rm -f /tmp/outside-$$.xlsx
}

@test "ingest-xlsx exits 5 with hint when python-calamine is faked-missing" {
  run env AWIKI_FAKE_MISSING=python_calamine bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/batch/single-sheet.xlsx
  [ "$status" -eq 5 ]
  [[ "$output" == *"reason=missing-dep"* ]]
  [[ "$output" == *"pip3 install python-calamine"* ]]
}

@test "ingest-xlsx empty workbook exits 6 and preserves source" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/batch/empty-and-hidden.xlsx
  [ "$status" -eq 6 ]
  [[ "$output" == *"XLSX-EMPTY|"* ]]
  [ -f raw/inbox/batch/empty-and-hidden.xlsx ]
}

@test "ingest-xlsx corrupt workbook exits 3 and preserves source" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/batch/corrupt.xlsx
  [ "$status" -eq 3 ]
  [ -f raw/inbox/batch/corrupt.xlsx ]
}

@test "ingest-xlsx --preview-rows propagated" {
  cp raw/inbox/batch/single-sheet.xlsx raw/inbox/interactive/single-sheet.xlsx
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-xlsx.sh" raw/inbox/interactive/single-sheet.xlsx --preview-rows=1
  [ "$status" -eq 0 ]
  grep -E '^rows_preview: 1$' raw/inbox/interactive/single-sheet--sales.md
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `bats tests/ingest_xlsx_test.bats -f "ingest-xlsx"`
Expected: all FAIL — script does not exist.

- [ ] **Step 3: Implement the wrapper**

Create `scripts/ingest-xlsx.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi

PREVIEW_ROWS="${AWIKI_XLSX_PREVIEW_ROWS:-50}"
SRC=""
ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --preview-rows) PREVIEW_ROWS="$2"; shift 2 ;;
    --preview-rows=*) PREVIEW_ROWS="${1#--preview-rows=}"; shift ;;
    -h|--help)
      cat <<'EOF'
usage: ingest-xlsx.sh <path-under-raw/inbox/> [--preview-rows N]
EOF
      exit 0 ;;
    --) shift; ARGS+=("$@"); break ;;
    *) ARGS+=("$1"); shift ;;
  esac
done
set -- "${ARGS[@]}"
[[ $# -eq 1 ]] || { echo "XLSX-ERROR|reason=usage" >&2; exit 2; }
SRC="$1"

# Normalize to repo-relative
if [[ "$SRC" = /* ]]; then
  case "$SRC" in
    "$REPO_ROOT"/*) SRC="${SRC#"$REPO_ROOT/"}" ;;
    *) echo "XLSX-ERROR|reason=bad-path|src=$SRC" >&2; exit 2 ;;
  esac
fi

case "$SRC" in
  raw/inbox/*) ;;
  *) echo "XLSX-ERROR|reason=bad-path|src=$SRC" >&2; exit 2 ;;
esac

[[ -f "$SRC" ]] || { echo "XLSX-ERROR|reason=not-found|src=$SRC" >&2; exit 1; }

case "$SRC" in
  *.xlsx|*.xls|*.ods) ;;
  *) echo "XLSX-ERROR|reason=bad-ext|src=$SRC" >&2; exit 2 ;;
esac

# Preflight: python-calamine. Honour AWIKI_FAKE_MISSING for tests.
if [[ "${AWIKI_FAKE_MISSING:-}" == "python_calamine" ]] || \
   ! python3 -c "import python_calamine" >/dev/null 2>&1; then
  echo "XLSX-ERROR|reason=missing-dep|dep=python-calamine"
  echo "  install: pip3 install python-calamine" >&2
  exit 5
fi

BASE="$(basename "$SRC")"
EXT="${BASE##*.}"
STEM="${BASE%.*}"
SLUG="$(python3 "$SCRIPT_DIR/lib/xlsx-extract.py" --slugify "$STEM")"

ORIG_DIR="raw/processed/_originals/$SLUG"
mkdir -p "$ORIG_DIR"
cp "$SRC" "$ORIG_DIR/$BASE"

OUT_DIR="$(dirname "$SRC")"
CSV_DIR="$ORIG_DIR"

EXTRACT_RC=0
MANIFEST="$(python3 "$SCRIPT_DIR/lib/xlsx-extract.py" \
  --in "$SRC" \
  --out-dir "$OUT_DIR" \
  --csv-dir "$CSV_DIR" \
  --csv-rel "$ORIG_DIR" \
  --original-rel "$ORIG_DIR/$BASE" \
  --slug-prefix "$SLUG" \
  --preview-rows "$PREVIEW_ROWS")" || EXTRACT_RC=$?

if [[ "$EXTRACT_RC" -ne 0 ]]; then
  echo "XLSX-ERROR|reason=extract|src=$SRC|rc=$EXTRACT_RC" >&2
  bash "$SCRIPT_DIR/log-append.sh" xlsx "failed $BASE rc=$EXTRACT_RC" >/dev/null 2>&1 || true
  exit "$EXTRACT_RC"
fi

# Manifest is single-line JSON on stdout.
SHEETS=$(python3 -c "import json,sys; print(len(json.loads(sys.argv[1])['sheets']))" "$MANIFEST")
if [[ "$SHEETS" -eq 0 ]]; then
  echo "XLSX-EMPTY|workbook=$SLUG|src=$SRC"
  bash "$SCRIPT_DIR/log-append.sh" xlsx "empty $BASE" >/dev/null 2>&1 || true
  exit 6
fi

# Per-sheet log lines + XLSX-NEXT lines.
python3 - "$MANIFEST" "$OUT_DIR" <<'PY'
import json, os, sys
m = json.loads(sys.argv[1])
out_dir = sys.argv[2]
for s in m["sheets"]:
    md_rel = os.path.relpath(s["md"])
    csv_rel = os.path.relpath(s["csv"])
    print(f"XLSX-SHEET|slug={s['slug']}|rows={s['rows_total']}|md={md_rel}|csv={csv_rel}")
PY

rm "$SRC"

echo "XLSX-CONVERTED|in=$SRC|workbook=$SLUG|sheets=$SHEETS|orig=$ORIG_DIR/$BASE"
python3 - "$MANIFEST" <<'PY'
import json, os, sys
m = json.loads(sys.argv[1])
for s in m["sheets"]:
    print(f"XLSX-NEXT|{os.path.relpath(s['md'])}")
PY

bash "$SCRIPT_DIR/log-append.sh" xlsx "converted $BASE sheets=$SHEETS" >/dev/null 2>&1 || true
exit 0
```

Make executable:

```bash
chmod +x scripts/ingest-xlsx.sh
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/ingest_xlsx_test.bats`
Expected: every test PASSes.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-xlsx.sh tests/ingest_xlsx_test.bats
git commit -m "feat(xlsx): bash wrapper drives extractor, archives, logs"
```

---

## Task 9: `just ingest-xlsx` recipe

**Files:**
- Modify: `justfile`
- Test: `tests/ingest_xlsx_test.bats`

- [ ] **Step 1: Write the failing test**

Append to `tests/ingest_xlsx_test.bats`:

```bash
@test "just ingest-xlsx recipe wires up the wrapper" {
  command -v just >/dev/null 2>&1 || skip "just not installed"
  cp raw/inbox/batch/single-sheet.xlsx raw/inbox/interactive/single-sheet.xlsx
  cp -r "$BATS_TEST_DIRNAME/../justfile" .
  cp -r "$BATS_TEST_DIRNAME/../scripts" .
  run just ingest-xlsx raw/inbox/interactive/single-sheet.xlsx
  [ "$status" -eq 0 ]
  [[ "$output" == *"XLSX-CONVERTED|"* ]]
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bats tests/ingest_xlsx_test.bats -f "just ingest-xlsx"`
Expected: FAIL — recipe missing (`just` reports unknown recipe).

- [ ] **Step 3: Add the recipe**

Edit `justfile`. Locate the `# === ingest ===` section, find `ingest-with-agent`, and add immediately below it:

```make
ingest-xlsx path *flags:
    bash scripts/ingest-xlsx.sh {{path}} {{flags}}
```

- [ ] **Step 4: Run test to verify it passes**

Run: `bats tests/ingest_xlsx_test.bats -f "just ingest-xlsx"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add justfile tests/ingest_xlsx_test.bats
git commit -m "feat(xlsx): just ingest-xlsx recipe"
```

---

## Task 10: `watchdog.sh` extension dispatch

**Files:**
- Modify: `scripts/watchdog.sh`
- Create: `tests/watchdog_xlsx_test.bats`

Wire watchdog so a `.xlsx` / `.xls` / `.ods` drop triggers `ingest-xlsx.sh`; everything else still goes through `ingest.sh`.

- [ ] **Step 1: Write the failing integration test**

Create `tests/watchdog_xlsx_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  command -v python3 >/dev/null 2>&1 || skip "python3 not installed"
  python3 -c "import python_calamine" 2>/dev/null || skip "python-calamine not installed"
  WORK="$(mktemp -d)/repo"
  REPO="$BATS_TEST_DIRNAME/.."
  mkdir -p "$WORK/raw/inbox/batch" "$WORK/raw/processed/_originals" "$WORK/.awiki"
  cp -r "$REPO/scripts" "$WORK/scripts"
  cp "$REPO/tests/fixtures/xlsx/single-sheet.xlsx" "$WORK/raw/inbox/batch/single-sheet.xlsx"
  cd "$WORK"
}
teardown() { cd - >/dev/null; rm -rf "$WORK"; }

@test "watchdog extracts xlsx then ingests resulting md (one --once cycle)" {
  # First cycle: detect xlsx -> ingest-xlsx.sh -> writes md sibling, removes xlsx.
  run bash "$WORK/scripts/watchdog.sh" --once --catchup --backend=poll
  [ "$status" -eq 0 ]
  [[ "$output" == *"XLSX-CONVERTED|"* ]]
  [ -f raw/inbox/batch/single-sheet--sales.md ]
  [ ! -f raw/inbox/batch/single-sheet.xlsx ]

  # Second cycle: the new .md is picked up and ingested normally.
  run bash "$WORK/scripts/watchdog.sh" --once --catchup --backend=poll
  [ "$status" -eq 0 ]
  [[ "$output" == *"INGEST-OK|"* ]]
  [ -f raw/processed/batch/single-sheet--sales.md ]
}

@test "watchdog quarantines corrupt xlsx to _failed/" {
  cp "$BATS_TEST_DIRNAME/../tests/fixtures/xlsx/corrupt.xlsx" raw/inbox/batch/corrupt.xlsx
  run bash "$WORK/scripts/watchdog.sh" --once --catchup --backend=poll
  [ -f raw/inbox/batch/_failed/corrupt.xlsx ]
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bats tests/watchdog_xlsx_test.bats`
Expected: FAIL — watchdog still calls `ingest.sh` for xlsx (binary mv but no md produced).

- [ ] **Step 3: Implement dispatch_path**

Edit `scripts/watchdog.sh`. Locate `run_ingest` (around line 126). Immediately above `process_file()`, add:

```bash
dispatch_path() {
  local path="$1"
  case "${path,,}" in
    *.xlsx|*.xls|*.ods)
      bash "$SCRIPT_DIR/ingest-xlsx.sh" "$path"
      return $?
      ;;
  esac
  run_ingest "$path"
}
```

In `process_file()`, replace `run_ingest "$path"` (around line 188) with `dispatch_path "$path"`. Also propagate `--preview-rows` from `AWIKI_XLSX_PREVIEW_ROWS` automatically since `ingest-xlsx.sh` already reads that env.

- [ ] **Step 4: Run tests to verify they pass**

Run: `bats tests/watchdog_xlsx_test.bats`
Expected: both PASS.

Run the broader watchdog suite to ensure no regressions:

```bash
bats tests/watchdog_test.bats 2>/dev/null || bats tests/watchdog_*.bats
```

Expected: no failures.

- [ ] **Step 5: Commit**

```bash
git add scripts/watchdog.sh tests/watchdog_xlsx_test.bats
git commit -m "feat(watchdog): dispatch .xlsx/.xls/.ods to ingest-xlsx.sh"
```

---

## Task 11: Documentation pass

**Files:**
- Modify: `WIKI.md`

`WIKI.md` is the canonical schema doc. Per `CLAUDE.md`, every wiki operation must consult it. Add a one-paragraph note in §4.1 referencing the new flow.

- [ ] **Step 1: Inspect the current §4.1 block**

Run:
```bash
grep -n "^### 4\." WIKI.md
```

Expected: shows section anchors for 4.1, 4.2, etc.

- [ ] **Step 2: Edit §4.1**

In `WIKI.md`, immediately after the **Vision-aware ingest** paragraph, add:

```markdown
**Spreadsheet ingest:** `.xlsx` / `.xls` / `.ods` files dropped under `raw/inbox/` are pre-processed by `scripts/ingest-xlsx.sh` (manual: `just ingest-xlsx <path>`; auto: watchdog detects extension and runs the same script). The wrapper writes one `source` page per visible non-empty sheet (slug `<workbook>--<sheet>`) plus a per-sheet CSV under `raw/processed/_originals/<workbook>/`. The resulting markdown files re-enter the standard ingest flow (steps 2-9 above). Configure preview-row cap with `AWIKI_XLSX_PREVIEW_ROWS` (default 50). Requires `pip3 install python-calamine`.
```

- [ ] **Step 3: Verify lint passes**

Run: `bash scripts/lint.sh`
Expected: exit 0 or 1 (warnings only). Not 2.

- [ ] **Step 4: Commit**

```bash
git add WIKI.md
git commit -m "docs(wiki): note spreadsheet ingest path in §4.1"
```

---

## Task 12: End-to-end smoke + final test sweep

**Files:**
- Read-only: every test added so far.

- [ ] **Step 1: Manual smoke (interactive queue)**

```bash
cp tests/fixtures/xlsx/multi-sheet-with-hidden.xlsx raw/inbox/interactive/smoke.xlsx
just ingest-xlsx raw/inbox/interactive/smoke.xlsx
ls raw/inbox/interactive/smoke--*.md
ls raw/processed/_originals/smoke/
```

Expected: two `.md` files (Sales + Inventory), one `.xlsx` archive, two `.csv` files. No `smoke.xlsx` left in `raw/inbox/interactive/`.

Clean up: `git clean -fd raw/inbox/interactive/ raw/processed/_originals/smoke/ && rm -f raw/inbox/interactive/smoke--*.md`

- [ ] **Step 2: Run the full test suite**

```bash
bats tests/ingest_xlsx_test.bats
bats tests/watchdog_xlsx_test.bats
bats tests/check_deps_test.bats
bats tests/
```

Expected: every test PASSes (or skips gracefully when optional deps missing).

- [ ] **Step 3: Run lint**

```bash
just lint
```

Expected: exit 0 or 1 (no errors).

- [ ] **Step 4: Final commit (if any docs/log fix-ups)**

```bash
git status
```

If clean: nothing to commit. Otherwise commit with `chore(xlsx): post-merge cleanups`.

- [ ] **Step 5: Hand off**

Print completion summary:
- Files created: 8 (extractor, wrapper, generator, 5 fixtures, 2 bats files)
- Files modified: 4 (`watchdog.sh`, `check-deps.sh`, `check_deps_test.bats`, `justfile`, `WIKI.md`)
- Optional dep added: `python-calamine`

---

## Self-review notes

- **Spec coverage:** every spec section maps to a task — extractor (Tasks 3-7), wrapper (Task 8), justfile (Task 9), watchdog (Task 10), check-deps (Task 1), tests (interleaved), docs (Task 11), end-to-end (Task 12). Atomicity has its own dedicated task (7). Slug-collision-across-workbooks is documented in the spec but not given a dedicated test — it's covered implicitly by Task 10's quarantine assertion (a duplicate `.md` failing through to `_failed/` exercises the same path).
- **Placeholder scan:** every code block is concrete; no TODO / "implement later".
- **Type consistency:** `slugify`, `infer_headers`, `infer_type`, `extract`, `_resolve_collision`, `_md_table`, `_frontmatter` defined and referenced consistently. Wrapper exit codes match the spec table (1, 2, 3, 4, 5, 6).
- **Deviations from spec called out at top:** standalone Python unittest file dropped in favour of BATS-only testing (project convention).
