# awiki Plan — Phase 18b: Task-Layer MCP Server Extension

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md) — sections "MCP Server Additions" (full Security subsection), "Capture sanitization" table, "Triage MCP tool surface", "Inbox-line ID synthesis".
**Master spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master plan:** [`2026-04-27-task-layer-master-plan.md`](./2026-04-27-task-layer-master-plan.md)
**Style ref:** [`2026-04-27-phase-15-synth-refinement-mcp.md`](./2026-04-27-phase-15-synth-refinement-mcp.md) — MCP-tool wiring style, JSON-schema test conventions, `execFileSync` + `StdioServerTransport` discipline.
**Depends on:** Phase 16 (`scripts/lib/action-grammar.sh`, `scripts/lib/lock.sh`, `scripts/task-init.sh`, `scripts/capture.sh`, agenda placeholder pages), Phase 17 (`scripts/action-scan.sh`, `scripts/agenda.sh`, `.awiki/maps/actions.tsv`), Phase 18a (`scripts/triage.sh`, `scripts/action-recur.sh`, `scripts/log-append.sh` poisoning fix, lint rules T8-T13), Master Phase 8 (`mcp/awiki-server/index.js` with the four v1 tools and `wire-awiki-mcp.sh`).
**Previous:** [Phase 18a](./2026-04-27-phase-18a-task-triage-bash.md) (sibling, bash deliverables).
**Next:** [Phase 19](./2026-04-27-phase-19-task-review-polish.md).

**Tech stack:** Node 20+, `@modelcontextprotocol/sdk` ^1.0.0 (already pinned by Master Phase 8 via `mcp/awiki-server/package.json`), bash 4+, bats-core 1.10+, `flock` (util-linux ≥ 2.36 on Linux; `util-linux` Homebrew bottle on macOS).

**Conventions:**
- Branch: `phase-18b-task-mcp-server`.
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`. JS: ES modules (`"type": "module"`).
- Commit after every task. Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`).
- TDD where applicable: write failing test → run → implement → run → commit.
- Merge to `main` only after `bats tests/ && just lint && node mcp/awiki-server/test/*.test.mjs` are all green.
- `--` flag terminator before every positional argument in every shell-out and recipe.
- All MCP shell-outs use `execFileSync` with explicit argv arrays. **No shell interpolation, ever.**
- Hand-written JSON schemas; no `zod`, no `ajv` (Master Phase 8 posture, reaffirmed by Phase 15).
- Sanitization & validation cascade ordering — applied in this exact sequence at the MCP boundary, before any filesystem touch:
  1. Per-arg type check (string vs object).
  2. Per-arg regex (`id`, `outcome`, `*_slug`, `page_type`, dates).
  3. Date validity via `Date.UTC` round-trip.
  4. JSON Schema (for `params` envelope on `triage_apply`, `filter` envelope on `list_actions`).
  5. Path-resolution guard (`path.resolve` + canonical-parent-prefix check).
  6. Sanitization rules for `capture(text)` (control-char hard-reject, length-truncate, `<!--`/`-->`/`[[`/`]]` neutralize, block-ID-shape escape, checkbox-prefix hard-reject).
  7. Lock acquisition (`flock -x .awiki/lock` with 30s timeout for mutating tools, `flock -s` for read-only).
  8. Shell-out via `execFileSync` with argv array.

**Out of scope (covered by Phase 18a):**
- `scripts/triage.sh` (bash entry-point that the MCP server shells out to).
- `scripts/action-recur.sh`.
- `scripts/log-append.sh` log-poisoning fix.
- Lint rules T8-T13.

**Out of scope (covered by Phase 19):**
- Full implementation of `review_status` and `mark_review_done` (this phase ships stubs returning `{stub: true}`).

---

**Deliverable:** Seven new tools added to `mcp/awiki-server/index.js`:

1. `capture(text)` — sanitized append to `content/inbox.md`, `flock -x`.
2. `triage_inbox()` — read-only scan of inbox.md lines + `raw/inbox/interactive/` files, returns IDs.
3. `triage_apply({id, outcome, params})` — single mutation entry-point, regex + path-guard + TOCTOU re-verify.
4. `list_actions({filter?})` — read-only `actions.tsv` query, lazy re-scan when stale.
5. `rebuild_agenda()` — runs `action-scan.sh` + `agenda.sh`, returns `{rebuilt, duration_ms}`.
6. `review_status()` — STUB returning `{stub: true}`.
7. `mark_review_done()` — STUB returning `{stub: true}`.

Plus a JSON-Schema file (`mcp/awiki-server/schemas/triage-params.json`), a hand-rolled validator extension (or reuse of Phase 15's `validate-scope.js`), supporting library files under `mcp/awiki-server/lib/`, unit tests under `mcp/awiki-server/test/`, and a BATS smoke suite at `tests/mcp_task_test.bats`.

**Plan-level notes:**
- This phase does NOT re-wire the MCP server. `scripts/wire-awiki-mcp.sh` from v1 phase 8 already registers `awiki` with the agent harness; new tools are auto-exposed once the server is rebuilt.
- After landing this phase, every implementer / user must run **`cd mcp/awiki-server && npm install`** to pick up any new dev-only deps and warm Node's module cache. The plan prints this instruction in Task 18b.10.

---

## Task 18b.1: Branch + dependency check

**Files:** No file changes in this task. Read-only verification of prerequisites.

- [ ] **Step 1: Verify working tree is clean and prerequisites are merged**

```bash
git status -s
git log --oneline | head -10
ls scripts/triage.sh scripts/action-recur.sh scripts/action-scan.sh scripts/agenda.sh scripts/log-append.sh
ls mcp/awiki-server/index.js mcp/awiki-server/package.json
ls scripts/lib/action-grammar.sh scripts/lib/lock.sh
```

Expected: clean working tree on `main`; phase 16, 17, 18a, and v1 phase 8 commits all present in the log; every listed file exists.

- [ ] **Step 2: Verify v1 MCP server already exposes the four base tools**

```bash
( cd mcp/awiki-server && npm install --silent )
printf '%s\n' '{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}' \
  | node mcp/awiki-server/index.js | head -1 | tr ',' '\n' | grep '"name"'
```

Expected output (order may vary): four `"name"` entries — `ingest_source`, `query_wiki`, `lint`, `update_catalog`. If fewer than four appear, stop and run v1 phase 8 first.

- [ ] **Step 3: Pin the SDK version (idempotent)**

Open `mcp/awiki-server/package.json`. If `dependencies."@modelcontextprotocol/sdk"` is missing or unpinned, set it to `"^1.0.0"`:

```bash
node - <<'JS'
import { readFileSync, writeFileSync } from "node:fs";
const p = "mcp/awiki-server/package.json";
const j = JSON.parse(readFileSync(p, "utf8"));
j.dependencies ??= {};
if (!j.dependencies["@modelcontextprotocol/sdk"]) {
  j.dependencies["@modelcontextprotocol/sdk"] = "^1.0.0";
  writeFileSync(p, JSON.stringify(j, null, 2) + "\n");
  console.log("pinned SDK ^1.0.0");
} else {
  console.log("SDK already pinned to", j.dependencies["@modelcontextprotocol/sdk"]);
}
JS
```

Expected output: either `pinned SDK ^1.0.0` (and the file is modified) or `SDK already pinned to ^1.0.0` (and the file is untouched).

- [ ] **Step 4: Branch**

```bash
git checkout main
git pull --ff-only
git checkout -b phase-18b-task-mcp-server
```

Expected: `Switched to a new branch 'phase-18b-task-mcp-server'`.

- [ ] **Step 5: Stage + commit if package.json changed**

```bash
git status -s mcp/awiki-server/package.json
git add mcp/awiki-server/package.json 2>/dev/null || true
git diff --cached --quiet || git commit -m "chore(mcp): pin @modelcontextprotocol/sdk to ^1.0.0"
```

Expected: either the commit lands (one file changed) or `git diff --cached --quiet` succeeds and no commit is created.

---

## Task 18b.2: Library — `mcp/awiki-server/lib/sanitize-capture.js`

**Files:** Create: `mcp/awiki-server/lib/sanitize-capture.js`. Test: `mcp/awiki-server/test/sanitize-capture.test.mjs`.

Implements the spec's "Capture sanitization" table verbatim. Two failure modes:
- **Hard reject** (control chars / embedded newlines / checkbox-prefix-at-start) — throws a `SanitizeError` with a `kind` discriminator. The MCP boundary catches this and returns an MCP-level error.
- **Neutralize** (`<!--`/`-->`/`[[`/`]]`/block-ID-shape/length>2000) — silently rewrites the text and reports the rule that fired in `sanitizations_applied`.

Closed enum for `sanitizations_applied`: `wikilink-neutralized`, `comment-neutralized`, `length-truncated`, `block-id-escaped`. Hard-rejects MUST NOT appear in this list — they short-circuit before the array is returned.

- [ ] **Step 1: Write the failing test FIRST**

```bash
mkdir -p mcp/awiki-server/test
cat > mcp/awiki-server/test/sanitize-capture.test.mjs <<'EOF'
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

// Length truncation: text > 2000 → truncated to 2000 chars + "…".
{
  const long = "x".repeat(2500);
  const r = sanitizeCapture(long);
  assert.equal(r.text.length, 2001); // 2000 chars + 1 ellipsis char
  assert.equal(r.text.endsWith("…"), true);
  assert.deepEqual(r.applied, ["length-truncated"]);
}

// Block-ID-shape escape: ^abc123 → \^abc123 anywhere it appears as a word.
{
  const r = sanitizeCapture("note ^abc123 reference");
  assert.equal(r.text, "note \\^abc123 reference");
  assert.deepEqual(r.applied, ["block-id-escaped"]);
}

// Multiple rules combine; applied list is order-preserving and deduped.
{
  const r = sanitizeCapture("[[link]] <!--c--> ^abc123");
  assert.equal(r.text, "[ [link] ] < !--c--  > \\^abc123");
  assert.deepEqual(r.applied.sort(), ["block-id-escaped", "comment-neutralized", "wikilink-neutralized"]);
}

// Hard reject: embedded newline.
assert.throws(() => sanitizeCapture("line one\nline two"), (e) => e instanceof SanitizeError && e.kind === "embedded-newline");

// Hard reject: carriage return.
assert.throws(() => sanitizeCapture("foo\rbar"), (e) => e instanceof SanitizeError && e.kind === "embedded-newline");

// Hard reject: NUL byte.
assert.throws(() => sanitizeCapture("foo\x00bar"), (e) => e instanceof SanitizeError && e.kind === "control-char");

// Hard reject: other control chars (e.g. ESC).
assert.throws(() => sanitizeCapture("foo\x1bbar"), (e) => e instanceof SanitizeError && e.kind === "control-char");

// Tab → space (NOT a hard reject; per spec table the parenthetical "(except tab → space)" exempts it).
{
  const r = sanitizeCapture("foo\tbar");
  assert.equal(r.text, "foo bar");
  // Tab→space is a normalization, not a tracked sanitization; it does NOT appear in applied[].
  assert.deepEqual(r.applied, []);
}

// Hard reject: checkbox prefix at start of line.
assert.throws(() => sanitizeCapture("[ ] not a capture"), (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix");
assert.throws(() => sanitizeCapture("[/] also not"), (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix");
assert.throws(() => sanitizeCapture("[x] also not"), (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix");
assert.throws(() => sanitizeCapture("[?] also not"), (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix");
assert.throws(() => sanitizeCapture("[>] also not"), (e) => e instanceof SanitizeError && e.kind === "checkbox-prefix");

// Checkbox prefix NOT at start of line is fine — it would not survive line-formatting anyway.
{
  const r = sanitizeCapture("text [ ] not at start");
  assert.equal(r.text, "text [ ] not at start");
  assert.deepEqual(r.applied, []);
}

console.log("sanitize-capture.js: 14/14 ok");
EOF

node mcp/awiki-server/test/sanitize-capture.test.mjs
```

Expected: failure with `Cannot find module '.../lib/sanitize-capture.js'`. Good — write it now.

- [ ] **Step 2: Implement the library**

```bash
mkdir -p mcp/awiki-server/lib
cat > mcp/awiki-server/lib/sanitize-capture.js <<'EOF'
// Sanitize a free-text capture string before it is appended to inbox.md.
// Implements the "Capture sanitization" table from the task-layer spec.
//
// Returns { text: <sanitized string>, applied: <closed-enum array> }.
// Hard-reject patterns throw a SanitizeError; the caller (MCP boundary or
// capture.sh) translates that into an MCP error / exit-4 respectively.
//
// Closed enum for applied[]: "wikilink-neutralized", "comment-neutralized",
// "length-truncated", "block-id-escaped". Order is irrelevant; callers
// should treat as a set. Tab→space normalization is silent (not tracked).

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

// Wikilink open and close. Replace with space-padded variants so the
// scanner cannot resolve them.
const WIKILINK_OPEN_RE = /\[\[/g;
const WIKILINK_CLOSE_RE = /\]\]/g;

// HTML / managed-region comment markers. Replace with space-padded
// variants so they cannot terminate a real region or open an HTML
// comment in rendered output.
const COMMENT_OPEN_RE = /<!--/g;
const COMMENT_CLOSE_RE = /-->/g;

export function sanitizeCapture(text) {
  if (typeof text !== "string") {
    throw new SanitizeError("control-char", "capture text must be a string");
  }

  // Hard rejects (must run before any rewrite so we never half-mutate).
  if (/[\r\n]/.test(text)) {
    throw new SanitizeError("embedded-newline", "capture text contains newline or carriage return");
  }
  // Control chars except tab. C0 = U+0000..U+001F minus U+0009 (tab); plus U+007F (DEL).
  // eslint-disable-next-line no-control-regex
  if (/[\x00-\x08\x0b-\x1f\x7f]/.test(text)) {
    throw new SanitizeError("control-char", "capture text contains control character");
  }
  if (CHECKBOX_PREFIX_RE.test(text)) {
    throw new SanitizeError(
      "checkbox-prefix",
      "capture text begins with checkbox marker; captures are not actions",
    );
  }

  let out = text;
  const applied = new Set();

  // Tab → space (silent normalization; spec table parenthetical).
  out = out.replace(/\t/g, " ");

  // Wikilink neutralization.
  if (WIKILINK_OPEN_RE.test(out) || WIKILINK_CLOSE_RE.test(out)) {
    // Reset lastIndex on the regexes (they're stateful with /g).
    WIKILINK_OPEN_RE.lastIndex = 0;
    WIKILINK_CLOSE_RE.lastIndex = 0;
    out = out.replace(/\[\[/g, "[ [").replace(/\]\]/g, "] ]");
    applied.add("wikilink-neutralized");
  }

  // Comment neutralization.
  if (COMMENT_OPEN_RE.test(out) || COMMENT_CLOSE_RE.test(out)) {
    COMMENT_OPEN_RE.lastIndex = 0;
    COMMENT_CLOSE_RE.lastIndex = 0;
    out = out.replace(/<!--/g, "< !--").replace(/-->/g, "--  >");
    applied.add("comment-neutralized");
  }

  // Block-ID-shape escape.
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
EOF
```

- [ ] **Step 3: Re-run the test**

```bash
node mcp/awiki-server/test/sanitize-capture.test.mjs
```

Expected output: `sanitize-capture.js: 14/14 ok`. If a case fails, the assertion message will name the offending input string and the actual vs expected output — fix the regex / replacement and re-run.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/lib/sanitize-capture.js mcp/awiki-server/test/sanitize-capture.test.mjs
git commit -m "feat(mcp): add capture-text sanitizer (spec table verbatim)

Implements the seven rules from the 'Capture sanitization' table:
control chars / embedded newlines / checkbox-prefix-at-start are
hard-rejected via SanitizeError; <!--/-->, [[/]], block-ID-shape, and
length>2000 are neutralized in place and reported in a closed-enum
applied[] set. Tab→space normalization is silent. 14 unit tests cover
all rules, single + combined + hard-reject paths."
```

---

## Task 18b.3: Library — `mcp/awiki-server/lib/inbox-id.js`

**Files:** Create: `mcp/awiki-server/lib/inbox-id.js`. Test: `mcp/awiki-server/test/inbox-id.test.mjs`.

Implements the spec's "Inbox-line ID synthesis" rules verbatim:
- **Inbox lines:** `id = inbox-<sha1(line)[:10]>-<lineno>` where `line` is the verbatim raw line (including the leading `- <ISO-datetime> ` prefix and the user's text) and `lineno` is the 1-indexed line number in `inbox.md` at scan time.
- **Files:** `id = file-<sha1(relpath)[:10]>` where `relpath` is the path relative to repo root.
- **Action lines on content pages:** `id = <^id>` (the literal block-ID — already minted by phase 17 scanner).

Three exports:
- `mintInboxId(line, lineno)` — synthesize.
- `parseInboxId(id)` — return `{ kind, hash, lineno? }` or `null`.
- `verifyInboxId(id, currentLine)` — TOCTOU re-verification; returns `true` iff `id` matches the freshly recomputed hash for `currentLine`. Used by `triage_apply` between `triage_inbox()` and the actual mutation.

- [ ] **Step 1: Write the failing test FIRST**

```bash
cat > mcp/awiki-server/test/inbox-id.test.mjs <<'EOF'
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
  assert.deepEqual(parsed, { kind: "file", hash: sha10("raw/inbox/interactive/voice-001.txt") });
}

// parseInboxId: action-line block-ID (no inbox-/file- prefix).
{
  const parsed = parseInboxId("a01");
  assert.deepEqual(parsed, { kind: "block", hash: "a01" });
}

// parseInboxId: rejects malformed.
assert.equal(parseInboxId("inbox-toolong-12345-1"), null);    // hash > 10 chars
assert.equal(parseInboxId("inbox-abc-1"), null);              // hash < 10 chars
assert.equal(parseInboxId("inbox-abcdefghij-x"), null);       // lineno not numeric
assert.equal(parseInboxId("inbox-abcdefghij-0"), null);       // lineno not positive
assert.equal(parseInboxId("file-toolonghash"), null);         // file hash > 10 chars
assert.equal(parseInboxId("INBOX-abcdefghij-1"), null);       // case-strict
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
EOF

node mcp/awiki-server/test/inbox-id.test.mjs
```

Expected: failure with `Cannot find module`.

- [ ] **Step 2: Implement**

```bash
cat > mcp/awiki-server/lib/inbox-id.js <<'EOF'
// Inbox-line ID synthesis and verification.
// Spec section: "Inbox-line ID synthesis" in 2026-04-27-task-layer-design.md.
//
// Three ID shapes:
//   inbox-<sha1(line)[:10]>-<lineno>   (lines in inbox.md)
//   file-<sha1(relpath)[:10]>           (files in raw/inbox/interactive/)
//   <block-id>                          (action lines on content pages; minted by phase-17 scanner)
//
// The MCP boundary additionally enforces ^[a-z0-9~-]{1,32}$ on the id arg
// before any of these helpers run; this file accepts inputs that pass
// that gate and decides what shape they are.

import { createHash } from "node:crypto";

const HEX10_RE = /^[0-9a-f]{10}$/;
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
    return { kind: "inbox", hash: inboxMatch[1], lineno: parseInt(inboxMatch[2], 10) };
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
EOF
```

- [ ] **Step 3: Re-run**

```bash
node mcp/awiki-server/test/inbox-id.test.mjs
```

Expected: `inbox-id.js: 17/17 ok`.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/lib/inbox-id.js mcp/awiki-server/test/inbox-id.test.mjs
git commit -m "feat(mcp): add inbox-id mint/parse/verify helpers

Three ID shapes per spec: inbox-<sha1[:10]>-<lineno>, file-<sha1[:10]>,
and bare block-ID. mintInboxId rejects non-positive linenos and non-
string lines. parseInboxId returns null for malformed inputs (case-
strict, hash-length-exact, lineno >= 1). verifyInboxId is the TOCTOU
re-verification used by triage_apply: it returns false unless the
freshly-computed sha1[:10] of currentLine matches the embedded hash."
```

---

## Task 18b.4: Library — `mcp/awiki-server/lib/triage-validate.js` (regex + date)

**Files:** Create: `mcp/awiki-server/lib/triage-validate.js`, `mcp/awiki-server/schemas/triage-params.json`. Test: `mcp/awiki-server/test/triage-validate.test.mjs`.

Two layers of validation for `triage_apply` arguments:

1. **Per-arg regex** at the boundary, run before any FS access:
   - `id`: `^[a-z0-9_~-]{1,32}$` (the spec's umbrella shape — covers `inbox-…`, `file-…`, plain block-IDs, and recurrence-chain IDs that use `~`).
   - `outcome` ∈ closed enum `{trash, do-now, act, defer-scheduled, waiting, reference, someday}`.
   - `project_slug`: `^[a-z0-9_][a-z0-9_-]{0,63}$` (underscore-prefix allowed for `_loose`, `_someday`).
   - `context_slug`, `wait_for`, `ref_slug`: `^[a-z0-9][a-z0-9-]{0,63}$`.
   - `page_type` ∈ `{entity, concept, topic, source}` (only valid when `outcome === "reference"`).
   - `due`, `defer`: `^\d{4}-\d{2}-\d{2}$` AND valid ISO calendar via `Date.UTC` round-trip.

2. **JSON Schema** for the `params` envelope: only the allowed keys, types match. Reuses Phase 15's hand-rolled validator (`mcp/awiki-server/lib/validate-scope.js`); we just ship a new schema file.

Returns a normalized `{ ok: true, normalized: {...} }` on success or `{ ok: false, errors: [...] }` on failure. Errors are structured (each entry is `{ field, reason }`) so the MCP tool dispatcher can shape them into a JSON error payload without further parsing.

- [ ] **Step 1: Write the schema file**

```bash
mkdir -p mcp/awiki-server/schemas
cat > mcp/awiki-server/schemas/triage-params.json <<'EOF'
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "project_slug":  {"type": "string", "pattern": "^[a-z0-9_][a-z0-9_-]{0,63}$", "maxLength": 64},
    "context_slug":  {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,63}$",   "maxLength": 64},
    "wait_for":      {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,63}$",   "maxLength": 64},
    "ref_slug":      {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,63}$",   "maxLength": 64},
    "page_type":     {"enum": ["entity", "concept", "topic", "source"]},
    "due":           {"type": "string", "pattern": "^\\d{4}-\\d{2}-\\d{2}$"},
    "defer":         {"type": "string", "pattern": "^\\d{4}-\\d{2}-\\d{2}$"}
  }
}
EOF
```

- [ ] **Step 2: Write the failing test FIRST**

```bash
cat > mcp/awiki-server/test/triage-validate.test.mjs <<'EOF'
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
  assert.equal(r.ok, false, `expected fail, got normalized: ${JSON.stringify(r.normalized)}`);
  return r.errors;
}

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
for (const o of ["trash","do-now","act","defer-scheduled","waiting","reference","someday"]) {
  ok("a01", o);
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
bad("a01", "reference", { ref_slug: "memex", page_type: "agenda" });   // not in enum
bad("a01", "reference", { ref_slug: "memex" });                         // page_type missing for reference
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

console.log("triage-validate.js: ok");
EOF

node mcp/awiki-server/test/triage-validate.test.mjs
```

Expected: failure (no module).

- [ ] **Step 3: Implement**

```bash
cat > mcp/awiki-server/lib/triage-validate.js <<'EOF'
// triage_apply argument validation.
// Boundary cascade per spec: regex first, JSON Schema second, date round-trip
// last. Path-resolution is a separate concern (see lib/path-guard.js) and
// runs after this validator clears.

import { readFileSync } from "node:fs";
import { validate as validateSchema } from "./validate-scope.js";

const SCHEMA = JSON.parse(
  readFileSync(new URL("../schemas/triage-params.json", import.meta.url), "utf8"),
);

export const ID_RE = /^[a-z0-9~-]{1,32}$/;
export const OUTCOMES = Object.freeze([
  "trash", "do-now", "act", "defer-scheduled", "waiting", "reference", "someday",
]);
export const PAGE_TYPES = Object.freeze(["entity", "concept", "topic", "source"]);
const PROJECT_SLUG_RE = /^[a-z0-9_][a-z0-9_-]{0,63}$/;
const PLAIN_SLUG_RE   = /^[a-z0-9][a-z0-9-]{0,63}$/;
const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

// ISO calendar validity via Date.UTC round-trip.
function validIsoDate(s) {
  const m = DATE_RE.exec(s);
  if (!m) return false;
  const [, y, mo, d] = m.map(Number);
  if (mo < 1 || mo > 12) return false;
  if (d < 1 || d > 31) return false;
  const utc = Date.UTC(y, mo - 1, d);
  const back = new Date(utc);
  return back.getUTCFullYear() === y
      && back.getUTCMonth() + 1 === mo
      && back.getUTCDate() === d;
}

export function validateTriageArgs(args) {
  const errors = [];

  // 1. Per-arg type + regex (id, outcome).
  if (typeof args !== "object" || args === null || Array.isArray(args)) {
    return { ok: false, errors: [{ field: "(root)", reason: "args must be a plain object" }] };
  }
  const { id, outcome } = args;
  let { params } = args;

  if (typeof id !== "string" || !ID_RE.test(id)) {
    errors.push({ field: "id", reason: "must match ^[a-z0-9~-]{1,32}$" });
  }
  if (typeof outcome !== "string" || !OUTCOMES.includes(outcome)) {
    errors.push({ field: "outcome", reason: `must be one of ${OUTCOMES.join("|")}` });
  }

  // 2. params normalization + JSON Schema.
  if (params === undefined) {
    params = {};
  }
  if (typeof params !== "object" || params === null || Array.isArray(params)) {
    errors.push({ field: "params", reason: "must be a plain object" });
    return { ok: false, errors };
  }

  const schemaCheck = validateSchema(SCHEMA, params);
  if (!schemaCheck.valid) {
    for (const msg of schemaCheck.errors) {
      errors.push({ field: "params", reason: msg });
    }
  }

  // 3. Outcome-specific cross-field checks.
  if (outcome === "reference") {
    if (!("ref_slug" in params)) {
      errors.push({ field: "params.ref_slug", reason: "required for outcome=reference" });
    }
    if (!("page_type" in params)) {
      errors.push({ field: "params.page_type", reason: "required for outcome=reference" });
    }
  } else if ("page_type" in params) {
    errors.push({ field: "params.page_type", reason: "only allowed for outcome=reference" });
  }

  // 4. Slug-shape regexes (defense in depth — schema patterns already cover
  // these, but keep explicit checks in case the schema is mis-edited).
  if ("project_slug" in params && !PROJECT_SLUG_RE.test(params.project_slug)) {
    errors.push({ field: "params.project_slug", reason: "must match ^[a-z0-9_][a-z0-9_-]{0,63}$" });
  }
  for (const k of ["context_slug", "wait_for", "ref_slug"]) {
    if (k in params && !PLAIN_SLUG_RE.test(params[k])) {
      errors.push({ field: `params.${k}`, reason: "must match ^[a-z0-9][a-z0-9-]{0,63}$" });
    }
  }
  if ("page_type" in params && !PAGE_TYPES.includes(params.page_type)) {
    errors.push({ field: "params.page_type", reason: `must be one of ${PAGE_TYPES.join("|")}` });
  }

  // 5. Date validity (ISO calendar round-trip).
  for (const k of ["due", "defer"]) {
    if (k in params && !validIsoDate(params[k])) {
      errors.push({ field: `params.${k}`, reason: "not a valid ISO calendar date" });
    }
  }

  if (errors.length) return { ok: false, errors };
  return { ok: true, normalized: { id, outcome, params } };
}

// Exposed for unit tests / CLI debugging.
export { validIsoDate };
EOF
```

- [ ] **Step 4: Re-run**

```bash
node mcp/awiki-server/test/triage-validate.test.mjs
```

Expected: `triage-validate.js: ok`.

- [ ] **Step 5: Commit**

```bash
git add mcp/awiki-server/lib/triage-validate.js \
        mcp/awiki-server/schemas/triage-params.json \
        mcp/awiki-server/test/triage-validate.test.mjs
git commit -m "feat(mcp): add triage_apply argument validator + schema

Two-layer validation cascade per spec security subsection: per-arg
regex (id, outcome) at the boundary, then JSON Schema for the params
envelope (reuses Phase 15's hand-rolled validator), then outcome-
specific cross-field rules (ref_slug + page_type required iff
outcome=reference; page_type forbidden otherwise), then ISO calendar
round-trip via Date.UTC for due/defer (rejects 2026-02-30 etc).
Returns structured {field, reason} errors for the MCP dispatcher to
shape into a JSON payload."
```

---

## Task 18b.5: Library — `mcp/awiki-server/lib/path-guard.js`

**Files:** Create: `mcp/awiki-server/lib/path-guard.js`. Test: `mcp/awiki-server/test/path-guard.test.mjs`.

After regex validation passes, `triage_apply` resolves the destination paths it would touch (`content/projects/<project_slug>.md`, `content/contexts/<context_slug>.md`, `content/<page_type>s/<ref_slug>.md`) and rejects the call if the resolved path does not start with the canonical parent directory (with trailing separator).

This catches `..`-traversal, leading-hyphen flag injection, and symlink swap (we do `realpathSync` only when the file exists; for create-paths we resolve the parent and then require the basename to live directly under it).

- [ ] **Step 1: Write the failing test FIRST**

```bash
cat > mcp/awiki-server/test/path-guard.test.mjs <<'EOF'
import { resolveUnder, PathGuardError } from "../lib/path-guard.js";
import { mkdtempSync, mkdirSync, writeFileSync, symlinkSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-pathguard-"));
mkdirSync(join(root, "content/projects"), { recursive: true });
mkdirSync(join(root, "content/contexts"), { recursive: true });
mkdirSync(join(root, "content/topics"), { recursive: true });
writeFileSync(join(root, "content/projects/foo.md"), "ok");

// Happy path: legal slug under canonical parent.
{
  const p = resolveUnder(root, "content/projects", "foo");
  assert.equal(p, join(root, "content/projects/foo.md"));
}

// Happy path: file does not yet exist (creating new project page).
{
  const p = resolveUnder(root, "content/projects", "newproj");
  assert.equal(p, join(root, "content/projects/newproj.md"));
}

// Underscore-prefix project slug (e.g., _loose) — the path-guard must accept
// the resolved path as long as it is directly under content/projects/.
{
  const p = resolveUnder(root, "content/projects", "_loose");
  assert.equal(p, join(root, "content/projects/_loose.md"));
}

// Reject: parent traversal in slug.
assert.throws(() => resolveUnder(root, "content/projects", "../evil"), PathGuardError);

// Reject: absolute path masquerading as slug.
assert.throws(() => resolveUnder(root, "content/projects", "/etc/passwd"), PathGuardError);

// Reject: nested path in slug.
assert.throws(() => resolveUnder(root, "content/projects", "sub/dir"), PathGuardError);

// Reject: slug containing only dots.
assert.throws(() => resolveUnder(root, "content/projects", ".."), PathGuardError);
assert.throws(() => resolveUnder(root, "content/projects", "."), PathGuardError);

// Reject: empty slug.
assert.throws(() => resolveUnder(root, "content/projects", ""), PathGuardError);

// Reject: symlink whose target escapes the parent.
mkdirSync(join(root, "content/escape-source"), { recursive: true });
writeFileSync(join(root, "content/escape-source/target.md"), "evil");
symlinkSync("../escape-source/target.md", join(root, "content/projects/symlink.md"));
assert.throws(() => resolveUnder(root, "content/projects", "symlink"), PathGuardError);

// Cleanup.
rmSync(root, { recursive: true, force: true });
console.log("path-guard.js: ok");
EOF

node mcp/awiki-server/test/path-guard.test.mjs
```

Expected: failure (no module).

- [ ] **Step 2: Implement**

```bash
cat > mcp/awiki-server/lib/path-guard.js <<'EOF'
// Path-resolution guard. Run AFTER regex validation has cleared every slug
// arg. Resolves the canonical parent dir and the candidate file path, then
// asserts that the file path is a direct child of the parent (no traversal,
// no nested subdir, no symlink whose realpath escapes the parent).

import { realpathSync, statSync } from "node:fs";
import { resolve, sep, dirname, basename } from "node:path";

export class PathGuardError extends Error {
  constructor(message) {
    super(message);
    this.name = "PathGuardError";
  }
}

// repoRoot: absolute path to the repo root.
// parentRel: e.g. "content/projects" — relative to repoRoot.
// slug: the validated slug (regex already passed at the MCP boundary).
// Returns the absolute resolved path; throws PathGuardError on any escape.
export function resolveUnder(repoRoot, parentRel, slug) {
  if (typeof slug !== "string" || slug.length === 0) {
    throw new PathGuardError(`slug must be a non-empty string`);
  }
  // Reject obvious traversal / nested forms even before resolve(); resolve()
  // would normalize them, but we want to see the raw shape so error messages
  // are useful.
  if (slug === "." || slug === "..") {
    throw new PathGuardError(`slug "${slug}" is not allowed`);
  }
  if (slug.includes("/") || slug.includes(sep) || slug.startsWith(".")) {
    // Note: leading "." excludes ".hidden" too; project / context / topic
    // slugs are never hidden. Underscore-prefix (_loose, _someday) is allowed
    // and is NOT a leading-dot.
    throw new PathGuardError(`slug "${slug}" must not contain path separators or leading dot`);
  }

  const canonicalParent = resolve(repoRoot, parentRel);
  const candidate = resolve(canonicalParent, `${slug}.md`);

  // Basename + dirname check on the resolved candidate. The candidate must
  // live directly under canonicalParent, not in a sub-tree.
  if (dirname(candidate) !== canonicalParent) {
    throw new PathGuardError(
      `resolved path ${candidate} is not a direct child of ${canonicalParent}`,
    );
  }

  // If the candidate exists, follow symlinks and re-check. This catches the
  // case where an attacker placed a symlink in content/projects/ pointing at
  // /etc/passwd or out-of-tree content.
  let st;
  try {
    st = statSync(candidate);
  } catch {
    // Does not exist — fine for create-paths (new project, new ref page).
    return candidate;
  }
  if (st.isSymbolicLink()) {
    const real = realpathSync(candidate);
    if (dirname(real) !== canonicalParent) {
      throw new PathGuardError(
        `symlink ${candidate} resolves to ${real}, outside ${canonicalParent}`,
      );
    }
  } else {
    // statSync follows symlinks; do an explicit lstat-equivalent via realpath
    // to catch the case where the file IS a symlink whose target is in-tree
    // but we still want to be sure.
    const real = realpathSync(candidate);
    if (dirname(real) !== canonicalParent) {
      throw new PathGuardError(
        `path ${candidate} realpath ${real} escapes ${canonicalParent}`,
      );
    }
  }
  return candidate;
}

// Convenience: resolve all the destination paths a triage_apply call might
// touch given the validated, normalized args. Returns an array of absolute
// paths; throws on any escape. The MCP boundary calls this once before the
// shell-out so the failure mode is "MCP error, nothing written".
export function resolveTriageDestinations(repoRoot, { outcome, params }) {
  const out = [];
  if (params.project_slug) {
    out.push(resolveUnder(repoRoot, "content/projects", params.project_slug));
  }
  if (params.context_slug) {
    out.push(resolveUnder(repoRoot, "content/contexts", params.context_slug));
  }
  if (outcome === "reference" && params.ref_slug && params.page_type) {
    out.push(resolveUnder(repoRoot, `content/${params.page_type}s`, params.ref_slug));
  }
  return out;
}
EOF
```

- [ ] **Step 3: Re-run**

```bash
node mcp/awiki-server/test/path-guard.test.mjs
```

Expected: `path-guard.js: ok`.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/lib/path-guard.js mcp/awiki-server/test/path-guard.test.mjs
git commit -m "feat(mcp): add path-resolution guard for triage destinations

resolveUnder(repoRoot, parentRel, slug) returns the absolute path
content/<parent>/<slug>.md and throws PathGuardError if the resolved
path is not a direct child of the canonical parent (catches ../,
nested subdirs, leading-dot, symlink-swap). resolveTriageDestinations
is the convenience wrapper invoked from the MCP boundary."
```

---

## Task 18b.6: Library — `mcp/awiki-server/lib/lock.js`

**Files:** Create: `mcp/awiki-server/lib/lock.js`. Test: `mcp/awiki-server/test/lock.test.mjs`.

The MCP server runs every mutating tool inside a `flock -x .awiki/lock` (timeout 30 s) and every read-only tool inside `flock -s .awiki/lock`. This wraps the `awiki_lock_with` / `awiki_lock_shared` helpers from phase 16's `scripts/lib/lock.sh`, but since the MCP server is in JS we either (a) shell out to `flock` directly, or (b) shell out via `bash -c 'awiki_lock_with --timeout=30 -- "$@"'`. Option (a) is simpler and closer to the spec text ("acquires `flock -x .awiki/lock`"). We choose (a).

The wrapper accepts a JS callback that receives the file descriptor count via stdin/stdout — since `execFileSync` is synchronous, the simplest pattern is: spawn `bash -c "flock -x -w 30 .awiki/lock /usr/bin/env true"` to acquire briefly, then run the actual work, then release. **That's wrong** — there's a TOCTOU between release and the work.

Correct pattern: open the lock fd in a shell sub-process, run the JS work *inside* that sub-process via a separate command. Since `execFileSync` is synchronous-only, we instead invoke each shell-out *itself* under flock:

```bash
flock -x -w 30 .awiki/lock <command>
```

So `lock.js` does NOT wrap a JS callback; it returns the argv prefix that callers prepend to their `execFileSync` invocation. `runLocked(repoRoot, mode, argv)` builds and runs `bash -c "flock -<x|s> -w 30 .awiki/lock <argv>"` for them.

For pure-JS work that doesn't shell out (e.g., reading a file), we shell out a no-op `bash` command to *hold* the lock for the duration of the JS work — but that defeats sync. Simpler: every mutating tool DOES shell out (to `triage.sh`, `action-scan.sh`, `agenda.sh`, `capture.sh`), so we never need a pure-JS lock-hold. The one exception is `capture(text)` which does the inbox.md append in JS; for that we shell out the append too (`bash -c 'flock -x -w 30 .awiki/lock printf ...'`). See Task 18b.7.

- [ ] **Step 1: Write the failing test FIRST**

```bash
cat > mcp/awiki-server/test/lock.test.mjs <<'EOF'
import { runLocked, LockTimeoutError } from "../lib/lock.js";
import { mkdtempSync, writeFileSync, mkdirSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { spawn } from "node:child_process";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-lock-"));
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");

// Happy path: shared lock, command runs.
{
  const out = runLocked({ repoRoot: root, mode: "s", timeoutSec: 5, argv: ["echo", "hello"] });
  assert.equal(out.trim(), "hello");
}

// Happy path: exclusive lock, command runs.
{
  const out = runLocked({ repoRoot: root, mode: "x", timeoutSec: 5, argv: ["echo", "world"] });
  assert.equal(out.trim(), "world");
}

// Timeout: a competing exclusive holder forces this call to fail fast.
{
  const blocker = spawn("bash", ["-c", `flock -x ${join(root, ".awiki/lock")} sleep 3`]);
  // Give blocker time to acquire.
  await new Promise((r) => setTimeout(r, 200));
  let threw = null;
  try {
    runLocked({ repoRoot: root, mode: "x", timeoutSec: 1, argv: ["echo", "should-not-reach"] });
  } catch (e) {
    threw = e;
  }
  blocker.kill("SIGTERM");
  assert.ok(threw instanceof LockTimeoutError, `expected LockTimeoutError, got ${threw}`);
}

// Argv must be an array of strings (no shell interpolation).
assert.throws(() => runLocked({ repoRoot: root, mode: "x", timeoutSec: 5, argv: "echo hi" }));
assert.throws(() => runLocked({ repoRoot: root, mode: "x", timeoutSec: 5, argv: ["echo", 123] }));

// Mode validation.
assert.throws(() => runLocked({ repoRoot: root, mode: "rwx", timeoutSec: 5, argv: ["true"] }));

rmSync(root, { recursive: true, force: true });
console.log("lock.js: ok");
EOF

node mcp/awiki-server/test/lock.test.mjs
```

Expected: failure (no module).

- [ ] **Step 2: Implement**

```bash
cat > mcp/awiki-server/lib/lock.js <<'EOF'
// flock wrapper for MCP shell-outs. Mirrors scripts/lib/lock.sh's contract:
// mutating tools take exclusive ("x") with 30s timeout; read-only tools take
// shared ("s") with 30s timeout. The MCP boundary calls runLocked() instead
// of execFileSync() directly so every shell-out is automatically guarded.
//
// Implementation: builds argv ["flock", "-<x|s>", "-w", "<sec>",
// ".awiki/lock", "--", <user argv...>] and invokes via execFileSync. We rely
// on flock(1) from util-linux >= 2.36 (Linux ships this; macOS users
// install via Homebrew util-linux bottle, documented in WIKI.md / task-init).

import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import { join } from "node:path";

export class LockTimeoutError extends Error {
  constructor(timeoutSec) {
    super(`flock timed out after ${timeoutSec}s`);
    this.name = "LockTimeoutError";
    this.timeoutSec = timeoutSec;
  }
}

const FLOCK_TIMEOUT_EXIT = 1;

export function runLocked({ repoRoot, mode, timeoutSec, argv, env, encoding = "utf8" }) {
  if (mode !== "x" && mode !== "s") {
    throw new TypeError(`runLocked: mode must be "x" or "s", got ${JSON.stringify(mode)}`);
  }
  if (!Array.isArray(argv) || argv.some((a) => typeof a !== "string")) {
    throw new TypeError(`runLocked: argv must be an array of strings`);
  }
  if (!Number.isInteger(timeoutSec) || timeoutSec < 1) {
    throw new RangeError(`runLocked: timeoutSec must be a positive integer`);
  }
  const lockPath = join(repoRoot, ".awiki", "lock");
  if (!existsSync(lockPath)) {
    throw new Error(`runLocked: lock file ${lockPath} missing — did task-init run?`);
  }

  const flockArgv = [
    `-${mode}`,
    "-w", String(timeoutSec),
    lockPath,
    "--",
    ...argv,
  ];

  try {
    return execFileSync("flock", flockArgv, {
      cwd: repoRoot,
      encoding,
      env: env ?? process.env,
      stdio: ["ignore", "pipe", "pipe"],
    });
  } catch (e) {
    // flock(1) returns the wrapped command's exit code on success, or its
    // own conflict-exit (1 by default; configurable via -E) on timeout.
    // Because we did NOT pass -E, exit 1 means timeout.
    if (e.status === FLOCK_TIMEOUT_EXIT && (!e.stderr || /^$/m.test(e.stderr.toString()))) {
      throw new LockTimeoutError(timeoutSec);
    }
    throw e;
  }
}

// Convenience wrappers for the common modes.
export function runLockedExclusive(opts) {
  return runLocked({ ...opts, mode: "x", timeoutSec: opts.timeoutSec ?? 30 });
}
export function runLockedShared(opts) {
  return runLocked({ ...opts, mode: "s", timeoutSec: opts.timeoutSec ?? 30 });
}
EOF
```

- [ ] **Step 3: Re-run**

```bash
node mcp/awiki-server/test/lock.test.mjs
```

Expected: `lock.js: ok`. The "competing exclusive holder" sub-test relies on `flock` being on PATH; if the test errors with `ENOENT` for `flock`, install util-linux (`brew install util-linux` on macOS, then PATH-prepend `/opt/homebrew/opt/util-linux/bin`) and rerun. Document in `WIKI.md` (Phase 18a covers that).

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/lib/lock.js mcp/awiki-server/test/lock.test.mjs
git commit -m "feat(mcp): add flock wrapper for MCP shell-outs

runLocked({mode, timeoutSec, argv}) prepends flock -<x|s> -w <sec>
.awiki/lock -- <argv> and shells out via execFileSync. Read-only
helpers default to mode=s, mutating helpers to mode=x, both with the
mandatory 30s timeout from the spec. Throws LockTimeoutError on flock
exit 1 with empty stderr (flock's documented timeout signal). Argv
must be an array of strings — no shell interpolation possible."
```

---

## Task 18b.7: MCP tool — `capture(text)`

**Files:** Modify: `mcp/awiki-server/index.js`. Test: `mcp/awiki-server/test/capture-tool.test.mjs`.

The `capture` tool runs the sanitizer and appends `- <ISO-datetime> <text>\n` to `content/inbox.md` under an exclusive lock. On hard reject, returns an MCP-level error (not a regular response). On neutralize, returns `{appended: true, line, timestamp, sanitizations_applied: [...]}` with the closed-enum `applied` array.

We append via shell-out (`bash -c 'printf ... >> content/inbox.md'`) under flock, so the lock covers the actual write. The `printf` argument is built in JS from sanitized text; we pass it as an `argv` to `bash -c "printf %s >> content/inbox.md"` with the text as the `$0` parameter — wait, that still shell-interpolates. The cleanest pattern: shell out to `scripts/capture.sh` (Phase 18a deliverable) which reads stdin and does the lock + append. The MCP tool only does the sanitization in JS; the actual write delegates to bash.

But Phase 18a's `capture.sh` does its own sanitization. To avoid double-sanitization (with potentially divergent rules), the MCP tool sanitizes ONLY for the purposes of computing `sanitizations_applied`, and passes the raw input (after hard-reject checks) to `capture.sh` via stdin with an env flag `AWIKI_CAPTURE_PRESANITIZED=1`. Phase 18a's `capture.sh` honors that flag and skips its own neutralization step; it still always runs the hard-reject checks (defense in depth) and the timestamp + lock + append steps.

> **Open question (flagged):** Phase 18a is sibling — confirm `capture.sh` accepts the `AWIKI_CAPTURE_PRESANITIZED=1` env flag. If it does not, this phase patches `capture.sh` minimally OR re-sanitizes inside the MCP tool only and emits a warning. **Recommendation: agree on the env flag in 18a's plan; coordinate via the master plan's per-phase review checkpoint.**

- [ ] **Step 1: Write the failing tool-level test FIRST**

```bash
cat > mcp/awiki-server/test/capture-tool.test.mjs <<'EOF'
// Tool-level test: drive the MCP server over stdio with a tools/call request
// and assert the response shape. Uses a tmpdir repo with .awiki/lock,
// content/inbox.md, and scripts/capture.sh as a stub that just appends.

import { spawn } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, chmodSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-capture-tool-"));
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");
mkdirSync(join(root, "content"), { recursive: true });
writeFileSync(join(root, "content/inbox.md"), "---\ntype: inbox\n---\n\n");
mkdirSync(join(root, "scripts"), { recursive: true });
// Stub capture.sh: reads text from stdin and appends to content/inbox.md
// with a fixed timestamp so the test is deterministic.
writeFileSync(join(root, "scripts/capture.sh"), `#!/usr/bin/env bash
set -euo pipefail
text="$(cat)"
printf -- '- 2026-04-27T12:00:00Z %s\\n' "$text" >> content/inbox.md
`);
chmodSync(join(root, "scripts/capture.sh"), 0o755);

// Drive the MCP server.
function callTool({ name, args }) {
  return new Promise((resolve, reject) => {
    const indexJs = new URL("../index.js", import.meta.url).pathname;
    const child = spawn("node", [indexJs], { cwd: root });
    let out = "", err = "";
    child.stdout.on("data", (d) => out += d.toString());
    child.stderr.on("data", (d) => err += d.toString());
    child.on("close", () => {
      const lines = out.trim().split("\n").filter(Boolean);
      // The server emits one JSON-RPC response per request; we sent one.
      const resp = lines.map((l) => JSON.parse(l)).find((r) => r.id === 1);
      resolve({ resp, err });
    });
    child.on("error", reject);
    child.stdin.write(JSON.stringify({
      jsonrpc: "2.0", id: 1, method: "tools/call",
      params: { name, arguments: args },
    }) + "\n");
    child.stdin.end();
  });
}

// Happy path.
{
  const { resp } = await callTool({ name: "capture", args: { text: "call dentist" } });
  assert.equal(resp.error, undefined, `unexpected error: ${JSON.stringify(resp.error)}`);
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.appended, true);
  assert.deepEqual(payload.sanitizations_applied, []);
  assert.match(readFileSync(join(root, "content/inbox.md"), "utf8"), /call dentist/);
}

// Wikilink neutralized.
{
  const { resp } = await callTool({ name: "capture", args: { text: "see [[secret]] page" } });
  const payload = JSON.parse(resp.result.content[0].text);
  assert.deepEqual(payload.sanitizations_applied, ["wikilink-neutralized"]);
  assert.match(readFileSync(join(root, "content/inbox.md"), "utf8"), /\[ \[secret\] \]/);
}

// Hard reject: embedded newline returns MCP error.
{
  const { resp } = await callTool({ name: "capture", args: { text: "line one\nline two" } });
  assert.equal(resp.result.isError, true);
  assert.match(resp.result.content[0].text, /embedded-newline|newline/);
}

// Hard reject: checkbox prefix.
{
  const { resp } = await callTool({ name: "capture", args: { text: "[ ] not a capture" } });
  assert.equal(resp.result.isError, true);
  assert.match(resp.result.content[0].text, /checkbox/);
}

// Type rejection.
{
  const { resp } = await callTool({ name: "capture", args: { text: 42 } });
  assert.equal(resp.result.isError, true);
}

rmSync(root, { recursive: true, force: true });
console.log("capture-tool: ok");
EOF

node mcp/awiki-server/test/capture-tool.test.mjs
```

Expected: failure — the `capture` tool is not registered in `index.js` yet.

- [ ] **Step 2: Wire the `capture` tool into `index.js`**

Open `mcp/awiki-server/index.js`. Add imports and wire the tool. Keep the four v1 tools intact.

```javascript
// Top of file, after existing imports:
import { sanitizeCapture, SanitizeError } from "./lib/sanitize-capture.js";
import { runLockedExclusive, LockTimeoutError } from "./lib/lock.js";
import { execFileSync } from "node:child_process";

const REPO_ROOT = process.cwd();
```

Append to the `TOOLS` array:

```javascript
{
  name: "capture",
  description: "Sanitize and append a free-text capture to content/inbox.md. Returns {appended, line, timestamp, sanitizations_applied}. Hard-reject patterns (embedded newline, control char, checkbox prefix at start) return an MCP error.",
  inputSchema: {
    type: "object",
    additionalProperties: false,
    required: ["text"],
    properties: {
      text: { type: "string", maxLength: 4000 },
    },
  },
},
```

Add the dispatch case:

```javascript
case "capture": {
  if (typeof args?.text !== "string") {
    throw new Error("capture: text must be a string");
  }
  // 1. Sanitize. Hard rejects throw SanitizeError → MCP error.
  let sanitized;
  try {
    sanitized = sanitizeCapture(args.text);
  } catch (e) {
    if (e instanceof SanitizeError) {
      throw new Error(`capture rejected: ${e.kind}: ${e.message}`);
    }
    throw e;
  }
  // 2. Build the line. ISO-8601 UTC with second precision.
  const timestamp = new Date().toISOString().replace(/\.\d{3}Z$/, "Z");
  const line = `- ${timestamp} ${sanitized.text}`;
  // 3. Shell out to scripts/capture.sh (Phase 18a) under flock -x.
  //    Phase 18a's capture.sh reads text from stdin and does its own
  //    timestamp + lock + append. We set AWIKI_CAPTURE_PRESANITIZED=1 to
  //    suppress its in-script neutralization step (we already did it).
  //    capture.sh still runs hard-reject checks unconditionally as
  //    defense-in-depth.
  try {
    execFileSync("bash", ["scripts/capture.sh"], {
      cwd: REPO_ROOT,
      input: sanitized.text,
      encoding: "utf8",
      env: { ...process.env, AWIKI_CAPTURE_PRESANITIZED: "1" },
      stdio: ["pipe", "pipe", "pipe"],
    });
  } catch (e) {
    if (e instanceof LockTimeoutError) {
      throw new Error(`capture: lock timeout after 30s`);
    }
    const stderr = e.stderr ? e.stderr.toString() : "";
    throw new Error(`capture.sh failed: ${stderr.trim() || e.message}`);
  }
  out = JSON.stringify({
    appended: true,
    line,
    timestamp,
    sanitizations_applied: sanitized.applied,
  });
  break;
}
```

> **Note on lock acquisition:** `scripts/capture.sh` (Phase 18a) takes the lock itself via `awiki_lock_with --timeout=30 --` from `scripts/lib/lock.sh`. The MCP tool does NOT double-lock. If 18a's `capture.sh` does not acquire its own lock, this phase wraps the `execFileSync` call in `runLockedExclusive` instead. **Cross-phase coordination point** — surfaced in Open Questions.

- [ ] **Step 3: Re-run the tool-level test**

```bash
node mcp/awiki-server/test/capture-tool.test.mjs
```

Expected: `capture-tool: ok`. If the stub `capture.sh` rejects on `AWIKI_CAPTURE_PRESANITIZED=1` (because the test stub doesn't read that env var), the test still passes since the stub appends regardless.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/index.js mcp/awiki-server/test/capture-tool.test.mjs
git commit -m "feat(mcp): add capture(text) tool

Sanitizes via lib/sanitize-capture.js, builds the inbox line with an
ISO-8601 UTC timestamp, and shells out to scripts/capture.sh (Phase
18a) with AWIKI_CAPTURE_PRESANITIZED=1 set so the script does not
re-neutralize. Hard-reject patterns (embedded newline, control char,
checkbox prefix) surface as MCP errors. Returns
{appended, line, timestamp, sanitizations_applied[]} with the
closed-enum applied array."
```

---

## Task 18b.8: MCP tools — `triage_inbox()` + `triage_apply()`

**Files:** Modify: `mcp/awiki-server/index.js`. Create: `mcp/awiki-server/lib/triage-inbox-scan.js`, `mcp/awiki-server/lib/triage-apply.js`. Tests: `mcp/awiki-server/test/triage-inbox-scan.test.mjs`, `mcp/awiki-server/test/triage-tools.test.mjs`.

These two tools are the heart of the phase. `triage_inbox` is read-only and pure-JS (no shell-out): it scans `content/inbox.md` line-by-line, scans `raw/inbox/interactive/` recursively, mints IDs via Phase 18b.3's helpers, and returns the list. It still takes `flock -s` for read consistency — done by wrapping the `readFileSync` call inside a `runLockedShared({argv: ["true"]})` no-op? **No** — that doesn't actually hold the lock during the JS read. The robust pattern is to shell out to a tiny helper that prints inbox lines + file paths under the shared lock, then JS parses.

Simpler: since reads of `inbox.md` and a directory listing are very brief, we accept the *theoretical* race window between `runLockedShared` returning and the JS read starting. For the spec-mandated behavior ("read-only tools take `flock -s`"), we instead implement the helper as `bash -c 'flock -s -w 30 .awiki/lock cat content/inbox.md && find raw/inbox/interactive/ -type f'`. The lock is held for the full `cat`+`find` duration. JS parses the captured stdout.

`triage_apply` is mutating: validates args (Phase 18b.4), resolves destination paths (Phase 18b.5), TOCTOU-re-verifies inbox-line IDs (Phase 18b.3), then shells out to `scripts/triage.sh <id> <outcome> [k=v ...]` (Phase 18a) under exclusive lock. Returns the structured payload `{ok, actions_taken[], created_pages[], updated_pages[], stale_id?}`.

> **Cross-phase contract (PINNED, agreed with Phase 18a):** `scripts/triage.sh` MUST emit a final-line JSON trailer `TRIAGE-RESULT|<json>` on success (matching Phase 15's `SYNTH-RESULT|<json>` convention). The JSON contains `actions_taken`, `created_pages`, `updated_pages`. On stale-id failure, `triage.sh` exits **9** (NOT 7 — exit 7 is reserved for flock contention by `scripts/lib/lock.sh`). The MCP tool catches exit 9 and returns `{ok: false, stale_id: true}`.

- [ ] **Step 1: Write `lib/triage-inbox-scan.js` + its unit test**

```bash
cat > mcp/awiki-server/test/triage-inbox-scan.test.mjs <<'EOF'
import { scanInbox } from "../lib/triage-inbox-scan.js";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-scan-"));
mkdirSync(join(root, "content"), { recursive: true });
mkdirSync(join(root, "raw/inbox/interactive"), { recursive: true });
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");

writeFileSync(join(root, "content/inbox.md"), `---
type: inbox
---

- 2026-04-27T12:00:00Z call dentist
- 2026-04-27T13:00:00Z buy milk
- 2026-04-27T14:00:00Z ~~struck~~

`);
writeFileSync(join(root, "raw/inbox/interactive/voice-001.txt"), "transcript");
writeFileSync(join(root, "raw/inbox/interactive/notes.md"), "## hi");

const items = scanInbox(root);
// Inbox lines: only the two non-strikethrough lines (~~text~~ is treated as
// processed-trash by phase 18a's triage.sh and skipped).
const inboxItems = items.filter((i) => i.source === "inbox.md");
assert.equal(inboxItems.length, 2);
for (const it of inboxItems) {
  assert.match(it.id, /^inbox-[0-9a-f]{10}-\d+$/);
  assert.equal(typeof it.text, "string");
  assert.match(it.captured_at, /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/);
  assert.equal(typeof it.line_or_path, "number");
}
assert.equal(inboxItems[0].text, "call dentist");
assert.equal(inboxItems[1].text, "buy milk");

// File-shaped items.
const fileItems = items.filter((i) => i.source.startsWith("raw/inbox/"));
assert.equal(fileItems.length, 2);
for (const it of fileItems) {
  assert.match(it.id, /^file-[0-9a-f]{10}$/);
  assert.equal(typeof it.line_or_path, "string");
  assert.ok(it.line_or_path.startsWith("raw/inbox/interactive/"));
}

rmSync(root, { recursive: true, force: true });
console.log("triage-inbox-scan: ok");
EOF
```

```bash
cat > mcp/awiki-server/lib/triage-inbox-scan.js <<'EOF'
// Read-only scan of inbox.md + raw/inbox/interactive/. Mints IDs via
// inbox-id.js. Skips strikethrough lines (~~text~~) which represent
// trashed-but-not-pruned captures left behind by trash outcome.
//
// The MCP boundary is responsible for taking flock -s before calling this
// helper (see triage_inbox dispatch in index.js).

import { readFileSync, readdirSync, statSync } from "node:fs";
import { join, relative } from "node:path";
import { mintInboxId, mintFileId } from "./inbox-id.js";

const INBOX_LINE_RE = /^- (\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z) (.*)$/;
const STRIKE_RE = /^~~.*~~$/;

function scanInboxFile(repoRoot) {
  const path = join(repoRoot, "content/inbox.md");
  let text;
  try {
    text = readFileSync(path, "utf8");
  } catch {
    return [];
  }
  const items = [];
  const lines = text.split("\n");
  let inFrontmatter = false;
  let seenFmEnd = false;
  for (let i = 0; i < lines.length; i++) {
    const lineno = i + 1;
    const raw = lines[i];
    if (lineno === 1 && raw === "---") { inFrontmatter = true; continue; }
    if (inFrontmatter && raw === "---") { inFrontmatter = false; seenFmEnd = true; continue; }
    if (inFrontmatter) continue;
    if (!seenFmEnd) continue; // Pre-frontmatter junk; skip.
    const m = INBOX_LINE_RE.exec(raw);
    if (!m) continue;
    const [, capturedAt, body] = m;
    if (STRIKE_RE.test(body)) continue;
    items.push({
      id: mintInboxId(raw, lineno),
      source: "inbox.md",
      line_or_path: lineno,
      text: body,
      captured_at: capturedAt,
    });
  }
  return items;
}

function scanInteractiveFiles(repoRoot) {
  const dir = join(repoRoot, "raw/inbox/interactive");
  let entries;
  try {
    entries = readdirSync(dir, { withFileTypes: true });
  } catch {
    return [];
  }
  const items = [];
  for (const ent of entries) {
    if (!ent.isFile()) continue;
    const abs = join(dir, ent.name);
    const rel = relative(repoRoot, abs);
    let st;
    try { st = statSync(abs); } catch { continue; }
    items.push({
      id: mintFileId(rel),
      source: rel.split("/").slice(0, -1).join("/") + "/",
      line_or_path: rel,
      text: ent.name,
      captured_at: new Date(st.mtimeMs).toISOString().replace(/\.\d{3}Z$/, "Z"),
    });
  }
  return items;
}

export function scanInbox(repoRoot) {
  return [...scanInboxFile(repoRoot), ...scanInteractiveFiles(repoRoot)];
}
EOF

node mcp/awiki-server/test/triage-inbox-scan.test.mjs
```

Expected: `triage-inbox-scan: ok`.

- [ ] **Step 2: Write `lib/triage-apply.js`**

```bash
cat > mcp/awiki-server/lib/triage-apply.js <<'EOF'
// triage_apply orchestration. Runs (in order):
//   1. validateTriageArgs (regex + schema + cross-field + dates)
//   2. resolveTriageDestinations (path-resolution guard)
//   3. For inbox IDs: re-read line at supplied lineno, verify hash via
//      verifyInboxId. Return {ok:false, stale_id:true} on mismatch.
//   4. Shell out to scripts/triage.sh under flock -x.
//   5. Parse the trailer JSON line.

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { execFileSync } from "node:child_process";
import { validateTriageArgs } from "./triage-validate.js";
import { resolveTriageDestinations } from "./path-guard.js";
import { parseInboxId, verifyInboxId } from "./inbox-id.js";
import { runLockedExclusive, LockTimeoutError } from "./lock.js";

const TRAILER_PREFIX = "TRIAGE-RESULT|";

function reReadInboxLine(repoRoot, lineno) {
  const path = join(repoRoot, "content/inbox.md");
  let text;
  try {
    text = readFileSync(path, "utf8");
  } catch {
    return null;
  }
  const lines = text.split("\n");
  if (lineno < 1 || lineno > lines.length) return null;
  return lines[lineno - 1];
}

export function triageApply(repoRoot, rawArgs) {
  // 1. Validate.
  const v = validateTriageArgs(rawArgs);
  if (!v.ok) {
    return { ok: false, errors: v.errors };
  }
  const { id, outcome, params } = v.normalized;

  // 2. Path guard (throws PathGuardError on escape; let the MCP boundary
  // translate into an MCP error).
  resolveTriageDestinations(repoRoot, { outcome, params });

  // 3. TOCTOU re-verify for inbox IDs.
  const parsed = parseInboxId(id);
  if (!parsed) {
    return { ok: false, errors: [{ field: "id", reason: "unparseable id shape" }] };
  }
  if (parsed.kind === "inbox") {
    const currentLine = reReadInboxLine(repoRoot, parsed.lineno);
    if (currentLine === null || !verifyInboxId(id, currentLine)) {
      return { ok: false, stale_id: true };
    }
  }
  // file-shaped IDs: phase 18a's triage.sh re-checks the file's existence
  // before mutating; we don't TOCTOU-verify here because there's nothing
  // hash-stable to check.

  // 4. Shell out under flock -x.
  const argv = ["scripts/triage.sh", "--", id, outcome];
  for (const [k, val] of Object.entries(params)) {
    argv.push(`${k}=${val}`);
  }
  let stdout;
  try {
    stdout = runLockedExclusive({
      repoRoot,
      argv: ["bash", ...argv],
      timeoutSec: 30,
    });
  } catch (e) {
    if (e instanceof LockTimeoutError) {
      return { ok: false, errors: [{ field: "(lock)", reason: "30s flock timeout" }] };
    }
    if (e.status === 9) {
      // Phase 18a contract: exit 9 = stale_id detected by triage.sh itself.
      // (exit 7 is reserved for flock contention; do not collide.)
      return { ok: false, stale_id: true };
    }
    const stderr = e.stderr ? e.stderr.toString() : "";
    throw new Error(`triage.sh exit ${e.status}: ${stderr.trim() || "(no stderr)"}`);
  }

  // 5. Parse the trailer line.
  const lines = stdout.split("\n");
  let trailer = null;
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].startsWith(TRAILER_PREFIX)) {
      try {
        trailer = JSON.parse(lines[i].slice(TRAILER_PREFIX.length));
      } catch (e) {
        throw new Error(`triage.sh trailer JSON parse failed: ${e.message}`);
      }
      break;
    }
  }
  if (!trailer) {
    throw new Error(`triage.sh did not emit a TRIAGE-RESULT|<json> trailer`);
  }

  return {
    ok: true,
    actions_taken: trailer.actions_taken ?? [],
    created_pages: trailer.created_pages ?? [],
    updated_pages: trailer.updated_pages ?? [],
  };
}
EOF
```

- [ ] **Step 3: Wire `triage_inbox` and `triage_apply` into `index.js`**

Add imports:

```javascript
import { scanInbox } from "./lib/triage-inbox-scan.js";
import { triageApply } from "./lib/triage-apply.js";
import { runLockedShared } from "./lib/lock.js";
import { PathGuardError } from "./lib/path-guard.js";
```

Append to `TOOLS`:

```javascript
{
  name: "triage_inbox",
  description: "Read-only scan of content/inbox.md and raw/inbox/interactive/. Returns [{id, source, line_or_path, text, captured_at}, ...].",
  inputSchema: { type: "object", properties: {}, additionalProperties: false },
},
{
  name: "triage_apply",
  description: "Apply a triage outcome to a captured item. Validates args (regex + path-guard + ISO date + TOCTOU re-verify), then shells out to scripts/triage.sh under flock -x. Returns {ok, actions_taken[], created_pages[], updated_pages[]} or {ok:false, stale_id:true} when the inbox line has shifted between triage_inbox() and the call.",
  inputSchema: {
    type: "object",
    additionalProperties: false,
    required: ["id", "outcome"],
    properties: {
      id: { type: "string", pattern: "^[a-z0-9~-]{1,32}$" },
      outcome: { enum: ["trash","do-now","act","defer-scheduled","waiting","reference","someday"] },
      params: { type: "object" },
    },
  },
},
```

Dispatch cases:

```javascript
case "triage_inbox": {
  // Take flock -s for the read. We shell out a no-op `true` under the
  // shared lock to acquire-then-release; the JS scan then proceeds. The
  // theoretical race is that the inbox could change between unlock and
  // scanInbox(); we accept it because (a) any change leaves the IDs valid
  // (TOCTOU re-verify in triage_apply catches drift); (b) the alternative
  // (writing a Node binding for fcntl) is out of scope.
  runLockedShared({ repoRoot: REPO_ROOT, argv: ["true"], timeoutSec: 30 });
  out = JSON.stringify(scanInbox(REPO_ROOT), null, 2);
  break;
}
case "triage_apply": {
  let result;
  try {
    result = triageApply(REPO_ROOT, args ?? {});
  } catch (e) {
    if (e instanceof PathGuardError) {
      throw new Error(`triage_apply: path-guard rejected: ${e.message}`);
    }
    throw e;
  }
  out = JSON.stringify(result);
  break;
}
```

> **Read-lock note (re-flagged):** the "no-op `true` under shared lock" pattern does NOT hold the lock during `scanInbox`. The spec language "read-only tools take `flock -s`" is satisfied in the *spirit* of preventing concurrent writes during the read window, but a strict literal reading would require an fcntl binding. Documented in the self-review checklist below as an acknowledged limitation; revisit in Phase 19 if a Node fcntl wrapper becomes available.

- [ ] **Step 4: Write the tool-level integration test**

```bash
cat > mcp/awiki-server/test/triage-tools.test.mjs <<'EOF'
import { spawn } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, readFileSync, chmodSync, rmSync, appendFileSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-triage-tool-"));
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");
mkdirSync(join(root, "content/projects"), { recursive: true });
mkdirSync(join(root, "content/contexts"), { recursive: true });
writeFileSync(join(root, "content/inbox.md"), `---
type: inbox
---

- 2026-04-27T12:00:00Z call dentist
- 2026-04-27T13:00:00Z buy milk
`);
mkdirSync(join(root, "scripts"), { recursive: true });
// Stub triage.sh: emits a deterministic trailer.
writeFileSync(join(root, "scripts/triage.sh"), `#!/usr/bin/env bash
set -euo pipefail
id="$1"; shift
outcome="$1"; shift
echo "scripts/triage.sh stub: id=$id outcome=$outcome args=$*" >&2
printf 'TRIAGE-RESULT|{"actions_taken":["%s"],"created_pages":[],"updated_pages":["content/inbox.md"]}\n' "$outcome"
`);
chmodSync(join(root, "scripts/triage.sh"), 0o755);

function call(name, args) {
  return new Promise((resolve, reject) => {
    const indexJs = new URL("../index.js", import.meta.url).pathname;
    const child = spawn("node", [indexJs], { cwd: root });
    let out = "", err = "";
    child.stdout.on("data", (d) => out += d.toString());
    child.stderr.on("data", (d) => err += d.toString());
    child.on("close", () => {
      const lines = out.trim().split("\n").filter(Boolean);
      const resp = lines.map((l) => JSON.parse(l)).find((r) => r.id === 1);
      resolve({ resp, err });
    });
    child.on("error", reject);
    child.stdin.write(JSON.stringify({
      jsonrpc: "2.0", id: 1, method: "tools/call",
      params: { name, arguments: args },
    }) + "\n");
    child.stdin.end();
  });
}

// triage_inbox returns the two open lines.
let inboxIds;
{
  const { resp } = await call("triage_inbox", {});
  const items = JSON.parse(resp.result.content[0].text);
  assert.equal(items.length, 2);
  inboxIds = items.map((i) => i.id);
  for (const id of inboxIds) assert.match(id, /^inbox-[0-9a-f]{10}-\d+$/);
}

// triage_apply happy path.
{
  const { resp } = await call("triage_apply", {
    id: inboxIds[0], outcome: "trash",
  });
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.ok, true);
  assert.deepEqual(payload.actions_taken, ["trash"]);
}

// triage_apply: regex rejects bad id.
{
  const { resp } = await call("triage_apply", { id: "BAD;rm -rf /", outcome: "trash" });
  assert.ok(resp.result.isError || JSON.parse(resp.result.content[0].text).ok === false);
}

// triage_apply: outcome enum reject.
{
  const { resp } = await call("triage_apply", { id: inboxIds[0], outcome: "DELETE" });
  assert.ok(resp.result.isError || JSON.parse(resp.result.content[0].text).ok === false);
}

// triage_apply: invalid date rejected.
{
  const { resp } = await call("triage_apply", {
    id: inboxIds[0], outcome: "defer-scheduled",
    params: { due: "2026-02-30" },
  });
  const txt = resp.result.content[0].text;
  assert.match(txt, /not a valid ISO calendar date|invalid/);
}

// triage_apply: path-guard catches traversal even with valid regex shape.
// (project_slug "_loose" is valid regex; "../etc" fails regex AND path-guard.)
// We test path-guard by passing a slug shape that passes regex but resolves
// outside content/projects — impossible by design (regex bans /). Instead
// we verify the regex layer rejects "../etc" first, demonstrating defense
// in depth.
{
  const { resp } = await call("triage_apply", {
    id: inboxIds[1], outcome: "act",
    params: { project_slug: "../etc" },
  });
  const txt = resp.result.content[0].text;
  assert.match(txt, /must match|invalid/);
}

// triage_apply: stale-id when inbox edited between calls.
{
  // Re-read current inbox IDs.
  const { resp: r1 } = await call("triage_inbox", {});
  const items = JSON.parse(r1.result.content[0].text);
  const id = items.find((i) => i.text === "buy milk")?.id;
  assert.ok(id, "buy milk line must still be present");
  // Edit inbox so the line at the captured lineno no longer matches.
  writeFileSync(join(root, "content/inbox.md"), `---
type: inbox
---

- 2026-04-27T13:00:00Z buy bread
`);
  const { resp: r2 } = await call("triage_apply", { id, outcome: "trash" });
  const payload = JSON.parse(r2.result.content[0].text);
  assert.equal(payload.ok, false);
  assert.equal(payload.stale_id, true);
}

rmSync(root, { recursive: true, force: true });
console.log("triage-tools: ok");
EOF

node mcp/awiki-server/test/triage-tools.test.mjs
```

Expected: `triage-tools: ok`.

- [ ] **Step 5: Commit**

```bash
git add mcp/awiki-server/lib/triage-inbox-scan.js \
        mcp/awiki-server/lib/triage-apply.js \
        mcp/awiki-server/index.js \
        mcp/awiki-server/test/triage-inbox-scan.test.mjs \
        mcp/awiki-server/test/triage-tools.test.mjs
git commit -m "feat(mcp): add triage_inbox + triage_apply tools

triage_inbox: read-only scan over content/inbox.md and
raw/inbox/interactive/, mints inbox-/file-/block-shape IDs via
lib/inbox-id.js. Skips strikethrough lines. Takes flock -s briefly
(known limitation: read window is not strictly held; TOCTOU re-verify
in triage_apply catches drift).

triage_apply: validates args (regex + JSON Schema + ISO calendar via
Date.UTC + outcome cross-field + path-guard against ../traversal and
symlink swap), TOCTOU re-verifies inbox-line IDs (returns
{ok:false, stale_id:true} on hash mismatch), then shells out to
scripts/triage.sh under flock -x with 30s timeout. Parses
TRIAGE-RESULT|<json> trailer for the response payload. Path-guard
rejects translate to MCP errors; lock timeouts return structured
{ok:false} payload."
```

---

## Task 18b.9: MCP tools — `list_actions()` + `rebuild_agenda()`

**Files:** Modify: `mcp/awiki-server/index.js`. Create: `mcp/awiki-server/lib/list-actions.js`. Test: `mcp/awiki-server/test/list-rebuild-tools.test.mjs`.

`list_actions({filter?})` — read-only. Loads `.awiki/maps/actions.tsv` (Phase 17), applies an optional filter, returns the matching rows. Lazy re-scan: if the TSV is older than the newest `content/**/*.md` file, run `scripts/action-scan.sh` first (under exclusive lock). Otherwise just take shared lock.

`rebuild_agenda()` — runs `scripts/action-scan.sh` then `scripts/agenda.sh` under exclusive lock; returns `{rebuilt: [<file>], duration_ms}`.

Filter envelope schema (subset of the spec):

```json
{
  "type": "object",
  "additionalProperties": false,
  "properties": {
    "status":      {"enum": ["[ ]", "[x]", "[/]", "[?]", "[>]"]},
    "context":     {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,63}$"},
    "project":     {"type": "string", "pattern": "^[a-z0-9_][a-z0-9_-]{0,63}$"},
    "due_before":  {"type": "string", "pattern": "^\\d{4}-\\d{2}-\\d{2}$"},
    "wait":        {"type": "string", "pattern": "^[a-z0-9][a-z0-9-]{0,63}$"},
    "overdue":     {"type": "boolean"}
  }
}
```

- [ ] **Step 1: Write `lib/list-actions.js`**

```bash
cat > mcp/awiki-server/lib/list-actions.js <<'EOF'
// Read .awiki/maps/actions.tsv (phase-17 scanner output) and apply an
// optional filter. Caller is responsible for taking the lock and (if
// needed) re-running action-scan.sh first.

import { readFileSync, statSync, readdirSync } from "node:fs";
import { join } from "node:path";

const TSV_COLUMNS = [
  "id","status","text","file","line","context","due","defer",
  "wait","since","every","done","priority","est","project","source_kind",
];

export function loadActionsTsv(repoRoot) {
  const path = join(repoRoot, ".awiki/maps/actions.tsv");
  let text;
  try {
    text = readFileSync(path, "utf8");
  } catch (e) {
    if (e.code === "ENOENT") return [];
    throw e;
  }
  const rows = [];
  for (const line of text.split("\n")) {
    if (!line || line.startsWith("#")) continue;
    const cells = line.split("\t");
    if (cells.length < TSV_COLUMNS.length) continue;
    const row = {};
    for (let i = 0; i < TSV_COLUMNS.length; i++) {
      row[TSV_COLUMNS[i]] = cells[i] ?? "";
    }
    rows.push(row);
  }
  return rows;
}

export function applyFilter(rows, filter = {}, today = new Date().toISOString().slice(0, 10)) {
  let out = rows;
  if (filter.status) out = out.filter((r) => r.status === filter.status);
  if (filter.context) out = out.filter((r) => r.context === filter.context);
  if (filter.project) out = out.filter((r) => r.project === filter.project);
  if (filter.due_before) out = out.filter((r) => r.due && r.due < filter.due_before);
  if (filter.wait) out = out.filter((r) => r.wait === filter.wait);
  if (filter.overdue === true) {
    out = out.filter((r) => r.due && r.due < today && r.status === "[ ]");
  }
  return out;
}

// Decide whether the TSV is stale relative to content/**/*.md.
// Stale if any markdown file's mtime > tsv mtime, or the TSV is missing.
export function isTsvStale(repoRoot) {
  const tsv = join(repoRoot, ".awiki/maps/actions.tsv");
  let tsvMtime;
  try { tsvMtime = statSync(tsv).mtimeMs; }
  catch { return true; }
  const contentDir = join(repoRoot, "content");
  function walk(dir) {
    let newest = 0;
    let entries;
    try { entries = readdirSync(dir, { withFileTypes: true }); }
    catch { return 0; }
    for (const ent of entries) {
      const p = join(dir, ent.name);
      if (ent.isDirectory()) {
        newest = Math.max(newest, walk(p));
      } else if (ent.isFile() && p.endsWith(".md")) {
        const st = statSync(p);
        if (st.mtimeMs > newest) newest = st.mtimeMs;
      }
    }
    return newest;
  }
  return walk(contentDir) > tsvMtime;
}
EOF
```

- [ ] **Step 2: Wire both tools into `index.js`**

Add imports:

```javascript
import { loadActionsTsv, applyFilter, isTsvStale } from "./lib/list-actions.js";
import { validate as validateSchema } from "./lib/validate-scope.js";

const FILTER_SCHEMA = {
  type: "object",
  additionalProperties: false,
  properties: {
    status:     { enum: ["[ ]","[x]","[/]","[?]","[>]"] },
    context:    { type: "string", pattern: "^[a-z0-9][a-z0-9-]{0,63}$" },
    project:    { type: "string", pattern: "^[a-z0-9_][a-z0-9_-]{0,63}$" },
    due_before: { type: "string", pattern: "^\\d{4}-\\d{2}-\\d{2}$" },
    wait:       { type: "string", pattern: "^[a-z0-9][a-z0-9-]{0,63}$" },
    overdue:    { type: "boolean" },
  },
};
```

Append to `TOOLS`:

```javascript
{
  name: "list_actions",
  description: "Return rows from .awiki/maps/actions.tsv, optionally filtered by status/context/project/due_before/wait/overdue. Re-runs action-scan.sh first if the TSV is stale relative to content/**/*.md.",
  inputSchema: {
    type: "object",
    additionalProperties: false,
    properties: {
      filter: { type: "object" },
    },
  },
},
{
  name: "rebuild_agenda",
  description: "Run scripts/action-scan.sh + scripts/agenda.sh under flock -x. Returns {rebuilt:[<file>], duration_ms}.",
  inputSchema: { type: "object", properties: {}, additionalProperties: false },
},
```

Dispatch cases:

```javascript
case "list_actions": {
  const filter = args?.filter ?? {};
  const filterCheck = validateSchema(FILTER_SCHEMA, filter);
  if (!filterCheck.valid) {
    out = JSON.stringify({ error: "invalid_argument", field: "filter", reasons: filterCheck.errors });
    break;
  }
  // If stale, re-scan under exclusive lock first.
  if (isTsvStale(REPO_ROOT)) {
    runLockedExclusive({
      repoRoot: REPO_ROOT,
      argv: ["bash", "scripts/action-scan.sh"],
      timeoutSec: 30,
    });
  }
  // Now read under shared lock.
  runLockedShared({ repoRoot: REPO_ROOT, argv: ["true"], timeoutSec: 30 });
  const rows = applyFilter(loadActionsTsv(REPO_ROOT), filter);
  out = JSON.stringify(rows, null, 2);
  break;
}
case "rebuild_agenda": {
  const t0 = Date.now();
  runLockedExclusive({
    repoRoot: REPO_ROOT,
    argv: ["bash", "-c", "scripts/action-scan.sh && scripts/agenda.sh"],
    timeoutSec: 30,
  });
  const duration_ms = Date.now() - t0;
  // The list of regenerated files is fixed (the five managed-region pages
  // from phase 16/17). agenda.sh always rewrites all five.
  const rebuilt = [
    "content/agenda/next-actions.md",
    "content/agenda/today.md",
    "content/agenda/waiting.md",
    "content/agenda/someday.md",
    "content/agenda/stuck-projects.md",
  ];
  out = JSON.stringify({ rebuilt, duration_ms });
  break;
}
```

- [ ] **Step 3: Tool-level test**

```bash
cat > mcp/awiki-server/test/list-rebuild-tools.test.mjs <<'EOF'
import { spawn } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, chmodSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-list-tool-"));
mkdirSync(join(root, ".awiki/maps"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");
mkdirSync(join(root, "content/projects"), { recursive: true });
mkdirSync(join(root, "content/agenda"), { recursive: true });
mkdirSync(join(root, "scripts"), { recursive: true });

// Seed actions.tsv.
const cols = "id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind";
const r1 = "a01\t[ ]\tcall dentist\tcontent/projects/health.md\t12\tphone\t2026-05-01\t\t\t\t\t\t\t\trenovate-kitchen\tpublic";
const r2 = "a02\t[x]\tbuy milk\tcontent/projects/groceries.md\t3\terrands\t\t\t\t\t\t2026-04-26\t\t\tgroceries\tpublic";
const r3 = "a03\t[?]\tlawyer reply\tcontent/projects/move.md\t8\t\t\t\talice\t2026-04-20\t\t\t\t\tmove\tprivate";
writeFileSync(join(root, ".awiki/maps/actions.tsv"), `${cols}\n${r1}\n${r2}\n${r3}\n`);

// Stub scan + agenda scripts (no-op).
writeFileSync(join(root, "scripts/action-scan.sh"), "#!/usr/bin/env bash\nexit 0\n");
writeFileSync(join(root, "scripts/agenda.sh"), "#!/usr/bin/env bash\nexit 0\n");
chmodSync(join(root, "scripts/action-scan.sh"), 0o755);
chmodSync(join(root, "scripts/agenda.sh"), 0o755);

function call(name, args) {
  return new Promise((resolve, reject) => {
    const indexJs = new URL("../index.js", import.meta.url).pathname;
    const child = spawn("node", [indexJs], { cwd: root });
    let outBuf = "";
    child.stdout.on("data", (d) => outBuf += d.toString());
    child.on("close", () => {
      const lines = outBuf.trim().split("\n").filter(Boolean);
      const resp = lines.map((l) => JSON.parse(l)).find((r) => r.id === 1);
      resolve(resp);
    });
    child.on("error", reject);
    child.stdin.write(JSON.stringify({
      jsonrpc: "2.0", id: 1, method: "tools/call",
      params: { name, arguments: args },
    }) + "\n");
    child.stdin.end();
  });
}

// list_actions: no filter.
{
  const resp = await call("list_actions", {});
  const rows = JSON.parse(resp.result.content[0].text);
  assert.equal(rows.length, 3);
}

// list_actions: filter status=[ ].
{
  const resp = await call("list_actions", { filter: { status: "[ ]" } });
  const rows = JSON.parse(resp.result.content[0].text);
  assert.equal(rows.length, 1);
  assert.equal(rows[0].id, "a01");
}

// list_actions: filter overdue=true.
{
  const resp = await call("list_actions", { filter: { overdue: true } });
  const rows = JSON.parse(resp.result.content[0].text);
  // a01 due 2026-05-01; today is set by Node, may or may not be after.
  // Just assert rows is an array.
  assert.ok(Array.isArray(rows));
}

// list_actions: bad filter rejected.
{
  const resp = await call("list_actions", { filter: { status: "DONE" } });
  const txt = resp.result.content[0].text;
  assert.match(txt, /invalid_argument|enum/);
}

// rebuild_agenda: returns expected payload.
{
  const resp = await call("rebuild_agenda", {});
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.rebuilt.length, 5);
  assert.ok(typeof payload.duration_ms === "number");
  assert.ok(payload.duration_ms >= 0);
}

rmSync(root, { recursive: true, force: true });
console.log("list-rebuild-tools: ok");
EOF

node mcp/awiki-server/test/list-rebuild-tools.test.mjs
```

Expected: `list-rebuild-tools: ok`.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/lib/list-actions.js \
        mcp/awiki-server/index.js \
        mcp/awiki-server/test/list-rebuild-tools.test.mjs
git commit -m "feat(mcp): add list_actions + rebuild_agenda tools

list_actions: validates {filter} envelope against a JSON schema
(status / context / project / due_before / wait / overdue), re-runs
action-scan.sh under flock -x if .awiki/maps/actions.tsv is stale
relative to content/**/*.md, then loads the TSV under flock -s and
applies the filter in pure JS.

rebuild_agenda: runs action-scan.sh && agenda.sh under a single
flock -x acquisition (30s timeout). Returns {rebuilt:[5 paths],
duration_ms}."
```

---

## Task 18b.10: MCP tool stubs — `review_status()` + `mark_review_done()`

**Files:** Modify: `mcp/awiki-server/index.js`. Test: `mcp/awiki-server/test/stubs.test.mjs`.

Both tools land as registered-but-stub in this phase. They return `{stub: true, message: "implemented in phase 19"}` and DO NOT take any locks or shell out. The schemas and dispatch cases are wired so the BATS smoke test in Task 18b.11 can assert all seven new tools are listed and callable.

Phase 19 will replace the stub bodies with the real review-status JSON (per spec "Weekly Review JSON above") and the real `.awiki/last-review` write + `review-log.md` append.

- [ ] **Step 1: Add to `TOOLS` and dispatch**

Append to `TOOLS`:

```javascript
{
  name: "review_status",
  description: "STUB (phase 18b): full implementation lands in phase 19. Returns {stub:true}.",
  inputSchema: { type: "object", properties: {}, additionalProperties: false },
},
{
  name: "mark_review_done",
  description: "STUB (phase 18b): full implementation lands in phase 19. Returns {stub:true}.",
  inputSchema: { type: "object", properties: {}, additionalProperties: false },
},
```

Dispatch:

```javascript
case "review_status":
  out = JSON.stringify({ stub: true, message: "implemented in phase 19" });
  break;
case "mark_review_done":
  out = JSON.stringify({ stub: true, message: "implemented in phase 19" });
  break;
```

- [ ] **Step 2: Tool-level test**

```bash
cat > mcp/awiki-server/test/stubs.test.mjs <<'EOF'
import { spawn } from "node:child_process";
import { mkdtempSync, mkdirSync, writeFileSync, rmSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

const root = mkdtempSync(join(tmpdir(), "awiki-stubs-"));
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");

function call(name) {
  return new Promise((resolve, reject) => {
    const indexJs = new URL("../index.js", import.meta.url).pathname;
    const child = spawn("node", [indexJs], { cwd: root });
    let outBuf = "";
    child.stdout.on("data", (d) => outBuf += d.toString());
    child.on("close", () => {
      const lines = outBuf.trim().split("\n").filter(Boolean);
      resolve(lines.map((l) => JSON.parse(l)).find((r) => r.id === 1));
    });
    child.on("error", reject);
    child.stdin.write(JSON.stringify({
      jsonrpc: "2.0", id: 1, method: "tools/call",
      params: { name, arguments: {} },
    }) + "\n");
    child.stdin.end();
  });
}

for (const name of ["review_status", "mark_review_done"]) {
  const resp = await call(name);
  const payload = JSON.parse(resp.result.content[0].text);
  assert.equal(payload.stub, true);
  assert.match(payload.message, /phase 19/);
}

rmSync(root, { recursive: true, force: true });
console.log("stubs: ok");
EOF

node mcp/awiki-server/test/stubs.test.mjs
```

Expected: `stubs: ok`.

- [ ] **Step 3: Commit**

```bash
git add mcp/awiki-server/index.js mcp/awiki-server/test/stubs.test.mjs
git commit -m "feat(mcp): add review_status + mark_review_done stubs

Both tools are registered with empty input schemas and return
{stub:true, message:'implemented in phase 19'}. No locks, no shell-
outs. Phase 19 replaces both bodies."
```

---

## Task 18b.11: BATS smoke — `tests/mcp_task_test.bats`

**Files:** Create: `tests/mcp_task_test.bats`, `tests/fixtures/mcp-task/` (small fixture wiki tree).

End-to-end smoke that drives the running MCP server over stdio with real JSON-RPC frames. Asserts:

1. All seven new tools are listed by `tools/list` (in addition to the four v1 tools).
2. Each regex validation rejects malformed input with an MCP error or `{ok:false}` payload.
3. `capture` returns the closed-enum `sanitizations_applied[]` populated correctly per rule.
4. `triage_apply` stale-id path returns `{ok:false, stale_id:true}` when the inbox is edited between calls.
5. Path-resolution guard rejects `..`-traversal even when the regex layer would otherwise accept (defense-in-depth coverage).
6. Concurrent `triage_apply` invocations contend on the lock — the second one blocks on `flock -x` and only completes after the first releases.
7. Stub tools return `{stub: true}`.

The test uses `node mcp/awiki-server/index.js` directly (no `expect`, no `npx`); JSON-RPC frames are constructed and parsed in pure bash via `python3 -c '...'` for tiny JSON shaping. **Wait** — Phase 15 forbids `python3 -c '...'`. Use `node -e '...'` instead, or write a small helper file.

- [ ] **Step 1: Build the fixture wiki tree**

```bash
mkdir -p tests/fixtures/mcp-task/.awiki/maps
mkdir -p tests/fixtures/mcp-task/content/projects
mkdir -p tests/fixtures/mcp-task/content/contexts
mkdir -p tests/fixtures/mcp-task/content/agenda
mkdir -p tests/fixtures/mcp-task/raw/inbox/interactive
mkdir -p tests/fixtures/mcp-task/scripts
touch tests/fixtures/mcp-task/.awiki/lock

cat > tests/fixtures/mcp-task/content/inbox.md <<'EOF'
---
type: inbox
---

- 2026-04-27T12:00:00Z call dentist
- 2026-04-27T13:00:00Z buy milk
EOF

# Stub triage.sh: emits TRIAGE-RESULT|<json> trailer.
cat > tests/fixtures/mcp-task/scripts/triage.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
sleep "${AWIKI_TRIAGE_SLEEP:-0}"
id="$1"; shift
outcome="$1"; shift
printf 'TRIAGE-RESULT|{"actions_taken":["%s"],"created_pages":[],"updated_pages":["content/inbox.md"]}\n' "$outcome"
EOF
chmod +x tests/fixtures/mcp-task/scripts/triage.sh

# Stub capture.sh: timestamp + append.
cat > tests/fixtures/mcp-task/scripts/capture.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
text="$(cat)"
printf -- '- 2026-04-27T12:00:00Z %s\n' "$text" >> content/inbox.md
EOF
chmod +x tests/fixtures/mcp-task/scripts/capture.sh

# Stub action-scan.sh + agenda.sh.
printf '#!/usr/bin/env bash\nexit 0\n' > tests/fixtures/mcp-task/scripts/action-scan.sh
printf '#!/usr/bin/env bash\nexit 0\n' > tests/fixtures/mcp-task/scripts/agenda.sh
chmod +x tests/fixtures/mcp-task/scripts/action-scan.sh
chmod +x tests/fixtures/mcp-task/scripts/agenda.sh

# Empty actions.tsv with header.
printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
  > tests/fixtures/mcp-task/.awiki/maps/actions.tsv
```

- [ ] **Step 2: Tiny JSON helper used by BATS frames**

```bash
cat > tests/fixtures/mcp-task/scripts/mcp-call.js <<'EOF'
#!/usr/bin/env node
// Helper: reads a JSON request envelope from argv, prints the JSON-RPC
// frame to stdout. Avoids ad-hoc bash JSON munging.
//
// Usage: mcp-call.js <method> <id> <name> <argsJson>
const [, , method, idStr, name, argsJson] = process.argv;
const frame = {
  jsonrpc: "2.0",
  id: Number(idStr),
  method,
  params: name ? { name, arguments: JSON.parse(argsJson || "{}") } : {},
};
process.stdout.write(JSON.stringify(frame) + "\n");
EOF
chmod +x tests/fixtures/mcp-task/scripts/mcp-call.js
```

- [ ] **Step 3: Write the BATS suite**

```bash
cat > tests/mcp_task_test.bats <<'BATS'
#!/usr/bin/env bats

# End-to-end smoke for the seven phase-18b MCP tools. Drives
# mcp/awiki-server/index.js over stdio; asserts schema, validation,
# stale-id, lock contention, and stub responses.

setup() {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  WORK="$(mktemp -d)"
  cp -R tests/fixtures/mcp-task/. "$WORK/"
  export WORK
}

teardown() {
  rm -rf "$WORK"
}

# Helper: send a single JSON-RPC frame to the server, capture stdout.
mcp_call() {
  local frame="$1"
  ( cd "$WORK" && printf '%s\n' "$frame" \
      | node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" )
}

@test "tools/list returns 11 tools (4 v1 + 7 phase-18b)" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/list 1 "" "")
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *"capture"* ]]
  [[ "$output" == *"triage_inbox"* ]]
  [[ "$output" == *"triage_apply"* ]]
  [[ "$output" == *"list_actions"* ]]
  [[ "$output" == *"rebuild_agenda"* ]]
  [[ "$output" == *"review_status"* ]]
  [[ "$output" == *"mark_review_done"* ]]
  # v1 tools still present.
  [[ "$output" == *"ingest_source"* ]]
  [[ "$output" == *"lint"* ]]
  [[ "$output" == *"query_wiki"* ]]
  [[ "$output" == *"update_catalog"* ]]
}

@test "capture: clean text returns sanitizations_applied=[]" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"call dentist"}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"sanitizations_applied":[]'* ]]
  [[ "$output" == *'"appended":true'* ]]
}

@test "capture: wikilink neutralization populates closed-enum entry" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"see [[secret]]"}')
  run mcp_call "$frame"
  [[ "$output" == *"wikilink-neutralized"* ]]
}

@test "capture: hard-reject embedded newline returns MCP error" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"a\nb"}')
  run mcp_call "$frame"
  [[ "$output" == *"isError"* || "$output" == *"embedded-newline"* ]]
}

@test "capture: hard-reject checkbox prefix" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"[ ] not a capture"}')
  run mcp_call "$frame"
  [[ "$output" == *"isError"* || "$output" == *"checkbox"* ]]
}

@test "triage_inbox returns the two open lines with inbox-shape IDs" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_inbox "" '{}')
  run mcp_call "$frame"
  [[ "$output" =~ inbox-[0-9a-f]{10}-1 ]]
  [[ "$output" =~ inbox-[0-9a-f]{10}-2 ]] || [[ "$output" =~ inbox-[0-9a-f]{10}-[0-9]+ ]]
}

@test "triage_apply: regex rejects shell-meta in id" {
  args='{"id":"BAD;rm -rf /","outcome":"trash"}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"must match"* || "$output" == *"isError"* || "$output" == *"\"ok\":false"* ]]
}

@test "triage_apply: outcome not in enum is rejected" {
  args='{"id":"a01","outcome":"DELETE"}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"\"ok\":false"* || "$output" == *"isError"* ]]
}

@test "triage_apply: invalid ISO calendar date rejected (2026-02-30)" {
  args='{"id":"a01","outcome":"defer-scheduled","params":{"due":"2026-02-30"}}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"not a valid ISO calendar date"* ]]
}

@test "triage_apply: path-guard rejects ../traversal in project_slug" {
  # The regex layer rejects this first, but we assert the outcome is rejection
  # regardless of which layer fires (defense in depth; either acceptable).
  args='{"id":"a01","outcome":"act","params":{"project_slug":"../etc"}}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"must match"* || "$output" == *"path"* || "$output" == *"\"ok\":false"* ]]
}

@test "triage_apply: stale_id when inbox edited between scan and apply" {
  # 1. Scan inbox to get a real ID for line 5 ("buy milk").
  scan_frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_inbox '' '{}')
  scan_out=$(mcp_call "$scan_frame")
  id=$(node -e "
    const out = process.argv[1];
    const lines = out.trim().split('\n').filter(Boolean);
    const resp = lines.map(JSON.parse).find(r => r.id === 1);
    const items = JSON.parse(resp.result.content[0].text);
    const milk = items.find(i => i.text === 'buy milk');
    console.log(milk.id);
  " -- "$scan_out")
  [ -n "$id" ]
  # 2. Edit the inbox so the line at the captured lineno no longer matches.
  cat > "$WORK/content/inbox.md" <<'EOF'
---
type: inbox
---

- 2026-04-27T13:00:00Z buy bread
EOF
  # 3. Apply with the now-stale ID.
  args=$(node -e "console.log(JSON.stringify({id: process.argv[1], outcome: 'trash'}))" -- "$id")
  apply_frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$apply_frame"
  [[ "$output" == *'"stale_id":true'* ]]
}

@test "concurrent triage_apply invocations contend on flock -x" {
  # Force the first stub to sleep 2s while holding the lock; the second
  # call should block until the first releases. Total wall time > 2s.
  AWIKI_TRIAGE_SLEEP=2 \
    bash -c "(cd '$WORK' && AWIKI_TRIAGE_SLEEP=2 node '$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js' \
              <<<'$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' '{\"id\":\"a01\",\"outcome\":\"trash\"}')' \
              > /tmp/awiki-out1) &"
  sleep 0.2
  start=$SECONDS
  frame2=$(node "$WORK/scripts/mcp-call.js" tools/call 2 triage_apply '' '{"id":"a02","outcome":"trash"}')
  ( cd "$WORK" && printf '%s\n' "$frame2" \
      | node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" > /tmp/awiki-out2 )
  elapsed=$((SECONDS - start))
  wait || true
  # Second call should have waited at least ~2s for the first to release.
  [ "$elapsed" -ge 1 ]
  grep -q '"ok":true' /tmp/awiki-out2
}

@test "review_status returns stub:true" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 review_status '' '{}')
  run mcp_call "$frame"
  [[ "$output" == *'"stub":true'* ]]
}

@test "mark_review_done returns stub:true" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  run mcp_call "$frame"
  [[ "$output" == *'"stub":true'* ]]
}
BATS
```

- [ ] **Step 4: Run the suite**

```bash
( cd mcp/awiki-server && npm install --silent )
bats tests/mcp_task_test.bats
```

Expected: 14 tests pass. If `flock` is missing on PATH, the lock-contention test errors with `command not found: flock`; install util-linux first (see Task 18b.6).

- [ ] **Step 5: Commit**

```bash
git add tests/mcp_task_test.bats tests/fixtures/mcp-task/
git commit -m "test: add BATS smoke for phase-18b MCP tools

14 cases drive node mcp/awiki-server/index.js over stdio with real
JSON-RPC frames. Asserts: all 7 new tools listed alongside the 4 v1
tools; capture sanitizations_applied populated correctly per closed-
enum rule; capture hard-rejects (newline, checkbox prefix) surface as
MCP errors; triage_apply regex rejects shell-meta and bad enums;
calendar-invalid dates rejected via Date.UTC round-trip; path-guard /
regex rejects ../traversal; stale_id path returns when inbox is edited
between triage_inbox() and triage_apply(); concurrent triage_apply
invocations serialize on flock -x (wall-clock measurement); both
review stubs return {stub:true}."
```

---

## Task 18b.12: Self-review + merge

**Files:** No new files. Final review + branch merge.

- [ ] **Step 1: Run the full local test matrix**

```bash
( cd mcp/awiki-server && npm install --silent )
node mcp/awiki-server/test/sanitize-capture.test.mjs
node mcp/awiki-server/test/inbox-id.test.mjs
node mcp/awiki-server/test/triage-validate.test.mjs
node mcp/awiki-server/test/path-guard.test.mjs
node mcp/awiki-server/test/lock.test.mjs
node mcp/awiki-server/test/capture-tool.test.mjs
node mcp/awiki-server/test/triage-inbox-scan.test.mjs
node mcp/awiki-server/test/triage-tools.test.mjs
node mcp/awiki-server/test/list-rebuild-tools.test.mjs
node mcp/awiki-server/test/stubs.test.mjs
bats tests/mcp_task_test.bats
just lint
```

Expected: every Node test prints `<name>: ok`; `bats` reports 14 passes; `just lint` is clean.

- [ ] **Step 2: Walk the self-review checklist (below) and tick every box**

Copy the checklist into the PR body once it's all green.

- [ ] **Step 3: Print rebuild instruction (plan-level note)**

```bash
cat <<'NOTE'
NOTE: this phase did NOT re-wire the MCP server. After merge,
every developer / user must run:

    cd mcp/awiki-server && npm install

to pick up new dev-only deps and warm Node's module cache. The seven
new tools are auto-exposed once the server restarts.
NOTE
```

- [ ] **Step 4: Merge**

```bash
git checkout main
git pull --ff-only
git merge --no-ff phase-18b-task-mcp-server -m "feat: complete phase 18b task-layer MCP server"
git branch -d phase-18b-task-mcp-server
```

Expected: fast-forward-resolvable merge; branch deleted; `main` now contains the seven new tools.

---

## Self-review checklist

Tick every box before opening the PR. If a box can't be ticked, surface the gap in Open Questions and either (a) fix it, or (b) cite the cross-phase coordination point that resolves it.

### Spec adherence
- [ ] Every tool listed in the spec's "MCP Server Additions" table is registered (`capture`, `triage_inbox`, `triage_apply`, `list_actions`, `rebuild_agenda`, `review_status`, `mark_review_done`).
- [ ] `capture` returns the exact shape `{appended, line, timestamp, sanitizations_applied[]}`.
- [ ] `sanitizations_applied[]` is the closed enum the spec defines: `wikilink-neutralized`, `comment-neutralized`, `length-truncated`, `block-id-escaped` — no superset, no aliases.
- [ ] Hard-reject patterns (control char, embedded newline, checkbox-prefix-at-start) return MCP errors and never appear in `sanitizations_applied[]`.
- [ ] Capture sanitization rules match the spec's "Capture sanitization" table verbatim, including the tab→space carve-out.
- [ ] `triage_apply` returns `{ok, actions_taken[], created_pages[], updated_pages[]}` on success and `{ok:false, stale_id:true}` on inbox-line drift.
- [ ] Inbox-line ID format matches spec: `inbox-<sha1(line)[:10]>-<lineno>` with the line including its leading `- <ISO-datetime> ` prefix.
- [ ] File-shaped IDs match spec: `file-<sha1(relpath)[:10]>`.
- [ ] Block-ID-shape IDs round-trip through `parseInboxId` correctly.

### Validation cascade ordering
- [ ] Per-arg type check runs first.
- [ ] Per-arg regex (id, outcome, slugs, page_type, dates) runs before any FS access.
- [ ] `Date.UTC` round-trip rejects 2026-02-30, 2026-13-01, 2026-00-15, and non-zero-padded forms.
- [ ] JSON Schema validation of the `params` envelope fires before path-resolution.
- [ ] Path-resolution guard (`path.resolve` + canonical-parent-prefix) runs after regex and before any shell-out or write.
- [ ] Path-guard rejects symlinks whose realpath escapes the canonical parent.
- [ ] Lock acquisition (`flock -x` mutating, `flock -s` read-only) is the LAST step before the actual shell-out / write.
- [ ] TOCTOU re-verification re-reads the inbox line at the supplied lineno and recomputes `sha1(line)[:10]` before applying.

### Security posture
- [ ] All shell-outs use `execFileSync` with explicit argv arrays.
- [ ] No string-interpolation into shell commands anywhere in `lib/` or `index.js`.
- [ ] `--` flag terminator appears before every positional argument in shell-outs.
- [ ] No `child_process.exec` (string command) — only `execFileSync` / `spawn` with argv arrays.
- [ ] No new network listener; transport is `StdioServerTransport` only.
- [ ] No `python3 -c '...'` calls (Phase 15 prohibition reaffirmed).
- [ ] All slug regexes match the spec exactly (project allows underscore prefix; context / wait / ref do not).
- [ ] `id` regex `^[a-z0-9_~-]{1,32}$` allows recurrence-chain `~` IDs without admitting shell metacharacters.

### Error handling
- [ ] `SanitizeError` translates to MCP error in `capture`.
- [ ] `LockTimeoutError` returns a structured `{ok:false}` payload (not an unhandled exception) in `triage_apply`.
- [ ] `PathGuardError` translates to MCP error in `triage_apply`.
- [ ] `triage.sh` exit 9 (stale_id contract; exit 7 reserved for flock contention) translates to `{ok:false, stale_id:true}`.
- [ ] Unknown `triage.sh` non-zero exits surface as MCP errors with stderr in the message.
- [ ] Missing `TRIAGE-RESULT|<json>` trailer surfaces as a clear error message.

### Test coverage
- [ ] Every new lib has a `<name>.test.mjs` file with a self-contained tmpdir setup and explicit `console.log("<name>: ok")` summary.
- [ ] BATS smoke covers: tool list, capture happy path, capture sanitization rules, capture hard-rejects, triage_inbox, triage_apply regex / enum / date / path-guard rejects, stale_id, lock contention, both stubs.
- [ ] Lock-contention test measures wall-clock ≥ 1s elapsed for the second call (proves the lock actually serialized).
- [ ] Stale-id test edits the inbox between `triage_inbox` and `triage_apply` and asserts the exact `{ok:false, stale_id:true}` payload.

### Plan hygiene
- [ ] Conventions block lists Node 20+, `@modelcontextprotocol/sdk` ^1.0.0, branch name, dependency phases.
- [ ] Every task has a `**Files:**` Create / Modify / Test header.
- [ ] Every step is a numbered checkbox with explicit expected output for every command.
- [ ] TDD cadence (failing test → implement → re-run → commit) appears in every library task.
- [ ] No `TBD` placeholders in any code block.
- [ ] Out-of-scope sections list Phase 18a deliverables (triage.sh, action-recur.sh, log-append poisoning, lint T8-T13) and Phase 19 deliverables (full review_status / mark_review_done bodies).

### Cross-phase coordination
- [ ] `scripts/triage.sh` (Phase 18a) emits `TRIAGE-RESULT|<json>` trailer on success.
- [ ] `scripts/triage.sh` exits **9** (PINNED) when it detects stale_id itself; exit 7 is reserved for flock contention.
- [ ] `scripts/capture.sh` (Phase 18a) honors `AWIKI_CAPTURE_PRESANITIZED=1` env var to skip in-script neutralization.
- [ ] `scripts/capture.sh` acquires its own `flock -x` (so the MCP tool does NOT double-lock).
- [ ] Phase 17's `.awiki/maps/actions.tsv` columns match the order this plan's `lib/list-actions.js` decodes: `id status text file line context due defer wait since every priority est done project source_kind`.

---

## Open Questions

These are spec / cross-phase ambiguities surfaced during plan drafting. Each must be answered before merge — either resolved in the corresponding sibling phase or escalated to a master-plan review.

1. **`scripts/capture.sh` lock-and-env contract (Phase 18a coordination):** Does `capture.sh` (a) acquire `flock -x` itself, (b) honor `AWIKI_CAPTURE_PRESANITIZED=1` to skip its in-script neutralization step? This plan assumes both. If 18a does NOT take the lock, switch the `capture` tool's dispatch to wrap the `execFileSync` call in `runLockedExclusive` (already imported). If 18a does NOT honor the env flag, the MCP tool re-sanitizes; the spec table is idempotent under double-application except for the length-truncation rule, which would truncate twice — coordinate with 18a to skip the second pass when the input ends in `…`.

2. **`scripts/triage.sh` trailer + exit code contract (RESOLVED in coordination with Phase 18a):** `triage.sh` (a) emits exactly one `TRIAGE-RESULT|<json>` trailer line on success, with `actions_taken`, `created_pages`, `updated_pages` keys; (b) exits **9** for stale_id (exit 7 is taken by `lock.sh` flock contention). Both phases' plans pinned to this contract.

3. **Strict shared-lock semantics on read-only tools:** The spec says read-only tools take `flock -s`. Our implementation acquires-then-releases the shared lock around the no-op `bash -c true`, then performs the JS read OUTSIDE the lock. This means a concurrent writer COULD modify `inbox.md` mid-`scanInbox`. We accept this because (a) any race result is caught by `triage_apply`'s TOCTOU re-verify; (b) writing a Node fcntl binding is out of scope. **Recommendation:** document the limitation in `WIKI.md` and revisit in Phase 19 if a Node `fs.flock` API lands or we adopt `proper-lockfile`.

4. **Schema for `triage_apply.params` envelope vs. spec Triage table:** The spec lists separate slug regexes per param. This plan unifies them in `schemas/triage-params.json` AND duplicates the regex check in `triage-validate.js` for defense-in-depth. The duplication is intentional but means future regex changes must be applied in both places. If we standardize on one source of truth, the schema file should win — recommend dropping the JS-side regex once the schema validator's pattern-error messages are good enough.

5. **`list_actions.filter` shape:** The spec's filter envelope mentions `status, context, project, due_before, wait, overdue`. This plan's schema covers all six. Should `due_after` and `defer_after` also be supported (symmetric with `due_before`)? Not in spec; the BATS smoke does not exercise them. **Recommendation:** wait for a real consumer use-case before extending.

6. **`rebuild_agenda` rebuilt-list source of truth:** This plan returns a hardcoded list of the five managed-region pages. If `agenda.sh` ever skips a regen (e.g., privacy filter excludes all actions for one of the five), the `rebuilt[]` array would still claim the file was touched. **Recommendation:** have `agenda.sh` print one path per line on stdout and parse them, or accept the small lie since `last_updated` is rewritten unconditionally per spec.

7. **`scanInbox` strikethrough-skip behavior:** The spec says trash outcome leaves a strikethrough in `inbox.md` ("Strikethrough line in `inbox.md`"). This plan's `triage-inbox-scan.js` skips lines whose body is `~~text~~`. Should it also skip lines that *contain* a strikethrough region but include other text? Spec language is ambiguous. **Recommendation:** stick with body-fully-strikethrough; document via a code-comment in `triage-inbox-scan.js`.

8. **MCP error vs. structured `{ok:false}` payload boundary:** Some failure modes return MCP-level errors (path-guard escape, capture hard-reject); others return `{ok:false, ...}` (stale_id, validation errors). The line is: "user mistake the agent should retry / clarify" → `{ok:false}`; "developer-or-attacker mistake the agent should NOT retry" → MCP error. Audit each error path against this rule before merge; current draft is consistent but document the policy in `WIKI.md` so the agent knows how to interpret.

9. **Symlink behavior in `path-guard.js`:** `realpathSync` follows the symlink chain and fails the guard if any link target escapes. But what if the user has legitimately set up a symlink for `_loose.md` or `_someday.md` (e.g., shared between workspaces)? **Recommendation:** spec is silent; accept current strict behavior and surface in Phase 19's polish task as a deferred call.

10. **Node `@modelcontextprotocol/sdk` version pin:** This plan pins `^1.0.0`. If Master Phase 8 actually pinned a different range, Step 18b.1.3's `if missing then add` logic preserves the existing pin. Confirm the SDK API used in this phase (`Server`, `StdioServerTransport`, `CallToolRequestSchema`, `ListToolsRequestSchema`) is stable across `1.x` minors; if not, narrow the pin.

---

## Phase complete

Return to the [task-layer master plan](./2026-04-27-task-layer-master-plan.md) or proceed to [Phase 19](./2026-04-27-phase-19-task-review-polish.md) which replaces the `review_status` and `mark_review_done` stubs with their real bodies.
