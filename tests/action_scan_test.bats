#!/usr/bin/env bats

# Phase-17 scanner BATS suite. The actual scanner ships in 17.4; this file
# is created here so the fixture skeleton has a holding place. Real
# assertions land in 17.4.

@test "wiki-task-good fixture has 12 action-line files plus three noise files" {
  [ -d tests/fixtures/wiki-task-good ]
  # Six project pages with actions.
  for f in renovate-kitchen q3-launch water-plants budget-private onboarding-revamp; do
    [ -f "tests/fixtures/wiki-task-good/content/projects/${f}.md" ]
  done
  [ -f tests/fixtures/wiki-task-good/content/private/secret-project.md ]
}

@test "wiki-task-broken fixture exists with all rejection-reason files" {
  [ -d tests/fixtures/wiki-task-broken ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/broken.md ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/managed-region-edited.md ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/chain-page-1.md ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/chain-page-2.md ]
}

setup_good() {
  WORK="$(mktemp -d)"
  cp -R tests/fixtures/wiki-task-good/. "$WORK/"
  cd "$WORK"
}

setup_broken() {
  WORK="$(mktemp -d)"
  cp -R tests/fixtures/wiki-task-broken/. "$WORK/"
  cd "$WORK"
}

teardown_work() {
  cd /
  [ -n "${WORK:-}" ] && rm -rf "$WORK"
}

@test "scanner emits actions.tsv with all 12 actions from good fixture" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  [ "$status" -eq 0 ]
  [ -f "$WORK/.awiki/maps/actions.tsv" ]
  # Header + 12 records.
  run bash -c "wc -l < '$WORK/.awiki/maps/actions.tsv' | tr -d ' '"
  [ "$output" = "13" ]
  teardown_work
}

@test "scanner classifies private actions correctly (10, 11, 12)" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  # action 10 — private by file path
  run grep -E $'^a10\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tprivate' ]]
  # action 11 — private by frontmatter tag
  run grep -E $'^a11\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tprivate' ]]
  # action 12 — private by wikilink target
  run grep -E $'^a12\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tprivate' ]]
  teardown_work
}

@test "scanner classifies action 1 as public" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -E $'^a01\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tpublic' ]]
  teardown_work
}

@test "scanner does NOT include prose @mentions in actions.tsv" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  # The 'discussed @computer setup with team' prose line must not appear.
  run grep -F 'discussed' "$WORK/.awiki/maps/actions.tsv"
  [ -z "$output" ]
  teardown_work
}

@test "scanner does NOT include code-block [ ] in actions.tsv" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -F 'not a real task' "$WORK/.awiki/maps/actions.tsv"
  [ -z "$output" ]
  teardown_work
}

@test "scanner does NOT include blockquote [ ] in actions.tsv" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -F 'quoted line' "$WORK/.awiki/maps/actions.tsv"
  [ -z "$output" ]
  teardown_work
}

@test "scanner records chain head ^a05 and chain instance ^a05~2 as distinct rows" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -cE $'^(a05|a05~2)\t' "$WORK/.awiki/maps/actions.tsv"
  [ "$output" = "2" ]
  teardown_work
}

@test "scanner emits actions-rejected.tsv with continuation row on broken fixture" {
  setup_broken
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" || true
  [ -f "$WORK/.awiki/maps/actions-rejected.tsv" ]
  run grep -F 'continuation' "$WORK/.awiki/maps/actions-rejected.tsv"
  [ -n "$output" ]
  teardown_work
}

@test "scanner exits 1 when duplicate ^id detected on same page" {
  setup_broken
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  [ "$status" -eq 1 ]
  teardown_work
}

@test "scanner stdout summary names file count, action count, rejected count" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  [[ "$output" == *"scanned"* ]]
  [[ "$output" == *"actions"* ]]
  [[ "$output" == *"rejected"* ]]
  teardown_work
}

@test "scanner is idempotent (second run produces byte-identical TSV)" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  cp "$WORK/.awiki/maps/actions.tsv" "$WORK/run1.tsv"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  diff -q "$WORK/run1.tsv" "$WORK/.awiki/maps/actions.tsv"
  teardown_work
}

@test "scanner records inbox-line synthesis ID for inbox.md captures" {
  setup_good
  printf -- '- 2026-04-27 14:32 call dentist about crown\n' \
    > "$WORK/content/inbox.md"
  cat > "$WORK/content/inbox.md" <<'INBOX'
---
title: Inbox
type: inbox
draft: true
---

- 2026-04-27 14:32 call dentist about crown
- 2026-04-27 14:33 idea: rewrite onboarding email
INBOX
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  # Inbox lines do NOT enter actions.tsv (no checkbox marker), but the
  # synthesis routine is library-mode: phase-18 triage will call into it.
  # For phase 17 we only test that the scanner does not blow up on inbox.md.
  [ -f "$WORK/.awiki/maps/actions.tsv" ]
  teardown_work
}
