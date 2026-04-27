#!/usr/bin/env bats
# Phase-17 agenda regen tests for scripts/agenda.sh.
# Each test builds a sandboxed wiki tree under $WORK with a minimal
# .awiki/maps/actions.tsv and five managed-region placeholder pages,
# then invokes scripts/agenda.sh against the sandbox.

setup_agenda() {
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/.awiki/maps" \
           "$WORK/content/agenda" \
           "$WORK/content/projects" \
           "$WORK/content/contexts"
  for region in next-actions today waiting someday stuck-projects; do
    cat > "$WORK/content/agenda/${region}.md" <<EOM
---
title: "${region}"
type: agenda
last_updated: 2025-01-01
draft: false
---

User notes above the managed region survive regen.

<!-- BEGIN agenda:${region} -->
<!-- END agenda:${region} -->

User notes below the managed region also survive regen.
EOM
  done
  # Minimal alias map (4 tab-separated columns: alias, slug, path, kind).
  printf '@phone\tphone\tcontent/contexts/phone.md\tpublic\n' \
    >  "$WORK/.awiki/maps/alias-to-slug.tsv"
  printf '@computer\tcomputer\tcontent/contexts/computer.md\tpublic\n' \
    >> "$WORK/.awiki/maps/alias-to-slug.tsv"
  printf '@home\thome\tcontent/contexts/home.md\tpublic\n' \
    >> "$WORK/.awiki/maps/alias-to-slug.tsv"

  # Build a minimal actions.tsv. Use printf with explicit \t so the
  # tab structure is unambiguous regardless of editor settings.
  local TSV="$WORK/.awiki/maps/actions.tsv"
  printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' > "$TSV"
  # a01 [ ] @phone, due:2026-05-01, project renovate-kitchen
  printf 'a01\t \tcall dentist about crown\tcontent/projects/renovate-kitchen.md\t11\t@phone\t2026-05-01\t\t\t\t\t\t\t\trenovate-kitchen\tpublic\n' >> "$TSV"
  # a02 [/] @computer, project q3-launch
  printf 'a02\t/\tdraft proposal\tcontent/projects/q3-launch.md\t11\t@computer\t\t\t\t\t\t\t\t\tq3-launch\tpublic\n' >> "$TSV"
  # a03 [?] waiting on bob-smith since 2026-04-22
  printf 'a03\t?\tq3 budget approval\tcontent/projects/q3-launch.md\t12\t\t\t\tbob-smith\t2026-04-22\t\t\t\t\tq3-launch\tpublic\n' >> "$TSV"
  # a04 [>] someday @home q3-launch
  printf 'a04\t>\treorganize garage someday\tcontent/projects/q3-launch.md\t13\t@home\t\t\t\t\t\t\t\t\tq3-launch\tpublic\n' >> "$TSV"
  # a07 [x] @computer done 2026-04-13 due 2026-04-15 q3-launch
  printf 'a07\tx\tfile taxes\tcontent/projects/q3-launch.md\t14\t@computer\t2026-04-15\t\t\t\t\t2026-04-13\t\t\tq3-launch\tpublic\n' >> "$TSV"
  # a10 [ ] sensitive call (PRIVATE source)
  printf 'a10\t \tsensitive call\tcontent/private/secret-project.md\t11\t@phone\t\t\t\t\t\t\t\t\tsecret-project\tprivate\n' >> "$TSV"
}

teardown_agenda() {
  cd /
  [ -n "${WORK:-}" ] && rm -rf "$WORK"
}

@test "agenda.sh emits next-actions content with @phone heading" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  [ "$status" -eq 0 ]
  run grep -F '### @phone' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  run grep -F 'call dentist about crown' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh respects the privacy filter (a10 not in next-actions)" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'sensitive call' "$WORK/content/agenda/next-actions.md"
  [ -z "$output" ]
  run grep -F 'action(s) hidden' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh waiting.md groups by wait person" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'bob-smith' "$WORK/content/agenda/waiting.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh someday.md lists [>] actions grouped by project" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'reorganize garage' "$WORK/content/agenda/someday.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh preserves user notes outside the managed region" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'User notes above the managed region' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  run grep -F 'User notes below the managed region' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh rewrites last_updated to today on every regen" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  today="$(date +%Y-%m-%d)"
  run grep -F "last_updated: $today" "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh uses atomic rename (no .tmp file leaks)" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  # No leftover temp files.
  run find "$WORK/content/agenda" -name '*.tmp.*'
  [ -z "$output" ]
  teardown_agenda
}

@test "agenda.sh idempotent (second run is byte-identical)" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  cp "$WORK/content/agenda/next-actions.md" "$WORK/run1.md"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  diff -q "$WORK/run1.md" "$WORK/content/agenda/next-actions.md"
  teardown_agenda
}

@test "agenda.sh exits 5 when AWIKI_AGENDA_INCLUDE_PRIVATE=1 without encryption coverage" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" AWIKI_AGENDA_INCLUDE_PRIVATE=1 \
    run bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  [ "$status" -eq 5 ]
  teardown_agenda
}

@test "phase-17 placeholder agenda pages carry per-region markers (sandbox)" {
  setup_agenda
  for region in next-actions today waiting someday stuck-projects; do
    f="$WORK/content/agenda/${region}.md"
    grep -qF "<!-- BEGIN agenda:${region} -->" "$f"
    grep -qF "<!-- END agenda:${region} -->" "$f"
  done
  teardown_agenda
}
