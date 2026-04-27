#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "escape: pipe -> %7C" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a|b'
  [ "$status" -eq 0 ]
  [ "$output" = "a%7Cb" ]
}

@test "escape: newline -> %0A" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" "$(printf 'a\nb')"
  [ "$status" -eq 0 ]
  [ "$output" = "a%0Ab" ]
}

@test "escape: percent -> %25 (encoded first)" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a%b'
  [ "$status" -eq 0 ]
  [ "$output" = "a%25b" ]
}

@test "escape: combined" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/escape.py" 'a|b%c'
  [ "$status" -eq 0 ]
  [ "$output" = "a%7Cb%25c" ]
}
