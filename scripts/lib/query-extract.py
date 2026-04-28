#!/usr/bin/env python3
"""Extract a dataset fence body from content/datasets/<slug>.md to a file.

Reads `format:` from frontmatter, finds the matching ``` fence, writes its
body verbatim (with a trailing newline if missing) to --out.
"""
import argparse
import re
import sys
from pathlib import Path


def _frontmatter(text: str) -> dict[str, str]:
    m = re.match(r"^---\s*\n(.*?)\n---\s*\n", text, re.S)
    if not m:
        return {}
    out: dict[str, str] = {}
    for line in m.group(1).splitlines():
        kv = re.match(r"^([a-z0-9_]+):\s*(.*)$", line.strip())
        if kv:
            out[kv.group(1)] = kv.group(2).strip().strip('"').strip("'")
    return out


def _fence_body(text: str, fmt: str) -> str | None:
    pat = re.compile(rf"```{re.escape(fmt)}\s*\n(.*?)\n```", re.S)
    m = pat.search(text)
    return m.group(1) if m else None


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--slug", required=True)
    ap.add_argument("--datasets-dir", required=True)
    ap.add_argument("--out", required=True)
    args = ap.parse_args()

    page = Path(args.datasets_dir) / f"{args.slug}.md"
    if not page.is_file():
        print(f"QUERY|ERROR|dataset not found: {args.slug}", file=sys.stderr)
        return 3

    text = page.read_text(encoding="utf-8")
    fm = _frontmatter(text)
    fmt = fm.get("format")
    if not fmt:
        print(f"QUERY|ERROR|no format in frontmatter: {page}", file=sys.stderr)
        return 4

    body = _fence_body(text, fmt)
    if body is None:
        print(f"QUERY|ERROR|no ```{fmt} fence in {page}", file=sys.stderr)
        return 4

    out = Path(args.out)
    out.parent.mkdir(parents=True, exist_ok=True)
    if not body.endswith("\n"):
        body += "\n"
    out.write_text(body, encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main())
