#!/usr/bin/env python3
"""Migration runner + LLM stager.

Subcommands:
  parse-header <script>
  validate-touches <script>     — fails if touches: includes secrets/, .awiki/, .git/, themes/
  run <script> --repo-root <dir> --old-version <v> --new-version <v>
  stage-prompt <prompt-md> --user-tree <dir> --pending-dir <dir> --gitattributes <path>
"""
from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from fnmatch import fnmatch
from pathlib import Path

REQUIRED_HEADER_KEYS = {"migration", "requires", "touches", "idempotent"}
BLOCKED_PATTERNS = ("secrets/", "themes/", ".awiki/", ".git/")


def parse_header(script: Path) -> dict:
    out: dict = {}
    with script.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.startswith("#"):
                if line.strip() == "":
                    continue
                if line.startswith("#!"):
                    continue
                # Hit non-comment line; stop.
                break
            m = re.match(r"#\s*([a-z_]+)\s*:\s*(.*)$", line.rstrip("\n"))
            if m:
                out[m.group(1)] = m.group(2).strip()
    return out


def cmd_parse_header(args: argparse.Namespace) -> int:
    h = parse_header(args.script)
    missing = REQUIRED_HEADER_KEYS - set(h.keys())
    if missing:
        print(f"missing header keys: {', '.join(sorted(missing))}", file=sys.stderr)
        return 1
    for k, v in h.items():
        print(f"{k}={v}")
    return 0


def cmd_validate_touches(args: argparse.Namespace) -> int:
    h = parse_header(args.script)
    touches = h.get("touches", "").split()
    for glob in touches:
        for blocked in BLOCKED_PATTERNS:
            if glob.startswith(blocked) or glob == blocked.rstrip("/"):
                print(f"touches blocked: {glob}", file=sys.stderr)
                return 1
    return 0


def cmd_run(args: argparse.Namespace) -> int:
    h = parse_header(args.script)
    if cmd_validate_touches(args) != 0:
        return 1
    touches = h.get("touches", "").split()

    repo_root = args.repo_root
    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": os.environ.get("HOME", str(Path.home())),
        "AWIKI_REPO_ROOT": str(repo_root),
        "AWIKI_TEMPLATE_OLD_VERSION": args.old_version,
        "AWIKI_TEMPLATE_NEW_VERSION": args.new_version,
        "LANG": os.environ.get("LANG", "C.UTF-8"),
        "LC_ALL": os.environ.get("LC_ALL", "C.UTF-8"),
    }

    # Run --dry-run first as audit log.
    print(f"info: dry-run {args.script}", file=sys.stderr)
    rc = subprocess.call(["bash", str(args.script), "--dry-run"], env=env, cwd=str(repo_root))
    if rc != 0:
        print(f"migration dry-run failed: {args.script}", file=sys.stderr)
        return rc

    # Real run.
    rc = subprocess.call(["bash", str(args.script)], env=env, cwd=str(repo_root))
    if rc != 0:
        return rc

    # Post-run audit: every modified path must match touches: or be in .awiki/ blocklist.
    out = subprocess.run(
        ["git", "-C", str(repo_root), "status", "--porcelain"],
        capture_output=True, text=True
    ).stdout

    out_of_scope_tracked: list = []
    out_of_scope_untracked: list = []
    awiki_writes_tracked: list = []
    awiki_writes_untracked: list = []

    # If the migration script itself lives under repo_root, exclude it from audit
    # (it's the script being executed, not a write produced by execution).
    script_rel = None
    try:
        script_rel = str(Path(args.script).resolve().relative_to(Path(repo_root).resolve()))
    except (ValueError, RuntimeError):
        script_rel = None

    # Parse porcelain to distinguish tracked vs untracked.
    # Format: "XY filename" — untracked is "?? filename".
    for line in out.splitlines():
        if not line.strip():
            continue
        xy = line[:2]
        rel = line[3:].strip()
        if "->" in rel:
            rel = rel.split("->", 1)[1].strip()
        if script_rel is not None and rel == script_rel:
            continue
        is_untracked = (xy == "??")
        if rel.startswith(".awiki/"):
            (awiki_writes_untracked if is_untracked else awiki_writes_tracked).append(rel)
        elif not any(_glob_match(g, rel) for g in touches):
            (out_of_scope_untracked if is_untracked else out_of_scope_tracked).append(rel)

    def revert(tracked: list, untracked: list) -> None:
        if tracked:
            subprocess.call(["git", "-C", str(repo_root), "restore", "--source=HEAD", "--", *tracked])
        for u in untracked:
            try:
                (repo_root / u).unlink()
            except FileNotFoundError:
                pass

    if awiki_writes_tracked or awiki_writes_untracked:
        all_awiki = awiki_writes_tracked + awiki_writes_untracked
        print(f"migration wrote to .awiki/: {all_awiki}", file=sys.stderr)
        revert(awiki_writes_tracked, awiki_writes_untracked)
        return 1
    if out_of_scope_tracked or out_of_scope_untracked:
        all_oos = out_of_scope_tracked + out_of_scope_untracked
        print(f"migration wrote outside touches:: {all_oos}", file=sys.stderr)
        revert(out_of_scope_tracked, out_of_scope_untracked)
        return 1
    return 0


def _glob_match(glob: str, path: str) -> bool:
    if "**" in glob:
        parts = glob.split("**")
        if len(parts) == 2:
            prefix, suffix = parts
            prefix = prefix.rstrip("/")
            suffix = suffix.lstrip("/")
            if prefix and not (path == prefix or path.startswith(prefix + "/")):
                return False
            if suffix and not fnmatch(path, "*" + suffix):
                return False
            return True
    return fnmatch(path, glob)


def cmd_stage_prompt(args: argparse.Namespace) -> int:
    text = args.prompt_md.read_text(encoding="utf-8")
    fm = _parse_yaml_frontmatter(text)
    stem = args.prompt_md.stem
    if stem.endswith(".prompt"):
        stem = stem[: -len(".prompt")]
    sid = fm.get("id", stem)
    scope = fm.get("scope_glob", "")
    risk = fm.get("risk", "medium")

    if not scope:
        print(f"prompt missing scope_glob: {args.prompt_md}", file=sys.stderr)
        return 1
    for blocked in BLOCKED_PATTERNS:
        if scope.startswith(blocked) or scope == blocked.rstrip("/"):
            print(f"scope_glob blocked: {scope}", file=sys.stderr)
            return 1

    # Resolve scope.
    matches: list = []
    for p in args.user_tree.rglob("*"):
        if p.is_file():
            rel = str(p.relative_to(args.user_tree))
            if _glob_match(scope, rel):
                matches.append(rel)

    # Encrypted-path filter: reject scope_glob matching any path with filter=git-crypt.
    if args.gitattributes is not None and args.gitattributes.is_file():
        for rel in matches:
            cr = subprocess.run(
                ["git", "-c", f"core.attributesfile={args.gitattributes}",
                 "check-attr", "filter", rel],
                capture_output=True, text=True
            )
            if "filter: git-crypt" in cr.stdout:
                print(f"scope_glob would match encrypted path: {rel}", file=sys.stderr)
                return 1

    args.pending_dir.mkdir(parents=True, exist_ok=True)
    dest = args.pending_dir / args.prompt_md.name
    body = text
    metadata = "\n\n## Resolved scope\n" + "\n".join(f"- {m}" for m in matches)
    metadata += "\n\n## Acceptance\nAfter completing, run: `just lint`."
    metadata += "\n\n## Trust note\nConfirm intent with user before bulk edits."
    metadata += f"\n\n## Risk\n{risk}\n"
    dest.write_text(body + metadata, encoding="utf-8")
    print(f"info: staged {dest.name} (id={sid}, risk={risk}, files={len(matches)})", file=sys.stderr)
    return 0


def _parse_yaml_frontmatter(text: str) -> dict:
    out: dict = {}
    in_fm = False
    for line in text.splitlines():
        if line.strip() == "---":
            if in_fm:
                break
            in_fm = True
            continue
        if in_fm and ":" in line:
            k, v = line.split(":", 1)
            out[k.strip()] = v.strip().strip('"').strip("'").strip("[").strip("]")
    return out


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("parse-header"); p.add_argument("script", type=Path)
    p = sub.add_parser("validate-touches"); p.add_argument("script", type=Path)
    p = sub.add_parser("run")
    p.add_argument("script", type=Path)
    p.add_argument("--repo-root", required=True, type=Path)
    p.add_argument("--old-version", required=True)
    p.add_argument("--new-version", required=True)
    p = sub.add_parser("stage-prompt")
    p.add_argument("prompt_md", type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--pending-dir", required=True, type=Path)
    p.add_argument("--gitattributes", required=False, type=Path, default=None)

    args = parser.parse_args()
    return {
        "parse-header": cmd_parse_header,
        "validate-touches": cmd_validate_touches,
        "run": cmd_run,
        "stage-prompt": cmd_stage_prompt,
    }[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
