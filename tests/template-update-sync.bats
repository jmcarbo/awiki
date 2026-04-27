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

@test "sync apply-new-file overwrite: copies into user tree" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-new-file \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh \
    --decision overwrite
  [ "$status" -eq 0 ]
  [ -f scripts/new-helper.sh ]
}

@test "sync apply-new-file skip: does not copy" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-new-file \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh \
    --decision skip
  [ "$status" -eq 0 ]
  [ ! -f scripts/new-helper.sh ]
}

@test "sync apply-new-file mark-as-user-deleted: does not copy" {
  cd "$TMP"
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-new-file \
    --new-tree "$V1" --user-tree "$TMP" --rel scripts/new-helper.sh \
    --decision mark-as-user-deleted
  [ "$status" -eq 0 ]
  [ ! -f scripts/new-helper.sh ]
}

@test "sync apply-deletion remove: deletes file" {
  cd "$TMP"
  mkdir -p scripts && echo x > scripts/old.sh
  git add . && git -c user.email=a@b -c user.name=t commit -q -m add
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-deletion \
    --user-tree "$TMP" --rel scripts/old.sh --decision remove
  [ "$status" -eq 0 ]
  [ ! -f scripts/old.sh ]
}

@test "sync apply-deletion preserve-local: keeps file" {
  cd "$TMP"
  mkdir -p scripts && echo x > scripts/keepme.sh
  git add . && git -c user.email=a@b -c user.name=t commit -q -m add
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" apply-deletion \
    --user-tree "$TMP" --rel scripts/keepme.sh --decision preserve-local
  [ "$status" -eq 0 ]
  [ -f scripts/keepme.sh ]
}

@test "sync has-conflict-markers: detects" {
  cd "$TMP"
  cat > WIKI.md <<EOF
<<<<<<< current
a
=======
b
>>>>>>> new
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" has-conflict-markers \
    --tree "$TMP" --paths WIKI.md
  [ "$status" -eq 1 ]
}

@test "sync has-conflict-markers: clean file -> no detection" {
  cd "$TMP"
  echo clean > WIKI.md
  run python3 "$REPO_ROOT/scripts/_template_helpers/sync.py" has-conflict-markers \
    --tree "$TMP" --paths WIKI.md
  [ "$status" -eq 0 ]
}

@test "template-update --apply: Commit A creates branch and applies sync" {
  cd "$TMP"
  # Pin to v0 (current HEAD); update will move to v1.
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  git add .awiki
  git -c user.email=a@b -c user.name=t commit -q -m bootstrap
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --non-interactive
  [ "$status" -eq 0 ] || {
    echo "STATUS=$status"
    echo "$output"
    false
  }
  CUR_BRANCH=$(git rev-parse --abbrev-ref HEAD)
  [[ "$CUR_BRANCH" =~ ^awiki-template-update/ ]]
  [ -f scripts/new-helper.sh ]
  grep -q "Added in v1" WIKI.md
  git log --format=%s -1 | grep -q "sync to"
}
