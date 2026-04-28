#!/usr/bin/env python3
"""Reject non-deterministic SQL when materializing.

Reads SQL on stdin. Exits 0 if deterministic, 6 with a stderr message otherwise.

Rules:
  - Banned tokens: NOW(), CURRENT_TIMESTAMP, CURRENT_DATE, CURRENT_TIME,
    RANDOM(), UUID(), gen_random_uuid().
  - Top-level statement must contain ORDER BY (case-insensitive).
  - Inner ORDER BY (inside parentheses) does not count.
"""
import re
import sys

BANNED = [
    r"\bNOW\s*\(",
    r"\bCURRENT_TIMESTAMP\b",
    r"\bCURRENT_DATE\b",
    r"\bCURRENT_TIME\b",
    r"\bRANDOM\s*\(",
    r"\bUUID\s*\(",
    r"\bGEN_RANDOM_UUID\s*\(",
]


def _strip_strings_and_comments(sql: str) -> str:
    sql = re.sub(r"--[^\n]*", "", sql)
    sql = re.sub(r"/\*.*?\*/", "", sql, flags=re.S)
    sql = re.sub(r"'(?:''|[^'])*'", "''", sql)
    return sql


def _toplevel(sql: str) -> str:
    """Return SQL with all parenthesised groups erased (depth>0)."""
    out: list[str] = []
    depth = 0
    for ch in sql:
        if ch == "(":
            depth += 1
            continue
        if ch == ")":
            depth = max(0, depth - 1)
            continue
        if depth == 0:
            out.append(ch)
    return "".join(out)


def main() -> int:
    raw = sys.stdin.read()
    sql = _strip_strings_and_comments(raw)
    for pat in BANNED:
        m = re.search(pat, sql, re.I)
        if m:
            print(
                f"QUERY|ERROR|non-deterministic token: {m.group(0).strip()}",
                file=sys.stderr,
            )
            return 6
    top = _toplevel(sql)
    if not re.search(r"\bORDER\s+BY\b", top, re.I):
        print(
            "QUERY|ERROR|materialized SQL must have ORDER BY on the outer SELECT",
            file=sys.stderr,
        )
        return 6
    return 0


if __name__ == "__main__":
    sys.exit(main())
