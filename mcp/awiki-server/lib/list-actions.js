// Read .awiki/maps/actions.tsv (phase-17 scanner output) and apply an
// optional filter. Caller is responsible for taking the lock and (if
// needed) re-running action-scan.sh first.

import { readFileSync, statSync, readdirSync } from "node:fs";
import { join } from "node:path";

const TSV_COLUMNS = [
  "id", "status", "text", "file", "line", "context", "due", "defer",
  "wait", "since", "every", "done", "priority", "est", "project", "source_kind",
];

export function loadActionsTsv(repoRoot) {
  const path = join(repoRoot, ".awiki/maps/actions.tsv");
  let text;
  try {
    text = readFileSync(path, "utf8");
  } catch (e) {
    if (e.code === "ENOENT") return [];
    throw e;
  }
  const rows = [];
  for (const line of text.split("\n")) {
    if (!line || line.startsWith("#")) continue;
    const cells = line.split("\t");
    if (cells.length < TSV_COLUMNS.length) continue;
    // Skip header row (first cell is literally "id").
    if (cells[0] === "id" && cells[1] === "status") continue;
    const row = {};
    for (let i = 0; i < TSV_COLUMNS.length; i++) {
      row[TSV_COLUMNS[i]] = cells[i] ?? "";
    }
    rows.push(row);
  }
  return rows;
}

export function applyFilter(rows, filter = {}, today = new Date().toISOString().slice(0, 10)) {
  let out = rows;
  if (filter.status) out = out.filter((r) => r.status === filter.status);
  if (filter.context) out = out.filter((r) => r.context === filter.context);
  if (filter.project) out = out.filter((r) => r.project === filter.project);
  if (filter.due_before) out = out.filter((r) => r.due && r.due < filter.due_before);
  if (filter.wait) out = out.filter((r) => r.wait === filter.wait);
  if (filter.overdue === true) {
    out = out.filter((r) => r.due && r.due < today && r.status === "[ ]");
  }
  return out;
}

// Decide whether the TSV is stale relative to content/**/*.md.
// Stale if any markdown file's mtime > tsv mtime, or the TSV is missing.
export function isTsvStale(repoRoot) {
  const tsv = join(repoRoot, ".awiki/maps/actions.tsv");
  let tsvMtime;
  try {
    tsvMtime = statSync(tsv).mtimeMs;
  } catch {
    return true;
  }
  const contentDir = join(repoRoot, "content");
  function walk(dir) {
    let newest = 0;
    let entries;
    try {
      entries = readdirSync(dir, { withFileTypes: true });
    } catch {
      return 0;
    }
    for (const ent of entries) {
      const p = join(dir, ent.name);
      if (ent.isDirectory()) {
        newest = Math.max(newest, walk(p));
      } else if (ent.isFile() && p.endsWith(".md")) {
        let st;
        try {
          st = statSync(p);
        } catch {
          continue;
        }
        if (st.mtimeMs > newest) newest = st.mtimeMs;
      }
    }
    return newest;
  }
  return walk(contentDir) > tsvMtime;
}
