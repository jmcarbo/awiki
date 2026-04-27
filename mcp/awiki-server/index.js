#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { execFileSync } from "node:child_process";

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
