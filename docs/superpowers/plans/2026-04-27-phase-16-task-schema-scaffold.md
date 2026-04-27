# awiki Plan — Phase 16: Task Layer — Schema + Scaffold

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** v1 phases 1, 2, 3, 6 (skeleton, scripts core, hugo render, section indexes). MCP server (phase 8) is NOT a dependency for this phase — task-layer MCP tools land in phase 18.
**Previous:** v1 phase chain (this is the first task-layer phase, on top of v1).
**Next:** Phase 17 — scanner + agenda generation.

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, bats-core 1.10+, `flock` (util-linux on Linux; Homebrew `util-linux` keg-only or `flock` shim on macOS), git 2.30+, GNU coreutils (date, sha1sum/shasum). No new tool versions beyond v1 except the dep-check addition for `flock`.

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`. Pure bash 4+ for shell logic — no python helpers in this phase.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run-it-fails → implement → run-it-passes → commit. Each is its own checkbox step.
- Branch per phase. Merge to main only after `bats tests/ && just lint` are clean.
- All shell-out sites in `task-init.sh` and `capture.sh` use `--` to terminate flag parsing before positional arguments (mirrors phase 13 hardening).
- Slug regex `^[a-z0-9][a-z0-9-]*$` (no leading hyphen) for context / project slugs. Underscore-prefixed project slugs (`_loose`, `_someday`) are NOT created in this phase — they appear lazily in phase 18.
- Block-ID regex `^[a-z0-9]{3,16}$` (per spec T14); freshly minted IDs are exactly 8 chars. **Phase 16 does not mint IDs**; minting moves into phase 17 / 18 consumers.
- Marker syntax for the task-layer block in `WIKI.md` is `<!-- BEGIN task-layer -->` / `<!-- END task-layer -->` — never bare `BEGIN`/`END`. Managed-region markers for agenda pages are `<!-- BEGIN managed-region -->` / `<!-- END managed-region -->` (phase 17 fills the body; phase 16 only writes the empty pair).
- No emojis anywhere. No `Co-Authored-By` trailers in commits.
- All log lines via `bash scripts/log-append.sh -- <action> <message>` (the existing v1 helper). Phase 16 does not extend log-append.

**Phase 16 lib-only-this-phase split:** the action grammar library and the lock library ship in this phase along with **only** their unit tests. The consumers — scanner, agenda generator, lint task-rules, recur, triage — ship in phases 17 and 18. Do not attempt to wire them up here.

---

**Deliverable:** `scripts/task-init.sh` (per-step idempotent enable script); `scripts/lib/action-grammar.sh` (sourced library exposing one canonical action-line regex set + tail-metadata parser, with unit tests); `scripts/lib/lock.sh` (`flock` wrapper with 30s and 180s timeout profiles, exit-7 on contention); `scripts/check-deps.sh` extended to detect `flock` with OS-specific install hint; `scripts/capture.sh` with the full sanitization table from the spec; pre-templated `<!-- BEGIN task-layer -->` block in `WIKI.md` (page-kind enum extension, system-page entries, inline-grammar table; lint rules listed but only T1-T15 names — full bodies land in 17/18 plans); system pages created lazily by `task-init`: `content/inbox.md`, `content/projects/_index.md`, `content/contexts/_index.md`, `content/agenda/_index.md`, five managed-region agenda pages, `content/agenda/review-log.md`, three starter context pages (`phone`, `errands`, `computer`); `.awiki/config` extended with `AWIKI_AGENDA_AFTER_N=5` and `AWIKI_TASK_LAYER=on`; `.awiki/last-review` and `.awiki/task-count` initial state; encryption-coverage prompt in `task-init`; justfile recipes `task-init` + `capture` (other task-layer recipes added as commented stubs); BATS tests `tests/task_init_test.bats`, `tests/capture_test.bats`, `tests/action_grammar_test.bats`, `tests/lock_test.bats`; smoke test `just task-init` followed by `just capture "test"` writes the expected line to `content/inbox.md`.

**Branch:** `phase-16-task-schema-scaffold`

---

## Task 16.1: Branch + dependency check extension for `flock`

`flock` is a hard requirement for the lock library. We bolt the dep check on first because every later task that runs under `set -euo pipefail` will fail noisily if the binary is missing on the test box.

**Files:** Create: (none). Modify: `scripts/check-deps.sh`, `tests/check_deps_test.bats`.

- [ ] **Step 1: Create branch**

```bash
git checkout main
git pull --ff-only
git checkout -b phase-16-task-schema-scaffold
```

Expected: `Switched to a new branch 'phase-16-task-schema-scaffold'`.

- [ ] **Step 2: Append failing test for `flock` detection**

```bash
cat >> tests/check_deps_test.bats <<'EOF'

@test "check-deps reports flock when AWIKI_FAKE_MISSING=flock" {
  run env AWIKI_FAKE_MISSING=flock bash scripts/check-deps.sh
  [ "$status" -ne 0 ]
  [[ "$output" == *"flock"* ]]
}

@test "check-deps prints macOS install hint for flock" {
  run env AWIKI_FAKE_MISSING=flock bash scripts/check-deps.sh
  # Hint mentions util-linux on Linux OR flock shim/util-linux on macOS.
  [[ "$output" == *"util-linux"* || "$output" == *"flock"* ]]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL)**

```bash
bats tests/check_deps_test.bats
```

Expected: the two new tests fail (`flock` is not yet listed as a required dep; `AWIKI_FAKE_MISSING=flock` slips through).

- [ ] **Step 4: Add `flock` to the required-deps list**

Open `scripts/check-deps.sh` and find the block of `check <cmd> <min> <macos-hint> <linux-hint>` calls. Insert this line directly after the `check bats ...` line (keep the block alphabetised within "required" vs "optional"):

```bash
check flock "n/a" "brew install util-linux  # then add the flock binary to PATH (see brew info util-linux)" "apt install util-linux  # provides /usr/bin/flock"
```

Verify the file with:

```bash
grep -n 'check flock' scripts/check-deps.sh
```

Expected: one line printed, immediately after the `check bats` line.

- [ ] **Step 5: Run test (expect PASS)**

```bash
bats tests/check_deps_test.bats
```

Expected: every test in the file passes (including the prior phase-1 tests).

- [ ] **Step 6: Commit**

```bash
git add scripts/check-deps.sh tests/check_deps_test.bats
git commit -m "feat(deps): require flock for the task-layer concurrency model"
```

---

## Task 16.2: `scripts/lib/lock.sh` — flock wrapper with two timeout profiles

`scripts/lib/lock.sh` is sourced by every mutating script in phases 17/18 and by the MCP server's mutating handlers. Phase 16 ships **only** the library + its unit tests. No consumers wired in this phase.

The library exposes:
- `awiki_lock_with --timeout=<seconds> -- <command...>` — runs `<command>` under `flock -x .awiki/lock`. On contention, exits 7.
- `awiki_lock_shared --timeout=<seconds> -- <command...>` — same but `flock -s` (shared/read).
- `AWIKI_LOCK_TIMEOUT_USER=30` and `AWIKI_LOCK_TIMEOUT_DEFERRED=180` exported defaults.

The library does **not** mutate `.awiki/lock` ownership beyond the `flock` syscall. The lock file is touched (created with mode 0644) on first call and is gitignored (handled in task 16.6).

**Files:** Create: `scripts/lib/lock.sh`, `tests/lock_test.bats`.

- [ ] **Step 1: Create the `lib` directory**

```bash
mkdir -p scripts/lib
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/lock_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/.awiki"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "lock library defines awiki_lock_with and awiki_lock_shared" {
  run bash -c 'source scripts/lib/lock.sh && type -t awiki_lock_with && type -t awiki_lock_shared'
  [ "$status" -eq 0 ]
  [[ "$output" == *"function"* ]]
}

@test "awiki_lock_with creates .awiki/lock and runs command" {
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=5 -- echo HELLO'
  [ "$status" -eq 0 ]
  [[ "$output" == *"HELLO"* ]]
  [ -f .awiki/lock ]
}

@test "awiki_lock_with exits 7 on contention" {
  # Hold the lock in a background shell, then try to acquire.
  bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=10 -- sleep 3' &
  HOLDER_PID=$!
  sleep 0.3
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=1 -- echo SHOULD_NOT_RUN'
  [ "$status" -eq 7 ]
  [[ "$output" != *"SHOULD_NOT_RUN"* ]]
  wait "$HOLDER_PID" 2>/dev/null || true
}

@test "awiki_lock_shared allows concurrent shared readers" {
  bash -c 'source scripts/lib/lock.sh && awiki_lock_shared --timeout=5 -- sleep 2' &
  READER_PID=$!
  sleep 0.2
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_shared --timeout=2 -- echo SECOND_READER'
  [ "$status" -eq 0 ]
  [[ "$output" == *"SECOND_READER"* ]]
  wait "$READER_PID" 2>/dev/null || true
}

@test "awiki_lock_with rejects missing -- separator" {
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=5 echo nope'
  [ "$status" -ne 0 ]
  [[ "$output" == *"--"* || "$output" == *"separator"* ]]
}

@test "awiki_lock_with rejects non-numeric timeout" {
  run bash -c 'source scripts/lib/lock.sh && awiki_lock_with --timeout=abc -- echo x'
  [ "$status" -ne 0 ]
  [[ "$output" == *"timeout"* ]]
}

@test "exported defaults present" {
  run bash -c 'source scripts/lib/lock.sh && echo "$AWIKI_LOCK_TIMEOUT_USER|$AWIKI_LOCK_TIMEOUT_DEFERRED"'
  [ "$status" -eq 0 ]
  [[ "$output" == "30|180" ]]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL)**

```bash
bats tests/lock_test.bats
```

Expected: every test fails — `scripts/lib/lock.sh: No such file or directory`.

- [ ] **Step 4: Write `scripts/lib/lock.sh`**

```bash
cat > scripts/lib/lock.sh <<'EOF'
#!/usr/bin/env bash
# Source-only helper. Wraps flock around .awiki/lock with two timeout profiles.
# Defines:
#   awiki_lock_with    --timeout=<seconds> -- <command...>   (exclusive write lock)
#   awiki_lock_shared  --timeout=<seconds> -- <command...>   (shared read lock)
# On contention (timeout reached), exits 7. Rejects missing `--` separator
# and non-numeric timeouts (exit 1). Caller is responsible for set -e discipline.
#
# Exports:
#   AWIKI_LOCK_TIMEOUT_USER=30        (user-facing mutations)
#   AWIKI_LOCK_TIMEOUT_DEFERRED=180   (deferred agenda rebuild path)
#
# The lock file lives at .awiki/lock relative to the current working directory
# (which is the repo root in normal awiki usage). The directory must exist
# before sourcing — task-init.sh ensures this.

export AWIKI_LOCK_TIMEOUT_USER=30
export AWIKI_LOCK_TIMEOUT_DEFERRED=180

_awiki_lock_run() {
  # $1 = mode flag for flock (-x or -s); $2.. = parsed args.
  local mode="$1"; shift
  local timeout=""
  local saw_dashdash=0

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --timeout=*) timeout="${1#--timeout=}"; shift ;;
      --) saw_dashdash=1; shift; break ;;
      *) echo "ERROR: unexpected arg before '--': $1" >&2; return 1 ;;
    esac
  done

  if [[ "$saw_dashdash" -ne 1 ]]; then
    echo "ERROR: missing '--' separator before command" >&2
    return 1
  fi
  if [[ -z "$timeout" || ! "$timeout" =~ ^[0-9]+$ ]]; then
    echo "ERROR: invalid or missing --timeout=<seconds>" >&2
    return 1
  fi
  if [[ $# -eq 0 ]]; then
    echo "ERROR: no command provided after '--'" >&2
    return 1
  fi

  mkdir -p .awiki
  : > /dev/null  # explicit no-op; ensures shell state is sane
  if [[ ! -e .awiki/lock ]]; then
    : > .awiki/lock
  fi

  # flock -w <secs> -x|-s <fd>; opens the lock file on fd 9 via shell redirect.
  # Use a subshell so the redirect is cleaned up automatically.
  (
    flock -w "$timeout" "$mode" 9 || exit 7
    "$@"
  ) 9>>.awiki/lock
}

awiki_lock_with() {
  _awiki_lock_run -x "$@"
}

awiki_lock_shared() {
  _awiki_lock_run -s "$@"
}
EOF
chmod +x scripts/lib/lock.sh
```

- [ ] **Step 5: Run test (expect PASS)**

```bash
bats tests/lock_test.bats
```

Expected: 7 tests pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/lib/lock.sh tests/lock_test.bats
git commit -m "feat(task): add scripts/lib/lock.sh flock wrapper with 30s/180s profiles"
```

---

## Task 16.3: `scripts/lib/action-grammar.sh` — canonical regex + tail parser

The grammar library is sourced by phase-17 scanner / phase-17 lint task-rules / phase-18 recur / phase-18 triage. Phase 16 ships **only** the library + its unit tests. The library is intentionally minimal — it does not write to disk and does not parse files. It exposes:

- `AWIKI_ACTION_LINE_RE` — extended-regex string matching a canonical action line (use with `[[ "$line" =~ $AWIKI_ACTION_LINE_RE ]]`).
- `AWIKI_TAIL_KEY_RE` — extended-regex matching one `key:value` token (one of the eight allowed keys).
- `AWIKI_BLOCK_ID_RE` — extended-regex matching `^<id>` where `<id>` is `[a-z0-9]{3,16}` optionally followed by `~<digits>`.
- `AWIKI_STATUS_MARKERS` — bash array of valid status chars: `' ' '/' '?' '>' 'x' '-'`.
- `awiki_grammar_parse_action <line>` — pure-bash parser. On match, sets `AWIKI_AG_STATUS`, `AWIKI_AG_TEXT`, `AWIKI_AG_CONTEXT`, `AWIKI_AG_TAIL` (raw `key:value...` substring), `AWIKI_AG_ID` (block-id including the leading `^`, or empty); returns 0. On non-match, returns 1.
- `awiki_grammar_split_tail <tail-string>` — splits the tail into one `key=value` pair per line on stdout. Reject reasons (bad-key / bad-date / bad-format) are flagged via `AWIKI_AG_REJECT_REASON` if a key fails the per-key shape check; the function still emits whatever it can parse so the caller (lint, scanner) can decide to keep or reject.

**Files:** Create: `scripts/lib/action-grammar.sh`, `tests/action_grammar_test.bats`.

- [ ] **Step 1: Write the test**

```bash
cat > tests/action_grammar_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "library defines all advertised symbols" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && [[ -n "$AWIKI_ACTION_LINE_RE" ]] \
      && [[ -n "$AWIKI_TAIL_KEY_RE" ]] \
      && [[ -n "$AWIKI_BLOCK_ID_RE" ]] \
      && declare -p AWIKI_STATUS_MARKERS >/dev/null \
      && type -t awiki_grammar_parse_action \
      && type -t awiki_grammar_split_tail'
  [ "$status" -eq 0 ]
  [[ "$output" == *"function"* ]]
}

@test "parses minimal action line" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_parse_action "- [ ] call dentist ^a01" \
      && echo "$AWIKI_AG_STATUS|$AWIKI_AG_TEXT|$AWIKI_AG_CONTEXT|$AWIKI_AG_TAIL|$AWIKI_AG_ID"'
  [ "$status" -eq 0 ]
  [[ "$output" == " |call dentist||"*"|^a01" ]]
}

@test "parses action line with context, tail, and id" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_parse_action "- [ ] call dentist about crown @phone due:2026-05-01 ^a01" \
      && echo "$AWIKI_AG_STATUS|$AWIKI_AG_CONTEXT|$AWIKI_AG_TAIL|$AWIKI_AG_ID"'
  [ "$status" -eq 0 ]
  [[ "$output" == " |@phone|due:2026-05-01|^a01" ]]
}

@test "parses each valid status marker" {
  for s in " " "/" "?" ">" "x" "-"; do
    run bash -c "source scripts/lib/action-grammar.sh && awiki_grammar_parse_action '- [$s] x ^id1' && echo \"\$AWIKI_AG_STATUS\""
    [ "$status" -eq 0 ]
    [[ "$output" == "$s" ]]
  done
}

@test "rejects line without checkbox" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_parse_action "- not an action"'
  [ "$status" -ne 0 ]
}

@test "rejects line with bogus status marker" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_parse_action "- [Z] x"'
  [ "$status" -ne 0 ]
}

@test "rejects double-checkbox line" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_parse_action "- [ ] [ ] doubled"'
  [ "$status" -ne 0 ]
}

@test "block-id regex accepts plain id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^a01" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == "OK" ]]
}

@test "block-id regex accepts recurrence chain id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^a01~2" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == "OK" ]]
}

@test "block-id regex rejects too-short id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^xx" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK || echo NO'
  [ "$status" -eq 0 ]
  [[ "$output" == "NO" ]]
}

@test "block-id regex rejects too-long id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^abcdefghijklmnopq" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK || echo NO'
  [ "$status" -eq 0 ]
  [[ "$output" == "NO" ]]
}

@test "split_tail emits one key=value per line for known keys" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_split_tail "due:2026-05-01 every:1w priority:2"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"due=2026-05-01"* ]]
  [[ "$output" == *"every=1w"* ]]
  [[ "$output" == *"priority=2"* ]]
}

@test "split_tail flags bad key via AWIKI_AG_REJECT_REASON" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_split_tail "due:2026-05-01 bogus:xyz" \
      && echo "REASON=$AWIKI_AG_REJECT_REASON"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"due=2026-05-01"* ]]
  [[ "$output" == *"REASON=bad-key"* ]]
}

@test "split_tail flags bad date via AWIKI_AG_REJECT_REASON" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_split_tail "due:2026-13-40" \
      && echo "REASON=$AWIKI_AG_REJECT_REASON"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"REASON=bad-date"* ]]
}

@test "split_tail accepts every: named cadences and Nd/Nw/Nm" {
  for v in 1d 7d 1w 2w 1m 3m daily weekly monthly; do
    run bash -c "source scripts/lib/action-grammar.sh \
        && AWIKI_AG_REJECT_REASON='' \
        && awiki_grammar_split_tail 'every:$v' \
        && echo \"REASON=\$AWIKI_AG_REJECT_REASON\""
    [ "$status" -eq 0 ]
    [[ "$output" == *"every=$v"* ]]
    [[ "$output" != *"REASON=bad-"* ]]
  done
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
bats tests/action_grammar_test.bats
```

Expected: every test fails — library does not exist.

- [ ] **Step 3: Write `scripts/lib/action-grammar.sh`**

```bash
cat > scripts/lib/action-grammar.sh <<'EOF'
#!/usr/bin/env bash
# Source-only helper. Canonical action-line grammar for the awiki task layer.
# Sourced by: action-scan.sh (phase 17), lint task-rules T1-T15 (phase 17/18),
# action-recur.sh (phase 18), triage.sh (phase 18). Phase 16 ships ONLY this
# library and its unit tests — do not import it from any consumer in phase 16.

# Allowed status markers (raw chars; pad-with-space rendering is the same).
AWIKI_STATUS_MARKERS=(' ' '/' '?' '>' 'x' '-')

# Match a canonical single-line action:
#   leading "- " (or "* ") + "[<status>] " + text + optional "  ^<id>" tail.
# Captures (positional via BASH_REMATCH):
#   1: status char
#   2: full body after the checkbox (text + optional context + tail + id)
# We deliberately keep the regex permissive on the body and let the helper
# function below pick out the structured fields.
AWIKI_ACTION_LINE_RE='^[[:space:]]*[-*][[:space:]]+\[([ /?>x-])\][[:space:]]+(.+)$'

# A single tail key:value token. Allowed keys per spec:
#   due, defer, wait, since, every, done, priority, est.
# Values are checked separately (per-key shape) by awiki_grammar_split_tail.
AWIKI_TAIL_KEY_RE='^(due|defer|wait|since|every|done|priority|est):[^[:space:]]+$'

# Block-ID:
#   ^<base>           where <base> = [a-z0-9]{3,16}
#   OR ^<base>~<n>    recurrence chain instance, n = 1+ digits
# The leading ^ is part of the literal token in the source line.
AWIKI_BLOCK_ID_RE='^\^[a-z0-9]{3,16}(~[0-9]+)?$'

# Internal: per-key value shape regexes.
_AWIKI_RE_DATE='^[0-9]{4}-[0-9]{2}-[0-9]{2}$'
_AWIKI_RE_EVERY='^([0-9]+[dwm]|daily|weekly|monthly)$'
_AWIKI_RE_PRIORITY='^[1-3]$'
_AWIKI_RE_EST='^[0-9]+[mh]$'
_AWIKI_RE_WAIT='^\[\[[a-z0-9][a-z0-9-]*\]\]$'

# Validate that a YYYY-MM-DD string is also a real calendar date.
# Pure bash month-day clamp; leap year handled.
_awiki_is_calendar_date() {
  local s="$1"
  [[ "$s" =~ $_AWIKI_RE_DATE ]] || return 1
  local y="${s:0:4}" m="${s:5:2}" d="${s:8:2}"
  # Strip leading zeros for arithmetic.
  y=$((10#$y)); m=$((10#$m)); d=$((10#$d))
  (( m >= 1 && m <= 12 )) || return 1
  local maxday=31
  case "$m" in
    4|6|9|11) maxday=30 ;;
    2)
      if (( y % 4 == 0 && (y % 100 != 0 || y % 400 == 0) )); then
        maxday=29
      else
        maxday=28
      fi
      ;;
  esac
  (( d >= 1 && d <= maxday ))
}

# Parse an action line. On match (return 0):
#   AWIKI_AG_STATUS  — single status char
#   AWIKI_AG_TEXT    — text portion (everything between status and the first
#                      @context / tail-key / block-id, trimmed)
#   AWIKI_AG_CONTEXT — "@<slug>" or empty
#   AWIKI_AG_TAIL    — space-joined tail key:value tokens, in source order
#   AWIKI_AG_ID      — "^<id>" or empty
# On non-match: returns 1.
awiki_grammar_parse_action() {
  local line="$1"
  AWIKI_AG_STATUS=""; AWIKI_AG_TEXT=""; AWIKI_AG_CONTEXT=""
  AWIKI_AG_TAIL=""; AWIKI_AG_ID=""

  if ! [[ "$line" =~ $AWIKI_ACTION_LINE_RE ]]; then
    return 1
  fi
  local status="${BASH_REMATCH[1]}"
  local body="${BASH_REMATCH[2]}"

  # Reject double-checkbox lines (e.g., "- [ ] [ ] doubled"): if the body
  # itself starts with "[X]", the line is malformed.
  if [[ "$body" =~ ^\[.\][[:space:]] ]]; then
    return 1
  fi

  AWIKI_AG_STATUS="$status"

  # Tokenize body on whitespace.
  local -a toks=()
  local IFS=' '
  read -r -a toks <<< "$body"

  local text_toks=()
  local ctx="" tail_toks=() id=""
  local tok
  for tok in "${toks[@]}"; do
    if [[ -z "$tok" ]]; then continue; fi
    if [[ -z "$ctx" && "$tok" =~ ^@[a-z0-9][a-z0-9-]*$ ]]; then
      ctx="$tok"; continue
    fi
    if [[ "$tok" =~ $AWIKI_TAIL_KEY_RE ]]; then
      tail_toks+=("$tok"); continue
    fi
    if [[ "$tok" =~ $AWIKI_BLOCK_ID_RE ]]; then
      id="$tok"; continue
    fi
    # If we've already seen any tail/context/id token, anything after that
    # which isn't tail/id is a stray. The scanner / lint will flag.
    if (( ${#tail_toks[@]} > 0 )) || [[ -n "$id" || -n "$ctx" ]]; then
      # Treat as stray; do not append to text_toks — keep text contiguous.
      continue
    fi
    text_toks+=("$tok")
  done

  AWIKI_AG_TEXT="${text_toks[*]}"
  AWIKI_AG_CONTEXT="$ctx"
  AWIKI_AG_TAIL="${tail_toks[*]}"
  AWIKI_AG_ID="$id"
  return 0
}

# Split a tail string ("k1:v1 k2:v2 ...") and emit "k=v" per line on stdout.
# Sets AWIKI_AG_REJECT_REASON to one of:
#   ""            — clean
#   bad-key       — at least one token used a key outside the allowed set
#   bad-date      — a date-shaped key had an invalid calendar date
#   bad-format    — value did not match per-key shape regex
# When multiple reasons fire, the first one wins (caller can re-run for diag).
awiki_grammar_split_tail() {
  local tail="$1"
  AWIKI_AG_REJECT_REASON=""
  local tok key val
  for tok in $tail; do
    if [[ "$tok" != *:* ]]; then
      [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
      continue
    fi
    key="${tok%%:*}"
    val="${tok#*:}"
    case "$key" in
      due|defer|since|done)
        if ! _awiki_is_calendar_date "$val"; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-date"
        fi
        ;;
      every)
        if ! [[ "$val" =~ $_AWIKI_RE_EVERY ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      priority)
        if ! [[ "$val" =~ $_AWIKI_RE_PRIORITY ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      est)
        if ! [[ "$val" =~ $_AWIKI_RE_EST ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      wait)
        if ! [[ "$val" =~ $_AWIKI_RE_WAIT ]]; then
          [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-format"
        fi
        ;;
      *)
        [[ -z "$AWIKI_AG_REJECT_REASON" ]] && AWIKI_AG_REJECT_REASON="bad-key"
        ;;
    esac
    printf -- '%s=%s\n' "$key" "$val"
  done
}
EOF
chmod +x scripts/lib/action-grammar.sh
```

- [ ] **Step 4: Run test (expect PASS)**

```bash
bats tests/action_grammar_test.bats
```

Expected: every test passes (14 cases).

- [ ] **Step 5: Commit**

```bash
git add scripts/lib/action-grammar.sh tests/action_grammar_test.bats
git commit -m "feat(task): add scripts/lib/action-grammar.sh canonical grammar lib + tests"
```

---

## Task 16.4: `scripts/capture.sh` — append to `content/inbox.md` with full sanitization

`capture.sh` is the bash entry-point for quick capture. It implements the **full** spec sanitization table:

| Pattern | Action |
|---------|--------|
| Newlines / `\r` / NUL / control chars (except tab → space) | Reject; exit 4. |
| Length > 2000 chars | Truncate, append `…`, log warning. |
| `<!--` / `-->` | Replace with `< !--` / `--  >`. |
| `[[` / `]]` | Replace with `[ [` / `] ]`. |
| `^[a-z0-9]{4,}` block-ID-shaped tokens | Prefix with backslash. |
| Markdown checkbox prefix `[ ]`/`[/]`/etc. at line start | Reject; exit 4. |

When **any** sanitization rule fires (replace category), the script prints a unified diff between the raw input and the final line on stderr and prefixes the header `SANITIZATION-APPLIED|<rule1>,<rule2>,...`. Hard-rejections exit 4 with `ERROR|<reason>` on stderr and write nothing.

Output line format: `- <ISO-datetime> <sanitized-text>` appended to `content/inbox.md`. ISO datetime is `date '+%Y-%m-%d %H:%M'` — minute precision, matching the inbox-line example in the spec.

`capture.sh` does NOT yet acquire the lock — the spec says the MCP server's `capture` handler does, but the bash entrypoint runs solo per single-user invocation. The lock library is sourced and used in phase 17/18 by the MCP server and triage paths. (Reviewers: this is intentional — see "Lib-only-this-phase split" in the conventions block; phase 18 patches `capture.sh` to acquire `awiki_lock_with --timeout=$AWIKI_LOCK_TIMEOUT_USER` once the MCP `capture` handler is wired so they share the same critical section.)

**Files:** Create: `scripts/capture.sh`, `tests/capture_test.bats`.

- [ ] **Step 1: Write the test**

```bash
cat > tests/capture_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "---\ntitle: \"Inbox\"\ntype: inbox\ndraft: true\n---\n" > "$WORK/content/inbox.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "capture appends a line with ISO datetime prefix" {
  run bash scripts/capture.sh -- "call dentist about crown"
  [ "$status" -eq 0 ]
  run grep -E '^- 20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2} call dentist about crown$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture joins multi-arg input with single spaces" {
  run bash scripts/capture.sh -- pick up groceries
  [ "$status" -eq 0 ]
  run grep -E '^- .* pick up groceries$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture rejects empty text" {
  run bash scripts/capture.sh -- ""
  [ "$status" -eq 4 ]
  [[ "$output" == *"empty"* ]]
}

@test "capture rejects text containing a literal newline" {
  run bash scripts/capture.sh -- "line one
line two"
  [ "$status" -eq 4 ]
  [[ "$output" == *"newline"* || "$output" == *"control"* ]]
}

@test "capture rejects checkbox-prefix-at-start" {
  run bash scripts/capture.sh -- "[ ] not an action yet"
  [ "$status" -eq 4 ]
  [[ "$output" == *"checkbox"* ]]
}

@test "capture rejects all six checkbox markers at start" {
  for s in " " "/" "?" ">" "x" "-"; do
    run bash scripts/capture.sh -- "[$s] foo"
    [ "$status" -eq 4 ]
  done
}

@test "capture truncates >2000 chars and appends ellipsis" {
  long=$(printf 'a%.0s' {1..2050})
  run bash scripts/capture.sh -- "$long"
  [ "$status" -eq 0 ]
  [[ "$output" == *"length-truncated"* ]]
  # Last char of appended line should be "…".
  last=$(tail -1 content/inbox.md)
  [[ "$last" == *"…" ]]
}

@test "capture neutralizes wikilinks" {
  run bash scripts/capture.sh -- "see [[s-as-we-may-think]] later"
  [ "$status" -eq 0 ]
  [[ "$output" == *"wikilink-neutralized"* ]]
  run grep -F "[ [s-as-we-may-think] ]" content/inbox.md
  [ "$status" -eq 0 ]
  run grep -F "[[s-as-we-may-think]]" content/inbox.md
  [ "$status" -ne 0 ]
}

@test "capture neutralizes HTML comment markers" {
  run bash scripts/capture.sh -- "watch out <!-- inside --> here"
  [ "$status" -eq 0 ]
  [[ "$output" == *"comment-neutralized"* ]]
  run grep -F "< !--" content/inbox.md
  [ "$status" -eq 0 ]
  run grep -F "--  >" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture backslash-escapes block-ID-shaped tokens" {
  run bash scripts/capture.sh -- "remember ^abc1234 token"
  [ "$status" -eq 0 ]
  [[ "$output" == *"block-id-escaped"* ]]
  run grep -F "\\^abc1234" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture leaves a clean line untouched" {
  run bash scripts/capture.sh -- "totally normal text"
  [ "$status" -eq 0 ]
  [[ "$output" != *"SANITIZATION-APPLIED"* ]]
}

@test "capture creates inbox.md if missing (with frontmatter scaffold)" {
  rm content/inbox.md
  run bash scripts/capture.sh -- "first capture"
  [ "$status" -eq 0 ]
  run head -1 content/inbox.md
  [[ "$output" == "---" ]]
  run grep '^type: inbox$' content/inbox.md
  [ "$status" -eq 0 ]
  run grep -E '^- .* first capture$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "capture rejects tab as newline-equivalent control? no — tab is converted to space" {
  printf -- "with\ttab" > /tmp/awiki-cap-tab.txt
  txt="$(cat /tmp/awiki-cap-tab.txt)"
  run bash scripts/capture.sh -- "$txt"
  [ "$status" -eq 0 ]
  run grep -E '^- .* with tab$' content/inbox.md
  [ "$status" -eq 0 ]
  rm -f /tmp/awiki-cap-tab.txt
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
bats tests/capture_test.bats
```

Expected: every test fails — `scripts/capture.sh` does not exist.

- [ ] **Step 3: Write `scripts/capture.sh`**

```bash
cat > scripts/capture.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# scripts/capture.sh — append a sanitized capture line to content/inbox.md.
# Usage: capture.sh -- "<text...>"
# Exit codes: 0 OK; 1 usage; 4 hard-rejection (control-char / checkbox-prefix);
#             other non-zero unexpected.

INBOX="${AWIKI_INBOX_FILE:-content/inbox.md}"

usage() {
  cat <<USAGE
usage: capture.sh -- "<text>"
  Appends a sanitized capture line to $INBOX.
  See capture --help / docs/just-help.txt for the sanitization table.
USAGE
}

# Parse args: require leading "--" so callers can pass any text safely.
if [[ $# -eq 0 ]]; then usage >&2; exit 1; fi
if [[ "$1" == "--help" || "$1" == "-h" ]]; then usage; exit 0; fi
if [[ "$1" != "--" ]]; then
  echo "ERROR|missing '--' separator (use: capture.sh -- \"<text>\")" >&2
  exit 1
fi
shift

if [[ $# -eq 0 ]]; then
  echo "ERROR|empty: no text after '--'" >&2
  exit 4
fi

raw="$*"
if [[ -z "$raw" ]]; then
  echo "ERROR|empty: text is blank after argument join" >&2
  exit 4
fi

# === Hard-rejections ===

# Newline / CR / NUL / non-tab control chars.
# Use LC_ALL=C for byte-precise scan.
if LC_ALL=C printf -- '%s' "$raw" | LC_ALL=C grep -q $'[\x00-\x08\x0a-\x1f\x7f]'; then
  echo "ERROR|control: text contains a newline / CR / NUL / control char (only tab is allowed and converts to space)" >&2
  exit 4
fi

# Checkbox-prefix-at-start: "[ ]" / "[/]" / "[?]" / "[>]" / "[x]" / "[-]" at column 1.
if [[ "$raw" =~ ^\[[\ /\?\>x-]\] ]]; then
  echo "ERROR|checkbox: line begins with a checkbox marker; captures are not actions" >&2
  exit 4
fi

# === Replace-category sanitizations ===

text="$raw"
applied=()

# Tab → space (always; not counted as a sanitization since it's a benign normalization).
text="${text//$'\t'/ }"

# Length > 2000.
if [[ ${#text} -gt 2000 ]]; then
  text="${text:0:2000}…"
  applied+=("length-truncated")
fi

# Wikilink open/close.
if [[ "$text" == *"[["* || "$text" == *"]]"* ]]; then
  text="${text//\[\[/[ [}"
  text="${text//\]\]/] ]}"
  applied+=("wikilink-neutralized")
fi

# HTML comment markers.
if [[ "$text" == *"<!--"* || "$text" == *"-->"* ]]; then
  text="${text//<!--/< !--}"
  text="${text//-->/--  >}"
  applied+=("comment-neutralized")
fi

# Block-ID-shaped tokens: ^[a-z0-9]{4,} → prefix with backslash.
# We use bash extended-regex via a loop to be conservative.
if [[ "$text" =~ \^[a-z0-9]{4,} ]]; then
  text="$(printf -- '%s' "$text" | sed -E 's/(\^)([a-z0-9]{4,})/\\\1\2/g')"
  applied+=("block-id-escaped")
fi

# === Emit ===

mkdir -p "$(dirname "$INBOX")"
if [[ ! -f "$INBOX" ]]; then
  {
    printf -- '---\n'
    printf -- 'title: "Inbox"\n'
    printf -- 'type: inbox\n'
    printf -- 'draft: true\n'
    printf -- '---\n'
  } > "$INBOX"
fi

ts="$(date '+%Y-%m-%d %H:%M')"
line="- $ts $text"
printf -- '%s\n' "$line" >> "$INBOX"

if (( ${#applied[@]} > 0 )); then
  IFS=, ; tags="${applied[*]}" ; unset IFS
  echo "SANITIZATION-APPLIED|$tags" >&2
  # Print a minimal diff (raw vs final) so the user can audit.
  echo "  raw  : $raw" >&2
  echo "  final: $text" >&2
fi

# Best-effort log (non-fatal if log-append.sh missing in the test sandbox).
if [[ -x scripts/log-append.sh ]]; then
  bash scripts/log-append.sh -- capture "$(printf -- '%s' "$text" | head -c 80)" >/dev/null 2>&1 || true
fi

echo "OK|appended|$line"
EOF
chmod +x scripts/capture.sh
```

- [ ] **Step 4: Run test (expect PASS)**

```bash
bats tests/capture_test.bats
```

Expected: 13 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/capture.sh tests/capture_test.bats
git commit -m "feat(task): add scripts/capture.sh with full sanitization table"
```

---

## Task 16.5: Pre-templated `<!-- BEGIN task-layer -->` block content

`task-init.sh` will append this block to `WIKI.md`. We need the canonical text on disk first so the script can `cat` it. We store it under `scripts/templates/wiki-task-layer.md` (a new templates dir reserved for `task-init` scaffolding).

The template covers per the spec "Schema Additions to WIKI.md":
- Page-kind enum extension (`project`, `context`).
- System-page entries (`inbox`, `agenda`).
- Inline-grammar table (status markers, tail keys, examples).
- Lint-rule names T1-T15 with one-line summaries (full bodies arrive in phase 17/18 plans).
- Triage workflow stub pointing to phase 18.
- Weekly-review workflow stub pointing to phase 19.

**Files:** Create: `scripts/templates/wiki-task-layer.md`.

- [ ] **Step 1: Create the templates directory and file**

```bash
mkdir -p scripts/templates
cat > scripts/templates/wiki-task-layer.md <<'EOF'
<!-- BEGIN task-layer -->
## Task Layer (opt-in)

This block is managed by `scripts/task-init.sh`. To remove the layer,
delete everything between the BEGIN and END markers and run
`bash scripts/lint.sh` to surface broken references.

### Page-kind enum extension

| Kind      | Meaning                                                          |
|-----------|------------------------------------------------------------------|
| `project` | Outcome requiring multiple actions; tracked to completion.       |
| `context` | Situation/tool/place where actions can happen (`phone`, `home`). |

Areas of responsibility (e.g. `home`, `career`, `health`) reuse the
existing `entity` page kind. Project frontmatter references areas via
`area: [[entity-slug]]`.

### System pages

| Page      | Path                                | Notes                                  |
|-----------|-------------------------------------|----------------------------------------|
| `inbox`   | `content/inbox.md`                  | Append-only capture stream.            |
| `agenda`  | `content/agenda/<view>.md`          | Generated; managed-region.             |

Both are exempt from the orphan-check. `agenda/*.md` is also exempt
from the staleness rule (rebuilt on cadence).

### Inline action grammar

Canonical line:

```
- [STATUS] <text> [@context] [key:value ...] [^id]
```

Action lines are strictly single-line; multi-line continuation is not
supported (lint rule `T15` warns).

Status markers (render in vanilla Obsidian):

| Marker | Meaning                          |
|--------|----------------------------------|
| `[ ]`  | open / next                      |
| `[/]`  | in-progress                      |
| `[?]`  | waiting (delegated)              |
| `[>]`  | someday / deferred-indefinite    |
| `[x]`  | done                             |
| `[-]`  | cancelled                        |

Tail metadata keys (lowercase, ASCII):

| Key        | Format                            | Meaning                          |
|------------|-----------------------------------|----------------------------------|
| `due:`     | `YYYY-MM-DD`                      | hard date — overdue if past      |
| `defer:`   | `YYYY-MM-DD`                      | hidden from next-actions until   |
| `wait:`    | `[[entity-slug]]`                 | required if `[?]`                |
| `since:`   | `YYYY-MM-DD`                      | when waiting started             |
| `every:`   | `Nd` / `Nw` / `Nm` / `daily` …    | recurrence on completion         |
| `done:`    | `YYYY-MM-DD`                      | auto-stamped on flip to `[x]`    |
| `priority:`| `1`–`3`                           | optional; lower = higher         |
| `est:`     | `Nm` / `Nh`                       | optional time estimate           |

Block-IDs (`^abc12345`) follow `^[a-z0-9]{3,16}`. Recurrence-chain
instances use the form `^<base>~<n>` with `n` ≥ 2.

### Capture + triage

Quick capture: `just capture "<text>"` appends to `content/inbox.md`.
Full capture-and-triage workflow with the seven outcomes (trash,
do-now, act, defer-scheduled, waiting, reference, someday) is
documented at the agent layer (phase 18).

### Weekly review

Documented at the agent layer (phase 19). Backed by
`scripts/review-status.sh` and the `review_status` MCP tool.

### Task-layer lint rules

| Code  | Level | Summary                                                  |
|-------|-------|----------------------------------------------------------|
| `T1`  | error | bad status marker                                        |
| `T2`  | error | duplicate block-ID                                       |
| `T3`  | error | tail key not in allowed set                              |
| `T4`  | error | bad date in `due:`/`defer:`/`since:`/`done:`             |
| `T5`  | error | `[?]` without `wait:`                                    |
| `T6`  | error | `@context` references missing context page              |
| `T7`  | error | hand-edit detected inside agenda managed region          |
| `T8`  | warn  | active project with zero open actions                    |
| `T9`  | warn  | `[?]` with `since:` > 14 days ago                        |
| `T10` | warn  | `[ ]`/`[/]` overdue                                      |
| `T11` | warn  | `[>]` on stale page (90+ days)                           |
| `T12` | warn  | recur chain ≥150 (warn) / ≥200 (error)                   |
| `T13` | info  | context page with zero referencing actions               |
| `T14` | error | bad block-ID shape                                       |
| `T15` | warn  | indented continuation line after a checkbox              |

Bodies and `--fix` semantics for T1-T7, T14, T15 ship with phase 17;
T8-T13 ship with phase 18.
<!-- END task-layer -->
EOF
```

- [ ] **Step 2: Verify the marker pair is balanced**

```bash
grep -c '^<!-- BEGIN task-layer -->' scripts/templates/wiki-task-layer.md
grep -c '^<!-- END task-layer -->' scripts/templates/wiki-task-layer.md
```

Expected: each prints `1`.

- [ ] **Step 3: Commit**

```bash
git add scripts/templates/wiki-task-layer.md
git commit -m "feat(task): add WIKI.md task-layer block template (page kinds, grammar, T1-T15 names)"
```

---

## Task 16.6: `.gitignore` updates for `.awiki/lock`, task-count, last-review, maps

The new state files under `.awiki/` must be gitignored except for `config` (already tracked). The lock file is local-only. `task-count` and `last-review` are local-only too — they are per-checkout. Maps under `.awiki/maps/` are scanner outputs (phase 17) but we add the ignore rule now to avoid commit churn later.

**Files:** Modify: `.gitignore`.

- [ ] **Step 1: Inspect current `.awiki/` rules**

```bash
grep -n '^\.awiki' .gitignore
```

Expected: existing lines from phase 1, e.g. `.awiki/*` plus `!.awiki/.gitkeep` and `!.awiki/config`.

- [ ] **Step 2: Append phase-16 ignore block**

```bash
cat >> .gitignore <<'EOF'

# === phase 16: task layer ===
.awiki/lock
.awiki/last-review
.awiki/task-count
.awiki/maps/
EOF
```

- [ ] **Step 3: Verify**

```bash
git check-ignore -v .awiki/lock .awiki/last-review .awiki/task-count .awiki/maps/actions.tsv 2>&1 | head -8
```

Expected: each path matches a `.gitignore:<line>:<pattern>` row.

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "chore(task): gitignore .awiki/lock, last-review, task-count, maps/"
```

---

## Task 16.7: `scripts/task-init.sh` skeleton + branch step + state-files step

`task-init.sh` is per-step idempotent — every step checks its own state and skips if already done. We build it incrementally: this task covers the script skeleton (arg parse, log header, dispatcher), step 4 (`.awiki/config` patch), and step 5 (state files: `.awiki/last-review`, `.awiki/task-count`).

The remaining steps land in tasks 16.8 (pages), 16.9 (`WIKI.md` patch), 16.10 (encryption coverage prompt), 16.11 (pre-commit hook prompt — DEFERRED to phase 19 per spec; we land only the no-op scaffold), 16.12 (smoke instructions + log).

**Files:** Create: `scripts/task-init.sh`, `tests/task_init_test.bats`.

- [ ] **Step 1: Write the test (covers ONLY the steps in this task — config + state files)**

```bash
cat > tests/task_init_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\nAWIKI_STALE_DAYS=90\nAWIKI_LOG_QUERIES=0\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "task-init creates .awiki/last-review = today" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f .awiki/last-review ]
  today="$(date '+%Y-%m-%d')"
  run cat .awiki/last-review
  [[ "$output" == "$today" ]]
}

@test "task-init creates .awiki/task-count = 0" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f .awiki/task-count ]
  run cat .awiki/task-count
  [[ "$output" == "0" ]]
}

@test "task-init appends AWIKI_AGENDA_AFTER_N=5 and AWIKI_TASK_LAYER=on if absent" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_AGENDA_AFTER_N=5$' .awiki/config
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_TASK_LAYER=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "task-init preserves existing user-set config values" {
  echo 'AWIKI_AGENDA_AFTER_N=10' >> .awiki/config
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  # Must not duplicate, must not overwrite.
  run grep -c '^AWIKI_AGENDA_AFTER_N=' .awiki/config
  [[ "$output" == "1" ]]
  run grep '^AWIKI_AGENDA_AFTER_N=' .awiki/config
  [[ "$output" == "AWIKI_AGENDA_AFTER_N=10" ]]
}

@test "task-init re-run is idempotent: state files unchanged" {
  bash scripts/task-init.sh
  cp .awiki/last-review /tmp/awiki-lr-1
  cp .awiki/task-count /tmp/awiki-tc-1
  bash scripts/task-init.sh
  diff /tmp/awiki-lr-1 .awiki/last-review
  diff /tmp/awiki-tc-1 .awiki/task-count
  rm -f /tmp/awiki-lr-1 /tmp/awiki-tc-1
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

```bash
bats tests/task_init_test.bats
```

Expected: every test fails — `task-init.sh` does not exist.

- [ ] **Step 3: Write the skeleton + step 4 + step 5**

```bash
cat > scripts/task-init.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# scripts/task-init.sh — per-step idempotent enabler for the awiki task layer.
# Each step detects its own existing state and skips work that is already done,
# so partial-failure re-runs are safe. There is no binary "already enabled"
# gate.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

CONFIG_FILE=".awiki/config"
LAST_REVIEW=".awiki/last-review"
TASK_COUNT=".awiki/task-count"
WIKI_MD="WIKI.md"
TEMPLATE="scripts/templates/wiki-task-layer.md"

note() { echo "TASK-INIT|$*"; }
warn() { echo "TASK-INIT|WARN|$*" >&2; }

step_state_files() {
  mkdir -p .awiki
  if [[ ! -f "$LAST_REVIEW" ]]; then
    date '+%Y-%m-%d' > "$LAST_REVIEW"
    note "created $LAST_REVIEW"
  else
    note "skip $LAST_REVIEW (exists)"
  fi
  if [[ ! -f "$TASK_COUNT" ]]; then
    echo 0 > "$TASK_COUNT"
    note "created $TASK_COUNT"
  else
    note "skip $TASK_COUNT (exists)"
  fi
}

step_config() {
  if [[ ! -f "$CONFIG_FILE" ]]; then
    mkdir -p .awiki
    : > "$CONFIG_FILE"
  fi
  if ! grep -q '^AWIKI_AGENDA_AFTER_N=' "$CONFIG_FILE"; then
    printf -- 'AWIKI_AGENDA_AFTER_N=5\n' >> "$CONFIG_FILE"
    note "appended AWIKI_AGENDA_AFTER_N=5"
  else
    note "skip AWIKI_AGENDA_AFTER_N (already set)"
  fi
  if ! grep -q '^AWIKI_TASK_LAYER=' "$CONFIG_FILE"; then
    printf -- 'AWIKI_TASK_LAYER=on\n' >> "$CONFIG_FILE"
    note "appended AWIKI_TASK_LAYER=on"
  else
    note "skip AWIKI_TASK_LAYER (already set)"
  fi
}

main() {
  note "start"
  step_config
  step_state_files
  note "done"
}

main "$@"
EOF
chmod +x scripts/task-init.sh
```

- [ ] **Step 4: Run test (expect PASS)**

```bash
bats tests/task_init_test.bats
```

Expected: 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/task-init.sh tests/task_init_test.bats
git commit -m "feat(task): scaffold task-init.sh with config + state-files steps"
```

---

## Task 16.8: `task-init.sh` step — system pages (inbox, projects, contexts, agenda)

Add `step_pages` to `task-init.sh`. Creates files only if missing (`[ -f "$f" ] || write`). Files per spec "task-init Workflow" step 1:

- `content/inbox.md` (frontmatter + empty body) — already auto-scaffolded by `capture.sh`, but we ensure the canonical version with the title field is in place.
- `content/projects/_index.md` (section index, page-list shortcode).
- `content/contexts/_index.md` (section index).
- `content/agenda/_index.md`.
- `content/agenda/{next-actions,today,waiting,someday,stuck-projects}.md` — empty managed-region pair + minimal frontmatter (`type: agenda`, `last_updated: <today>`, `draft: false`).
- `content/agenda/review-log.md` — frontmatter `type: agenda`, body header only.
- Three starter context pages: `content/contexts/{phone,errands,computer}.md`.

Managed-region markers in agenda pages use `<!-- BEGIN managed-region -->` / `<!-- END managed-region -->`. Phase 17's `agenda.sh` will replace `managed-region` in the marker name with the per-view name (`agenda:next-actions` etc.) — for phase 16 we only ship the empty placeholder pair so Hugo can render the page.

**Files:** Modify: `scripts/task-init.sh`, `tests/task_init_test.bats`.

- [ ] **Step 1: Append failing tests**

```bash
cat >> tests/task_init_test.bats <<'EOF'

@test "task-init creates content/inbox.md with type: inbox frontmatter" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/inbox.md ]
  run grep '^type: inbox$' content/inbox.md
  [ "$status" -eq 0 ]
  run grep '^draft: true$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "task-init creates section indexes" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/projects/_index.md ]
  [ -f content/contexts/_index.md ]
  [ -f content/agenda/_index.md ]
}

@test "task-init creates five agenda pages with managed-region pair" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  for view in next-actions today waiting someday stuck-projects; do
    [ -f "content/agenda/$view.md" ]
    run grep '^<!-- BEGIN managed-region -->$' "content/agenda/$view.md"
    [ "$status" -eq 0 ]
    run grep '^<!-- END managed-region -->$' "content/agenda/$view.md"
    [ "$status" -eq 0 ]
    run grep '^type: agenda$' "content/agenda/$view.md"
    [ "$status" -eq 0 ]
  done
}

@test "task-init creates agenda/review-log.md without managed region" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/agenda/review-log.md ]
  run grep '^type: agenda$' content/agenda/review-log.md
  [ "$status" -eq 0 ]
  run grep '^<!-- BEGIN managed-region -->' content/agenda/review-log.md
  [ "$status" -ne 0 ]
}

@test "task-init creates three starter context pages" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  for ctx in phone errands computer; do
    [ -f "content/contexts/$ctx.md" ]
    run grep '^type: context$' "content/contexts/$ctx.md"
    [ "$status" -eq 0 ]
    run grep -F "@$ctx" "content/contexts/$ctx.md"
    [ "$status" -eq 0 ]
  done
}

@test "task-init does not overwrite an existing custom page" {
  mkdir -p content/contexts
  printf -- "---\ntype: context\ncustom: yes\n---\n\nMy own thing.\n" > content/contexts/phone.md
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep '^custom: yes$' content/contexts/phone.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL on the new ones)**

```bash
bats tests/task_init_test.bats
```

Expected: 6 new tests fail; 5 prior tests still pass.

- [ ] **Step 3: Add `step_pages` to `scripts/task-init.sh` and call it from `main`**

Open `scripts/task-init.sh` and insert the following function definition between `step_state_files()` and `main()`:

```bash
step_pages() {
  local today; today="$(date '+%Y-%m-%d')"

  # 1. content/inbox.md
  if [[ ! -f content/inbox.md ]]; then
    mkdir -p content
    cat > content/inbox.md <<INBOX
---
title: "Inbox"
type: inbox
draft: true
---

INBOX
    note "created content/inbox.md"
  else
    note "skip content/inbox.md (exists)"
  fi

  # 2. Section indexes.
  mkdir -p content/projects content/contexts content/agenda
  for sec in projects contexts agenda; do
    local idx="content/$sec/_index.md"
    if [[ ! -f "$idx" ]]; then
      cat > "$idx" <<INDEX
---
title: "${sec^}"
type: section
draft: false
---

{{< page-list >}}
INDEX
      note "created $idx"
    else
      note "skip $idx (exists)"
    fi
  done

  # 3. Five agenda views with empty managed-region pair.
  local view
  for view in next-actions today waiting someday stuck-projects; do
    local f="content/agenda/$view.md"
    if [[ ! -f "$f" ]]; then
      cat > "$f" <<AGENDA
---
title: "Agenda — ${view//-/ }"
type: agenda
last_updated: $today
draft: false
---

<!-- BEGIN managed-region -->
<!-- END managed-region -->
AGENDA
      note "created $f"
    else
      note "skip $f (exists)"
    fi
  done

  # 4. agenda/review-log.md (append-only history; no managed region).
  if [[ ! -f content/agenda/review-log.md ]]; then
    cat > content/agenda/review-log.md <<RLOG
---
title: "Review Log"
type: agenda
last_updated: $today
draft: false
---

# Review Log

Append-only history of weekly reviews. New entries are added by
\`scripts/review-status.sh mark_done\` (phase 19).
RLOG
    note "created content/agenda/review-log.md"
  else
    note "skip content/agenda/review-log.md (exists)"
  fi

  # 5. Three starter context pages.
  local ctx
  for ctx in phone errands computer; do
    local f="content/contexts/$ctx.md"
    if [[ ! -f "$f" ]]; then
      cat > "$f" <<CTX
---
title: "@$ctx"
date: $today
last_updated: $today
type: context
aliases: ['@$ctx']
tools: []
draft: false
---

Actions tagged \`@$ctx\` reference this page.
CTX
      note "created $f"
    else
      note "skip $f (exists)"
    fi
  done
}
```

Then update `main()` to call it:

```bash
# Before:
#   step_config
#   step_state_files
# After:
#   step_pages
#   step_config
#   step_state_files
```

Use `Edit` rather than rewriting the whole file. Concretely:

```bash
# Verify the patch with:
grep -n 'step_pages' scripts/task-init.sh
```

Expected: at least two lines — the function definition and the call inside `main`.

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/task_init_test.bats
```

Expected: 11 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/task-init.sh tests/task_init_test.bats
git commit -m "feat(task): task-init step — system pages (inbox, sections, agenda views, contexts)"
```

---

## Task 16.9: `task-init.sh` step — patch `WIKI.md` task-layer block

This step detects the `<!-- BEGIN task-layer -->` marker in `WIKI.md`. If absent, append the templated block (from `scripts/templates/wiki-task-layer.md`). If the marker is present but the contents differ from the template (defined as: `<!-- BEGIN task-layer -->` exists), print a **warn** and skip — user has customised; manual reconciliation required. We do NOT diff or attempt to merge in phase 16.

**Files:** Modify: `scripts/task-init.sh`, `tests/task_init_test.bats`.

- [ ] **Step 1: Append failing tests**

```bash
cat >> tests/task_init_test.bats <<'EOF'

@test "task-init appends task-layer block to WIKI.md" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep '^<!-- BEGIN task-layer -->$' WIKI.md
  [ "$status" -eq 0 ]
  run grep '^<!-- END task-layer -->$' WIKI.md
  [ "$status" -eq 0 ]
}

@test "task-init WIKI.md block contains expected sections" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -F 'Task Layer (opt-in)' WIKI.md
  [ "$status" -eq 0 ]
  run grep -F '## Task Layer' WIKI.md
  [ "$status" -eq 0 ]
  run grep -F 'T15' WIKI.md
  [ "$status" -eq 0 ]
}

@test "task-init does not double-append if marker already present" {
  bash scripts/task-init.sh
  bash scripts/task-init.sh
  run grep -c '^<!-- BEGIN task-layer -->$' WIKI.md
  [[ "$output" == "1" ]]
}

@test "task-init prints WARN if marker present but body modified" {
  bash scripts/task-init.sh
  # Add stray text inside the block (simulate user customization).
  awk '/^<!-- BEGIN task-layer -->$/ {print; print "MY CUSTOM TEXT"; next} {print}' WIKI.md > WIKI.md.tmp
  mv WIKI.md.tmp WIKI.md
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]] || [[ "$output" == *"customized"* || "$output" == *"customised"* ]]
  # Block is still there, MY CUSTOM TEXT preserved.
  run grep -F 'MY CUSTOM TEXT' WIKI.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/task_init_test.bats
```

Expected: 4 new tests fail; 11 prior tests still pass.

- [ ] **Step 3: Add `step_wiki_md` and call from `main`**

Insert in `scripts/task-init.sh` between `step_pages()` and `step_config()`:

```bash
step_wiki_md() {
  if [[ ! -f "$WIKI_MD" ]]; then
    warn "WIKI.md not found at repo root; skipping task-layer block patch"
    return 0
  fi
  if [[ ! -f "$TEMPLATE" ]]; then
    warn "template missing: $TEMPLATE; skipping"
    return 0
  fi

  if grep -q '^<!-- BEGIN task-layer -->$' "$WIKI_MD"; then
    # Marker present. Compute reference template content between markers
    # and current file content between markers. If they differ, warn and skip.
    local tmpl_body cur_body
    tmpl_body="$(awk '/^<!-- BEGIN task-layer -->$/{f=1} f{print} /^<!-- END task-layer -->$/{f=0}' "$TEMPLATE")"
    cur_body="$(awk '/^<!-- BEGIN task-layer -->$/{f=1} f{print} /^<!-- END task-layer -->$/{f=0}' "$WIKI_MD")"
    if [[ "$tmpl_body" != "$cur_body" ]]; then
      warn "WIKI.md task-layer block has been customised; skipping (manual reconciliation required)"
    else
      note "skip WIKI.md (block already up to date)"
    fi
    return 0
  fi

  # Append a blank line then the block.
  {
    printf -- '\n'
    cat "$TEMPLATE"
  } >> "$WIKI_MD"
  note "appended task-layer block to WIKI.md"
}
```

Update `main()` so the order is:

```bash
step_pages
step_wiki_md
step_config
step_state_files
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/task_init_test.bats
```

Expected: 15 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/task-init.sh tests/task_init_test.bats
git commit -m "feat(task): task-init step — patch WIKI.md with task-layer block"
```

---

## Task 16.10: `task-init.sh` step — encryption coverage prompt

Per spec "task-init Workflow" step 3 / Threat Model: detect `encrypt-init` state by reading `.gitattributes`. If the git-crypt section is present, prompt the user:

> Add `content/inbox.md` and `content/agenda/**` to git-crypt patterns? (Y/n)

On `Y` (or empty / EOF in non-interactive mode where `AWIKI_TASK_INIT_ASSUME_YES=1`), append the patterns atomically. On `n`, print warning that agenda pages will exclude private actions.

We detect the git-crypt section by the presence of any line matching `filter=git-crypt diff=git-crypt` in `.gitattributes`.

To keep the test deterministic we honor `AWIKI_TASK_INIT_ASSUME_YES=1` (yes) and `AWIKI_TASK_INIT_ASSUME_NO=1` (no) env overrides; absent both, the script reads from stdin with a 1-line `read -r`.

**Files:** Modify: `scripts/task-init.sh`, `tests/task_init_test.bats`.

- [ ] **Step 1: Append failing tests**

```bash
cat >> tests/task_init_test.bats <<'EOF'

@test "task-init skips encryption step when no .gitattributes" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"skip encryption"* ]] || [[ "$output" == *"no git-crypt"* ]]
}

@test "task-init appends git-crypt patterns when ASSUME_YES" {
  cat > .gitattributes <<EOF2
content/private/** filter=git-crypt diff=git-crypt
EOF2
  AWIKI_TASK_INIT_ASSUME_YES=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -F 'content/inbox.md filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  run grep -F 'content/agenda/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
}

@test "task-init does NOT append patterns when ASSUME_NO" {
  cat > .gitattributes <<EOF2
content/private/** filter=git-crypt diff=git-crypt
EOF2
  AWIKI_TASK_INIT_ASSUME_NO=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -F 'content/inbox.md filter=git-crypt' .gitattributes
  [ "$status" -ne 0 ]
  [[ "$output" == *"WARN"* ]] || [[ "$output" == *"declined"* ]]
}

@test "task-init does not double-append patterns on rerun" {
  cat > .gitattributes <<EOF2
content/private/** filter=git-crypt diff=git-crypt
EOF2
  AWIKI_TASK_INIT_ASSUME_YES=1 bash scripts/task-init.sh
  AWIKI_TASK_INIT_ASSUME_YES=1 bash scripts/task-init.sh
  run grep -c -F 'content/inbox.md filter=git-crypt diff=git-crypt' .gitattributes
  [[ "$output" == "1" ]]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/task_init_test.bats
```

Expected: 4 new tests fail.

- [ ] **Step 3: Add `step_encryption` and call from `main`**

Insert after `step_wiki_md()`:

```bash
step_encryption() {
  local ga=".gitattributes"
  if [[ ! -f "$ga" ]]; then
    note "skip encryption (no .gitattributes — encrypt-init has not run)"
    return 0
  fi
  if ! grep -q 'filter=git-crypt diff=git-crypt' "$ga"; then
    note "skip encryption (no git-crypt section in .gitattributes)"
    return 0
  fi

  local need_inbox=0 need_agenda=0
  grep -q -F 'content/inbox.md filter=git-crypt diff=git-crypt' "$ga" || need_inbox=1
  grep -q -F 'content/agenda/** filter=git-crypt diff=git-crypt' "$ga" || need_agenda=1

  if [[ $need_inbox -eq 0 && $need_agenda -eq 0 ]]; then
    note "skip encryption (patterns already present)"
    return 0
  fi

  local answer=""
  if [[ "${AWIKI_TASK_INIT_ASSUME_YES:-0}" == "1" ]]; then
    answer="y"
  elif [[ "${AWIKI_TASK_INIT_ASSUME_NO:-0}" == "1" ]]; then
    answer="n"
  else
    echo "Add 'content/inbox.md' and 'content/agenda/**' to git-crypt patterns? (Y/n) "
    read -r answer || answer="y"
    answer="${answer:-y}"
  fi

  case "$answer" in
    y|Y|yes|YES)
      [[ $need_inbox -eq 1  ]] && printf -- '\ncontent/inbox.md filter=git-crypt diff=git-crypt\n' >> "$ga"
      [[ $need_agenda -eq 1 ]] && printf -- 'content/agenda/** filter=git-crypt diff=git-crypt\n' >> "$ga"
      note "added git-crypt patterns for inbox + agenda"
      ;;
    *)
      warn "encryption patterns declined; agenda pages will exclude private actions (placeholder count only)"
      ;;
  esac
}
```

Update `main()` to call `step_encryption` between `step_wiki_md` and `step_config`. Final order:

```bash
step_pages
step_wiki_md
step_encryption
step_config
step_state_files
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/task_init_test.bats
```

Expected: 19 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/task-init.sh tests/task_init_test.bats
git commit -m "feat(task): task-init step — git-crypt encryption-coverage prompt"
```

---

## Task 16.11: `task-init.sh` final step — log + smoke instructions; pre-commit hook deferred

Per spec, the pre-commit hook installer is **deferred to phase 19** (encrypt-init coupling). This phase:

- Logs `task-init enabled` (or `task-init noop` on idempotent re-run) via `scripts/log-append.sh -- task-init <message>` if log-append exists.
- Prints the smoke instructions: `Try: 'just capture "pick up groceries"'` and `(scanner / agenda — phase 17)`.

We track first-run vs re-run by checking whether `.awiki/task-count` existed before `step_state_files`. We capture that via a flag set early in `main`.

**Files:** Modify: `scripts/task-init.sh`, `tests/task_init_test.bats`.

- [ ] **Step 1: Append tests**

```bash
cat >> tests/task_init_test.bats <<'EOF'

@test "task-init prints smoke instructions on success" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"just capture"* ]]
  [[ "$output" == *"phase 17"* || "$output" == *"scanner"* ]]
}

@test "task-init reports first-run vs noop on rerun (when log-append present)" {
  # Provide a stub log-append and an empty log file so we can grep.
  cat > scripts/log-append.sh <<'STUB'
#!/usr/bin/env bash
shift  # drop --
echo "LOG|$*" >> "${AWIKI_LOG_FILE:-content/log.md}"
STUB
  chmod +x scripts/log-append.sh
  printf -- "---\ntype: log\n---\n" > content/log.md
  AWIKI_LOG_FILE="content/log.md" bash scripts/task-init.sh
  AWIKI_LOG_FILE="content/log.md" bash scripts/task-init.sh
  run grep 'task-init enabled' content/log.md
  [ "$status" -eq 0 ]
  run grep 'task-init noop' content/log.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/task_init_test.bats
```

Expected: 2 new tests fail.

- [ ] **Step 3: Update `main()` in `scripts/task-init.sh`**

Replace the existing `main()` body with:

```bash
main() {
  note "start"

  local first_run=1
  [[ -f "$TASK_COUNT" ]] && first_run=0

  step_pages
  step_wiki_md
  step_encryption
  step_config
  step_state_files

  # Logging (best-effort; no failure if log-append is absent).
  if [[ -x scripts/log-append.sh ]]; then
    if [[ $first_run -eq 1 ]]; then
      bash scripts/log-append.sh -- task-init "enabled" >/dev/null 2>&1 || true
    else
      bash scripts/log-append.sh -- task-init "noop" >/dev/null 2>&1 || true
    fi
  fi

  # Smoke instructions.
  cat <<HINTS
TASK-INIT|done

Smoke test:
  just capture "pick up groceries"
  cat content/inbox.md   # should show your line below the frontmatter

Scanner + agenda generation arrives in phase 17. Triage + recurrence
arrive in phase 18.

Pre-commit hook installer is deferred to phase 19 (encrypt-init coupling).
HINTS
}
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/task_init_test.bats
```

Expected: 21 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/task-init.sh tests/task_init_test.bats
git commit -m "feat(task): task-init final step — log + smoke instructions"
```

---

## Task 16.12: justfile recipes (`task-init`, `capture`) and commented stubs

Add a `# === task layer ===` section to the existing `justfile` with the two live recipes. The other task-layer recipes (`agenda`, `scan`, `triage`, `review`) ship in phases 17/18; we add them as commented stubs so reviewers can see the intended shape without the recipes accidentally erroring out before the scripts exist.

**Files:** Modify: `justfile`, `docs/just-help.txt`.

- [ ] **Step 1: Append the task-layer recipes to `justfile`**

Open `justfile` and after the final existing recipe block, append:

```just

# === task layer (phase 16) ===
task-init:
    bash scripts/task-init.sh

capture *text:
    bash scripts/capture.sh -- {{text}}

# Stubs land in later phases:
# scan:                                  # phase 17
#     bash scripts/action-scan.sh
# agenda:                                # phase 17
#     bash scripts/action-scan.sh
#     bash scripts/agenda.sh
# triage:                                # phase 18
#     bash scripts/triage.sh --interactive
# review:                                # phase 19
#     bash scripts/agenda.sh
#     bash scripts/lint.sh
#     bash scripts/review-status.sh
```

- [ ] **Step 2: Verify `just --list` includes the new recipes**

```bash
just --list | grep -E '(task-init|capture)'
```

Expected output:

```
    capture *text
    task-init
```

- [ ] **Step 3: Append docs to `docs/just-help.txt`**

```bash
cat >> docs/just-help.txt <<'EOF'

TASK LAYER (phase 16)
  just task-init                Enable the task layer in this wiki.
                                Idempotent — safe to re-run. Creates inbox,
                                projects/, contexts/, agenda/ system pages
                                and patches WIKI.md with the schema block.
  just capture "<text>"         Append a sanitized capture line to
                                content/inbox.md. Quote multi-word text.
                                Sanitization (full): rejects newlines /
                                control chars / leading-checkbox; truncates
                                > 2000 chars; neutralizes [[wikilinks]],
                                <!-- HTML comments -->, ^block-id-shaped
                                tokens.
                                Example: just capture "call dentist"
                                Reports applied rules to stderr as
                                SANITIZATION-APPLIED|<rule1>,<rule2>,...
EOF
```

- [ ] **Step 4: Commit**

```bash
git add justfile docs/just-help.txt
git commit -m "feat(task): add justfile recipes task-init + capture (phase 16)"
```

---

## Task 16.13: End-to-end smoke test (`task-init` → `capture`)

Ensures the full happy-path flow from a fresh clone works as a single shell sequence. We add a dedicated BATS file rather than mixing into `task_init_test.bats` to keep the smoke test isolated and easy to point reviewers at.

**Files:** Create: `tests/task_smoke_test.bats`.

- [ ] **Step 1: Write the smoke test**

```bash
cat > tests/task_smoke_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "smoke: task-init then capture writes a line into inbox" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/inbox.md ]

  run bash scripts/capture.sh -- "pick up groceries"
  [ "$status" -eq 0 ]

  run grep -E '^- 20[0-9]{2}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2} pick up groceries$' content/inbox.md
  [ "$status" -eq 0 ]

  # Frontmatter still intact.
  run head -1 content/inbox.md
  [[ "$output" == "---" ]]

  # State files exist.
  [ -f .awiki/task-count ]
  [ -f .awiki/last-review ]

  # Agenda pages exist with managed-region pair.
  [ -f content/agenda/next-actions.md ]
  run grep '^<!-- BEGIN managed-region -->$' content/agenda/next-actions.md
  [ "$status" -eq 0 ]

  # WIKI.md has the task-layer block.
  run grep '^<!-- BEGIN task-layer -->$' WIKI.md
  [ "$status" -eq 0 ]
}

@test "smoke: capture's sanitization fires when needed" {
  bash scripts/task-init.sh >/dev/null
  run bash scripts/capture.sh -- "see [[s-as-we-may-think]] later"
  [ "$status" -eq 0 ]
  [[ "$output" == *"wikilink-neutralized"* ]]
  run grep -F "[ [s-as-we-may-think] ]" content/inbox.md
  [ "$status" -eq 0 ]
}

@test "smoke: idempotent task-init followed by capture twice" {
  bash scripts/task-init.sh >/dev/null
  bash scripts/task-init.sh >/dev/null
  bash scripts/capture.sh -- "first"  >/dev/null
  bash scripts/capture.sh -- "second" >/dev/null
  run grep -c '^- ' content/inbox.md
  [[ "$output" == "2" ]]
}
EOF
```

- [ ] **Step 2: Run the smoke test**

```bash
bats tests/task_smoke_test.bats
```

Expected: 3 tests pass.

- [ ] **Step 3: Commit**

```bash
git add tests/task_smoke_test.bats
git commit -m "test(task): add task-init → capture end-to-end smoke test"
```

---

## Task 16.14: Phase-done checklist + branch merge

- [ ] **Step 1: Run the full BATS suite**

```bash
bats tests/
```

Expected: every test in the suite passes (including all prior phases' tests).

- [ ] **Step 2: Run the existing lint**

```bash
just lint
```

Expected: clean, or warnings only — no errors. (Phase 16 does NOT extend `lint.sh`; the new T1-T15 rule bodies arrive in phase 17/18. The current lint should still pass because `task-init.sh` does not introduce any frontmatter that violates v1 rules.)

- [ ] **Step 3: Run `check-deps`**

```bash
bash scripts/check-deps.sh
```

Expected: `OK|flock` line present (or `MISSING|flock|...` with the OS-specific install hint, in which case install `flock` and re-run).

- [ ] **Step 4: Walk the phase-done checklist**

Manually confirm each:

- [ ] `scripts/lib/lock.sh` exists; `awiki_lock_with` and `awiki_lock_shared` callable; exit 7 on contention; `AWIKI_LOCK_TIMEOUT_USER=30` and `AWIKI_LOCK_TIMEOUT_DEFERRED=180` exported.
- [ ] `scripts/lib/action-grammar.sh` exists; exposes `AWIKI_ACTION_LINE_RE`, `AWIKI_TAIL_KEY_RE`, `AWIKI_BLOCK_ID_RE`, `AWIKI_STATUS_MARKERS`, `awiki_grammar_parse_action`, `awiki_grammar_split_tail`. **Not yet imported** by any consumer (consumers ship in 17/18).
- [ ] `scripts/check-deps.sh` lists `flock` as a required dep with macOS + Linux install hints.
- [ ] `scripts/capture.sh` implements every row of the spec sanitization table (control-char reject, length truncate, wikilink neutralize, comment neutralize, block-id escape, leading-checkbox reject); prints `SANITIZATION-APPLIED|<rules>` + raw/final diff to stderr when any rule fires.
- [ ] `scripts/templates/wiki-task-layer.md` shipped with page-kind enum, system pages, inline-grammar table, and lint-rule names T1-T15.
- [ ] `scripts/task-init.sh` is per-step idempotent: `step_pages`, `step_wiki_md`, `step_encryption`, `step_config`, `step_state_files`. Re-running on a fully-enabled wiki produces no diffs.
- [ ] `task-init` creates: `content/inbox.md`, `content/projects/_index.md`, `content/contexts/_index.md`, `content/agenda/_index.md`, `content/agenda/{next-actions,today,waiting,someday,stuck-projects}.md` (each with empty managed-region pair), `content/agenda/review-log.md`, `content/contexts/{phone,errands,computer}.md`.
- [ ] `task-init` patches `WIKI.md` with the templated `<!-- BEGIN task-layer --> ... <!-- END task-layer -->` block; warns and skips on customised block.
- [ ] `task-init` extends `.awiki/config` with `AWIKI_AGENDA_AFTER_N=5` and `AWIKI_TASK_LAYER=on`, preserving existing user-set values.
- [ ] `task-init` creates `.awiki/last-review=<today>` and `.awiki/task-count=0` (only if missing).
- [ ] `task-init` detects `.gitattributes` git-crypt section and prompts to extend patterns to `content/inbox.md` and `content/agenda/**`; honours `AWIKI_TASK_INIT_ASSUME_YES=1` / `AWIKI_TASK_INIT_ASSUME_NO=1`.
- [ ] `task-init` logs `task-init enabled` on first run, `task-init noop` on subsequent runs (when `log-append.sh` exists).
- [ ] `task-init` prints smoke-test instructions ending with `just capture "..."`.
- [ ] `task-init` does NOT install a pre-commit hook (deferred to phase 19 per spec).
- [ ] `justfile` exposes `task-init` and `capture *text`; the four other task-layer recipes (`scan`, `agenda`, `triage`, `review`) appear as commented stubs with the correct phase number.
- [ ] `docs/just-help.txt` documents the two live recipes with the sanitization table summary.
- [ ] `.gitignore` ignores `.awiki/lock`, `.awiki/last-review`, `.awiki/task-count`, `.awiki/maps/`.
- [ ] `tests/task_init_test.bats`, `tests/capture_test.bats`, `tests/action_grammar_test.bats`, `tests/lock_test.bats`, `tests/task_smoke_test.bats` all pass.
- [ ] No emojis in any phase-16 file. No `Co-Authored-By` trailers in any phase-16 commit.
- [ ] All `--` flag terminators present in `bash scripts/*.sh` shell-out sites within `task-init.sh` and `capture.sh` (where they accept user input).
- [ ] No phase-17/18 features leaked in: no scanner, no agenda generator, no triage, no recurrence, no MCP server changes, no lint rule bodies, no pre-commit hook installer.

- [ ] **Step 5: Final cleanup commit (if needed)**

```bash
git status
# If anything is unstaged:
git add -p
git commit -m "fix: phase 16 final cleanup per phase-done checklist"
```

- [ ] **Step 6: Merge to main**

```bash
git checkout main
git merge --no-ff phase-16-task-schema-scaffold -m "feat: complete phase 16 task-layer schema + scaffold"
git branch -d phase-16-task-schema-scaffold
```

Expected: fast-forward refused (`--no-ff`); merge commit created on `main`.

- [ ] **Step 7: Tag (skip — phase 16 is mid-feature)**

Tagging waits for phase 19 (full task-layer ship).

---

## Self-Review Checklist

Walk this list once after Task 16.14 step 4 has been ticked off. Each item is scoped to phase-16 deliverables only.

- [ ] **Lib-only-this-phase split honored.** `scripts/lib/action-grammar.sh` and `scripts/lib/lock.sh` ship with their unit tests and **no consumer wiring**. No script under `scripts/` (other than the libraries themselves) sources either file in phase 16. Verify with `grep -r 'action-grammar.sh\|lib/lock.sh' scripts/ tests/` — only the libraries' own tests should appear.
- [ ] **Spec-table fidelity for `capture.sh`.** The six sanitization rules from the spec table are each covered by a dedicated bats case in `capture_test.bats`. Reject categories return exit 4. Replace categories return exit 0 with the rule name in `SANITIZATION-APPLIED|...`.
- [ ] **`task-init` per-step idempotence.** Running `task-init` twice in a row produces zero diffs in `content/`, `WIKI.md`, `.gitattributes`, and `.awiki/`. Add a manual check: `bash scripts/task-init.sh && git status -s; bash scripts/task-init.sh && git status -s` — second run should be byte-identical.
- [ ] **No phase 17/18/19 features leaked.** No scanner, no agenda generator, no triage, no recur, no MCP changes, no lint-rule bodies, no pre-commit hook installer. Verify by listing every new script: `ls scripts/ scripts/lib/ scripts/templates/`.
- [ ] **Lock library matches spec timeouts.** `AWIKI_LOCK_TIMEOUT_USER=30` and `AWIKI_LOCK_TIMEOUT_DEFERRED=180` match the spec "Concurrency & Atomicity" section verbatim. Exit 7 on contention matches.
- [ ] **Action-grammar regex covers all six status markers.** Test enumerates `[ ]`, `[/]`, `[?]`, `[>]`, `[x]`, `[-]`. The library does not silently accept `[X]` or `[*]`.
- [ ] **Block-ID regex matches phase-16 spec.** `^[a-z0-9]{3,16}` accepted; `~<digits>` recurrence-chain suffix accepted; tokens shorter than 3 or longer than 16 rejected.
- [ ] **Capture sanitization order: hard-rejections BEFORE replacements.** Control-char and checkbox-prefix-at-start rejections fire first; text is never partially mutated before a hard-reject is raised.
- [ ] **Encryption-coverage prompt non-interactive overrides.** `AWIKI_TASK_INIT_ASSUME_YES=1` and `AWIKI_TASK_INIT_ASSUME_NO=1` are honored; in their absence the script reads from stdin (interactive). Tests cover the `YES`/`NO` paths; the stdin path is exercised manually.
- [ ] **WIKI.md block reversal works.** Manually verify: copy `WIKI.md`, run `task-init`, then delete the block (`sed -i'' -e '/<!-- BEGIN task-layer -->/,/<!-- END task-layer -->/d' WIKI.md`) — `WIKI.md` matches the pre-init copy.
- [ ] **`flock` dep-check failure is OS-helpful.** `AWIKI_FAKE_MISSING=flock bash scripts/check-deps.sh` prints both an `apt install util-linux` hint (Linux) and a `brew install util-linux` hint (macOS) — matching the spec Dependencies & Compatibility note.
- [ ] **Marker syntax discipline.** `<!-- BEGIN task-layer -->` / `<!-- END task-layer -->` for `WIKI.md` block. `<!-- BEGIN managed-region -->` / `<!-- END managed-region -->` for the agenda placeholder pages. Never bare `BEGIN` / `END`.
- [ ] **No emojis, no co-author trailers.** Verify the commit log: `git log --oneline phase-16-task-schema-scaffold..main` (run before merge) and `git log -p --grep='Co-Authored-By' phase-16-task-schema-scaffold` should be empty.
- [ ] **All scripts use `set -euo pipefail` and `--` arg termination.** `head -2 scripts/task-init.sh scripts/capture.sh` shows the shebang + set line. All `bash scripts/log-append.sh ...` invocations include `--` before positional args.
- [ ] **Smoke test reproducible.** `bats tests/task_smoke_test.bats` passes from a clean checkout (`git stash && git checkout main && git checkout phase-16-task-schema-scaffold && bats tests/task_smoke_test.bats`).
- [ ] **Master plan / dependency table consistency.** Phase 17 (next) declares it depends on phase 16 — open `docs/superpowers/plans/2026-04-27-awiki-master-plan.md` if it exists and confirm row 16's deliverable list matches what shipped here.

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to Phase 17 (scanner + agenda generation).
