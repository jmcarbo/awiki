#!/usr/bin/env python3
"""Template-update lint subroutines.

Run as: lint_template.py --root <repo-root>

Emits LINT|<level>|<file>|<msg> lines. Exit 0 = clean, 1 = warnings, 2 = errors.
"""
from __future__ import annotations

import argparse
import json
import re
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import bootstrap_hash  # noqa: E402
import manifest_parse  # noqa: E402

REQUIRED_MIG_HEADERS = {"migration", "requires", "touches", "idempotent"}
REQUIRED_PROMPT_KEYS = {"id", "requires", "scope_glob", "risk"}
RISK_VALUES = {"low", "medium", "high"}
BLOCKED_PATTERNS = ("secrets/", "themes/", ".awiki/", ".git/")


def emit(level: str, file: str, msg: str) -> None:
    print(f"LINT|{level}|{file}|{msg}")


def lint_manifest(root: Path) -> tuple[int, int]:
    errors = warnings = 0
    mp = root / "template.manifest.toml"
    if not mp.is_file():
        emit("error", "template.manifest.toml", "missing")
        return (1, 0)
    m = manifest_parse.load(mp)
    for strat, globs in m.get("strategies", {}).items():
        seen: set[str] = set()
        for g in globs:
            if g in seen:
                emit("warning", "template.manifest.toml", f"duplicate glob in {strat}: {g}")
                warnings += 1
            seen.add(g)
    return (errors, warnings)


def lint_migrations(root: Path) -> tuple[int, int]:
    errors = warnings = 0
    mig_dir = root / "migrations"
    if not mig_dir.is_dir():
        return (0, 0)
    pat = re.compile(r"^(\d{4}-[a-z0-9-]+\.(?:sh|prompt\.md)|schema-\d+-to-\d+\.sh|README\.md|\.gitkeep)$")
    for entry in mig_dir.iterdir():
        if not entry.is_file():
            continue
        if not pat.match(entry.name):
            emit("error", str(entry.relative_to(root)), "migration filename does not match pattern")
            errors += 1
            continue
        if entry.name.endswith(".sh") and not entry.name.startswith("schema-"):
            text = entry.read_text(encoding="utf-8", errors="replace")
            headers: set[str] = set()
            for line in text.splitlines():
                if not line.startswith("#"):
                    if line.strip() == "" or line.startswith("#!"):
                        continue
                    break
                m_hdr = re.match(r"#\s*([a-z_]+)\s*:", line)
                if m_hdr:
                    headers.add(m_hdr.group(1))
            missing = REQUIRED_MIG_HEADERS - headers
            if missing:
                emit("error", str(entry.relative_to(root)), f"missing header keys: {sorted(missing)}")
                errors += 1
            for line in text.splitlines():
                m_t = re.match(r"#\s*touches\s*:\s*(.*)$", line)
                if m_t:
                    for g in m_t.group(1).split():
                        for blocked in BLOCKED_PATTERNS:
                            if g.startswith(blocked) or g == blocked.rstrip("/"):
                                emit("error", str(entry.relative_to(root)),
                                     f"touches: includes blocked pattern {g}")
                                errors += 1
        elif entry.name.endswith(".prompt.md"):
            text = entry.read_text(encoding="utf-8", errors="replace")
            fm: dict[str, str] = {}
            in_fm = False
            seen_open = False
            for line in text.splitlines():
                if line.strip() == "---":
                    if not seen_open:
                        seen_open = True
                        in_fm = True
                        continue
                    else:
                        in_fm = False
                        break
                if in_fm and ":" in line:
                    k, v = line.split(":", 1)
                    fm[k.strip()] = v.strip().strip('"').strip("'")
            missing = REQUIRED_PROMPT_KEYS - fm.keys()
            if missing:
                emit("error", str(entry.relative_to(root)), f"missing frontmatter keys: {sorted(missing)}")
                errors += 1
            risk = fm.get("risk", "")
            if risk and risk not in RISK_VALUES:
                emit("error", str(entry.relative_to(root)), f"risk must be low|medium|high: {risk}")
                errors += 1
            scope = fm.get("scope_glob", "")
            for blocked in BLOCKED_PATTERNS:
                if scope.startswith(blocked) or scope == blocked.rstrip("/"):
                    emit("error", str(entry.relative_to(root)),
                         f"scope_glob points at blocked pattern: {scope}")
                    errors += 1
    return (errors, warnings)


def lint_pending_prompts(root: Path) -> tuple[int, int]:
    errors = warnings = 0
    pp = root / ".awiki" / "pending-prompts"
    if not pp.is_dir():
        return (0, 0)
    cutoff = datetime.now(timezone.utc) - timedelta(days=14)
    for p in pp.glob("*.md"):
        mtime = datetime.fromtimestamp(p.stat().st_mtime, tz=timezone.utc)
        if mtime < cutoff:
            emit("warning", str(p.relative_to(root)),
                 f"pending prompt older than 14 days (mtime={mtime.isoformat()})")
            warnings += 1
    return (errors, warnings)


def lint_provenance_drift(root: Path) -> tuple[int, int]:
    pj = root / ".awiki" / "template.json"
    if not pj.is_file():
        return (0, 0)
    d = json.loads(pj.read_text(encoding="utf-8"))
    warnings = 0
    if d.get("repo") != d.get("original_repo"):
        emit("warning", ".awiki/template.json",
             f"repo differs from original_repo (repo={d.get('repo')}, original={d.get('original_repo')})")
        warnings += 1
    pinned = d.get("commit", "")
    if pinned and not (root / ".awiki" / "template-cache" / pinned).is_dir():
        emit("warning", ".awiki/template.json",
             f"pinned commit {pinned[:12]} not in template-cache; will auto-recover on next update")
        warnings += 1
    return (0, warnings)


def lint_orphans(root: Path) -> tuple[int, int]:
    """_fetch and _scratch-merge orphans outside an active --continue session."""
    warnings = 0
    fetch_state = root / ".awiki" / "template-cache" / "_fetch" / ".update-state.json"
    if fetch_state.is_file():
        emit("warning", ".awiki/template-cache/_fetch/.update-state.json",
             "orphaned state file (in-progress update or crashed run); use --continue or --abort")
        warnings += 1
    scratch = root / ".awiki" / "template-cache" / "_fetch" / "_scratch-merge"
    if scratch.is_dir():
        emit("warning", ".awiki/template-cache/_fetch/_scratch-merge",
             "orphaned plan-time scratch dir; safe to delete")
        warnings += 1
    return (0, warnings)


def lint_pending_prompt_drift(root: Path) -> tuple[int, int]:
    """Pending-prompt mtime newer than most recent applied_migrations entry for same id."""
    pp = root / ".awiki" / "pending-prompts"
    pj = root / ".awiki" / "template.json"
    if not pp.is_dir() or not pj.is_file():
        return (0, 0)
    d = json.loads(pj.read_text(encoding="utf-8"))
    applied_ids = {m.get("id"): m for m in d.get("applied_migrations", [])}
    warnings = 0
    for p in pp.glob("*.md"):
        mid = p.stem.removesuffix(".prompt")
        if mid in applied_ids:
            mtime = datetime.fromtimestamp(p.stat().st_mtime, tz=timezone.utc)
            emit("warning", str(p.relative_to(root)),
                 f"pending prompt for {mid} present but applied_migrations[] already records it (mtime={mtime.isoformat()}) — agent may have run without recording")
            warnings += 1
    return (0, warnings)


def lint_prompt_body_scope(root: Path) -> tuple[int, int]:
    """Pending-prompt body must not reference paths outside its declared scope_glob.

    Specifically: the resolved scope listed in the prompt body must not include
    paths in BLOCKED_PATTERNS (secrets/, themes/, .awiki/, .git/).
    """
    pp = root / ".awiki" / "pending-prompts"
    if not pp.is_dir():
        return (0, 0)
    errors = 0
    for p in pp.glob("*.md"):
        text = p.read_text(encoding="utf-8")
        in_fm = False
        seen_open = False
        scope = ""
        for line in text.splitlines():
            if line.strip() == "---":
                if not seen_open:
                    seen_open = True
                    in_fm = True
                    continue
                else:
                    in_fm = False
                    break
            if in_fm and line.startswith("scope_glob:"):
                scope = line.split(":", 1)[1].strip().strip('"').strip("'")
        # Parse "## Resolved scope" body section.
        resolved: list[str] = []
        in_resolved = False
        for line in text.splitlines():
            stripped = line.strip()
            if stripped.startswith("## ") and "resolved scope" in stripped.lower():
                in_resolved = True
                continue
            if in_resolved:
                if stripped.startswith("## "):
                    in_resolved = False
                elif stripped.startswith("- "):
                    resolved.append(stripped[2:].strip())
        if scope:
            for rel in resolved:
                for blocked in BLOCKED_PATTERNS:
                    if rel.startswith(blocked):
                        emit("error", str(p.relative_to(root)),
                             f"resolved scope includes blocked path: {rel}")
                        errors += 1
                        break
    return (errors, 0)


def lint_availability(root: Path) -> tuple[int, int]:
    """Run availability.py --check; output goes through to stdout. Never fails the run."""
    subprocess.run(
        ["python3", str(HERE / "availability.py"), "--root", str(root), "--check"],
        check=False,
    )
    return (0, 0)


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--root", required=True, type=Path)
    args = p.parse_args()
    total_e = total_w = 0
    for fn in (
        lint_manifest,
        lint_migrations,
        lint_pending_prompts,
        lint_provenance_drift,
        lint_orphans,
        lint_pending_prompt_drift,
        lint_prompt_body_scope,
        lint_availability,
    ):
        e, w = fn(args.root)
        total_e += e
        total_w += w
    if total_e > 0:
        return 2
    if total_w > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
