#!/usr/bin/env python3
"""S3 fuzzy-match helper.

Reads a source file (markdown) and a quote file. Splits the source into
paragraphs, applies the same normalization the shell-side lint applies,
runs difflib.get_close_matches against the normalized quote, and prints
the top match (raw paragraph form) to stdout.

Inputs are file paths only. Source bodies are user-supplied (PDFs, web
clippings) and may contain adversarial content; therefore source text
NEVER appears in argv or in a -c expression. The shell side calls this
file as: python3 scripts/lint-synth-fuzzy.py <source-path> <quote-path>
"""

from __future__ import annotations

import re
import sys
import unicodedata
from difflib import get_close_matches
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
    text = text.replace(" ", " ")
    text = re.sub(r"\s+", " ", text).strip()
    return text


def strip_frontmatter(text: str) -> str:
    if not text.startswith("---\n"):
        return text
    end = text.find("\n---\n", 4)
    if end == -1:
        return text
    return text[end + len("\n---\n"):]


def split_paragraphs(text: str) -> list[str]:
    return [p.strip() for p in re.split(r"\n\s*\n", text) if p.strip()]


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--"]
    if len(args) != 2:
        print("usage: lint-synth-fuzzy.py <source-path> <quote-path>", file=sys.stderr)
        return 2

    source_path = Path(args[0])
    quote_path = Path(args[1])

    if not source_path.is_file():
        print(f"source not found: {source_path}", file=sys.stderr)
        return 1
    if not quote_path.is_file():
        print(f"quote not found: {quote_path}", file=sys.stderr)
        return 1

    source = strip_frontmatter(source_path.read_text(encoding="utf-8", errors="replace"))
    quote = quote_path.read_text(encoding="utf-8", errors="replace")

    paragraphs = split_paragraphs(source)
    if not paragraphs:
        return 0

    norm_quote = normalize(quote)
    norm_paragraphs = [normalize(p) for p in paragraphs]

    matches = get_close_matches(norm_quote, norm_paragraphs, n=1, cutoff=0.55)
    if not matches:
        return 0

    idx = norm_paragraphs.index(matches[0])
    suggestion = paragraphs[idx]
    if len(suggestion) > 200:
        suggestion = suggestion[:200].rsplit(" ", 1)[0] + "..."
    print(suggestion)
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
