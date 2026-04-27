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

@test "action-recur: every:1m clamps 2026-01-31 to 2026-02-28" {
  mkdir -p content/projects
  cat > content/projects/billing.md <<'EOF'
---
title: "Billing"
type: project
status: active
last_updated: 2026-01-31
draft: false
---

## Done

- [x] file VAT @computer every:1m due:2026-01-31 done:2026-01-31 ^v01
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
v01	x	file VAT	content/projects/billing.md	10	computer		    	1m	2026-01-31				billing	public
EOF
  run bash scripts/action-recur.sh content/projects/billing.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] file VAT @computer every:1m due:2026-02-28 \^v01~2$' content/projects/billing.md
}

@test "action-recur: every:1m on 2026-03-15 -> 2026-04-15 (no clamp)" {
  mkdir -p content/projects
  cat > content/projects/billing.md <<'EOF'
---
title: "Billing"
type: project
status: active
last_updated: 2026-03-15
draft: false
---

## Done

- [x] file VAT @computer every:1m due:2026-03-15 done:2026-03-15 ^v02
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
v02	x	file VAT	content/projects/billing.md	10	computer		    	1m	2026-03-15				billing	public
EOF
  run bash scripts/action-recur.sh content/projects/billing.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] file VAT @computer every:1m due:2026-04-15 \^v02~2$' content/projects/billing.md
}

@test "action-recur: every:1m on 2024-01-31 -> 2024-02-29 (leap year)" {
  mkdir -p content/projects
  cat > content/projects/billing.md <<'EOF'
---
title: "Billing"
type: project
status: active
last_updated: 2024-01-31
draft: false
---

## Done

- [x] file VAT @computer every:1m due:2024-01-31 done:2024-01-31 ^v03
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
v03	x	file VAT	content/projects/billing.md	10	computer		    	1m	2024-01-31				billing	public
EOF
  run bash scripts/action-recur.sh content/projects/billing.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] file VAT @computer every:1m due:2024-02-29 \^v03~2$' content/projects/billing.md
}

@test "action-recur: every:1w on 2026-05-04 -> 2026-05-11" {
  seed_weekly_done_a05
  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] water plants @home every:1w due:2026-05-11 \^a05~2$' content/projects/garden.md
}

@test "action-recur: every:3d on 2026-04-27 -> 2026-04-30" {
  mkdir -p content/projects
  cat > content/projects/study.md <<'EOF'
---
title: "Study"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] review flashcards @computer every:3d due:2026-04-27 done:2026-04-27 ^s01
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
s01	x	review flashcards	content/projects/study.md	10	computer		    	3d	2026-04-27				study	public
EOF
  run bash scripts/action-recur.sh content/projects/study.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] review flashcards @computer every:3d due:2026-04-30 \^s01~2$' content/projects/study.md
}
