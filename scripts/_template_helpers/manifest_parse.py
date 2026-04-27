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

import sys
import tomllib
from pathlib import Path


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
    print(f"unknown subcmd: {subcmd}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
