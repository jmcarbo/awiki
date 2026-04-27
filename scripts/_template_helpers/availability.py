#!/usr/bin/env python3
"""Update-availability check.

--check: read .awiki/template-cache/_check-stamp; if older than 7 days OR missing,
         run `git ls-remote <repo> <ref>` (cached for 7 days via _check-stamp mtime).
         If new commits available, emit LINT|info|template|...

Suppressed by AWIKI_NO_TEMPLATE_CHECK=1 or .awiki/config no_template_check=true.
Network failures are silent (return 0).
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--root", required=True, type=Path)
    p.add_argument("--check", action="store_true")
    args = p.parse_args()
    if not args.check:
        return 0

    if os.environ.get("AWIKI_NO_TEMPLATE_CHECK") == "1":
        return 0

    pj = args.root / ".awiki" / "template.json"
    if not pj.is_file():
        return 0
    try:
        d = json.loads(pj.read_text(encoding="utf-8"))
    except Exception:
        return 0

    config_path = args.root / ".awiki" / "config"
    if config_path.is_file():
        for line in config_path.read_text(encoding="utf-8").splitlines():
            if line.strip() == "no_template_check=true":
                return 0

    cache_dir = args.root / ".awiki" / "template-cache"
    cache_dir.mkdir(parents=True, exist_ok=True)
    stamp = cache_dir / "_check-stamp"
    cutoff = datetime.now(timezone.utc) - timedelta(days=7)
    if stamp.is_file():
        mtime = datetime.fromtimestamp(stamp.stat().st_mtime, tz=timezone.utc)
        if mtime > cutoff:
            return 0  # fresh

    repo = d.get("repo", "")
    ref = d.get("ref", "main")
    pinned = d.get("commit", "")
    if not repo:
        return 0
    try:
        result = subprocess.run(
            ["git", "ls-remote", repo, ref],
            capture_output=True, text=True, timeout=10,
        )
    except Exception:
        return 0  # network/timeout/missing-git; skip silently
    if result.returncode != 0:
        return 0
    line = result.stdout.split("\n")[0].strip()
    if not line:
        return 0
    upstream = line.split()[0]

    try:
        stamp.touch()
    except Exception:
        pass

    if upstream != pinned:
        print(
            f"LINT|info|template|template updates available since {pinned[:12]}. "
            "Run `just template-status` to review."
        )

    return 0


if __name__ == "__main__":
    sys.exit(main())
