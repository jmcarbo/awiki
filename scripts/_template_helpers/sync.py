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


def cmd_apply_three_way(args: argparse.Namespace) -> int:
    base = args.old_tree / args.rel
    new = args.new_tree / args.rel
    cur = args.user_tree / args.rel
    if not new.is_file():
        return 0  # not in new tree — nothing to do
    if not cur.is_file():
        # User deleted the file; treat as new-file-with-base.
        cur.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(new, cur)
        return 0
    if not base.is_file():
        # No ancestor — fall back to overwrite-with-warning.
        shutil.copyfile(new, cur)
        print(f"warn: no ancestor for {args.rel}; overwrote", file=sys.stderr)
        return 0
    rc = subprocess.call(
        ["git", "merge-file", "--diff3",
         "-L", "current", "-L", "base", "-L", "new",
         str(cur), str(base), str(new)]
    )
    return rc


def _add_overwrite(sub):
    p = sub.add_parser("apply-overwrite")
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)


def _add_three_way(sub):
    p = sub.add_parser("apply-three-way")
    p.add_argument("--old-tree", required=True, type=Path)
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    _add_overwrite(sub)
    _add_three_way(sub)

    args = parser.parse_args()
    dispatch = {
        "apply-overwrite": cmd_apply_overwrite,
        "apply-three-way": cmd_apply_three_way,
    }
    return dispatch[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
