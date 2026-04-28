#!/usr/bin/env python3
"""Resolve `[[slug]]` references in a Vega-Lite spec to dataset URLs / values.

Usage:
  vl-resolve.py --spec=<path> --base-url=<base> [--repo-root=<dir>]

Exits non-zero on resolution failure with a `RESOLVE|<src>|<slug>|<reason>`
line on stdout (so callers can capture it).
"""
import argparse
import csv
import json
import re
import sys
from pathlib import Path

DATA_NAME_RE = re.compile(r"^\[\[([a-z0-9][a-z0-9-]*)\]\]$")
FRONTMATTER_RE = re.compile(r"^---\s*\n(.*?)\n---\s*$", re.M | re.S)


def _read_dataset_page(repo_root: Path, slug: str):
    page = repo_root / "content" / "datasets" / f"{slug}.md"
    if not page.exists():
        return None
    text = page.read_text(encoding="utf-8")
    m = FRONTMATTER_RE.search(text)
    if not m:
        return {"_no_fm": True}
    fm = {}
    for line in m.group(1).splitlines():
        kv = re.match(r"^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$", line)
        if not kv:
            continue
        k, raw = kv.group(1), kv.group(2).strip()
        if raw.startswith('"') and raw.endswith('"'):
            raw = raw[1:-1]
        fm[k] = raw
    fm["_text"] = text
    return fm


def _extract_inline(text: str, fmt: str) -> str:
    fence = re.compile(r"## Data[\s\S]*?```" + re.escape(fmt) + r"\s*\n(.*?)\n```", re.S)
    m = fence.search(text)
    return m.group(1) if m else ""


def _parse_csv(body: str, delim: str):
    return list(csv.DictReader(body.splitlines(), delimiter=delim))


def _to_values(body: str, fmt: str):
    if fmt == "csv":
        return _parse_csv(body, ",")
    if fmt == "tsv":
        return _parse_csv(body, "\t")
    if fmt == "dsv":
        first = body.splitlines()[0] if body else ""
        d = next((c for c in (",", ";", "|", "\t") if c in first), ",")
        return _parse_csv(body, d)
    if fmt == "json":
        d = json.loads(body)
        return d if isinstance(d, list) else [d]
    if fmt == "topojson":
        return [json.loads(body)]
    return []


def _resolve_data_block(data: dict, repo_root: Path, base_url: str, src: str):
    name = data.get("name") if isinstance(data, dict) else None
    if not isinstance(name, str):
        return data  # pass through `url`, `values`, or already-resolved blocks.
    m = DATA_NAME_RE.match(name)
    if not m:
        return data  # `name` exists but isn't `[[slug]]` syntax — leave to Vega-Lite.
    slug = m.group(1)
    fm = _read_dataset_page(repo_root, slug)
    if fm is None:
        print(f"RESOLVE|{src}|{slug}|not_found", flush=True)
        sys.exit(2)
    if fm.get("_no_fm") or fm.get("type") != "dataset":
        print(f"RESOLVE|{src}|{slug}|not_a_dataset", flush=True)
        sys.exit(3)
    storage = fm.get("storage")
    fmt = fm.get("format")
    if storage == "file":
        path = fm.get("data_path", f"data/{slug}.{fmt}")
        url = base_url.rstrip("/") + "/" + path.lstrip("/")
        return {"url": url, "format": {"type": fmt}}
    if storage == "inline":
        body = _extract_inline(fm["_text"], fmt)
        return {"values": _to_values(body, fmt)}
    print(f"RESOLVE|{src}|{slug}|bad_storage", flush=True)
    sys.exit(4)


def _walk(node, repo_root: Path, base_url: str, src: str):
    if isinstance(node, dict):
        if "data" in node and isinstance(node["data"], dict):
            node["data"] = _resolve_data_block(node["data"], repo_root, base_url, src)
        for k, v in list(node.items()):
            node[k] = _walk(v, repo_root, base_url, src)
        return node
    if isinstance(node, list):
        return [_walk(item, repo_root, base_url, src) for item in node]
    return node


def main():
    p = argparse.ArgumentParser()
    p.add_argument("--spec", required=True)
    p.add_argument("--base-url", default="/")
    p.add_argument("--repo-root", default=".")
    p.add_argument("--src", default="(unknown)", help="Source page path for diagnostics.")
    args = p.parse_args()

    repo_root = Path(args.repo_root).resolve()
    spec = json.loads(Path(args.spec).read_text(encoding="utf-8"))
    resolved = _walk(spec, repo_root, args.base_url, args.src)
    print(json.dumps(resolved, indent=2))


if __name__ == "__main__":
    main()
