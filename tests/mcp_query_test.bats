#!/usr/bin/env bats
# Tests for the MCP list_queries tool.
# Drives mcp/awiki-server/index.js over stdio with JSON-RPC frames,
# using tests/fixtures/mcp-query/ as the working tree.
#
# Fixture layout:
#   content/queries/top-trades.md    — type:query, sources:[trades]
#   content/queries/region-summary.md — type:query, sources:[trades,regions]
#   content/notes/analysis.md        — inline ```sql awiki-query id="cf-by-cat"

setup() {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  WORK="$(mktemp -d)"
  cp -R "$BATS_TEST_DIRNAME/fixtures/mcp-query/." "$WORK/"
  SERVER="$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js"
  export WORK SERVER
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

# Send a single JSON-RPC frame to the server, capture stdout.
mcp_call() {
  local frame="$1"
  ( cd "$WORK" && printf '%s\n' "$frame" | node "$SERVER" )
}

# Extract the inner JSON payload from the MCP response envelope.
extract_payload() {
  node -e "
    const lines = require('fs').readFileSync(0,'utf8').trim().split('\n').filter(Boolean);
    const resp = lines.map(JSON.parse).find(r => r.id === 1);
    if (!resp) { console.error('no response with id=1'); process.exit(1); }
    if (resp.result && resp.result.content && resp.result.content[0] && resp.result.content[0].text) {
      process.stdout.write(resp.result.content[0].text);
    } else {
      process.stdout.write(JSON.stringify(resp));
    }
  "
}

@test "list_queries: registered in tools/list" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/list 1 "" "")
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *'"name":"list_queries"'* ]]
}

@test "list_queries: returns both type:query pages" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 list_queries '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    if (!Array.isArray(d.queries)) { console.error('queries not array'); process.exit(1); }
    const pages = d.queries.filter(q => q.kind === 'page');
    const slugs = pages.map(q => q.slug).sort();
    if (!slugs.includes('top-trades'))      { console.error('top-trades missing; got: '+slugs.join(',')); process.exit(1); }
    if (!slugs.includes('region-summary'))  { console.error('region-summary missing; got: '+slugs.join(',')); process.exit(1); }
  " "$payload"
}

@test "list_queries: type:query page carries slug, out, sources, kind=page" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 list_queries '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    const tt = d.queries.find(q => q.kind === 'page' && q.slug === 'top-trades');
    if (!tt) { console.error('top-trades page entry missing'); process.exit(1); }
    if (tt.out !== 'top-trades-out') { console.error('out='+tt.out); process.exit(1); }
    if (!Array.isArray(tt.sources) || !tt.sources.includes('trades')) {
      console.error('sources='+JSON.stringify(tt.sources)); process.exit(1);
    }
    if (!tt.page) { console.error('page field missing'); process.exit(1); }
  " "$payload"
}

@test "list_queries: inline awiki-query fence returned with kind=fence" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 list_queries '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    const fence = d.queries.find(q => q.kind === 'fence' && q.id === 'cf-by-cat');
    if (!fence) { console.error('cf-by-cat fence entry missing'); process.exit(1); }
    if (typeof fence.fence_index !== 'number') { console.error('fence_index missing'); process.exit(1); }
    if (!fence.page) { console.error('page field missing'); process.exit(1); }
  " "$payload"
}

@test "list_queries: list_charts still works (no regression)" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 list_charts '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    if (!Array.isArray(d.charts)) { console.error('charts not array'); process.exit(1); }
  " "$payload"
}
