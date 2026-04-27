import { validateTriageArgs, OUTCOMES, ID_RE } from "../lib/triage-validate.js";
import { strict as assert } from "node:assert";

// Helper: short-form happy path.
function ok(id, outcome, params = {}) {
  const r = validateTriageArgs({ id, outcome, params });
  assert.equal(r.ok, true, `expected ok, got errors: ${JSON.stringify(r.errors)}`);
  return r.normalized;
}
function bad(id, outcome, params = {}) {
  const r = validateTriageArgs({ id, outcome, params });
  assert.equal(
    r.ok,
    false,
    `expected fail, got normalized: ${JSON.stringify(r.normalized)}`,
  );
  return r.errors;
}

// ID_RE export sanity.
assert.ok(ID_RE instanceof RegExp);

// OUTCOMES export sanity.
assert.deepEqual(
  [...OUTCOMES].sort(),
  ["act", "defer-scheduled", "do-now", "reference", "someday", "trash", "waiting"],
);

// id regex.
ok("inbox-abcdefghij-7", "trash");
ok("file-abcdefghij", "trash");
ok("a01", "trash");
ok("a~b1c2d3e4", "trash"); // recurrence-chain id with ~
bad("INBOX-abc-1", "trash");                    // case
bad("inbox-abc-1; rm -rf /", "trash");          // shell-meta
bad("a".repeat(33), "trash");                   // too long
bad("", "trash");
bad("foo bar", "trash");                        // space

// outcome enum.
for (const o of [
  "trash",
  "do-now",
  "act",
  "defer-scheduled",
  "waiting",
  "reference",
  "someday",
]) {
  // outcome=reference requires ref_slug + page_type, so test it with those.
  if (o === "reference") {
    ok("a01", o, { ref_slug: "memex", page_type: "topic" });
  } else if (o === "waiting") {
    // waiting is fine without wait_for in this validator (cross-field
    // check for wait_for is not part of this layer).
    ok("a01", o);
  } else {
    ok("a01", o);
  }
}
bad("a01", "TRASH");
bad("a01", "delete");
bad("a01", "");

// project_slug: underscore-prefix allowed.
ok("a01", "act", { project_slug: "_loose" });
ok("a01", "act", { project_slug: "_someday" });
ok("a01", "act", { project_slug: "renovate-kitchen" });
bad("a01", "act", { project_slug: "-bad" });           // leading hyphen
bad("a01", "act", { project_slug: "Bad" });            // uppercase
bad("a01", "act", { project_slug: "../etc" });         // traversal hint
bad("a01", "act", { project_slug: "a".repeat(65) });   // length

// context_slug: NO underscore prefix.
ok("a01", "act", { context_slug: "phone" });
bad("a01", "act", { context_slug: "_phone" });

// wait_for: same shape as context_slug.
ok("a01", "waiting", { wait_for: "alice" });
bad("a01", "waiting", { wait_for: "_alice" });

// ref_slug + page_type for reference outcome.
ok("a01", "reference", { ref_slug: "memex", page_type: "topic" });
bad("a01", "reference", { ref_slug: "memex", page_type: "agenda" }); // not in enum
bad("a01", "reference", { ref_slug: "memex" });                      // page_type missing
// page_type allowed only on reference outcome.
bad("a01", "act", { page_type: "topic" });

// due / defer date validation.
ok("a01", "defer-scheduled", { due: "2026-05-01" });
ok("a01", "defer-scheduled", { defer: "2026-12-31" });
bad("a01", "defer-scheduled", { due: "2026-02-30" });   // calendar invalid
bad("a01", "defer-scheduled", { due: "2026-13-01" });
bad("a01", "defer-scheduled", { due: "2026-00-15" });
bad("a01", "defer-scheduled", { due: "2026-1-1" });     // not zero-padded
bad("a01", "defer-scheduled", { due: "26-05-01" });
bad("a01", "defer-scheduled", { due: "" });

// additional properties rejected.
bad("a01", "act", { evil: "x" });

// missing params object normalizes to {}.
{
  const r = validateTriageArgs({ id: "a01", outcome: "trash" });
  assert.equal(r.ok, true);
  assert.deepEqual(r.normalized.params, {});
}

// non-object params rejected.
bad("a01", "act", "string");
bad("a01", "act", null);
bad("a01", "act", []);

// non-object args envelope rejected.
{
  const r = validateTriageArgs(null);
  assert.equal(r.ok, false);
}
{
  const r = validateTriageArgs("foo");
  assert.equal(r.ok, false);
}

console.log("triage-validate.js: ok");
