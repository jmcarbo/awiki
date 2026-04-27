#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/.awiki"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "lock library defines awiki_lock_with and awiki_lock_shared" {
  run bash -c 'source scripts/lib/lock.sh && type -t awiki_lock_with && type -t awiki_lock_shared'
  [ "$status" -eq 0 ]
  [[ "$output" == *"function"* ]]
}

@test "awiki_lock_with creates .awiki/lock and runs command" {
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=5 -- echo HELLO'
  [ "$status" -eq 0 ]
  [[ "$output" == *"HELLO"* ]]
  [ -f .awiki/lock ]
}

@test "awiki_lock_with exits 7 on contention" {
  bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=10 -- sleep 3' &
  HOLDER_PID=$!
  sleep 0.3
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=1 -- echo SHOULD_NOT_RUN'
  [ "$status" -eq 7 ]
  [[ "$output" != *"SHOULD_NOT_RUN"* ]]
  wait "$HOLDER_PID" 2>/dev/null || true
}

@test "awiki_lock_shared allows concurrent shared readers" {
  bash -c 'source scripts/lib/lock.sh && awiki_lock_shared --timeout=5 -- sleep 2' &
  READER_PID=$!
  sleep 0.2
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_shared --timeout=2 -- echo SECOND_READER'
  [ "$status" -eq 0 ]
  [[ "$output" == *"SECOND_READER"* ]]
  wait "$READER_PID" 2>/dev/null || true
}

@test "awiki_lock_with rejects missing -- separator" {
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=5 echo nope'
  [ "$status" -ne 0 ]
  [[ "$output" == *"--"* || "$output" == *"separator"* ]]
}

@test "awiki_lock_with rejects non-numeric timeout" {
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=abc -- echo x'
  [ "$status" -ne 0 ]
  [[ "$output" == *"timeout"* ]]
}

@test "exported defaults present" {
  run bash -c 'source scripts/lib/lock.sh && echo "$AWIKI_LOCK_TIMEOUT_USER|$AWIKI_LOCK_TIMEOUT_DEFERRED"'
  [ "$status" -eq 0 ]
  [[ "$output" == "30|180" ]]
}
