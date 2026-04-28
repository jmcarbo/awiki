import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const SCRIPT = resolve("scripts/lib/query-resolve.py");

function refs(sql) {
  const out = execFileSync("python3", [SCRIPT], {
    input: sql,
    encoding: "utf-8",
  });
  return out.trim().split("\n").filter(Boolean).sort();
}

test("bare FROM ref", () => {
  assert.deepEqual(refs("SELECT * FROM trades"), ["trades"]);
});

test("quoted identifier", () => {
  assert.deepEqual(refs('SELECT * FROM "trades"'), ["trades"]);
});

test("JOIN", () => {
  assert.deepEqual(
    refs("SELECT * FROM trades t JOIN regions r ON t.region_id=r.id"),
    ["regions", "trades"],
  );
});

test("CTE alias not treated as dataset", () => {
  assert.deepEqual(
    refs(
      "WITH agg AS (SELECT category FROM trades) SELECT * FROM agg",
    ),
    ["trades"],
  );
});

test("subquery", () => {
  assert.deepEqual(
    refs("SELECT * FROM (SELECT id FROM trades) x"),
    ["trades"],
  );
});

test("schema-qualified main.<slug>", () => {
  assert.deepEqual(refs("SELECT * FROM main.trades"), ["trades"]);
});

test("multiple statements", () => {
  assert.deepEqual(
    refs("SELECT * FROM trades; SELECT * FROM regions;"),
    ["regions", "trades"],
  );
});

test("ATTACH rejected as ref (caller will handle separately)", () => {
  assert.deepEqual(refs("SELECT * FROM trades"), ["trades"]);
});
