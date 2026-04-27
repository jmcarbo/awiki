#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "---\ntitle: \"Inbox\"\ntype: inbox\ndraft: true\n---\n" > "$WORK/content/inbox.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "capture appends a line with ISO datetime prefix" {
  run bash scripts/capture.sh -- "call dentist about crown"
  [ "$status" -eq 0 ]
  run grep -E '^- 20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2} call dentist about crown$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture joins multi-arg input with single spaces" {
  run bash scripts/capture.sh -- pick up groceries
  [ "$status" -eq 0 ]
  run grep -E '^- .* pick up groceries$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture rejects empty text" {
  run bash scripts/capture.sh -- ""
  [ "$status" -eq 4 ]
  [[ "$output" == *"empty"* ]]
}

@test "capture rejects text containing a literal newline" {
  run bash scripts/capture.sh -- "line one
line two"
  [ "$status" -eq 4 ]
  [[ "$output" == *"newline"* || "$output" == *"control"* ]]
}

@test "capture rejects checkbox-prefix-at-start" {
  run bash scripts/capture.sh -- "[ ] not an action yet"
  [ "$status" -eq 4 ]
  [[ "$output" == *"checkbox"* ]]
}

@test "capture rejects all six checkbox markers at start" {
  for s in " " "/" "?" ">" "x" "-"; do
    run bash scripts/capture.sh -- "[$s] foo"
    [ "$status" -eq 4 ]
  done
}

@test "capture truncates >2000 chars and appends ellipsis" {
  long=$(printf 'a%.0s' {1..2050})
  run bash scripts/capture.sh -- "$long"
  [ "$status" -eq 0 ]
  [[ "$output" == *"length-truncated"* ]]
  # Last char of appended line should be "…".
  last=$(tail -1 content/inbox.md)
  [[ "$last" == *"…" ]]
}

@test "capture neutralizes wikilinks" {
  run bash scripts/capture.sh -- "see [[s-as-we-may-think]] later"
  [ "$status" -eq 0 ]
  [[ "$output" == *"wikilink-neutralized"* ]]
  run grep -F "[ [s-as-we-may-think] ]" content/inbox.md
  [ "$status" -eq 0 ]
  run grep -F "[[s-as-we-may-think]]" content/inbox.md
  [ "$status" -ne 0 ]
}

@test "capture neutralizes HTML comment markers" {
  run bash scripts/capture.sh -- "watch out <!-- inside --> here"
  [ "$status" -eq 0 ]
  [[ "$output" == *"comment-neutralized"* ]]
  run grep -F -e "< !--" content/inbox.md
  [ "$status" -eq 0 ]
  run grep -F -e "--  >" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture backslash-escapes block-ID-shaped tokens" {
  run bash scripts/capture.sh -- "remember ^abc1234 token"
  [ "$status" -eq 0 ]
  [[ "$output" == *"block-id-escaped"* ]]
  run grep -F "\\^abc1234" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture leaves a clean line untouched" {
  run bash scripts/capture.sh -- "totally normal text"
  [ "$status" -eq 0 ]
  [[ "$output" != *"SANITIZATION-APPLIED"* ]]
}

@test "capture creates inbox.md if missing (with frontmatter scaffold)" {
  rm content/inbox.md
  run bash scripts/capture.sh -- "first capture"
  [ "$status" -eq 0 ]
  run head -1 content/inbox.md
  [[ "$output" == "---" ]]
  run grep '^type: inbox$' content/inbox.md
  [ "$status" -eq 0 ]
  run grep -E '^- .* first capture$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture rejects tab as newline-equivalent control? no — tab is converted to space" {
  printf -- "with\ttab" > /tmp/awiki-cap-tab.txt
  txt="$(cat /tmp/awiki-cap-tab.txt)"
  run bash scripts/capture.sh -- "$txt"
  [ "$status" -eq 0 ]
  run grep -E '^- .* with tab$' content/inbox.md
  [ "$status" -eq 0 ]
  rm -f /tmp/awiki-cap-tab.txt
}
