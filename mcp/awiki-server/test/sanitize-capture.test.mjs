import { sanitizeCapture, SanitizeError } from "../lib/sanitize-capture.js";
import { strict as assert } from "node:assert";

// Happy path: clean text passes unchanged.
{
  const r = sanitizeCapture("call dentist about crown");
  assert.equal(r.text, "call dentist about crown");
  assert.deepEqual(r.applied, []);
}

// Wikilink neutralization (open + close).
{
  const r = sanitizeCapture("see [[secret-page]] for details");
  assert.equal(r.text, "see [ [secret-page] ] for details");
  assert.deepEqual(r.applied, ["wikilink-neutralized"]);
}

// Comment neutralization.
{
  const r = sanitizeCapture("foo <!-- BEGIN GENERATED --> bar");
  assert.equal(r.text, "foo < !-- BEGIN GENERATED --  > bar");
  assert.deepEqual(r.applied, ["comment-neutralized"]);
}

// Length truncation: text > 2000 -> truncated to 2000 chars + ellipsis.
{
  const long = "x".repeat(2500);
  const r = sanitizeCapture(long);
  assert.equal(r.text.length, 2001); // 2000 chars + 1 ellipsis char
  assert.equal(r.text.endsWith("…"), true);
  assert.deepEqual(r.applied, ["length-truncated"]);
}

// Block-ID-shape escape: ^abc123 -> \^abc123 anywhere it appears as a word.
{
  const r = sanitizeCapture("note ^abc123 reference");
  assert.equal(r.text, "note \\^abc123 reference");
  assert.deepEqual(r.applied, ["block-id-escaped"]);
}

// Multiple rules combine; applied list is order-preserving and deduped.
{
  const r = sanitizeCapture("[[link]] <!--c--> ^abc123");
  assert.equal(r.text, "[ [link] ] < !--c--  > \\^abc123");
  assert.deepEqual(
    r.applied.slice().sort(),
    ["block-id-escaped", "comment-neutralized", "wikilink-neutralized"],
  );
}

// Hard reject: embedded newline.
assert.throws(
  () => sanitizeCapture("line one\nline two"),
  (e) => e instanceof SanitizeError && e.kind === "embedded-newline",
);

// Hard reject: carriage return.
assert.throws(
  () => sanitizeCapture("foo\rbar"),
  (e) => e instanceof SanitizeError && e.kind === "embedded-newline",
);

// Hard reject: NUL byte.
assert.throws(
  () => sanitizeCapture("foo\x00bar"),
  (e) => e instanceof SanitizeError && e.kind === "control-char",
);

// Hard reject: other control chars (e.g. ESC).
assert.throws(
  () => sanitizeCapture("foo\x1bbar"),
  (e) => e instanceof SanitizeError && e.kind === "control-char",
);

// Tab -> space (NOT a hard reject; per spec table the parenthetical exempts it).
{
  const r = sanitizeCapture("foo\tbar");
  assert.equal(r.text, "foo bar");
  // Tab->space is a normalization, not a tracked sanitization.
  assert.deepEqual(r.applied, []);
}

// Hard reject: checkbox prefix at start of line.
assert.throws(
  () => sanitizeCapture("[ ] not a capture"),
  (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix",
);
assert.throws(
  () => sanitizeCapture("[/] also not"),
  (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix",
);
assert.throws(
  () => sanitizeCapture("[x] also not"),
  (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix",
);
assert.throws(
  () => sanitizeCapture("[?] also not"),
  (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix",
);
assert.throws(
  () => sanitizeCapture("[>] also not"),
  (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix",
);

// Checkbox prefix NOT at start of line is fine.
{
  const r = sanitizeCapture("text [ ] not at start");
  assert.equal(r.text, "text [ ] not at start");
  assert.deepEqual(r.applied, []);
}

console.log("sanitize-capture.js: 14/14 ok");
