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

@test "data-init creates content/datasets/ and data/ directories" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/datasets ]
  [ -d data ]
}

@test "data-init is idempotent on directory creation" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/datasets ]
  [ -d data ]
}
