import { readdirSync, readFileSync, statSync, existsSync } from "node:fs";
import { join, relative, sep } from "node:path";

const FRONTMATTER_RE = /^---\s*\n([\s\S]*?)\n---\s*$/m;
const FENCE_RE = /```vega-lite\s*\n([\s\S]*?)\n```/g;
const NAME_RE = /"name"\s*:\s*"(\[\[[a-z0-9][a-z0-9-]*\]\])"/;

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

export function listCharts(repoRoot) {
  const charts = [];
  const errors = [];
  const contentDir = join(repoRoot, "content");
  if (!existsSync(contentDir)) return { charts, errors };

  for (const path of walk(contentDir)) {
    let text;
    try { text = readFileSync(path, "utf8"); } catch (e) {
      errors.push({ path, reason: String(e.message || e) });
      continue;
    }
    const fmMatch = FRONTMATTER_RE.exec(text);
    const fm = fmMatch ? fmMatch[1] : "";
    const isChartPage = /^type:\s*chart\s*$/m.test(fm);
    const fences = [...text.matchAll(FENCE_RE)];
    const slug = path.split(sep).pop().slice(0, -3);
    const rel = relative(repoRoot, path);

    fences.forEach((m, idx) => {
      const body = m[1];
      const refMatch = NAME_RE.exec(body);
      const dataRef = refMatch ? refMatch[1] : null;
      const isSingleChartPage = isChartPage && fences.length === 1;
      const cid = isSingleChartPage ? slug : `${slug}-fig${idx}`;
      const sidecarRel = `assets/charts/${cid}.svg`;
      const sidecarAbs = join(repoRoot, sidecarRel);
      const entry = {
        page: rel,
        engine: "vega-lite",
        data_ref: dataRef,
      };
      if (isSingleChartPage) {
        entry.slug = slug;
      } else {
        entry.fence_index = idx;
      }
      if (existsSync(sidecarAbs)) entry.sidecar_path = sidecarRel;
      charts.push(entry);
    });
  }
  return { charts, errors };
}
