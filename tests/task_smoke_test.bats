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

@test "smoke phase 17: capture → manual triage → agenda → action under @phone" {
  PHASE17_WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || cd "$BATS_TEST_DIRNAME/.." && pwd)"
  cp -R "$REPO_ROOT" "$PHASE17_WORK/awiki"
  cd "$PHASE17_WORK/awiki"
  AWIKI_TASK_INIT_ASSUME_NO=1 just task-init
  just capture "call dentist about crown"

  mkdir -p content/projects
  cat > content/projects/dentist.md <<'PROJEOM'
---
title: "Dentist"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [ ] call dentist about crown @phone ^d01
PROJEOM
  awk '!/call dentist about crown/' content/inbox.md > content/inbox.md.tmp \
    && mv content/inbox.md.tmp content/inbox.md

  just agenda

  run grep -F '### @phone' content/agenda/next-actions.md
  [ -n "$output" ]
  run grep -F 'call dentist about crown' content/agenda/next-actions.md
  [ -n "$output" ]

  cd /
  rm -rf "$PHASE17_WORK"
}
