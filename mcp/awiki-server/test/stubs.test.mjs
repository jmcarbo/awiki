// Tool-level test for the phase 18b stub tools (review_status,
// mark_review_done). Both must return {stub:true, message:/phase 19/}.

import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const SERVER = new URL("../index.js", import.meta.url).pathname;

function makeRoot() {
  const root = mkdtempSync(join(tmpdir(), "awiki-stubs-"));
  mkdirSync(join(root, ".awiki"), { recursive: true });
  writeFileSync(join(root, ".awiki/lock"), "");
  return root;
}

function call(cwd, name) {
  const req = JSON.stringify({
    jsonrpc: "2.0",
    id: 1,
    method: "tools/call",
    params: { name, arguments: {} },
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
  throw new Error(`no JSON-RPC response. stdout=${r.stdout} stderr=${r.stderr}`);
}

let pass = 0;
for (const name of ["review_status", "mark_review_done"]) {
  const root = makeRoot();
  const resp = call(root, name);
  assert.notEqual(resp.result.isError, true, `unexpected MCP error: ${resp.result.content[0].text}`);
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.stub, true);
  assert.match(payload.message, /phase 19/);
  rmSync(root, { recursive: true, force: true });
  console.log(`ok - ${name} returns stub payload`);
  pass++;
}
console.log(`# stubs: ${pass} passed`);
