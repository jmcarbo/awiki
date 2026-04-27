#!/usr/bin/env python3
"""template.json reader/writer with schema validation.

Subcommands:
  init <path> <repo> <ref> <version> <commit>
  get <path> <field>
  set <path> <field> <value>      — refuses original_repo, schema_version
  append-migration <path> <id> <status> [reason]
  append-bootstrap-step <path> <id> <status> [reason] [content_hash]
  has-step-applied <path> <id> <expected_hash>   — exit 0 if applied + hash matches
  list-steps <path>               — emit "id status content_hash" per line
"""
from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

SCHEMA_VERSION = 1
IMMUTABLE_FIELDS = {"original_repo", "schema_version"}


def load(path: Path) -> dict:
    if not path.is_file():
        return {}
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def save(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def cmd_init(path: Path, repo: str, ref: str, version: str, commit: str) -> int:
    data = {
        "schema_version": SCHEMA_VERSION,
        "repo": repo,
        "original_repo": repo,
        "ref": ref,
        "version": version,
        "commit": commit,
        "applied_migrations": [],
        "deleted": [],
        "bootstrap_steps_done": [],
    }
    save(path, data)
    return 0


def cmd_get(path: Path, field: str) -> int:
    d = load(path)
    if field not in d:
        print(f"field not found: {field}", file=sys.stderr)
        return 1
    val = d[field]
    if isinstance(val, (dict, list)):
        print(json.dumps(val))
    else:
        print(val)
    return 0


def cmd_set(path: Path, field: str, value: str) -> int:
    if field in IMMUTABLE_FIELDS:
        print(f"field is immutable: {field}", file=sys.stderr)
        return 1
    d = load(path)
    d[field] = value
    save(path, d)
    return 0


def cmd_append_migration(path: Path, mid: str, status: str, reason: str = "") -> int:
    d = load(path)
    entry: dict[str, Any] = {"id": mid, "status": status}
    if reason:
        entry["reason"] = reason
    d.setdefault("applied_migrations", []).append(entry)
    save(path, d)
    return 0


def cmd_append_bootstrap_step(
    path: Path, sid: str, status: str, reason: str = "", content_hash: str = ""
) -> int:
    d = load(path)
    entry: dict[str, Any] = {"id": sid, "status": status}
    if reason:
        entry["reason"] = reason
    if content_hash:
        entry["content_hash"] = content_hash
    d.setdefault("bootstrap_steps_done", []).append(entry)
    save(path, d)
    return 0


def cmd_has_step_applied(path: Path, sid: str, expected_hash: str) -> int:
    d = load(path)
    for s in d.get("bootstrap_steps_done", []):
        if s.get("id") == sid and s.get("status") == "applied" and s.get("content_hash") == expected_hash:
            return 0
    return 1


def cmd_list_steps(path: Path) -> int:
    d = load(path)
    for s in d.get("bootstrap_steps_done", []):
        print(f"{s.get('id')} {s.get('status')} {s.get('content_hash', '')}")
    return 0


DISPATCH = {
    "init": (5, cmd_init),
    "get": (2, cmd_get),
    "set": (3, cmd_set),
    "append-migration": (3, cmd_append_migration),
    "append-bootstrap-step": (3, cmd_append_bootstrap_step),
    "has-step-applied": (3, cmd_has_step_applied),
    "list-steps": (1, cmd_list_steps),
}


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: provenance.py <subcmd> <path> [args...]", file=sys.stderr)
        return 2
    subcmd = sys.argv[1]
    if subcmd not in DISPATCH:
        print(f"unknown subcmd: {subcmd}", file=sys.stderr)
        return 2
    min_args, fn = DISPATCH[subcmd]
    path = Path(sys.argv[2])
    args = sys.argv[3:]
    if len(args) < min_args - 1:
        print(f"too few args for {subcmd}", file=sys.stderr)
        return 2
    return fn(path, *args)


if __name__ == "__main__":
    sys.exit(main())
