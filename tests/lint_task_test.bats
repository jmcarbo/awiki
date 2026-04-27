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
  # --fix may exit non-zero (e.g., T8 warning about active-but-empty pages).
  # Use `run` so the test only validates the file-content change.
  run env AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  run grep -F 'due:2026-04-27' "$WORK/content/projects/page.md"
  [ -n "$output" ]
}

@test "lint --fix is idempotent (second run is no-op)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run env AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  cp -R "$WORK/content" "$WORK/content_run1"
  run env AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
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
  # --fix may exit non-zero (T8 warning until ^id is minted and scanner reruns).
  run env AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  run grep -E '\^[a-z0-9]{8}' "$WORK/content/projects/p.md"
  [ -n "$output" ]
}

# --- T8: no-next-action (warn) -------------------------------------------

@test "T8 fires on active project with no open actions" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  cat > "$WORK/content/projects/idle.md" <<'EOF'
---
title: "Idle"
type: project
status: active
draft: false
---

## Open Actions

(none)

## Done

- [x] something old @computer done:2025-12-01 ^old1
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  [[ "$output" == *"T8:"* ]]
  [[ "$output" == *"content/projects/idle.md"* ]]
}

@test "T8 silent on _loose.md (catch-all exemption)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  cat > "$WORK/content/projects/_loose.md" <<'EOF'
---
title: "Loose"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'content/projects/_loose.md|T8' <<< "$output"
}

@test "T8 silent on _someday.md (catch-all exemption)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  cat > "$WORK/content/projects/_someday.md" <<'EOF'
---
title: "Someday"
type: project
status: someday
draft: false
---

## Open Actions
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'content/projects/_someday.md|T8' <<< "$output"
}

@test "T8 silent on status: someday and status: done projects" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  cat > "$WORK/content/projects/dormant.md" <<'EOF'
---
title: "Dormant"
type: project
status: someday
draft: false
---
EOF
  cat > "$WORK/content/projects/finished.md" <<'EOF'
---
title: "Finished"
type: project
status: done
draft: false
---
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -E 'content/projects/(dormant|finished)\.md\|T8' <<< "$output"
}

@test "T8 in-progress [/] counts as an open action" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  cat > "$WORK/content/projects/working.md" <<'EOF'
---
title: "Working"
type: project
status: active
draft: false
---

## Open Actions

- [/] in progress @computer ^w01
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'content/projects/working.md|T8' <<< "$output"
}

@test "T8 silent on good fixture (every active project has open actions)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T8:' <<< "$output"
}

# Helper: produce a YYYY-MM-DD that is N days before today, GNU+BSD portable.
_n_days_ago() {
  local n="$1"
  date -u -d "-${n} days" +%Y-%m-%d 2>/dev/null \
    || date -u -j -v-"${n}"d +%Y-%m-%d
}

# --- T9: waiting-stale (warn, since: > 14d ago) --------------------------

@test "T9 fires when [?] since: > 14d ago" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local stale; stale="$(_n_days_ago 30)"
  cat > "$WORK/content/projects/q3-wait.md" <<EOF
---
title: "Q3 wait"
type: project
status: active
draft: false
---

## Open Actions

- [?] q3 budget approval wait:[[bob-smith]] since:${stale} ^w03
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  [[ "$output" == *"T9:"* ]]
  [[ "$output" == *"content/projects/q3-wait.md"* ]]
}

@test "T9 silent when [?] since: <= 14d ago" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local fresh; fresh="$(_n_days_ago 7)"
  cat > "$WORK/content/projects/q3-wait.md" <<EOF
---
title: "Q3 wait"
type: project
status: active
draft: false
---

## Open Actions

- [?] q3 budget approval wait:[[bob-smith]] since:${fresh} ^w03
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T9:' <<< "$output"
}

@test "T9 boundary: exactly 14d is silent, exactly 15d is warn" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local d14; d14="$(_n_days_ago 14)"
  local d15; d15="$(_n_days_ago 15)"
  cat > "$WORK/content/projects/edge.md" <<EOF
---
title: "Edge"
type: project
status: active
draft: false
---

## Open Actions

- [?] item14 wait:[[bob-smith]] since:${d14} ^edge14
- [?] item15 wait:[[bob-smith]] since:${d15} ^edge15
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  [[ "$output" == *"T9:"* ]]
  [[ "$output" == *"edge15"* ]]
  ! grep -E 'T9:.*edge14' <<< "$output"
}

# --- T10: overdue ([ ]/[/] with due: < today) ----------------------------

@test "T10 fires on [ ] with due in the past" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local past; past="$(_n_days_ago 3)"
  cat > "$WORK/content/projects/late.md" <<EOF
---
title: "Late"
type: project
status: active
draft: false
---

## Open Actions

- [ ] file taxes @computer due:${past} ^t01
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  [[ "$output" == *"T10:"* ]]
  [[ "$output" == *"content/projects/late.md"* ]]
}

@test "T10 fires on [/] with due in the past" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local past; past="$(_n_days_ago 1)"
  cat > "$WORK/content/projects/late2.md" <<EOF
---
title: "Late2"
type: project
status: active
draft: false
---

## Open Actions

- [/] in flight @computer due:${past} ^t02
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  [[ "$output" == *"T10:"* ]]
  [[ "$output" == *"content/projects/late2.md"* ]]
}

@test "T10 silent on [ ] with due today" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local today; today="$(date -u +%Y-%m-%d)"
  cat > "$WORK/content/projects/duetoday.md" <<EOF
---
title: "Today"
type: project
status: active
draft: false
---

## Open Actions

- [ ] something @computer due:${today} ^t03
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T10:' <<< "$output"
}

@test "T10 silent on [x] with due in the past (completed)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local past; past="$(_n_days_ago 30)"
  cat > "$WORK/content/projects/done-old.md" <<EOF
---
title: "Done old"
type: project
status: active
draft: false
---

## Open Actions

- [ ] keep alive @home ^t04open

## Done

- [x] old @computer due:${past} done:${past} ^t04
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T10:' <<< "$output"
}

# --- T11: stale-someday (warn, page last_updated > 90d) ------------------

@test "T11 fires on [>] when enclosing page last_updated > 90d ago" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local stale; stale="$(_n_days_ago 100)"
  cat > "$WORK/content/projects/dusty.md" <<EOF
---
title: "Dusty"
type: project
status: active
last_updated: ${stale}
draft: false
---

## Open Actions

- [ ] keep dusty active @home ^dustopen
- [>] reorganize garage someday @home ^s01
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  [[ "$output" == *"T11:"* ]]
  [[ "$output" == *"content/projects/dusty.md"* ]]
}

@test "T11 silent on [>] when enclosing page last_updated <= 90d ago" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local fresh; fresh="$(_n_days_ago 30)"
  cat > "$WORK/content/projects/recent.md" <<EOF
---
title: "Recent"
type: project
status: active
last_updated: ${fresh}
draft: false
---

## Open Actions

- [ ] keep recent active @home ^recopen
- [>] reorganize garage someday @home ^s02
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T11:' <<< "$output"
}

@test "T11 boundary: exactly 90d is silent, 91d is warn" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  local d90; d90="$(_n_days_ago 90)"
  local d91; d91="$(_n_days_ago 91)"
  cat > "$WORK/content/projects/p90.md" <<EOF
---
title: "P90"
type: project
status: active
last_updated: ${d90}
draft: false
---

- [ ] keep p90 alive @home ^p90keep
- [>] item @home ^p90a
EOF
  cat > "$WORK/content/projects/p91.md" <<EOF
---
title: "P91"
type: project
status: active
last_updated: ${d91}
draft: false
---

- [ ] keep p91 alive @home ^p91keep
- [>] item @home ^p91a
EOF
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -E 'content/projects/p90\.md\|T11' <<< "$output"
  grep -E 'content/projects/p91\.md\|T11' <<< "$output"
}
