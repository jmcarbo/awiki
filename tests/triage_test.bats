#!/usr/bin/env bats

# Tests for scripts/triage.sh — phase 18a tasks 18a.7..18a.11.

setup() {
  # Ensure GNU flock is on PATH on macOS.
  if [[ -x /opt/homebrew/opt/util-linux/bin/flock ]]; then
    export PATH="/opt/homebrew/opt/util-linux/bin:$PATH"
  fi

  TMP="$(mktemp -d)"
  export AWIKI_REPO_ROOT="$TMP"
  # Tests reference .awiki/log directly; route the log-append.sh helper there.
  export AWIKI_LOG_FILE="$TMP/.awiki/log"

  mkdir -p "$TMP/.awiki/maps" "$TMP/content/projects" "$TMP/content/contexts" \
           "$TMP/content/inbox" "$TMP/raw/inbox/interactive" "$TMP/raw/processed" \
           "$TMP/scripts/lib"
  cp -r "$BATS_TEST_DIRNAME/../scripts/." "$TMP/scripts/"
  echo "0" > "$TMP/.awiki/task-count"
  date -u +%Y-%m-%dT%H:%M:%SZ > "$TMP/.awiki/last-review"
  : > "$TMP/.awiki/log"
  cat > "$TMP/content/inbox.md" <<'EOF'
---
title: "Inbox"
type: inbox
draft: true
---

- 2026-04-27 14:32 call dentist about crown
- 2026-04-27 14:33 idea: rewrite onboarding email
EOF
  cat > "$TMP/content/contexts/phone.md" <<'EOF'
---
title: "@phone"
type: context
aliases: ['@phone']
draft: false
---
EOF
  cat > "$TMP/content/contexts/computer.md" <<'EOF'
---
title: "@computer"
type: context
aliases: ['@computer']
draft: false
---
EOF
  cd "$TMP"
}

teardown() {
  cd /
  rm -rf "$TMP"
}

# Helper: capture the synthesized id for an inbox line at lineno N.
inbox_id_for_lineno() {
  local n="$1"
  local line
  line="$(sed -n "${n}p" content/inbox.md)"
  local sha
  sha="$(printf '%s' "$line" | sha1sum | awk '{print substr($1,1,10)}')"
  printf 'inbox-%s-%d' "$sha" "$n"
}

@test "triage.sh --help prints usage and exits 0" {
  run bash scripts/triage.sh --help
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "Usage:"
  echo "$output" | grep -q "outcomes:"
}

@test "triage.sh rejects unknown outcome with exit 2" {
  run bash scripts/triage.sh inbox-aaaaaaaaaa-7 banana
  [ "$status" -eq 2 ]
  echo "$output" | grep -q "unknown outcome"
}

@test "triage.sh acquires flock and serializes against a held lock" {
  # Hold the lock in a background subshell, then attempt triage with a 1s timeout.
  (
    flock -x -w 5 "$TMP/.awiki/lock" -c 'sleep 3'
  ) &
  bg_pid=$!
  sleep 0.2  # let the background grab it
  AWIKI_LOCK_TIMEOUT_USER=1 run bash scripts/triage.sh \
    inbox-deadbeef00-7 trash lineno=7
  # Either exit 7 (lock contention) or exit 9 (stale id). The lock-contention path is
  # what we want; if the script returns exit 9, the lock branch was never reached.
  [ "$status" -eq 7 ]
  wait $bg_pid 2>/dev/null || true
}

@test "triage trash: removes inbox line via strikethrough, logs" {
  local id; id="$(inbox_id_for_lineno 7)"   # "- 2026-04-27 14:32 call dentist..."
  run bash scripts/triage.sh "$id" trash lineno=7
  [ "$status" -eq 0 ]
  # Original line replaced with strikethrough form (~~...~~) per spec table.
  grep -q '^~~- 2026-04-27 14:32 call dentist about crown~~$' content/inbox.md
  grep -q 'triage | trash' .awiki/log
}

@test "triage do-now: appends [x] to chosen project, removes inbox line" {
  local id; id="$(inbox_id_for_lineno 7)"
  # Pre-create a project page so we can check append (act test will exercise lazy create).
  cat > content/projects/dentist.md <<'EOF'
---
title: "Dentist"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF
  run bash scripts/triage.sh "$id" do-now project_slug=dentist context_slug=phone lineno=7
  [ "$status" -eq 0 ]
  grep -qE '^- \[x\] call dentist about crown @phone done:[0-9]{4}-[0-9]{2}-[0-9]{2} \^[a-z0-9]{8}$' content/projects/dentist.md
  ! grep -q 'call dentist about crown' content/inbox.md
  grep -q 'triage | do-now' .awiki/log
}

@test "triage act with project_slug=_loose lazily creates _loose.md" {
  local id; id="$(inbox_id_for_lineno 7)"
  [ ! -f content/projects/_loose.md ]
  run bash scripts/triage.sh "$id" act project_slug=_loose context_slug=phone lineno=7
  [ "$status" -eq 0 ]
  [ -f content/projects/_loose.md ]
  grep -q '^type: project$'  content/projects/_loose.md
  grep -q '^status: active$' content/projects/_loose.md
  grep -qE '^- \[ \] call dentist about crown @phone \^[a-z0-9]{8}$' content/projects/_loose.md
  ! grep -q 'call dentist about crown' content/inbox.md
}

@test "triage someday with project_slug=_someday lazily creates _someday.md" {
  local id; id="$(inbox_id_for_lineno 7)"
  [ ! -f content/projects/_someday.md ]
  run bash scripts/triage.sh "$id" someday project_slug=_someday lineno=7
  [ "$status" -eq 0 ]
  [ -f content/projects/_someday.md ]
  grep -q '^type: project$'   content/projects/_someday.md
  grep -q '^status: someday$' content/projects/_someday.md
  grep -qE '^- \[>\] call dentist about crown \^[a-z0-9]{8}$' content/projects/_someday.md
}

@test "triage defer-scheduled: appends [ ] with due:" {
  local id; id="$(inbox_id_for_lineno 7)"
  cat > content/projects/dentist.md <<'EOF'
---
title: "Dentist"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF
  run bash scripts/triage.sh "$id" defer-scheduled \
    project_slug=dentist context_slug=phone due=2026-05-15 lineno=7
  [ "$status" -eq 0 ]
  grep -qE '^- \[ \] call dentist about crown @phone due:2026-05-15 \^[a-z0-9]{8}$' content/projects/dentist.md
}

@test "triage defer-scheduled: rejects bad date with exit 4" {
  local id; id="$(inbox_id_for_lineno 7)"
  run bash scripts/triage.sh "$id" defer-scheduled \
    project_slug=dentist due=2026-02-30 lineno=7
  [ "$status" -eq 4 ]
  echo "$output" | grep -q "bad date"
}

@test "triage waiting: requires wait_for and stamps since:" {
  local id; id="$(inbox_id_for_lineno 7)"
  cat > content/projects/q3.md <<'EOF'
---
title: "Q3"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF
  today="$(date -u +%Y-%m-%d)"
  run bash scripts/triage.sh "$id" waiting \
    project_slug=q3 wait_for=bob-smith lineno=7
  [ "$status" -eq 0 ]
  grep -qE "^- \[\?\] call dentist about crown wait:\[\[bob-smith\]\] since:${today} \^[a-z0-9]{8}$" content/projects/q3.md
}

@test "triage reference: writes new page under content/<page_type>s/" {
  local id; id="$(inbox_id_for_lineno 8)"   # "idea: rewrite onboarding email"
  run bash scripts/triage.sh "$id" reference \
    page_type=concept ref_slug=onboarding-email-rewrite lineno=8
  [ "$status" -eq 0 ]
  [ -f content/concepts/onboarding-email-rewrite.md ]
  grep -q '^type: concept$' content/concepts/onboarding-email-rewrite.md
  ! grep -q 'idea: rewrite onboarding email' content/inbox.md
}

@test "triage rejects bad project_slug with exit 4" {
  local id; id="$(inbox_id_for_lineno 7)"
  run bash scripts/triage.sh "$id" act project_slug='../etc/passwd' lineno=7
  [ "$status" -eq 4 ]
}

@test "triage rejects stale inbox-line id with exit 9 + structured stdout" {
  local id; id="$(inbox_id_for_lineno 7)"
  # Mutate the inbox line so the recomputed sha differs.
  sed -i.bak '7s/.*/- 2026-04-27 14:32 something else entirely/' content/inbox.md
  rm -f content/inbox.md.bak

  run bash scripts/triage.sh "$id" act project_slug=_loose context_slug=phone lineno=7
  [ "$status" -eq 9 ]
  echo "$output" | grep -q '"stale_id":true'
  echo "$output" | grep -q '"ok":false'
  # The mutated inbox line is NOT removed.
  grep -q 'something else entirely' content/inbox.md
}

@test "triage rejects when inbox lineno no longer exists (file shorter)" {
  local id; id="$(inbox_id_for_lineno 7)"
  # Truncate the inbox so line 7 is gone.
  sed -i.bak '7d' content/inbox.md
  rm -f content/inbox.md.bak
  run bash scripts/triage.sh "$id" trash lineno=7
  [ "$status" -eq 9 ]
  echo "$output" | grep -q '"stale_id":true'
}

@test "triage accepts when inbox line is unchanged" {
  local id; id="$(inbox_id_for_lineno 7)"
  run bash scripts/triage.sh "$id" trash lineno=7
  [ "$status" -eq 0 ]
  grep -q '^~~- 2026-04-27 14:32 call dentist about crown~~$' content/inbox.md
}

@test "triage stale-id does NOT emit TRIAGE-RESULT trailer" {
  local id; id="$(inbox_id_for_lineno 7)"
  sed -i.bak '7s/.*/- 2026-04-27 14:32 something else entirely/' content/inbox.md
  rm -f content/inbox.md.bak
  run bash scripts/triage.sh "$id" act project_slug=_loose context_slug=phone lineno=7
  [ "$status" -eq 9 ]
  ! echo "$output" | grep -q 'TRIAGE-RESULT|'
}

@test "triage emits TRIAGE-RESULT trailer with structured payload" {
  local id; id="$(inbox_id_for_lineno 7)"
  run bash scripts/triage.sh "$id" act lineno=7 project_slug=renovate-kitchen context_slug=phone
  [ "$status" -eq 0 ]
  # Last non-blank line is the trailer.
  local trailer
  trailer="$(printf '%s\n' "$output" | awk 'NF{last=$0} END{print last}')"
  [[ "$trailer" == TRIAGE-RESULT\|* ]]
  # Strip prefix and parse.
  local json="${trailer#TRIAGE-RESULT|}"
  echo "$json" | jq -e '.actions_taken | type == "array"' >/dev/null
  echo "$json" | jq -e '.created_pages | type == "array"' >/dev/null
  echo "$json" | jq -e '.updated_pages | type == "array"' >/dev/null
}

@test "triage --interactive walks inbox one item at a time and applies outcomes" {
  command -v expect >/dev/null 2>&1 || skip "expect not installed"

  # Pre-create a project page so the walker can pick it.
  cat > content/projects/dentist.md <<'EOF'
---
title: "Dentist"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF

  # Drive the walker via expect: outcome=trash on line 7, then EOF on line 8.
  cat > /tmp/triage_drive.exp <<EOF
#!/usr/bin/env expect
set timeout 5
set env(AWIKI_REPO_ROOT) "$TMP"
set env(AWIKI_LOG_FILE) "$TMP/.awiki/log"
set env(PATH) "$PATH"
spawn bash scripts/triage.sh --interactive
expect "Outcome*"
send "trash\r"
expect -re "Outcome.*"
send -- "\x04"
expect eof
EOF
  run expect /tmp/triage_drive.exp
  [ "$status" -eq 0 ]
  grep -q '^~~- 2026-04-27 14:32 call dentist about crown~~$' content/inbox.md
  # Line 8 (idea: rewrite onboarding email) was NOT applied because we EOF'd.
  grep -q 'idea: rewrite onboarding email' content/inbox.md
}

@test "triage --interactive re-prompts on regex-invalid project_slug" {
  command -v expect >/dev/null 2>&1 || skip "expect not installed"
  cat > /tmp/triage_drive2.exp <<EOF
#!/usr/bin/env expect
set timeout 5
set env(AWIKI_REPO_ROOT) "$TMP"
set env(AWIKI_LOG_FILE) "$TMP/.awiki/log"
set env(PATH) "$PATH"
spawn bash scripts/triage.sh --interactive
expect "Outcome*"
send "act\r"
expect "Project slug*"
send "../etc/passwd\r"
expect {
  "Project slug*" { send "_loose\r" }
  timeout { exit 1 }
}
expect "Context slug*"
send "phone\r"
expect "Outcome*"
send -- "\x04"
expect eof
EOF
  run expect /tmp/triage_drive2.exp
  [ "$status" -eq 0 ]
  [ -f content/projects/_loose.md ]
}

@test "triage threshold lookup: env > config > default" {
  # Default = 5 with no env, no config.
  rm -f .awiki/config
  unset AWIKI_AGENDA_AFTER_N || true

  local id; id="$(inbox_id_for_lineno 7)"
  cat > content/projects/dentist.md <<'EOF'
---
title: "Dentist"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF
  echo "0" > .awiki/task-count

  # Apply 4 outcomes; agenda.sh should NOT run yet.
  for i in 1 2 3 4; do
    bash scripts/triage.sh "$id" trash lineno=7 || true
    # Re-seed line 7 because trash strikes it through.
    sed -i.bak '7s/.*/- 2026-04-27 14:32 call dentist about crown/' content/inbox.md
    rm -f content/inbox.md.bak
    id="$(inbox_id_for_lineno 7)"
  done
  ! grep -q 'agenda | rebuild' .awiki/log

  # 5th outcome triggers rebuild.
  bash scripts/triage.sh "$id" trash lineno=7
  grep -q 'agenda | rebuild' .awiki/log
  # Counter resets after rebuild.
  count=$(cat .awiki/task-count)
  [ "$count" -eq 0 ]
}

@test "triage threshold from .awiki/config overrides default" {
  printf 'AWIKI_AGENDA_AFTER_N=2\n' > .awiki/config
  unset AWIKI_AGENDA_AFTER_N || true
  echo "0" > .awiki/task-count

  local id; id="$(inbox_id_for_lineno 7)"
  bash scripts/triage.sh "$id" trash lineno=7
  ! grep -q 'agenda | rebuild' .awiki/log

  sed -i.bak '7s/.*/- 2026-04-27 14:32 call dentist about crown/' content/inbox.md
  rm -f content/inbox.md.bak
  id="$(inbox_id_for_lineno 7)"
  bash scripts/triage.sh "$id" trash lineno=7
  grep -q 'agenda | rebuild' .awiki/log
}

@test "triage threshold from env overrides .awiki/config" {
  printf 'AWIKI_AGENDA_AFTER_N=99\n' > .awiki/config
  export AWIKI_AGENDA_AFTER_N=1
  echo "0" > .awiki/task-count
  local id; id="$(inbox_id_for_lineno 7)"
  bash scripts/triage.sh "$id" trash lineno=7
  grep -q 'agenda | rebuild' .awiki/log
}

@test "triage threshold rebuild advances agenda last_updated" {
  printf 'AWIKI_AGENDA_AFTER_N=1\n' > .awiki/config
  unset AWIKI_AGENDA_AFTER_N || true
  echo "0" > .awiki/task-count
  # Seed an agenda page with a stale last_updated.
  mkdir -p content/agenda
  cat > content/agenda/next-actions.md <<'EOF'
---
title: "Next actions"
type: agenda
last_updated: 2025-01-01
draft: false
---
<!-- BEGIN agenda:next-actions -->
<!-- END agenda:next-actions -->
EOF
  printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
    > .awiki/maps/actions.tsv
  local id; id="$(inbox_id_for_lineno 7)"
  run bash scripts/triage.sh "$id" trash lineno=7
  [ "$status" -eq 0 ]
  today="$(date -u +%Y-%m-%d)"
  grep -q "^last_updated: ${today}$" content/agenda/next-actions.md
}

@test "triage --interactive ctrl-c aborts current item but keeps prior items applied" {
  command -v expect >/dev/null 2>&1 || skip "expect not installed"
  cat > /tmp/triage_drive3.exp <<EOF
#!/usr/bin/env expect
set timeout 5
set env(AWIKI_REPO_ROOT) "$TMP"
set env(AWIKI_LOG_FILE) "$TMP/.awiki/log"
set env(PATH) "$PATH"
spawn bash scripts/triage.sh --interactive
expect "Outcome*"
send "trash\r"
expect "Outcome*"
send "act\r"
expect "Project slug*"
send -- "\x03"
expect eof
EOF
  run expect /tmp/triage_drive3.exp
  # The first item should still be applied (strikethrough on line 7).
  grep -q '^~~- 2026-04-27 14:32 call dentist about crown~~$' content/inbox.md
  # The second item (line 8) was aborted -> still present, no _loose action.
  grep -q 'idea: rewrite onboarding email' content/inbox.md
  # If _loose.md was created, it MUST NOT contain the second item's text.
  if [ -f content/projects/_loose.md ]; then
    ! grep -q 'idea: rewrite onboarding email' content/projects/_loose.md
  fi
}
