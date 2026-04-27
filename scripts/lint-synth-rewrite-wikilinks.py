#!/usr/bin/env python3
"""Read a quote from a file path; rewrite [[slug]] wikilinks to their
frontmatter title (using <wiki-root>/.awiki/maps/slug-to-path.tsv); write to
stdout. File-path argv only — no interpolation of quote text into argv or -c.

Usage: lint-synth-rewrite-wikilinks.py [--map=<map-path>] <quote-path>
If --map is omitted, defaults to ./.awiki/maps/slug-to-path.tsv.
"""

from __future__ import annotations
import pathlib
import re
import sys


def title_for(slug: str, map_path: pathlib.Path) -> str:
    if not map_path.is_file():
        return slug
    map_dir = map_path.parent.parent.parent  # .awiki/maps/foo.tsv → wiki-root
    for line in map_path.read_text(encoding="utf-8").splitlines():
        parts = line.split("\t")
        if len(parts) >= 2 and parts[0] == slug:
            page = pathlib.Path(parts[1])
            if not page.is_absolute():
                page = map_dir / page
            if page.is_file():
                for ln in page.read_text(encoding="utf-8").splitlines():
                    if ln.startswith("title:"):
                        return ln.split(":", 1)[1].strip().strip('"')
    return slug


def main(argv: list[str]) -> int:
    args = [a for a in argv[1:] if a != "--"]
    map_path = pathlib.Path(".awiki/maps/slug-to-path.tsv")
    rest: list[str] = []
    for a in args:
        if a.startswith("--map="):
            map_path = pathlib.Path(a[len("--map="):])
        else:
            rest.append(a)
    if len(rest) != 1:
        print("usage: lint-synth-rewrite-wikilinks.py [--map=<path>] <quote-path>",
              file=sys.stderr)
        return 2
    text = pathlib.Path(rest[0]).read_text(encoding="utf-8", errors="replace")

    def repl(match: re.Match[str]) -> str:
        raw = match.group(1)
        slug, _, disp = raw.partition("|")
        return disp or title_for(slug, map_path)

    sys.stdout.write(re.sub(r"\[\[([^\]]+)\]\]", repl, text))
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv))
