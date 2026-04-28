#!/usr/bin/env python3
"""
Extract a workbook into per-sheet CSV + per-sheet markdown source pages.
Invoked by scripts/ingest-xlsx.sh. Pure: knows nothing about awiki paths
beyond what the bash wrapper passes in.
"""
from __future__ import annotations

import argparse
import json
import re
import sys


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
