#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\nAWIKI_STALE_DAYS=90\nAWIKI_LOG_QUERIES=0\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "task-init creates .awiki/last-review = today" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f .awiki/last-review ]
  today="$(date '+%Y-%m-%d')"
  run cat .awiki/last-review
  [[ "$output" == "$today" ]]
}

@test "task-init creates .awiki/task-count = 0" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f .awiki/task-count ]
  run cat .awiki/task-count
  [[ "$output" == "0" ]]
}

@test "task-init appends AWIKI_AGENDA_AFTER_N=5 and AWIKI_TASK_LAYER=on if absent" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_AGENDA_AFTER_N=5$' .awiki/config
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_TASK_LAYER=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "task-init preserves existing user-set config values" {
  echo 'AWIKI_AGENDA_AFTER_N=10' >> .awiki/config
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  # Must not duplicate, must not overwrite.
  run grep -c '^AWIKI_AGENDA_AFTER_N=' .awiki/config
  [[ "$output" == "1" ]]
  run grep '^AWIKI_AGENDA_AFTER_N=' .awiki/config
  [[ "$output" == "AWIKI_AGENDA_AFTER_N=10" ]]
}

@test "task-init re-run is idempotent: state files unchanged" {
  bash scripts/task-init.sh
  cp .awiki/last-review /tmp/awiki-lr-1
  cp .awiki/task-count /tmp/awiki-tc-1
  bash scripts/task-init.sh
  diff /tmp/awiki-lr-1 .awiki/last-review
  diff /tmp/awiki-tc-1 .awiki/task-count
  rm -f /tmp/awiki-lr-1 /tmp/awiki-tc-1
}
