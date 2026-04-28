import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const FRONTMATTER_RE = /^---\s*\n([\s\S]*?)\n---\s*$/m;

function parseFrontmatter(text) {
  const m = FRONTMATTER_RE.exec(text);
  if (!m) return null;
  const out = {};
  const body = m[1];
  // Naive YAML scalar parser. Sufficient because we only read scalars + a
  // flat tags array; nested structures (columns:) are parsed lazily by
  // get-dataset.js when needed.
  let inTags = false;
  for (const line of body.split("\n")) {
    if (inTags) {
      const m2 = line.match(/^\s+-\s*(.+)$/);
      if (m2) {
        const v = m2[1].trim().replace(/^["']|["']$/g, "");
        out.tags.push(v);
        continue;
      } else if (!line.startsWith(" ") && !line.startsWith("\t")) {
        inTags = false;
      } else {
        continue;
      }
    }
    const kv = line.match(/^([A-Za-z_][A-Za-z0-9_]*):\s*(.*)$/);
    if (!kv) continue;
    const [, k, raw] = kv;
    if (k === "tags") {
      const inline = raw.trim();
      if (inline.startsWith("[") && inline.endsWith("]")) {
        out.tags = inline
          .slice(1, -1)
          .split(",")
          .map((s) => s.trim().replace(/^["']|["']$/g, ""))
          .filter(Boolean);
      } else {
        out.tags = [];
        inTags = true;
      }
      continue;
    }
    let v = raw.trim();
    if (v.startsWith('"') && v.endsWith('"')) v = v.slice(1, -1);
    out[k] = v;
  }
  return out;
}

export function listDatasets(repoRoot) {
  const dir = join(repoRoot, "content", "datasets");
  let entries;
  try {
    entries = readdirSync(dir);
  } catch {
    return { datasets: [], errors: [] };
  }
  const datasets = [];
  const errors = [];
  for (const name of entries) {
    if (!name.endsWith(".md")) continue;
    const slug = name.slice(0, -3);
    const path = join(dir, name);
    try {
      const text = readFileSync(path, "utf8");
      const fm = parseFrontmatter(text);
      if (!fm || fm.type !== "dataset") continue;
      datasets.push({
        slug,
        format: fm.format,
        storage: fm.storage,
        rows: fm.rows ? Number(fm.rows) : null,
        tags: fm.tags || [],
        last_updated: fm.last_updated || null,
      });
    } catch (e) {
      errors.push({ slug, reason: String(e.message || e) });
    }
  }
  datasets.sort((a, b) => a.slug.localeCompare(b.slug));
  return { datasets, errors };
}
