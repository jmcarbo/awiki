import { scanInbox } from "../lib/triage-inbox-scan.js";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-scan-"));
mkdirSync(join(root, "content"), { recursive: true });
mkdirSync(join(root, "raw/inbox/interactive"), { recursive: true });
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");

writeFileSync(
  join(root, "content/inbox.md"),
  `---
type: inbox
---

- 2026-04-27T12:00:00Z call dentist
- 2026-04-27T13:00:00Z buy milk
- 2026-04-27T14:00:00Z ~~struck~~

`,
);
writeFileSync(join(root, "raw/inbox/interactive/voice-001.txt"), "transcript");
writeFileSync(join(root, "raw/inbox/interactive/notes.md"), "## hi");

const items = scanInbox(root);

// Inbox lines: only the two non-strikethrough lines (~~text~~ is treated as
// processed-trash by phase 18a's triage.sh and skipped).
const inboxItems = items.filter((i) => i.source === "inbox.md");
assert.equal(inboxItems.length, 2);
for (const it of inboxItems) {
  assert.match(it.id, /^inbox-[0-9a-f]{10}-\d+$/);
  assert.equal(typeof it.text, "string");
  assert.match(it.captured_at, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/);
  assert.equal(typeof it.line_or_path, "number");
}
assert.equal(inboxItems[0].text, "call dentist");
assert.equal(inboxItems[1].text, "buy milk");

// File-shaped items.
const fileItems = items.filter((i) => i.source.startsWith("raw/inbox/"));
assert.equal(fileItems.length, 2);
for (const it of fileItems) {
  assert.match(it.id, /^file-[0-9a-f]{10}$/);
  assert.equal(typeof it.line_or_path, "string");
  assert.ok(it.line_or_path.startsWith("raw/inbox/interactive/"));
}

rmSync(root, { recursive: true, force: true });
console.log("triage-inbox-scan: ok");
