#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "state init: writes JSON with required keys" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch awiki-template-update/bbb
  [ "$status" -eq 0 ]
  python3 -c "
import json
d = json.load(open('$TMP/state.json'))
assert d['phase'] == 'fetch'
assert d['status'] == 'committed'
assert d['commit_old'] == 'aaa'
assert d['commit_new'] == 'bbb'
assert d['branch'] == 'awiki-template-update/bbb'
assert d['last_completed_commit'] is None
assert d['applied_migrations_pending'] == []
assert d['bootstrap_steps_pending'] == []
assert d['deletions_user_decisions'] == {}
assert d['deleted_pending'] == []
"
}

@test "state validate: passes on clean init; fails on missing fields" {
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch br
  run python3 "$REPO_ROOT/scripts/_template_helpers/state.py" validate "$TMP/state.json"
  [ "$status" -eq 0 ]
  echo '{"phase": "fetch"}' > "$TMP/bad.json"
  run python3 "$REPO_ROOT/scripts/_template_helpers/state.py" validate "$TMP/bad.json"
  [ "$status" -ne 0 ]
}

@test "state set-phase: updates phase and status" {
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch br
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" set-phase "$TMP/state.json" \
    --phase commit-a --status started
  run python3 -c "import json; d=json.load(open('$TMP/state.json')); print(d['phase'], d['status'])"
  [ "$output" = "commit-a started" ]
}

@test "state set-last-completed: updates SHA" {
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch br
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" set-last-completed "$TMP/state.json" deadbeef
  run python3 -c "import json; print(json.load(open('$TMP/state.json'))['last_completed_commit'])"
  [ "$output" = "deadbeef" ]
}

@test "preflight: clean tree -> ok" {
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-tree
  [ "$status" -eq 0 ]
}

@test "preflight: untracked file -> halt" {
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  echo dirty > leftover.txt
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-tree
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "untracked"
}

@test "preflight: unstaged change -> halt" {
  cd "$TMP"
  git init -q
  echo a > tracked.txt
  git add tracked.txt
  git -c user.email=a@b -c user.name=t commit -q -m add
  echo b > tracked.txt
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-tree
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "modified|unstaged"
}

@test "preflight: branch on main -> ok" {
  cd "$TMP"
  git init -q -b main
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-branch --expected main
  [ "$status" -eq 0 ]
}

@test "preflight: branch on update branch -> halt" {
  cd "$TMP"
  git init -q -b main
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  git checkout -q -b awiki-template-update/abc
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-branch --expected main
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "awiki-template-update"
}

@test "preflight: empty pending-prompts dir -> ok" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-pending-prompts
  [ "$status" -eq 0 ]
}

@test "preflight: non-empty pending-prompts -> halt" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo body > .awiki/pending-prompts/0001-x.md
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-pending-prompts
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "0001-x.md"
}
