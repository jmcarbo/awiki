#!/usr/bin/env python3
"""template-plan emitter. Diffs old/new template trees against a user tree
through the new manifest's strategies. Emits PLAN|... lines.

Args (all required):
  --old-tree  <dir>     # snapshot of last-applied template
  --new-tree  <dir>     # snapshot of incoming template
  --user-tree <dir>     # current bootstrapped repo (working tree minus .awiki/)
  --manifest  <path>    # incoming template's template.manifest.toml
  --commit-old <sha>
  --commit-new <sha>
  --provenance <path>   # path to .awiki/template.json (for bootstrap-step diff and user-deleted)
"""
from __future__ import annotations

import argparse
import filecmp
import hashlib
import os
import re
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import manifest_parse  # noqa: E402
from escape import escape  # noqa: E402


def emit(*cols: str) -> None:
    print("PLAN|" + "|".join(escape(c) for c in cols))


def list_files(root: Path) -> set[str]:
    out: set[str] = set()
    for p in root.rglob("*"):
        if p.is_file():
            rel = str(p.relative_to(root))
            if rel.startswith(".git/") or rel == ".git":
                continue
            out.add(rel)
    return out


def file_sha256(path: Path) -> str:
    h = hashlib.sha256()
    with path.open("rb") as f:
        for chunk in iter(lambda: f.read(65536), b""):
            h.update(chunk)
    return h.hexdigest()


def line_count(path: Path) -> int:
    with path.open("rb") as f:
        return sum(1 for _ in f)


def test_merge(base: Path, new: Path, user: Path, scratch: Path) -> tuple[str, int]:
    """Run git merge-file --diff3 in scratch dir. Return (status, conflict_count)."""
    if not user.exists():
        # User has the file deleted; treat as no merge needed (will be Commit-A decision).
        return ("clean", 0)
    if not base.exists():
        return ("clean", 0)
    work = scratch / "merge"
    work.mkdir(exist_ok=True)
    try:
        # git merge-file mutates the "current" file in place; use a copy.
        cur = work / "cur"
        shutil.copyfile(user, cur)
        b = work / "base"
        shutil.copyfile(base, b)
        n = work / "new"
        shutil.copyfile(new, n)
        result = subprocess.run(
            ["git", "merge-file", "--diff3", "-p", str(cur), str(b), str(n)],
            capture_output=True,
            text=True,
        )
        # exit 0 = clean; >0 = number of conflicts.
        if result.returncode == 0:
            return ("clean", 0)
        return ("conflict-predicted", result.returncode)
    finally:
        shutil.rmtree(work, ignore_errors=True)


def emit_categories(args: argparse.Namespace, manifest: dict) -> dict[str, int]:
    counts = {"errors": 0, "warnings": 0, "prompts": 0, "conflicts": 0}
    old_files = list_files(args.old_tree)
    new_files = list_files(args.new_tree)
    user_files = list_files(args.user_tree)

    changed_or_new = new_files - old_files | {p for p in old_files & new_files
                                              if (args.old_tree / p).is_file()
                                              and (args.new_tree / p).is_file()
                                              and not filecmp.cmp(args.old_tree / p, args.new_tree / p, shallow=False)}
    deleted_in_template = old_files - new_files
    really_new = new_files - old_files

    # Sort path-events alphabetically.
    paths = sorted(changed_or_new | deleted_in_template)

    scratch = Path(tempfile.mkdtemp(prefix="awiki-plan-"))
    try:
        for rel in paths:
            strategy = manifest_parse.resolve_strategy(manifest, rel)

            if rel in deleted_in_template:
                locally_modified = "false"
                if rel in user_files:
                    if not (args.old_tree / rel).is_file() or \
                       not (args.user_tree / rel).is_file() or \
                       not filecmp.cmp(args.old_tree / rel, args.user_tree / rel, shallow=False):
                        locally_modified = "true"
                emit("deletion-in-template", rel, locally_modified)
                continue

            if strategy == "overwrite":
                emit("overwrite", rel, "")
            elif strategy == "preserve":
                emit("preserve", rel, "")
            elif strategy == "template_only":
                emit("template_only", rel, "")
            elif strategy in ("three_way", "attributes_merge"):
                status, conflicts = test_merge(
                    args.old_tree / rel, args.new_tree / rel, args.user_tree / rel, scratch
                )
                emit(strategy, rel, status, str(conflicts))
                if status == "conflict-predicted":
                    counts["conflicts"] += conflicts
            elif strategy == "prompt":
                if rel in really_new:
                    emit("new_file", rel, "prompt")
                    counts["prompts"] += 1
                else:
                    emit("overwrite", rel, "fallback")  # shouldn't normally happen
            else:
                emit("template_only", rel, f"unknown-strategy:{strategy}")
                counts["warnings"] += 1
    finally:
        shutil.rmtree(scratch, ignore_errors=True)

    return counts


def emit_migrations(args: argparse.Namespace, manifest: dict) -> int:
    """Emit migration lines for any NNNN-*.sh or .prompt.md in new_tree/migrations/
    that aren't covered by old_tree/migrations/. Returns count of new migrations."""
    new_mig_dir = args.new_tree / "migrations"
    old_mig_dir = args.old_tree / "migrations"
    if not new_mig_dir.is_dir():
        return 0
    new_mig_files = sorted(p.name for p in new_mig_dir.iterdir() if p.is_file())
    old_mig_files = set()
    if old_mig_dir.is_dir():
        old_mig_files = {p.name for p in old_mig_dir.iterdir() if p.is_file()}
    added = [n for n in new_mig_files if n not in old_mig_files]
    count = 0
    for fname in added:
        if fname == "README.md" or fname == ".gitkeep":
            continue
        full = new_mig_dir / fname
        if fname.endswith(".sh"):
            mid = fname[:-3]
            count += 1
            stat = subprocess.run(
                ["wc", "-l", str(full)],
                capture_output=True, text=True
            )
            lines = stat.stdout.split()[0] if stat.returncode == 0 else "0"
            emit("migration", mid, f"migrations/{fname}", f"{lines}-lines")
            sha = file_sha256(full)
            emit("migration-content", mid, f"sha256:{sha}", lines)
        elif fname.endswith(".prompt.md"):
            mid = fname[:-len(".prompt.md")]
            count += 1
            # Extract scope_glob + risk from frontmatter.
            text = full.read_text(encoding="utf-8")
            scope = ""
            risk = "medium"
            in_fm = False
            for line in text.splitlines():
                if line.strip() == "---":
                    in_fm = not in_fm
                    if not in_fm:
                        break
                    continue
                if in_fm:
                    if line.startswith("scope_glob:"):
                        scope = line.split(":", 1)[1].strip().strip('"').strip("'")
                    elif line.startswith("risk:"):
                        risk = line.split(":", 1)[1].strip()
            # Resolve scope to count (simple glob via Python).
            from fnmatch import fnmatch
            user_files = list_files(args.user_tree)
            matched = [f for f in user_files if fnmatch(f, scope)] if scope else []
            emit(
                "migration-prompt",
                mid,
                f"migrations/{fname}",
                scope,
                str(len(matched)),
                risk,
            )
    return count


def emit_user_deleted(args: argparse.Namespace) -> int:
    """Emit PLAN|user-deleted lines for paths in template.json.deleted[]."""
    if not args.provenance or not args.provenance.is_file():
        return 0
    import json as _json
    d = _json.loads(args.provenance.read_text(encoding="utf-8"))
    count = 0
    for entry in d.get("deleted", []):
        rel = entry.get("path", "")
        reason = entry.get("reason", "")
        if rel:
            emit("user-deleted", rel, reason)
            count += 1
    return count


def emit_bootstrap_steps(args: argparse.Namespace, manifest: dict) -> int:
    """Emit bootstrap-step-{new,content-changed,dangerous} lines."""
    bs_md = args.new_tree / "BOOTSTRAP.md"
    if not bs_md.is_file():
        return 0
    sys.path.insert(0, str(HERE))
    import bootstrap_hash as _bh  # noqa: E402

    new_bodies = _bh.parse(bs_md.read_text(encoding="utf-8"))
    ordered = manifest.get("bootstrap", {}).get("ordered_steps", [])
    dangerous = set(manifest.get("bootstrap", {}).get("dangerous", {}).get("ids", []))

    pin_steps: dict[str, dict] = {}
    if args.provenance and args.provenance.is_file():
        import json as _json
        d = _json.loads(args.provenance.read_text(encoding="utf-8"))
        for s in d.get("bootstrap_steps_done", []):
            pin_steps[s.get("id", "")] = s

    count = 0
    for sid in ordered:
        if sid not in new_bodies:
            continue
        norm = re.sub(r"\s+", " ", new_bodies[sid]).strip()
        new_hash = "sha256:" + hashlib.sha256(norm.encode("utf-8")).hexdigest()
        existing = pin_steps.get(sid)
        if existing is None:
            if sid in dangerous:
                emit("bootstrap-step-dangerous", sid, "marked-dangerous-new")
            else:
                emit("bootstrap-step-new", sid)
            count += 1
            continue
        old_hash = existing.get("content_hash", "")
        if old_hash == new_hash and existing.get("status") == "applied":
            continue   # already done, no plan output
        if sid in dangerous:
            emit("bootstrap-step-dangerous", sid, "marked-dangerous-changed")
        else:
            emit("bootstrap-step-content-changed", sid, old_hash, new_hash)
        count += 1
    return count


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--old-tree", required=True, type=Path)
    p.add_argument("--new-tree", required=True, type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--manifest", required=True, type=Path)
    p.add_argument("--commit-old", required=True)
    p.add_argument("--commit-new", required=True)
    p.add_argument("--provenance", type=Path, default=None,
                   help=".awiki/template.json (for bootstrap-step + user-deleted lines)")
    args = p.parse_args()

    manifest = manifest_parse.load(args.manifest)
    schema = manifest.get("schema_version", 1)
    print(f"PLAN|header|{schema}|{args.commit_old}|{args.commit_new}")

    counts = emit_categories(args, manifest)
    counts["new_migrations"] = emit_migrations(args, manifest)
    emit_user_deleted(args)
    emit_bootstrap_steps(args, manifest)

    print(
        f"PLAN|footer|errors={counts['errors']}|warnings={counts['warnings']}"
        f"|prompts={counts['prompts']}|conflicts={counts['conflicts']}"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
