#!/usr/bin/env bats

bats_require_minimum_version 1.5.0

setup() {
  command -v python3 >/dev/null 2>&1 || skip "python3 not installed"
  python3 -c "import python_calamine" 2>/dev/null || skip "python-calamine not installed"
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/raw/inbox/interactive" "$WORK/raw/inbox/batch" "$WORK/raw/inbox/checkpoint"
  mkdir -p "$WORK/raw/processed" "$WORK/raw/processed/_originals" "$WORK/raw/assets"
  mkdir -p "$WORK/.awiki" "$WORK/content"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cp .awiki/config "$WORK/.awiki/config"
  cp "$BATS_TEST_DIRNAME/fixtures/xlsx/single-sheet.xlsx" "$WORK/raw/inbox/batch/single-sheet.xlsx"
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

@test "watchdog extracts xlsx then ingests resulting md (two --once cycles)" {
  # First cycle: detect xlsx -> ingest-xlsx.sh -> writes md sibling, removes xlsx.
  run WD --catchup --once
  [ "$status" -eq 0 ]
  [[ "$output" == *"XLSX-CONVERTED|"* ]]
  [ -f raw/inbox/batch/single-sheet--sales.md ]
  [ ! -f raw/inbox/batch/single-sheet.xlsx ]

  # Second cycle: the new .md is picked up and ingested normally.
  run WD --catchup --once
  [ "$status" -eq 0 ]
  [[ "$output" == *"WATCHDOG|ingest-ok|"* ]]
  [ -f raw/processed/batch/single-sheet--sales.md ]
}

@test "watchdog quarantines corrupt xlsx to _failed/" {
  cp "$BATS_TEST_DIRNAME/fixtures/xlsx/corrupt.xlsx" raw/inbox/batch/corrupt.xlsx
  run WD --catchup --once
  [ -f raw/inbox/batch/_failed/corrupt.xlsx ]
}
