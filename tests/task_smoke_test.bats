#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "smoke: task-init then capture writes a line into inbox" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/inbox.md ]

  run bash scripts/capture.sh -- "pick up groceries"
  [ "$status" -eq 0 ]

  run grep -E '^- 20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2} pick up groceries$' content/inbox.md
  [ "$status" -eq 0 ]

  run head -1 content/inbox.md
  [[ "$output" == "---" ]]

  [ -f .awiki/task-count ]
  [ -f .awiki/last-review ]

  [ -f content/agenda/next-actions.md ]
  run grep '^<!-- BEGIN agenda:next-actions -->$' content/agenda/next-actions.md
  [ "$status" -eq 0 ]

  run grep '^<!-- BEGIN task-layer -->$' WIKI.md
  [ "$status" -eq 0 ]
}

@test "smoke: capture's sanitization fires when needed" {
  bash scripts/task-init.sh >/dev/null
  run bash scripts/capture.sh -- "see [[s-as-we-may-think]] later"
  [ "$status" -eq 0 ]
  [[ "$output" == *"wikilink-neutralized"* ]]
  run grep -F "[ [s-as-we-may-think] ]" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "smoke: idempotent task-init followed by capture twice" {
  bash scripts/task-init.sh >/dev/null
  bash scripts/task-init.sh >/dev/null
  bash scripts/capture.sh -- "first"  >/dev/null
  bash scripts/capture.sh -- "second" >/dev/null
  run grep -c '^- ' content/inbox.md
  [[ "$output" == "2" ]]
}
