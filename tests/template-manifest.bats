#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
}

@test "manifest load: emits schema_version" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE 'schema_version=1'
}

@test "manifest load: emits template_version" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE 'template_version=0\.1\.0'
}

@test "manifest load: missing file -> nonzero exit" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load /nonexistent.toml
  [ "$status" -ne 0 ]
}

@test "manifest resolve: scripts/foo.sh -> overwrite" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" scripts/foo.sh
  [ "$status" -eq 0 ]
  [ "$output" = "overwrite" ]
}

@test "manifest resolve: WIKI.md -> three_way" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" WIKI.md
  [ "$status" -eq 0 ]
  [ "$output" = "three_way" ]
}

@test "manifest resolve: .gitattributes -> attributes_merge" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" .gitattributes
  [ "$status" -eq 0 ]
  [ "$output" = "attributes_merge" ]
}

@test "manifest resolve: content/notes/foo.md -> preserve" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" content/notes/foo.md
  [ "$status" -eq 0 ]
  [ "$output" = "preserve" ]
}

@test "manifest resolve: unknown.txt -> new_file_default (prompt)" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" random/unknown.txt
  [ "$status" -eq 0 ]
  [ "$output" = "prompt" ]
}

@test "manifest resolve: most-specific glob wins (content/log.md)" {
  # Create a fixture with overlapping globs.
  TMP=$(mktemp -d)
  cat > "$TMP/manifest.toml" <<'EOF'
schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = []
preserve = ["content/**"]
three_way = ["content/log.md"]
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
EOF
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$TMP/manifest.toml" content/log.md
  [ "$status" -eq 0 ]
  [ "$output" = "three_way" ]
  rm -rf "$TMP"
}

@test "manifest bootstrap-ids: prints in declared order" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" bootstrap-ids "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "dep-check" ]
  [ "${lines[1]}" = "domain" ]
  [ "${lines[2]}" = "stage-commit" ]
}

@test "manifest dangerous-ids: empty for v0 fixture" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" dangerous-ids "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "manifest has-glob-overlap: clean fixture -> exit 0" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" has-glob-overlap "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
}

@test "manifest has-glob-overlap: duplicate within strategy -> exit 1" {
  TMP=$(mktemp -d)
  cat > "$TMP/manifest.toml" <<'EOF'
schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = ["scripts/**", "scripts/**"]
preserve = []
three_way = []
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
EOF
  run bash "$REPO_ROOT/scripts/template-manifest.sh" has-glob-overlap "$TMP/manifest.toml"
  [ "$status" -ne 0 ]
  rm -rf "$TMP"
}
