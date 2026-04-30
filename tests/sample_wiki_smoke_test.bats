#!/usr/bin/env bats

# Phase-19 sample-wiki end-to-end smoke. Pins the README's 6-step manual
# task-layer walkthrough as an automated test. Every test runs against a
# disposable copy of examples/sample-wiki/ so the upstream sample state
# is never perturbed.

setup() {
  REPO="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP="$(mktemp -d)"
  cp -R "$REPO/examples/sample-wiki" "$TMP/wiki"
  # Carry script + lib into the clone (sample-wiki is pure content; the
  # scripts live in $REPO).
  mkdir -p "$TMP/wiki/scripts"
  cp -R "$REPO/scripts/." "$TMP/wiki/scripts/"
  AWIKI_BIN="$REPO/bin/awiki"
  cd "$TMP/wiki"
  git init -q
  git add -A >/dev/null 2>&1
  git -c user.email=test@test.local -c user.name=test commit -q -m seed
  export AWIKI_TODAY="2026-04-27"
  export AWIKI_REPO_ROOT="$TMP/wiki"
  # macOS-friendly flock shim.
  if [[ -x /opt/homebrew/opt/util-linux/bin/flock ]]; then
    PATH="/opt/homebrew/opt/util-linux/bin:$PATH"
    export PATH
  fi
}

teardown() {
  cd /
  [ -n "${TMP:-}" ] && rm -rf "$TMP"
}

@test "sample-wiki: re-running task-init is a no-op (sample already initialized)" {
  # The committed sample-wiki ships the populated layout. A second
  # task-init pass should detect every artefact and skip.
  run env AWIKI_TASK_INIT_ASSUME_NO=1 "$AWIKI_BIN" task-init
  [ "$status" -eq 0 ]
  # Inbox content survives.
  run grep -F 'reread chapter 3' content/inbox.md
  [ "$status" -eq 0 ]
  # Curated project survives.
  run grep -F 'Renovate kitchen' content/projects/renovate-kitchen.md
  [ "$status" -eq 0 ]
}

@test "sample-wiki: capture appends an inbox line" {
  before="$(wc -l < content/inbox.md | tr -d ' ')"
  run "$AWIKI_BIN" capture -- "test capture from smoke"
  [ "$status" -eq 0 ]
  after="$(wc -l < content/inbox.md | tr -d ' ')"
  [ "$((after - before))" -eq 1 ]
  run grep -F "test capture from smoke" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "sample-wiki: triage walk via triage.sh act lands action in _loose" {
  "$AWIKI_BIN" capture -- "call dentist about crown" >/dev/null
  # Locate the captured line and synthesize its inbox-<sha>-<lineno> id.
  local lineno raw sha id
  lineno="$(awk '/^- /{n=NR} END{print n}' content/inbox.md)"
  [ -n "$lineno" ]
  raw="$(awk -v n="$lineno" 'NR==n' content/inbox.md)"
  sha="$(printf '%s' "$raw" | shasum | awk '{print substr($1,1,10)}')"
  id="inbox-${sha}-${lineno}"

  run "$AWIKI_BIN" triage-apply "$id" act project_slug=_loose context_slug=phone "lineno=${lineno}"
  [ "$status" -eq 0 ]
  run grep -F 'call dentist about crown @phone' content/projects/_loose.md
  [ "$status" -eq 0 ]
  # Inbox no longer contains the captured line.
  run grep -F 'call dentist about crown' content/inbox.md
  [ "$status" -ne 0 ]
}

@test "sample-wiki: agenda regenerates managed regions and surfaces sample actions" {
  run "$AWIKI_BIN" scan
  [ "$status" -eq 0 ]
  run "$AWIKI_BIN" agenda
  [ "$status" -eq 0 ]
  run grep -F '<!-- BEGIN agenda:next-actions -->' content/agenda/next-actions.md
  [ "$status" -eq 0 ]
  run grep -F '<!-- END agenda:next-actions -->' content/agenda/next-actions.md
  [ "$status" -eq 0 ]
  # Renovate-kitchen + q3-launch actions are surfaced under their @context.
  run grep -F 'schedule contractor walk-through' content/agenda/next-actions.md
  [ "$status" -eq 0 ]
  run grep -F 'draft launch announcement' content/agenda/next-actions.md
  [ "$status" -eq 0 ]
  # Waiting is grouped by wait person — single (not double) wikilink wrap.
  run grep -F 'wait:[[bob-smith]]' content/agenda/waiting.md
  [ "$status" -eq 0 ]
}

@test "sample-wiki: review chain runs to completion (agenda -> lint -> review-status)" {
  # Inline equivalent of `just review`: scan + agenda regen + lint
  # (best-effort; pre-existing synth lint errors in memex-briefing.md do
  # not block the review chain) + structured review report.
  "$AWIKI_BIN" scan
  "$AWIKI_BIN" agenda
  bash scripts/lint.sh >/dev/null 2>&1 || true
  run "$AWIKI_BIN" review-status
  [ "$status" -eq 0 ]
  # The eight REVIEW| lines + summary line must all be present.
  for k in inbox-unprocessed raw-inbox-files projects-no-next-action \
           waiting-stale-14d overdue completed-since-last-review \
           stuck-projects someday-count; do
    echo "$output" | grep -qE "^REVIEW\\|${k}\\|count=" || {
      echo "missing key: $k"
      return 1
    }
  done
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=[^|]+\|attention=[0-9]+$'
}
