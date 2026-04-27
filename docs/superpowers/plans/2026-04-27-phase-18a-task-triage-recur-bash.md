# awiki Plan — Phase 18a: Task Layer — Triage + Recurrence (BASH side)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md) — sections "Capture + Triage Workflow", "Recurrence", and the T8-T13 entries in "Lint Rules Added".
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Sibling (Node MCP):** [`2026-04-27-phase-18b-task-triage-recur-mcp.md`](./2026-04-27-phase-18b-task-triage-recur-mcp.md) — written separately. Phase 18b extends `mcp/awiki-server` with `triage_inbox`, `triage_apply`, `list_actions`, `rebuild_agenda`, plus per-arg validation, capture sanitization, JSON-schema artifacts, and `tests/mcp_task_test.bats`. The bash deliverables in this file (18a) are independent of 18b — `triage.sh` is the cold-start fallback the MCP tool calls into via the same library entry-points, but the MCP server itself is the 18b deliverable.
**Depends on:** Phase 16 (`scripts/lib/action-grammar.sh`, `scripts/lib/lock.sh`, `scripts/log-append.sh` v1, `scripts/capture.sh`, `.awiki/task-count`, `.awiki/config`); Phase 17 (`scripts/action-scan.sh`, `scripts/agenda.sh`, recurrence-chain separator constant pinned in `scripts/lib/action-grammar.sh`, lint T1-T7 + T14 + T15, the agenda-deferred-enqueue path).
**Previous:** [Phase 17](./2026-04-27-phase-17-task-scanner-agenda.md)
**Next:** [Phase 18b](./2026-04-27-phase-18b-task-triage-recur-mcp.md), then [Phase 19 — review + polish](./2026-04-27-phase-19-task-review-polish.md).

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, bats-core 1.10+, `flock` (util-linux on Linux; brew `util-linux` on macOS), GNU coreutils (`date`, `sha1sum`/`shasum`, `mktemp`, `mv`, `awk`, `grep`, `sed`, `sort`, `comm`), `expect` (BATS uses it to drive interactive-mode tests; install hint added to dep-check). No new tool versions beyond phase 17.

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`. Pure bash 4+ for shell logic. No python helpers in this phase. Date arithmetic is hand-coded around `date -u` (BSD/GNU portable subset documented per script).
- Every consumer sources `scripts/lib/action-grammar.sh` (phase 16) for the canonical action-line regex, tail-metadata parser, status-marker enum, AND the recurrence-chain separator constant `AWIKI_RECUR_SEP` (`~` by default, `__` if the phase-17 spike fell back). Re-implementing the regex inline is a phase-blocking bug — fix by sourcing the lib instead.
- Every mutating script (`triage.sh` writes pages, removes inbox lines; `action-recur.sh` rewrites pages; `lint.sh --fix` rewrites pages) acquires `flock -x .awiki/lock` via `awiki_lock_with --timeout=30 -- ...` from `scripts/lib/lock.sh`. The deferred 180s `agenda.sh` rebuild path enqueued by `triage.sh` runs **after** the 30s lock is released — it picks the lock up again with the deferred profile. This matches the spec's "no nested invocation" rule.
- Slug regex `^[a-z0-9][a-z0-9-]{0,63}$` (no leading hyphen, no underscore). Project slugs additionally accept a single leading underscore (`_loose`, `_someday`) — regex `^[a-z0-9_][a-z0-9_-]{0,63}$`. Only project slugs may start with `_`.
- Block-ID regex `^[a-z0-9]{3,16}$` for hand-typed and lint-minted IDs; freshly minted IDs are exactly 8 chars. Recurrence-chain instance ID shape: `<base>${AWIKI_RECUR_SEP}<digits>` where `<digits>` is decimal, ≥ 2.
- All scripts handle Ctrl-C cleanly: a single `trap 'awiki_lock_release; exit 130' INT TERM` at top frees the lock and surfaces the original signal; the `triage.sh --interactive` walker additionally per-item resets state so the abort drops the in-flight item but leaves already-applied items intact.
- Inbox-line ID resolution: `triage.sh` re-reads the line at the supplied `<lineno>` and recomputes `sha1(line)[:10]`. If the recomputed hash does not match the hash embedded in the supplied id, exit 9 with a structured `stale_id:true` JSON line on stdout (so the MCP tool surface can re-emit the same shape).
- Lazy creation of `_loose.md` / `_someday.md`: the first `act` outcome with `project_slug=_loose` (or first `someday` outcome with `project_slug=_someday`, or any outcome that needs the catch-all) writes the page with `type: project, status: active` (or `status: someday`) frontmatter, an empty `## Open Actions` section, and the action line appended. Subsequent outcomes append to the existing file.
- Threshold lookup chain for the `agenda.sh` auto-rebuild trigger: `AWIKI_AGENDA_AFTER_N` env var → `.awiki/config` `AWIKI_AGENDA_AFTER_N=N` line → default `5`. Identical lookup chain to v1's `AWIKI_LINT_AFTER_N`. Implemented once in a `awiki_resolve_agenda_threshold` helper inside `triage.sh` (not the grammar lib — it is triage-specific).
- Lint output line format unchanged from phase 17: `LINT|<level>|<file>|<msg>` with rule code prefix (`T8: ...`, `T9: ...`). Final line `LINT-SUMMARY|errors=N|warnings=M|info=K`. Phase 18a adds T8-T13.
- Recurrence cap: `action-recur.sh` exits 6 (no write, structured stderr message) at chain `≥200`. T12 lint warns at 150, errors at 200, **independent** of action-recur's refuse-to-emit. The two checks are deliberately decoupled (one prevents new growth via the only emit path; the other surfaces violations created by hand edits).
- Date arithmetic for `every:`:
  - `Nd` and `daily` and `Nw` and `weekly`: pure-day UTC calendar arithmetic via `date -u -d "YYYY-MM-DD + N days"` (GNU) with a portable BSD fallback (`date -u -j -f "%Y-%m-%d" -v +Nd "$base"`); the lib helper `awiki_date_add_days` already shipped in phase 16 — reuse, do not reimplement.
  - `Nm` and `monthly`: month-add with last-day clamp. Hand-coded inside `action-recur.sh` (helper `awiki_date_add_months_clamped` is added in this phase to `scripts/lib/action-grammar.sh` so other future consumers — agenda T10 surfaces, review-status — can reuse). `2026-01-31 + 1m → 2026-02-28`. `2026-03-15 + 1m → 2026-04-15`. Never produces an invalid calendar date.
- Mid-chain `every:` change is honored: `action-recur.sh` reads the interval from the **completed line being processed**, NOT from the chain head. Documented in WIKI.md (phase 17 already added the line; verify in Task 18a.10).
- `--dry-run` for `action-recur.sh`: prints a unified diff (`diff -u` against the would-be temp file) to stdout, exits 0, makes no changes. Used by lint and `--fix` workflows.
- Log-poisoning protection: `scripts/log-append.sh` (phase 16 v1) is extended to neutralize `|`, `\n`, `\r` in caller-supplied substrings. Replacements: `|` → `_`, `\n` → ` `, `\r` → ` `. The caller's intent is preserved as far as possible; the goal is to keep the log line parseable as a single `|`-delimited record.
- Commit after every task. Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- TDD discipline: failing fixture+test → run-it-fails → implement → run-it-passes → commit. One outcome (or rule) = one task with its own bats case.
- Branch per phase. Merge to `main` only after `bats tests/ && just lint` are clean.
- No emojis. No `Co-Authored-By` trailers. No "Generated with Claude Code" lines.

---

**Deliverable:** `scripts/triage.sh` (positional CLI + `--interactive` walker; per-item-atomic; regex-validated prompts; slug autocomplete; flock-protected; threshold-resolved auto-rebuild enqueue; inbox-line ID TOCTOU re-verify; lazy `_loose`/`_someday` creation), `scripts/action-recur.sh` (idempotent re-emit, `every:` arithmetic with last-day month clamp, mid-chain interval change, refuse-to-emit at chain ≥200, `--dry-run`, flock-protected), `scripts/lint.sh` extended with semantic rules **T8-T13** (T8 with the `_loose`/`_someday` exemption, T9 14-day staleness, T10 overdue, T11 90-day someday staleness with the catch-all caveat, T12 chain cap at 150/200 independent of refuse-to-emit, T13 unused-context info), `scripts/log-append.sh` poisoning protection extended in-place, BATS suites `tests/triage_test.bats`, `tests/recur_test.bats`, `tests/log_append_poisoning_test.bats`, plus extensions to the existing `tests/lint_task_test.bats` for T8-T13. Scope split: this plan does **not** touch `mcp/awiki-server/`. The MCP server extension, JSON-schema artifacts, MCP per-arg validation, MCP capture sanitization, MCP `triage_apply` return shape, and `tests/mcp_task_test.bats` ship in **phase 18b** (sibling plan).

**Branch:** `phase-18a-task-triage-recur-bash`

---

## Task outline

| # | Task | TDD test | Files |
|---|------|----------|-------|
| 18a.1 | Branch + scope check | — | (no source changes) |
| 18a.2 | `log-append.sh` poisoning protection (extend v1) | `tests/log_append_poisoning_test.bats` | `scripts/log-append.sh` |
| 18a.3 | `action-recur.sh` skeleton + arg parsing + `--dry-run` | `tests/recur_test.bats` (skeleton case) | `scripts/action-recur.sh` |
| 18a.4 | `action-recur.sh` `every:` arithmetic incl. month-clamp | `tests/recur_test.bats` (clamp case + Nd/Nw) | `scripts/action-recur.sh`, `scripts/lib/action-grammar.sh` (helper) |
| 18a.5 | `action-recur.sh` idempotent re-emit + chain-cap refuse-to-emit | `tests/recur_test.bats` (idempotence + cap) | `scripts/action-recur.sh` |
| 18a.6 | `action-recur.sh` mid-chain `every:` change + separator round-trip | `tests/recur_test.bats` (mid-chain + sep) | `scripts/action-recur.sh` |
| 18a.7 | `triage.sh` skeleton + positional CLI + flock | `tests/triage_test.bats` (positional + lock) | `scripts/triage.sh` |
| 18a.8 | `triage.sh` seven outcomes (state machine + lazy `_loose`/`_someday`) | `tests/triage_test.bats` (per-outcome × 7) | `scripts/triage.sh` |
| 18a.9 | `triage.sh` inbox-line ID TOCTOU re-verify | `tests/triage_test.bats` (stale-id) | `scripts/triage.sh` |
| 18a.10 | `triage.sh` `--interactive` walker (regex-validated prompts, slug autocomplete) | `tests/triage_test.bats` (expect-driven) | `scripts/triage.sh` |
| 18a.11 | `triage.sh` threshold-resolved auto-rebuild enqueue | `tests/triage_test.bats` (threshold) | `scripts/triage.sh` |
| 18a.12 | Lint T8 — no-next-action, with `_loose`/`_someday` exemption | `tests/lint_task_test.bats` (T8) | `scripts/lint.sh` |
| 18a.13 | Lint T9 — waiting-stale 14d | `tests/lint_task_test.bats` (T9) | `scripts/lint.sh` |
| 18a.14 | Lint T10 — overdue | `tests/lint_task_test.bats` (T10) | `scripts/lint.sh` |
| 18a.15 | Lint T11 — stale-someday 90d (catch-all caveat documented) | `tests/lint_task_test.bats` (T11) | `scripts/lint.sh`, WIKI.md |
| 18a.16 | Lint T12 — recur-chain-cap (independent of refuse-to-emit) | `tests/lint_task_test.bats` (T12) | `scripts/lint.sh` |
| 18a.17 | Lint T13 — context-unused (info) | `tests/lint_task_test.bats` (T13) | `scripts/lint.sh` |
| 18a.18 | Justfile recipe `triage` + `docs/just-help.txt` updates | manual smoke | `justfile`, `docs/just-help.txt` |
| 18a.19 | Smoke + self-review + merge | `tests/task_smoke_test.bats` extension | (no source changes) |

---

## Task 18a.1: Branch + scope check

**Files:** (no source changes; verifies preconditions and creates the working branch).

- [ ] **Step 1: Verify prerequisites**

```bash
git status -s
git log --oneline | head -5
ls scripts/lib/action-grammar.sh scripts/lib/lock.sh scripts/log-append.sh scripts/capture.sh
ls scripts/action-scan.sh scripts/agenda.sh
ls .awiki/config 2>/dev/null && grep -E '^AWIKI_TASK_LAYER=' .awiki/config
ls tests/action_scan_test.bats tests/agenda_test.bats tests/lint_task_test.bats
grep -E '^AWIKI_RECUR_SEP=' scripts/lib/action-grammar.sh
```

Expected: working tree clean; phase 17 merged; the listed scripts exist; `.awiki/config` shows `AWIKI_TASK_LAYER=on` (else phase 16 was not run on this checkout); the phase-17 spike pinned `AWIKI_RECUR_SEP` to either `~` or `__` in `scripts/lib/action-grammar.sh`. If `AWIKI_RECUR_SEP` is missing, stop — phase 17 did not complete its spike artifact and this plan cannot proceed.

- [ ] **Step 2: Branch**

```bash
git checkout main
git pull --ff-only
git checkout -b phase-18a-task-triage-recur-bash
```

Expected: `Switched to a new branch 'phase-18a-task-triage-recur-bash'`.

- [ ] **Step 3: Confirm `flock` and `expect` are present**

```bash
command -v flock && flock --version
command -v expect && expect -v
```

Expected: both resolve. If `expect` is missing on the operator's box, the BATS interactive-mode tests in Task 18a.10 will skip. Add to dep-check at the end of this phase (see Task 18a.18).

- [ ] **Step 4: Read the spec sections in scope**

Open `docs/superpowers/specs/2026-04-27-task-layer-design.md`, focused read of "Capture + Triage Workflow", "Recurrence", and the T8-T13 rows in the "Lint Rules Added" table. Skim "Concurrency & Atomicity" once more — Task 18a.7 and 18a.11 lean on the deferred-enqueue rule.

---

## Task 18a.2: `log-append.sh` poisoning protection

**Files:** Modify: `scripts/log-append.sh`. Test: `tests/log_append_poisoning_test.bats` (new).

The phase-16 v1 of `log-append.sh` writes `<timestamp>|<event>|<message>` to `.awiki/log`. Caller-supplied substrings (slugs, page titles, free-form free text from `triage.sh`) MUST be neutralized: `|` breaks the field separator, `\n`/`\r` break the line terminator. Phase 18a extends the script in-place; it must remain backward-compatible with phase-16 callers.

- [ ] **Step 1: Failing test FIRST**

Create `tests/log_append_poisoning_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  TMP="$(mktemp -d)"
  export AWIKI_REPO_ROOT="$TMP"
  mkdir -p "$TMP/.awiki" "$TMP/scripts"
  cp "$BATS_TEST_DIRNAME/../scripts/log-append.sh" "$TMP/scripts/log-append.sh"
  cp "$BATS_TEST_DIRNAME/../scripts/lib/lock.sh" "$TMP/scripts/lib/lock.sh" 2>/dev/null || mkdir -p "$TMP/scripts/lib"
  cp "$BATS_TEST_DIRNAME/../scripts/lib/lock.sh" "$TMP/scripts/lib/lock.sh"
  cd "$TMP"
}

teardown() {
  rm -rf "$TMP"
}

@test "log-append neutralizes pipe in caller substring" {
  run bash scripts/log-append.sh triage "act | _loose | phantom"
  [ "$status" -eq 0 ]
  # Exactly three pipes: the two field separators plus zero from the message.
  pipe_count=$(grep -c '|' .awiki/log)
  [ "$pipe_count" -eq 1 ]   # one log line
  line=$(cat .awiki/log)
  # Field count should be 3 (timestamp|event|message); pipes inside the message replaced with '_'.
  field_count=$(awk -F'|' '{print NF}' <<<"$line")
  [ "$field_count" -eq 3 ]
  # Last field carries the neutralized message.
  echo "$line" | grep -q 'act _ _loose _ phantom'
}

@test "log-append neutralizes embedded LF in caller substring" {
  # Pass an explicit \n via $'...'.
  run bash scripts/log-append.sh triage $'first\nsecond'
  [ "$status" -eq 0 ]
  # Log file must contain exactly one line.
  line_count=$(wc -l < .awiki/log)
  [ "$line_count" -eq 1 ]
  cat .awiki/log | grep -q 'first second'
}

@test "log-append neutralizes embedded CR in caller substring" {
  run bash scripts/log-append.sh triage $'first\rsecond'
  [ "$status" -eq 0 ]
  line_count=$(wc -l < .awiki/log)
  [ "$line_count" -eq 1 ]
  cat .awiki/log | grep -q 'first second'
}

@test "log-append leaves clean message untouched" {
  run bash scripts/log-append.sh triage "do-now / dentist / phone"
  [ "$status" -eq 0 ]
  cat .awiki/log | grep -q 'do-now / dentist / phone'
}
```

Run:

```bash
bats tests/log_append_poisoning_test.bats
```

Expected: 4 failures (the v1 script writes raw substrings).

- [ ] **Step 2: Implement neutralization**

Edit `scripts/log-append.sh`. The v1 has a function `awiki_log_append <event> <message>` (or similar — match the existing name). Add a sanitize helper called once on each caller substring:

```bash
# Neutralize log-poisoning characters in caller-supplied substrings.
# - '|' -> '_'   (would break the field separator)
# - '\n' -> ' '  (would break the line terminator)
# - '\r' -> ' '  (CR alone or before LF is also a line terminator under some readers)
# Keep tabs as-is; the log format is pipe-delimited, tab is fine inside a field.
awiki_log_sanitize() {
  local s="$1"
  # bash 4 parameter expansion handles literal substitution. For \n and \r we
  # use printf to materialize the chars then sed to replace.
  # Order: pipe first, then CR, then LF (LF replacement keeps the rest of the
  # message on one line).
  s="${s//|/_}"
  # tr replaces single bytes; safe and portable.
  s="$(printf '%s' "$s" | tr '\n\r' '  ')"
  printf '%s' "$s"
}
```

Then in the existing `awiki_log_append` (or whatever the v1 entry-point is named) wrap each caller substring before composing the line. The v1 typically composes:

```bash
printf '%s|%s|%s\n' "$ts" "$event" "$message" >> "$AWIKI_REPO_ROOT/.awiki/log"
```

Replace with:

```bash
local clean_event clean_message
clean_event="$(awiki_log_sanitize "$event")"
clean_message="$(awiki_log_sanitize "$message")"
printf '%s|%s|%s\n' "$ts" "$clean_event" "$clean_message" >> "$AWIKI_REPO_ROOT/.awiki/log"
```

Note: `$ts` itself is generated internally (via `date -u +%Y-%m-%dT%H:%M:%SZ`) and is trusted; it does not pass through the sanitizer.

- [ ] **Step 3: Re-run the test**

```bash
bats tests/log_append_poisoning_test.bats
```

Expected: 4 passes.

- [ ] **Step 4: Re-run the full phase-16 log-append test suite**

```bash
bats tests/log_append_test.bats   # if it exists from phase 16; skip if not
bats tests/                        # broader regression sanity
```

Expected: all green. Phase-16 callers passing clean strings see no behavioral change; the sanitize is idempotent on clean input.

- [ ] **Step 5: Commit**

```bash
git add scripts/log-append.sh tests/log_append_poisoning_test.bats
git commit -m "feat(log): neutralize pipe/LF/CR in caller substrings (poisoning protection)

Phase 18a: triage.sh and action-recur.sh log slugs, action text, and
free-form messages — any of which may contain '|', '\n', or '\r' and
break the pipe-delimited single-line log format. log-append.sh now
sanitizes each caller-supplied substring at the boundary: '|' -> '_',
'\n' -> ' ', '\r' -> ' '. Internal-only fields (timestamps) are not
touched. Backward-compatible: callers passing clean strings see no
change. Adds BATS coverage for pipe, LF, CR, and a clean control case."
```

---

## Task 18a.3: `action-recur.sh` skeleton + arg parsing + `--dry-run`

**Files:** Create: `scripts/action-recur.sh`. Test: `tests/recur_test.bats` (new file; this task adds the skeleton and the `--dry-run` happy-path case).

The recur script processes one page at a time. Invocation forms:

- `action-recur.sh <page-path>` — scans the page for `[x] every:...` lines, re-emits open copies for any whose computed-next-due is not already present in the chain.
- `action-recur.sh --dry-run <page-path>` — prints a unified diff of the would-be changes; makes no writes.
- `action-recur.sh --all` — iterates every page that contains at least one `[x] every:...` line per the current `actions.tsv`.

This task lays the skeleton and `--dry-run`. Arithmetic, idempotence, and cap come in later tasks.

- [ ] **Step 1: Failing test FIRST**

Create `tests/recur_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  TMP="$(mktemp -d)"
  export AWIKI_REPO_ROOT="$TMP"
  mkdir -p "$TMP/.awiki/maps" "$TMP/content/projects" "$TMP/scripts/lib"
  cp "$BATS_TEST_DIRNAME/../scripts/action-recur.sh" "$TMP/scripts/action-recur.sh" 2>/dev/null || true
  cp "$BATS_TEST_DIRNAME/../scripts/lib/action-grammar.sh" "$TMP/scripts/lib/action-grammar.sh"
  cp "$BATS_TEST_DIRNAME/../scripts/lib/lock.sh" "$TMP/scripts/lib/lock.sh"
  cp "$BATS_TEST_DIRNAME/../scripts/log-append.sh" "$TMP/scripts/log-append.sh"
  touch "$TMP/.awiki/log"
  cd "$TMP"
}

teardown() {
  rm -rf "$TMP"
}

# Helper: write a fixture project page with one completed weekly action.
seed_weekly_done_a05() {
  cat > content/projects/garden.md <<'EOF'
---
title: "Garden"
date: 2026-04-27
last_updated: 2026-05-04
type: project
status: active
outcome: "Healthy plants."
tags: [home]
draft: false
---

## Open Actions

## Done

- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  # Minimal actions.tsv for the script to consult — single chain head, no instances yet.
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/garden.md	14	home		    	1w	2026-05-04				garden	public
EOF
}

@test "action-recur.sh --dry-run prints unified diff and writes nothing" {
  seed_weekly_done_a05
  run bash scripts/action-recur.sh --dry-run content/projects/garden.md
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '^---'
  echo "$output" | grep -q '^+++'
  echo "$output" | grep -q '^+- \[ \] water plants @home every:1w due:2026-05-11'
  # Page on disk unchanged.
  grep -c '^- \[ \]' content/projects/garden.md | grep -qx 0
}
```

Run:

```bash
bats tests/recur_test.bats -f "dry-run prints unified diff"
```

Expected: failure — script does not exist yet.

- [ ] **Step 2: Create the script skeleton**

Create `scripts/action-recur.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# action-recur.sh — re-emit open copies of completed [x] every:... actions.
#
# Idempotent. Reads actions.tsv (built under lock by action-scan.sh) for chain
# state. Emits each instance immediately above its [x] predecessor on the same
# page. Uses temp-file + atomic rename for writes. Refuses to emit at chain
# >=200 (exit 6). Honors AWIKI_RECUR_SEP from action-grammar.sh.

# shellcheck source=lib/action-grammar.sh
source "$(dirname "$0")/lib/action-grammar.sh"
# shellcheck source=lib/lock.sh
source "$(dirname "$0")/lib/lock.sh"

usage() {
  cat <<EOF
Usage:
  action-recur.sh [--dry-run] <page-path>
  action-recur.sh [--dry-run] --all
  action-recur.sh -h | --help
EOF
}

main() {
  local dry_run=0
  local all=0
  local target=""

  while [[ $# -gt 0 ]]; do
    case "$1" in
      --dry-run) dry_run=1; shift ;;
      --all)     all=1;     shift ;;
      -h|--help) usage; exit 0 ;;
      --)        shift; break ;;
      -*)        echo "action-recur: unknown flag: $1" >&2; usage >&2; exit 2 ;;
      *)         target="$1"; shift ;;
    esac
  done

  if [[ "$all" -eq 1 && -n "$target" ]]; then
    echo "action-recur: --all and <page-path> are mutually exclusive" >&2
    exit 2
  fi
  if [[ "$all" -eq 0 && -z "$target" ]]; then
    usage >&2
    exit 2
  fi

  if [[ "$all" -eq 1 ]]; then
    # Discover pages with completed every: lines from actions.tsv.
    local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
    if [[ ! -f "$actions_tsv" ]]; then
      echo "action-recur: actions.tsv missing; run action-scan.sh first" >&2
      exit 3
    fi
    # Column layout matches phase 17. status=x AND every is non-empty.
    awk -F'\t' 'NR>1 && $2=="x" && $11!="" { print $4 }' "$actions_tsv" \
      | sort -u \
      | while IFS= read -r page; do
          recur_one_page "$page" "$dry_run"
        done
    return 0
  fi

  recur_one_page "$target" "$dry_run"
}

# recur_one_page <page-path> <dry-run-flag>
recur_one_page() {
  local page="$1"
  local dry="$2"

  if [[ ! -f "$page" ]]; then
    echo "action-recur: page not found: $page" >&2
    exit 3
  fi

  if [[ "$dry" -eq 0 ]]; then
    awiki_lock_with --timeout=30 -- recur_one_page_locked "$page"
  else
    # Dry-run reads only; shared lock is sufficient. The diff output goes to
    # stdout for the caller to inspect.
    awiki_lock_shared --timeout=30 -- recur_one_page_diff "$page"
  fi
}

# Compute the would-be page contents into a temp file and emit `diff -u`.
recur_one_page_diff() {
  local page="$1"
  local tmp
  tmp="$(mktemp "${page}.tmp.XXXXXX")"
  trap 'rm -f "$tmp"' RETURN
  recur_compute_new_contents "$page" > "$tmp"
  # If no change, exit cleanly with no output.
  if cmp -s "$page" "$tmp"; then
    return 0
  fi
  diff -u "$page" "$tmp"
}

recur_one_page_locked() {
  local page="$1"
  local tmp
  tmp="$(mktemp "${page}.tmp.XXXXXX")"
  recur_compute_new_contents "$page" > "$tmp"
  if cmp -s "$page" "$tmp"; then
    rm -f "$tmp"
    return 0
  fi
  mv "$tmp" "$page"
  bash "$(dirname "$0")/log-append.sh" recur "page=$page"
}

# Phase 18a.4 onward: real arithmetic + chain logic. Skeleton stub:
recur_compute_new_contents() {
  local page="$1"
  cat "$page"   # no-op until task 18a.4
}

main "$@"
```

```bash
chmod +x scripts/action-recur.sh
```

- [ ] **Step 3: Run the dry-run test (still failing)**

```bash
bats tests/recur_test.bats -f "dry-run prints unified diff"
```

Expected: failure — `recur_compute_new_contents` is a no-op, so `cmp -s` reports identical and `diff -u` prints nothing. The test asserts `+- [ ] water plants ...` which won't appear.

(That failure is expected and is exactly why this task is split from 18a.4. Leave the dry-run test marked `# TODO 18a.4` if the BATS runner blocks on it; alternatively gate it behind `skip` until task 18a.4 implements the arithmetic.)

- [ ] **Step 4: Add a smoke-only assertion that the skeleton exits cleanly**

Add a separate bats test that the skeleton parses args without crashing:

```bash
@test "action-recur.sh --help prints usage and exits 0" {
  run bash scripts/action-recur.sh --help
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "Usage:"
}

@test "action-recur.sh rejects unknown flag with exit 2" {
  run bash scripts/action-recur.sh --bogus content/projects/garden.md
  [ "$status" -eq 2 ]
}
```

Run:

```bash
bats tests/recur_test.bats -f "help prints usage|rejects unknown flag"
```

Expected: 2 passes.

- [ ] **Step 5: Commit**

```bash
git add scripts/action-recur.sh tests/recur_test.bats
git commit -m "feat(recur): add action-recur.sh skeleton with arg parsing and dry-run scaffold

Phase 18a: lays the script structure for the recurrence re-emitter.
Real arithmetic (every:Nd / Nw / Nm with month-clamp), idempotence,
chain-cap refuse-to-emit, and mid-chain interval handling land in
follow-up tasks. Skeleton wires --dry-run, --all, single-page mode,
flock acquisition (exclusive for writes, shared for diffs), and
temp-file + atomic rename for writes. Honors AWIKI_RECUR_SEP from
the phase-17 spike outcome. Tests cover --help and unknown-flag
rejection; the dry-run end-to-end test is parked until task 18a.4."
```

---

## Task 18a.4: `action-recur.sh` `every:` arithmetic incl. month-clamp

**Files:** Modify: `scripts/action-recur.sh`, `scripts/lib/action-grammar.sh` (add `awiki_date_add_months_clamped` helper). Test: `tests/recur_test.bats` (clamp + Nd/Nw cases).

The interval grammar:

| Token | Semantics |
|-------|-----------|
| `Nd` | `done + N days` (UTC) |
| `daily` | equivalent to `1d` |
| `Nw` | `done + 7N days` (UTC) |
| `weekly` | equivalent to `1w` |
| `Nm` | `done + N months`, clamped to last day of target month if target day is invalid |
| `monthly` | equivalent to `1m` |

- [ ] **Step 1: Failing test first — month clamp**

Append to `tests/recur_test.bats`:

```bash
@test "action-recur: every:1m clamps 2026-01-31 to 2026-02-28" {
  mkdir -p content/projects
  cat > content/projects/billing.md <<'EOF'
---
title: "Billing"
type: project
status: active
last_updated: 2026-01-31
draft: false
---

## Done

- [x] file VAT @computer every:1m due:2026-01-31 done:2026-01-31 ^v01
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
v01	x	file VAT	content/projects/billing.md	10	computer		    	1m	2026-01-31				billing	public
EOF
  run bash scripts/action-recur.sh content/projects/billing.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] file VAT @computer every:1m due:2026-02-28 \^v01~2$' content/projects/billing.md
}

@test "action-recur: every:1m on 2026-03-15 -> 2026-04-15 (no clamp)" {
  mkdir -p content/projects
  cat > content/projects/billing.md <<'EOF'
---
title: "Billing"
type: project
status: active
last_updated: 2026-03-15
draft: false
---

## Done

- [x] file VAT @computer every:1m due:2026-03-15 done:2026-03-15 ^v02
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
v02	x	file VAT	content/projects/billing.md	10	computer		    	1m	2026-03-15				billing	public
EOF
  run bash scripts/action-recur.sh content/projects/billing.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] file VAT @computer every:1m due:2026-04-15 \^v02~2$' content/projects/billing.md
}

@test "action-recur: every:1w on 2026-05-04 -> 2026-05-11" {
  seed_weekly_done_a05
  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] water plants @home every:1w due:2026-05-11 \^a05~2$' content/projects/garden.md
}

@test "action-recur: every:3d on 2026-04-27 -> 2026-04-30" {
  mkdir -p content/projects
  cat > content/projects/study.md <<'EOF'
---
title: "Study"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] review flashcards @computer every:3d due:2026-04-27 done:2026-04-27 ^s01
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
s01	x	review flashcards	content/projects/study.md	10	computer		    	3d	2026-04-27				study	public
EOF
  run bash scripts/action-recur.sh content/projects/study.md
  [ "$status" -eq 0 ]
  grep -q '^- \[ \] review flashcards @computer every:3d due:2026-04-30 \^s01~2$' content/projects/study.md
}
```

Run:

```bash
bats tests/recur_test.bats -f "every:"
```

Expected: 4 failures (the skeleton is a no-op).

- [ ] **Step 2: Add `awiki_date_add_months_clamped` to `scripts/lib/action-grammar.sh`**

Append (the lib already exports `awiki_date_add_days` from phase 16):

```bash
# awiki_date_add_months_clamped <YYYY-MM-DD> <N>
# Returns YYYY-MM-DD that is N months after the input, with last-day clamp.
# 2026-01-31 + 1 -> 2026-02-28
# 2026-01-31 + 2 -> 2026-03-31
# 2026-03-15 + 1 -> 2026-04-15
# Pure-bash arithmetic; does not shell out to `date` for the clamp because
# `date -d "2026-01-31 + 1 month"` returns "2026-03-03" on GNU (overflow,
# not clamp), which is the wrong semantic.
awiki_date_add_months_clamped() {
  local in="$1"; local n="$2"
  local y m d
  IFS=- read -r y m d <<<"$in"
  # Strip leading zeros — bash treats "08" / "09" as invalid octal under arithmetic.
  y=$((10#$y)); m=$((10#$m)); d=$((10#$d))
  m=$((m + n))
  while [[ "$m" -gt 12 ]]; do
    m=$((m - 12)); y=$((y + 1))
  done
  while [[ "$m" -lt 1 ]]; do
    m=$((m + 12)); y=$((y - 1))
  done
  # Last day of target month.
  local last
  case "$m" in
    1|3|5|7|8|10|12) last=31 ;;
    4|6|9|11)        last=30 ;;
    2)
      # Gregorian leap-year rule.
      if (( (y % 4 == 0 && y % 100 != 0) || (y % 400 == 0) )); then
        last=29
      else
        last=28
      fi
      ;;
  esac
  if [[ "$d" -gt "$last" ]]; then d="$last"; fi
  printf '%04d-%02d-%02d' "$y" "$m" "$d"
}

# awiki_recur_compute_due <done-date> <every-token>
# Dispatches Nd / Nw / Nm / daily / weekly / monthly. Echoes YYYY-MM-DD.
# Exits 4 on bad token.
awiki_recur_compute_due() {
  local done_date="$1"; local every="$2"
  local n unit
  case "$every" in
    daily)   awiki_date_add_days "$done_date" 1; return 0 ;;
    weekly)  awiki_date_add_days "$done_date" 7; return 0 ;;
    monthly) awiki_date_add_months_clamped "$done_date" 1; return 0 ;;
  esac
  if [[ "$every" =~ ^([0-9]+)([dwm])$ ]]; then
    n="${BASH_REMATCH[1]}"; unit="${BASH_REMATCH[2]}"
    case "$unit" in
      d) awiki_date_add_days "$done_date" "$n" ;;
      w) awiki_date_add_days "$done_date" "$((n * 7))" ;;
      m) awiki_date_add_months_clamped "$done_date" "$n" ;;
    esac
    return 0
  fi
  echo "awiki_recur_compute_due: bad every: '$every'" >&2
  return 4
}
```

Add a focused unit test for the helper itself in `tests/lib_grammar_test.bats` (extend the existing phase-16 file):

```bash
@test "awiki_date_add_months_clamped: 2026-01-31 + 1m = 2026-02-28" {
  source scripts/lib/action-grammar.sh
  run awiki_date_add_months_clamped 2026-01-31 1
  [ "$status" -eq 0 ]
  [ "$output" = "2026-02-28" ]
}

@test "awiki_date_add_months_clamped: 2026-01-31 + 13m = 2027-02-28" {
  source scripts/lib/action-grammar.sh
  run awiki_date_add_months_clamped 2026-01-31 13
  [ "$status" -eq 0 ]
  [ "$output" = "2027-02-28" ]
}

@test "awiki_date_add_months_clamped: 2024-01-31 + 1m = 2024-02-29 (leap)" {
  source scripts/lib/action-grammar.sh
  run awiki_date_add_months_clamped 2024-01-31 1
  [ "$status" -eq 0 ]
  [ "$output" = "2024-02-29" ]
}

@test "awiki_recur_compute_due: weekly = +7d" {
  source scripts/lib/action-grammar.sh
  run awiki_recur_compute_due 2026-05-04 weekly
  [ "$status" -eq 0 ]
  [ "$output" = "2026-05-11" ]
}
```

Run:

```bash
bats tests/lib_grammar_test.bats -f "awiki_date_add_months_clamped|awiki_recur_compute_due"
```

Expected: 4 passes.

- [ ] **Step 3: Implement `recur_compute_new_contents` in `action-recur.sh`**

Replace the no-op stub with the real engine. The strategy:

1. Read the page line-by-line; for each `[x] every:...` line, parse the tail metadata (use the lib's `awiki_parse_action_tail`), compute the next due, build the open-copy line.
2. Emit the open-copy line **immediately above** the `[x]` line in the output stream.
3. Skip emit if the next-instance ID already exists in the chain (idempotence — covered in Task 18a.5).

Replace `recur_compute_new_contents` with:

```bash
recur_compute_new_contents() {
  local page="$1"
  awiki_recur_emit_pass "$page"
}

# Emit transformed page contents on stdout. One pass; preserves all non-action
# content verbatim. For each [x] every:... line, prepends an open instance
# line if not already present in the chain.
awiki_recur_emit_pass() {
  local page="$1"

  # First, build the set of existing instance suffixes per chain on this page.
  # Format: lines of "<base>\t<n>" for every action line whose ^id matches
  # the chain shape <base><sep><digits>.
  local chains
  chains="$(awiki_collect_chain_state "$page")"

  # Stream the page; emit open-copy above each [x] every:... line.
  local line
  while IFS= read -r line || [[ -n "$line" ]]; do
    if awiki_is_completed_recurring_action "$line"; then
      local base every done_date next_due next_n new_id new_line
      base="$(awiki_extract_chain_base "$line")"
      every="$(awiki_extract_tail_key "$line" every)"
      done_date="$(awiki_extract_tail_key "$line" done)"
      [[ -z "$done_date" ]] && done_date="$(date -u +%Y-%m-%d)"
      next_due="$(awiki_recur_compute_due "$done_date" "$every")"
      next_n="$(awiki_recur_next_instance_n "$chains" "$base")"
      # Idempotence: if next_n already exists, emit nothing extra.
      if awiki_recur_chain_has "$chains" "$base" "$next_n"; then
        printf '%s\n' "$line"
        continue
      fi
      # Cap check (full body in Task 18a.5).
      if [[ "$next_n" -ge 200 ]]; then
        echo "action-recur: chain ^${base} reached 200 instances; refusing to emit (exit 6)" >&2
        exit 6
      fi
      new_id="${base}${AWIKI_RECUR_SEP}${next_n}"
      new_line="$(awiki_build_open_copy "$line" "$next_due" "$new_id")"
      printf '%s\n' "$new_line"
      printf '%s\n' "$line"
      # Update local chain state for any subsequent [x] in same chain on this page.
      chains="${chains}"$'\n'"${base}"$'\t'"${next_n}"
    else
      printf '%s\n' "$line"
    fi
  done < "$page"
}
```

Then add the supporting helpers to `scripts/lib/action-grammar.sh`:

```bash
# Detect a [x] action line that carries an every: tail key.
awiki_is_completed_recurring_action() {
  local l="$1"
  [[ "$l" =~ ^-\ \[x\]\  ]] || return 1
  [[ "$l" =~ \ every:[A-Za-z0-9]+ ]]
}

# Extract the chain base of an action line — the portion of ^id before the
# AWIKI_RECUR_SEP, or the full id if no separator is present.
awiki_extract_chain_base() {
  local l="$1"
  local id
  if [[ "$l" =~ \^([a-z0-9${AWIKI_RECUR_SEP}]+)[[:space:]]*$ ]]; then
    id="${BASH_REMATCH[1]}"
    # Strip the suffix.
    if [[ "$id" == *"${AWIKI_RECUR_SEP}"* ]]; then
      printf '%s' "${id%%"${AWIKI_RECUR_SEP}"*}"
    else
      printf '%s' "$id"
    fi
  fi
}

# Extract a tail key value ("every", "done", "due") from an action line.
# Returns empty if absent.
awiki_extract_tail_key() {
  local l="$1"; local k="$2"
  if [[ "$l" =~ [[:space:]]${k}:([^[:space:]]+) ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  fi
}

# Build an open-copy line from a completed-recurring line by:
#  - flipping [x] -> [ ]
#  - replacing due:<old> with due:<new>
#  - removing done:<date>
#  - replacing the trailing ^id with the new instance id
awiki_build_open_copy() {
  local in="$1"; local new_due="$2"; local new_id="$3"
  local out="$in"
  out="${out/- [x] /- [ ] }"
  # due:
  if [[ "$out" =~ (.*[[:space:]])due:[0-9]{4}-[0-9]{2}-[0-9]{2}(.*) ]]; then
    out="${BASH_REMATCH[1]}due:${new_due}${BASH_REMATCH[2]}"
  else
    # No prior due — splice one in just before the ^id.
    out="${out% ^*} due:${new_due} ^${new_id}"
    printf '%s' "$out"; return 0
  fi
  # done:
  out="$(printf '%s' "$out" | sed -E 's/[[:space:]]done:[0-9]{4}-[0-9]{2}-[0-9]{2}//')"
  # ^id (replace trailing one).
  out="$(printf '%s' "$out" | sed -E "s/\\^[a-z0-9${AWIKI_RECUR_SEP}]+\$/^${new_id}/")"
  printf '%s' "$out"
}

# Collect chain state from a page: emits "<base>\t<n>" lines for each instance.
# The chain head (no separator) is "<base>\t1".
awiki_collect_chain_state() {
  local page="$1"
  awk -v sep="${AWIKI_RECUR_SEP}" '
    /^- \[[ \/?>x-]\]/ {
      if (match($0, /\^[a-z0-9'"${AWIKI_RECUR_SEP}"']+[[:space:]]*$/)) {
        id = substr($0, RSTART+1, RLENGTH-1)
        sub(/[[:space:]]+$/, "", id)
        # Split on sep.
        sep_pos = index(id, sep)
        if (sep_pos == 0) {
          print id "\t1"
        } else {
          base = substr(id, 1, sep_pos - 1)
          n = substr(id, sep_pos + length(sep))
          print base "\t" n
        }
      }
    }' "$page"
}

# Given collected chain state, return the next free <n> for <base>. Always >= 2.
awiki_recur_next_instance_n() {
  local chains="$1"; local base="$2"
  local max=1
  while IFS=$'\t' read -r b n; do
    if [[ "$b" == "$base" && "$n" =~ ^[0-9]+$ && "$n" -gt "$max" ]]; then
      max="$n"
    fi
  done <<<"$chains"
  printf '%s' "$((max + 1))"
}

# Membership check: does <base> already have <n> in chain state?
awiki_recur_chain_has() {
  local chains="$1"; local base="$2"; local n="$3"
  while IFS=$'\t' read -r b nn; do
    if [[ "$b" == "$base" && "$nn" == "$n" ]]; then return 0; fi
  done <<<"$chains"
  return 1
}
```

- [ ] **Step 4: Re-run the every: tests**

```bash
bats tests/recur_test.bats -f "every:"
bats tests/recur_test.bats -f "dry-run prints unified diff"
```

Expected: all 5 passes (the four every: cases plus the now-working dry-run case from Task 18a.3 step 1).

- [ ] **Step 5: Commit**

```bash
git add scripts/action-recur.sh scripts/lib/action-grammar.sh tests/recur_test.bats tests/lib_grammar_test.bats
git commit -m "feat(recur): implement every: arithmetic with last-day month clamp

Adds awiki_date_add_months_clamped and awiki_recur_compute_due to the
shared grammar lib (pure-bash; avoids GNU date's overflow semantics
for 'last-day + 1 month'). action-recur.sh now re-emits open copies
of [x] every: lines immediately above their predecessor on the same
page, with correctly-computed due dates: Nd / Nw use pure-day UTC
arithmetic; Nm clamps to the last valid day of the target month
(2026-01-31 + 1m -> 2026-02-28, leap-year aware). Tests cover the
clamp, the no-clamp common case, weekly, and Nd."
```

---

## Task 18a.5: `action-recur.sh` idempotent re-emit + chain-cap refuse-to-emit

**Files:** Modify: `scripts/action-recur.sh`. Test: `tests/recur_test.bats` (idempotence + cap cases).

Task 18a.4 already wired the chain-state collection and next-instance lookup, so idempotence almost falls out for free. This task hardens it with explicit tests AND adds the formal cap test for the refuse-to-emit at chain length 200.

- [ ] **Step 1: Failing tests first**

Append to `tests/recur_test.bats`:

```bash
@test "action-recur: re-running on the same page is a no-op (idempotent)" {
  seed_weekly_done_a05
  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  # First run produced one open instance.
  open_count=$(grep -c '^- \[ \] water plants' content/projects/garden.md)
  [ "$open_count" -eq 1 ]

  # Re-scan: actions.tsv would now include the new open instance, but the test
  # bypasses action-scan.sh; build the next actions.tsv by hand to mirror reality.
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/garden.md	15	home		    	1w	2026-05-04				garden	public
a05~2	 	water plants	content/projects/garden.md	14	home	2026-05-11	    	1w					garden	public
EOF

  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  # No new instance created.
  open_count=$(grep -c '^- \[ \] water plants' content/projects/garden.md)
  [ "$open_count" -eq 1 ]
}

@test "action-recur: refuses to emit at chain length 200 (exit 6)" {
  mkdir -p content/projects
  # Synthesize a page with one [x] head + 199 [ ] instances. The next emit would be
  # ^h01~201, but the cap test fires when next_n >= 200. Per spec: refuse-to-emit at >=200.
  {
    cat <<'EOF'
---
title: "Cap test"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] tick @computer every:1d due:2026-04-27 done:2026-04-27 ^h01
EOF
    # 198 prior open instances numbered 2..199 — making 199 the highest existing n,
    # so next_n would be 200, which the cap rejects.
    for n in $(seq 2 199); do
      printf -- '- [ ] tick @computer every:1d due:2026-04-%02d ^h01~%d\n' $((27 + n)) "$n"
    done
  } > content/projects/cap.md
  # actions.tsv with 199 instances.
  {
    printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n'
    printf 'h01\tx\ttick\tcontent/projects/cap.md\t10\tcomputer\t\t\t\t\t1d\t2026-04-27\t\t\tcap\tpublic\n'
    for n in $(seq 2 199); do
      printf 'h01~%d\t \ttick\tcontent/projects/cap.md\t%d\tcomputer\t\t\t\t\t1d\t\t\t\tcap\tpublic\n' "$n" "$((10 + n))"
    done
  } > .awiki/maps/actions.tsv

  run bash scripts/action-recur.sh content/projects/cap.md
  [ "$status" -eq 6 ]
  echo "$output" | grep -q "chain \^h01 reached 200 instances; refusing to emit"
  # Page must NOT have a 200th instance.
  ! grep -q '\^h01~200' content/projects/cap.md
}

@test "action-recur: cap message goes to stderr, not stdout" {
  mkdir -p content/projects
  cat > content/projects/cap2.md <<'EOF'
---
title: "Cap test 2"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Done

- [x] tick @computer every:1d due:2026-04-27 done:2026-04-27 ^h02
EOF
  # Synthesize 199 open instances directly on the page.
  for n in $(seq 2 199); do
    printf -- '- [ ] tick @computer every:1d due:2026-04-%02d ^h02~%d\n' $((27 + n)) "$n" >> content/projects/cap2.md
  done
  {
    printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n'
    printf 'h02\tx\ttick\tcontent/projects/cap2.md\t10\tcomputer\t\t\t\t\t1d\t2026-04-27\t\t\tcap2\tpublic\n'
    for n in $(seq 2 199); do
      printf 'h02~%d\t \ttick\tcontent/projects/cap2.md\t%d\tcomputer\t\t\t\t\t1d\t\t\t\tcap2\tpublic\n' "$n" "$((10 + n))"
    done
  } > .awiki/maps/actions.tsv

  # Capture stdout and stderr separately.
  stdout=$(bash scripts/action-recur.sh content/projects/cap2.md 2>/dev/null || true)
  stderr=$(bash scripts/action-recur.sh content/projects/cap2.md 2>&1 >/dev/null || true)
  echo "$stdout" | grep -vq "refusing to emit"
  echo "$stderr" | grep -q  "refusing to emit"
}
```

Run:

```bash
bats tests/recur_test.bats -f "idempotent|chain length 200|stderr"
```

Expected: idempotence test should already pass (Task 18a.4 wired `awiki_recur_chain_has` correctly); cap tests fail with the wrong exit code OR wrong channel until the next step.

- [ ] **Step 2: Verify (and if necessary fix) the cap implementation**

Re-read `recur_compute_new_contents` from Task 18a.4 — the cap branch is already in place:

```bash
if [[ "$next_n" -ge 200 ]]; then
  echo "action-recur: chain ^${base} reached 200 instances; refusing to emit (exit 6)" >&2
  exit 6
fi
```

Verify:
1. Message goes to stderr (`>&2`).
2. Exit code is 6.
3. The exit happens **before** the page is rewritten — `exit 6` aborts the temp-file write inside `recur_one_page_locked` so `mv` never fires. Confirm by checking the flow: the temp file is built by `recur_compute_new_contents`, and that function calls `exit 6` directly, which terminates the whole script (because `set -e` is on). The temp file leaks; `recur_one_page_locked` does NOT have a trap. Add one:

Edit `recur_one_page_locked`:

```bash
recur_one_page_locked() {
  local page="$1"
  local tmp
  tmp="$(mktemp "${page}.tmp.XXXXXX")"
  trap 'rm -f "$tmp"' EXIT
  if ! recur_compute_new_contents "$page" > "$tmp"; then
    # recur_compute_new_contents called `exit 6` already; this branch is for
    # genuine failures (e.g. exit 4 from awiki_recur_compute_due on a bad
    # every: token). Propagate the failure.
    return $?
  fi
  if cmp -s "$page" "$tmp"; then
    return 0
  fi
  mv "$tmp" "$page"
  trap - EXIT   # `mv` consumed the temp file; clear the cleanup trap.
  bash "$(dirname "$0")/log-append.sh" recur "page=$page"
}
```

Caveat: `set -euo pipefail` plus an `exit 6` inside a function called via process-substitution `> "$tmp"` works correctly because `exit` aborts the whole shell. The trap-based cleanup ensures the orphan tempfile gets removed.

- [ ] **Step 3: Re-run the cap tests**

```bash
bats tests/recur_test.bats -f "idempotent|chain length 200|stderr"
```

Expected: all three pass.

Also verify no orphan `.tmp.*` files were left in `content/projects/`:

```bash
ls content/projects/*.tmp.* 2>/dev/null && echo "LEAK" || echo "clean"
```

Expected: `clean`.

- [ ] **Step 4: Commit**

```bash
git add scripts/action-recur.sh tests/recur_test.bats
git commit -m "feat(recur): refuse-to-emit at chain length 200 + temp-file cleanup trap

Phase 18a: action-recur.sh now exits 6 with a structured stderr message
when a chain has reached 200 instances on a single page; the temp file
is cleaned up via an EXIT trap so partial state never leaks. Idempotence
test confirms re-running on a page with the next instance already
present is a no-op. The cap is independent of lint T12 (which reports
the violation but does not abort writes); both checks land in this
phase per the spec's Recurrence section."
```

---

## Task 18a.6: `action-recur.sh` mid-chain `every:` change + separator round-trip

**Files:** Modify: `scripts/action-recur.sh` (verify behavior), `tests/recur_test.bats` (mid-chain + separator).

The spec is explicit: the `every:` interval is read from the **completed line being processed**, not from the chain head. The chain-state collector uses `AWIKI_RECUR_SEP` indirectly, so the round-trip needs an explicit test that flips the separator between `~` and `__` and confirms the lookup still works.

- [ ] **Step 1: Failing test — mid-chain interval change**

Append to `tests/recur_test.bats`:

```bash
@test "action-recur: mid-chain every: change reads interval from completed line" {
  mkdir -p content/projects
  cat > content/projects/garden2.md <<'EOF'
---
title: "Garden 2"
type: project
status: active
last_updated: 2026-05-25
draft: false
---

## Open Actions

## Done

- [ ] water plants @home every:2w due:2026-06-08 ^a05~3
- [x] water plants @home every:2w due:2026-05-25 done:2026-05-25 ^a05~3
- [ ] water plants @home every:1w due:2026-05-25 ^a05~2
- [x] water plants @home every:1w due:2026-05-18 done:2026-05-18 ^a05~2
- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  # The chain head is ^a05 (every:1w); ^a05~2 was completed (still 1w);
  # ^a05~3 was a 2w instance. The user just completed ^a05~3 at 2026-05-25;
  # the next instance must use 2w (the interval on the completed line),
  # NOT 1w (the chain head).
  # However the page above already has the open ~3 line in addition to its
  # completed twin AND already has an open future ^a05~? line we haven't
  # accounted for. To isolate: remove the spurious open ~3 line so the page
  # has only completed instances + the next emit slot.
  cat > content/projects/garden2.md <<'EOF'
---
title: "Garden 2"
type: project
status: active
last_updated: 2026-05-25
draft: false
---

## Open Actions

## Done

- [x] water plants @home every:2w due:2026-05-25 done:2026-05-25 ^a05~3
- [x] water plants @home every:1w due:2026-05-18 done:2026-05-18 ^a05~2
- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/garden2.md	16	home		    	1w	2026-05-04				garden2	public
a05~2	x	water plants	content/projects/garden2.md	15	home		    	1w	2026-05-18				garden2	public
a05~3	x	water plants	content/projects/garden2.md	14	home		    	2w	2026-05-25				garden2	public
EOF

  run bash scripts/action-recur.sh content/projects/garden2.md
  [ "$status" -eq 0 ]
  # Top of chain (the most recent [x]) is ~3 with every:2w done:2026-05-25.
  # Next emit is ^a05~4 with due = 2026-05-25 + 14d = 2026-06-08.
  grep -q '^- \[ \] water plants @home every:2w due:2026-06-08 \^a05~4$' content/projects/garden2.md
  # And the older [x] lines should NOT each have spawned a new emit because their
  # next instance numbers (4, 3, 2 respectively) are already present in chain.
  emit_count=$(grep -c '^- \[ \] water plants' content/projects/garden2.md)
  [ "$emit_count" -eq 1 ]
}
```

Run:

```bash
bats tests/recur_test.bats -f "mid-chain every:"
```

Expected: this should already pass given the Task 18a.4 implementation reads `every:` from the completed line. If it doesn't pass, the bug is in `recur_compute_new_contents` — re-verify `awiki_extract_tail_key "$line" every` reads from `$line` (the current iterated line), not from a chain-head lookup.

- [ ] **Step 2: Failing test — separator round-trip**

The phase-17 spike pinned `AWIKI_RECUR_SEP` to either `~` (ideal) or `__` (fallback). The recur engine must work with whichever the spike chose. Test both shapes by stubbing the constant:

```bash
@test "action-recur: chain detection works under AWIKI_RECUR_SEP=~ (default)" {
  source scripts/lib/action-grammar.sh
  [ "$AWIKI_RECUR_SEP" = "~" ] || skip "spike pinned a different separator"
  seed_weekly_done_a05
  run bash scripts/action-recur.sh content/projects/garden.md
  [ "$status" -eq 0 ]
  grep -q '\^a05~2$' content/projects/garden.md
}

@test "action-recur: chain detection works under AWIKI_RECUR_SEP=__ (fallback)" {
  # Force the fallback separator for this test by overriding the lib export.
  # We cannot easily mutate the sourced lib in-process; instead we patch the
  # lib copy in $TMP/scripts/lib/action-grammar.sh to redefine the constant.
  sed -i.bak -E 's/^export AWIKI_RECUR_SEP=.*/export AWIKI_RECUR_SEP=__/' scripts/lib/action-grammar.sh
  rm -f scripts/lib/action-grammar.sh.bak
  source scripts/lib/action-grammar.sh
  [ "$AWIKI_RECUR_SEP" = "__" ]

  mkdir -p content/projects
  cat > content/projects/sep.md <<'EOF'
---
title: "Sep test"
type: project
status: active
last_updated: 2026-05-04
draft: false
---

## Done

- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a05	x	water plants	content/projects/sep.md	10	home		    	1w	2026-05-04				sep	public
EOF
  run bash scripts/action-recur.sh content/projects/sep.md
  [ "$status" -eq 0 ]
  grep -q '\^a05__2$' content/projects/sep.md
  # The ~ shape MUST NOT appear under fallback mode.
  ! grep -q '\^a05~2' content/projects/sep.md
}
```

Run:

```bash
bats tests/recur_test.bats -f "AWIKI_RECUR_SEP"
```

Expected: under default `~`, first test passes second skips OR second runs and passes (depending on which the spike chose). The point is both shapes round-trip cleanly.

- [ ] **Step 3: Audit `awiki_collect_chain_state` for regex correctness with `__`**

Look at the awk regex:

```awk
match($0, /\^[a-z0-9'"${AWIKI_RECUR_SEP}"']+[[:space:]]*$/)
```

When `AWIKI_RECUR_SEP=__`, the regex character class becomes `[a-z0-9_]`, which is fine for awk POSIX BRE. When `AWIKI_RECUR_SEP=~`, the class becomes `[a-z0-9~]`, which is also accepted (the `~` in awk character classes is literal — confirmed across mawk, gawk, BSD awk).

Add a defensive sanity check at the top of `awiki_collect_chain_state`:

```bash
case "$AWIKI_RECUR_SEP" in
  '~'|'__') ;;
  *) echo "awiki_collect_chain_state: unknown AWIKI_RECUR_SEP='$AWIKI_RECUR_SEP'" >&2; return 1 ;;
esac
```

This makes it clear that future changes to the separator are gated.

- [ ] **Step 4: Re-run the suite**

```bash
bats tests/recur_test.bats
```

Expected: all green.

- [ ] **Step 5: Commit**

```bash
git add scripts/action-recur.sh tests/recur_test.bats scripts/lib/action-grammar.sh
git commit -m "feat(recur): mid-chain every: change + separator round-trip coverage

Confirms via test that action-recur.sh reads the every: token from the
completed line being processed, not from the chain head — a user can
flip every:1w to every:2w mid-chain and the next emit honors the new
interval. Adds explicit round-trip coverage for both AWIKI_RECUR_SEP
values pinned by the phase-17 spike (~ default, __ fallback) and a
defensive guard in awiki_collect_chain_state that rejects any other
separator."
```

---

## Task 18a.7: `triage.sh` skeleton + positional CLI + flock

**Files:** Create: `scripts/triage.sh`. Test: `tests/triage_test.bats` (new file; this task adds skeleton + the `--help` smoke + flock-acquired path).

The CLI:

```
triage.sh <id> <outcome> [k=v ...]
triage.sh --interactive
triage.sh -h | --help
```

Outcomes: `trash`, `do-now`, `act`, `defer-scheduled`, `waiting`, `reference`, `someday`. Positional `[k=v]` pairs supply per-outcome params (`project_slug=`, `context_slug=`, `wait_for=`, `page_type=`, `ref_slug=`, `due=`, `defer=`, plus `lineno=` for inbox-line ids).

- [ ] **Step 1: Failing test first**

Create `tests/triage_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  TMP="$(mktemp -d)"
  export AWIKI_REPO_ROOT="$TMP"
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
  rm -rf "$TMP"
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
```

Run:

```bash
bats tests/triage_test.bats
```

Expected: 3 failures (script does not exist).

- [ ] **Step 2: Create the skeleton**

Create `scripts/triage.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

# triage.sh — bash fallback for the seven-outcome triage workflow.
# Two modes: positional CLI (single item) and --interactive (walks inbox).
# The MCP triage_apply tool (phase 18b) calls into the same library entry-points
# as this script. Side effects must be identical.

# shellcheck source=lib/action-grammar.sh
source "$(dirname "$0")/lib/action-grammar.sh"
# shellcheck source=lib/lock.sh
source "$(dirname "$0")/lib/lock.sh"

readonly OUTCOMES="trash do-now act defer-scheduled waiting reference someday"

usage() {
  cat <<EOF
Usage:
  triage.sh <id> <outcome> [k=v ...]
  triage.sh --interactive
  triage.sh -h | --help

outcomes:
  trash             — strikethrough inbox line / move file to .trash
  do-now            — append [x] line under chosen project / context, log
  act               — append [ ] line, default project=_loose if absent
  defer-scheduled   — append [ ] with due: or defer:
  waiting           — append [?] with wait:[[<entity>]] since:<today>
  reference         — promote to a wiki page (page_type + ref_slug)
  someday           — append [>] under chosen project or _someday

Per-outcome k=v params:
  trash:           lineno=<int>     (required for inbox-* ids)
  do-now:          project_slug=    context_slug=    lineno=
  act:             project_slug=    context_slug=    lineno=
  defer-scheduled: project_slug=    context_slug=    due=YYYY-MM-DD | defer=YYYY-MM-DD    lineno=
  waiting:         project_slug=    wait_for=<entity-slug>    lineno=
  reference:       page_type=       ref_slug=    lineno=
  someday:         project_slug=    lineno=

Common options (env):
  AWIKI_AGENDA_AFTER_N — integer threshold for auto-rebuild (default 5)
  AWIKI_LOCK_TIMEOUT_USER — flock timeout in seconds (default 30)
EOF
}

main() {
  if [[ $# -eq 0 ]]; then
    usage; exit 2
  fi
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --interactive) shift; triage_interactive "$@"; return $? ;;
  esac
  if [[ $# -lt 2 ]]; then
    usage >&2; exit 2
  fi
  local id="$1"; shift
  local outcome="$1"; shift

  if ! [[ " $OUTCOMES " == *" $outcome "* ]]; then
    echo "triage: unknown outcome: $outcome" >&2
    usage >&2
    exit 2
  fi

  # Parse k=v positional args into associative array.
  declare -A params=()
  local kv
  for kv in "$@"; do
    if [[ "$kv" != *=* ]]; then
      echo "triage: arg must be k=v: '$kv'" >&2
      exit 2
    fi
    params["${kv%%=*}"]="${kv#*=}"
  done

  # Acquire exclusive lock and dispatch. The lock holds for the duration of the
  # outcome handler; the deferred agenda rebuild runs after release.
  local timeout="${AWIKI_LOCK_TIMEOUT_USER:-30}"
  awiki_lock_with --timeout="$timeout" -- triage_apply_locked "$id" "$outcome" params

  # After the lock releases, increment task-count and possibly enqueue rebuild.
  triage_increment_and_maybe_rebuild
}

# triage_apply_locked <id> <outcome> <params-name>
triage_apply_locked() {
  local id="$1"; local outcome="$2"; local -n p="$3"

  # 1. Resolve the source: inbox-<sha>-<lineno> | file-<sha> | <block-id>.
  local src_kind src_path src_lineno src_text
  awiki_resolve_triage_source "$id" "${p[lineno]:-}" \
    src_kind src_path src_lineno src_text

  # 2. Inbox TOCTOU re-verify (Task 18a.9).
  if [[ "$src_kind" == "inbox-line" ]]; then
    awiki_verify_inbox_line_id "$id" "$src_path" "$src_lineno"
  fi

  # 3. Dispatch to outcome handler (Task 18a.8).
  case "$outcome" in
    trash)           triage_outcome_trash           "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
    do-now)          triage_outcome_do_now          "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
    act)             triage_outcome_act             "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
    defer-scheduled) triage_outcome_defer_scheduled "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
    waiting)         triage_outcome_waiting         "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
    reference)       triage_outcome_reference       "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
    someday)         triage_outcome_someday         "$src_kind" "$src_path" "$src_lineno" "$src_text" p ;;
  esac

  # 4. Log via the (now poisoning-protected) log-append.
  bash "$(dirname "$0")/log-append.sh" triage "$outcome | ${p[project_slug]:-} | ${p[ref_slug]:-}"
}

# Stubs filled in by tasks 18a.8 / 18a.9 / 18a.11.
triage_outcome_trash() { echo "TODO trash" >&2; return 0; }
triage_outcome_do_now() { echo "TODO do-now" >&2; return 0; }
triage_outcome_act() { echo "TODO act" >&2; return 0; }
triage_outcome_defer_scheduled() { echo "TODO defer-scheduled" >&2; return 0; }
triage_outcome_waiting() { echo "TODO waiting" >&2; return 0; }
triage_outcome_reference() { echo "TODO reference" >&2; return 0; }
triage_outcome_someday() { echo "TODO someday" >&2; return 0; }
awiki_resolve_triage_source() { :; }
awiki_verify_inbox_line_id() { :; }
triage_increment_and_maybe_rebuild() { :; }
triage_interactive() { echo "TODO interactive" >&2; return 0; }

main "$@"
```

```bash
chmod +x scripts/triage.sh
```

- [ ] **Step 3: Re-run the smoke tests**

```bash
bats tests/triage_test.bats -f "help prints usage|unknown outcome|acquires flock"
```

Expected:
- `--help` test: pass.
- unknown-outcome test: pass.
- flock-contention test: pass (the `awiki_lock_with --timeout=1` from the env override exits 7 on contention).

- [ ] **Step 4: Commit**

```bash
git add scripts/triage.sh tests/triage_test.bats
git commit -m "feat(triage): add triage.sh skeleton — CLI parse, flock, dispatch table

Phase 18a: lays the script structure for the seven-outcome triage
workflow. CLI form: triage.sh <id> <outcome> [k=v ...]. The seven
outcome handlers are stubbed; flock acquisition (configurable timeout
via AWIKI_LOCK_TIMEOUT_USER), arg parsing, outcome enum check, and
k=v parameter parsing are functional. Help, unknown-outcome rejection,
and lock-contention serialization are covered by BATS smoke tests.
Outcome bodies, inbox-line TOCTOU verification, interactive walker,
and threshold-resolved rebuild enqueue land in follow-up tasks."
```

---

## Task 18a.8: `triage.sh` seven outcomes (state machine + lazy `_loose`/`_someday`)

**Files:** Modify: `scripts/triage.sh`. Test: `tests/triage_test.bats` (per-outcome assertions).

The seven outcomes share a state machine:

```
1. resolve source -> kind, path, lineno (or path for file-shaped capture)
2. validate per-outcome params at the bash boundary (regex-checked)
3. write/update destination page (lazy-create _loose / _someday if needed)
4. remove source line from inbox.md (or mv file to raw/processed/)
5. (caller) log + counter increment
```

This task builds steps 1-4 for all seven outcomes. TOCTOU verify is Task 18a.9; counter + rebuild is Task 18a.11.

- [ ] **Step 1: Failing tests — one per outcome**

Append to `tests/triage_test.bats`. Each test seeds an inbox line + invokes triage.sh + asserts the expected side effects.

```bash
# Helper: capture the synthesized id for an inbox line at lineno N.
inbox_id_for_lineno() {
  local n="$1"
  local line
  line="$(sed -n "${n}p" content/inbox.md)"
  local sha
  sha="$(printf '%s' "$line" | sha1sum | awk '{print substr($1,1,10)}')"
  printf 'inbox-%s-%d' "$sha" "$n"
}

@test "triage trash: removes inbox line via strikethrough, logs" {
  local id; id="$(inbox_id_for_lineno 7)"   # "- 2026-04-27 14:32 call dentist..."
  run bash scripts/triage.sh "$id" trash lineno=7
  [ "$status" -eq 0 ]
  # Original line replaced with strikethrough form (~~...~~) per spec table.
  grep -q '^~~- 2026-04-27 14:32 call dentist about crown~~$' content/inbox.md
  grep -q '|triage|trash' .awiki/log
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
  grep -q '|triage|do-now' .awiki/log
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
```

Run:

```bash
bats tests/triage_test.bats -f "triage "
```

Expected: 9 failures (outcome handlers are stubs).

- [ ] **Step 2: Implement source resolution**

Replace the `awiki_resolve_triage_source` stub:

```bash
# Resolves <id> to (kind, path, lineno, text). Output is via name-ref.
# Kinds: "inbox-line" | "raw-file" | "action-line"
awiki_resolve_triage_source() {
  local id="$1"; local lineno_param="$2"
  local -n out_kind="$3"
  local -n out_path="$4"
  local -n out_lineno="$5"
  local -n out_text="$6"

  case "$id" in
    inbox-*)
      out_kind="inbox-line"
      out_path="${AWIKI_REPO_ROOT:-.}/content/inbox.md"
      if [[ -z "$lineno_param" ]]; then
        # Try to recover lineno from id suffix (last `-` group).
        if [[ "$id" =~ ^inbox-[a-z0-9]{10}-([0-9]+)$ ]]; then
          lineno_param="${BASH_REMATCH[1]}"
        else
          echo "triage: inbox id missing lineno: $id" >&2
          exit 9
        fi
      fi
      out_lineno="$lineno_param"
      out_text="$(awiki_extract_inbox_text "$out_path" "$out_lineno")"
      ;;
    file-*)
      out_kind="raw-file"
      # Reverse: id encodes sha1 of relpath; we need the relpath. Look it up by
      # scanning raw/inbox/interactive/ entries.
      local sha="${id#file-}"
      local found=""
      while IFS= read -r -d '' f; do
        local rel="${f#"${AWIKI_REPO_ROOT:-.}"/}"
        local fsha; fsha="$(printf '%s' "$rel" | sha1sum | awk '{print substr($1,1,10)}')"
        if [[ "$fsha" == "$sha" ]]; then found="$rel"; break; fi
      done < <(find "${AWIKI_REPO_ROOT:-.}/raw/inbox/interactive" -type f -print0 2>/dev/null)
      if [[ -z "$found" ]]; then
        echo "triage: file id does not resolve: $id" >&2
        exit 9
      fi
      out_path="${AWIKI_REPO_ROOT:-.}/$found"
      out_lineno=0
      out_text="$(basename "$found")"
      ;;
    *)
      out_kind="action-line"
      # Look up via actions.tsv.
      local row
      row="$(awk -F'\t' -v id="$id" 'NR>1 && $1==id {print; exit}' \
              "${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv" 2>/dev/null || true)"
      if [[ -z "$row" ]]; then
        echo "triage: action id not found in actions.tsv: $id" >&2
        exit 9
      fi
      out_path="${AWIKI_REPO_ROOT:-.}/$(awk -F'\t' '{print $4}' <<<"$row")"
      out_lineno="$(awk -F'\t' '{print $5}' <<<"$row")"
      out_text="$(awk -F'\t' '{print $3}' <<<"$row")"
      ;;
  esac
}

# Strips the leading "- <ISO-datetime> " prefix from an inbox line.
awiki_extract_inbox_text() {
  local path="$1"; local lineno="$2"
  local raw; raw="$(sed -n "${lineno}p" "$path")"
  # Pattern: - YYYY-MM-DD HH:MM <text>
  if [[ "$raw" =~ ^-\ [0-9]{4}-[0-9]{2}-[0-9]{2}\ [0-9]{2}:[0-9]{2}\ (.*)$ ]]; then
    printf '%s' "${BASH_REMATCH[1]}"
  else
    printf '%s' "$raw"
  fi
}
```

- [ ] **Step 3: Implement validation helpers**

Append to `scripts/triage.sh`:

```bash
# Slug validation. Project slugs allow leading underscore; others do not.
awiki_check_project_slug() {
  local s="$1"
  [[ "$s" =~ ^[a-z0-9_][a-z0-9_-]{0,63}$ ]] || { echo "triage: bad project_slug: $s" >&2; exit 4; }
}
awiki_check_slug() {
  local label="$1"; local s="$2"
  [[ "$s" =~ ^[a-z0-9][a-z0-9-]{0,63}$ ]] || { echo "triage: bad $label: $s" >&2; exit 4; }
}
awiki_check_page_type() {
  case "$1" in entity|concept|topic|source) ;; *) echo "triage: bad page_type: $1" >&2; exit 4 ;; esac
}
awiki_check_iso_date() {
  local label="$1"; local s="$2"
  [[ "$s" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || { echo "triage: bad date ($label): $s" >&2; exit 4; }
  # Calendar validity: round-trip via date.
  if ! date -u -d "$s" +%Y-%m-%d >/dev/null 2>&1; then
    # BSD fallback.
    if ! date -u -j -f "%Y-%m-%d" "$s" +%Y-%m-%d >/dev/null 2>&1; then
      echo "triage: bad date ($label): $s" >&2; exit 4
    fi
  fi
}

# Mint a fresh 8-char block-id, collision-checked against actions.tsv.
awiki_mint_id() {
  local actions="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  local i id existing=""
  if [[ -f "$actions" ]]; then
    existing="$(awk -F'\t' 'NR>1 {print $1}' "$actions")"
  fi
  for i in 1 2 3 4 5; do
    id="$(LC_ALL=C tr -dc 'a-z0-9' </dev/urandom | head -c 8)"
    if ! grep -qx "$id" <<<"$existing"; then
      printf '%s' "$id"; return 0
    fi
  done
  echo "triage: failed to mint unique id after 5 retries" >&2
  exit 8
}
```

- [ ] **Step 4: Implement lazy page creation + outcome handlers**

```bash
# Ensures content/projects/<slug>.md exists; lazily creates _loose / _someday
# with the right status. For non-underscore slugs, creates a minimal stub if
# absent. Returns absolute path of the page on stdout.
# Sets the global AWIKI_LAST_PAGE_CREATED=1 if a fresh page was just minted,
# 0 if the page already existed. Caller reads this to decide which TRIAGE_*
# array (created vs updated) to push to.
awiki_ensure_project_page() {
  local slug="$1"
  awiki_check_project_slug "$slug"
  local path="${AWIKI_REPO_ROOT:-.}/content/projects/${slug}.md"
  if [[ -f "$path" ]]; then
    AWIKI_LAST_PAGE_CREATED=0
    printf '%s' "$path"
    return 0
  fi
  AWIKI_LAST_PAGE_CREATED=1
  local status_default="active"
  local title="$slug"
  case "$slug" in
    _someday) status_default="someday"; title="Someday/Maybe (catch-all)" ;;
    _loose)   status_default="active";  title="Loose actions (catch-all)" ;;
  esac
  local today; today="$(date -u +%Y-%m-%d)"
  cat > "$path" <<EOF
---
title: "${title}"
date: ${today}
last_updated: ${today}
type: project
status: ${status_default}
outcome: ""
tags: []
draft: false
---

## Open Actions

## Done
EOF
  printf '%s' "$path"
}

# Append <line> to the page's "## Open Actions" section atomically.
awiki_append_under_open_actions() {
  local path="$1"; local line="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v ln="$line" '
    BEGIN { inserted=0 }
    /^## Open Actions[[:space:]]*$/ {
      print
      print ln
      inserted=1
      next
    }
    { print }
    END {
      if (!inserted) {
        # Page lacks the heading; append at end.
        print ""
        print "## Open Actions"
        print ln
      }
    }' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# Append <line> at end of file (used by do-now under ## Done).
awiki_append_under_done() {
  local path="$1"; local line="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v ln="$line" '
    BEGIN { inserted=0 }
    /^## Done[[:space:]]*$/ {
      print
      print ln
      inserted=1
      next
    }
    { print }
    END {
      if (!inserted) {
        print ""
        print "## Done"
        print ln
      }
    }' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# Remove the inbox line at <lineno> atomically.
awiki_remove_inbox_line() {
  local path="$1"; local lineno="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v n="$lineno" 'NR != n' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# Replace the inbox line at <lineno> with strikethrough form.
awiki_strikethrough_inbox_line() {
  local path="$1"; local lineno="$2"
  local tmp; tmp="$(mktemp "${path}.tmp.XXXXXX")"
  awk -v n="$lineno" 'NR == n { print "~~" $0 "~~"; next } { print }' "$path" > "$tmp"
  mv "$tmp" "$path"
}

# === Outcome handlers ===

# Helper: every handler that touches a project page should call this AFTER
# awiki_ensure_project_page so the trailer lists creates vs updates correctly.
_triage_record_project_page() {
  local proj_path="$1"
  if [[ "${AWIKI_LAST_PAGE_CREATED:-0}" == "1" ]]; then
    TRIAGE_CREATED_PAGES+=("$proj_path")
  else
    TRIAGE_UPDATED_PAGES+=("$proj_path")
  fi
}

triage_outcome_trash() {
  local kind="$1"; local path="$2"; local lineno="$3"; local _text="$4"
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_strikethrough_inbox_line "$path" "$lineno"
    TRIAGE_ACTIONS_TAKEN+=("strikethrough inbox line $lineno")
    TRIAGE_UPDATED_PAGES+=("$path")
  elif [[ "$kind" == "raw-file" ]]; then
    mkdir -p "${AWIKI_REPO_ROOT:-.}/raw/inbox/.trash"
    mv "$path" "${AWIKI_REPO_ROOT:-.}/raw/inbox/.trash/"
    TRIAGE_ACTIONS_TAKEN+=("moved raw capture to .trash: $(basename "$path")")
  fi
}

triage_outcome_do_now() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]}"
  awiki_check_slug context_slug "${p[context_slug]}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local today; today="$(date -u +%Y-%m-%d)"
  awiki_append_under_done "$proj" \
    "- [x] ${text} @${p[context_slug]} done:${today} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("do-now @${p[context_slug]} -> ${p[project_slug]} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_act() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_loose}"
  if [[ -n "${p[context_slug]:-}" ]]; then
    awiki_check_slug context_slug "${p[context_slug]}"
  fi
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_loose}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local ctx_token=""
  [[ -n "${p[context_slug]:-}" ]] && ctx_token=" @${p[context_slug]}"
  awiki_append_under_open_actions "$proj" \
    "- [ ] ${text}${ctx_token} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("act${ctx_token} -> ${p[project_slug]:-_loose} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_defer_scheduled() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_loose}"
  [[ -n "${p[context_slug]:-}" ]] && awiki_check_slug context_slug "${p[context_slug]}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_loose}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local ctx_token="" date_token=""
  [[ -n "${p[context_slug]:-}" ]] && ctx_token=" @${p[context_slug]}"
  if [[ -n "${p[due]:-}" ]]; then
    awiki_check_iso_date due "${p[due]}"; date_token=" due:${p[due]}"
  elif [[ -n "${p[defer]:-}" ]]; then
    awiki_check_iso_date defer "${p[defer]}"; date_token=" defer:${p[defer]}"
  else
    echo "triage: defer-scheduled requires due= or defer=" >&2; exit 4
  fi
  awiki_append_under_open_actions "$proj" \
    "- [ ] ${text}${ctx_token}${date_token} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("defer-scheduled${date_token} -> ${p[project_slug]:-_loose} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_waiting() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_loose}"
  [[ -n "${p[wait_for]:-}" ]] || { echo "triage: waiting requires wait_for=" >&2; exit 4; }
  awiki_check_slug wait_for "${p[wait_for]}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_loose}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  local today; today="$(date -u +%Y-%m-%d)"
  awiki_append_under_open_actions "$proj" \
    "- [?] ${text} wait:[[${p[wait_for]}]] since:${today} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("waiting on [[${p[wait_for]}]] -> ${p[project_slug]:-_loose} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_reference() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_page_type "${p[page_type]}"
  awiki_check_slug ref_slug "${p[ref_slug]}"
  local target_dir="${AWIKI_REPO_ROOT:-.}/content/${p[page_type]}s"
  mkdir -p "$target_dir"
  local target="${target_dir}/${p[ref_slug]}.md"
  # Path-resolution guard: confirm the resolved path is under the canonical parent.
  local canonical; canonical="$(cd "$target_dir" && pwd)"
  case "$canonical" in
    "${AWIKI_REPO_ROOT:-.}"/content/*s) ;;
    *) echo "triage: refusing to write outside content/<page_type>s/" >&2; exit 4 ;;
  esac
  if [[ -e "$target" ]]; then
    echo "triage: reference target already exists: $target" >&2; exit 5
  fi
  local today; today="$(date -u +%Y-%m-%d)"
  cat > "$target" <<EOF
---
title: "${text}"
date: ${today}
last_updated: ${today}
type: ${p[page_type]}
tags: []
aliases: []
draft: false
---

(captured ${today})
EOF
  TRIAGE_ACTIONS_TAKEN+=("reference -> ${p[page_type]}/${p[ref_slug]}")
  TRIAGE_CREATED_PAGES+=("$target")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}

triage_outcome_someday() {
  local kind="$1"; local path="$2"; local lineno="$3"; local text="$4"
  local -n p="$5"
  awiki_check_project_slug "${p[project_slug]:-_someday}"
  local proj; proj="$(awiki_ensure_project_page "${p[project_slug]:-_someday}")"
  _triage_record_project_page "$proj"
  local id; id="$(awiki_mint_id)"
  awiki_append_under_open_actions "$proj" \
    "- [>] ${text} ^${id}"
  TRIAGE_ACTIONS_TAKEN+=("someday -> ${p[project_slug]:-_someday} (^${id})")
  if [[ "$kind" == "inbox-line" ]]; then
    awiki_remove_inbox_line "$path" "$lineno"
    TRIAGE_UPDATED_PAGES+=("$path")
  fi
}
```

Note: each handler validates its own params, so a missing or malformed param exits 4 at the boundary — same shape the MCP server (phase 18b) will mirror.

- [ ] **Step 5: Re-run the per-outcome tests**

```bash
bats tests/triage_test.bats -f "triage "
```

Expected: all 9 pass.

- [ ] **Step 6: Emit `TRIAGE-RESULT|<json>` trailer on success**

The MCP layer (phase 18b) parses a single trailer line of the form
`TRIAGE-RESULT|{"actions_taken":[...],"created_pages":[...],"updated_pages":[...]}`
on `triage.sh` stdout. Each outcome handler must populate three
arrays during its work and the dispatcher prints the trailer last.

Add tracking arrays to the dispatcher and modify each handler to push
into them. Failing test first:

```bash
cat >> tests/triage_test.bats <<'EOF'

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
EOF
```

Run — expect failure (no trailer emitted):

```bash
bats tests/triage_test.bats -f "TRIAGE-RESULT"
```

Implement the trailer in `scripts/triage.sh`. Add three globals
declared near the top of the dispatcher, populate them inside each
`triage_outcome_*` handler with the human-readable action verbs +
page paths, then emit the trailer just before the dispatcher exits 0.

```bash
# Add near the top of the dispatcher (after arg parsing):
declare -a TRIAGE_ACTIONS_TAKEN=()
declare -a TRIAGE_CREATED_PAGES=()
declare -a TRIAGE_UPDATED_PAGES=()

# Helper: emit JSON array from bash array.
_triage_json_array() {
  local arr_name="$1"
  local -n arr="$arr_name"
  if [[ ${#arr[@]} -eq 0 ]]; then
    printf '[]'
    return
  fi
  printf '['
  local first=1
  for item in "${arr[@]}"; do
    [[ $first -eq 0 ]] && printf ','
    # Escape backslashes and double-quotes for JSON.
    item="${item//\\/\\\\}"
    item="${item//\"/\\\"}"
    printf '"%s"' "$item"
    first=0
  done
  printf ']'
}

# Each outcome handler in step 4 above pushes into these arrays directly.
# `_triage_record_project_page <path>` is the helper that picks created vs
# updated based on the AWIKI_LAST_PAGE_CREATED flag set by
# `awiki_ensure_project_page`. The reference outcome pushes to
# TRIAGE_CREATED_PAGES directly (it always creates a fresh page).
# The trash + inbox-line removal paths push to TRIAGE_UPDATED_PAGES.

# At dispatcher end, just before `exit 0`:
emit_trailer() {
  local at cp up
  at="$(_triage_json_array TRIAGE_ACTIONS_TAKEN)"
  cp="$(_triage_json_array TRIAGE_CREATED_PAGES)"
  up="$(_triage_json_array TRIAGE_UPDATED_PAGES)"
  printf 'TRIAGE-RESULT|{"actions_taken":%s,"created_pages":%s,"updated_pages":%s}\n' \
    "$at" "$cp" "$up"
}
trap 'rc=$?; if [[ $rc -eq 0 ]]; then emit_trailer; fi' EXIT
```

Important: the trailer is emitted ONLY on success (rc=0). On stale-id
(exit 9) or any other failure, no trailer is emitted — the MCP layer
parses the exit code instead.

Re-run:

```bash
bats tests/triage_test.bats -f "TRIAGE-RESULT"
```

Expected: pass.

- [ ] **Step 7: Commit**

```bash
git add scripts/triage.sh tests/triage_test.bats
git commit -m "feat(triage): implement seven outcome handlers + TRIAGE-RESULT trailer

Phase 18a: each outcome (trash, do-now, act, defer-scheduled, waiting,
reference, someday) writes the right destination, removes the source
inbox line atomically, and validates per-outcome params at the bash
boundary (slugs, page_type enum, ISO calendar dates). _loose.md and
_someday.md are created lazily on first use with the correct status
(active vs someday). Reference outcome resolves and verifies the
destination path is under content/<page_type>s/ before writing.

Dispatcher emits a final TRIAGE-RESULT|<json> stdout trailer on rc=0
with actions_taken / created_pages / updated_pages arrays so the
phase 18b MCP layer can parse the structured payload. The trailer is
suppressed on any non-zero exit (stale-id exits 9; flock contention
exits 7) — callers parse the exit code instead."
```

---

## Task 18a.9: `triage.sh` inbox-line ID TOCTOU re-verify

**Files:** Modify: `scripts/triage.sh`. Test: `tests/triage_test.bats` (stale-id case).

The spec is explicit: between `triage_inbox()` (which synthesizes the id) and `triage_apply(...)` (which acts on it), the user may have edited `inbox.md`. The id encodes `sha1(line)[:10]` plus `lineno`. Before applying, the bash entry-point re-reads the line at `<lineno>`, recomputes `sha1(line)[:10]`, and compares to the embedded hash. Mismatch = exit 9 with structured stdout `{"ok":false,"stale_id":true}` (one JSON line so the MCP server can pass it through verbatim).

- [ ] **Step 1: Failing test first**

Append to `tests/triage_test.bats`:

```bash
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
```

Run:

```bash
bats tests/triage_test.bats -f "stale|unchanged|lineno no longer"
```

Expected: 2 failures (stale-id verification not yet implemented; unchanged-line case may already pass since previous tasks didn't break it).

- [ ] **Step 2: Implement `awiki_verify_inbox_line_id`**

Replace the stub:

```bash
# Re-reads inbox line at <lineno>, recomputes sha1[:10], compares to the hash
# embedded in <id>. On mismatch (or missing line), prints a single-line JSON
# diagnostic to stdout and exits 9.
awiki_verify_inbox_line_id() {
  local id="$1"; local path="$2"; local lineno="$3"

  if [[ ! "$id" =~ ^inbox-([a-z0-9]{10})-([0-9]+)$ ]]; then
    echo '{"ok":false,"stale_id":true,"reason":"id_shape"}'
    exit 9
  fi
  local expect_sha="${BASH_REMATCH[1]}"
  local expect_line="${BASH_REMATCH[2]}"

  if [[ "$expect_line" != "$lineno" ]]; then
    echo '{"ok":false,"stale_id":true,"reason":"lineno_mismatch"}'
    exit 9
  fi

  if [[ ! -f "$path" ]]; then
    echo '{"ok":false,"stale_id":true,"reason":"inbox_missing"}'
    exit 9
  fi

  local total
  total="$(wc -l < "$path")"
  if [[ "$lineno" -lt 1 || "$lineno" -gt "$total" ]]; then
    echo '{"ok":false,"stale_id":true,"reason":"lineno_out_of_range"}'
    exit 9
  fi

  local actual_line actual_sha
  actual_line="$(sed -n "${lineno}p" "$path")"
  actual_sha="$(printf '%s' "$actual_line" | sha1sum | awk '{print substr($1,1,10)}')"
  if [[ "$actual_sha" != "$expect_sha" ]]; then
    echo '{"ok":false,"stale_id":true,"reason":"sha_mismatch"}'
    exit 9
  fi
}
```

Note: the call site (`triage_apply_locked`) already invokes this for `inbox-line` kinds. Verify the call ordering: source-resolve → verify → outcome handler. Stale verification must run **before** any mutation; the temp-file + rename pattern in the outcome handlers means a verify failure aborts cleanly with no on-disk change.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/triage_test.bats -f "stale|unchanged|lineno no longer"
```

Expected: all 3 pass.

- [ ] **Step 4: Confirm no regression on the per-outcome suite**

```bash
bats tests/triage_test.bats
```

Expected: all green (Tasks 18a.7 + 18a.8 + 18a.9 cases).

- [ ] **Step 5: Commit**

```bash
git add scripts/triage.sh tests/triage_test.bats
git commit -m "feat(triage): TOCTOU re-verify on inbox-line ids before mutation

The synthesized inbox-<sha>-<lineno> id may go stale between the
triage_inbox synthesis and triage_apply mutation if the user edits
the inbox file in the gap. triage.sh now re-reads the line at the
supplied lineno, recomputes sha1[:10], and compares to the hash
embedded in the id. Mismatch (or missing line, or out-of-range
lineno) prints a single-line JSON diagnostic on stdout (matching
the MCP triage_apply return shape from phase 18b) and exits 9.
Verification runs before any disk mutation so the inbox stays
intact on rejection."
```

---

## Task 18a.10: `triage.sh --interactive` walker (regex prompts + slug autocomplete)

**Files:** Modify: `scripts/triage.sh`. Test: `tests/triage_test.bats` (driven via `expect`).

The interactive walker prints each unprocessed inbox line + each `raw/inbox/interactive/` file, prompts for an outcome, prompts for the per-outcome params (regex-validated; re-prompts on rejection), and applies the outcome — per item atomically. Slug prompts read existing slugs from `content/projects/` and `content/contexts/` and emit them as a comma-joined hint string.

- [ ] **Step 1: Failing test first (skipped if `expect` missing)**

Append to `tests/triage_test.bats`:

```bash
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
  cat > /tmp/triage_drive.exp <<'EOF'
#!/usr/bin/env expect
set timeout 5
spawn bash scripts/triage.sh --interactive
expect "Outcome (trash|do-now|act|defer-scheduled|waiting|reference|someday)?"
send "trash\r"
expect "Lineno?"
send "7\r"
expect "Outcome (trash|do-now|act|defer-scheduled|waiting|reference|someday)?"
send -- "\x04"   ;# EOF — abort current item
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
  cat > /tmp/triage_drive2.exp <<'EOF'
#!/usr/bin/env expect
set timeout 5
spawn bash scripts/triage.sh --interactive
expect "Outcome*"
send "act\r"
expect "Lineno?"
send "7\r"
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

@test "triage --interactive ctrl-c aborts current item but keeps prior items applied" {
  command -v expect >/dev/null 2>&1 || skip "expect not installed"
  cat > /tmp/triage_drive3.exp <<'EOF'
#!/usr/bin/env expect
set timeout 5
spawn bash scripts/triage.sh --interactive
expect "Outcome*"
send "trash\r"
expect "Lineno?"
send "7\r"
# First item applied. Now interrupt mid-second-item.
expect "Outcome*"
send "act\r"
expect "Lineno?"
send "8\r"
expect "Project slug*"
send -- "\x03"   ;# Ctrl-C
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
```

Run:

```bash
bats tests/triage_test.bats -f "interactive"
```

Expected: 3 failures (interactive walker is a stub).

- [ ] **Step 2: Implement the walker**

Replace the `triage_interactive` stub:

```bash
triage_interactive() {
  # Build the work list: each unprocessed inbox line, then each raw file.
  local inbox="${AWIKI_REPO_ROOT:-.}/content/inbox.md"
  local raw_dir="${AWIKI_REPO_ROOT:-.}/raw/inbox/interactive"

  # Collect inbox lines (skip frontmatter + blank lines).
  local lineno=0
  local in_fm=0
  while IFS= read -r ln; do
    lineno=$((lineno + 1))
    if [[ "$ln" == "---" ]]; then
      if [[ $in_fm -eq 0 ]]; then in_fm=1; continue; else in_fm=0; continue; fi
    fi
    [[ $in_fm -eq 1 ]] && continue
    [[ -z "$ln" ]] && continue
    [[ "$ln" =~ ^~~ ]] && continue   # already trashed
    [[ "$ln" =~ ^- ]] || continue

    triage_walk_one_inbox_item "$inbox" "$lineno" "$ln" || true
  done < "$inbox"

  # Then walk raw files.
  if [[ -d "$raw_dir" ]]; then
    while IFS= read -r -d '' f; do
      triage_walk_one_raw_item "$f" || true
    done < <(find "$raw_dir" -type f -print0)
  fi
}

triage_walk_one_inbox_item() {
  local inbox="$1"; local lineno="$2"; local raw_line="$3"

  printf '\n--- inbox line %d ---\n%s\n' "$lineno" "$raw_line"

  # Per-item Ctrl-C handler: catch SIGINT, print "(aborted)", return without
  # applying. The outer trap restores at function return.
  local aborted=0
  trap 'aborted=1; printf "\n(aborted, item left unprocessed)\n"; return 0' INT

  local outcome
  outcome="$(triage_prompt_outcome)" || { trap - INT; return 0; }
  [[ -z "$outcome" ]] && { trap - INT; return 0; }
  [[ "$outcome" == "EOF" ]] && { trap - INT; return 0; }

  local id
  id="$(awiki_synthesize_inbox_id "$raw_line" "$lineno")"

  declare -A params=()
  params[lineno]="$lineno"
  triage_prompt_params "$outcome" params || { trap - INT; return 0; }
  [[ $aborted -eq 1 ]] && { trap - INT; return 0; }

  # Build positional arg list and re-invoke triage.sh's apply path.
  local kv_args=()
  local k
  for k in "${!params[@]}"; do
    kv_args+=("${k}=${params[$k]}")
  done
  if ! bash "$0" "$id" "$outcome" "${kv_args[@]}"; then
    printf '(skipping; previous error)\n' >&2
  fi
  trap - INT
}

triage_walk_one_raw_item() {
  local f="$1"
  printf '\n--- raw file %s ---\n' "$f"
  local sha; sha="$(printf '%s' "${f#"${AWIKI_REPO_ROOT:-.}"/}" | sha1sum | awk '{print substr($1,1,10)}')"
  local id="file-${sha}"
  local outcome
  outcome="$(triage_prompt_outcome)" || return 0
  [[ -z "$outcome" || "$outcome" == "EOF" ]] && return 0
  declare -A params=()
  triage_prompt_params "$outcome" params || return 0
  local kv_args=()
  local k
  for k in "${!params[@]}"; do kv_args+=("${k}=${params[$k]}"); done
  bash "$0" "$id" "$outcome" "${kv_args[@]}" || true
}

# Synthesize the same id format as triage_inbox does: inbox-<sha>-<lineno>.
awiki_synthesize_inbox_id() {
  local raw="$1"; local lineno="$2"
  local sha; sha="$(printf '%s' "$raw" | sha1sum | awk '{print substr($1,1,10)}')"
  printf 'inbox-%s-%d' "$sha" "$lineno"
}

# Outcome prompt: regex-validated against the OUTCOMES enum; re-prompts on
# bad input. EOF (Ctrl-D) returns "EOF" so the caller can skip the item.
triage_prompt_outcome() {
  local choice
  while true; do
    if ! IFS= read -r -p 'Outcome (trash|do-now|act|defer-scheduled|waiting|reference|someday)? ' choice; then
      printf 'EOF'; return 0
    fi
    if [[ " $OUTCOMES " == *" $choice "* ]]; then
      printf '%s' "$choice"; return 0
    fi
    printf '  unknown outcome; try again.\n' >&2
  done
}

# Per-outcome param prompts. Populates the named-ref associative array.
triage_prompt_params() {
  local outcome="$1"; local -n out="$2"
  # Lineno prompt is shared.
  if [[ -z "${out[lineno]:-}" ]]; then
    triage_prompt_value lineno '^[0-9]+$' "Lineno? " out
  fi
  case "$outcome" in
    trash) ;;
    do-now)
      triage_prompt_slug project_slug "Project slug (suggestions: $(triage_slug_hints projects))? " 1 out
      triage_prompt_slug context_slug "Context slug (suggestions: $(triage_slug_hints contexts))? " 0 out
      ;;
    act)
      triage_prompt_slug project_slug "Project slug (suggestions: $(triage_slug_hints projects))? " 1 out
      triage_prompt_slug context_slug "Context slug (suggestions: $(triage_slug_hints contexts))? " 0 out
      ;;
    defer-scheduled)
      triage_prompt_slug project_slug "Project slug? " 1 out
      triage_prompt_slug context_slug "Context slug? " 0 out
      triage_prompt_value due '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' "Due (YYYY-MM-DD, blank for defer)? " out
      if [[ -z "${out[due]:-}" ]]; then
        triage_prompt_value defer '^[0-9]{4}-[0-9]{2}-[0-9]{2}$' "Defer (YYYY-MM-DD)? " out
      fi
      ;;
    waiting)
      triage_prompt_slug project_slug "Project slug? " 1 out
      triage_prompt_value wait_for '^[a-z0-9][a-z0-9-]{0,63}$' "Wait-for entity slug? " out
      ;;
    reference)
      triage_prompt_value page_type '^(entity|concept|topic|source)$' "Page type? " out
      triage_prompt_value ref_slug '^[a-z0-9][a-z0-9-]{0,63}$' "Reference slug? " out
      ;;
    someday)
      triage_prompt_slug project_slug "Project slug? " 1 out
      ;;
  esac
}

# Generic prompt: regex-validated, blank allowed unless required.
# Args: <key> <regex> <prompt-string> <namref>
triage_prompt_value() {
  local key="$1"; local regex="$2"; local prompt="$3"; local -n out="$4"
  local v
  while true; do
    if ! IFS= read -r -p "$prompt" v; then return 1; fi
    if [[ -z "$v" ]]; then out[$key]=""; return 0; fi
    if [[ "$v" =~ $regex ]]; then out[$key]="$v"; return 0; fi
    printf '  invalid; try again.\n' >&2
  done
}

# Slug prompt with autocomplete-hint string. <required> is 1 or 0.
triage_prompt_slug() {
  local key="$1"; local prompt="$2"; local required="$3"; local -n out="$4"
  local regex='^[a-z0-9_][a-z0-9_-]{0,63}$'
  if [[ "$key" != "project_slug" ]]; then regex='^[a-z0-9][a-z0-9-]{0,63}$'; fi
  local v
  while true; do
    if ! IFS= read -r -p "$prompt" v; then return 1; fi
    if [[ -z "$v" && "$required" -eq 0 ]]; then out[$key]=""; return 0; fi
    if [[ -n "$v" && "$v" =~ $regex ]]; then out[$key]="$v"; return 0; fi
    printf '  invalid slug; try again.\n' >&2
  done
}

# Slug autocomplete hints. <kind> = "projects" | "contexts".
triage_slug_hints() {
  local kind="$1"
  local dir="${AWIKI_REPO_ROOT:-.}/content/${kind}"
  [[ -d "$dir" ]] || { printf 'none'; return 0; }
  find "$dir" -maxdepth 1 -name '*.md' -not -name '_index.md' -exec basename {} .md \; \
    | sort | paste -sd, -
}
```

- [ ] **Step 3: Re-run tests**

```bash
bats tests/triage_test.bats -f "interactive"
```

Expected: 3 passes (or skipped on a box without `expect`).

Important: each spawned `triage.sh` subprocess in the walker takes the lock anew. The walker holds the lock only briefly per item (the duration of the inner `triage_apply_locked` call inside the spawned subprocess). The Ctrl-C handler does NOT re-enter `awiki_lock_with`, so partial state is impossible — a SIGINT during the prompt phase aborts before any lock is taken.

- [ ] **Step 4: Commit**

```bash
git add scripts/triage.sh tests/triage_test.bats
git commit -m "feat(triage): interactive walker with regex prompts and slug autocomplete

triage.sh --interactive walks inbox.md and raw/inbox/interactive/
one item at a time, prompting for the outcome and per-outcome params.
Each prompt is regex-validated and re-prompts on bad input. Slug
prompts surface existing slugs as comma-joined autocomplete hints.
Per-item atomicity: each item applies fully (page write + log +
counter) before the next prompt; Ctrl-C / EOF abort the current item
without applying it; previously-applied items remain applied. Tests
are gated on 'expect' availability and skip cleanly otherwise."
```

---

## Task 18a.11: `triage.sh` threshold-resolved auto-rebuild enqueue

**Files:** Modify: `scripts/triage.sh`. Test: `tests/triage_test.bats` (threshold cases).

After every successful outcome, `triage.sh` increments `.awiki/task-count`. When the count crosses the threshold, the deferred-enqueue path runs: release the 30s user lock first, then re-acquire with the 180s deferred profile and run `agenda.sh`. The threshold lookup chain is: `AWIKI_AGENDA_AFTER_N` env → `.awiki/config` → default 5.

- [ ] **Step 1: Failing tests first**

Append to `tests/triage_test.bats`:

```bash
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
  ! grep -q '|agenda|rebuild' .awiki/log

  # 5th outcome triggers rebuild.
  bash scripts/triage.sh "$id" trash lineno=7
  grep -q '|agenda|rebuild' .awiki/log
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
  ! grep -q '|agenda|rebuild' .awiki/log

  sed -i.bak '7s/.*/- 2026-04-27 14:32 call dentist about crown/' content/inbox.md
  rm -f content/inbox.md.bak
  id="$(inbox_id_for_lineno 7)"
  bash scripts/triage.sh "$id" trash lineno=7
  grep -q '|agenda|rebuild' .awiki/log
}

@test "triage threshold from env overrides .awiki/config" {
  printf 'AWIKI_AGENDA_AFTER_N=99\n' > .awiki/config
  export AWIKI_AGENDA_AFTER_N=1
  echo "0" > .awiki/task-count
  local id; id="$(inbox_id_for_lineno 7)"
  bash scripts/triage.sh "$id" trash lineno=7
  grep -q '|agenda|rebuild' .awiki/log
}
```

Run:

```bash
bats tests/triage_test.bats -f "threshold"
```

Expected: 3 failures (counter logic stubbed).

- [ ] **Step 2: Implement counter + rebuild enqueue**

Replace the `triage_increment_and_maybe_rebuild` stub:

```bash
awiki_resolve_agenda_threshold() {
  if [[ -n "${AWIKI_AGENDA_AFTER_N:-}" ]]; then
    printf '%s' "$AWIKI_AGENDA_AFTER_N"; return 0
  fi
  if [[ -f "${AWIKI_REPO_ROOT:-.}/.awiki/config" ]]; then
    local v
    v="$(awk -F= '/^AWIKI_AGENDA_AFTER_N=/ {print $2}' "${AWIKI_REPO_ROOT:-.}/.awiki/config" | tail -1)"
    if [[ -n "$v" ]]; then printf '%s' "$v"; return 0; fi
  fi
  printf '5'
}

triage_increment_and_maybe_rebuild() {
  local count_file="${AWIKI_REPO_ROOT:-.}/.awiki/task-count"
  [[ -f "$count_file" ]] || echo "0" > "$count_file"
  local n; n="$(cat "$count_file")"
  n=$((n + 1))
  echo "$n" > "$count_file"

  local threshold; threshold="$(awiki_resolve_agenda_threshold)"
  if [[ "$n" -ge "$threshold" ]]; then
    # Deferred-enqueue: the user lock has already been released by the time
    # main returned to here. Re-acquire with the deferred profile and run
    # agenda.sh inline (the spec calls it a 'follow-up tool call slot' for
    # the MCP path; for the bash path it is the last step before exit).
    AWIKI_LOCK_TIMEOUT_USER=180 awiki_lock_with --timeout=180 -- triage_run_agenda
    echo "0" > "$count_file"
  fi
}

triage_run_agenda() {
  bash "$(dirname "$0")/agenda.sh"
  bash "$(dirname "$0")/log-append.sh" agenda "rebuild"
}
```

Also wire `triage_increment_and_maybe_rebuild` to be called from `main` after the lock-protected dispatch, AS DESIGNED in the Task 18a.7 skeleton:

```bash
# inside main() — already in place from Task 18a.7
awiki_lock_with --timeout="$timeout" -- triage_apply_locked "$id" "$outcome" params
triage_increment_and_maybe_rebuild
```

Note: the deferred rebuild itself acquires its own lock with a different timeout. The inner lock is exclusive (`flock -x`), and `flock` is reentrant per-fd-lifecycle but the user lock has been released cleanly so re-acquisition is fresh.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/triage_test.bats -f "threshold"
```

Expected: all 3 pass. Verify the lock-release ordering by adding a temporary `set -x` in `awiki_lock_with` if a flake appears.

- [ ] **Step 4: Cross-test against the agenda.sh stub from phase 17**

The phase-17 `agenda.sh` is a real script that builds `content/agenda/*.md` from `actions.tsv`. The threshold tests above use `|agenda|rebuild` in `.awiki/log` as a proxy for "agenda ran". Add a sanity assertion that the agenda files were indeed touched (their `last_updated` advanced):

```bash
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
  cat > .awiki/maps/actions.tsv <<EOF
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
EOF
  local id; id="$(inbox_id_for_lineno 7)"
  run bash scripts/triage.sh "$id" trash lineno=7
  [ "$status" -eq 0 ]
  today="$(date -u +%Y-%m-%d)"
  grep -q "^last_updated: ${today}$" content/agenda/next-actions.md
}
```

Run:

```bash
bats tests/triage_test.bats -f "advances agenda last_updated"
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/triage.sh tests/triage_test.bats
git commit -m "feat(triage): threshold-resolved auto-rebuild enqueue

After each outcome, triage.sh increments .awiki/task-count and runs
agenda.sh when the count crosses the threshold. Lookup chain:
AWIKI_AGENDA_AFTER_N env > .awiki/config AWIKI_AGENDA_AFTER_N= line
> default 5 (mirrors v1's AWIKI_LINT_AFTER_N chain). The rebuild
runs after the user-facing 30s lock has been released, in a fresh
lock acquisition with the 180s deferred profile, matching the spec's
'no nested invocation' rule. Counter resets after rebuild. Tests
cover all three lookup-chain branches plus end-to-end agenda
last_updated advancement."
```

---

## Task 18a.12: Lint T8 — no-next-action (warn) with `_loose`/`_someday` exemption

**Files:** Modify: `scripts/lint.sh`. Test: `tests/lint_task_test.bats` (T8 cases).

T8 fires when a `type: project, status: active` page has zero open `[ ]`/`[/]` action lines. Exemptions: `_loose.md`, `_someday.md`, any `status: someday`, any `status: done`.

- [ ] **Step 1: Failing tests first**

Append to `tests/lint_task_test.bats`:

```bash
@test "T8 fires on active project with no open actions" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T8a"
  cd "$BATS_TMPDIR/T8a"
  cat > content/projects/idle.md <<'EOF'
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
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  [ "$status" -ne 0 ] || true   # warnings do not fail; check stdout only.
  echo "$output" | grep -qE 'LINT\|WARN\|content/projects/idle.md\|T8: '
}

@test "T8 silent on _loose.md (catch-all exemption)" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T8b"
  cd "$BATS_TMPDIR/T8b"
  cat > content/projects/_loose.md <<'EOF'
---
title: "Loose"
type: project
status: active
draft: false
---

## Open Actions

## Done
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'content/projects/_loose.md|T8'
}

@test "T8 silent on _someday.md (catch-all exemption)" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T8c"
  cd "$BATS_TMPDIR/T8c"
  cat > content/projects/_someday.md <<'EOF'
---
title: "Someday"
type: project
status: someday
draft: false
---

## Open Actions
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'content/projects/_someday.md|T8'
}

@test "T8 silent on status: someday and status: done projects" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T8d"
  cd "$BATS_TMPDIR/T8d"
  cat > content/projects/dormant.md <<'EOF'
---
title: "Dormant"
type: project
status: someday
draft: false
---
EOF
  cat > content/projects/finished.md <<'EOF'
---
title: "Finished"
type: project
status: done
draft: false
---
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -qE 'content/projects/(dormant|finished)\.md\|T8'
}

@test "T8 in-progress [/] counts as an open action" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T8e"
  cd "$BATS_TMPDIR/T8e"
  cat > content/projects/working.md <<'EOF'
---
title: "Working"
type: project
status: active
draft: false
---

## Open Actions

- [/] in progress @computer ^w01
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'content/projects/working.md|T8'
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "T8"
```

Expected: 5 failures (T8 not implemented).

- [ ] **Step 2: Implement T8**

Add to `scripts/lint.sh` in the task-layer rule block:

```bash
# T8: type:project, status:active page with zero open [ ]/[/] actions.
# Exempt: _loose.md, _someday.md, status:someday, status:done.
lint_T8_no_next_action() {
  local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  [[ -f "$actions_tsv" ]] || return 0

  # Build set of project pages with at least one open action.
  declare -A has_open
  while IFS=$'\t' read -r id status _text file _rest; do
    [[ "$status" == " " || "$status" == "/" ]] && has_open["$file"]=1
  done < <(tail -n +2 "$actions_tsv")

  # Walk every type:project page; check status; emit warning if applicable.
  while IFS= read -r -d '' page; do
    local rel="${page#"${AWIKI_REPO_ROOT:-.}"/}"
    # Exempt by filename.
    case "$(basename "$page")" in
      _loose.md|_someday.md) continue ;;
    esac
    local fm_type fm_status
    fm_type="$(awiki_frontmatter_value "$page" type)"
    [[ "$fm_type" == "project" ]] || continue
    fm_status="$(awiki_frontmatter_value "$page" status)"
    case "$fm_status" in
      someday|done) continue ;;
    esac
    if [[ -z "${has_open[$rel]:-}" ]]; then
      lint_emit warn "$rel" "T8: type:project, status:active page has zero open [ ]/[/] actions"
    fi
  done < <(find "${AWIKI_REPO_ROOT:-.}/content/projects" -maxdepth 1 -name '*.md' -not -name '_index.md' -print0 2>/dev/null)
}
```

(`awiki_frontmatter_value` should already exist from phase 16 / 17; add it to `scripts/lib/action-grammar.sh` if not. Confirm with: `grep -n awiki_frontmatter_value scripts/lib/action-grammar.sh`.)

Wire `lint_T8_no_next_action` into the lint-runner's "task-layer rules" section so it fires alongside T1-T7, T14, T15.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/lint_task_test.bats -f "T8"
```

Expected: all 5 pass.

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats
git commit -m "feat(lint): T8 no-next-action warning with _loose/_someday exemption

Active project pages with zero open [ ]/[/] action lines are flagged
WARN T8. Exempt: _loose.md, _someday.md (catch-all pages by spec
convention), and any project page whose status is 'someday' or 'done'.
[/] in-progress lines count as open. Tests cover the firing case,
all three exemption branches, and the in-progress-counts-as-open
case."
```

---

## Task 18a.13: Lint T9 — waiting-stale 14d (warn)

**Files:** Modify: `scripts/lint.sh`. Test: `tests/lint_task_test.bats` (T9 cases).

T9 fires on any `[?]` line whose `since:` value is more than 14 days before today (UTC).

- [ ] **Step 1: Failing tests first**

Append to `tests/lint_task_test.bats`:

```bash
@test "T9 fires when [?] since: > 14d ago" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T9a"
  cd "$BATS_TMPDIR/T9a"
  # Pick a since: 30 days ago.
  local stale; stale="$(date -u -d '-30 days' +%Y-%m-%d 2>/dev/null \
                       || date -u -j -v-30d +%Y-%m-%d)"
  cat > content/projects/q3.md <<EOF
---
title: "Q3"
type: project
status: active
draft: false
---

## Open Actions

- [?] q3 budget approval wait:[[bob-smith]] since:${stale} ^w03
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|WARN\|content/projects/q3\.md\|T9: '
}

@test "T9 silent when [?] since: <= 14d ago" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T9b"
  cd "$BATS_TMPDIR/T9b"
  local fresh; fresh="$(date -u -d '-7 days' +%Y-%m-%d 2>/dev/null \
                       || date -u -j -v-7d +%Y-%m-%d)"
  cat > content/projects/q3.md <<EOF
---
title: "Q3"
type: project
status: active
draft: false
---

## Open Actions

- [?] q3 budget approval wait:[[bob-smith]] since:${fresh} ^w03
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'T9: '
}

@test "T9 boundary: exactly 14d is silent, exactly 15d is warn" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T9c"
  cd "$BATS_TMPDIR/T9c"
  local d14; d14="$(date -u -d '-14 days' +%Y-%m-%d 2>/dev/null \
                   || date -u -j -v-14d +%Y-%m-%d)"
  local d15; d15="$(date -u -d '-15 days' +%Y-%m-%d 2>/dev/null \
                   || date -u -j -v-15d +%Y-%m-%d)"
  cat > content/projects/edge.md <<EOF
---
title: "Edge"
type: project
status: active
draft: false
---

## Open Actions

- [?] item14 wait:[[x]] since:${d14} ^e14
- [?] item15 wait:[[x]] since:${d15} ^e15
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'content/projects/edge\.md\|T9: .*\^e15'
  ! echo "$output" | grep -qE 'content/projects/edge\.md\|T9: .*\^e14'
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "T9"
```

Expected: 3 failures.

- [ ] **Step 2: Implement T9**

Add to `scripts/lint.sh`:

```bash
# T9: [?] action lines whose since: is > 14 days ago.
lint_T9_waiting_stale() {
  local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  [[ -f "$actions_tsv" ]] || return 0
  local today; today="$(date -u +%Y-%m-%d)"

  while IFS=$'\t' read -r id status _text file _line _ctx _due _defer _wait since _rest; do
    [[ "$status" == "?" ]] || continue
    [[ -n "$since" ]] || continue
    [[ "$since" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || continue
    local age_days; age_days="$(awiki_days_between "$since" "$today")"
    if [[ "$age_days" -gt 14 ]]; then
      lint_emit warn "$file" "T9: waiting-stale (since:${since}, ${age_days}d ago) ^${id}"
    fi
  done < <(tail -n +2 "$actions_tsv")
}

# Days between two YYYY-MM-DD dates (b - a). Pure-bash via UTC seconds.
awiki_days_between() {
  local a_secs b_secs
  a_secs="$(date -u -d "$1" +%s 2>/dev/null || date -u -j -f "%Y-%m-%d" "$1" +%s)"
  b_secs="$(date -u -d "$2" +%s 2>/dev/null || date -u -j -f "%Y-%m-%d" "$2" +%s)"
  printf '%d' $(( (b_secs - a_secs) / 86400 ))
}
```

Wire into the lint runner.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/lint_task_test.bats -f "T9"
```

Expected: all 3 pass. The boundary test is the important one — `>14`, not `>=14`, mirrors the spec's "> 14 days ago" wording.

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats
git commit -m "feat(lint): T9 waiting-stale warning at >14 days since: stamp

Any [?] action line whose since: value is more than 14 days ago
emits LINT|WARN|<file>|T9: waiting-stale .... Boundary is strict:
exactly 14 days is silent, 15+ days warns. Helper awiki_days_between
added with UTC + GNU/BSD date fallbacks. Tests cover firing,
silence-within-window, and the 14/15 boundary."
```

---

## Task 18a.14: Lint T10 — overdue (warn)

**Files:** Modify: `scripts/lint.sh`. Test: `tests/lint_task_test.bats` (T10 cases).

T10 fires on `[ ]` or `[/]` action lines whose `due:` is before today (UTC). Today itself is NOT overdue.

- [ ] **Step 1: Failing tests first**

Append to `tests/lint_task_test.bats`:

```bash
@test "T10 fires on [ ] with due in the past" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T10a"
  cd "$BATS_TMPDIR/T10a"
  local past; past="$(date -u -d '-3 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-3d +%Y-%m-%d)"
  cat > content/projects/late.md <<EOF
---
title: "Late"
type: project
status: active
draft: false
---

## Open Actions

- [ ] file taxes @computer due:${past} ^t01
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|WARN\|content/projects/late\.md\|T10: '
}

@test "T10 fires on [/] with due in the past" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T10b"
  cd "$BATS_TMPDIR/T10b"
  local past; past="$(date -u -d '-1 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-1d +%Y-%m-%d)"
  cat > content/projects/late2.md <<EOF
---
title: "Late2"
type: project
status: active
draft: false
---

## Open Actions

- [/] in flight @computer due:${past} ^t02
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|WARN\|content/projects/late2\.md\|T10: '
}

@test "T10 silent on [ ] with due today" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T10c"
  cd "$BATS_TMPDIR/T10c"
  local today; today="$(date -u +%Y-%m-%d)"
  cat > content/projects/today.md <<EOF
---
title: "Today"
type: project
status: active
draft: false
---

## Open Actions

- [ ] something @computer due:${today} ^t03
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'T10: '
}

@test "T10 silent on [x] with due in the past (completed)" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T10d"
  cd "$BATS_TMPDIR/T10d"
  local past; past="$(date -u -d '-30 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-30d +%Y-%m-%d)"
  cat > content/projects/done-old.md <<EOF
---
title: "Done old"
type: project
status: active
draft: false
---

## Done

- [x] old @computer due:${past} done:${past} ^t04
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'T10: '
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "T10"
```

Expected: 4 failures.

- [ ] **Step 2: Implement T10**

```bash
# T10: open or in-progress action lines whose due: is before today.
lint_T10_overdue() {
  local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  [[ -f "$actions_tsv" ]] || return 0
  local today; today="$(date -u +%Y-%m-%d)"
  while IFS=$'\t' read -r id status _text file _line _ctx due _rest; do
    case "$status" in ' '|'/') ;; *) continue ;; esac
    [[ -n "$due" ]] || continue
    [[ "$due" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || continue
    local diff; diff="$(awiki_days_between "$due" "$today")"
    if [[ "$diff" -gt 0 ]]; then
      lint_emit warn "$file" "T10: overdue (due:${due}, ${diff}d ago) ^${id}"
    fi
  done < <(tail -n +2 "$actions_tsv")
}
```

Wire into the lint runner.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/lint_task_test.bats -f "T10"
```

Expected: 4 passes.

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats
git commit -m "feat(lint): T10 overdue warning on [ ]/[/] with past due:

Open and in-progress action lines with due: before today emit
LINT|WARN|<file>|T10: overdue (due:..., Nd ago). Today itself is
not overdue. Completed [x] / cancelled [-] / waiting [?] / someday
[>] lines are not flagged. Tests cover all four boundary cases."
```

---

## Task 18a.15: Lint T11 — stale-someday 90d (warn) + catch-all caveat doc

**Files:** Modify: `scripts/lint.sh`, `WIKI.md` (catch-all caveat). Test: `tests/lint_task_test.bats` (T11 cases).

T11 fires on `[>]` someday lines whose enclosing page has not been touched in 90+ days. "Page touched" is approximated as the file's `last_updated` frontmatter value (NOT mtime — `last_updated` is the explicit user-facing timestamp). Caveat: for `_someday.md`, every someday item shares the page-level timestamp; adding a new item resets the clock for all entries on that page. This is documented in WIKI.md as an acceptable v1 approximation.

- [ ] **Step 1: Failing tests first**

Append to `tests/lint_task_test.bats`:

```bash
@test "T11 fires on [>] when enclosing page last_updated > 90d ago" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T11a"
  cd "$BATS_TMPDIR/T11a"
  local stale; stale="$(date -u -d '-100 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-100d +%Y-%m-%d)"
  cat > content/projects/dusty.md <<EOF
---
title: "Dusty"
type: project
status: active
last_updated: ${stale}
draft: false
---

## Open Actions

- [>] reorganize garage someday @home ^s01
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|WARN\|content/projects/dusty\.md\|T11: '
}

@test "T11 silent on [>] when enclosing page last_updated <= 90d ago" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T11b"
  cd "$BATS_TMPDIR/T11b"
  local fresh; fresh="$(date -u -d '-30 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-30d +%Y-%m-%d)"
  cat > content/projects/recent.md <<EOF
---
title: "Recent"
type: project
status: active
last_updated: ${fresh}
draft: false
---

## Open Actions

- [>] reorganize garage someday @home ^s02
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'T11: '
}

@test "T11 boundary: exactly 90d is silent, 91d is warn" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T11c"
  cd "$BATS_TMPDIR/T11c"
  local d90; d90="$(date -u -d '-90 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-90d +%Y-%m-%d)"
  local d91; d91="$(date -u -d '-91 days' +%Y-%m-%d 2>/dev/null || date -u -j -v-91d +%Y-%m-%d)"
  cat > content/projects/p90.md <<EOF
---
title: "P90"
type: project
status: active
last_updated: ${d90}
draft: false
---
- [>] item @home ^p90a
EOF
  cat > content/projects/p91.md <<EOF
---
title: "P91"
type: project
status: active
last_updated: ${d91}
draft: false
---
- [>] item @home ^p91a
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -qE 'content/projects/p90\.md\|T11'
  echo "$output" | grep -qE 'content/projects/p91\.md\|T11'
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "T11"
```

Expected: 3 failures.

- [ ] **Step 2: Implement T11**

```bash
# T11: [>] someday lines whose enclosing page's last_updated frontmatter is
# > 90 days old. Caveat: page-level granularity, documented in WIKI.md.
lint_T11_stale_someday() {
  local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  [[ -f "$actions_tsv" ]] || return 0
  local today; today="$(date -u +%Y-%m-%d)"
  declare -A seen=()    # avoid emitting per-line for the same page; one warn per (page, id)

  while IFS=$'\t' read -r id status _text file _line _rest; do
    [[ "$status" == ">" ]] || continue
    local abs="${AWIKI_REPO_ROOT:-.}/$file"
    [[ -f "$abs" ]] || continue
    local lu; lu="$(awiki_frontmatter_value "$abs" last_updated)"
    [[ "$lu" =~ ^[0-9]{4}-[0-9]{2}-[0-9]{2}$ ]] || continue
    local diff; diff="$(awiki_days_between "$lu" "$today")"
    if [[ "$diff" -gt 90 ]]; then
      lint_emit warn "$file" "T11: stale-someday (page last_updated:${lu}, ${diff}d ago) ^${id}"
    fi
  done < <(tail -n +2 "$actions_tsv")
}
```

- [ ] **Step 3: Document the catch-all caveat in WIKI.md**

Locate the lint-rules table inside the `<!-- BEGIN task-layer -->` block in `WIKI.md` (added by phase 16). Verify the T11 row already mentions the caveat:

```bash
grep -A1 'T11-stale-someday' WIKI.md
```

If the caveat is missing, edit the row to read:

```
| warn | T11-stale-someday | [>] line whose enclosing page hasn't been touched in 90+ days. Caveat: for the `_someday.md` catch-all, every someday item shares the page-level last_updated timestamp; adding a new someday item resets the clock for all entries on that page. Acceptable approximation for v1; per-line review-timestamps deferred. |
```

- [ ] **Step 4: Re-run tests**

```bash
bats tests/lint_task_test.bats -f "T11"
```

Expected: 3 passes.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats WIKI.md
git commit -m "feat(lint): T11 stale-someday warning + WIKI.md catch-all caveat

Someday [>] lines on pages whose last_updated is more than 90 days
ago emit LINT|WARN|<file>|T11: stale-someday .... Granularity is
page-level (per spec): adding any item to _someday.md resets the
clock for every entry on that page. Caveat documented in the
WIKI.md task-layer lint-rules table. Tests cover firing,
silence-within-window, and the 90/91 boundary."
```

---

## Task 18a.16: Lint T12 — recur-chain-cap (warn ≥150, error ≥200)

**Files:** Modify: `scripts/lint.sh`. Test: `tests/lint_task_test.bats` (T12 cases).

T12 is independent of `action-recur.sh`'s refuse-to-emit at chain `≥200`. Two distinct safeties: refuse-to-emit prevents new growth via the only emit path; T12 lint surfaces violations created by hand edits.

- [ ] **Step 1: Failing tests first**

Append to `tests/lint_task_test.bats`:

```bash
@test "T12 silent on chain length 149" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T12a"
  cd "$BATS_TMPDIR/T12a"
  cat > content/projects/c149.md <<'EOF'
---
title: "Chain149"
type: project
status: active
draft: false
---

- [x] x @home every:1d done:2026-04-27 ^c1
EOF
  for n in $(seq 2 149); do
    printf -- '- [x] x @home every:1d done:2026-04-27 ^c1~%d\n' "$n" >> content/projects/c149.md
  done
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -q 'T12: '
}

@test "T12 warns at chain length 150-199" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T12b"
  cd "$BATS_TMPDIR/T12b"
  cat > content/projects/c150.md <<'EOF'
---
title: "Chain150"
type: project
status: active
draft: false
---

- [x] x @home every:1d done:2026-04-27 ^c2
EOF
  for n in $(seq 2 150); do
    printf -- '- [x] x @home every:1d done:2026-04-27 ^c2~%d\n' "$n" >> content/projects/c150.md
  done
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|WARN\|content/projects/c150\.md\|T12: .*chain.*length=150'
}

@test "T12 errors at chain length >=200" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T12c"
  cd "$BATS_TMPDIR/T12c"
  cat > content/projects/c200.md <<'EOF'
---
title: "Chain200"
type: project
status: active
draft: false
---

- [x] x @home every:1d done:2026-04-27 ^c3
EOF
  for n in $(seq 2 200); do
    printf -- '- [x] x @home every:1d done:2026-04-27 ^c3~%d\n' "$n" >> content/projects/c200.md
  done
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|ERROR\|content/projects/c200\.md\|T12: .*chain.*length=200'
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "T12"
```

Expected: 3 failures.

- [ ] **Step 2: Implement T12**

```bash
# T12: count instances per chain (across all pages); warn at 150-199, error at >=200.
lint_T12_recur_chain_cap() {
  local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  [[ -f "$actions_tsv" ]] || return 0

  declare -A counts=()
  declare -A any_file=()
  local sep="${AWIKI_RECUR_SEP:-~}"

  while IFS=$'\t' read -r id _status _text file _rest; do
    local base="$id"
    if [[ "$id" == *"${sep}"* ]]; then
      base="${id%%"${sep}"*}"
    fi
    counts[$base]=$(( ${counts[$base]:-0} + 1 ))
    any_file[$base]="$file"   # last seen file is fine for the diagnostic
  done < <(tail -n +2 "$actions_tsv")

  local b
  for b in "${!counts[@]}"; do
    local n="${counts[$b]}"
    if [[ "$n" -ge 200 ]]; then
      lint_emit error "${any_file[$b]}" "T12: recur-chain ^${b} length=${n} (>=200)"
    elif [[ "$n" -ge 150 ]]; then
      lint_emit warn  "${any_file[$b]}" "T12: recur-chain ^${b} length=${n} (>=150)"
    fi
  done
}
```

Note: a chain length of exactly 1 is a chain head with no instances yet — no T12 warning. The base counter starts at 1 (the head itself) so a chain with 199 prior instances counts as 200 total entries — matching the recur script's refuse-to-emit boundary.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/lint_task_test.bats -f "T12"
```

Expected: 3 passes. Verify the lint summary line includes the new error count when chain ≥ 200.

- [ ] **Step 4: Cross-check independence from refuse-to-emit**

Add a single test confirming T12 errors even when the chain was created by hand edits, not by `action-recur.sh`:

```bash
@test "T12 fires on hand-edited chain (no recur run)" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T12d"
  cd "$BATS_TMPDIR/T12d"
  cat > content/projects/handedit.md <<'EOF'
---
title: "Handedit"
type: project
status: active
draft: false
---
EOF
  for n in 1 $(seq 2 250); do
    if [[ "$n" -eq 1 ]]; then
      echo "- [x] manual @home every:1d done:2026-04-27 ^h99" >> content/projects/handedit.md
    else
      printf -- '- [x] manual @home every:1d done:2026-04-27 ^h99~%d\n' "$n" >> content/projects/handedit.md
    fi
  done
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|ERROR\|.*\|T12: .*\^h99.*length=250'
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "hand-edited chain"
```

Expected: pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats
git commit -m "feat(lint): T12 recur-chain-cap (warn >=150, error >=200)

Counts instances per chain across all pages and emits LINT|WARN at
length 150-199, LINT|ERROR at length >=200. Independent of
action-recur.sh's refuse-to-emit at chain >=200 (which prevents new
growth via the only emit path); T12 surfaces violations created by
hand edits or by mid-chain interval changes that desync from the
emit cap. Honors AWIKI_RECUR_SEP. Tests cover the silent boundary
(149), warn boundary (150), error boundary (200), and a hand-edited
chain at length 250."
```

---

## Task 18a.17: Lint T13 — context-unused (info)

**Files:** Modify: `scripts/lint.sh`. Test: `tests/lint_task_test.bats` (T13 cases).

T13 is informational: a `type: context` page with zero referencing actions emits `LINT|INFO|<file>|T13: context-unused`. This catches contexts that are configured but never tagged on actions — useful nudge but never blocks a build.

- [ ] **Step 1: Failing tests first**

Append to `tests/lint_task_test.bats`:

```bash
@test "T13 fires when a context page has zero referencing actions" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T13a"
  cd "$BATS_TMPDIR/T13a"
  cat > content/contexts/garage.md <<'EOF'
---
title: "@garage"
type: context
aliases: ['@garage']
draft: false
---
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  echo "$output" | grep -qE 'LINT\|INFO\|content/contexts/garage\.md\|T13: '
}

@test "T13 silent when at least one action references the context" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T13b"
  cd "$BATS_TMPDIR/T13b"
  cat > content/contexts/phone.md <<'EOF'
---
title: "@phone"
type: context
aliases: ['@phone']
draft: false
---
EOF
  cat > content/projects/p.md <<'EOF'
---
title: "P"
type: project
status: active
draft: false
---

- [ ] call x @phone ^p1
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  ! echo "$output" | grep -qE 'content/contexts/phone\.md\|T13'
}

@test "T13 honors aliases (alias parity check)" {
  cp -r "$BATS_TEST_DIRNAME/fixtures/wiki-task-good" "$BATS_TMPDIR/T13c"
  cd "$BATS_TMPDIR/T13c"
  cat > content/contexts/computer.md <<'EOF'
---
title: "@computer"
type: context
aliases: ['@computer', '@laptop']
draft: false
---
EOF
  cat > content/projects/p.md <<'EOF'
---
title: "P"
type: project
status: active
draft: false
---

- [ ] code @laptop ^p2
EOF
  bash scripts/action-scan.sh
  run bash scripts/lint.sh
  # @laptop alias should resolve to computer.md; T13 silent.
  ! echo "$output" | grep -qE 'content/contexts/computer\.md\|T13'
}
```

Run:

```bash
bats tests/lint_task_test.bats -f "T13"
```

Expected: 3 failures.

- [ ] **Step 2: Implement T13**

```bash
# T13: type:context pages with zero referencing actions.
lint_T13_context_unused() {
  local actions_tsv="${AWIKI_REPO_ROOT:-.}/.awiki/maps/actions.tsv"
  local ctx_dir="${AWIKI_REPO_ROOT:-.}/content/contexts"
  [[ -d "$ctx_dir" ]] || return 0

  # Build set of context slugs referenced by at least one action.
  declare -A referenced=()
  if [[ -f "$actions_tsv" ]]; then
    while IFS=$'\t' read -r _id _status _text _file _line context _rest; do
      [[ -n "$context" ]] && referenced[$context]=1
    done < <(tail -n +2 "$actions_tsv")
  fi

  while IFS= read -r -d '' page; do
    local slug; slug="$(basename "$page" .md)"
    [[ "$slug" == "_index" ]] && continue
    local fm_type; fm_type="$(awiki_frontmatter_value "$page" type)"
    [[ "$fm_type" == "context" ]] || continue
    if [[ -z "${referenced[$slug]:-}" ]]; then
      local rel="${page#"${AWIKI_REPO_ROOT:-.}"/}"
      lint_emit info "$rel" "T13: context-unused (no actions reference @${slug})"
    fi
  done < <(find "$ctx_dir" -maxdepth 1 -name '*.md' -print0)
}
```

Note: the alias-parity test relies on the **scanner** normalizing `@laptop` → `computer` via the alias map at `.awiki/maps/alias-to-slug.tsv` (built by phase-17 `lint.sh`'s alias-build step). T13 reads only the scanner's `context` column, which is the canonical slug. So if the scanner is doing its job, the alias-parity test passes for free.

If the alias-build step is not yet wired in this checkout, mark the alias-parity test as `skip` with a reference to phase-17 follow-up.

- [ ] **Step 3: Re-run tests**

```bash
bats tests/lint_task_test.bats -f "T13"
```

Expected: 3 passes (or 2 + 1 skip if the alias map isn't present in this fixture).

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats
git commit -m "feat(lint): T13 context-unused info on contexts with zero references

A type:context page with zero actions referencing its slug (after
alias resolution by the scanner) emits LINT|INFO|<file>|T13:
context-unused. Informational only — never raises the lint exit
code. Reads the canonical context column from actions.tsv so
alias-resolution is handled upstream. Tests cover firing,
referenced-via-canonical-slug silence, and referenced-via-alias
silence."
```

---

## Task 18a.18: Justfile + `docs/just-help.txt` additions

**Files:** Modify: `justfile`, `docs/just-help.txt`. Test: manual smoke + grep assertions.

The phase-16 justfile already declares the `triage` recipe as a stub (`bash scripts/triage.sh --interactive`). Phase 18a replaces the stub with the real recipe, adds a positional-CLI shortcut, and extends `docs/just-help.txt`.

- [ ] **Step 1: Confirm phase-16 stub exists**

```bash
grep -n '^triage' justfile
```

Expected: a `triage` recipe is present (possibly empty or commented). If absent, phase 16 was not fully completed — stop and address.

- [ ] **Step 2: Replace the stub**

Edit `justfile`:

```just
# === task layer ===
# (existing recipes from phase 16/17 stay above)

# Run the triage walker over content/inbox.md and raw/inbox/interactive/.
# For non-interactive single-item application, use:
#   just triage-apply <id> <outcome> [k=v ...]
triage:
    bash scripts/triage.sh --interactive

triage-apply *args:
    bash scripts/triage.sh {{args}}

# Re-emit open copies for any [x] every:... lines whose next-due is missing.
recur:
    bash scripts/action-recur.sh --all

# Dry-run variant — prints unified diff, makes no changes.
recur-dry:
    bash scripts/action-recur.sh --dry-run --all
```

Quoting convention: variadic `*args` is bash-like — `just triage-apply inbox-... act project_slug=_loose context_slug=phone lineno=7` works. For text containing shell metacharacters, single-quote: `just triage-apply 'inbox-...' act 'project_slug=_loose'`.

- [ ] **Step 3: Extend `docs/just-help.txt`**

Append a task-layer section describing each new recipe (the synthesis-spec convention is used for synth recipes; the task layer mirrors it):

```text
## Triage

just triage
  Walks content/inbox.md + raw/inbox/interactive/ one item at a time.
  Prompts for outcome and per-outcome params; regex-validated. Slug
  prompts surface existing project/context slugs as autocomplete
  hints. Ctrl-C aborts the current item; previously-applied items
  remain applied. Per-item-atomic.

just triage-apply <id> <outcome> [k=v ...]
  Non-interactive single-item application. <id> is the synthesized
  inbox-<sha>-<lineno>, file-<sha>, or block-id form. <outcome> is
  one of: trash, do-now, act, defer-scheduled, waiting, reference,
  someday. k=v params per outcome; see `bash scripts/triage.sh --help`.

## Recurrence

just recur
  Re-emit open copies for every page with completed [x] every:...
  lines that have no open instance for the next due date. Runs across
  the whole content tree. Idempotent.

just recur-dry
  Dry-run variant: prints the unified diff of would-be changes and
  exits without writing.
```

- [ ] **Step 4: Extend dep-check (if `expect` is missing the interactive walker still runs but the BATS interactive tests skip)**

Locate the dependency-check script (phase 16 added `flock` here):

```bash
grep -n 'flock' scripts/check-deps.sh 2>/dev/null
```

If present, add a soft check for `expect`:

```bash
if ! command -v expect >/dev/null 2>&1; then
  echo "warn: 'expect' not installed; interactive tests will be skipped." >&2
  echo "  macOS: brew install expect" >&2
  echo "  debian: apt-get install expect" >&2
fi
```

Soft (not exit-1) — `triage.sh --interactive` does not require `expect`; only the BATS test driver does.

- [ ] **Step 5: Smoke test the recipes**

```bash
just --list | grep -E '^(triage|triage-apply|recur|recur-dry)'
just triage-apply --help
just recur-dry --help 2>&1 | head -5  # `--help` is consumed by action-recur.sh, prints usage
```

Expected: all four recipes listed; `triage.sh --help` and `action-recur.sh --help` both print their usage.

- [ ] **Step 6: Commit**

```bash
git add justfile docs/just-help.txt scripts/check-deps.sh
git commit -m "feat(just): wire triage, triage-apply, recur, recur-dry recipes

Phase 18a: replaces the phase-16 triage stub with the real
interactive recipe and adds a triage-apply shortcut for the
positional-CLI form. Adds recur and recur-dry for whole-tree
recurrence emission. docs/just-help.txt gains a Triage and
Recurrence section. Dep-check soft-warns if 'expect' is missing
(only the BATS interactive driver needs it; the walker itself
runs without)."
```

---

## Task 18a.19: Smoke + self-review + merge

**Files:** Modify: `tests/task_smoke_test.bats` (extend phase-17's smoke). Test: full BATS pass.

End-to-end smoke that covers the bash side of the spec's manual smoke (steps 1-5):

1. `just task-init` (already covered by phase 16).
2. `just capture "call dentist"` (already covered by phase 16).
3. `just triage-apply <id> act project_slug=_loose context_slug=phone lineno=...` — covered here.
4. `just agenda` → action appears under `@phone` with project `[[_loose]]`.
5. Mark `[x]` on `_loose.md`; `just agenda` removes it from `next-actions.md`.
6. `just review` (deferred to phase 19 — phase 18a does NOT cover it; the smoke stops at step 5).

- [ ] **Step 1: Extend `tests/task_smoke_test.bats`**

Append:

```bash
@test "smoke: capture -> triage-apply act -> agenda places action under @phone" {
  cd "$BATS_TMPDIR/smoke-wiki" || skip "phase-17 smoke fixture missing"
  bash scripts/capture.sh "call dentist about crown"
  # Inbox now has the new line at the end. Get its lineno + id.
  local lineno; lineno="$(awk '/^- /{n=NR} END{print n}' content/inbox.md)"
  local raw; raw="$(sed -n "${lineno}p" content/inbox.md)"
  local sha; sha="$(printf '%s' "$raw" | sha1sum | awk '{print substr($1,1,10)}')"
  local id="inbox-${sha}-${lineno}"

  # Apply act outcome to _loose with phone context.
  run bash scripts/triage.sh "$id" act project_slug=_loose context_slug=phone "lineno=${lineno}"
  [ "$status" -eq 0 ]
  [ -f content/projects/_loose.md ]
  grep -qE '^- \[ \] call dentist about crown @phone \^[a-z0-9]{8}$' content/projects/_loose.md

  # Run agenda; assert appearance in next-actions.md under @phone.
  bash scripts/action-scan.sh
  bash scripts/agenda.sh
  grep -q '### @phone' content/agenda/next-actions.md
  grep -q 'call dentist about crown' content/agenda/next-actions.md
}

@test "smoke: flip [x] -> agenda removes from next-actions" {
  cd "$BATS_TMPDIR/smoke-wiki"
  # Sanity: action present before flip.
  grep -q 'call dentist about crown' content/agenda/next-actions.md

  # Flip the only open action under _loose.md to [x].
  sed -i.bak 's/^- \[ \] call dentist/- [x] call dentist/' content/projects/_loose.md
  rm -f content/projects/_loose.md.bak

  bash scripts/action-scan.sh
  bash scripts/agenda.sh
  ! grep -q 'call dentist about crown' content/agenda/next-actions.md
}
```

- [ ] **Step 2: Run the full BATS suite**

```bash
bats tests/
```

Expected: all green. If any phase-16 / phase-17 test breaks, the breakage is a regression introduced by phase-18a wiring — fix in-place before merge.

- [ ] **Step 3: Run `just lint` clean**

```bash
just lint
```

Expected: no T1-T13 errors (warnings/info are acceptable in fixtures, but on `main` content the count is 0).

- [ ] **Step 4: Self-review checklist (see end of file)**

Walk the full self-review checklist. Each item references a spec section or a Task above; tick only if the implementation actually does what the line says.

- [ ] **Step 5: Merge to `main`**

```bash
git checkout main
git pull --ff-only
git merge --no-ff phase-18a-task-triage-recur-bash -m "Merge phase 18a: task layer — triage + recurrence (bash side)"
git push origin main
```

(Or open a PR if the repo's branch protection requires it. Phase 18b lands on its own branch + PR; the two are intentionally independent so they can land in either order.)

- [ ] **Step 6: Tag**

```bash
git tag -a phase-18a-complete -m "Phase 18a complete: triage.sh, action-recur.sh, lint T8-T13, log-poison protection"
git push origin phase-18a-complete
```

---

---

## Self-review checklist

Tick each only after verifying the corresponding code AND the test exists and passes.

### Triage (`scripts/triage.sh`)

- [ ] All seven outcomes (`trash`, `do-now`, `act`, `defer-scheduled`, `waiting`, `reference`, `someday`) implemented and have at least one BATS case.
- [ ] Positional CLI form `triage.sh <id> <outcome> [k=v ...]` works without flags.
- [ ] `--interactive` walker: per-item-atomic, regex-validated prompts, slug autocomplete hints from `content/projects/` + `content/contexts/`.
- [ ] `--interactive` Ctrl-C aborts the current item only; previously-applied items remain applied.
- [ ] Source resolution handles all three id shapes: `inbox-<sha>-<lineno>`, `file-<sha>`, plain block-id (looked up in `actions.tsv`).
- [ ] Inbox-line ID TOCTOU re-verify: re-reads the line at `<lineno>`, recomputes `sha1[:10]`, rejects with structured `{"ok":false,"stale_id":true,"reason":"..."}` JSON on stdout + exit 9.
- [ ] Lazy creation of `_loose.md` (`status: active`) and `_someday.md` (`status: someday`) on first use; subsequent outcomes append.
- [ ] All slug, page-type, and date params regex-validated at the bash boundary; bad input exits 4 with a clear message.
- [ ] Reference outcome resolves the destination directory and confirms it is under `content/<page_type>s/` before writing (path-resolution guard mirroring the MCP boundary's defense).
- [ ] `flock -x` acquired via `awiki_lock_with --timeout=30` for the user-facing dispatch; lock contention exits 7.
- [ ] `.awiki/task-count` increments only after the lock-protected dispatch succeeds.
- [ ] Deferred-rebuild enqueue: counter at threshold ⇒ release lock, re-acquire with 180s deferred profile, run `agenda.sh`, reset counter.
- [ ] Threshold lookup chain: `AWIKI_AGENDA_AFTER_N` env > `.awiki/config` value > default 5. Tested for all three branches.
- [ ] `log-append.sh triage "<outcome> | <project_slug> | <ref_slug>"` called on every successful outcome (poisoning protection from Task 18a.2 active).

### Recurrence (`scripts/action-recur.sh`)

- [ ] Three invocation forms (`<page-path>`, `--all`, `--dry-run`) implemented; bad flag exits 2.
- [ ] Idempotent: re-running on a page with the next instance already present is a no-op.
- [ ] Reads `every:` from the **completed line being processed**, not the chain head (mid-chain change honored).
- [ ] `Nd` / `daily` / `Nw` / `weekly` use pure-day UTC arithmetic via `awiki_date_add_days`.
- [ ] `Nm` / `monthly` use `awiki_date_add_months_clamped` (last-day clamp; leap-year aware).
- [ ] Refuse-to-emit at chain `≥200` (exit 6); message goes to stderr; temp file cleaned up via EXIT trap; no on-disk change.
- [ ] `--dry-run` prints unified `diff -u` to stdout, exits 0, makes no changes.
- [ ] Honors `AWIKI_RECUR_SEP` from the phase-17 spike outcome (`~` default, `__` fallback). Chain detection works under both.
- [ ] `flock -x` for writes (30s); `flock -s` for `--dry-run` (30s).
- [ ] New instance ID format `<base>${AWIKI_RECUR_SEP}<n>`; `<n>` monotonically increasing within the chain, starting at 2.

### Lint rules T8-T13 (`scripts/lint.sh`)

- [ ] T8 `no-next-action` (warn): fires on `type:project, status:active` with zero open `[ ]`/`[/]`. Exempts `_loose.md`, `_someday.md`, `status:someday`, `status:done`. `[/]` counts as open.
- [ ] T9 `waiting-stale` (warn): fires on `[?]` with `since:` strictly more than 14 days ago. Boundary 14d=silent, 15d=warn.
- [ ] T10 `overdue` (warn): fires on `[ ]`/`[/]` with `due:` strictly before today. Today=silent, yesterday=warn. `[x]`/`[-]` not flagged.
- [ ] T11 `stale-someday` (warn): fires on `[>]` whose enclosing page's `last_updated` is strictly more than 90 days ago. Page-level granularity caveat documented in WIKI.md.
- [ ] T12 `recur-chain-cap`: warn at length `≥150`, error at length `≥200`. Independent of `action-recur.sh` refuse-to-emit. Chain length counts ALL instances (head + suffixed).
- [ ] T13 `context-unused` (info): fires on `type:context` page with zero referencing actions in `actions.tsv`. Honors alias resolution via the scanner's canonical `context` column.

### `log-append.sh` poisoning protection

- [ ] `|`, `\n`, `\r` in caller-supplied substrings replaced with `_`, ` `, ` ` respectively.
- [ ] Internal-only fields (timestamp) NOT sanitized.
- [ ] Backward-compatible: phase-16 callers passing clean strings see no change.

### BATS coverage

- [ ] `tests/triage_test.bats`: per-outcome × 7 + interactive walker + stale-id + threshold (all three branches) + lazy `_loose`/`_someday` creation + reference path-resolution rejection + bad slug.
- [ ] `tests/recur_test.bats`: dry-run, per-interval (`Nd`/`Nw`/`Nm`/`daily`/`weekly`/`monthly`), clamp boundary, idempotence, refuse-to-emit at 200, separator round-trip (`~` default + `__` fallback), mid-chain interval change.
- [ ] `tests/log_append_poisoning_test.bats`: `|`, `\n`, `\r`, clean control case.
- [ ] `tests/lint_task_test.bats`: T8 (5 cases incl. exemptions), T9 (3 incl. boundary), T10 (4), T11 (3 incl. boundary), T12 (4 incl. hand-edit), T13 (3 incl. alias parity).
- [ ] `tests/lib_grammar_test.bats`: `awiki_date_add_months_clamped`, `awiki_recur_compute_due`, `awiki_days_between` (if introduced).
- [ ] `tests/task_smoke_test.bats`: capture → triage-apply act → agenda places under `@phone` → flip [x] → agenda removes.

### Conventions / hygiene

- [ ] Every consumer sources `scripts/lib/action-grammar.sh` and `scripts/lib/lock.sh`; no inline regex re-implementation.
- [ ] All temp-file writes use `mktemp` and `mv` (atomic rename); EXIT traps clean orphans on error paths.
- [ ] No `python3 -c '...'` introduced anywhere in this phase (matches phase-15+ ban).
- [ ] All scripts start with `#!/usr/bin/env bash` + `set -euo pipefail`.
- [ ] No emoji, no `Co-Authored-By`, no "Generated with Claude Code" trailers.
- [ ] Conventional Commits subject prefixes (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- [ ] Tests pass under both GNU coreutils (Linux) and BSD coreutils (macOS) — date-arithmetic helpers carry both forms.

---

## Open Questions

These flag spec ambiguities or implementation tradeoffs that the phase author resolved in this plan but that the spec leaves under-specified. Surface to the operator before merge if any of the resolutions are wrong.

1. **`triage.sh` JSON-on-stdout for stale-id (Task 18a.9).** The spec defines the `triage_apply` MCP return shape (`{ok:false, stale_id:true}`) but says the bash fallback "invokes the same logic" without specifying the bash output format. This plan emits a single-line JSON diagnostic on stdout from the bash path so the MCP server (phase 18b) can pass it through verbatim or wrap it. Alternative: bash prints a human-readable line and the MCP server reconstructs the JSON. Choosing the JSON-on-stdout form gives the MCP server fewer cross-language transformations and keeps both surfaces auditable. **Coordinate with phase 18b** — if 18b prefers human-readable output, change Task 18a.9 step 2 to emit `triage: stale id, reason=<reason>` and have the MCP server build the JSON.

2. **Per-item lock semantics in `--interactive` (Task 18a.10).** The walker shells out to `triage.sh <id> <outcome> ...` for each item — each subprocess takes the lock independently. Trade-off: the user's input pause between prompts releases the lock so a parallel `agenda.sh` or another agent's tool call can sneak in. The alternative (hold the lock across the whole walk) blocks every other writer for the full triage session. The spec's "per-item-atomic" phrasing argues for releasing between items; this plan follows that. If the operator hits ordering bugs (e.g., a new inbox line appearing mid-walk), document the workaround: complete the walk, re-run.

3. **Reference outcome destination directory plural-suffix mapping.** `triage.sh` writes `content/<page_type>s/<slug>.md` for `page_type ∈ {entity, concept, topic, source}`. The spec says "the resolved path stays under `content/<page_type>s/`" but does NOT say whether `page_type=source` maps to `content/sources/` or whether some page types have irregular plurals. This plan assumes simple `+s` for all four. Verify against the phase-1/3 directory layout — if any page_type uses a different directory name, fix the mapping in `triage_outcome_reference`.

4. **`do-now` placement under `## Done` vs `## Open Actions`.** The spec table says do-now appends `- [x] <text> done:<today>` "to a project / context page" without specifying the section. This plan places do-now items under `## Done` (the conventional "completed" section). Alternative interpretation: append at end of body. The `## Done` placement keeps the page tidy and matches the existing project-page convention (see spec "Project frontmatter" body structure section). If the operator pushes back, change Task 18a.8's `triage_outcome_do_now` to call `awiki_append_under_open_actions` with `[x]` instead.

5. **Empty `wait_for` validation chain.** The spec validates `wait_for` as a slug regex (`^[a-z0-9][a-z0-9-]{0,63}$`); this plan also requires it to refer to an existing entity page. The current implementation does NOT verify entity-page existence at the bash boundary — it leaves that to lint rule T6 (extended). Trade-off: stricter at the boundary catches the typo earlier; looser keeps `triage.sh` decoupled from the entity-page index. Plan choice: looser (boundary regex only); T6 catches at lint time. If 18b wants stricter MCP behavior, the bash side stays as-is and 18b adds the existence check on the MCP boundary alone.

6. **`agenda.sh` deferred-enqueue when threshold hit but lock contention persists.** Task 18a.11 acquires the deferred-profile lock with a 180s timeout. If the lock is still held after 180s (very unlikely on a single-user wiki), the rebuild enqueue exits 7 silently — the next outcome that crosses threshold will retry. This plan does NOT add a separate retry queue or persistent "rebuild pending" marker. Spec says "exit-7 logged and retried on the next rebuild trigger". The implementation logs the contention via `log-append.sh agenda "rebuild-deferred-contended"` for traceability. If a real wiki shows recurring contention, add a `.awiki/rebuild-pending` marker and have the next trigger consume it.

7. **Phase 18b coordination items.** The following bash-side decisions need to be mirrored by the MCP server in phase 18b:
   - Stale-id JSON shape (item 1 above) — both must emit the same JSON.
   - Slug regexes (`^[a-z0-9_][a-z0-9_-]{0,63}$` for project, `^[a-z0-9][a-z0-9-]{0,63}$` for context/wait_for/ref_slug) — both must regex-check at the boundary.
   - ISO date validity (calendar-checked, not just regex) — both must reject `2026-02-30`.
   - Path-resolution guard for reference outcome — both must resolve and confirm canonical parent.
   - Counter increment + threshold rebuild — phase 18b's MCP `triage_apply` must call into the same bash path for the rebuild (i.e., shell out to `triage.sh` OR re-implement the same `awiki_resolve_agenda_threshold` logic in JS). Recommended: shell out, to keep the threshold logic single-sourced.
   See `docs/superpowers/plans/2026-04-27-phase-18b-task-triage-recur-mcp.md` for how the MCP server is expected to call these surfaces.

8. **`--all` parallelism on `action-recur.sh`.** Task 18a.3's `--all` path iterates pages serially, each acquiring its own 30s lock. For a wiki with hundreds of recurring chains across hundreds of pages, this is slow but safe. Parallelizing (e.g., `xargs -P`) would conflict with the single global advisory lock. Plan choice: keep serial. If wikis grow to where serial is painful, the optimization is per-page lock files instead of one global lock — out of scope for phase 18a (and probably phase 18b).
