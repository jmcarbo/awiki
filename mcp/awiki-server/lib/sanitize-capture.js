// Sanitize a free-text capture string before it is appended to inbox.md.
// Implements the "Capture sanitization" table from the task-layer spec.
//
// Returns { text: <sanitized string>, applied: <closed-enum array> }.
// Hard-reject patterns throw a SanitizeError; the caller (MCP boundary or
// capture.sh) translates that into an MCP error / exit-4 respectively.
//
// Closed enum for applied[]: "wikilink-neutralized", "comment-neutralized",
// "length-truncated", "block-id-escaped". Order is irrelevant; callers
// should treat as a set. Tab->space normalization is silent (not tracked).

export class SanitizeError extends Error {
  constructor(kind, message) {
    super(message);
    this.name = "SanitizeError";
    this.kind = kind; // "control-char" | "embedded-newline" | "checkbox-prefix"
  }
}

const MAX_LEN = 2000;

// Checkbox markers that the action grammar treats as state. Reject any
// of these as a line-leading prefix because captures are NOT actions —
// the inbox line is "- <ISO> <text>" and the user's text is appended after.
const CHECKBOX_PREFIX_RE = /^\s*\[[ x/?>]\]/;

// Block-ID shape: ^ followed by [a-z0-9]{4,}. Treat as word-bounded so we
// don't mangle e.g. "carret^foo" inside a URL fragment (paranoid; URLs
// are rare in captures but legal).
const BLOCK_ID_RE = /(^|[^\\\w])\^([a-z0-9]{4,})\b/g;

export function sanitizeCapture(text) {
  if (typeof text !== "string") {
    throw new SanitizeError("control-char", "capture text must be a string");
  }

  // Hard rejects (must run before any rewrite so we never half-mutate).
  if (/[\r\n]/.test(text)) {
    throw new SanitizeError(
      "embedded-newline",
      "capture text contains newline or carriage return",
    );
  }
  // Control chars except tab. C0 = U+0000..U+001F minus U+0009 (tab); plus U+007F (DEL).
  // eslint-disable-next-line no-control-regex
  if (/[\x00-\x08\x0b-\x1f\x7f]/.test(text)) {
    throw new SanitizeError(
      "control-char",
      "capture text contains control character",
    );
  }
  if (CHECKBOX_PREFIX_RE.test(text)) {
    throw new SanitizeError(
      "checkbox-prefix",
      "capture text begins with checkbox marker; captures are not actions",
    );
  }

  let out = text;
  const applied = new Set();

  // Tab -> space (silent normalization; spec table parenthetical).
  out = out.replace(/\t/g, " ");

  // Wikilink neutralization. Use fresh regexes (not shared /g state) so
  // tests are not affected by prior calls.
  if (/\[\[/.test(out) || /\]\]/.test(out)) {
    out = out.replace(/\[\[/g, "[ [").replace(/\]\]/g, "] ]");
    applied.add("wikilink-neutralized");
  }

  // Comment neutralization.
  if (/<!--/.test(out) || /-->/.test(out)) {
    out = out.replace(/<!--/g, "< !--").replace(/-->/g, "--  >");
    applied.add("comment-neutralized");
  }

  // Block-ID-shape escape. Reset lastIndex on the shared /g regex.
  BLOCK_ID_RE.lastIndex = 0;
  if (BLOCK_ID_RE.test(out)) {
    BLOCK_ID_RE.lastIndex = 0;
    out = out.replace(BLOCK_ID_RE, (_, lead, body) => `${lead}\\^${body}`);
    applied.add("block-id-escaped");
  }

  // Length cap (run last so neutralization doesn't push us over the limit
  // after truncation; truncation is the final word).
  if (out.length > MAX_LEN) {
    out = out.slice(0, MAX_LEN) + "…";
    applied.add("length-truncated");
  }

  return { text: out, applied: Array.from(applied) };
}
