import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { listDatasets } from "../lib/list-datasets.js";

function setupRepo() {
  const root = mkdtempSync(join(tmpdir(), "awiki-data-"));
  mkdirSync(join(root, "content", "datasets"), { recursive: true });
  return root;
}

test("listDatasets returns empty array on empty repo", () => {
  const root = setupRepo();
  try {
    const result = listDatasets(root);
    assert.deepEqual(result, { datasets: [], errors: [] });
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("listDatasets enumerates dataset pages", () => {
  const root = setupRepo();
  try {
    writeFileSync(join(root, "content", "datasets", "us-pop.md"), `---
title: "us-pop"
type: dataset
storage: inline
format: csv
rows: 3
tags: [population]
last_updated: 2026-04-28
---

## Data

\`\`\`csv
a,b
1,2
3,4
5,6
\`\`\`
`);
    const result = listDatasets(root);
    assert.equal(result.datasets.length, 1);
    const d = result.datasets[0];
    assert.equal(d.slug, "us-pop");
    assert.equal(d.format, "csv");
    assert.equal(d.storage, "inline");
    assert.equal(d.rows, 3);
    assert.deepEqual(d.tags, ["population"]);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});

test("listDatasets skips non-dataset markdown in the directory", () => {
  const root = setupRepo();
  try {
    writeFileSync(join(root, "content", "datasets", "_index.md"), `---
type: section-index
---
`);
    const result = listDatasets(root);
    assert.equal(result.datasets.length, 0);
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
});
