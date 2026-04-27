#!/usr/bin/env python3
"""Phase 3 Commit A sync operations.

Subcommands:
  apply-overwrite     --new-tree <dir> --user-tree <dir> --rel <relpath>
  apply-three-way     --old-tree <dir> --new-tree <dir> --user-tree <dir> --rel <relpath>
  apply-attributes    --old-tree <dir> --new-tree <dir> --user-tree <dir> --rel <relpath>
                      [--accept-attribute-changes]
  apply-new-file      --new-tree <dir> --user-tree <dir> --rel <relpath>
                      --decision <overwrite|skip|mark-as-user-deleted>
  apply-deletion      --user-tree <dir> --rel <relpath> --decision <remove|preserve-local>
  has-conflict-markers --tree <dir> --paths <relpath>...
"""
from __future__ import annotations

import argparse
import shutil
import subprocess
import sys
from pathlib import Path


def cmd_apply_overwrite(args: argparse.Namespace) -> int:
    src = args.new_tree / args.rel
    dst = args.user_tree / args.rel
    if not src.is_file():
        print(f"source not found: {src}", file=sys.stderr)
        return 1
    dst.parent.mkdir(parents=True, exist_ok=True)
    shutil.copyfile(src, dst)
    # Preserve mode bits (executable).
    dst.chmod(src.stat().st_mode)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_o = sub.add_parser("apply-overwrite")
    p_o.add_argument("--new-tree", required=True, type=Path)
    p_o.add_argument("--user-tree", required=True, type=Path)
    p_o.add_argument("--rel", required=True)

    args = parser.parse_args()
    dispatch = {
        "apply-overwrite": cmd_apply_overwrite,
    }
    return dispatch[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
