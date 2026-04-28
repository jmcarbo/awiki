#!/usr/bin/env python3
"""Transform one upstream markdown file into an awiki source page.

This is the v1 mechanical path — no LLM. Reads slug map on stdin (JSON
mapping repo-relpath → slug). Strips upstream frontmatter, rewrites
relative md links to wikilinks (Task 7), copies image assets (Task 8),
preserves code blocks verbatim, and emits awiki frontmatter.

Exit 0 on success, 1 on parse failure.
"""
import argparse
import json
import os
import re
import sys
from datetime import date
from pathlib import Path

UPSTREAM_FM_RE = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)


def strip_upstream_frontmatter(text: str) -> tuple[dict, str]:
    m = UPSTREAM_FM_RE.match(text)
    if not m:
        return {}, text
    raw = m.group(1)
    title = ""
    for line in raw.splitlines():
        line = line.rstrip()
        if line.startswith("title:"):
            title = line.split(":", 1)[1].strip().strip('"').strip("'")
            break
    return {"title": title}, text[m.end():]


def derive_title(upstream_meta: dict, body: str, fallback: str) -> str:
    if upstream_meta.get("title"):
        return upstream_meta["title"]
    for line in body.splitlines():
        if line.startswith("# "):
            return line[2:].strip()
    return fallback


def emit_frontmatter(args: argparse.Namespace, title: str, today: str) -> str:
    tags = ["git", args.repo_name]
    if args.private:
        tags.append("private")
    tags_inline = "[" + ", ".join(tags) + "]"
    lines = [
        "---",
        f'title: "{title}"',
        f"date: {today}",
        f"last_updated: {today}",
        "type: source",
        "provenance: git",
        f"git_repo: {args.repo_name}",
        f"git_path: {args.repo_relpath}",
        f"git_blob_sha: {args.git_blob_sha}",
        f"git_url: {args.git_url}",
        f"tags: {tags_inline}",
        "sources: []",
        "draft: false",
        "---",
        "",
    ]
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--in", dest="in_path", required=True)
    p.add_argument("--out", dest="out_path", required=True)
    p.add_argument("--repo-key", required=True)
    p.add_argument("--repo-name", required=True)
    p.add_argument("--repo-relpath", required=True)
    p.add_argument("--git-url", required=True)
    p.add_argument("--git-blob-sha", required=True)
    p.add_argument("--asset-out-dir", required=True)
    p.add_argument("--upstream-root", default="")
    p.add_argument("--private", action="store_true")
    args = p.parse_args(argv)

    try:
        slug_map = json.loads(sys.stdin.read() or "{}")
    except json.JSONDecodeError as e:
        print(f"ERROR|stdin slug map parse: {e}", file=sys.stderr); return 1

    in_path = Path(args.in_path)
    if not in_path.is_file():
        print(f"ERROR|in not found: {in_path}", file=sys.stderr); return 1

    raw = in_path.read_text(encoding="utf-8", errors="strict")
    if len(raw.strip()) < 10:
        print("WARN|skipped: <10 char body")
        return 0
    upstream_meta, body = strip_upstream_frontmatter(raw)
    fallback_title = in_path.stem.replace("-", " ").title()
    title = derive_title(upstream_meta, body, fallback_title)

    today = date.today().isoformat()
    out_text = emit_frontmatter(args, title, today) + body.lstrip("\n")

    out_path = Path(args.out_path)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    tmp = out_path.with_suffix(out_path.suffix + f".tmp.{os.getpid()}")
    tmp.write_text(out_text, encoding="utf-8")
    tmp.replace(out_path)
    print("OK|0")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
