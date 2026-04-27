#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
}

@test "bootstrap-hash: computes sha256 for known step" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$FIXTURE/BOOTSTRAP.md" dep-check
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^sha256: ]]
  [ "${#output}" -eq 71 ]   # sha256: + 64 hex
}

@test "bootstrap-hash: same body -> same hash (whitespace-normalized)" {
  TMP=$(mktemp -d)
  cat > "$TMP/a.md" <<'EOF'
### Step 1.
<!-- bootstrap-step: x -->
hello world

EOF
  cat > "$TMP/b.md" <<'EOF'
### Step 1.
<!-- bootstrap-step: x -->
   hello   world
EOF
  HASH_A=$(python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$TMP/a.md" x)
  HASH_B=$(python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$TMP/b.md" x)
  [ "$HASH_A" = "$HASH_B" ]
  rm -rf "$TMP"
}

@test "bootstrap-hash: missing step -> nonzero exit" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$FIXTURE/BOOTSTRAP.md" nonexistent
  [ "$status" -ne 0 ]
}

@test "bootstrap-hash: list emits all step IDs" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" list "$FIXTURE/BOOTSTRAP.md"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "^dep-check$"
  echo "$output" | grep -q "^domain$"
  echo "$output" | grep -q "^stage-commit$"
}
