import { readdirSync, readFileSync, statSync, existsSync } from "node:fs";
import { join, relative, sep } from "node:path";

const FRONTMATTER_RE = /^---\s*\n([\s\S]*?)\n---\s*$/m;
// Inline awiki-query fence: ```sql awiki-query id="<id>"
const FENCE_RE = /```sql\s+awiki-query\s+id="([a-z0-9][a-z0-9-]*)"\s*\n([\s\S]*?)\n```/g;
// Extract dataset slugs from FROM clauses in SQL (rough but sufficient for awiki's deterministic shape).
const FROM_RE = /\bFROM\s+([a-z0-9][a-z0-9-]*)/gi;

function* walk(dir) {
  let entries;
  try { entries = readdirSync(dir); } catch { return; }
  for (const e of entries) {
    const p = join(dir, e);
    let st;
    try { st = statSync(p); } catch { continue; }
    if (st.isDirectory()) yield* walk(p);
    else if (e.endsWith(".md")) yield p;
  }
}

function parseScalar(fm, key) {
  const re = new RegExp("^" + key + ":\\s*(.*)$", "m");
  const m = re.exec(fm);
  if (!m) return null;
  let v = m[1].trim();
  if (v.startsWith('"') && v.endsWith('"')) v = v.slice(1, -1);
  return v;
}

// Parse the inline-list shape used by `sources:` in query frontmatter.
// Accepts: `sources: [trades, regions]` or `sources: []`.
// Returns array of slug strings.
function parseSources(fm) {
  const raw = parseScalar(fm, "sources");
  if (!raw) return [];
  const inner = raw.trim();
  if (!inner.startsWith("[") || !inner.endsWith("]")) return [];
  return inner
    .slice(1, -1)
    .split(",")
    .map((s) => s.trim().replace(/^["']|["']$/g, ""))
    .filter(Boolean);
}

// Extract deduplicated source slugs from SQL via FROM clauses.
function sourcesFromSql(sql) {
  const seen = new Set();
  let m;
  FROM_RE.lastIndex = 0;
  while ((m = FROM_RE.exec(sql)) !== null) {
    seen.add(m[1].toLowerCase());
  }
  return [...seen].sort();
}

export function listQueries(repoRoot) {
  const queries = [];
  const errors = [];
  const contentDir = join(repoRoot, "content");
  if (!existsSync(contentDir)) return { queries, errors };

  for (const path of walk(contentDir)) {
    let text;
    try { text = readFileSync(path, "utf8"); } catch (e) {
      errors.push({ path, reason: String(e.message || e) });
      continue;
    }
    const fmMatch = FRONTMATTER_RE.exec(text);
    const fm = fmMatch ? fmMatch[1] : "";
    const isQueryPage = /^type:\s*query\s*$/m.test(fm);
    const slug = path.split(sep).pop().slice(0, -3);
    const rel = relative(repoRoot, path);

    // --- type:query page ---
    if (isQueryPage) {
      const out = parseScalar(fm, "out");
      const sources = parseSources(fm);
      const entry = {
        page: rel,
        kind: "page",
        slug,
        out: out ?? null,
        sources,
      };
      queries.push(entry);
    }

    // --- inline awiki-query fences ---
    const fences = [...text.matchAll(FENCE_RE)];
    fences.forEach((m, idx) => {
      const id = m[1];
      const sql = m[2];
      const sources = sourcesFromSql(sql);
      queries.push({
        page: rel,
        kind: "fence",
        id,
        fence_index: idx,
        sources,
      });
    });
  }

  return { queries, errors };
}
