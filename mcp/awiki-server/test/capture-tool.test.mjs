// Tool-level test: drive the MCP server over stdio with a tools/call request
// and assert the response shape. Uses a tmpdir repo with .awiki/lock,
// content/inbox.md, and scripts/capture.sh as a stub that just appends.

import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, chmodSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

// macOS Homebrew util-linux ships flock at /opt/homebrew/opt/util-linux/bin/flock.
// Linux tends to have it on PATH already. Prepend the brew prefix so both
// platforms resolve `flock` the same way (capture.sh stub does not call flock,
// but we keep this for parity with sibling tool tests).
const HOMEBREW_FLOCK_DIR = "/opt/homebrew/opt/util-linux/bin";
if (existsSync(join(HOMEBREW_FLOCK_DIR, "flock"))) {
  process.env.PATH = `${HOMEBREW_FLOCK_DIR}:${process.env.PATH ?? ""}`;
}

const SERVER = new URL("../index.js", import.meta.url).pathname;

function makeRoot() {
  const root = mkdtempSync(join(tmpdir(), "awiki-capture-tool-"));
  mkdirSync(join(root, ".awiki"), { recursive: true });
  writeFileSync(join(root, ".awiki/lock"), "");
  mkdirSync(join(root, "content"), { recursive: true });
  writeFileSync(join(root, "content/inbox.md"), "---\ntype: inbox\n---\n\n");
  mkdirSync(join(root, "scripts"), { recursive: true });
  // Stub capture.sh: reads text from stdin and appends to content/inbox.md
  // with a fixed timestamp so the test is deterministic.
  writeFileSync(
    join(root, "scripts/capture.sh"),
    `#!/usr/bin/env bash
set -euo pipefail
text="$(cat)"
printf -- '- 2026-04-27T12:00:00Z %s\\n' "$text" >> content/inbox.md
`,
  );
  chmodSync(join(root, "scripts/capture.sh"), 0o755);
  return root;
}

function callTool(cwd, name, args) {
  const req = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "tools/call",
    params: { name, arguments: args },
  });
  const r = spawnSync("node", [SERVER], {
    input: req + "\n",
    cwd,
    encoding: "utf8",
  });
  const lines = (r.stdout ?? "").split("\n").filter(Boolean);
  for (const line of lines) {
    try {
      const parsed = JSON.parse(line);
      if (parsed.id === 1) return parsed;
    } catch {}
  }
  throw new Error(`no JSON-RPC response found. stdout=${r.stdout} stderr=${r.stderr}`);
}

let pass = 0;

// Happy path.
{
  const root = makeRoot();
  const resp = callTool(root, "capture", { text: "call dentist" });
  assert.equal(resp.error, undefined, `unexpected JSON-RPC error: ${JSON.stringify(resp.error)}`);
  assert.notEqual(resp.result.isError, true, `unexpected MCP error: ${resp.result.content[0].text}`);
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.appended, true);
  assert.deepEqual(payload.sanitizations_applied, []);
  assert.match(payload.timestamp, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/);
  assert.match(payload.line, /^- \d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z call dentist$/);
  assert.match(readFileSync(join(root, "content/inbox.md"), "utf8"), /call dentist/);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 1 - capture happy path");
  pass++;
}

// Wikilink neutralized.
{
  const root = makeRoot();
  const resp = callTool(root, "capture", { text: "see [[secret]] page" });
  const payload = JSON.parse(resp.result.content[0].text);
  assert.deepEqual(payload.sanitizations_applied, ["wikilink-neutralized"]);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 2 - capture wikilink-neutralized");
  pass++;
}

// Hard reject: embedded newline returns MCP error.
{
  const root = makeRoot();
  const resp = callTool(root, "capture", { text: "line one\nline two" });
  assert.equal(resp.result.isError, true);
  assert.match(resp.result.content[0].text, /embedded-newline|newline/);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 3 - capture rejects embedded newline");
  pass++;
}

// Hard reject: checkbox prefix.
{
  const root = makeRoot();
  const resp = callTool(root, "capture", { text: "[ ] not a capture" });
  assert.equal(resp.result.isError, true);
  assert.match(resp.result.content[0].text, /checkbox/);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 4 - capture rejects checkbox prefix");
  pass++;
}

// Type rejection: non-string text.
{
  const root = makeRoot();
  const resp = callTool(root, "capture", { text: 42 });
  // The MCP SDK rejects with a JSON-RPC error (input schema violation),
  // OR the dispatch surfaces an MCP-level error. Either is acceptable.
  const isErr =
    resp.error !== undefined ||
    (resp.result && resp.result.isError === true);
  assert.ok(isErr, `expected error for non-string text, got ${JSON.stringify(resp)}`);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 5 - capture rejects non-string text");
  pass++;
}

console.log(`# capture-tool: ${pass} passed`);
