import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";

const PLUGIN_NAME_RE = /^[a-z][a-z0-9-]*$/;

// Parse a YAML frontmatter block (between leading --- and the next --- on its
// own line). Returns a plain object. Only flat scalar/list values are
// supported — all manifest fields are scalar/list, no nested objects, so a
// minimal hand-rolled parser keeps us free of YAML library deps.
function parseFrontmatter(text) {
  if (!text.startsWith("---\n")) return null;
  const end = text.indexOf("\n---\n", 4);
  if (end < 0) return null;
  const block = text.slice(4, end);
  const out = {};
  let currentList = null;
  let currentKey = null;
  for (const raw of block.split("\n")) {
    if (raw.startsWith("  - ")) {
      if (currentList && currentKey) currentList.push(raw.slice(4).trim().replace(/^"(.*)"$/, "$1"));
      continue;
    }
    const m = raw.match(/^([a-z_][a-z0-9_]*):\s*(.*)$/);
    if (!m) continue;
    const [, key, val] = m;
    if (val === "") {
      currentKey = key;
      currentList = [];
      out[key] = currentList;
    } else if (val.startsWith("[") && val.endsWith("]")) {
      out[key] = val.slice(1, -1).split(",").map((s) => s.trim().replace(/^"(.*)"$/, "$1")).filter(Boolean);
      currentList = null;
    } else {
      const stripped = val.replace(/^"(.*)"$/, "$1");
      if (stripped === "null") out[key] = null;
      else if (/^-?\d+$/.test(stripped)) out[key] = parseInt(stripped, 10);
      else out[key] = stripped;
      currentList = null;
    }
  }
  return out;
}

export function listSynthPlugins(repoRoot) {
  const dir = join(repoRoot, "synthesis-plugins");
  let entries;
  try {
    entries = readdirSync(dir);
  } catch (e) {
    return { plugins: [], errors: [`cannot read synthesis-plugins/: ${e.message}`] };
  }

  const plugins = [];
  const errors = [];

  for (const entry of entries) {
    if (!entry.endsWith(".md")) continue;
    const name = entry.slice(0, -3);
    if (!PLUGIN_NAME_RE.test(name)) {
      errors.push(`plugin filename rejected: ${entry} (does not match ^[a-z][a-z0-9-]*$)`);
      continue;
    }
    const path = join(dir, entry);
    try {
      const text = readFileSync(path, "utf8");
      const fm = parseFrontmatter(text);
      if (!fm) {
        errors.push(`${entry}: missing or malformed frontmatter`);
        continue;
      }
      let missing = false;
      for (const required of ["name", "description", "output_type", "min_sources"]) {
        if (!(required in fm)) {
          errors.push(`${entry}: missing required field "${required}"`);
          missing = true;
        }
      }
      if (missing) continue;
      plugins.push({
        name: fm.name,
        description: fm.description,
        output_type: fm.output_type,
        output_subtype: fm.output_subtype ?? fm.name,
        version: fm.version ?? 1,
        min_sources: fm.min_sources,
        max_sources: fm.max_sources ?? null,
        max_evidence_total_words: fm.max_evidence_total_words ?? 500,
        required_sections: fm.required_sections ?? [],
        post_hook: fm.post_hook ?? null,
      });
    } catch (e) {
      errors.push(`${entry}: ${e.message}`);
    }
  }

  return { plugins, errors };
}
