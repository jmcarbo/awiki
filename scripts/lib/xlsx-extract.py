#!/usr/bin/env python3
"""
Extract a workbook into per-sheet CSV + per-sheet markdown source pages.
Invoked by scripts/ingest-xlsx.sh. Pure: knows nothing about awiki paths
beyond what the bash wrapper passes in.
"""
from __future__ import annotations

import argparse
import csv
import json
import math
import os
import re
import shutil
import sys
import tempfile
from datetime import date
from pathlib import Path


def slugify(text: str) -> str:
    """Lowercase, collapse non [a-z0-9-] to '-', strip leading/trailing '-'.

    Returns '' for empty input or input containing only non-alphanumeric
    characters (e.g. '!!!', '---', '中文'). Callers that turn the result
    into a path or slug component must validate non-empty before use.
    """
    s = text.lower()
    s = re.sub(r"[^a-z0-9-]+", "-", s)
    s = re.sub(r"-+", "-", s)
    return s.strip("-")


def infer_headers(first_row: list) -> list[str]:
    """Return first_row when every cell is a non-empty string; else col_1..col_N."""
    if first_row and all(isinstance(c, str) and c.strip() for c in first_row):
        return [c for c in first_row]
    return [f"col_{i + 1}" for i in range(len(first_row))]


def infer_type(samples: list) -> str:
    """One of text|number|date|bool|mixed based on a column sample.

    None values are skipped so an all-None sample returns "text" rather
    than "date" via the generic else-branch. Callers may also pre-filter,
    but the helper is safe either way.
    """
    seen: set[str] = set()
    for v in samples:
        if v is None:
            continue
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
    if not seen:
        return "text"
    return next(iter(seen))


def _stringify_cell(v) -> str:
    """Render a calamine cell value as a CSV/markdown-safe string.

    Excel integers come back as float (calamine's default), so finite
    floats whose value is an exact integer are normalised back to int
    rendering: 100.0 -> "100". NaN / +-inf retain their float repr
    rather than raising in the int(v) conversion.
    """
    if v is None:
        return ""
    if isinstance(v, bool):
        return "true" if v else "false"
    if isinstance(v, float) and math.isfinite(v) and v == int(v):
        return str(int(v))
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
    # Free-form user strings (title, workbook filename, sheet name) go through
    # json.dumps so embedded quotes / colons / control chars do not produce
    # invalid YAML. JSON strings are valid YAML scalars.
    cols_json = json.dumps(columns)
    return (
        "---\n"
        f"title: {json.dumps(title)}\n"
        f"date: {today}\n"
        f"last_updated: {today}\n"
        "type: source\n"
        "tags: [xlsx]\n"
        "aliases: []\n"
        "sources: []\n"
        f"workbook: {json.dumps(workbook)}\n"
        f"sheet: {json.dumps(sheet_name)}\n"
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

    # Create staging dirs under each destination so os.replace is same-filesystem.
    stage_md = Path(tempfile.mkdtemp(prefix=".xlsx-extract-", dir=str(out_dir)))
    stage_csv = Path(tempfile.mkdtemp(prefix=".xlsx-extract-", dir=str(csv_dir)))

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

        csv_path = stage_csv / f"{sheet_slug}.csv"
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
        md_path = stage_md / f"{sheet_slug}.md"
        md_path.write_text(fm + body, encoding="utf-8")

        sheets_out.append({
            "name": sheet_name,
            "slug": sheet_slug,
            "rows_total": rows_total,
            "rows_preview": rows_preview,
            "csv": str(csv_path),
            "md": str(md_path),
        })

    # Atomic rename phase: move staged files into final destinations.
    # AWIKI_XLSX_FORCE_FAIL_AFTER=N injects a failure AFTER the Nth sheet
    # has been renamed (1-indexed) so the rollback path that unlinks
    # already-renamed files is exercised by tests.
    force_fail_raw = os.environ.get("AWIKI_XLSX_FORCE_FAIL_AFTER", "")
    force_fail_after = int(force_fail_raw) if force_fail_raw.isdigit() else None
    renamed_md: list[Path] = []
    renamed_csv: list[Path] = []
    try:
        for idx, s in enumerate(sheets_out):
            staged_md = Path(s["md"])
            staged_csv = Path(s["csv"])
            final_md = out_dir / staged_md.name
            final_csv = csv_dir / staged_csv.name
            os.replace(staged_md, final_md)
            renamed_md.append(final_md)
            os.replace(staged_csv, final_csv)
            renamed_csv.append(final_csv)
            s["md"] = str(final_md)
            s["csv"] = str(final_csv)
            if force_fail_after is not None and idx + 1 == force_fail_after:
                raise IOError(
                    f"forced failure for atomicity test (after sheet {idx + 1})"
                )

    except Exception as e:
        # Order: unlink finals first (we know exactly which paths committed),
        # then rmtree staging (untracked but bounded by mkdtemp prefix).
        for f in renamed_md:
            try:
                f.unlink()
            except OSError:
                pass
        for f in renamed_csv:
            try:
                f.unlink()
            except OSError:
                pass
        shutil.rmtree(stage_md, ignore_errors=True)
        shutil.rmtree(stage_csv, ignore_errors=True)
        # Escape pipes / newlines in {e} so the XLSX-ERROR line stays
        # parseable when split on '|'.
        err_msg = str(e).replace("|", r"\|").replace("\n", " ")
        print(f"XLSX-ERROR|reason=io|err={err_msg}", file=sys.stderr)
        return 4

    finally:
        shutil.rmtree(stage_md, ignore_errors=True)
        shutil.rmtree(stage_csv, ignore_errors=True)

    manifest = {"workbook_slug": args.slug_prefix, "sheets": sheets_out}
    print(json.dumps(manifest))
    return 0


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
    p.add_argument("--infer-headers", metavar="JSON", help="Print headers inferred from a JSON list and exit")
    p.add_argument("--infer-type", metavar="JSON", help="Print type inferred from a JSON sample list and exit")
    return p


def main(argv: list[str]) -> int:
    # --slugify is intercepted before argparse because argparse rejects
    # values that begin with '--' (e.g. "---abc---" -> "argument --slugify:
    # expected one argument"). The build_parser() declaration of --slugify
    # is kept only so it appears in --help output; the runtime never hits
    # the argparse branch for --slugify.
    for i, arg in enumerate(argv):
        if arg == "--":
            break
        if arg == "--slugify" and i + 1 < len(argv):
            print(slugify(argv[i + 1]))
            return 0
        if arg.startswith("--slugify="):
            print(slugify(arg[len("--slugify="):]))
            return 0
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.in_path:
        return extract(args)
    if args.infer_headers is not None:
        print(json.dumps(infer_headers(json.loads(args.infer_headers))))
        return 0
    if args.infer_type is not None:
        print(infer_type(json.loads(args.infer_type)))
        return 0
    parser.print_help(sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
