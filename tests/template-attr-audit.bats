#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
}

teardown() { rm -rf "$TMP"; }

@test "attr-audit: detects non-encryption attribute change" {
  cd "$TMP"
  echo "content" > tracked.txt
  git add tracked.txt
  git commit -q -m add
  printf '' > .gitattributes-old
  printf 'tracked.txt diff=foo\n' > .gitattributes-new
  run bash "$REPO_ROOT/scripts/template-attr-audit.sh" \
    --old .gitattributes-old --new .gitattributes-new --paths tracked.txt
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^PLAN\|attribute-change\|tracked.txt\|'
}

@test "attr-audit: no change -> no output" {
  cd "$TMP"
  echo content > tracked.txt
  git add tracked.txt
  git commit -q -m add
  printf 'tracked.txt diff=foo\n' > .gitattributes-old
  printf 'tracked.txt diff=foo\n' > .gitattributes-new
  run bash "$REPO_ROOT/scripts/template-attr-audit.sh" \
    --old .gitattributes-old --new .gitattributes-new --paths tracked.txt
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "attr-audit: filter=git-crypt added -> exit 1 (signals encryption flip)" {
  cd "$TMP"
  echo content > tracked.txt
  git add tracked.txt
  git commit -q -m add
  printf '' > .gitattributes-old
  printf 'tracked.txt filter=git-crypt diff=git-crypt\n' > .gitattributes-new
  run bash "$REPO_ROOT/scripts/template-attr-audit.sh" \
    --old .gitattributes-old --new .gitattributes-new --paths tracked.txt
  [ "$status" -eq 1 ]
}
