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
