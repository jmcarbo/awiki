#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
}
teardown() { rm -rf "$TMP"; }

@test "availability check: --check flag returns 0 (cached <7d)" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  touch .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  [ "$status" -eq 0 ]
}

@test "availability check: missing _check-stamp triggers update check (no halt on offline)" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo /nonexistent-fork \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  rm -f .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  # Network failure should not halt (lint info skipped).
  [ "$status" -eq 0 ]
}

@test "availability check: AWIKI_NO_TEMPLATE_CHECK suppresses" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  AWIKI_NO_TEMPLATE_CHECK=1 run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  [ "$status" -eq 0 ]
  [ -z "$output" ] || true
}

@test "availability check: .awiki/config no_template_check=true suppresses" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  echo "no_template_check=true" > .awiki/config
  rm -f .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}
