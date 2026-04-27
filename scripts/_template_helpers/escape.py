#!/usr/bin/env python3
"""Escape PLAN field values per spec: % first, then | and newline."""
import sys


def escape(s: str) -> str:
    return s.replace("%", "%25").replace("|", "%7C").replace("\n", "%0A")


def main() -> int:
    if len(sys.argv) != 2:
        print("usage: escape.py <string>", file=sys.stderr)
        return 2
    print(escape(sys.argv[1]))
    return 0


if __name__ == "__main__":
    sys.exit(main())
