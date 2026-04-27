#!/usr/bin/env python3
"""Bootstrap step replay helper.

Subcommands:
  classify --bootstrap <md> --manifest <toml> --id <id>
    -> prints "ok" | "dangerous" | "missing"
       Exit 0 for ok / dangerous; exit 1 for missing.
  body --bootstrap <md> --id <id>
    -> prints raw body (for orchestrator to surface to user).
"""
from __future__ import annotations

import argparse
import sys
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import bootstrap_hash  # noqa: E402
import manifest_parse  # noqa: E402


def cmd_classify(args: argparse.Namespace) -> int:
    md = args.bootstrap.read_text(encoding="utf-8")
    bodies = bootstrap_hash.parse(md)
    if args.id not in bodies:
        print("missing")
        return 1
    m = manifest_parse.load(args.manifest)
    dangerous = m.get("bootstrap", {}).get("dangerous", {}).get("ids", [])
    if args.id in dangerous:
        print("dangerous")
        return 0
    print("ok")
    return 0


def cmd_body(args: argparse.Namespace) -> int:
    md = args.bootstrap.read_text(encoding="utf-8")
    bodies = bootstrap_hash.parse(md)
    if args.id not in bodies:
        print(f"missing step: {args.id}", file=sys.stderr)
        return 1
    sys.stdout.write(bodies[args.id])
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)
    p_c = sub.add_parser("classify")
    p_c.add_argument("--bootstrap", required=True, type=Path)
    p_c.add_argument("--manifest", required=True, type=Path)
    p_c.add_argument("--id", required=True)
    p_b = sub.add_parser("body")
    p_b.add_argument("--bootstrap", required=True, type=Path)
    p_b.add_argument("--id", required=True)
    args = parser.parse_args()
    return {"classify": cmd_classify, "body": cmd_body}[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
