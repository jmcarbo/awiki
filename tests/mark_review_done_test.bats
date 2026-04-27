#!/usr/bin/env bats
# Phase-19 MCP mark_review_done tests.
# Drives mcp/awiki-server/index.js over stdio. Asserts:
#   * .awiki/last-review is updated to the current ISO timestamp.
#   * content/agenda/review-log.md gets one new "## [<dt>] review | ..." line.
#   * Repeated invocations append exactly one line each (idempotent shape).

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
  # mark_review_done appends to content/agenda/review-log.md, which the
  # mcp-review fixture does not include by default (real wikis create it
  # via task-init.sh).
  mkdir -p "$WORK/content/agenda"
  cat > "$WORK/content/agenda/review-log.md" <<'LOG'
---
title: "Review Log"
type: agenda
draft: false
---

# Review Log

LOG
  SERVER="$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js"
  export WORK SERVER AWIKI_TODAY="2026-04-27"
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

mcp_call() {
  local frame="$1"
  ( cd "$WORK" && export AWIKI_TODAY && printf '%s\n' "$frame" | node "$SERVER" )
}

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

@test "mark_review_done writes ISO timestamp to .awiki/last-review" {
  before="$(cat "$WORK/.awiki/last-review")"
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  run mcp_call "$frame"
  [ "$status" -eq 0 ]
  payload="$(printf '%s\n' "$output" | extract_payload)"
  [[ "$payload" != *'\"stub\":true'* ]]
  echo "$payload" | node -e "
    const d = JSON.parse(require('fs').readFileSync(0,'utf8'));
    if (!d.last_review || !d.last_review.match(/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}Z$/)) {
      console.error('bad last_review='+d.last_review); process.exit(1);
    }
  "
  after="$(cat "$WORK/.awiki/last-review")"
  [ "$before" != "$after" ]
  # Bare ISO-8601 timestamp on disk.
  echo "$after" | grep -qE '^[0-9]{4}-[0-9]{2}-[0-9]{2}T[0-9]{2}:[0-9]{2}:[0-9]{2}Z$'
}

@test "mark_review_done appends one log line with task counts" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  mcp_call "$frame" >/dev/null
  tail -1 "$WORK/content/agenda/review-log.md" \
    | grep -qE '^## \[[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}\] review \| inbox=[0-9]+ next=[0-9]+ waiting=[0-9]+ overdue=[0-9]+ completed=[0-9]+ stuck=[0-9]+$'
}

@test "mark_review_done log line counts match the fixture state" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  mcp_call "$frame" >/dev/null
  line="$(tail -1 "$WORK/content/agenda/review-log.md")"
  # Fixture has: 3 inbox, 5 open ([ ] or [/]) actions, 1 waiting,
  # 2 overdue, 3 completed-since-last-review, 1 stuck-projects.
  [[ "$line" == *'inbox=3'* ]]
  [[ "$line" == *'next=5'* ]]
  [[ "$line" == *'waiting=1'* ]]
  [[ "$line" == *'overdue=2'* ]]
  [[ "$line" == *'completed=3'* ]]
  [[ "$line" == *'stuck=1'* ]]
}

@test "mark_review_done is repeatable: each call appends exactly one line" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  mcp_call "$frame" >/dev/null
  lines_before="$(wc -l < "$WORK/content/agenda/review-log.md")"
  mcp_call "$frame" >/dev/null
  lines_after="$(wc -l < "$WORK/content/agenda/review-log.md")"
  [ "$((lines_after - lines_before))" -eq 1 ]
}

@test "mark_review_done updates last-review on every invocation" {
  frame=$(node "$WORK/scripts/mcp-call.js" tools/call 1 mark_review_done '' '{}')
  mcp_call "$frame" >/dev/null
  first="$(cat "$WORK/.awiki/last-review")"
  sleep 1
  mcp_call "$frame" >/dev/null
  second="$(cat "$WORK/.awiki/last-review")"
  [ "$first" != "$second" ]
}
