#!/usr/bin/env python3
"""Apply S3 normalization steps 2 (zero-width strip), 3 (hyphen variants),
4 (smart quotes) only inside the BEGIN..END region of a synthesis page.
Writes the modified file back in place. File-path argv only.
"""

from __future__ import annotations
import re
import sys
from pathlib import Path

ZW_RX = re.compile(r"[​‌‍﻿‎‏‪-‮]")
HYPHEN_RX = re.compile(r"[‐-—]")
SMART_RX = re.compile(r"[“”‘’]")

SMART = {"“": '"', "”": '"', "‘": "'", "’": "'"}


def fix_region(text: str) -> str:
    text = ZW_RX.sub("", text)
    text = HYPHEN_RX.sub("-", text)
    text = SMART_RX.sub(lambda m: SMART[m.group(0)], text)
    return text


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--"]
    if len(args) != 1:
        print("usage: lint-synth-fix-region.py <path>", file=sys.stderr)
        return 2
    p = Path(args[0])
    src = p.read_text(encoding="utf-8")
    out_lines: list[str] = []
    in_region = False
    for line in src.splitlines(keepends=True):
        if line.startswith("<!-- BEGIN GENERATED ") and line.rstrip().endswith("-->"):
            in_region = True
            out_lines.append(line)
            continue
        if line.rstrip() == "<!-- END GENERATED -->":
            in_region = False
            out_lines.append(line)
            continue
        out_lines.append(fix_region(line) if in_region else line)
    p.write_text("".join(out_lines), encoding="utf-8")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
