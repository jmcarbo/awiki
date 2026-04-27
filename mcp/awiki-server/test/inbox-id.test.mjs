import { mintInboxId, mintFileId, parseInboxId, verifyInboxId } from "../lib/inbox-id.js";
import { strict as assert } from "node:assert";
import { createHash } from "node:crypto";

// Helper: compute the expected hash the same way the lib should.
function sha10(s) {
  return createHash("sha1").update(s, "utf8").digest("hex").slice(0, 10);
}

// mintInboxId: returns inbox-<sha1[:10]>-<lineno>.
{
  const line = "- 2026-04-27T12:34:56Z call dentist";
  const id = mintInboxId(line, 7);
  const expectedHash = sha10(line);
  assert.equal(id, `inbox-${expectedHash}-7`);
}

// mintInboxId: lineno must be a positive integer.
assert.throws(() => mintInboxId("- foo", 0));
assert.throws(() => mintInboxId("- foo", -1));
assert.throws(() => mintInboxId("- foo", 1.5));
assert.throws(() => mintInboxId("- foo", "1"));

// mintInboxId: line must be a string.
assert.throws(() => mintInboxId(null, 1));
assert.throws(() => mintInboxId(123, 1));

// Two lines with identical bytes but different linenos produce different IDs.
{
  const line = "- 2026-04-27T12:34:56Z duplicate";
  assert.notEqual(mintInboxId(line, 5), mintInboxId(line, 9));
}

// mintFileId: file-<sha1(relpath)[:10]>.
{
  const id = mintFileId("raw/inbox/interactive/voice-001.txt");
  assert.equal(id, `file-${sha10("raw/inbox/interactive/voice-001.txt")}`);
}

// parseInboxId: round-trip.
{
  const line = "- 2026-04-27T12:34:56Z call dentist";
  const id = mintInboxId(line, 7);
  const parsed = parseInboxId(id);
  assert.deepEqual(parsed, { kind: "inbox", hash: sha10(line), lineno: 7 });
}

// parseInboxId: file-shaped.
{
  const id = mintFileId("raw/inbox/interactive/voice-001.txt");
  const parsed = parseInboxId(id);
  assert.deepEqual(parsed, {
    kind: "file",
    hash: sha10("raw/inbox/interactive/voice-001.txt"),
  });
}

// parseInboxId: action-line block-ID (no inbox-/file- prefix).
{
  const parsed = parseInboxId("a01");
  assert.deepEqual(parsed, { kind: "block", hash: "a01" });
}

// parseInboxId: rejects malformed.
assert.equal(parseInboxId("inbox-toolong-12345-1"), null); // hash > 10 chars
assert.equal(parseInboxId("inbox-abc-1"), null);           // hash < 10 chars
assert.equal(parseInboxId("inbox-abcdefghij-x"), null);    // lineno not numeric
assert.equal(parseInboxId("inbox-abcdefghij-0"), null);    // lineno not positive
assert.equal(parseInboxId("file-toolonghash"), null);      // file hash > 10 chars
assert.equal(parseInboxId("INBOX-abcdefghij-1"), null);    // case-strict
assert.equal(parseInboxId(""), null);
assert.equal(parseInboxId(null), null);

// verifyInboxId: matching line passes.
{
  const line = "- 2026-04-27T12:34:56Z call dentist";
  const id = mintInboxId(line, 7);
  assert.equal(verifyInboxId(id, line), true);
}

// verifyInboxId: edited line fails.
{
  const original = "- 2026-04-27T12:34:56Z call dentist";
  const id = mintInboxId(original, 7);
  assert.equal(verifyInboxId(id, "- 2026-04-27T12:34:56Z call doctor"), false);
}

// verifyInboxId: non-inbox ID returns false (block IDs and file IDs cannot
// be verified by this function — caller must dispatch on parsed.kind).
{
  assert.equal(verifyInboxId("a01", "irrelevant"), false);
  assert.equal(verifyInboxId("file-abcdefghij", "irrelevant"), false);
}

console.log("inbox-id.js: 17/17 ok");
