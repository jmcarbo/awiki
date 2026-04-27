#!/usr/bin/env bats

setup() {
  TEST_DIR="$(mktemp -d)"
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  cp -r "$REPO_ROOT/scripts" "$TEST_DIR/scripts"
  cd "$TEST_DIR"
}

teardown() {
  cd /
  rm -rf "$TEST_DIR"
}

@test "install-qmd is idempotent if qmd already on PATH" {
  if ! command -v qmd >/dev/null 2>&1; then skip "qmd not installed"; fi
  run bash scripts/install-qmd.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"already installed"* ]]
}

@test "install-qmd writes status file on success" {
  if ! command -v cargo >/dev/null 2>&1 && ! command -v qmd >/dev/null 2>&1; then
    skip "no toolchain to build qmd and qmd not on PATH"
  fi
  run bash scripts/install-qmd.sh
  if [ "$status" -eq 0 ]; then
    [ -f .awiki/qmd-status ]
    [[ "$(cat .awiki/qmd-status)" =~ ^(ok|missing)$ ]]
  else
    # Build can fail in constrained environments; status file must still
    # reflect the failure so the agent uses the grep fallback.
    [ -f .awiki/qmd-status ]
    [ "$(cat .awiki/qmd-status)" = "missing" ]
  fi
}

@test "install-qmd writes status=missing when cargo and qmd both unavailable" {
  if command -v qmd >/dev/null 2>&1; then skip "qmd is on PATH"; fi
  if command -v cargo >/dev/null 2>&1; then skip "cargo is available"; fi
  run bash scripts/install-qmd.sh
  [ "$status" -ne 0 ]
  [ -f .awiki/qmd-status ]
  [ "$(cat .awiki/qmd-status)" = "missing" ]
}
