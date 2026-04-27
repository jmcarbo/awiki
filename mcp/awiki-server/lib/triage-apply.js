// triage_apply orchestration. Runs (in order):
//   1. validateTriageArgs (regex + schema + cross-field + dates).
//   2. resolveTriageDestinations (path-resolution guard).
//   3. For inbox-shaped IDs: re-read the line at supplied lineno and verify
//      the hash via verifyInboxId. Return {ok:false, stale_id:true} on
//      mismatch (no exception, recoverable failure).
//   4. Shell out to scripts/triage.sh under flock -x.
//   5. Parse the TRIAGE-RESULT|<json> trailer line.

import { readFileSync } from "node:fs";
import { join } from "node:path";
import { validateTriageArgs } from "./triage-validate.js";
import { resolveTriageDestinations, PathGuardError } from "./path-guard.js";
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

  // 2. Path guard. Translate PathGuardError to a structured {ok:false} so
  //    the MCP boundary can surface it without throwing — keeps the contract
  //    that recoverable validation failures stay in-payload.
  try {
    resolveTriageDestinations(repoRoot, { outcome, params });
  } catch (e) {
    if (e instanceof PathGuardError) {
      return {
        ok: false,
        errors: [{ field: "(path-guard)", reason: e.message }],
      };
    }
    throw e;
  }

  // 3. TOCTOU re-verify for inbox-shaped IDs.
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
  const argv = ["scripts/triage.sh", id, outcome];
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
      // (Exit 7 is reserved for flock contention; do not collide.)
      return { ok: false, stale_id: true };
    }
    if (e.status === 7) {
      // flock contention surfaced via bash exit code (defense in depth).
      throw new Error(`triage.sh: lock contention (exit 7)`);
    }
    const stderr = e.stderr ? e.stderr.toString() : "";
    return {
      ok: false,
      errors: [{ field: "(triage.sh)", reason: `exit ${e.status ?? "?"}: ${stderr.trim() || "(no stderr)"}` }],
    };
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
