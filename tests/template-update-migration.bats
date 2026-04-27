#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "migration parse-header: extracts touches + idempotent + requires" {
  cat > "$TMP/0001-x.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: WIKI.md content/synthesis/**/*.md
# idempotent: yes
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" parse-header "$TMP/0001-x.sh"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^touches=WIKI.md content/synthesis/\*\*/\*\.md$'
  echo "$output" | grep -qE '^idempotent=yes$'
  echo "$output" | grep -qE '^requires=agent=false$'
}

@test "migration parse-header: missing required field -> nonzero" {
  cat > "$TMP/0001-x.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# touches: WIKI.md
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" parse-header "$TMP/0001-x.sh"
  [ "$status" -ne 0 ]
}

@test "migration validate-touches: secrets/ blocked" {
  cat > "$TMP/m.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: secrets/foo.age
# idempotent: yes
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" validate-touches "$TMP/m.sh"
  [ "$status" -ne 0 ]
}

@test "migration validate-touches: clean touches passes" {
  cat > "$TMP/m.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: WIKI.md
# idempotent: yes
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" validate-touches "$TMP/m.sh"
  [ "$status" -eq 0 ]
}
