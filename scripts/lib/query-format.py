#!/usr/bin/env python3
"""Convert DuckDB JSON rows on stdin to a markdown table on stdout."""
import argparse
import json
import sys


def md(rows: list[dict]) -> str:
    if not rows:
        return "(no rows)\n"
    cols = list(rows[0].keys())
    def cell(v):
        s = "" if v is None else str(v)
        return s.replace("|", "\\|")
    lines = [
        "| " + " | ".join(cols) + " |",
        "|" + "|".join(["---"] * len(cols)) + "|",
    ]
    for r in rows:
        lines.append("| " + " | ".join(cell(r.get(c)) for c in cols) + " |")
    return "\n".join(lines) + "\n"


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--format", choices=["md"], default="md")
    ap.parse_args()
    raw = sys.stdin.read().strip() or "[]"
    rows = json.loads(raw)
    sys.stdout.write(md(rows))
    return 0


if __name__ == "__main__":
    sys.exit(main())
