#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "bootstrap-replay: emits dangerous for dangerous IDs" {
  cd "$TMP"
  cat > BOOTSTRAP.md <<'EOF'
### Step 1
<!-- bootstrap-step: theme -->
mkdir -p themes/foo
EOF
  cat > template.manifest.toml <<'EOF'
schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite=[]
preserve=[]
three_way=[]
attributes_merge=[]
template_only=[]
[new_file_default]
strategy="prompt"
[bootstrap]
ordered_steps = ["theme"]
[bootstrap.dangerous]
ids = ["theme"]
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_replay.py" classify \
    --bootstrap BOOTSTRAP.md --manifest template.manifest.toml --id theme
  [ "$status" -eq 0 ]
  [ "$output" = "dangerous" ]
}

@test "bootstrap-replay: classifies new step as ok" {
  cd "$TMP"
  cat > BOOTSTRAP.md <<'EOF'
### Step 1
<!-- bootstrap-step: domain -->
ask domain
EOF
  cat > template.manifest.toml <<'EOF'
schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite=[]
preserve=[]
three_way=[]
attributes_merge=[]
template_only=[]
[new_file_default]
strategy="prompt"
[bootstrap]
ordered_steps = ["domain"]
[bootstrap.dangerous]
ids = []
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_replay.py" classify \
    --bootstrap BOOTSTRAP.md --manifest template.manifest.toml --id domain
  [ "$status" -eq 0 ]
  [ "$output" = "ok" ]
}

@test "bootstrap-replay: classifies missing step as missing (exit 1)" {
  cd "$TMP"
  cat > BOOTSTRAP.md <<'EOF'
### Step 1
<!-- bootstrap-step: domain -->
ask domain
EOF
  cat > template.manifest.toml <<'EOF'
schema_version = 1
template_version = "0.1.0"
[strategies]
overwrite=[]
preserve=[]
three_way=[]
attributes_merge=[]
template_only=[]
[new_file_default]
strategy="prompt"
[bootstrap]
ordered_steps = ["domain"]
[bootstrap.dangerous]
ids = []
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_replay.py" classify \
    --bootstrap BOOTSTRAP.md --manifest template.manifest.toml --id nonexistent
  [ "$status" -eq 1 ]
  [ "$output" = "missing" ]
}

@test "bootstrap-replay: body prints the raw step body" {
  cd "$TMP"
  cat > BOOTSTRAP.md <<'EOF'
### Step 1
<!-- bootstrap-step: domain -->
echo hello
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_replay.py" body \
    --bootstrap BOOTSTRAP.md --id domain
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "echo hello"
}
