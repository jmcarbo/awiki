#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  AWIKI_BIN="$REPO_ROOT/bin/awiki"
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

  run "$AWIKI_BIN" capture -- "pick up groceries"
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
  run "$AWIKI_BIN" capture -- "see [[s-as-we-may-think]] later"
  [ "$status" -eq 0 ]
  [[ "$output" == *"wikilink-neutralized"* ]]
  run grep -F "[ [s-as-we-may-think] ]" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "smoke: idempotent task-init followed by capture twice" {
  bash scripts/task-init.sh >/dev/null
  bash scripts/task-init.sh >/dev/null
  "$AWIKI_BIN" capture -- "first"  >/dev/null
  "$AWIKI_BIN" capture -- "second" >/dev/null
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

@test "smoke phase 18a: capture → triage-apply act → agenda → flip [x] → agenda removes" {
  PHASE18A_WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || cd "$BATS_TEST_DIRNAME/.." && pwd)"
  cp -R "$REPO_ROOT" "$PHASE18A_WORK/awiki"
  cd "$PHASE18A_WORK/awiki"
  AWIKI_TASK_INIT_ASSUME_NO=1 just task-init
  just capture "call dentist about crown"

  # Locate the captured line and synthesize its inbox-<sha>-<lineno> id.
  local lineno
  lineno="$(awk '/^- /{n=NR} END{print n}' content/inbox.md)"
  [ -n "$lineno" ]
  local raw
  raw="$(awk -v n="$lineno" 'NR==n' content/inbox.md)"
  local sha
  sha="$(printf '%s' "$raw" | shasum | awk '{print substr($1,1,10)}')"
  local id="inbox-${sha}-${lineno}"

  # Apply act outcome to _loose with @phone context via the bash CLI form.
  run bash scripts/triage.sh "$id" act project_slug=_loose context_slug=phone "lineno=${lineno}"
  [ "$status" -eq 0 ]
  [ -f content/projects/_loose.md ]
  run grep -E '^- \[ \] call dentist about crown @phone \^[a-z0-9]{8}' content/projects/_loose.md
  [ "$status" -eq 0 ]

  # Inbox should no longer contain the captured line.
  run grep -F 'call dentist about crown' content/inbox.md
  [ "$status" -ne 0 ]

  # First agenda regen — action should appear under @phone in next-actions.
  just agenda
  run grep -F '### @phone' content/agenda/next-actions.md
  [ -n "$output" ]
  run grep -F 'call dentist about crown' content/agenda/next-actions.md
  [ -n "$output" ]

  # Flip [ ] -> [x] in _loose.md. Also clear the previous next-actions
  # body inside the managed region so the next scan does not see two
  # copies of the same ^id (one open, one done) and reject as dup-id.
  if sed --version >/dev/null 2>&1; then
    sed -i 's/^- \[ \] call dentist/- [x] call dentist/' content/projects/_loose.md
  else
    sed -i '' 's/^- \[ \] call dentist/- [x] call dentist/' content/projects/_loose.md
  fi
  awk '
    BEGIN { drop = 0 }
    /<!-- BEGIN agenda:/ { print; drop = 1; next }
    /<!-- END agenda:/   { drop = 0; print; next }
    !drop                { print }
  ' content/agenda/next-actions.md > content/agenda/next-actions.md.tmp \
    && mv content/agenda/next-actions.md.tmp content/agenda/next-actions.md

  just agenda
  run grep -F 'call dentist about crown' content/agenda/next-actions.md
  [ -z "$output" ]

  cd /
  rm -rf "$PHASE18A_WORK"
}
