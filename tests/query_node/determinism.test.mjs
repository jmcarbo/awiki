import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const SCRIPT = resolve("scripts/lib/query-determinism.py");

function check(sql) {
  try {
    execFileSync("python3", [SCRIPT], { input: sql, encoding: "utf-8" });
    return { ok: true };
  } catch (e) {
    return { ok: false, stderr: e.stderr?.toString?.() ?? "" };
  }
}

test("plain SELECT with ORDER BY passes", () => {
  assert.equal(check("SELECT id FROM trades ORDER BY id").ok, true);
});

test("missing outer ORDER BY rejected", () => {
  const r = check("SELECT id FROM trades");
  assert.equal(r.ok, false);
  assert.match(r.stderr, /ORDER BY/);
});

test("NOW() rejected", () => {
  const r = check("SELECT NOW() AS t FROM trades ORDER BY t");
  assert.equal(r.ok, false);
  assert.match(r.stderr, /NOW/);
});

test("CURRENT_DATE rejected", () => {
  const r = check("SELECT CURRENT_DATE FROM trades ORDER BY 1");
  assert.equal(r.ok, false);
});

test("RANDOM() rejected", () => {
  const r = check("SELECT RANDOM() AS r FROM trades ORDER BY r");
  assert.equal(r.ok, false);
  assert.match(r.stderr, /RANDOM/);
});

test("UUID() rejected", () => {
  const r = check("SELECT UUID() AS u FROM trades ORDER BY u");
  assert.equal(r.ok, false);
});

test("ORDER BY in inner subquery does not satisfy", () => {
  const r = check(
    "SELECT * FROM (SELECT id FROM trades ORDER BY id) x",
  );
  assert.equal(r.ok, false);
});

test("LIMIT without ORDER BY rejected", () => {
  const r = check("SELECT id FROM trades LIMIT 10");
  assert.equal(r.ok, false);
});
