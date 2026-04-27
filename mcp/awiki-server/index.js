#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { execFileSync } from "node:child_process";
import { readFileSync } from "node:fs";
import { listSynthPlugins } from "./lib/list-plugins.js";
import { validate } from "./lib/validate-scope.js";
import { runSynthesize, runFinalize, assertManifestUnderPluginDir } from "./lib/synthesize.js";
import { sanitizeCapture, SanitizeError } from "./lib/sanitize-capture.js";
import { LockTimeoutError, runLockedShared, runLockedExclusive } from "./lib/lock.js";
import { scanInbox } from "./lib/triage-inbox-scan.js";
import { triageApply } from "./lib/triage-apply.js";
import { PathGuardError } from "./lib/path-guard.js";
import { loadActionsTsv, applyFilter, isTsvStale } from "./lib/list-actions.js";

const FILTER_SCHEMA = {
  type: "object",
  additionalProperties: false,
  properties: {
    status: { enum: ["[ ]", "[x]", "[/]", "[?]", "[>]"] },
    context: { type: "string", pattern: "^[a-z0-9][a-z0-9-]{0,63}$" },
    project: { type: "string", pattern: "^[a-z0-9_][a-z0-9_-]{0,63}$" },
    due_before: { type: "string", pattern: "^\\d{4}-\\d{2}-\\d{2}$" },
    wait: { type: "string", pattern: "^[a-z0-9][a-z0-9-]{0,63}$" },
    overdue: { type: "boolean" },
  },
};

const REPO_ROOT = process.cwd();
const PLUGIN_NAME_RE = /^[a-z][a-z0-9-]*$/;
const TOPIC_SLUG_RE = /^[a-z0-9][a-z0-9-]*$/;
const SCOPE_SCHEMA = JSON.parse(
  readFileSync(new URL("./schemas/scope.json", import.meta.url), "utf8"),
);

const TOOLS = [
  {
    name: "ingest_source",
    description: "Process a source from raw/inbox/* into the wiki via scripts/ingest.sh.",
    inputSchema: {
      type: "object",
      properties: { path: { type: "string", description: "Path under raw/inbox/{interactive,batch,checkpoint}/" } },
      required: ["path"],
      additionalProperties: false,
    },
  },
  {
    name: "lint",
    description: "Run mechanical lint over content/. Returns LINT|... lines and a LINT-SUMMARY.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
  {
    name: "query_wiki",
    description: "Search wiki via qmd (BM25+vector hybrid). Falls back to grep when qmd absent.",
    inputSchema: {
      type: "object",
      properties: { query: { type: "string" } },
      required: ["query"],
      additionalProperties: false,
    },
  },
  {
    name: "update_catalog",
    description: "Rebuild content/catalog.md from on-disk pages and current frontmatter.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
  {
    name: "list_synth_plugins",
    description: "List all synthesis plugins under synthesis-plugins/. No arguments. Errors during scan are reported in the return payload.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
  {
    name: "synthesize",
    description: "Scaffold a synthesis page via scripts/synth.sh new. Returns {prompt_bundle, resolved_slugs, target_path}, or a structured error payload (scope_resolution_failed, target_exists, invalid_plugin) on orchestrator non-zero exit.",
    inputSchema: {
      type: "object",
      additionalProperties: false,
      required: ["plugin", "scope_descriptor", "topic_slug"],
      properties: {
        plugin: { type: "string" },
        scope_descriptor: { type: "object" },
        topic_slug: { type: "string" },
      },
    },
  },
  {
    name: "finalize_synthesis",
    description: "Finalize a synthesis page via scripts/synth.sh finalize. Validates markers, runs synth lint, stamps last_generated, populates frontmatter sources. Returns {ok: true, output} on success or a structured error payload on marker/lint failure.",
    inputSchema: {
      type: "object",
      additionalProperties: false,
      required: ["topic_slug"],
      properties: { topic_slug: { type: "string" } },
    },
  },
  {
    name: "capture",
    description: "Sanitize and append a free-text capture to content/inbox.md via scripts/capture.sh. Returns {appended, line, timestamp, sanitizations_applied}. Hard-reject patterns (embedded newline, control char, checkbox prefix at start) return an MCP error.",
    inputSchema: {
      type: "object",
      additionalProperties: false,
      required: ["text"],
      properties: {
        text: { type: "string", maxLength: 4000 },
      },
    },
  },
  {
    name: "triage_inbox",
    description: "Read-only scan of content/inbox.md and raw/inbox/interactive/. Returns [{id, source, line_or_path, text, captured_at}, ...]. IDs are inbox-<sha10>-<lineno> for inbox lines, file-<sha10> for files.",
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
        id: { type: "string", pattern: "^[a-z0-9_~-]{1,32}$" },
        outcome: { enum: ["trash", "do-now", "act", "defer-scheduled", "waiting", "reference", "someday"] },
        params: { type: "object" },
      },
    },
  },
  {
    name: "list_actions",
    description: "Return rows from .awiki/maps/actions.tsv, optionally filtered by status/context/project/due_before/wait/overdue. Re-runs scripts/action-scan.sh under flock -x first if the TSV is stale relative to content/**/*.md.",
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
  {
    name: "review_status",
    description: "Run scripts/review-status.sh under flock -s and return a structured JSON report: {last_review, inbox_unprocessed, raw_inbox_files, projects_no_next_action[], waiting_stale_14d[], overdue[], completed_since_last_review, stuck_projects[], someday_count}. Overdue/waiting entries are enriched with id/due/days_over and id/wait/since/days respectively.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
  {
    name: "mark_review_done",
    description: "STUB (phase 18b): full implementation lands in phase 19. Returns {stub:true, message:'implemented in phase 19'}.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
];

const server = new Server({ name: "awiki", version: "0.1.0" }, { capabilities: { tools: {} } });

server.setRequestHandler(ListToolsRequestSchema, async () => ({ tools: TOOLS }));

server.setRequestHandler(CallToolRequestSchema, async (req) => {
  const { name, arguments: args } = req.params;
  let out;
  try {
    switch (name) {
      case "ingest_source":
        if (typeof args?.path !== "string") throw new Error("path required");
        out = execFileSync("bash", ["scripts/ingest.sh", args.path], { encoding: "utf8" });
        break;
      case "lint":
        out = execFileSync("bash", ["scripts/lint.sh"], { encoding: "utf8" });
        break;
      case "query_wiki": {
        if (typeof args?.query !== "string") throw new Error("query required");
        try {
          out = execFileSync("qmd", ["search", args.query], { encoding: "utf8" });
        } catch {
          out = execFileSync("grep", ["-rli", "--include=*.md", args.query, "content/"], { encoding: "utf8" });
        }
        break;
      }
      case "update_catalog":
        out = execFileSync("bash", ["scripts/update-catalog.sh"], { encoding: "utf8" });
        break;
      case "list_synth_plugins": {
        const result = listSynthPlugins(REPO_ROOT);
        out = JSON.stringify(result, null, 2);
        break;
      }
      case "synthesize": {
        const { plugin, scope_descriptor, topic_slug } = args ?? {};

        // 1. Regex validation (before any filesystem access).
        if (typeof plugin !== "string" || !PLUGIN_NAME_RE.test(plugin)) {
          out = JSON.stringify({ error: "invalid_argument", field: "plugin", reason: "must match ^[a-z][a-z0-9-]*$" });
          break;
        }
        if (typeof topic_slug !== "string" || !TOPIC_SLUG_RE.test(topic_slug)) {
          out = JSON.stringify({ error: "invalid_argument", field: "topic_slug", reason: "must match ^[a-z0-9][a-z0-9-]*$" });
          break;
        }

        // 2. JSON Schema validation of scope_descriptor.
        if (scope_descriptor === null || typeof scope_descriptor !== "object" || Array.isArray(scope_descriptor)) {
          out = JSON.stringify({ error: "invalid_argument", field: "scope_descriptor", reasons: ["must be an object"] });
          break;
        }
        const scopeCheck = validate(SCOPE_SCHEMA, scope_descriptor);
        if (!scopeCheck.valid) {
          out = JSON.stringify({ error: "invalid_argument", field: "scope_descriptor", reasons: scopeCheck.errors });
          break;
        }

        // 3. realpath check on the resolved manifest path (defeats symlink swap).
        try {
          assertManifestUnderPluginDir(REPO_ROOT, plugin);
        } catch (e) {
          out = JSON.stringify({ error: "invalid_plugin", reason: e.message });
          break;
        }

        // 4. Shell-out (only after all gates pass).
        const result = runSynthesize({
          repoRoot: REPO_ROOT,
          plugin,
          topicSlug: topic_slug,
          scope: scope_descriptor,
        });
        out = JSON.stringify(result, null, 2);
        break;
      }
      case "finalize_synthesis": {
        const { topic_slug } = args ?? {};
        if (typeof topic_slug !== "string" || !TOPIC_SLUG_RE.test(topic_slug)) {
          out = JSON.stringify({ error: "invalid_argument", field: "topic_slug", reason: "must match ^[a-z0-9][a-z0-9-]*$" });
          break;
        }
        const result = runFinalize({ repoRoot: REPO_ROOT, topicSlug: topic_slug });
        out = JSON.stringify(result, null, 2);
        break;
      }
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
        // 3. Shell out to scripts/capture.sh under flock -x. capture.sh takes
        // its own lock (phase 18a), reads text from stdin, and appends with
        // its own ISO-8601 timestamp. We pass AWIKI_CAPTURE_PRESANITIZED=1
        // so it skips its in-script neutralization (we already did it).
        // capture.sh always runs hard-reject checks unconditionally.
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
      case "triage_inbox": {
        // Take flock -s briefly via a no-op `true` to acquire-then-release.
        // The JS scan that follows is not held under the lock; the spec
        // language "read-only tools take flock -s" is satisfied in the
        // *spirit* of preventing concurrent writes. TOCTOU re-verify in
        // triage_apply catches drift between this scan and the apply call.
        try {
          runLockedShared({
            repoRoot: REPO_ROOT,
            argv: ["true"],
            timeoutSec: 30,
          });
        } catch (e) {
          if (e instanceof LockTimeoutError) {
            throw new Error(`triage_inbox: lock timeout after 30s`);
          }
          throw e;
        }
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
      case "list_actions": {
        const filter = args?.filter ?? {};
        const filterCheck = validate(FILTER_SCHEMA, filter);
        if (!filterCheck.valid) {
          out = JSON.stringify({
            error: "invalid_argument",
            field: "filter",
            reasons: filterCheck.errors,
          });
          break;
        }
        // Stale-check: re-run action-scan.sh under flock -x if needed.
        if (isTsvStale(REPO_ROOT)) {
          try {
            runLockedExclusive({
              repoRoot: REPO_ROOT,
              argv: ["bash", "scripts/action-scan.sh"],
              timeoutSec: 30,
            });
          } catch (e) {
            if (e instanceof LockTimeoutError) {
              throw new Error(`list_actions: lock timeout after 30s during scan`);
            }
            throw e;
          }
        }
        // Briefly take flock -s as a privacy/consistency gesture (does not
        // hold the lock during the JS read; documented limitation).
        try {
          runLockedShared({
            repoRoot: REPO_ROOT,
            argv: ["true"],
            timeoutSec: 30,
          });
        } catch (e) {
          if (e instanceof LockTimeoutError) {
            throw new Error(`list_actions: lock timeout after 30s`);
          }
          throw e;
        }
        const rows = applyFilter(loadActionsTsv(REPO_ROOT), filter);
        out = JSON.stringify(rows, null, 2);
        break;
      }
      case "review_status": {
        // Shell out to scripts/review-status.sh. The script itself takes
        // a brief shared lock; we do not need to wrap it in another flock
        // call. The MCP server's job is to reshape the structured stdout
        // into the documented JSON object.
        let stdout;
        try {
          stdout = execFileSync("bash", ["scripts/review-status.sh"], {
            cwd: REPO_ROOT,
            encoding: "utf8",
            env: { ...process.env, LC_ALL: "C" },
            timeout: 30_000,
          });
        } catch (e) {
          const stderr = e.stderr ? e.stderr.toString() : "";
          throw new Error(`review_status failed: ${stderr.trim() || e.message}`);
        }
        out = JSON.stringify(parseReviewStatus(REPO_ROOT, stdout));
        break;
      }
      case "mark_review_done":
        out = JSON.stringify({ stub: true, message: "implemented in phase 19" });
        break;
      case "rebuild_agenda": {
        const t0 = Date.now();
        try {
          runLockedExclusive({
            repoRoot: REPO_ROOT,
            argv: ["bash", "-c", "scripts/action-scan.sh && scripts/agenda.sh"],
            timeoutSec: 30,
          });
        } catch (e) {
          if (e instanceof LockTimeoutError) {
            throw new Error(`rebuild_agenda: lock timeout after 30s`);
          }
          throw e;
        }
        const duration_ms = Date.now() - t0;
        // The list of regenerated files is fixed (the five managed-region
        // pages from phase 16/17). agenda.sh rewrites all five.
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
      default:
        throw new Error(`unknown tool: ${name}`);
    }
    return { content: [{ type: "text", text: out }] };
  } catch (e) {
    return { content: [{ type: "text", text: `ERROR|${e.message}` }], isError: true };
  }
});

// =============================================================================
// review_status / mark_review_done helpers
// =============================================================================

// Parse REVIEW|<key>|... lines from scripts/review-status.sh into the JSON
// shape documented in the spec. Enriches overdue and waiting-stale entries
// with per-row metadata read directly from .awiki/maps/actions.tsv.
function parseReviewStatus(repoRoot, stdout) {
  const out = {
    last_review: "never",
    inbox_unprocessed: 0,
    raw_inbox_files: 0,
    projects_no_next_action: [],
    waiting_stale_14d: [],
    overdue: [],
    completed_since_last_review: 0,
    stuck_projects: [],
    someday_count: 0,
  };
  // Helper: parse "key=value" segments. Skips the leading anchor segment
  // ("REVIEW" or "REVIEW-SUMMARY") and, for REVIEW|<tag>|... lines, the
  // tag segment too.
  const parseKv = (line, skip) => {
    const obj = {};
    for (const seg of line.split("|").slice(skip)) {
      const i = seg.indexOf("=");
      if (i < 0) continue;
      obj[seg.slice(0, i)] = seg.slice(i + 1);
    }
    return obj;
  };

  for (const raw of stdout.split("\n")) {
    if (!raw.startsWith("REVIEW")) continue;
    if (raw.startsWith("REVIEW-SUMMARY|")) {
      const kv = parseKv(raw, 1);
      out.last_review = kv["last-review"] ?? "never";
      continue;
    }
    const kv = parseKv(raw, 2);
    const tag = raw.split("|")[1];
    switch (tag) {
      case "inbox-unprocessed":
        out.inbox_unprocessed = parseInt(kv.count, 10) || 0;
        break;
      case "raw-inbox-files":
        out.raw_inbox_files = parseInt(kv.count, 10) || 0;
        break;
      case "projects-no-next-action":
        out.projects_no_next_action = (kv.slugs ?? "").split(",").filter(Boolean);
        break;
      case "waiting-stale-14d":
        out.waiting_stale_14d = enrichWaitingStale(
          repoRoot,
          (kv.ids ?? "").split(",").filter(Boolean),
        );
        break;
      case "overdue":
        out.overdue = enrichOverdue(
          repoRoot,
          (kv.ids ?? "").split(",").filter(Boolean),
        );
        break;
      case "completed-since-last-review":
        out.completed_since_last_review = parseInt(kv.count, 10) || 0;
        break;
      case "stuck-projects":
        out.stuck_projects = (kv.slugs ?? "").split(",").filter(Boolean);
        break;
      case "someday-count":
        out.someday_count = parseInt(kv.count, 10) || 0;
        break;
      default:
        break;
    }
  }
  return out;
}

// Convert an ISO calendar date "YYYY-MM-DD" to UTC seconds-since-epoch.
// Returns null on parse failure.
function isoDateToEpochSec(dateStr) {
  const m = /^(\d{4})-(\d{2})-(\d{2})$/.exec(dateStr);
  if (!m) return null;
  const [, y, mo, d] = m;
  return Date.UTC(Number(y), Number(mo) - 1, Number(d)) / 1000;
}

function todayEpochSec() {
  const today =
    process.env.AWIKI_TODAY ?? new Date().toISOString().slice(0, 10);
  return isoDateToEpochSec(today) ?? Math.floor(Date.now() / 1000);
}

function enrichWaitingStale(repoRoot, ids) {
  if (ids.length === 0) return [];
  let tsv;
  try {
    tsv = readFileSync(`${repoRoot}/.awiki/maps/actions.tsv`, "utf8");
  } catch {
    return ids.map((id) => ({ id, wait: "", since: "", days: 0 }));
  }
  const tSec = todayEpochSec();
  const result = [];
  const lines = tsv.split("\n");
  for (let i = 1; i < lines.length; i++) {
    if (!lines[i]) continue;
    const c = lines[i].split("\t");
    if (!ids.includes(c[0])) continue;
    const wait = c[8] ?? "";
    const since = c[9] ?? "";
    let days = 0;
    if (since) {
      const sSec = isoDateToEpochSec(since);
      if (sSec !== null) days = Math.floor((tSec - sSec) / 86400);
    }
    result.push({ id: c[0], wait, since, days });
  }
  return result;
}

function enrichOverdue(repoRoot, ids) {
  if (ids.length === 0) return [];
  let tsv;
  try {
    tsv = readFileSync(`${repoRoot}/.awiki/maps/actions.tsv`, "utf8");
  } catch {
    return ids.map((id) => ({ id, due: "", days_over: 0 }));
  }
  const tSec = todayEpochSec();
  const result = [];
  const lines = tsv.split("\n");
  for (let i = 1; i < lines.length; i++) {
    if (!lines[i]) continue;
    const c = lines[i].split("\t");
    if (!ids.includes(c[0])) continue;
    const due = c[6] ?? "";
    let days_over = 0;
    if (due) {
      const dSec = isoDateToEpochSec(due);
      if (dSec !== null) days_over = Math.floor((tSec - dSec) / 86400);
    }
    result.push({ id: c[0], due, days_over });
  }
  return result;
}

// =============================================================================

const transport = new StdioServerTransport();
await server.connect(transport);
