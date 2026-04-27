#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  PJ="$TMP/template.json"
}

teardown() { rm -rf "$TMP"; }

@test "provenance init: writes valid JSON with required fields" {
  run bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" \
    https://github.com/jmcarbo/awiki main 1.0.0 abc123def456
  [ "$status" -eq 0 ]
  [ -f "$PJ" ]
  python3 -c "import json; d=json.load(open('$PJ')); assert d['repo']=='https://github.com/jmcarbo/awiki'; assert d['original_repo']==d['repo']; assert d['commit']=='abc123def456'"
}

@test "provenance get: reads commit field" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" get "$PJ" commit
  [ "$status" -eq 0 ]
  [ "$output" = "abc123" ]
}

@test "provenance set: updates commit field, original_repo unchanged" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" set "$PJ" commit def456 >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" get "$PJ" commit
  [ "$output" = "def456" ]
  run bash "$REPO_ROOT/scripts/template-provenance.sh" get "$PJ" original_repo
  [ "$output" = "url" ]
}

@test "provenance set original_repo: refused (immutable)" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" set "$PJ" original_repo other
  [ "$status" -ne 0 ]
}

@test "provenance append-migration: adds entry" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-migration "$PJ" 0001-foo applied >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-migration "$PJ" 0002-bar skipped "user --skip-migration" >/dev/null
  run python3 -c "import json; d=json.load(open('$PJ')); print(len(d['applied_migrations']))"
  [ "$output" = "2" ]
}

@test "provenance append-bootstrap-step: adds with content_hash" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-bootstrap-step "$PJ" domain applied "" sha256:deadbeef >/dev/null
  run python3 -c "import json; d=json.load(open('$PJ')); s=d['bootstrap_steps_done'][0]; print(s['id'], s['status'], s['content_hash'])"
  [ "$output" = "domain applied sha256:deadbeef" ]
}

@test "provenance has-step-applied: returns 0 if applied with matching hash" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-bootstrap-step "$PJ" domain applied "" sha256:deadbeef >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" has-step-applied "$PJ" domain sha256:deadbeef
  [ "$status" -eq 0 ]
}

@test "provenance has-step-applied: nonzero if hash differs" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-bootstrap-step "$PJ" domain applied "" sha256:deadbeef >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" has-step-applied "$PJ" domain sha256:00ff
  [ "$status" -ne 0 ]
}
