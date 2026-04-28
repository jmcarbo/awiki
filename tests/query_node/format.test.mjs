import test from "node:test";
import assert from "node:assert/strict";
import { execFileSync } from "node:child_process";
import { resolve } from "node:path";

const SCRIPT = resolve("scripts/lib/query-format.py");

function fmt(rows) {
  return execFileSync("python3", [SCRIPT, "--format=md"], {
    input: JSON.stringify(rows),
    encoding: "utf-8",
  });
}

test("two-column table", () => {
  const out = fmt([{ a: 1, b: "x" }, { a: 2, b: "y" }]);
  assert.match(out, /\| a \| b \|/);
  assert.match(out, /\|---\|---\|/);
  assert.match(out, /\| 1 \| x \|/);
});

test("empty rows yields explicit empty marker", () => {
  const out = fmt([]);
  assert.match(out, /\(no rows\)/);
});

test("escapes pipe characters in cells", () => {
  const out = fmt([{ a: "x|y" }]);
  assert.match(out, /\| x\\\|y \|/);
});
