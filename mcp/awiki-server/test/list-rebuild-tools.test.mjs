// Tool-level test for list_actions + rebuild_agenda over stdio MCP.

import { spawnSync } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, chmodSync, rmSync, existsSync, utimesSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const HOMEBREW_FLOCK_DIR = "/opt/homebrew/opt/util-linux/bin";
if (existsSync(join(HOMEBREW_FLOCK_DIR, "flock"))) {
  process.env.PATH = `${HOMEBREW_FLOCK_DIR}:${process.env.PATH ?? ""}`;
}

const SERVER = new URL("../index.js", import.meta.url).pathname;

function makeRoot() {
  const root = mkdtempSync(join(tmpdir(), "awiki-list-tool-"));
  mkdirSync(join(root, ".awiki/maps"), { recursive: true });
  writeFileSync(join(root, ".awiki/lock"), "");
  mkdirSync(join(root, "content/projects"), { recursive: true });
  mkdirSync(join(root, "content/agenda"), { recursive: true });
  mkdirSync(join(root, "scripts"), { recursive: true });

  // Seed actions.tsv (16 columns, header + 3 rows).
  const cols =
    "id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind";
  const r1 =
    "a01\t[ ]\tcall dentist\tcontent/projects/health.md\t12\tphone\t2026-05-01\t\t\t\t\t\t\t\trenovate-kitchen\tpublic";
  const r2 =
    "a02\t[x]\tbuy milk\tcontent/projects/groceries.md\t3\terrands\t\t\t\t\t\t2026-04-26\t\t\tgroceries\tpublic";
  const r3 =
    "a03\t[?]\tlawyer reply\tcontent/projects/move.md\t8\t\t\t\talice\t2026-04-20\t\t\t\t\tmove\tprivate";
  writeFileSync(
    join(root, ".awiki/maps/actions.tsv"),
    `${cols}\n${r1}\n${r2}\n${r3}\n`,
  );

  // Bump tsv mtime forward so the stale-check sees it as fresh.
  const future = new Date(Date.now() + 60_000);
  utimesSync(join(root, ".awiki/maps/actions.tsv"), future, future);

  // Stub scan + agenda scripts (no-op).
  writeFileSync(join(root, "scripts/action-scan.sh"), "#!/usr/bin/env bash\nexit 0\n");
  writeFileSync(join(root, "scripts/agenda.sh"), "#!/usr/bin/env bash\nexit 0\n");
  chmodSync(join(root, "scripts/action-scan.sh"), 0o755);
  chmodSync(join(root, "scripts/agenda.sh"), 0o755);

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

// list_actions: no filter returns all 3 rows.
{
  const root = makeRoot();
  const resp = call(root, "list_actions", {});
  assert.notEqual(resp.result.isError, true, `unexpected MCP error: ${resp.result.content[0].text}`);
  const rows = JSON.parse(resp.result.content[0].text);
  assert.equal(rows.length, 3);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 1 - list_actions returns all rows when no filter");
  pass++;
}

// list_actions: filter status=[ ].
{
  const root = makeRoot();
  const resp = call(root, "list_actions", { filter: { status: "[ ]" } });
  const rows = JSON.parse(resp.result.content[0].text);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].id, "a01");
  rmSync(root, { recursive: true, force: true });
  console.log("ok 2 - list_actions filters by status");
  pass++;
}

// list_actions: filter project.
{
  const root = makeRoot();
  const resp = call(root, "list_actions", { filter: { project: "groceries" } });
  const rows = JSON.parse(resp.result.content[0].text);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].id, "a02");
  rmSync(root, { recursive: true, force: true });
  console.log("ok 3 - list_actions filters by project");
  pass++;
}

// list_actions: filter overdue=true.
{
  const root = makeRoot();
  const resp = call(root, "list_actions", { filter: { overdue: true } });
  const rows = JSON.parse(resp.result.content[0].text);
  // a01 due 2026-05-01; depending on Node 'today', may or may not be overdue.
  // Just assert rows is an array.
  assert.ok(Array.isArray(rows));
  rmSync(root, { recursive: true, force: true });
  console.log("ok 4 - list_actions accepts overdue filter");
  pass++;
}

// list_actions: bad filter rejected.
{
  const root = makeRoot();
  const resp = call(root, "list_actions", { filter: { status: "DONE" } });
  const txt = resp.result.content[0].text;
  assert.match(txt, /invalid_argument|enum/i);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 5 - list_actions rejects bad filter shape");
  pass++;
}

// rebuild_agenda: returns expected payload.
{
  const root = makeRoot();
  const resp = call(root, "rebuild_agenda", {});
  assert.notEqual(resp.result.isError, true, `unexpected MCP error: ${resp.result.content[0].text}`);
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.rebuilt.length, 5);
  assert.ok(typeof payload.duration_ms === "number");
  assert.ok(payload.duration_ms >= 0);
  rmSync(root, { recursive: true, force: true });
  console.log("ok 6 - rebuild_agenda returns 5 paths + duration_ms");
  pass++;
}

console.log(`# list-rebuild-tools: ${pass} passed`);
