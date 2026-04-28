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
    # Pre-process --slugify before argparse to handle values that start with '-'
    for i, arg in enumerate(argv):
        if arg == "--slugify" and i + 1 < len(argv):
            print(slugify(argv[i + 1]))
            return 0
        if arg.startswith("--slugify="):
            print(slugify(arg[len("--slugify="):]))
            return 0
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.slugify is not None:
        print(slugify(args.slugify))
        return 0
    parser.print_help(sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
