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

@test "migration run: stripped env removes GH_TOKEN" {
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  cat > m.sh <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: token-leak.txt
# idempotent: yes
set -euo pipefail
[[ "${1:-}" == "--dry-run" ]] && exit 0
echo "token=${GH_TOKEN:-EMPTY}" > token-leak.txt
EOF
  chmod +x m.sh
  GH_TOKEN=secret run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" run \
    "$TMP/m.sh" --repo-root "$TMP" --old-version 0.1 --new-version 0.2
  [ "$status" -eq 0 ]
  grep -q '^token=EMPTY$' token-leak.txt
}

@test "migration run: writes outside touches: -> halt + revert" {
  cd "$TMP"
  git init -q
  echo content > inscope.txt
  echo content > outscope.txt
  git add . && git -c user.email=a@b -c user.name=t commit -q -m init
  cat > m.sh <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: inscope.txt
# idempotent: yes
set -euo pipefail
[[ "${1:-}" == "--dry-run" ]] && exit 0
echo modified > inscope.txt
echo broken > outscope.txt
EOF
  chmod +x m.sh
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" run \
    "$TMP/m.sh" --repo-root "$TMP" --old-version 0.1 --new-version 0.2
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "outside touches"
  # outscope reverted to original
  grep -q '^content$' outscope.txt
}

@test "migration run: writes to .awiki/ -> halt + revert" {
  cd "$TMP"
  git init -q
  mkdir -p .awiki
  echo content > .awiki/template.json
  git add . && git -c user.email=a@b -c user.name=t commit -q -m init
  cat > m.sh <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: .awiki/template.json
# idempotent: yes
set -euo pipefail
[[ "${1:-}" == "--dry-run" ]] && exit 0
echo broken > .awiki/template.json
EOF
  chmod +x m.sh
  # validate-touches itself should reject .awiki/ pattern.
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" run \
    "$TMP/m.sh" --repo-root "$TMP" --old-version 0.1 --new-version 0.2
  [ "$status" -ne 0 ]
}
