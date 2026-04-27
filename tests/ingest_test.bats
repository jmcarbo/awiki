#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  cp -r raw "$WORK/raw"
  cp tests/fixtures/sample-source.md "$WORK/raw/inbox/interactive/sample.md"
  mkdir -p "$WORK/.awiki" "$WORK/content"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cp .awiki/config "$WORK/.awiki/config"
  export AWIKI_REPO_ROOT="$WORK"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "ingest moves source from inbox to processed" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  [ "$status" -eq 0 ]
  [ ! -f raw/inbox/interactive/sample.md ]
  [ -f raw/processed/interactive/sample.md ]
}

@test "ingest appends log entry" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  run grep "ingest | sample.md | mode=interactive" content/log.md
  [ "$status" -eq 0 ]
}

@test "ingest increments .awiki/ingest-count" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  count="$(cat .awiki/ingest-count)"
  [ "$count" = "1" ]
}

@test "ingest rejects path outside inbox" {
  cp tests/fixtures/sample-source.md "$WORK/elsewhere.md" 2>/dev/null || cp "$BATS_TEST_DIRNAME/../tests/fixtures/sample-source.md" elsewhere.md
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" elsewhere.md
  [ "$status" -eq 2 ]
}

@test "ingest rejects missing arg" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh"
  [ "$status" -eq 1 ]
}
