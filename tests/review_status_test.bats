#!/usr/bin/env bats
# Phase-19 review-status.sh tests.
# Builds a sandboxed wiki tree with .awiki/maps/actions.tsv plus a
# fixture set of project pages, then asserts the structured REVIEW|...
# stdout shape.

setup() {
  if [[ -x /opt/homebrew/opt/util-linux/bin/flock ]]; then
    export PATH="/opt/homebrew/opt/util-linux/bin:$PATH"
  fi
  if ! command -v flock >/dev/null 2>&1; then
    skip "flock(1) not on PATH; install util-linux"
  fi

  REPO_ROOT_REAL="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  mkdir -p content/projects content/contexts content/agenda \
           raw/inbox/interactive .awiki/maps scripts/lib
  cp "$REPO_ROOT_REAL/scripts/lib/lock.sh"           scripts/lib/
  cp "$REPO_ROOT_REAL/scripts/lib/action-grammar.sh" scripts/lib/
  cp "$REPO_ROOT_REAL/scripts/review-status.sh"      scripts/ 2>/dev/null || true
  : > .awiki/lock
  printf '%s\n' "2026-04-20T09:00:00Z" > .awiki/last-review

  # Inbox with 3 unprocessed lines (under the frontmatter).
  cat > content/inbox.md <<'INBOX'
---
title: "Inbox"
type: inbox
draft: true
---

- 2026-04-26 09:00 call dentist
- 2026-04-26 09:01 buy cat food
- 2026-04-27 08:00 review draft proposal
INBOX

  # One file-shaped capture in raw/inbox/interactive/.
  printf "%s\n" "scratch note" > raw/inbox/interactive/note.md

  # actions.tsv fixture — header + 8 rows. Status is the bare marker
  # char (' ', '/', '?', '>', 'x'), matching what action-scan.sh emits.
  # Columns: id status text file line context due defer wait since every done priority est project source_kind
  printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
    > .awiki/maps/actions.tsv
  # a01: open + overdue (due 2026-04-25, today 2026-04-27)
  printf 'a01\t \tcall dentist\tcontent/projects/q3-launch.md\t5\tphone\t2026-04-25\t\t\t\t1w\t\t\t\tq3-launch\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a07: open + overdue (due 2026-04-26)
  printf 'a07\t \tsend invoice\tcontent/projects/onboarding-revamp.md\t7\tcomputer\t2026-04-26\t\t\t\t\t\t\t\tonboarding-revamp\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a02: in-progress (no due)
  printf 'a02\t/\tdraft proposal\tcontent/projects/q3-launch.md\t6\tcomputer\t\t\t\t\t\t\t\t\tq3-launch\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a03: waiting since 2026-04-08 (>14d before today 2026-04-27)
  printf 'a03\t?\tq3 budget approval\tcontent/projects/q3-launch.md\t8\t\t\t\tbob-smith\t2026-04-08\t\t\t3\t\tq3-launch\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a04: someday
  printf 'a04\t>\treorganize garage\tcontent/projects/_someday.md\t3\thome\t\t\t\t\t\t\t\t\t_someday\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a05: completed since last-review (done 2026-04-26 >= 2026-04-20)
  printf 'a05\tx\twater plants\tcontent/projects/home.md\t4\thome\t\t\t\t\t1w\t2026-04-26\t\t\thome\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a06: completed since last-review (done 2026-04-22)
  printf 'a06\tx\tfile taxes\tcontent/projects/admin.md\t5\tcomputer\t\t\t\t\t\t2026-04-22\t\t\tadmin\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a08: completed since last-review (done 2026-04-23)
  printf 'a08\tx\tbook flight\tcontent/projects/q3-launch.md\t9\tcomputer\t\t\t\t\t\t2026-04-23\t\t\tq3-launch\tpublic\n' \
    >> .awiki/maps/actions.tsv
  # a11/a12: open actions for home + admin so they don't become
  # no-next-action candidates (we're testing renovate-kitchen as the
  # canonical no-next-action project).
  printf 'a11\t \twater plants next week\tcontent/projects/home.md\t6\thome\t2026-05-03\t\t\t\t1w\t\t\t\thome\tpublic\n' \
    >> .awiki/maps/actions.tsv
  printf 'a12\t \tpay credit card\tcontent/projects/admin.md\t6\tcomputer\t2026-05-15\t\t\t\t1m\t\t\t\tadmin\tpublic\n' \
    >> .awiki/maps/actions.tsv

  # renovate-kitchen: active, no open actions in actions.tsv → no-next-action.
  cat > content/projects/renovate-kitchen.md <<'PROJ'
---
title: "Renovate kitchen"
date: 2026-04-01
last_updated: 2026-04-26
type: project
status: active
outcome: "Kitchen renovated."
tags: [home]
draft: false
---

## Open Actions
PROJ

  # onboarding-revamp: active, last_updated > 14d ago, has 1 open
  # action (a07). With current logic this counts as stuck-projects
  # because last_updated is stale and no recent completed action exists.
  cat > content/projects/onboarding-revamp.md <<'PROJ'
---
title: "Onboarding Revamp"
date: 2026-03-01
last_updated: 2026-04-01
type: project
status: active
outcome: "New onboarding shipped."
tags: [work]
draft: false
---

## Open Actions
- [ ] send invoice @computer due:2026-04-26 ^a07
PROJ

  # q3-launch: active with open actions and recent completion.
  cat > content/projects/q3-launch.md <<'PROJ'
---
title: "Q3 Launch"
date: 2026-04-15
last_updated: 2026-04-26
type: project
status: active
outcome: "Q3 product launched."
tags: [work]
draft: false
---

## Open Actions
- [ ] call dentist @phone due:2026-04-25 ^a01
- [/] draft proposal @computer ^a02
- [?] q3 budget approval wait:[[bob-smith]] since:2026-04-08 priority:3 ^a03
- [x] book flight @computer done:2026-04-23 ^a08
PROJ

  # home: active project; needed so 'home' file-paths in actions.tsv
  # don't make us trip over a missing project file. Has a recent
  # completed action so not stuck or no-next.
  cat > content/projects/home.md <<'PROJ'
---
title: "Home"
date: 2026-01-01
last_updated: 2026-04-26
type: project
status: active
outcome: "Home stays organized."
tags: [home]
draft: false
---

## Open Actions
- [x] water plants @home done:2026-04-26 every:1w ^a05
PROJ

  # admin: active project, recent completion.
  cat > content/projects/admin.md <<'PROJ'
---
title: "Admin"
date: 2026-01-01
last_updated: 2026-04-22
type: project
status: active
outcome: "Admin stays current."
tags: []
draft: false
---

## Open Actions
- [x] file taxes @computer done:2026-04-22 ^a06
PROJ

  # _someday: 12 entries to assert someday-count.
  cat > content/projects/_someday.md <<'PROJ'
---
title: "Someday"
date: 2026-01-01
last_updated: 2026-04-15
type: project
status: someday
outcome: "Bucket of deferred ideas."
tags: []
draft: false
---

## Open Actions
PROJ
  for i in 01 02 03 04 05 06 07 08 09 10 11 12; do
    printf '%s\n' "- [>] item $i ^s$i" >> content/projects/_someday.md
  done
  # Pad someday count in actions.tsv (already has a04, add 11 more).
  for i in 02 03 04 05 06 07 08 09 10 11 12; do
    printf 's%s\t>\titem %s\tcontent/projects/_someday.md\t%d\t\t\t\t\t\t\t\t\t\t_someday\tpublic\n' \
      "$i" "$i" "$((10 + 10#$i))" >> .awiki/maps/actions.tsv
  done

  # Today is fixed for the test via env var read by review-status.sh.
  export AWIKI_TODAY="2026-04-27"
}

teardown() {
  cd /
  [[ -n "${TEST_REPO:-}" ]] && rm -rf "$(dirname "$TEST_REPO")"
}

@test "review-status emits all eight REVIEW lines + REVIEW-SUMMARY" {
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW\|inbox-unprocessed\|count=3$'
  echo "$output" | grep -qE '^REVIEW\|raw-inbox-files\|count=1$'
  echo "$output" | grep -qE '^REVIEW\|projects-no-next-action\|count=1\|slugs=renovate-kitchen$'
  echo "$output" | grep -qE '^REVIEW\|waiting-stale-14d\|count=1\|ids=a03$'
  echo "$output" | grep -qE '^REVIEW\|overdue\|count=2\|ids=a01,a07$'
  echo "$output" | grep -qE '^REVIEW\|completed-since-last-review\|count=3$'
  echo "$output" | grep -qE '^REVIEW\|stuck-projects\|count=1\|slugs=onboarding-revamp$'
  echo "$output" | grep -qE '^REVIEW\|someday-count\|count=12$'
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=2026-04-20\|attention=[0-9]+$'
}

@test "review-status attention = overdue + waiting-stale + no-next + stuck" {
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  # 2 overdue + 1 waiting-stale + 1 no-next + 1 stuck = 5
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=2026-04-20\|attention=5$'
}

@test "review-status exit 0 when actions.tsv has only a header" {
  printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
    > .awiki/maps/actions.tsv
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW\|overdue\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|completed-since-last-review\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|someday-count\|count=0$'
}

@test "review-status emits last-review=never when .awiki/last-review absent" {
  rm -f .awiki/last-review
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=never\|attention=[0-9]+$'
}

@test "review-status emits zero-count lines without slugs/ids when empty" {
  rm -f content/inbox.md
  rm -f raw/inbox/interactive/note.md
  printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
    > .awiki/maps/actions.tsv
  rm -f content/projects/onboarding-revamp.md content/projects/renovate-kitchen.md
  rm -f content/projects/q3-launch.md content/projects/home.md content/projects/admin.md
  rm -f content/projects/_someday.md
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW\|inbox-unprocessed\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|raw-inbox-files\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|projects-no-next-action\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|waiting-stale-14d\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|stuck-projects\|count=0$'
}

@test "review-status raw-inbox-files ignores dotfiles" {
  : > raw/inbox/interactive/.hidden
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  # Still 1 (note.md), not 2.
  echo "$output" | grep -qE '^REVIEW\|raw-inbox-files\|count=1$'
}

@test "review-status overdue excludes future-dated [ ] actions" {
  # Add a future-due action; should not be in overdue.
  printf 'a09\t \tlater\tcontent/projects/q3-launch.md\t10\tcomputer\t2026-05-30\t\t\t\t\t\t\t\tq3-launch\tpublic\n' \
    >> .awiki/maps/actions.tsv
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  # Overdue stays at 2.
  echo "$output" | grep -qE '^REVIEW\|overdue\|count=2\|ids=a01,a07$'
}

@test "review-status waiting-stale excludes recent waiting (<14d since)" {
  # Add a [?] with since just 5 days ago; should not be stale.
  printf 'a10\t?\trecent waiting\tcontent/projects/q3-launch.md\t11\t\t\t\talice\t2026-04-22\t\t\t\t\tq3-launch\tpublic\n' \
    >> .awiki/maps/actions.tsv
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW\|waiting-stale-14d\|count=1\|ids=a03$'
}
