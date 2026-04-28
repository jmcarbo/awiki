#!/usr/bin/env python3
"""Bootstrap step body extractor + sha256 hasher.

Subcommands:
  list <bootstrap-md>        — print all step IDs in order
  hash <bootstrap-md> <id>   — print sha256 of step body (whitespace-normalized)
  body <bootstrap-md> <id>   — print raw step body (for replay)
"""
from __future__ import annotations

import hashlib
import re
import sys
from pathlib import Path

STEP_RE = re.compile(r"<!--\s*bootstrap-step:\s*([a-z0-9-]+)\s*-->")
HEADING_RE = re.compile(r"^###\s+", re.MULTILINE)


def parse(md: str) -> dict[str, str]:
    """Return {id: body} where body is text from the marker to next heading or EOF."""
    out: dict[str, str] = {}
    matches = list(STEP_RE.finditer(md))
    for i, m in enumerate(matches):
        sid = m.group(1)
        start = m.end()
        # End at next ### heading after this marker, or EOF.
        next_heading = HEADING_RE.search(md, start)
        end = next_heading.start() if next_heading else len(md)
        body = md[start:end]
        out[sid] = body
    return out


def list_ids(md_path: Path) -> int:
    md = md_path.read_text(encoding="utf-8")
    matches = STEP_RE.finditer(md)
    for m in matches:
        print(m.group(1))
    return 0


def normalize_whitespace(s: str) -> str:
    # Collapse runs of whitespace, strip leading/trailing.
    return re.sub(r"\s+", " ", s).strip()


def hash_step(md_path: Path, sid: str) -> int:
    md = md_path.read_text(encoding="utf-8")
    bodies = parse(md)
    if sid not in bodies:
        print(f"step not found: {sid}", file=sys.stderr)
        return 1
    norm = normalize_whitespace(bodies[sid])
    h = hashlib.sha256(norm.encode("utf-8")).hexdigest()
    print(f"sha256:{h}")
    return 0


def body_step(md_path: Path, sid: str) -> int:
    md = md_path.read_text(encoding="utf-8")
    bodies = parse(md)
    if sid not in bodies:
        print(f"step not found: {sid}", file=sys.stderr)
        return 1
    sys.stdout.write(bodies[sid])
    return 0


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: bootstrap_hash.py <list|hash|body> <bootstrap-md> [id]", file=sys.stderr)
        return 2
    subcmd = sys.argv[1]
    md_path = Path(sys.argv[2])
    if not md_path.is_file():
        print(f"not a file: {md_path}", file=sys.stderr)
        return 1
    if subcmd == "list":
        return list_ids(md_path)
    if subcmd in ("hash", "body"):
        if len(sys.argv) < 4:
            print(f"usage: bootstrap_hash.py {subcmd} <bootstrap-md> <id>", file=sys.stderr)
            return 2
        sid = sys.argv[3]
        return hash_step(md_path, sid) if subcmd == "hash" else body_step(md_path, sid)
    print(f"unknown subcmd: {subcmd}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
