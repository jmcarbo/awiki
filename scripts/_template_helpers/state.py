#!/usr/bin/env python3
"""Orchestrator state-file reader/writer.

Subcommands:
  init <path> --commit-old <sha> --commit-new <sha> --branch <name>
  set-phase <path> --phase <name> --status <started|committed>
  set-last-completed <path> <sha>
  add-migration-pending <path> --id <id> --status <applied|skipped> [--reason <r>]
  add-bootstrap-pending <path> --id <id> --status <applied|skipped> [--reason <r>] [--content-hash <h>]
  add-deletion-decision <path> --rel <relpath> --decision <remove|preserve-local>
  add-deleted-pending <path> --rel <relpath> [--reason <r>]
  get <path> <field>
  validate <path>
"""
from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

REQUIRED_FIELDS = (
    "phase", "status", "commit_old", "commit_new", "branch", "started_at",
    "last_completed_commit", "applied_migrations_pending",
    "bootstrap_steps_pending", "deletions_user_decisions", "deleted_pending",
)


def load(path: Path) -> dict:
    if not path.is_file():
        return {}
    with path.open("r") as f:
        return json.load(f)


def save(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def cmd_init(args: argparse.Namespace) -> int:
    data = {
        "phase": "fetch",
        "status": "committed",
        "commit_old": args.commit_old,
        "commit_new": args.commit_new,
        "branch": args.branch,
        "started_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "last_completed_commit": None,
        "applied_migrations_pending": [],
        "bootstrap_steps_pending": [],
        "deletions_user_decisions": {},
        "deleted_pending": [],
    }
    save(Path(args.path), data)
    return 0


def cmd_validate(args: argparse.Namespace) -> int:
    p = Path(args.path)
    if not p.is_file():
        print(f"state file not found: {p}", file=sys.stderr)
        return 1
    d = load(p)
    missing = set(REQUIRED_FIELDS) - set(d.keys())
    if missing:
        print(f"missing fields: {sorted(missing)}", file=sys.stderr)
        return 1
    if d.get("status") not in ("started", "committed"):
        print(f"invalid status: {d.get('status')}", file=sys.stderr)
        return 1
    return 0


def cmd_set_phase(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    d["phase"] = args.phase
    d["status"] = args.status
    save(p, d)
    return 0


def cmd_set_last_completed(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    d["last_completed_commit"] = args.sha
    save(p, d)
    return 0


def cmd_add_migration_pending(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    entry: dict = {"id": args.id, "status": args.status}
    if args.reason:
        entry["reason"] = args.reason
    d.setdefault("applied_migrations_pending", []).append(entry)
    save(p, d)
    return 0


def cmd_add_bootstrap_pending(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    entry: dict = {"id": args.id, "status": args.status}
    if args.reason:
        entry["reason"] = args.reason
    if args.content_hash:
        entry["content_hash"] = args.content_hash
    d.setdefault("bootstrap_steps_pending", []).append(entry)
    save(p, d)
    return 0


def cmd_add_deletion_decision(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    d.setdefault("deletions_user_decisions", {})[args.rel] = args.decision
    save(p, d)
    return 0


def cmd_add_deleted_pending(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    entry: dict = {"path": args.rel}
    if args.reason:
        entry["reason"] = args.reason
    d.setdefault("deleted_pending", []).append(entry)
    save(p, d)
    return 0


def cmd_get(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    if args.field not in d:
        print(f"field not found: {args.field}", file=sys.stderr)
        return 1
    val = d[args.field]
    if isinstance(val, (dict, list)):
        print(json.dumps(val))
    else:
        print(val)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_init = sub.add_parser("init")
    p_init.add_argument("path")
    p_init.add_argument("--commit-old", required=True)
    p_init.add_argument("--commit-new", required=True)
    p_init.add_argument("--branch", required=True)

    p_sp = sub.add_parser("set-phase")
    p_sp.add_argument("path")
    p_sp.add_argument("--phase", required=True)
    p_sp.add_argument("--status", required=True, choices=["started", "committed"])

    p_lc = sub.add_parser("set-last-completed")
    p_lc.add_argument("path")
    p_lc.add_argument("sha")

    p_amp = sub.add_parser("add-migration-pending")
    p_amp.add_argument("path")
    p_amp.add_argument("--id", required=True)
    p_amp.add_argument("--status", required=True, choices=["applied", "skipped"])
    p_amp.add_argument("--reason", default="")

    p_abp = sub.add_parser("add-bootstrap-pending")
    p_abp.add_argument("path")
    p_abp.add_argument("--id", required=True)
    p_abp.add_argument("--status", required=True, choices=["applied", "skipped"])
    p_abp.add_argument("--reason", default="")
    p_abp.add_argument("--content-hash", default="")

    p_add = sub.add_parser("add-deletion-decision")
    p_add.add_argument("path")
    p_add.add_argument("--rel", required=True)
    p_add.add_argument("--decision", required=True, choices=["remove", "preserve-local"])

    p_get = sub.add_parser("get")
    p_get.add_argument("path")
    p_get.add_argument("field")

    p_v = sub.add_parser("validate")
    p_v.add_argument("path")

    p_amd = sub.add_parser("add-deleted-pending")
    p_amd.add_argument("path")
    p_amd.add_argument("--rel", required=True)
    p_amd.add_argument("--reason", default="")

    args = parser.parse_args()
    dispatch = {
        "init": cmd_init,
        "set-phase": cmd_set_phase,
        "set-last-completed": cmd_set_last_completed,
        "add-migration-pending": cmd_add_migration_pending,
        "add-bootstrap-pending": cmd_add_bootstrap_pending,
        "add-deletion-decision": cmd_add_deletion_decision,
        "add-deleted-pending": cmd_add_deleted_pending,
        "get": cmd_get,
        "validate": cmd_validate,
    }
    return dispatch[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
