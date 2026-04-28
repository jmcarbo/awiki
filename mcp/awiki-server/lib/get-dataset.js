import { readFileSync, realpathSync } from "node:fs";
import { join, resolve } from "node:path";

const SLUG_RE = /^[a-z0-9][a-z0-9-]*$/;
const FRONTMATTER_RE = /^---\s*\n([\s\S]*?)\n---\s*$/m;
const FENCE_RE_FACTORY = (fmt) =>
  new RegExp("## Data[\\s\\S]*?```" + fmt + "\\s*\\n([\\s\\S]*?)\\n```", "m");

function parseScalar(text, key) {
  const re = new RegExp("^" + key + ":\\s*(.*)$", "m");
  const m = re.exec(text);
  if (!m) return null;
  let v = m[1].trim();
  if (v.startsWith('"') && v.endsWith('"')) v = v.slice(1, -1);
  return v;
}

function parseCsv(body, delimiter) {
  const lines = body.split("\n").filter((l) => l.length > 0);
  if (lines.length === 0) return [];
  const headers = lines[0].split(delimiter);
  const rows = [];
  for (let i = 1; i < lines.length; i++) {
    const cells = lines[i].split(delimiter);
    const row = {};
    headers.forEach((h, idx) => (row[h] = cells[idx] ?? ""));
    rows.push(row);
  }
  return rows;
}

function parseRows(text, format) {
  if (format === "csv") return parseCsv(text, ",");
  if (format === "tsv") return parseCsv(text, "\t");
  if (format === "dsv") {
    const first = text.split("\n")[0] || "";
    const d = [",", ";", "|", "\t"].find((c) => first.includes(c)) || ",";
    return parseCsv(text, d);
  }
  if (format === "json") {
    const data = JSON.parse(text);
    return Array.isArray(data) ? data : [data];
  }
  if (format === "topojson") {
    return [JSON.parse(text)];
  }
  return [];
}

export function getDataset(repoRoot, slug) {
  if (!SLUG_RE.test(slug)) return { error: "invalid_slug" };
  const page = join(repoRoot, "content", "datasets", `${slug}.md`);
  let text;
  try {
    text = readFileSync(page, "utf8");
  } catch {
    return { error: "not_found" };
  }
  const fmMatch = FRONTMATTER_RE.exec(text);
  if (!fmMatch) return { error: "no_frontmatter" };
  const fm = fmMatch[1];

  const storage = parseScalar(fm, "storage");
  const format = parseScalar(fm, "format");
  const dataPath = parseScalar(fm, "data_path");

  let body = "";
  if (storage === "inline") {
    const fenceMatch = FENCE_RE_FACTORY(format).exec(text);
    if (!fenceMatch) return { error: "no_data_fence" };
    body = fenceMatch[1];
  } else if (storage === "file") {
    if (!dataPath) return { error: "no_data_path" };
    const dataRootRaw = join(repoRoot, "data");
    const resolvedRaw = resolve(repoRoot, dataPath);
    // Check traversal on raw paths first (works even when target doesn't exist)
    let dataRoot;
    try {
      dataRoot = realpathSync(dataRootRaw);
    } catch {
      dataRoot = dataRootRaw;
    }
    let resolved;
    try {
      resolved = realpathSync(resolvedRaw);
    } catch {
      // realpathSync fails when path doesn't exist; fall back to raw resolved
      // path for the traversal check, then report missing if it passes
      if (!resolvedRaw.startsWith(dataRootRaw + "/") && resolvedRaw !== dataRootRaw) {
        return { error: "path_traversal" };
      }
      return { error: "data_file_missing" };
    }
    if (!resolved.startsWith(dataRoot + "/") && resolved !== dataRoot) {
      return { error: "path_traversal" };
    }
    body = readFileSync(resolved, "utf8");
  } else {
    return { error: "bad_storage" };
  }

  const rows = parseRows(body, format);
  return {
    slug,
    frontmatter: fm,
    storage,
    format,
    sample_rows: rows.slice(0, 20),
    total_rows: rows.length,
    data_path: dataPath || null,
  };
}
