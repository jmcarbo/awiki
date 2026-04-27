#!/usr/bin/env bash
set -euo pipefail

CATALOG="content/catalog.md"
[[ -f "$CATALOG" ]] || { echo "no catalog at $CATALOG" >&2; exit 1; }

python3 - "$CATALOG" content <<'PY'
import os, sys, re, glob, collections
catalog_path, content_dir = sys.argv[1:3]

groups = collections.OrderedDict([
    ("entity", "Entities"),
    ("concept", "Concepts"),
    ("topic", "Topics"),
    ("source", "Sources"),
    ("synthesis", "Synthesis"),
    ("deck", "Synthesis"),
    ("chart", "Synthesis"),
    ("canvas", "Synthesis"),
])

entries = collections.defaultdict(list)
for path in sorted(glob.glob(os.path.join(content_dir, "**", "*.md"), recursive=True)):
    rel = os.path.relpath(path, content_dir)
    slug = os.path.splitext(os.path.basename(path))[0]
    if slug in ("_index", "catalog", "log"):
        continue
    fm = {}
    with open(path) as f:
        text = f.read()
    m = re.match(r"---\n(.*?)\n---", text, re.S)
    if not m:
        continue
    for line in m.group(1).splitlines():
        if ":" in line:
            k, v = line.split(":", 1)
            fm[k.strip()] = v.strip().strip('"')
    t = fm.get("type", "")
    if t in ("log", "catalog", "section-index"):
        continue
    section = groups.get(t, "Misc")
    title = fm.get("title", slug)
    entries[section].append(f"- [[{slug}]] — {title}")

# Read existing catalog body up to first ##; replace below with our generated sections.
with open(catalog_path) as f:
    full = f.read()
m = re.split(r"^## ", full, maxsplit=1, flags=re.M)
prelude = m[0].rstrip() + "\n\n"
out = prelude
for section in ("Entities", "Concepts", "Topics", "Sources", "Synthesis", "Misc"):
    if entries.get(section):
        out += f"## {section}\n\n"
        out += "\n".join(entries[section]) + "\n\n"
with open(catalog_path, "w") as f:
    f.write(out.rstrip() + "\n")
PY

echo "CATALOG-OK"
