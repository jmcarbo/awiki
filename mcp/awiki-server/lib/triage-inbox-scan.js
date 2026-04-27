// Read-only scan of inbox.md + raw/inbox/interactive/. Mints IDs via
// inbox-id.js. Skips strikethrough lines (~~text~~) which represent
// trashed-but-not-pruned captures left behind by the trash outcome.
//
// The MCP boundary is responsible for taking flock -s before calling this
// helper (see triage_inbox dispatch in index.js).

import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { mintInboxId, mintFileId } from "./inbox-id.js";

const INBOX_LINE_RE = /^- (\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z) (.*)$/;
const STRIKE_RE = /^~~.*~~$/;

function scanInboxFile(repoRoot) {
  const path = join(repoRoot, "content/inbox.md");
  let text;
  try {
    text = readFileSync(path, "utf8");
  } catch {
    return [];
  }
  const items = [];
  const lines = text.split("\n");
  let inFrontmatter = false;
  let seenFmEnd = false;
  for (let i = 0; i < lines.length; i++) {
    const lineno = i + 1;
    const raw = lines[i];
    if (lineno === 1 && raw === "---") {
      inFrontmatter = true;
      continue;
    }
    if (inFrontmatter && raw === "---") {
      inFrontmatter = false;
      seenFmEnd = true;
      continue;
    }
    if (inFrontmatter) continue;
    if (!seenFmEnd) continue; // Pre-frontmatter junk; skip.
    const m = INBOX_LINE_RE.exec(raw);
    if (!m) continue;
    const [, capturedAt, body] = m;
    if (STRIKE_RE.test(body)) continue;
    items.push({
      id: mintInboxId(raw, lineno),
      source: "inbox.md",
      line_or_path: lineno,
      text: body,
      captured_at: capturedAt,
    });
  }
  return items;
}

function scanInteractiveFiles(repoRoot) {
  const dir = join(repoRoot, "raw/inbox/interactive");
  let entries;
  try {
    entries = readdirSync(dir, { withFileTypes: true });
  } catch {
    return [];
  }
  const items = [];
  for (const ent of entries) {
    if (!ent.isFile()) continue;
    const abs = join(dir, ent.name);
    const rel = relative(repoRoot, abs);
    let st;
    try {
      st = statSync(abs);
    } catch {
      continue;
    }
    items.push({
      id: mintFileId(rel),
      source: rel.split("/").slice(0, -1).join("/") + "/",
      line_or_path: rel,
      text: ent.name,
      captured_at: new Date(st.mtimeMs).toISOString().replace(/\.\d{3}Z$/, "Z"),
    });
  }
  return items;
}

export function scanInbox(repoRoot) {
  return [...scanInboxFile(repoRoot), ...scanInteractiveFiles(repoRoot)];
}
