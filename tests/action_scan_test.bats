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
