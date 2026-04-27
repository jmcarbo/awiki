#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
}
teardown() { rm -rf "$TMP"; }

@test "sync apply-overwrite: copies file from new tree to user tree" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-overwrite \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh
  [ "$status" -eq 0 ]
  [ -f "$TMP/scripts/new-helper.sh" ]
  diff "$V1/scripts/new-helper.sh" "$TMP/scripts/new-helper.sh"
}

@test "sync apply-three-way: clean merge with no user changes" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-three-way \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$V1" \
    --user-tree "$TMP" \
    --rel WIKI.md
  [ "$status" -eq 0 ]
  ! grep -q '<<<<<<<' "$TMP/WIKI.md"
  grep -q '## Added in v1' "$TMP/WIKI.md"
}

@test "sync apply-three-way: conflict leaves markers + nonzero" {
  cd "$TMP"
  cat > WIKI.md <<'EOF'
# Wiki schema (user)
USER EDITED
EOF
  git add WIKI.md
  git -c user.email=a@b -c user.name=t commit -q -m user-edit
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-three-way \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$V1" --user-tree "$TMP" --rel WIKI.md
  [ "$status" -ne 0 ]
  grep -q '<<<<<<<' "$TMP/WIKI.md"
}

@test "sync apply-attributes: encryption-flip halts without --accept-attribute-changes" {
  cd "$TMP"
  echo "" > .gitattributes
  git add .gitattributes
  git -c user.email=a@b -c user.name=t commit -q -m init-attrs
  TMP_NEW=$(mktemp -d)
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." "$TMP_NEW/"
  printf 'secrets/* filter=git-crypt diff=git-crypt\n' > "$TMP_NEW/.gitattributes"
  mkdir -p secrets && echo k > secrets/key.age
  git add secrets && git -c user.email=a@b -c user.name=t commit -q -m add-secrets
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-attributes \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$TMP_NEW" \
    --user-tree "$TMP" --rel .gitattributes
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "Encryption pattern change"
  rm -rf "$TMP_NEW"
}

@test "sync apply-attributes: encryption-flip proceeds with --accept-attribute-changes" {
  cd "$TMP"
  echo "" > .gitattributes
  git add .gitattributes
  git -c user.email=a@b -c user.name=t commit -q -m init-attrs
  TMP_NEW=$(mktemp -d)
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." "$TMP_NEW/"
  printf 'secrets/* filter=git-crypt diff=git-crypt\n' > "$TMP_NEW/.gitattributes"
  mkdir -p secrets && echo k > secrets/key.age
  git add secrets && git -c user.email=a@b -c user.name=t commit -q -m add-secrets
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-attributes \
    --old-tree "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --new-tree "$TMP_NEW" \
    --user-tree "$TMP" --rel .gitattributes \
    --accept-attribute-changes
  [ "$status" -eq 0 ]
  grep -q 'filter=git-crypt' .gitattributes
  rm -rf "$TMP_NEW"
}
