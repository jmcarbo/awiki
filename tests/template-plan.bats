#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "escape: pipe -> %7C" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a|b'
  [ "$status" -eq 0 ]
  [ "$output" = "a%7Cb" ]
}

@test "escape: newline -> %0A" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" "$(printf 'a\nb')"
  [ "$status" -eq 0 ]
  [ "$output" = "a%0Ab" ]
}

@test "escape: percent -> %25 (encoded first)" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a%b'
  [ "$status" -eq 0 ]
  [ "$output" = "a%25b" ]
}

@test "escape: combined" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a|b%c'
  [ "$status" -eq 0 ]
  [ "$output" = "a%7Cb%25c" ]
}

@test "plan: header line emitted" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" \
    --new-tree "$V1" \
    --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc123 \
    --commit-new def456
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|header\|1\|abc123\|def456$'
}

@test "plan: emits overwrite for new scripts/new-helper.sh" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|overwrite\|scripts/new-helper.sh\|'
}

@test "plan: emits three_way for changed WIKI.md" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  # WIKI.md changed in v1; user-tree = v0, so test-merge predicts clean (no user edits).
  echo "$output" | grep -qE '^PLAN\|three_way\|WIKI.md\|clean'
}

@test "plan: emits migration line for new 0001-add-tagline" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|migration\|0001-add-tagline\|migrations/0001-add-tagline.sh\|'
  echo "$output" | grep -qE '^PLAN\|migration-content\|0001-add-tagline\|sha256:[0-9a-f]+\|[0-9]+$'
}

@test "plan: emits footer with counts" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$V0" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|footer\|errors=[0-9]+\|warnings=[0-9]+\|prompts=[0-9]+\|conflicts=[0-9]+$'
}

@test "plan: predicted conflict when user edited WIKI.md" {
  V0="$REPO_ROOT/tests/fixtures/template-update/v0"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  TMP=$(mktemp -d)
  cp -R "$V0/." "$TMP/"
  # User edits the same area new template touches.
  cat > "$TMP/WIKI.md" <<'EOF'
# Wiki schema (user-edited!)

Different conventions go here.
EOF
  run bash "$REPO_ROOT/scripts/template-plan.sh" \
    --old-tree "$V0" --new-tree "$V1" --user-tree "$TMP" \
    --manifest "$V1/template.manifest.toml" \
    --commit-old abc --commit-new def
  echo "$output" | grep -qE '^PLAN\|three_way\|WIKI.md\|conflict-predicted\|[1-9]+'
  rm -rf "$TMP"
}
