#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git -c user.email=a@b -c user.name=t add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT=$(git rev-parse HEAD)
}
teardown() { rm -rf "$TMP"; }

@test "retrofit: no .awiki/template.json -> prompts and runs template-init" {
  cd "$TMP"
  [ ! -f .awiki/template.json ]
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive
  [ "$status" -eq 0 ]
  [ -f .awiki/template.json ]
}

@test "retrofit: existing template.json -> no-op with info" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$COMMIT" >/dev/null
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo url --ref main --version 0.1.0 --commit "$COMMIT" --non-interactive
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "already"
}

@test "retrofit: heuristic suggests install-qmd from .awiki/qmd-status" {
  cd "$TMP"
  mkdir -p .awiki
  echo "ok" > .awiki/qmd-status
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive --heuristics-only
  echo "$output" | grep -q "install-qmd"
}

@test "retrofit: heuristic detects git-crypt from .gitattributes" {
  cd "$TMP"
  echo "secrets/** filter=git-crypt diff=git-crypt" > .gitattributes
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo url --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive --heuristics-only
  echo "$output" | grep -q "privacy=git-crypt"
}

@test "retrofit: heuristic detects age from secrets/age-key.txt" {
  cd "$TMP"
  mkdir -p secrets
  echo "AGE-SECRET-KEY-..." > secrets/age-key.txt
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo url --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive --heuristics-only
  echo "$output" | grep -q "privacy=age"
}

@test "retrofit: heuristic detects awiki-server in .mcp.json" {
  cd "$TMP"
  cat > .mcp.json <<'EOF'
{"mcpServers": {"awiki-server": {"command": "awiki-mcp"}}}
EOF
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo url --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive --heuristics-only
  echo "$output" | grep -q "wire-awiki-mcp"
}

@test "retrofit: heuristic detects awiki-server in .cursor/mcp.json" {
  cd "$TMP"
  mkdir -p .cursor
  cat > .cursor/mcp.json <<'EOF'
{"mcpServers": {"awiki-server": {"command": "awiki-mcp"}}}
EOF
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo url --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive --heuristics-only
  echo "$output" | grep -q "wire-awiki-mcp"
}
