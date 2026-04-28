#!/usr/bin/env python3
"""Parse SQL on stdin, emit one referenced dataset slug per stdout line.

Heuristic: scan FROM/JOIN clauses; strip CTE names, quoted identifiers,
and `main.` schema prefix. Caller validates each slug exists on disk.
"""
import re
import sys

SLUG_RE = re.compile(r"^[a-z0-9][a-z0-9-]*$")


def _strip_strings_and_comments(sql: str) -> str:
    # Remove -- line comments and /* */ block comments and 'string literals'.
    sql = re.sub(r"--[^\n]*", "", sql)
    sql = re.sub(r"/\*.*?\*/", "", sql, flags=re.S)
    sql = re.sub(r"'(?:''|[^'])*'", "''", sql)
    return sql


def _cte_names(sql: str) -> set[str]:
    names: set[str] = set()
    for m in re.finditer(r"\bWITH\b(.+?)\bSELECT\b", sql, re.I | re.S):
        block = m.group(1)
        # Each CTE is `name AS ( ... )`; comma-separated at depth 0.
        for cte in re.finditer(r"([a-zA-Z_][\w]*)\s+AS\s*\(", block, re.I):
            names.add(cte.group(1).lower())
    return names


def _refs(sql: str) -> list[str]:
    sql = _strip_strings_and_comments(sql)
    skip = _cte_names(sql)
    found: list[str] = []
    for m in re.finditer(
        r"\b(?:FROM|JOIN)\s+(?:(?:main|public)\.)?\"?([a-zA-Z_][\w-]*)\"?",
        sql,
        re.I,
    ):
        name = m.group(1).lower()
        if name in skip:
            continue
        if not SLUG_RE.match(name):
            continue
        if name in found:
            continue
        found.append(name)
    return sorted(found)


def main() -> int:
    sql = sys.stdin.read()
    for slug in _refs(sql):
        print(slug)
    return 0


if __name__ == "__main__":
    sys.exit(main())
