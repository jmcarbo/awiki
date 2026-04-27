#!/usr/bin/env python3
"""Preflight checks for template-update orchestrator.

Subcommands:
  check-tree             — staged/unstaged/untracked + dirty submodules
  check-branch --expected <name>
  check-pending-prompts  — halts if .awiki/pending-prompts/*.md present
  check-encryption --manifest <path>   — halts if encrypted+locked + manifest needs merge
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


def run(cmd: list[str]) -> tuple[int, str, str]:
    try:
        r = subprocess.run(cmd, capture_output=True, text=True)
    except FileNotFoundError as e:
        return 127, "", str(e)
    return r.returncode, r.stdout, r.stderr


def check_tree() -> int:
    # Staged or unstaged tracked changes.
    rc, _, _ = run(["git", "diff-index", "--quiet", "HEAD", "--"])
    if rc != 0:
        print("dirty tree: staged or unstaged tracked changes (modified)", file=sys.stderr)
        return 1
    # Untracked files not gitignored.
    rc, out, _ = run(["git", "ls-files", "--others", "--exclude-standard"])
    if rc == 0 and out.strip():
        print(f"dirty tree: untracked files\n{out}", file=sys.stderr)
        return 1
    # Submodule dirty.
    rc, out, _ = run(["git", "submodule", "status"])
    if rc == 0:
        for line in out.splitlines():
            if line.startswith("+") or line.startswith("-"):
                print(f"dirty tree: submodule {line}", file=sys.stderr)
                return 1
    return 0


def check_branch(expected: str) -> int:
    rc, out, _ = run(["git", "rev-parse", "--abbrev-ref", "HEAD"])
    if rc != 0:
        print("not in a git repo", file=sys.stderr)
        return 2
    cur = out.strip()
    if cur.startswith("awiki-template-update/"):
        print(f"halt: on update branch {cur}; use --continue or --abort", file=sys.stderr)
        return 1
    if cur != expected:
        print(f"halt: on branch {cur}, expected {expected}", file=sys.stderr)
        return 1
    return 0


def check_pending_prompts() -> int:
    pp = Path(".awiki/pending-prompts")
    if not pp.is_dir():
        return 0
    leftovers = sorted(p.name for p in pp.glob("*.md"))
    if not leftovers:
        return 0
    print(
        "halt: pending LLM migrations from previous cycle:\n  "
        + "\n  ".join(leftovers)
        + "\nRun agent to complete or delete prompts before next update.",
        file=sys.stderr,
    )
    return 1


def check_encryption(manifest_path: Path) -> int:
    # Try git-crypt status.
    rc, out, err = run(["git-crypt", "status", "-e"])
    if rc == 127 or "command not found" in err.lower():
        # git-crypt not installed; skip silently.
        return 0
    if not out.strip():
        return 0  # no encrypted paths
    # Encrypted paths present. Check if locked.
    rc, status_out, _ = run(["git-crypt", "status"])
    locked = "locked" in status_out.lower()
    if not locked:
        return 0
    # Check if any encrypted path matches three_way / attributes_merge in manifest.
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    import manifest_parse  # noqa: E402
    m = manifest_parse.load(manifest_path)
    enc_paths = [line.strip() for line in out.splitlines() if line.strip()]
    for p in enc_paths:
        s = manifest_parse.resolve_strategy(m, p)
        if s in ("three_way", "attributes_merge"):
            print(
                f"halt: encrypted path {p} would require merge but checkout is locked.\n"
                "Run `git-crypt unlock` first.",
                file=sys.stderr,
            )
            return 1
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("check-tree")
    p_b = sub.add_parser("check-branch")
    p_b.add_argument("--expected", required=True)
    sub.add_parser("check-pending-prompts")
    p_e = sub.add_parser("check-encryption")
    p_e.add_argument("--manifest", required=True, type=Path)
    args = parser.parse_args()
    if args.cmd == "check-tree":
        return check_tree()
    if args.cmd == "check-branch":
        return check_branch(args.expected)
    if args.cmd == "check-pending-prompts":
        return check_pending_prompts()
    if args.cmd == "check-encryption":
        return check_encryption(args.manifest)
    return 2


if __name__ == "__main__":
    sys.exit(main())
