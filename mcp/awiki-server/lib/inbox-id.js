// Inbox-line ID synthesis and verification.
// Spec section: "Inbox-line ID synthesis" in 2026-04-27-task-layer-design.md.
//
// Three ID shapes:
//   inbox-<sha1(line)[:10]>-<lineno>   (lines in inbox.md)
//   file-<sha1(relpath)[:10]>          (files in raw/inbox/interactive/)
//   <block-id>                         (action lines on content pages; minted by phase-17 scanner)
//
// The MCP boundary additionally enforces ^[a-z0-9_~-]{1,32}$ on the id arg
// before any of these helpers run; this file accepts inputs that pass
// that gate and decides what shape they are.

import { createHash } from "node:crypto";

const INBOX_ID_RE = /^inbox-([0-9a-f]{10})-([1-9][0-9]*)$/;
const FILE_ID_RE = /^file-([0-9a-f]{10})$/;
const BLOCK_ID_RE = /^[a-z0-9]{1,16}$/;

function sha10(s) {
  return createHash("sha1").update(s, "utf8").digest("hex").slice(0, 10);
}

export function mintInboxId(line, lineno) {
  if (typeof line !== "string") {
    throw new TypeError("mintInboxId: line must be a string");
  }
  if (!Number.isInteger(lineno) || lineno < 1) {
    throw new RangeError("mintInboxId: lineno must be a positive integer");
  }
  return `inbox-${sha10(line)}-${lineno}`;
}

export function mintFileId(relpath) {
  if (typeof relpath !== "string" || relpath.length === 0) {
    throw new TypeError("mintFileId: relpath must be a non-empty string");
  }
  return `file-${sha10(relpath)}`;
}

export function parseInboxId(id) {
  if (typeof id !== "string" || id.length === 0) return null;
  const inboxMatch = id.match(INBOX_ID_RE);
  if (inboxMatch) {
    return {
      kind: "inbox",
      hash: inboxMatch[1],
      lineno: parseInt(inboxMatch[2], 10),
    };
  }
  const fileMatch = id.match(FILE_ID_RE);
  if (fileMatch) {
    return { kind: "file", hash: fileMatch[1] };
  }
  if (BLOCK_ID_RE.test(id)) {
    return { kind: "block", hash: id };
  }
  return null;
}

// TOCTOU re-verification for inbox IDs only. Recomputes sha1(currentLine)[:10]
// and compares against the hash embedded in the id. Returns false for any
// non-inbox id shape (caller must dispatch on parseInboxId().kind).
export function verifyInboxId(id, currentLine) {
  const parsed = parseInboxId(id);
  if (!parsed || parsed.kind !== "inbox") return false;
  if (typeof currentLine !== "string") return false;
  return sha10(currentLine) === parsed.hash;
}

// Internal helper exported for tests / debugging.
export function _sha10(s) {
  return sha10(s);
}
