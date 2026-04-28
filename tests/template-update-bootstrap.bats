#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "production manifest exists at repo root" {
  [ -f "$REPO_ROOT/template.manifest.toml" ]
}

@test "production manifest: schema_version=1, parses cleanly" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$REPO_ROOT/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^schema_version=1$'
}

@test "production manifest: dangerous-ids includes theme + wire-* + install-qmd" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" dangerous-ids "$REPO_ROOT/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qx "theme"
  echo "$output" | grep -qx "wire-qmd-mcp"
  echo "$output" | grep -qx "wire-awiki-mcp"
  echo "$output" | grep -qx "install-qmd"
}

@test "production manifest: bootstrap.ordered_steps matches BOOTSTRAP.md markers (bidirectional)" {
  MANIFEST_IDS=$(bash "$REPO_ROOT/scripts/template-manifest.sh" bootstrap-ids "$REPO_ROOT/template.manifest.toml" | sort)
  BS_IDS=$(python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" list "$REPO_ROOT/BOOTSTRAP.md" | sort)
  [ "$MANIFEST_IDS" = "$BS_IDS" ]
}
