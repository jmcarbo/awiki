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
import os
import shutil
import subprocess
import sys
import tempfile
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


def cmd_apply_attributes(args: argparse.Namespace) -> int:
    rel = args.rel
    base = args.old_tree / rel
    new = args.new_tree / rel
    cur = args.user_tree / rel
    if not new.is_file():
        return 0
    # 3-way merge first (or fall-back overwrite if no base / no cur).
    if not cur.is_file():
        cur.parent.mkdir(parents=True, exist_ok=True)
        shutil.copyfile(new, cur)
    elif base.is_file():
        rc = subprocess.call(
            ["git", "merge-file", "--diff3",
             "-L", "current", "-L", "base", "-L", "new",
             str(cur), str(base), str(new)]
        )
        if rc != 0:
            # Conflict; let orchestrator's marker check halt.
            return rc
    else:
        shutil.copyfile(new, cur)

    # Attr audit: compare OLD .gitattributes vs merged .gitattributes for each tracked file.
    # Run check-attr from a clean tmp git repo so workdir .gitattributes doesn't pollute.
    tracked = subprocess.run(
        ["git", "-C", str(args.user_tree), "ls-files"],
        capture_output=True, text=True
    ).stdout.splitlines()

    base_attrs = str(base) if base.is_file() else os.devnull
    new_attrs = str(cur)

    flipped = False
    with tempfile.TemporaryDirectory() as td:
        subprocess.run(["git", "-C", td, "init", "-q"], check=True)
        env = {**os.environ, "GIT_ATTR_NOSYSTEM": "1"}
        for p in tracked:
            old_out = subprocess.run(
                ["git", "-C", td, "-c", f"core.attributesfile={base_attrs}",
                 "check-attr", "-a", "--", p],
                capture_output=True, text=True, env=env,
            ).stdout
            new_out = subprocess.run(
                ["git", "-C", td, "-c", f"core.attributesfile={new_attrs}",
                 "check-attr", "-a", "--", p],
                capture_output=True, text=True, env=env,
            ).stdout
            if old_out != new_out:
                old_enc = "filter: git-crypt" in old_out
                new_enc = "filter: git-crypt" in new_out
                if old_enc != new_enc:
                    flipped = True
                    print(f"PLAN|attribute-change|{p}|filter-flipped", file=sys.stderr)
    if flipped and not args.accept_attribute_changes:
        print("Encryption pattern change detected. Re-run with --accept-attribute-changes.",
              file=sys.stderr)
        return 1
    return 0


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


def _add_attributes(sub):
    p = sub.add_parser("apply-attributes")
    p.add_argument("--old-tree", required=True, type=Path)
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--rel", required=True)
    p.add_argument("--accept-attribute-changes", action="store_true")


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    _add_overwrite(sub)
    _add_three_way(sub)
    _add_attributes(sub)

    args = parser.parse_args()
    dispatch = {
        "apply-overwrite": cmd_apply_overwrite,
        "apply-three-way": cmd_apply_three_way,
        "apply-attributes": cmd_apply_attributes,
    }
    return dispatch[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
