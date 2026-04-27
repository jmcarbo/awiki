#!/usr/bin/env bats

# End-to-end smoke for the seven phase-18b MCP tools (capture, triage_inbox,
# triage_apply, list_actions, rebuild_agenda, review_status,
# mark_review_done). Drives mcp/awiki-server/index.js over stdio with real
# JSON-RPC frames. Asserts:
#   * tools/list registers the seven new tools alongside the four v1 tools.
#   * capture: clean text returns sanitizations_applied=[]; wikilinks
#     populate the closed-enum entry; embedded newline / checkbox prefix
#     hard-rejects surface as MCP errors.
#   * triage_inbox returns inbox-shaped IDs.
#   * triage_apply: regex / enum / date / path-guard reject malformed args.
#   * triage_apply: stale_id path returns {ok:false, stale_id:true} when the
#     inbox has shifted between triage_inbox() and the apply call.
#   * list_actions accepts the filter envelope; rebuild_agenda returns the
#     five managed-region paths.
#   * Concurrent triage_apply invocations contend on flock -x (wall-clock
#     measurement proves the lock actually serialized).
#   * review_status / mark_review_done return {stub:true}.
#
# macOS Homebrew util-linux ships flock at /opt/homebrew/opt/util-linux/bin;
# prepend that to PATH so node's child_process can find it.

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
  cp -R tests/fixtures/mcp-task/. "$WORK/"
  SERVER="$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js"
  export WORK SERVER
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

# Helper: send a single JSON-RPC frame to the server, capture stdout.
mcp_call() {
  local frame="$1"
  ( cd "$WORK" && printf '%s\n' "$frame" | node "$SERVER" )
}

@test "tools/list registers all 7 phase-18b tools alongside the 4 v1 tools" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/list 1 "" "")
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  # Phase-18b additions.
  [[ "$output" == *"\"name\":\"capture\""* ]]
  [[ "$output" == *"\"name\":\"triage_inbox\""* ]]
  [[ "$output" == *"\"name\":\"triage_apply\""* ]]
  [[ "$output" == *"\"name\":\"list_actions\""* ]]
  [[ "$output" == *"\"name\":\"rebuild_agenda\""* ]]
  [[ "$output" == *"\"name\":\"review_status\""* ]]
  [[ "$output" == *"\"name\":\"mark_review_done\""* ]]
  # v1 tools still present (no regression).
  [[ "$output" == *"\"name\":\"ingest_source\""* ]]
  [[ "$output" == *"\"name\":\"lint\""* ]]
  [[ "$output" == *"\"name\":\"query_wiki\""* ]]
  [[ "$output" == *"\"name\":\"update_catalog\""* ]]
}

@test "capture: clean text returns sanitizations_applied=[]" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"call dentist"}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *'\"sanitizations_applied\":[]'* ]]
  [[ "$output" == *'\"appended\":true'* ]]
}

@test "capture: wikilink neutralization populates closed-enum entry" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"see [[secret]]"}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *"wikilink-neutralized"* ]]
}

@test "capture: hard-reject embedded newline returns MCP error" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"a\nb"}')
  run mcp_call "$frame"
  [[ "$output" == *"isError"* || "$output" == *"embedded-newline"* ]]
}

@test "capture: hard-reject checkbox prefix returns MCP error" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 capture '{"text":"[ ] not a capture"}')
  run mcp_call "$frame"
  [[ "$output" == *"isError"* || "$output" == *"checkbox"* ]]
}

@test "triage_inbox returns the two open lines with inbox-shape IDs" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_inbox '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" =~ inbox-[0-9a-f]{10}-[0-9]+ ]]
  [[ "$output" == *"call dentist"* ]]
  [[ "$output" == *"buy milk"* ]]
}

@test "triage_apply: regex rejects shell-meta in id" {
  args='{"id":"BAD;rm -rf /","outcome":"trash"}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"must match"* || "$output" == *"isError"* || "$output" == *'\"ok\":false'* ]]
}

@test "triage_apply: outcome not in enum is rejected" {
  args='{"id":"a01","outcome":"DELETE"}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *'\"ok\":false'* || "$output" == *"isError"* ]]
}

@test "triage_apply: invalid ISO calendar date rejected (2026-02-30)" {
  args='{"id":"a01","outcome":"defer-scheduled","params":{"due":"2026-02-30"}}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"not a valid ISO calendar date"* ]]
}

@test "triage_apply: path-guard / regex rejects ../traversal in project_slug" {
  # Defense in depth: regex layer rejects first; either layer firing is
  # acceptable. We only assert "rejected".
  args='{"id":"a01","outcome":"act","params":{"project_slug":"../etc"}}'
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$frame"
  [[ "$output" == *"must match"* || "$output" == *"path"* || "$output" == *'\"ok\":false'* ]]
}

@test "triage_apply: stale_id when inbox edited between scan and apply" {
  # 1. Scan inbox to get a real ID for the "buy milk" line.
  scan_frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_inbox '' '{}')
  scan_out=$(mcp_call "$scan_frame")
  id=$(node -e "
    const out = process.argv[1];
    const lines = out.trim().split('\n').filter(Boolean);
    const resp = lines.map(JSON.parse).find(r => r.id === 1);
    const items = JSON.parse(resp.result.content[0].text);
    const milk = items.find(i => i.text === 'buy milk');
    console.log(milk.id);
  " -- "$scan_out")
  [ -n "$id" ]
  # 2. Edit the inbox so the line at the captured lineno no longer matches.
  cat > "$WORK/content/inbox.md" <<'EOF'
---
type: inbox
---

- 2026-04-27T13:00:00Z buy bread
EOF
  # 3. Apply with the now-stale ID.
  args=$(node -e "console.log(JSON.stringify({id: process.argv[1], outcome: 'trash'}))" -- "$id")
  apply_frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' "$args")
  run mcp_call "$apply_frame"
  [[ "$output" == *'\"stale_id\":true'* ]]
}

@test "list_actions: filter envelope returns array (empty TSV → [])" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 list_actions '' '{"filter":{"status":"[ ]"}}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *"[]"* ]]
}

@test "rebuild_agenda regenerates the 5 managed agenda pages" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 rebuild_agenda '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  [[ "$output" == *"content/agenda/next-actions.md"* ]]
  [[ "$output" == *"content/agenda/today.md"* ]]
  [[ "$output" == *"content/agenda/waiting.md"* ]]
  [[ "$output" == *"content/agenda/someday.md"* ]]
  [[ "$output" == *"content/agenda/stuck-projects.md"* ]]
  [[ "$output" == *"duration_ms"* ]]
}

@test "concurrent triage_apply invocations serialize on flock -x" {
  # Force the first call's stub triage.sh to sleep 2s while the MCP server
  # holds flock -x. The second call must wait for the first to release.
  # Total wall-clock for the second call must be >= 1s (proving the lock
  # actually serialized); allow slack for slow CI.
  frame1=$(node "$WORK/scripts/mcp-call.js" tools/call 1 triage_apply '' '{"id":"a01","outcome":"trash"}')
  frame2=$(node "$WORK/scripts/mcp-call.js" tools/call 2 triage_apply '' '{"id":"a02","outcome":"trash"}')
  printf '%s\n' "$frame1" > "$WORK/.f1.json"
  printf '%s\n' "$frame2" > "$WORK/.f2.json"
  ( cd "$WORK" && AWIKI_TRIAGE_SLEEP=2 node "$SERVER" < .f1.json > .o1.json 2>&1 ) &
  bg=$!
  # Give the first call ~300ms to acquire the lock before the second starts.
  sleep 0.3
  start=$SECONDS
  ( cd "$WORK" && node "$SERVER" < .f2.json > .o2.json 2>&1 )
  elapsed=$((SECONDS - start))
  wait "$bg" || true
  # Second call should have waited at least ~1s for the first to release.
  [ "$elapsed" -ge 1 ]
  # The MCP envelope wraps the tool payload as a JSON string, so the inner
  # "ok":true appears as \"ok\":true in stdout. Match the escaped form.
  grep -q '\\"ok\\":true' "$WORK/.o2.json"
  grep -q '\\"ok\\":true' "$WORK/.o1.json"
}

@test "review_status returns stub:true" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 review_status '' '{}')
  run mcp_call "$frame"
  [[ "$output" == *'\"stub\":true'* ]]
}

@test "mark_review_done returns stub:true" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  run mcp_call "$frame"
  [[ "$output" == *'\"stub\":true'* ]]
}
