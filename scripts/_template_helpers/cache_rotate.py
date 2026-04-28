#!/usr/bin/env python3
"""Cache rotation: keep <current> + immediate previous (by mtime), delete older."""
from __future__ import annotations

import argparse
import shutil
import sys
from pathlib import Path


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--cache-dir", required=True, type=Path)
    p.add_argument("--current", required=True)
    args = p.parse_args()

    if not args.cache_dir.is_dir():
        return 0
    sha_dirs = []
    for entry in args.cache_dir.iterdir():
        if entry.name.startswith("_"):
            continue
        if not entry.is_dir():
            continue
        sha_dirs.append(entry)

    # Sort by mtime descending.
    sha_dirs.sort(key=lambda d: d.stat().st_mtime, reverse=True)

    keep = {args.current}
    # Add up to 1 most recent that isn't current.
    for d in sha_dirs:
        if d.name not in keep and len(keep) < 2:
            keep.add(d.name)
            break

    for d in sha_dirs:
        if d.name not in keep:
            shutil.rmtree(d, ignore_errors=True)
    return 0


if __name__ == "__main__":
    sys.exit(main())
