#!/usr/bin/env bats

setup() {
  TMP="$(mktemp -d)"
  export AWIKI_REPO_ROOT="$TMP"
  mkdir -p "$TMP/.awiki/maps" "$TMP/content/projects" "$TMP/scripts/lib"
  cp "$BATS_TEST_DIRNAME/../scripts/action-recur.sh" "$TMP/scripts/action-recur.sh" 2>/dev/null || true
  cp "$BATS_TEST_DIRNAME/../scripts/lib/action-grammar.sh" "$TMP/scripts/lib/action-grammar.sh"
  cp "$BATS_TEST_DIRNAME/../scripts/lib/lock.sh" "$TMP/scripts/lib/lock.sh"
  cp "$BATS_TEST_DIRNAME/../scripts/log-append.sh" "$TMP/scripts/log-append.sh"
  touch "$TMP/.awiki/log"
  cd "$TMP"
}

teardown() {
  rm -rf "$TMP"
}

# Helper: write a fixture project page with one completed weekly action.
seed_weekly_done_a05() {
  cat > content/projects/garden.md <<'EOF'
---
title: "Garden"
date: 2026-04-27
last_updated: 2026-05-04
type: project
status: active
outcome: "Healthy plants."
tags: [home]
draft: false
---

## Open Actions

## Done

- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  # Minimal actions.tsv for the script to consult — single chain head, no instances yet.
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/garden.md	14	home		    	1w	2026-05-04				garden	public
EOF
}

@test "action-recur.sh --help prints usage and exits 0" {
  run bash scripts/action-recur.sh --help
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "Usage:"
}

@test "action-recur.sh rejects unknown flag with exit 2" {
  run bash scripts/action-recur.sh --bogus content/projects/garden.md
  [ "$status" -eq 2 ]
}

@test "action-recur.sh --dry-run prints unified diff and writes nothing" {
  seed_weekly_done_a05
  run bash scripts/action-recur.sh --dry-run content/projects/garden.md
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '^---'
  echo "$output" | grep -q '^+++'
  echo "$output" | grep -q '^+- \[ \] water plants @home every:1w due:2026-05-11'
  # Page on disk unchanged.
  grep -c '^- \[ \]' content/projects/garden.md | grep -qx 0
}
