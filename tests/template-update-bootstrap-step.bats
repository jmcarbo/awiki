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

@test "template-update --apply: Commit C records content-changed bootstrap step as pending" {
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  git add .awiki
  git -c user.email=a@b -c user.name=t commit -q -m bootstrap
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  # Create a copy of v1 with content-changed bootstrap step (domain).
  TMP_V=$(mktemp -d)
  cp -R "$V1/." "$TMP_V/"
  sed -i.bak 's/Ask user: which domain/Ask user (UPDATED): which domain/' "$TMP_V/BOOTSTRAP.md"
  rm "$TMP_V/BOOTSTRAP.md.bak"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$TMP_V" --accept-source-change --apply --non-interactive
  [ "$status" -eq 0 ] || { echo "STATUS=$status"; echo "$output"; false; }
  # State file carries the pending entry until Commit D consolidates it.
  PENDING=$(python3 "$REPO_ROOT/scripts/_template_helpers/state.py" get \
    .awiki/template-cache/_fetch/.update-state.json bootstrap_steps_pending)
  echo "$PENDING" | grep -q '"id": "domain"'
  echo "$PENDING" | grep -q '"reason": "non-interactive default"'
  rm -rf "$TMP_V"
}
