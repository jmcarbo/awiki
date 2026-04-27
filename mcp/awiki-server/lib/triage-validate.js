// triage_apply argument validation.
// Boundary cascade per spec: regex first, JSON Schema second, date round-trip
// last. Path-resolution is a separate concern (see lib/path-guard.js) and
// runs after this validator clears.

import { readFileSync } from "node:fs";
import { validate as validateSchema } from "./validate-scope.js";

const SCHEMA = JSON.parse(
  readFileSync(new URL("../schemas/triage-params.json", import.meta.url), "utf8"),
);

// Spec umbrella shape for ids: covers inbox-, file-, plain block-IDs, and
// recurrence-chain ids that use ~. Underscore is permitted because some
// chain ids may include them.
export const ID_RE = /^[a-z0-9_~-]{1,32}$/;
export const OUTCOMES = Object.freeze([
  "trash",
  "do-now",
  "act",
  "defer-scheduled",
  "waiting",
  "reference",
  "someday",
]);
export const PAGE_TYPES = Object.freeze(["entity", "concept", "topic", "source"]);
const PROJECT_SLUG_RE = /^[a-z0-9_][a-z0-9_-]{0,63}$/;
const PLAIN_SLUG_RE = /^[a-z0-9][a-z0-9-]{0,63}$/;
const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;

// ISO calendar validity via Date.UTC round-trip.
export function validIsoDate(s) {
  if (typeof s !== "string") return false;
  const m = DATE_RE.exec(s);
  if (!m) return false;
  const y = Number(m[1]);
  const mo = Number(m[2]);
  const d = Number(m[3]);
  if (mo < 1 || mo > 12) return false;
  if (d < 1 || d > 31) return false;
  const utc = Date.UTC(y, mo - 1, d);
  const back = new Date(utc);
  return (
    back.getUTCFullYear() === y &&
    back.getUTCMonth() + 1 === mo &&
    back.getUTCDate() === d
  );
}

export function validateTriageArgs(args) {
  const errors = [];

  // 0. Args envelope must be a plain object.
  if (typeof args !== "object" || args === null || Array.isArray(args)) {
    return {
      ok: false,
      errors: [{ field: "(root)", reason: "args must be a plain object" }],
    };
  }
  const { id, outcome } = args;
  let { params } = args;

  // 1. Per-arg type + regex (id, outcome).
  if (typeof id !== "string" || !ID_RE.test(id)) {
    errors.push({ field: "id", reason: "must match ^[a-z0-9_~-]{1,32}$" });
  }
  if (typeof outcome !== "string" || !OUTCOMES.includes(outcome)) {
    errors.push({
      field: "outcome",
      reason: `must be one of ${OUTCOMES.join("|")}`,
    });
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
      errors.push({
        field: "params.ref_slug",
        reason: "required for outcome=reference",
      });
    }
    if (!("page_type" in params)) {
      errors.push({
        field: "params.page_type",
        reason: "required for outcome=reference",
      });
    }
  } else if ("page_type" in params) {
    errors.push({
      field: "params.page_type",
      reason: "only allowed for outcome=reference",
    });
  }

  // 4. Slug-shape regexes (defense in depth — schema patterns already cover
  // these, but keep explicit checks in case the schema is mis-edited).
  if ("project_slug" in params && !PROJECT_SLUG_RE.test(params.project_slug)) {
    errors.push({
      field: "params.project_slug",
      reason: "must match ^[a-z0-9_][a-z0-9_-]{0,63}$",
    });
  }
  for (const k of ["context_slug", "wait_for", "ref_slug"]) {
    if (k in params && !PLAIN_SLUG_RE.test(params[k])) {
      errors.push({
        field: `params.${k}`,
        reason: "must match ^[a-z0-9][a-z0-9-]{0,63}$",
      });
    }
  }
  if ("page_type" in params && !PAGE_TYPES.includes(params.page_type)) {
    errors.push({
      field: "params.page_type",
      reason: `must be one of ${PAGE_TYPES.join("|")}`,
    });
  }

  // 5. Date validity (ISO calendar round-trip).
  for (const k of ["due", "defer"]) {
    if (k in params && !validIsoDate(params[k])) {
      errors.push({
        field: `params.${k}`,
        reason: "not a valid ISO calendar date",
      });
    }
  }

  if (errors.length) return { ok: false, errors };
  return { ok: true, normalized: { id, outcome, params } };
}
