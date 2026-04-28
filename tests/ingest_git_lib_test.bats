#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
  mkdir -p .awiki/git-state
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "git-state: path() returns expected file location" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_path "github-com-foo-bar"
  [ "$status" -eq 0 ]
  [ "$output" = ".awiki/git-state/github-com-foo-bar.json" ]
}

@test "git-state: load() returns {} when missing" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_load "no-such-repo"
  [ "$status" -eq 0 ]
  [ "$output" = "{}" ]
}

@test "git-state: save() then load() roundtrips" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  awiki_git_state_save "rk" '{"schema":1,"repo_key":"rk","files":{}}'
  run awiki_git_state_load "rk"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"schema":1'
  echo "$output" | grep -q '"repo_key":"rk"'
}

@test "git-state: validate() rejects schema!=1" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_validate '{"schema":2,"repo_key":"rk","repo_name":"r","url":"u","default_branch":"main","head_sha":"s","ingested_at":"t","private":false,"files":{}}'
  [ "$status" -ne 0 ]
}

@test "git-state: validate() accepts well-formed" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_validate '{"schema":1,"repo_key":"rk","repo_name":"r","url":"u","default_branch":"main","head_sha":"s","ingested_at":"t","private":false,"files":{}}'
  [ "$status" -eq 0 ]
}

@test "git-state: validate() rejects missing ingested_at (spec §7.2 required)" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_validate '{"schema":1,"repo_key":"rk","repo_name":"r","url":"u","default_branch":"main","head_sha":"s","private":false,"files":{}}'
  [ "$status" -ne 0 ]
}

@test "git-state: path() rejects repo_key with .. (path traversal)" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_path "../escape"
  [ "$status" -ne 0 ]
}

@test "git-state: path() rejects repo_key with / " {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_path "evil/path"
  [ "$status" -ne 0 ]
}
