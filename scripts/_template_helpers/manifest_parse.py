#!/usr/bin/env python3
"""Manifest parser for template.manifest.toml.

Subcommands:
  load <path>             — emit shell-sourceable KEY=value
  resolve <path> <relpath> — print resolved strategy for relpath
  bootstrap-ids <path>    — print one ID per line in declared order
  dangerous-ids <path>    — print one ID per line
  has-glob-overlap <path> — exit 1 if same-strategy duplicate globs exist
"""
from __future__ import annotations

import fnmatch
import sys
import tomllib
from pathlib import Path
from typing import Tuple


STRATEGY_PRECEDENCE = [
    "template_only",
    "attributes_merge",
    "three_way",
    "overwrite",
    "preserve",
]


def load(path: Path) -> dict:
    with path.open("rb") as f:
        return tomllib.load(f)


def cmd_load(path: Path) -> int:
    m = load(path)
    print(f"schema_version={m.get('schema_version', 0)}")
    print(f"template_version={m.get('template_version', '')}")
    nfd = m.get("new_file_default", {}).get("strategy", "prompt")
    print(f"new_file_default={nfd}")
    return 0


def _glob_specificity(glob: str) -> Tuple[int, int, int, str]:
    """Return a sort key. Higher tuple = more specific."""
    star_idx = glob.find("*")
    literal_prefix_len = len(glob) if star_idx == -1 else star_idx
    segments = glob.count("/") + 1
    no_double_star = 0 if "**" in glob else 1
    # Negate glob string for lexicographic tie-break (later-declared wins
    # is enforced at iteration order; lex is final fallback).
    return (literal_prefix_len, segments, no_double_star, glob)


def _matches(glob: str, relpath: str) -> bool:
    # fnmatch handles * and ? but not **; expand ** to fnmatch's recursive form.
    # Python's fnmatch does not support **; emulate by removing path-segment boundary.
    if "**" in glob:
        # `a/**/c` matches `a/anything/here/c` AND `a/c`.
        # Replace `/**/` with `/` (zero segments) plus `/*/` patterns aren't easy.
        # Simpler: split on `**` and check prefix/suffix containment.
        parts = glob.split("**")
        if len(parts) == 2:
            prefix, suffix = parts
            prefix = prefix.rstrip("/")
            suffix = suffix.lstrip("/")
            if prefix and not (relpath == prefix or relpath.startswith(prefix + "/")):
                return False
            if suffix and not (relpath == suffix or relpath.endswith("/" + suffix) or fnmatch.fnmatch(relpath, "*" + suffix)):
                return False
            return True
        # Fallback: single ** at end
    return fnmatch.fnmatch(relpath, glob)


def resolve_strategy(manifest: dict, relpath: str) -> str:
    """Return resolved strategy for relpath. Most-specific glob wins; cross-strategy
    ties broken by STRATEGY_PRECEDENCE; same-strategy ties broken by later-declared."""
    strategies = manifest.get("strategies", {})
    candidates: list[Tuple[Tuple, str, int]] = []  # (specificity_key, strategy, declared_idx)

    # Walk in STRATEGY_PRECEDENCE order so that for same specificity the higher-precedence
    # strategy gets a higher idx.
    for strategy in STRATEGY_PRECEDENCE:
        globs = strategies.get(strategy, [])
        for idx, glob in enumerate(globs):
            if _matches(glob, relpath):
                candidates.append((_glob_specificity(glob), strategy, idx))

    if not candidates:
        nfd = manifest.get("new_file_default", {}).get("strategy", "prompt")
        return nfd

    # Sort: highest specificity first; ties broken by STRATEGY_PRECEDENCE position;
    # final fallback: later-declared (higher idx) wins.
    def _sort_key(item):
        spec, strat, idx = item
        precedence = STRATEGY_PRECEDENCE.index(strat)
        # Lower precedence index = higher priority. Negate for descending.
        return (spec, -precedence, idx)

    candidates.sort(key=_sort_key, reverse=True)
    return candidates[0][1]


def cmd_resolve(path: Path, relpath: str) -> int:
    m = load(path)
    print(resolve_strategy(m, relpath))
    return 0


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: manifest_parse.py <subcmd> <path> [args...]", file=sys.stderr)
        return 2
    subcmd, path_arg = sys.argv[1], Path(sys.argv[2])
    if not path_arg.is_file():
        print(f"manifest not found: {path_arg}", file=sys.stderr)
        return 1
    if subcmd == "load":
        return cmd_load(path_arg)
    if subcmd == "resolve":
        if len(sys.argv) < 4:
            print("usage: resolve <manifest> <relpath>", file=sys.stderr)
            return 2
        return cmd_resolve(path_arg, sys.argv[3])
    print(f"unknown subcmd: {subcmd}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
