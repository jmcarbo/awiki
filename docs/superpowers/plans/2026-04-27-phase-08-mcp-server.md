# awiki Plan — Phase 8: MCP Wiki-Ops Server

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 2, 4, 6
**Previous:** [Phase 07](./2026-04-27-phase-07-rename-delete.md)
**Next:** [Phase 09](./2026-04-27-phase-09-scheduled-lint.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** `mcp/awiki-server/` Node MCP server exposing `ingest_source`, `query_wiki`, `lint`, `update_catalog`. BOOTSTRAP wiring option.

**Branch:** `phase-8-mcp-server`

## Task 8.1: Branch + npm package

- [ ] **Step 1: Branch + scaffold**

```bash
git checkout -b phase-8-mcp-server
mkdir -p mcp/awiki-server
cd mcp/awiki-server
npm init -y
npm i @modelcontextprotocol/sdk
```

(Note: hand-written JSON schemas; no zod dependency. Keeps surface area small.)

- [ ] **Step 2: Write `mcp/awiki-server/index.js`**

```javascript
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
```

- [ ] **Step 3: Add npm bin + chmod**

Edit `mcp/awiki-server/package.json` to add `"bin": { "awiki-mcp": "./index.js" }` and `"type": "module"`.

```bash
chmod +x mcp/awiki-server/index.js
```

- [ ] **Step 4: Commit**

```bash
cd ../..
git add mcp/awiki-server
git commit -m "feat: add awiki MCP server (ingest/lint/query/update)"
```

## Task 8.2: BOOTSTRAP wiring helper for awiki MCP

- [ ] **Step 1: Add helper `scripts/wire-awiki-mcp.sh`**

(Mirrors `wire-qmd-mcp.sh` but registers `awiki` server pointing to `mcp/awiki-server/index.js`.)

```bash
cat > scripts/wire-awiki-mcp.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f mcp/awiki-server/index.js ]]; then
  echo "MCP server not built. Run: cd mcp/awiki-server && npm install" >&2
  exit 1
fi

TARGET=""
if [[ -d .claude || -f CLAUDE.md ]]; then TARGET="claude"; fi
if [[ -d .codex ]]; then TARGET="codex"; fi

case "$TARGET" in
  claude)
    CONFIG="./.mcp.json"
    [[ -f "$CONFIG" ]] || echo '{"mcpServers": {}}' > "$CONFIG"
    python3 - "$CONFIG" <<'PY'
import json, sys, os
path = sys.argv[1]
data = json.load(open(path))
data.setdefault("mcpServers", {})["awiki"] = {
    "command": "node",
    "args": [os.path.abspath("mcp/awiki-server/index.js")]
}
json.dump(data, open(path, "w"), indent=2)
PY
    echo "WIRED|claude|$CONFIG"
    ;;
  codex)
    CONFIG=".codex/config.toml"
    mkdir -p .codex
    if ! grep -q '^\[mcp.servers.awiki\]' "$CONFIG" 2>/dev/null; then
      ABS_PATH="$(cd mcp/awiki-server && pwd)/index.js"
      cat >> "$CONFIG" <<TOML

[mcp.servers.awiki]
command = "node"
args = ["$ABS_PATH"]
TOML
    fi
    echo "WIRED|codex|$CONFIG"
    ;;
  *)
    echo "No agent harness detected (.claude or .codex). Run from a configured project." >&2
    exit 1
    ;;
esac
EOF
chmod +x scripts/wire-awiki-mcp.sh
```

- [ ] **Step 2: Commit**

```bash
git add scripts/wire-awiki-mcp.sh
git commit -m "feat: add awiki MCP wiring helper"
```

## Task 8.3: Smoke test for awiki MCP server

**Files:** Create: `tests/mcp_server_test.bats`, `tests/fixtures/mcp-list-tools.json`

- [ ] **Step 1: Write the test**

```bash
cat > tests/fixtures/mcp-list-tools.json <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
EOF

cat > tests/mcp_server_test.bats <<'EOF'
#!/usr/bin/env bats

@test "awiki MCP server lists 4 tools" {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  run bash -c 'cat tests/fixtures/mcp-list-tools.json | node mcp/awiki-server/index.js | head -1'
  [ "$status" -eq 0 ]
  [[ "$output" == *"ingest_source"* ]]
  [[ "$output" == *"lint"* ]]
  [[ "$output" == *"query_wiki"* ]]
  [[ "$output" == *"update_catalog"* ]]
}

@test "awiki MCP server rejects unknown tool" {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  REQ='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bogus","arguments":{}}}'
  run bash -c "echo '$REQ' | node mcp/awiki-server/index.js"
  [[ "$output" == *"unknown tool"* || "$output" == *"isError"* ]]
}
EOF
```

- [ ] **Step 2: Install deps + run test**

```bash
( cd mcp/awiki-server && npm install --silent )
bats tests/mcp_server_test.bats
```

Expected: 2 tests pass.

- [ ] **Step 3: Commit**

```bash
git add tests/mcp_server_test.bats tests/fixtures/mcp-list-tools.json
git commit -m "test: add MCP server smoke tests (tools list + unknown-tool rejection)"
```

## Task 8.4: Phase 8 merge

```bash
git checkout main
git merge --no-ff phase-8-mcp-server -m "feat: complete phase 8 MCP server"
git branch -d phase-8-mcp-server
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 09](./2026-04-27-phase-09-scheduled-lint.md).
