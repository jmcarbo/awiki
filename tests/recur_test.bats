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

@test "action-recur: re-running on the same page is a no-op (idempotent)" {
  seed_weekly_done_a05
  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  # First run produced one open instance.
  open_count=$(grep -c '^- \[ \] water plants' content/projects/garden.md)
  [ "$open_count" -eq 1 ]

  # Re-scan: actions.tsv would now include the new open instance, but the test
  # bypasses action-scan.sh; build the next actions.tsv by hand to mirror reality.
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/garden.md	15	home		    	1w	2026-05-04				garden	public
a05~2	 	water plants	content/projects/garden.md	14	home	2026-05-11	    	1w					garden	public
EOF

  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  # No new instance created.
  open_count=$(grep -c '^- \[ \] water plants' content/projects/garden.md)
  [ "$open_count" -eq 1 ]
}

@test "action-recur: refuses to emit at chain length 200 (exit 6)" {
  mkdir -p content/projects
  # Synthesize a page with one [x] head + 198 [ ] instances numbered 2..199.
  # Next emit would be ^h01~200, which the cap rejects.
  {
    cat <<'EOF'
---
title: "Cap test"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] tick @computer every:1d due:2026-04-27 done:2026-04-27 ^h01
EOF
    for n in $(seq 2 199); do
      printf -- '- [ ] tick @computer every:1d due:2026-04-%02d ^h01~%d\n' $((27 + n)) "$n"
    done
  } > content/projects/cap.md
  # actions.tsv with 199 instances.
  {
    printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n'
    printf 'h01\tx\ttick\tcontent/projects/cap.md\t10\tcomputer\t\t\t\t\t1d\t2026-04-27\t\t\tcap\tpublic\n'
    for n in $(seq 2 199); do
      printf 'h01~%d\t \ttick\tcontent/projects/cap.md\t%d\tcomputer\t\t\t\t\t1d\t\t\t\tcap\tpublic\n' "$n" "$((10 + n))"
    done
  } > .awiki/maps/actions.tsv

  run bash scripts/action-recur.sh content/projects/cap.md
  [ "$status" -eq 6 ]
  echo "$output" | grep -q "chain \^h01 reached 200 instances; refusing to emit"
  # Page must NOT have a 200th instance.
  ! grep -q '\^h01~200' content/projects/cap.md
}

@test "action-recur: cap leaves no orphan temp files" {
  mkdir -p content/projects
  {
    cat <<'EOF'
---
title: "Cap test 2"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] tick @computer every:1d due:2026-04-27 done:2026-04-27 ^h02
EOF
    for n in $(seq 2 199); do
      printf -- '- [ ] tick @computer every:1d due:2026-04-%02d ^h02~%d\n' $((27 + n)) "$n"
    done
  } > content/projects/cap2.md
  {
    printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n'
    printf 'h02\tx\ttick\tcontent/projects/cap2.md\t10\tcomputer\t\t\t\t\t1d\t2026-04-27\t\t\tcap2\tpublic\n'
    for n in $(seq 2 199); do
      printf 'h02~%d\t \ttick\tcontent/projects/cap2.md\t%d\tcomputer\t\t\t\t\t1d\t\t\t\tcap2\tpublic\n' "$n" "$((10 + n))"
    done
  } > .awiki/maps/actions.tsv

  run bash scripts/action-recur.sh content/projects/cap2.md
  [ "$status" -eq 6 ]
  # No orphan tmp files in content/projects.
  run bash -c 'ls content/projects/*.tmp.* 2>/dev/null | wc -l | tr -d " "'
  [ "$output" = "0" ]
}

@test "action-recur: mid-chain every: change reads interval from completed line" {
  mkdir -p content/projects
  cat > content/projects/garden2.md <<'EOF'
---
title: "Garden 2"
type: project
status: active
last_updated: 2026-05-25
draft: false
---

## Open Actions

## Done

- [x] water plants @home every:2w due:2026-05-25 done:2026-05-25 ^a05~3
- [x] water plants @home every:1w due:2026-05-18 done:2026-05-18 ^a05~2
- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/garden2.md	16	home		    	1w	2026-05-04				garden2	public
a05~2	x	water plants	content/projects/garden2.md	15	home		    	1w	2026-05-18				garden2	public
a05~3	x	water plants	content/projects/garden2.md	14	home		    	2w	2026-05-25				garden2	public
EOF

  run bash scripts/action-recur.sh content/projects/garden2.md
  [ "$status" -eq 0 ]
  # Top of chain (the most recent [x]) is ~3 with every:2w done:2026-05-25.
  # Next emit is ^a05~4 with due = 2026-05-25 + 14d = 2026-06-08.
  grep -q '^- \[ \] water plants @home every:2w due:2026-06-08 \^a05~4$' content/projects/garden2.md
  # And the older [x] lines should NOT each have spawned a new emit because
  # their next instance numbers (~2 onward) are already present in the chain.
  emit_count=$(grep -c '^- \[ \] water plants' content/projects/garden2.md)
  [ "$emit_count" -eq 1 ]
}

@test "action-recur: chain detection works under AWIKI_RECUR_SEP=~ (default)" {
  source scripts/lib/action-grammar.sh
  [ "$AWIKI_RECUR_SEP" = "~" ] || skip "spike pinned a different separator"
  seed_weekly_done_a05
  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  grep -q '\^a05~2$' content/projects/garden.md
}

@test "action-recur: chain detection works under AWIKI_RECUR_SEP=__ (fallback)" {
  # Override the separator via the env var (the lib uses ${AWIKI_RECUR_SEP:-~}).
  export AWIKI_RECUR_SEP=__
  mkdir -p content/projects
  cat > content/projects/sep.md <<'EOF'
---
title: "Sep test"
type: project
status: active
last_updated: 2026-05-04
draft: false
---

## Done

- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/sep.md	10	home		    	1w	2026-05-04				sep	public
EOF
  run bash scripts/action-recur.sh content/projects/sep.md
  [ "$status" -eq 0 ]
  grep -q '\^a05__2$' content/projects/sep.md
  # The ~ shape MUST NOT appear under fallback mode.
  ! grep -q '\^a05~2' content/projects/sep.md
}

@test "action-recur: cap message goes to stderr, not stdout" {
  mkdir -p content/projects
  {
    cat <<'EOF'
---
title: "Cap test 3"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] tick @computer every:1d due:2026-04-27 done:2026-04-27 ^h03
EOF
    for n in $(seq 2 199); do
      printf -- '- [ ] tick @computer every:1d due:2026-04-%02d ^h03~%d\n' $((27 + n)) "$n"
    done
  } > content/projects/cap3.md
  {
    printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n'
    printf 'h03\tx\ttick\tcontent/projects/cap3.md\t10\tcomputer\t\t\t\t\t1d\t2026-04-27\t\t\tcap3\tpublic\n'
    for n in $(seq 2 199); do
      printf 'h03~%d\t \ttick\tcontent/projects/cap3.md\t%d\tcomputer\t\t\t\t\t1d\t\t\t\tcap3\tpublic\n' "$n" "$((10 + n))"
    done
  } > .awiki/maps/actions.tsv

  # Capture stdout and stderr separately.
  stdout=$(bash scripts/action-recur.sh content/projects/cap3.md 2>/dev/null || true)
  stderr=$(bash scripts/action-recur.sh content/projects/cap3.md 2>&1 >/dev/null || true)
  ! echo "$stdout" | grep -q "refusing to emit"
  echo "$stderr" | grep -q  "refusing to emit"
}
