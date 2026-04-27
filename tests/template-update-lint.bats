#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
}
teardown() { rm -rf "$TMP"; }

@test "lint_template: warns on identical globs in same strategy" {
  cd "$TMP"
  cat > template.manifest.toml <<'EOF'
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
  run python3 "$REPO_ROOT/scripts/_template_helpers/lint_template.py" --root "$TMP"
  echo "$output" | grep -q "duplicate glob"
}

@test "lint_template: fails on missing migration headers" {
  cd "$TMP"
  cp "$REPO_ROOT/template.manifest.toml" .
  mkdir -p migrations
  cat > migrations/0001-bad.sh <<'EOF'
#!/usr/bin/env bash
echo no headers
EOF
  chmod +x migrations/0001-bad.sh
  run python3 "$REPO_ROOT/scripts/_template_helpers/lint_template.py" --root "$TMP"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "missing header"
}

@test "lint_template: warns on aged pending-prompts" {
  cd "$TMP"
  cp "$REPO_ROOT/template.manifest.toml" .
  mkdir -p .awiki/pending-prompts
  echo body > .awiki/pending-prompts/0001-old.md
  touch -t 202001011200 .awiki/pending-prompts/0001-old.md
  run python3 "$REPO_ROOT/scripts/_template_helpers/lint_template.py" --root "$TMP"
  echo "$output" | grep -qE "old|aged|14"
}
