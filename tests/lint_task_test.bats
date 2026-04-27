#!/usr/bin/env bats
# Phase-17 Task 17.6: Lint task-rules T1-T7, T14, T15 + --fix mechanical.

setup() {
  WORK="$(mktemp -d)"
  cd "$WORK"
}

teardown() {
  cd /
  [ -n "${WORK:-}" ] && rm -rf "$WORK"
}

run_lint() {
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task "$@"
}

# --- Good fixture: every T-rule stays silent. ---

@test "good fixture produces no T-rule errors" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T1:' <<< "$output"
  ! grep -F 'T3:' <<< "$output"
  ! grep -F 'T4:' <<< "$output"
  ! grep -F 'T5:' <<< "$output"
  ! grep -F 'T7:' <<< "$output"
  ! grep -F 'T14:' <<< "$output"
}

# --- Per-rule fires on broken fixture. ---

@test "T1 fires on bad-status line" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T1:"* ]]
}

@test "T2 fires on duplicate ^id within a single page" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T2:"* ]]
  [[ "$output" == *"dupid"* ]]
}

@test "T2 fires on cross-page duplicate chain instance ^a05~2" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T2:"* ]]
  [[ "$output" == *"a05~2"* ]]
}

@test "T2 stays silent on chain head + chain instance (a05 + a05~2 different pages)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T2:' <<< "$output"
}

@test "T3 fires on bogus tail key" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T3:"* ]]
}

@test "T4 fires on bad date" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T4:"* ]]
}

@test "T5 fires on [?] without wait:" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T5:"* ]]
}

@test "T6 fires on @nonexistent context" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T6:"* ]]
  [[ "$output" == *"nonexistent"* ]]
}

@test "T6 stays silent on prose @mention and code-block content in good fixture" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  # No T6 should fire on the good fixture (q3-launch.md has prose @computer
  # mention on line 9 and a code-block "- [ ] not a real task" — neither
  # should trigger T6).
  ! grep -F 'T6:' <<< "$output"
}

@test "T7 fires on hand-edited managed region" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  cd "$WORK" && git init -q && git add -A && git commit -q -m init
  # Re-edit the file post-commit to simulate a hand-edit.
  printf '\nadditional hand edit\n' >> content/projects/managed-region-edited.md
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T7:"* ]]
}

@test "T14 fires on too-short id" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T14:"* ]]
}

@test "T14 fires on ~ outside chain shape" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T14:"* ]]
  [[ "$output" == *"abc~zz"* ]]
}

@test "T15 warns on indented continuation but does NOT auto-fix" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T15:"* ]]
  # --fix must NOT touch a file that has a wrapped continuation.
  before="$(cat "$WORK/content/projects/broken.md")"
  # Lint --fix may exit non-zero because broken.md still has T1/T4/T5 errors
  # (they are not auto-fixable). Use `run` so the non-zero exit doesn't kill
  # the test — we only care about the file-content invariant.
  run env AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  after="$(cat "$WORK/content/projects/broken.md")"
  [ "$before" = "$after" ]
}

# --- --fix idempotence + correctness ---

@test "lint --fix normalizes 2026/4/27 to 2026-04-27" {
  mkdir -p "$WORK/content/projects" "$WORK/content/contexts"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/.awiki" "$WORK/.awiki"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/content/contexts/." "$WORK/content/contexts/"
  : > "$WORK/.gitattributes"
  cat > "$WORK/content/projects/page.md" <<'EOM'
---
title: x
type: project
---
- [ ] thing @phone due:2026/4/27 ^ax01
EOM
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  run grep -F 'due:2026-04-27' "$WORK/content/projects/page.md"
  [ -n "$output" ]
}

@test "lint --fix is idempotent (second run is no-op)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  cp -R "$WORK/content" "$WORK/content_run1"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  diff -r "$WORK/content_run1" "$WORK/content"
}

@test "lint --fix mints missing ^id on bare action line" {
  mkdir -p "$WORK/content/projects" "$WORK/content/contexts"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/.awiki" "$WORK/.awiki"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/content/contexts/." "$WORK/content/contexts/"
  cat > "$WORK/content/projects/p.md" <<'EOM'
---
title: x
type: project
status: active
last_updated: 2026-04-27
---
- [ ] no id yet @phone
EOM
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  run grep -E '\^[a-z0-9]{8}' "$WORK/content/projects/p.md"
  [ -n "$output" ]
}
