#!/usr/bin/env bats
# Phase-19 MCP review_status / mark_review_done tests.
# Drives mcp/awiki-server/index.js over stdio with JSON-RPC frames,
# using tests/fixtures/mcp-review/ as the working tree.

setup() {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  if [[ -d /opt/homebrew/opt/util-linux/bin ]]; then
    export PATH="/opt/homebrew/opt/util-linux/bin:$PATH"
  fi
  if ! command -v flock >/dev/null 2>&1; then
    skip "flock(1) not on PATH; install util-linux"
  fi
  WORK="$(mktemp -d)"
  cp -R "$BATS_TEST_DIRNAME/fixtures/mcp-review/." "$WORK/"
  SERVER="$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js"
  export WORK SERVER AWIKI_TODAY="2026-04-27"
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

# Send a JSON-RPC frame. Exports AWIKI_TODAY into the subshell so
# review-status.sh (shelled out by the MCP server) computes deltas
# against the fixed test date.
mcp_call() {
  local frame="$1"
  ( cd "$WORK" && export AWIKI_TODAY && printf '%s\n' "$frame" | node "$SERVER" )
}

# Extract the inner JSON payload from the MCP response envelope. The
# server wraps tool output in { content: [ { type: "text", text: "<json>" } ] };
# we want to assert against that inner JSON directly.
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

# ===== review_status ========================================================

@test "review_status returns documented JSON shape (no stub flag)" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 review_status '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  echo "PAYLOAD: $payload" >&2
  node -e "
    const d = JSON.parse(process.argv[1]);
    if ('stub' in d) { console.error('stub flag still present'); process.exit(1); }
    const required = ['last_review','inbox_unprocessed','raw_inbox_files',
                      'projects_no_next_action','waiting_stale_14d','overdue',
                      'completed_since_last_review','stuck_projects','someday_count'];
    for (const k of required) {
      if (!(k in d)) { console.error('missing key '+k); process.exit(1); }
    }
    if (!Array.isArray(d.projects_no_next_action)) { console.error('projects_no_next_action not array'); process.exit(1); }
    if (!Array.isArray(d.waiting_stale_14d))      { console.error('waiting_stale_14d not array'); process.exit(1); }
    if (!Array.isArray(d.overdue))                { console.error('overdue not array'); process.exit(1); }
    if (!Array.isArray(d.stuck_projects))         { console.error('stuck_projects not array'); process.exit(1); }
  " "$payload"
}

@test "review_status counts match scripts/review-status.sh on the same fixture" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 review_status '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    if (d.inbox_unprocessed !== 3) { console.error('inbox_unprocessed='+d.inbox_unprocessed); process.exit(1); }
    if (d.raw_inbox_files !== 1)   { console.error('raw_inbox_files='+d.raw_inbox_files); process.exit(1); }
    if (d.someday_count !== 12)    { console.error('someday_count='+d.someday_count); process.exit(1); }
    if (d.completed_since_last_review !== 3) { console.error('completed='+d.completed_since_last_review); process.exit(1); }
    if (!d.projects_no_next_action.includes('renovate-kitchen')) { console.error('renovate-kitchen missing'); process.exit(1); }
    if (!d.stuck_projects.includes('onboarding-revamp')) { console.error('onboarding-revamp missing from stuck'); process.exit(1); }
    if (d.last_review !== '2026-04-20') { console.error('last_review='+d.last_review); process.exit(1); }
  " "$payload"
}

@test "review_status overdue entries carry id/due/days_over" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 review_status '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    if (d.overdue.length === 0) { console.error('expected at least one overdue entry'); process.exit(1); }
    for (const e of d.overdue) {
      for (const k of ['id','due','days_over']) {
        if (!(k in e)) { console.error('overdue entry missing '+k+': '+JSON.stringify(e)); process.exit(1); }
      }
    }
    // Check known IDs.
    const ids = d.overdue.map(e => e.id).sort();
    if (ids.join(',') !== 'a01,a07') { console.error('overdue ids='+ids.join(',')); process.exit(1); }
  " "$payload"
}

@test "review_status waiting_stale_14d entries carry id/wait/since/days" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 review_status '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  node -e "
    const d = JSON.parse(process.argv[1]);
    if (d.waiting_stale_14d.length !== 1) { console.error('waiting_stale len='+d.waiting_stale_14d.length); process.exit(1); }
    const e = d.waiting_stale_14d[0];
    for (const k of ['id','wait','since','days']) {
      if (!(k in e)) { console.error('waiting_stale entry missing '+k); process.exit(1); }
    }
    if (e.id !== 'a03')           { console.error('id='+e.id); process.exit(1); }
    if (e.wait !== 'bob-smith')   { console.error('wait='+e.wait); process.exit(1); }
    if (e.since !== '2026-04-08') { console.error('since='+e.since); process.exit(1); }
    if (e.days !== 19)            { console.error('days='+e.days); process.exit(1); }
  " "$payload"
}
