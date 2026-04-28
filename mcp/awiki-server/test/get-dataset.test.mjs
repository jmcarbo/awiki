import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, mkdirSync, rmSync, symlinkSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { getDataset } from "../lib/get-dataset.js";

function setup() {
  const root = mkdtempSync(join(tmpdir(), "awiki-data-"));
  mkdirSync(join(root, "content", "datasets"), { recursive: true });
  mkdirSync(join(root, "data"), { recursive: true });
  return root;
}

test("get_dataset rejects bad slug", () => {
  const root = setup();
  try {
    const r = getDataset(root, "Bad Slug");
    assert.equal(r.error, "invalid_slug");
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("get_dataset returns sample rows for inline csv", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "datasets", "us-pop.md"), `---
title: "us-pop"
type: dataset
storage: inline
format: csv
rows: 3
---

## Data

\`\`\`csv
year,pop
2020,331
2021,333
2022,335
\`\`\`
`);
    const r = getDataset(root, "us-pop");
    assert.equal(r.slug, "us-pop");
    assert.equal(r.total_rows, 3);
    assert.deepEqual(r.sample_rows[0], { year: "2020", pop: "331" });
    assert.equal(r.sample_rows.length, 3);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("get_dataset returns sample rows for file storage", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "data", "us-pop.csv"), "year,pop\n2020,331\n2021,333\n");
    writeFileSync(join(root, "content", "datasets", "us-pop.md"), `---
type: dataset
storage: file
format: csv
rows: 2
data_path: data/us-pop.csv
---

## Provenance
`);
    const r = getDataset(root, "us-pop");
    assert.equal(r.total_rows, 2);
    assert.equal(r.sample_rows.length, 2);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("get_dataset rejects path traversal in data_path", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "datasets", "evil.md"), `---
type: dataset
storage: file
format: csv
rows: 0
data_path: ../../../etc/passwd
---

## Provenance
`);
    const r = getDataset(root, "evil");
    assert.equal(r.error, "path_traversal");
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("get_dataset returns error when slug missing", () => {
  const root = setup();
  try {
    const r = getDataset(root, "nope");
    assert.equal(r.error, "not_found");
  } finally { rmSync(root, { recursive: true, force: true }); }
});
