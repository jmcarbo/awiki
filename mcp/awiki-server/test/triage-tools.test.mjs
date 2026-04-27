// Tool-level test for triage_inbox + triage_apply over stdio MCP.

import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, chmodSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

// macOS Homebrew util-linux ships flock at /opt/homebrew/opt/util-linux/bin/flock.
const HOMEBREW_FLOCK_DIR = "/opt/homebrew/opt/util-linux/bin";
if (existsSync(join(HOMEBREW_FLOCK_DIR, "flock"))) {
  process.env.PATH = `${HOMEBREW_FLOCK_DIR}:${process.env.PATH ?? ""}`;
}

const SERVER = new URL("../index.js", import.meta.url).pathname;

function makeRoot() {
  const root = mkdtempSync(join(tmpdir(), "awiki-triage-tool-"));
  mkdirSync(join(root, ".awiki"), { recursive: true });
  writeFileSync(join(root, ".awiki/lock"), "");
  mkdirSync(join(root, "content/projects"), { recursive: true });
  mkdirSync(join(root, "content/contexts"), { recursive: true });
  writeFileSync(
    join(root, "content/inbox.md"),
    `---
type: inbox
---

- 2026-04-27T12:00:00Z call dentist
- 2026-04-27T13:00:00Z buy milk
`,
  );
  mkdirSync(join(root, "scripts"), { recursive: true });
  // Stub triage.sh: emits a deterministic TRIAGE-RESULT trailer.
  writeFileSync(
    join(root, "scripts/triage.sh"),
    `#!/usr/bin/env bash
set -euo pipefail
id="$1"; shift
outcome="$1"; shift
echo "scripts/triage.sh stub: id=$id outcome=$outcome args=$*" >&2
printf 'TRIAGE-RESULT|{"actions_taken":["%s"],"created_pages":[],"updated_pages":["content/inbox.md"]}\\n' "$outcome"
`,
  );
  chmodSync(join(root, "scripts/triage.sh"), 0o755);
  return root;
}

function call(cwd, name, args) {
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
    env: { ...process.env },
  });
  const lines = (r.stdout ?? "").split("\n").filter(Boolean);
  for (const line of lines) {
    try {
      const parsed = JSON.parse(line);
      if (parsed.id === 1) return parsed;
    } catch {}
  }
  throw new Error(`no JSON-RPC response. stdout=${r.stdout} stderr=${r.stderr}`);
}

let pass = 0;

// triage_inbox returns the two open lines.
{
  const root = makeRoot();
  const resp = call(root, "triage_inbox", {});
  assert.notEqual(resp.result.isError, true, `unexpected MCP error: ${resp.result.content[0].text}`);
  const items = JSON.parse(resp.result.content[0].text);
  assert.equal(items.length, 2);
  for (const it of items) assert.match(it.id, /^inbox-[0-9a-f]{10}-\d+$/);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 1 - triage_inbox lists open captures");
  pass++;
}

// triage_apply happy path.
{
  const root = makeRoot();
  const r1 = call(root, "triage_inbox", {});
  const items = JSON.parse(r1.result.content[0].text);
  const id = items[0].id;
  const r2 = call(root, "triage_apply", { id, outcome: "trash" });
  assert.notEqual(r2.result.isError, true, `unexpected MCP error: ${r2.result.content[0].text}`);
  const payload = JSON.parse(r2.result.content[0].text);
  assert.equal(payload.ok, true);
  assert.deepEqual(payload.actions_taken, ["trash"]);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 2 - triage_apply happy path");
  pass++;
}

// triage_apply: regex rejects bad id.
{
  const root = makeRoot();
  const resp = call(root, "triage_apply", { id: "BAD;rm -rf /", outcome: "trash" });
  // SDK input-schema rejects (JSON-RPC error) OR validator rejects (ok:false).
  const isErr =
    resp.error !== undefined ||
    (resp.result &&
      (resp.result.isError === true ||
        JSON.parse(resp.result.content[0].text).ok === false));
  assert.ok(isErr, `expected error, got ${JSON.stringify(resp)}`);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 3 - triage_apply rejects bad id");
  pass++;
}

// triage_apply: outcome enum reject.
{
  const root = makeRoot();
  const r1 = call(root, "triage_inbox", {});
  const items = JSON.parse(r1.result.content[0].text);
  const id = items[0].id;
  const resp = call(root, "triage_apply", { id, outcome: "DELETE" });
  const isErr =
    resp.error !== undefined ||
    (resp.result &&
      (resp.result.isError === true ||
        JSON.parse(resp.result.content[0].text).ok === false));
  assert.ok(isErr, `expected error, got ${JSON.stringify(resp)}`);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 4 - triage_apply rejects bad outcome");
  pass++;
}

// triage_apply: invalid date rejected.
{
  const root = makeRoot();
  const r1 = call(root, "triage_inbox", {});
  const items = JSON.parse(r1.result.content[0].text);
  const id = items[0].id;
  const resp = call(root, "triage_apply", {
    id,
    outcome: "defer-scheduled",
    params: { defer: "2026-02-30" },
  });
  const txt = resp.result.content[0].text;
  assert.match(txt, /not a valid ISO calendar date|invalid/i);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 5 - triage_apply rejects invalid calendar date");
  pass++;
}

// triage_apply: project_slug "../etc" fails the params regex layer.
{
  const root = makeRoot();
  const r1 = call(root, "triage_inbox", {});
  const items = JSON.parse(r1.result.content[0].text);
  const id = items[1].id;
  const resp = call(root, "triage_apply", {
    id,
    outcome: "act",
    params: { project_slug: "../etc" },
  });
  const txt = resp.result.content[0].text;
  assert.match(txt, /must match|invalid|pattern/i);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 6 - triage_apply rejects traversal-shaped project_slug");
  pass++;
}

// triage_apply: stale-id when inbox edited between calls.
{
  const root = makeRoot();
  const r1 = call(root, "triage_inbox", {});
  const items = JSON.parse(r1.result.content[0].text);
  const milk = items.find((i) => i.text === "buy milk");
  assert.ok(milk, "buy milk line must be present");
  // Edit inbox so the line at the captured lineno no longer matches.
  writeFileSync(
    join(root, "content/inbox.md"),
    `---
type: inbox
---

- 2026-04-27T13:00:00Z buy bread
`,
  );
  const r2 = call(root, "triage_apply", { id: milk.id, outcome: "trash" });
  const payload = JSON.parse(r2.result.content[0].text);
  assert.equal(payload.ok, false);
  assert.equal(payload.stale_id, true);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 7 - triage_apply detects stale_id via TOCTOU re-verify");
  pass++;
}

console.log(`# triage-tools: ${pass} passed`);
