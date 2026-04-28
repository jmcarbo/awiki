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

@test "git-config: name validator accepts kebab" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_name "ok-name"
  [ "$status" -eq 0 ]
}

@test "git-config: name validator rejects uppercase" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_name "BadName"
  [ "$status" -ne 0 ]
}

@test "git-config: name validator rejects leading dash" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_name "-foo"
  [ "$status" -ne 0 ]
}

@test "git-config: get() loads named repo from yaml" {
  if ! python3 -c "import yaml" >/dev/null 2>&1; then skip "pyyaml not installed"; fi
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_get "$BATS_TEST_DIRNAME/fixtures/git-sources-good.yml" "example-repo"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"url":"https://github.com/example/example.git"'
  echo "$output" | grep -q '"private":false'
}

@test "git-config: get() detects SSH→private auto-flip on private-runbooks" {
  if ! python3 -c "import yaml" >/dev/null 2>&1; then skip "pyyaml not installed"; fi
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_get "$BATS_TEST_DIRNAME/fixtures/git-sources-good.yml" "private-runbooks"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"private":true'
}

@test "git-config: get() rejects bad name from bad fixture" {
  if ! python3 -c "import yaml" >/dev/null 2>&1; then skip "pyyaml not installed"; fi
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_get "$BATS_TEST_DIRNAME/fixtures/git-sources-bad.yml" "BadName"
  [ "$status" -ne 0 ]
}

@test "git-config: validate_paths rejects .." {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_paths '["docs/","../escape"]'
  [ "$status" -ne 0 ]
}

@test "git-config: validate_paths rejects leading /" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_paths '["docs/","/etc"]'
  [ "$status" -ne 0 ]
}

@test "git-config: validate_paths accepts clean entries" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_paths '["README.md","docs/","rfcs/"]'
  [ "$status" -eq 0 ]
}

@test "git-clone: repo_key from https github URL" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  run awiki_git_clone_repo_key "https://github.com/foo/bar.git"
  [ "$status" -eq 0 ]
  [ "$output" = "github-com-foo-bar" ]
}

@test "git-clone: repo_key from ssh URL" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  run awiki_git_clone_repo_key "git@github.com:foo/bar.git"
  [ "$status" -eq 0 ]
  [ "$output" = "github-com-foo-bar" ]
}

@test "git-clone: repo_key from local path" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  d="$(mktemp -d)/myrepo"
  mkdir -p "$d"
  run awiki_git_clone_repo_key "$d"
  [ "$status" -eq 0 ]
  [ "$output" = "local-myrepo" ]
}

@test "git-clone: is_ssh detects git@" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  run awiki_git_clone_is_ssh "git@github.com:foo/bar.git"
  [ "$status" -eq 0 ]
  run awiki_git_clone_is_ssh "https://github.com/foo/bar.git"
  [ "$status" -ne 0 ]
}

@test "git-clone: resolve returns path|sha|branch for local fixture" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  seed="$(mktemp -d)/seed"
  mkdir -p "$seed"
  echo "hi" > "$seed/README.md"
  fixture="$(mktemp -d)/repo"
  bash "$BATS_TEST_DIRNAME/util/build-git-fixture.sh" "$seed" "$fixture"
  run awiki_git_clone_resolve "$fixture" "local-repo"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE "^${fixture}\|[0-9a-f]{40}\|main$"
}
