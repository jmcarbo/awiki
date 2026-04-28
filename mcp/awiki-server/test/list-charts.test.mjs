import { test } from "node:test";
import assert from "node:assert/strict";
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { listCharts } from "../lib/list-charts.js";

function setup() {
  const root = mkdtempSync(join(tmpdir(), "awiki-charts-"));
  mkdirSync(join(root, "content", "charts"), { recursive: true });
  mkdirSync(join(root, "content", "concepts"), { recursive: true });
  mkdirSync(join(root, "assets", "charts"), { recursive: true });
  return root;
}

test("listCharts returns empty on empty repo", () => {
  const root = setup();
  try {
    const r = listCharts(root);
    assert.deepEqual(r.charts, []);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("listCharts finds inline fences with fence_index", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "concepts", "p.md"), `---
type: concept
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`

\`\`\`vega-lite
{"mark":"line","data":{"name":"[[demo2]]"}}
\`\`\`
`);
    const r = listCharts(root);
    assert.equal(r.charts.length, 2);
    assert.equal(r.charts[0].fence_index, 0);
    assert.equal(r.charts[1].fence_index, 1);
    assert.equal(r.charts[0].engine, "vega-lite");
    assert.equal(r.charts[0].data_ref, "[[demo]]");
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("listCharts finds type:chart pages with slug", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "charts", "demo-bar.md"), `---
type: chart
chart_engine: vega-lite
chart_data: "[[demo]]"
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
`);
    const r = listCharts(root);
    assert.equal(r.charts.length, 1);
    assert.equal(r.charts[0].slug, "demo-bar");
    assert.equal(r.charts[0].fence_index, undefined);
  } finally { rmSync(root, { recursive: true, force: true }); }
});

test("listCharts links sidecar_path when SVG exists", () => {
  const root = setup();
  try {
    writeFileSync(join(root, "content", "charts", "demo.md"), `---
type: chart
chart_engine: vega-lite
---

\`\`\`vega-lite
{"mark":"bar","data":{"name":"[[demo]]"}}
\`\`\`
`);
    writeFileSync(join(root, "assets", "charts", "demo.svg"), "<svg/>");
    const r = listCharts(root);
    assert.equal(r.charts[0].sidecar_path, "assets/charts/demo.svg");
  } finally { rmSync(root, { recursive: true, force: true }); }
});
