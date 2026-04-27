import { spawnSync } from "node:child_process";
import { mkdtempSync, cpSync, writeFileSync, rmSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { strict as assert } from "node:assert";
import { fillGoodBody } from "./util/fill-good-body.mjs";

const HERE = new URL(".", import.meta.url).pathname;
const REPO = resolve(HERE, "../../..");
const SERVER = join(REPO, "mcp/awiki-server/index.js");

// Helper: run one MCP tools/call request through the server via stdio and
// return the parsed result.
function callTool(cwd, name, args) {
  const req = JSON.stringify({
    jsonrpc: "2.0", id: 1,
    method: "tools/call",
    params: { name, arguments: args },
  });
  const r = spawnSync("node", [SERVER], {
    input: req + "\n",
    cwd,
    encoding: "utf8",
  });
  // Server emits one JSON-RPC response per line. Pick the response with id=1.
  const lines = (r.stdout ?? "").split("\n").filter(Boolean);
  for (const line of lines) {
    try {
      const parsed = JSON.parse(line);
      if (parsed.id === 1) return parsed;
    } catch {}
  }
  throw new Error(`no JSON-RPC response found. stdout=${r.stdout} stderr=${r.stderr}`);
}

// Each test gets a clean temp wiki copy.
function makeWiki() {
  const dir = mkdtempSync(join(tmpdir(), "awiki-mcp-"));
  // Copy the synth fixture content + .awiki.
  cpSync(join(REPO, "tests/fixtures/wiki-synth"), dir, { recursive: true });
  // Copy the synthesis-plugins/ directory and scripts/ from the live repo.
  cpSync(join(REPO, "synthesis-plugins"), join(dir, "synthesis-plugins"), { recursive: true });
  cpSync(join(REPO, "scripts"), join(dir, "scripts"), { recursive: true });
  return dir;
}

let pass = 0;
const total = 8;

// --- Test 1: list_synth_plugins returns 4 manifests ---
{
  const wiki = makeWiki();
  const res = callTool(wiki, "list_synth_plugins", {});
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.plugins.length, 4, `expected 4 plugins, got ${payload.plugins.length}`);
  const names = payload.plugins.map((p) => p.name).sort();
  assert.deepEqual(names, ["briefing", "mindmap", "study-guide", "timeline"]);
  assert.equal(payload.errors.length, 0);
  rmSync(wiki, { recursive: true });
  console.log("ok 1 - list_synth_plugins returns 4 manifests");
  pass++;
}

// --- Test 2: synthesize() happy path ---
{
  const wiki = makeWiki();
  const res = callTool(wiki, "synthesize", {
    plugin: "briefing",
    scope_descriptor: { tag: "memex" },
    topic_slug: "memex-test",
  });
  const payload = JSON.parse(res.result.content[0].text);
  assert.ok(payload.prompt_bundle, `expected prompt_bundle in payload, got: ${JSON.stringify(payload).slice(0, 200)}`);
  assert.ok(Array.isArray(payload.resolved_slugs), "expected resolved_slugs array");
  assert.ok(payload.target_path.endsWith("memex-test-briefing.md"));
  rmSync(wiki, { recursive: true });
  console.log("ok 2 - synthesize happy path");
  pass++;
}

// --- Test 3: synthesize() with plugin: "../etc/passwd" rejected by regex ---
{
  const wiki = makeWiki();
  const res = callTool(wiki, "synthesize", {
    plugin: "../etc/passwd",
    scope_descriptor: { tag: "memex" },
    topic_slug: "test",
  });
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.error, "invalid_argument");
  assert.equal(payload.field, "plugin");
  rmSync(wiki, { recursive: true });
  console.log("ok 3 - plugin path-traversal rejected by regex");
  pass++;
}

// --- Test 4: synthesize() with bad topic_slug rejected ---
{
  const wiki = makeWiki();
  const res = callTool(wiki, "synthesize", {
    plugin: "briefing",
    scope_descriptor: { tag: "memex" },
    topic_slug: "-evil",
  });
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.error, "invalid_argument");
  assert.equal(payload.field, "topic_slug");
  rmSync(wiki, { recursive: true });
  console.log("ok 4 - bad topic_slug rejected");
  pass++;
}

// --- Test 5: synthesize() with malformed scope_descriptor rejected by schema ---
{
  const wiki = makeWiki();
  const res = callTool(wiki, "synthesize", {
    plugin: "briefing",
    scope_descriptor: { tag: "memex", evil: "x" },  // additionalProperties violation
    topic_slug: "test",
  });
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.error, "invalid_argument");
  assert.equal(payload.field, "scope_descriptor");
  assert.ok(payload.reasons.some((r) => r.includes("additional property")));
  rmSync(wiki, { recursive: true });
  console.log("ok 5 - malformed scope_descriptor rejected");
  pass++;
}

// --- Test 6: synthesize() rejects symlink-swap manifest ---
{
  const wiki = makeWiki();
  // Replace synthesis-plugins/briefing.md with a symlink pointing outside.
  rmSync(join(wiki, "synthesis-plugins/briefing.md"));
  const target = join(wiki, "DECOY.md");
  writeFileSync(target, "decoy");
  // Make the symlink relative-up so its realpath escapes synthesis-plugins/.
  symlinkSync("../DECOY.md", join(wiki, "synthesis-plugins/briefing.md"));
  const res = callTool(wiki, "synthesize", {
    plugin: "briefing",
    scope_descriptor: { tag: "memex" },
    topic_slug: "test",
  });
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.error, "invalid_plugin");
  assert.ok(
    payload.reason.includes("escape") || payload.reason.includes("not a regular file"),
    `expected escape error, got ${payload.reason}`,
  );
  rmSync(wiki, { recursive: true });
  console.log("ok 6 - symlink-swap manifest rejected by realpath");
  pass++;
}

// --- Test 7: finalize_synthesis happy path ---
{
  const wiki = makeWiki();
  // First scaffold a page.
  const synthRes = callTool(wiki, "synthesize", {
    plugin: "briefing",
    scope_descriptor: { slugs: ["s1", "s2", "s3"] },
    topic_slug: "memex-fin",
  });
  const synthPayload = JSON.parse(synthRes.result.content[0].text);
  assert.ok(synthPayload.target_path, `synthesize failed: ${JSON.stringify(synthPayload)}`);
  // Hand-fill the page between the markers with a known-good body.
  const pagePath = join(wiki, "content/synthesis/memex-fin-briefing.md");
  fillGoodBody(pagePath);
  const res = callTool(wiki, "finalize_synthesis", { topic_slug: "memex-fin-briefing" });
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.ok, true, `expected ok, got: ${JSON.stringify(payload).slice(0, 500)}`);
  rmSync(wiki, { recursive: true });
  console.log("ok 7 - finalize_synthesis happy path");
  pass++;
}

// --- Test 8: finalize_synthesis bad topic_slug rejected ---
{
  const wiki = makeWiki();
  const res = callTool(wiki, "finalize_synthesis", { topic_slug: "-evil" });
  const payload = JSON.parse(res.result.content[0].text);
  assert.equal(payload.error, "invalid_argument");
  rmSync(wiki, { recursive: true });
  console.log("ok 8 - finalize_synthesis bad topic_slug rejected");
  pass++;
}

console.log(`# ${pass}/${total} tests passed`);
if (pass !== total) process.exit(1);
