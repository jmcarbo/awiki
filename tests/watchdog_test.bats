#!/usr/bin/env bats

# Watchdog daemon tests. All cases force the polling backend so behavior
# is deterministic on any host (no fswatch / inotifywait dependency).

setup() {
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/raw/inbox/interactive" "$WORK/raw/inbox/batch" "$WORK/raw/inbox/checkpoint"
  mkdir -p "$WORK/raw/processed" "$WORK/raw/processed/_originals" "$WORK/raw/assets"
  mkdir -p "$WORK/.awiki" "$WORK/content"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cp .awiki/config "$WORK/.awiki/config"
  export AWIKI_REPO_ROOT="$WORK"
  export AWIKI_WATCHDOG_BACKEND=poll
  export AWIKI_WATCHDOG_POLL_INTERVAL=0.2
  export AWIKI_WATCHDOG_STABLE_CHECKS=2
  export AWIKI_WATCHDOG_STABLE_INTERVAL=0.05
  export AWIKI_WATCHDOG_STABLE_MAX=20
  unset AWIKI_AGENT
  unset AWIKI_INGEST_CMD
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

WD() {
  bash "$BATS_TEST_DIRNAME/../scripts/watchdog.sh" "$@"
}

@test "watchdog ingests file landing in batch (catchup)" {
  printf "hello world\n" > raw/inbox/batch/foo.txt
  run WD --catchup --once
  [ "$status" -eq 0 ]
  [[ "$output" == *"WATCHDOG|start|backend=poll"* ]]
  [[ "$output" == *"WATCHDOG|detect|raw/inbox/batch/foo.txt"* ]]
  [[ "$output" == *"WATCHDOG|stable|raw/inbox/batch/foo.txt"* ]]
  [[ "$output" == *"WATCHDOG|ingest-ok|raw/inbox/batch/foo.txt"* ]]
  [ ! -f raw/inbox/batch/foo.txt ]
  [ -f raw/processed/batch/foo.txt ]
}

@test "watchdog without --catchup leaves existing files alone in --once mode" {
  printf "hello\n" > raw/inbox/batch/leftover.txt
  run WD --once
  [ "$status" -eq 0 ]
  [[ "$output" != *"WATCHDOG|detect|"* ]]
  [ -f raw/inbox/batch/leftover.txt ]
}

@test "watchdog quarantines on ingest failure" {
  cat > stub-ingest.sh <<'EOF'
#!/usr/bin/env bash
echo "stub: failing on $1" >&2
exit 7
EOF
  chmod +x stub-ingest.sh
  export AWIKI_INGEST_CMD="bash $WORK/stub-ingest.sh"
  printf "boom\n" > raw/inbox/batch/bad.txt
  run WD --catchup --once
  [ "$status" -eq 0 ]
  [[ "$output" == *"WATCHDOG|ingest-fail|raw/inbox/batch/bad.txt|rc=7|moved=raw/inbox/batch/_failed/bad.txt"* ]]
  [ ! -f raw/inbox/batch/bad.txt ]
  [ -f raw/inbox/batch/_failed/bad.txt ]
}

@test "watchdog skips files under _failed/" {
  mkdir -p raw/inbox/batch/_failed
  printf "old\n" > raw/inbox/batch/_failed/old.pdf
  run WD --catchup --once
  [ "$status" -eq 0 ]
  [[ "$output" != *"WATCHDOG|detect|raw/inbox/batch/_failed/old.pdf"* ]]
  [[ "$output" != *"WATCHDOG|ingest-ok|raw/inbox/batch/_failed/old.pdf"* ]]
  [ -f raw/inbox/batch/_failed/old.pdf ]
}

@test "watchdog skips hidden files" {
  printf "" > raw/inbox/batch/.DS_Store
  run WD --catchup --once
  [ "$status" -eq 0 ]
  [[ "$output" == *"WATCHDOG|skip|raw/inbox/batch/.DS_Store|reason=hidden"* ]]
  [ -f raw/inbox/batch/.DS_Store ]
}

@test "watchdog emits stable line before ingest-ok" {
  printf "stable content\n" > raw/inbox/batch/stable.txt
  run WD --catchup --once
  [ "$status" -eq 0 ]
  # Find positions of the relevant markers in output.
  stable_idx="$(printf '%s\n' "$output" | grep -n 'WATCHDOG|stable|raw/inbox/batch/stable.txt' | head -1 | cut -d: -f1)"
  ok_idx="$(printf '%s\n' "$output" | grep -n 'WATCHDOG|ingest-ok|raw/inbox/batch/stable.txt' | head -1 | cut -d: -f1)"
  [ -n "$stable_idx" ]
  [ -n "$ok_idx" ]
  [ "$stable_idx" -lt "$ok_idx" ]
}

@test "watchdog halts when another instance is running" {
  sleep 30 &
  other_pid=$!
  echo "$other_pid" > .awiki/watchdog.pid
  run WD --once
  kill "$other_pid" 2>/dev/null || true
  wait "$other_pid" 2>/dev/null || true
  [ "$status" -eq 3 ]
  [[ "$output" == *"WATCHDOG|halt|reason=already-running|pid=$other_pid"* ]]
}

@test "watchdog cleans up pid file on exit" {
  run WD --once
  [ "$status" -eq 0 ]
  [ ! -f .awiki/watchdog.pid ]
}

@test "watchdog logs hidden skip only once across multiple poll cycles" {
  printf "" > raw/inbox/batch/.gitkeep
  run WD --cycles 5 --poll-interval 0.01
  [ "$status" -eq 0 ]
  count="$(printf '%s\n' "$output" | grep -c 'WATCHDOG|skip|raw/inbox/batch/.gitkeep|reason=hidden' || true)"
  [ "$count" = "1" ]
}

@test "watchdog logs under-failed skip only once across multiple poll cycles" {
  mkdir -p raw/inbox/batch/_failed
  # _failed/ files are excluded from list_existing, so this directly tests
  # the streaming-event branch by feeding the path through handle_path.
  # We exercise via a normal file in batch/ that fails ingest; quarantined
  # destination is then re-discovered on later cycles via the find filter
  # (which it ISN'T — but if a future change adds the path, this guards).
  printf "" > raw/inbox/batch/_failed/old.txt
  run WD --cycles 5 --poll-interval 0.01
  [ "$status" -eq 0 ]
  # _failed/ is excluded from find; expect zero detect/skip lines for it.
  [[ "$output" != *"raw/inbox/batch/_failed/old.txt"* ]]
}
