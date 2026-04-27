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
