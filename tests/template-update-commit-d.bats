#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "cache-rotate: keeps current + previous; deletes older" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa .awiki/template-cache/bbb .awiki/template-cache/ccc
  # Make ccc oldest, aaa newest by mtime.
  touch -t 202001011200 .awiki/template-cache/ccc
  touch -t 202101011200 .awiki/template-cache/bbb
  touch -t 202201011200 .awiki/template-cache/aaa
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/aaa ]
  [ -d .awiki/template-cache/bbb ]
  [ ! -d .awiki/template-cache/ccc ]
}

@test "cache-rotate: ignores non-sha entries (_fetch, _check-stamp)" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa .awiki/template-cache/_fetch
  touch .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/_fetch ]
  [ -f .awiki/template-cache/_check-stamp ]
}

@test "cache-rotate: with only current, no-op" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/aaa ]
}

@test "cache-rotate: missing cache-dir is ok" {
  cd "$TMP"
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
}
