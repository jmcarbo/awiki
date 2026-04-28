#!/usr/bin/env node
// Build a single JSON-RPC frame and print it to stdout.
// Avoids ad-hoc bash JSON munging from BATS tests.
//
// Usage:
//   mcp-call.js <method> <id> <name> <argsJson>
//   mcp-call.js <method> <id> <name> <filler> <argsJson>   (5-arg variant)
//
// Notes:
//   - method:   tools/list | tools/call
//   - id:       integer request id
//   - name:     tool name (empty string for tools/list)
//   - argsJson: JSON-encoded arguments object (empty string => {})
//
// The 5-arg variant tolerates an empty placeholder slot between <name> and
// <argsJson>. This matches the BATS invocation pattern used in some plan
// drafts; either form produces the same JSON-RPC frame.
const argv = process.argv.slice(2);
const [method, idStr, nameArg] = argv;
const name = nameArg ?? "";
let argsJson;
if (argv.length >= 5) {
  argsJson = argv[4];
} else if (argv.length >= 4) {
  argsJson = argv[3];
} else {
  argsJson = "";
}
const frame = {
  jsonrpc: "2.0",
  id: Number(idStr),
  method,
  params: name ? { name, arguments: JSON.parse(argsJson || "{}") } : {},
};
process.stdout.write(JSON.stringify(frame) + "\n");
