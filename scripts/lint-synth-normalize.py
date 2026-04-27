#!/usr/bin/env python3
"""Apply S3 normalization to a file's contents and write to stdout.

Same normalization function as lint-synth-fuzzy.py. Kept as a separate
script so the shell side can pipe through it without spawning python -c.
"""

from __future__ import annotations
import re
import sys
import unicodedata
from pathlib import Path

ZERO_WIDTH = "​‌‍﻿‎‏‪‫‬‭‮"
HYPHENS = {"‐": "-", "‑": "-", "‒": "-", "–": "-", "—": "-"}
SMART_QUOTES = {"“": '"', "”": '"', "‘": "'", "’": "'"}


def normalize(text: str) -> str:
    text = unicodedata.normalize("NFC", text)
    text = text.translate(str.maketrans("", "", ZERO_WIDTH))
    for src, dst in HYPHENS.items():
        text = text.replace(src, dst)
    for src, dst in SMART_QUOTES.items():
        text = text.replace(src, dst)
    text = text.replace(" ", " ")
    text = re.sub(r"\s+", " ", text).strip()
    return text


def strip_frontmatter(text: str) -> str:
    if not text.startswith("---\n"):
        return text
    end = text.find("\n---\n", 4)
    if end == -1:
        return text
    return text[end + len("\n---\n"):]


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--"]
    if len(args) != 2 or args[0] not in {"--source", "--quote"}:
        print("usage: lint-synth-normalize.py {--source|--quote} <path>", file=sys.stderr)
        return 2
    mode, path = args[0], Path(args[1])
    text = path.read_text(encoding="utf-8", errors="replace")
    if mode == "--source":
        text = strip_frontmatter(text)
    sys.stdout.write(normalize(text))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
