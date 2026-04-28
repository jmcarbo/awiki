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
import shutil
import sys
from datetime import date
from pathlib import Path

from markdown_it import MarkdownIt

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


def _normalize_relpath(current_relpath: str, link_target: str) -> str:
    """Resolve link_target (relative to file containing it) to a repo-relpath
    suitable for slug-map lookup. Strips fragments and query strings.
    Returns '' if the target leaves the repo (..-overshoot)."""
    if "://" in link_target or link_target.startswith("/"):
        return ""
    target = link_target.split("#", 1)[0].split("?", 1)[0]
    if not target:
        return ""
    base = os.path.dirname(current_relpath)
    joined = os.path.normpath(os.path.join(base, target)) if base else os.path.normpath(target)
    if joined.startswith("..") or joined == ".":
        return ""
    return joined


def _is_md_target(path: str) -> bool:
    return path.lower().endswith(".md") or path.lower().endswith(".mdx")


def _compute_fence_ranges(body: str) -> list:
    """Walk markdown-it tokens and return list of (start_line, end_line)
    half-open ranges that are inside fenced code blocks or indented code blocks."""
    md = MarkdownIt("commonmark")
    tokens = md.parse(body)
    ranges = []
    for t in tokens:
        if t.type in ("fence", "code_block") and t.map:
            ranges.append((t.map[0], t.map[1]))
    return ranges


LINK_RE = re.compile(r"(?<!`)\[([^\]]+)\]\(([^)]+)\)")
REF_RE = re.compile(r"^\[([^\]]+)\]:\s*(\S+)\s*$")
REFLINK_RE = re.compile(r"\[([^\]]+)\]\[([^\]]+)\]")
IMG_RE = re.compile(r"!\[([^\]]*)\]\(([^)]+)\)")


def rewrite_images(body: str, current_relpath: str, upstream_root: str,
                   asset_out_dir: str, fence_ranges: list) -> tuple:
    """Find ![alt](path) refs outside code fences. Copy referenced files
    from upstream_root → asset_out_dir/<repo-relpath-of-image>, rewrite the
    path in the body. Missing files → leave the link, emit warning."""
    warnings = []

    def _line_in_fence(idx):
        for s, e in fence_ranges:
            if s <= idx < e:
                return True
        return False

    new_lines = []
    for i, line in enumerate(body.splitlines()):
        if _line_in_fence(i):
            new_lines.append(line); continue

        def _sub(m):
            alt, href = m.group(1), m.group(2)
            href_path = href.split()[0]
            if "://" in href_path or href_path.startswith("/"):
                return m.group(0)
            target = _normalize_relpath(current_relpath, href_path)
            if not target:
                return m.group(0)
            src = Path(upstream_root) / target
            if not src.is_file():
                warnings.append(f"missing image: {target}")
                return m.group(0)
            dest = Path(asset_out_dir) / target
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dest)
            rel_from_content = str(dest)
            if rel_from_content.startswith("content/"):
                rel_from_content = rel_from_content[len("content/"):]
            return f"![{alt}]({rel_from_content})"

        new_lines.append(IMG_RE.sub(_sub, line))
    out = "\n".join(new_lines)
    if body.endswith("\n"):
        out += "\n"
    return out, warnings


def rewrite_wikilinks(body: str, current_relpath: str, slug_map: dict, fence_ranges: list) -> tuple:
    """Replace inline + reference-style md links with wikilinks where target
    is in slug_map. Skip lines inside fence_ranges. Return (new_body, warnings)."""
    def in_fence(line_idx: int) -> bool:
        for s, e in fence_ranges:
            if s <= line_idx < e:
                return True
        return False

    refs = {}
    for i, line in enumerate(body.splitlines()):
        if in_fence(i):
            continue
        m = REF_RE.match(line)
        if m:
            refs[m.group(1).lower()] = m.group(2)

    warnings = []

    def rewrite_inline(line_idx: int, line: str) -> str:
        if in_fence(line_idx):
            return line

        def _sub(m):
            text, href = m.group(1), m.group(2)
            href_path = href.split()[0]
            if not _is_md_target(href_path):
                return m.group(0)
            target = _normalize_relpath(current_relpath, href_path)
            slug = slug_map.get(target)
            if not slug:
                return m.group(0)
            return f"[[{slug}|{text}]]"

        out = LINK_RE.sub(_sub, line)

        def _sub_ref(m):
            text, ref_id = m.group(1), m.group(2).lower()
            href = refs.get(ref_id)
            if not href or not _is_md_target(href):
                return m.group(0)
            target = _normalize_relpath(current_relpath, href)
            slug = slug_map.get(target)
            if not slug:
                return m.group(0)
            return f"[[{slug}|{text}]]"

        out = REFLINK_RE.sub(_sub_ref, out)
        return out

    new_lines = []
    for i, line in enumerate(body.splitlines()):
        new_lines.append(rewrite_inline(i, line))
    new_body = "\n".join(new_lines)
    if body.endswith("\n"):
        new_body += "\n"
    return new_body, warnings


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

    fence_ranges = _compute_fence_ranges(body)
    if args.upstream_root:
        body, img_warnings = rewrite_images(body, args.repo_relpath, args.upstream_root,
                                             args.asset_out_dir, fence_ranges)
        for w in img_warnings:
            print(f"WARN|{w}")
    body, link_warnings = rewrite_wikilinks(body, args.repo_relpath, slug_map, fence_ranges)
    for w in link_warnings:
        print(f"WARN|{w}")

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
