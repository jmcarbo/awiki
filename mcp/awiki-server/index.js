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
import { LockTimeoutError } from "./lib/lock.js";

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
      default:
        throw new Error(`unknown tool: ${name}`);
    }
    return { content: [{ type: "text", text: out }] };
  } catch (e) {
    return { content: [{ type: "text", text: `ERROR|${e.message}` }], isError: true };
  }
});

const transport = new StdioServerTransport();
await server.connect(transport);
