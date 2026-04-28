#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
  mkdir -p .awiki content/sources content/entities raw/_git-cache
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > content/log.md
  export AWIKI_REPO_ROOT="$WORK"
  # Build a tiny local fixture repo
  seed="$WORK/seed"
  mkdir -p "$seed"
  printf "# Hello\n\nbody\n" > "$seed/README.md"
  bash "$BATS_TEST_DIRNAME/util/build-git-fixture.sh" "$seed" "$WORK/repo"
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "ingest-git: rejects missing arg" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh"
  [ "$status" -eq 1 ]
}

@test "ingest-git: --dry-run on local fixture exits 0 with plan output" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "PLAN|"
  echo "$output" | grep -q "repo_key=local-repo"
}
