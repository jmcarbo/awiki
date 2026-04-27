#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}

teardown() { rm -rf "$TMP"; }

@test "config get: existing key returns value" {
  cat > "$TMP/config" <<EOF
default_branch=main
no_template_check=true
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/config" default_branch
  [ "$status" -eq 0 ]
  [ "$output" = "main" ]
}

@test "config get: missing key returns default" {
  echo "default_branch=main" > "$TMP/config"
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/config" require_signature false
  [ "$status" -eq 0 ]
  [ "$output" = "false" ]
}

@test "config get: missing file returns default" {
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/nonexistent" default_branch fallback
  [ "$status" -eq 0 ]
  [ "$output" = "fallback" ]
}

@test "config get: comments and blank lines ignored" {
  cat > "$TMP/config" <<'EOF'
# this is a comment
default_branch=main

# another comment
require_signature=true
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/config" require_signature
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "config validate: unknown key warns" {
  cat > "$TMP/config" <<EOF
default_branch=main
unknown_key=foo
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" validate "$TMP/config"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "unknown_key"
}

@test "config validate: malformed line errors" {
  cat > "$TMP/config" <<EOF
default_branch main
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" validate "$TMP/config"
  [ "$status" -ne 0 ]
}
