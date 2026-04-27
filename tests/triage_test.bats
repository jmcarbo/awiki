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
