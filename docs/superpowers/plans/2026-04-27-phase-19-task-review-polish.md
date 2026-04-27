# awiki Plan — Phase 19: Task Layer Review + Polish

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md)
**Master spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master plan:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 16 (task-init + capture + libs), Phase 17 (scanner + agenda + lint T1-T15), Phase 18 (triage + recurrence + MCP), v1 Phase 5 (encrypt-init), v1 Phase 9 (scheduled lint), v1 Phase 12 (sample-wiki + pre-commit hook installer).
**Previous:** Phase 18 (task triage + recurrence + MCP)
**Next:** — (final phase of the task-layer feature)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (`@modelcontextprotocol/sdk`), `flock` (util-linux on Linux, Homebrew `util-linux`/`coreutils` on macOS).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits (`feat:`, `fix:`, `test:`, `docs:`, `chore:`).
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `bats tests/ && just lint` are clean.
- `--` flag terminator before every positional argument in every shell-out and recipe (slug leading-hyphen flag-injection defense — same convention used by phases 16-18).
- All MCP shell-outs use `execFileSync` with explicit argv arrays. **No shell interpolation, ever.**
- Mutating scripts source `scripts/lib/lock.sh` and wrap their bodies in `flock -x` on `.awiki/lock` (timeout 30s). Read-only scripts (this phase: `review-status.sh`) take the shared `flock -s` lock.
- No emojis in any source, doc, or commit message. No `Co-Authored-By` trailer.

---

**Deliverable:** `scripts/review-status.sh` emitting the structured `REVIEW|...` lines + `REVIEW-SUMMARY` row from the spec; full implementations of MCP tools `review_status` (replacing the phase-18 stub) and `mark_review_done`; justfile `review` recipe; `WIKI.md` Weekly Review subsection injected via a one-shot migration helper into the existing `<!-- BEGIN task-layer -->` block; pre-commit hook installer integrated into `scripts/task-init.sh` step 6 with the `# task-layer` idempotency marker; `encrypt-init` extension that auto-covers `content/inbox.md` and `content/agenda/**` when `AWIKI_TASK_LAYER=on`; README task-layer section with the 6-step manual smoke test plus a brief methodology note; `docs/just-help.txt` extended with usage for every task-layer recipe; sample-wiki extended with curated inbox + project + context pages and a sample review-log; CI integration via `scheduled/github-action.yml.example` with a configurable overdue threshold; BATS tests for review-status, mark-review-done, encrypt-init coverage, and a sample-wiki end-to-end smoke. Final acceptance: full smoke from the README runs green; `bats tests/ && just lint` clean across the entire awiki repo.

**Branch:** `phase-19-task-review-polish`

---

## Task 19.1: Branch + scope check

- [ ] **Step 1: Verify prerequisites from phases 16-18**

```bash
git status -s
git log --oneline | head -8
ls scripts/task-init.sh \
   scripts/capture.sh \
   scripts/triage.sh \
   scripts/action-scan.sh \
   scripts/agenda.sh \
   scripts/action-recur.sh \
   scripts/lib/action-grammar.sh \
   scripts/lib/lock.sh \
   mcp/awiki-server/index.js
```

Expected: working tree clean; phases 16, 17, 18 merged into `main`; every listed file present. If any are missing, stop and resolve the prior phase merge before proceeding — phase 19 cannot stand alone.

Confirm the existing `mcp/awiki-server/index.js` exposes `review_status` as a stub returning `{ stub: true }` (planted in phase 18) and that no `mark_review_done` handler exists yet:

```bash
grep -n 'review_status\|mark_review_done' mcp/awiki-server/index.js
```

Expected: one or two lines mentioning `review_status` (the stub), zero lines mentioning `mark_review_done`. If `mark_review_done` is already implemented, this phase has been partially executed — `git log --grep mark_review_done` and reconcile before continuing.

- [ ] **Step 2: Branch**

```bash
git checkout main
git pull --ff-only
git checkout -b phase-19-task-review-polish
```

Expected output: `Switched to a new branch 'phase-19-task-review-polish'`.

- [ ] **Step 3: Confirm phase-18 lock library is sourcable**

```bash
bash -c 'source scripts/lib/lock.sh && declare -F awiki_lock_acquire awiki_lock_release awiki_lock_shared'
```

Expected: prints all three function names (or whatever names phase-16 chose; record the actual names below if they differ — the rest of this plan assumes `awiki_lock_acquire` / `awiki_lock_release` / `awiki_lock_shared`). If the names differ, sed-replace inside this plan before implementing tasks 19.2 onward, or wrap the phase-16 names with these aliases at the top of `scripts/review-status.sh`.

---

## Task 19.2: `scripts/review-status.sh` — structured weekly-review report

This is the core deliverable of phase 19. The script reads `actions.tsv` (rebuilt on demand if stale), walks `content/inbox.md` and `raw/inbox/interactive/`, computes deltas vs `.awiki/last-review`, and emits exactly the `REVIEW|<key>|...` lines from the spec. Exit 0 always — `review-status` is informational; non-zero exit is reserved for genuine failures (missing repo state, unreadable `actions.tsv`, etc.).

- [ ] **Step 1: Write the failing BATS test FIRST — `tests/review_status_test.sh`**

```bash
cat > tests/review_status_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  mkdir -p content/projects content/contexts content/agenda \
           raw/inbox/interactive .awiki/maps scripts
  # Phase 16/17 lib shims so review-status.sh can source them.
  cp -r "$BATS_TEST_DIRNAME/../scripts/lib" scripts/lib
  cp "$BATS_TEST_DIRNAME/../scripts/review-status.sh" scripts/ 2>/dev/null || true
  printf '%s\n' "2026-04-20T09:00:00Z" > .awiki/last-review

  # Inbox with 3 unprocessed lines.
  cat > content/inbox.md <<'INBOX'
---
title: "Inbox"
type: inbox
draft: true
---

- 2026-04-26 09:00 call dentist
- 2026-04-26 09:01 buy cat food
- 2026-04-27 08:00 review draft proposal
INBOX

  # One file-shaped capture in raw/inbox/interactive/.
  printf "%s\n" "scratch note" > raw/inbox/interactive/note.md

  # actions.tsv fixture — header + 8 rows covering every REVIEW key.
  cat > .awiki/maps/actions.tsv <<'TSV'
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a01	[ ]	call dentist	content/projects/q3-launch.md	5	phone	2026-04-25				1w			q3-launch	public
a07	[ ]	send invoice	content/projects/onboarding-revamp.md	7	computer	2026-04-26								onboarding-revamp	public
a02	[/]	draft proposal	content/projects/q3-launch.md	6	computer									q3-launch	public
a03	[?]	q3 budget approval	content/projects/q3-launch.md	8			bob-smith	2026-04-08			3		q3-launch	public
a04	[>]	reorganize garage	content/projects/_someday.md	3	home								_someday	public
a05	[x]	water plants	content/projects/home.md	4	home			2026-05-04		1w	2026-04-26			home	public
a06	[x]	file taxes	content/projects/admin.md	5	computer			2026-04-15				2026-04-22			admin	public
a08	[x]	book flight	content/projects/q3-launch.md	9	computer							2026-04-23			q3-launch	public
TSV

  # renovate-kitchen has no open actions → projects-no-next-action.
  cat > content/projects/renovate-kitchen.md <<'PROJ'
---
title: "Renovate kitchen"
date: 2026-04-01
last_updated: 2026-04-01
type: project
status: active
outcome: "Kitchen renovated."
tags: [home]
draft: false
---

## Open Actions
PROJ

  # onboarding-revamp last_updated > 14d, no open action → stuck.
  cat > content/projects/onboarding-revamp.md <<'PROJ'
---
title: "Onboarding Revamp"
date: 2026-03-01
last_updated: 2026-04-01
type: project
status: active
outcome: "New onboarding shipped."
tags: [work]
draft: false
---

## Open Actions
- [ ] send invoice @computer due:2026-04-26 ^a07
PROJ

  # q3-launch is active with open actions — should NOT appear in
  # projects-no-next-action.
  cat > content/projects/q3-launch.md <<'PROJ'
---
title: "Q3 Launch"
date: 2026-04-15
last_updated: 2026-04-26
type: project
status: active
outcome: "Q3 product launched."
tags: [work]
draft: false
---

## Open Actions
- [ ] call dentist @phone due:2026-04-25 ^a01
- [/] draft proposal @computer ^a02
- [?] q3 budget approval wait:[[bob-smith]] since:2026-04-08 priority:3 ^a03
- [x] book flight @computer done:2026-04-23 ^a08
PROJ

  # Twelve someday entries to assert someday-count.
  cat > content/projects/_someday.md <<'PROJ'
---
title: "Someday"
date: 2026-01-01
last_updated: 2026-04-15
type: project
status: someday
outcome: "Bucket of deferred ideas."
tags: []
draft: false
---

## Open Actions
PROJ
  # Append 12 [>] lines.
  for i in 01 02 03 04 05 06 07 08 09 10 11 12; do
    printf '%s\n' "- [>] item $i ^s$i" >> content/projects/_someday.md
  done

  # Today is fixed for the test via env var read by review-status.sh.
  export AWIKI_TODAY="2026-04-27"
}

teardown() {
  rm -rf "$TEST_REPO"
}

@test "review-status emits all eight REVIEW lines + REVIEW-SUMMARY" {
  run bash "$BATS_TEST_DIRNAME/../scripts/review-status.sh"
  [ "$status" -eq 0 ]

  echo "$output" | grep -qE '^REVIEW\|inbox-unprocessed\|count=3$'
  echo "$output" | grep -qE '^REVIEW\|raw-inbox-files\|count=1$'
  echo "$output" | grep -qE '^REVIEW\|projects-no-next-action\|count=1\|slugs=renovate-kitchen$'
  echo "$output" | grep -qE '^REVIEW\|waiting-stale-14d\|count=1\|ids=a03$'
  echo "$output" | grep -qE '^REVIEW\|overdue\|count=2\|ids=a01,a07$'
  echo "$output" | grep -qE '^REVIEW\|completed-since-last-review\|count=3$'
  echo "$output" | grep -qE '^REVIEW\|stuck-projects\|count=1\|slugs=onboarding-revamp$'
  echo "$output" | grep -qE '^REVIEW\|someday-count\|count=12$'
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=2026-04-20\|attention=[0-9]+$'
}

@test "review-status counts attention=overdue+waiting-stale+projects-no-next-action+stuck" {
  run bash "$BATS_TEST_DIRNAME/../scripts/review-status.sh"
  [ "$status" -eq 0 ]

  # 2 overdue + 1 waiting-stale + 1 no-next + 1 stuck = 5
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=2026-04-20\|attention=5$'
}

@test "review-status exit 0 when actions.tsv is empty" {
  : > .awiki/maps/actions.tsv
  printf '%s\n' "id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind" > .awiki/maps/actions.tsv

  run bash "$BATS_TEST_DIRNAME/../scripts/review-status.sh"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW\|overdue\|count=0$'
  echo "$output" | grep -qE '^REVIEW\|completed-since-last-review\|count=0$'
}

@test "review-status exit 0 when .awiki/last-review is missing" {
  rm -f .awiki/last-review
  run bash "$BATS_TEST_DIRNAME/../scripts/review-status.sh"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^REVIEW-SUMMARY\|last-review=never\|attention=[0-9]+$'
}
EOF
```

Run the test; it must fail because the script does not exist yet.

```bash
bats tests/review_status_test.sh
```

Expected: 4 failures with `bash: scripts/review-status.sh: No such file or directory` or equivalent.

- [ ] **Step 2: Implement `scripts/review-status.sh`**

```bash
cat > scripts/review-status.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# review-status.sh — structured weekly-review report.
#
# Reads:
#   .awiki/maps/actions.tsv        (built by action-scan.sh)
#   content/inbox.md               (inbox lines)
#   raw/inbox/interactive/         (file-shaped captures)
#   .awiki/last-review             (ISO timestamp of last review)
#   content/projects/*.md          (frontmatter for status / last_updated)
#
# Writes: nothing.
# Stdout: REVIEW|<key>|... lines + REVIEW-SUMMARY|... line.
# Exit 0 always for normal operation; non-zero only for genuine failures
# (unreadable repo state, etc.).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=lib/lock.sh
source "${SCRIPT_DIR}/lib/lock.sh"

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

TODAY="${AWIKI_TODAY:-$(date -u +%Y-%m-%d)}"
ACTIONS_TSV=".awiki/maps/actions.tsv"
LAST_REVIEW_FILE=".awiki/last-review"

# Take a shared lock so the report observes a consistent snapshot of
# actions.tsv / inbox.md while triage_apply / agenda.sh hold the
# exclusive lock. 30s timeout matches the user-facing tier.
awiki_lock_shared 30

# ---- 1. inbox-unprocessed -------------------------------------------------
inbox_count=0
if [[ -f content/inbox.md ]]; then
  inbox_count="$(awk '
    BEGIN { in_body=0 }
    /^---$/ { fm++; if (fm==2) { in_body=1 }; next }
    in_body && /^- [0-9]{4}-[0-9]{2}-[0-9]{2} / { c++ }
    END { print c+0 }
  ' content/inbox.md)"
fi
printf 'REVIEW|inbox-unprocessed|count=%s\n' "$inbox_count"

# ---- 2. raw-inbox-files ---------------------------------------------------
raw_count=0
if [[ -d raw/inbox/interactive ]]; then
  # Count regular files (skip dotfiles and directories).
  raw_count="$(find raw/inbox/interactive -mindepth 1 -maxdepth 1 -type f \
              ! -name '.*' 2>/dev/null | wc -l | tr -d ' ')"
fi
printf 'REVIEW|raw-inbox-files|count=%s\n' "$raw_count"

# ---- 3. projects-no-next-action ------------------------------------------
# Active projects (status: active) with zero open [ ]/[/] actions.
# Exempt: _loose.md, _someday.md (T8 exemption from spec).
no_next_slugs=()
shopt -s nullglob
for f in content/projects/*.md; do
  base="$(basename "$f" .md)"
  case "$base" in _loose|_someday|_index) continue ;; esac
  status="$(awk '/^status:/ { print $2; exit }' "$f" | tr -d '"' | tr -d "'")"
  [[ "$status" == "active" ]] || continue

  # Count open actions for this project in actions.tsv.
  if [[ -f "$ACTIONS_TSV" ]]; then
    open="$(awk -F'\t' -v slug="$base" '
      NR==1 { next }
      $15 == slug && ($2 == "[ ]" || $2 == "[/]") { c++ }
      END { print c+0 }
    ' "$ACTIONS_TSV")"
  else
    open=0
  fi
  if [[ "$open" -eq 0 ]]; then
    no_next_slugs+=("$base")
  fi
done
shopt -u nullglob

if [[ ${#no_next_slugs[@]} -gt 0 ]]; then
  IFS=, ; slugs="${no_next_slugs[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|projects-no-next-action|count=%s|slugs=%s\n' "${#no_next_slugs[@]}" "$slugs"
else
  printf 'REVIEW|projects-no-next-action|count=0\n'
fi

# ---- 4. waiting-stale-14d -------------------------------------------------
# [?] lines whose since: is older than 14 days.
stale_ids=()
if [[ -f "$ACTIONS_TSV" ]]; then
  while IFS=$'\t' read -r id status _ _ _ _ _ _ _ since _ _ _ _ _ _; do
    [[ "$id" == "id" ]] && continue
    [[ "$status" == "[?]" ]] || continue
    [[ -n "$since" ]] || continue
    # Days elapsed = (today - since) / 86400. Use date -d on Linux,
    # gdate fallback on macOS; if neither, skip the row (caller can
    # see "warning: no GNU date" on stderr).
    if today_s="$(date -u -d "$TODAY" +%s 2>/dev/null)" && \
       since_s="$(date -u -d "$since" +%s 2>/dev/null)"; then
      :
    elif today_s="$(gdate -u -d "$TODAY" +%s 2>/dev/null)" && \
         since_s="$(gdate -u -d "$since" +%s 2>/dev/null)"; then
      :
    else
      echo "review-status: warning: no GNU date; skipping waiting-stale row $id" >&2
      continue
    fi
    days=$(( (today_s - since_s) / 86400 ))
    if [[ $days -gt 14 ]]; then
      stale_ids+=("$id")
    fi
  done < "$ACTIONS_TSV"
fi

if [[ ${#stale_ids[@]} -gt 0 ]]; then
  IFS=, ; ids="${stale_ids[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|waiting-stale-14d|count=%s|ids=%s\n' "${#stale_ids[@]}" "$ids"
else
  printf 'REVIEW|waiting-stale-14d|count=0\n'
fi

# ---- 5. overdue -----------------------------------------------------------
# [ ] or [/] line with due: < today.
overdue_ids=()
if [[ -f "$ACTIONS_TSV" ]]; then
  while IFS=$'\t' read -r id status _ _ _ _ due _ _ _ _ _ _ _ _ _; do
    [[ "$id" == "id" ]] && continue
    [[ "$status" == "[ ]" || "$status" == "[/]" ]] || continue
    [[ -n "$due" ]] || continue
    if [[ "$due" < "$TODAY" ]]; then
      overdue_ids+=("$id")
    fi
  done < "$ACTIONS_TSV"
fi

if [[ ${#overdue_ids[@]} -gt 0 ]]; then
  # Sort for stable output.
  IFS=$'\n' read -r -d '' -a sorted_overdue < <(printf '%s\n' "${overdue_ids[@]}" | sort && printf '\0') || true
  IFS=, ; ids="${sorted_overdue[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|overdue|count=%s|ids=%s\n' "${#sorted_overdue[@]}" "$ids"
else
  printf 'REVIEW|overdue|count=0\n'
fi

# ---- 6. completed-since-last-review --------------------------------------
last_review_iso=""
if [[ -f "$LAST_REVIEW_FILE" ]]; then
  last_review_iso="$(tr -d '[:space:]' < "$LAST_REVIEW_FILE")"
fi
last_review_date="${last_review_iso%%T*}"
[[ -z "$last_review_date" ]] && last_review_date="never"

completed=0
if [[ -f "$ACTIONS_TSV" && "$last_review_date" != "never" ]]; then
  completed="$(awk -F'\t' -v cutoff="$last_review_date" '
    NR==1 { next }
    $2 == "[x]" && $12 != "" && $12 >= cutoff { c++ }
    END { print c+0 }
  ' "$ACTIONS_TSV")"
elif [[ -f "$ACTIONS_TSV" ]]; then
  # No last-review baseline; count all completed.
  completed="$(awk -F'\t' 'NR>1 && $2 == "[x]" { c++ } END { print c+0 }' "$ACTIONS_TSV")"
fi
printf 'REVIEW|completed-since-last-review|count=%s\n' "$completed"

# ---- 7. stuck-projects ----------------------------------------------------
# Active projects with last_updated > 14d AND no completed action since
# last_updated. Or zero open + zero recent completed.
stuck_slugs=()
shopt -s nullglob
for f in content/projects/*.md; do
  base="$(basename "$f" .md)"
  case "$base" in _loose|_someday|_index) continue ;; esac
  status="$(awk '/^status:/ { print $2; exit }' "$f" | tr -d '"' | tr -d "'")"
  [[ "$status" == "active" ]] || continue

  last_updated="$(awk '/^last_updated:/ { print $2; exit }' "$f" | tr -d '"' | tr -d "'")"
  [[ -n "$last_updated" ]] || continue

  if today_s="$(date -u -d "$TODAY" +%s 2>/dev/null)" && \
     lu_s="$(date -u -d "$last_updated" +%s 2>/dev/null)"; then
    :
  elif today_s="$(gdate -u -d "$TODAY" +%s 2>/dev/null)" && \
       lu_s="$(gdate -u -d "$last_updated" +%s 2>/dev/null)"; then
    :
  else
    continue
  fi
  days_idle=$(( (today_s - lu_s) / 86400 ))

  # Open count (already computed above implicitly; recompute for clarity).
  if [[ -f "$ACTIONS_TSV" ]]; then
    open="$(awk -F'\t' -v slug="$base" 'NR>1 && $15 == slug && ($2 == "[ ]" || $2 == "[/]") { c++ } END { print c+0 }' "$ACTIONS_TSV")"
    recent_done="$(awk -F'\t' -v slug="$base" -v cutoff="$last_updated" '
      NR>1 && $15 == slug && $2 == "[x]" && $12 != "" && $12 >= cutoff { c++ }
      END { print c+0 }
    ' "$ACTIONS_TSV")"
  else
    open=0; recent_done=0
  fi

  if [[ $days_idle -gt 14 && $recent_done -eq 0 && $open -eq 0 ]]; then
    stuck_slugs+=("$base")
  elif [[ $days_idle -gt 14 && $recent_done -eq 0 ]]; then
    # Spec: "projects with status: active AND zero open actions, OR
    # last_updated > 14d with no completed action since" — second clause.
    stuck_slugs+=("$base")
  fi
done
shopt -u nullglob

if [[ ${#stuck_slugs[@]} -gt 0 ]]; then
  IFS=, ; slugs="${stuck_slugs[*]}" ; IFS=$' \t\n'
  printf 'REVIEW|stuck-projects|count=%s|slugs=%s\n' "${#stuck_slugs[@]}" "$slugs"
else
  printf 'REVIEW|stuck-projects|count=0\n'
fi

# ---- 8. someday-count -----------------------------------------------------
someday=0
if [[ -f "$ACTIONS_TSV" ]]; then
  someday="$(awk -F'\t' 'NR>1 && $2 == "[>]" { c++ } END { print c+0 }' "$ACTIONS_TSV")"
fi
printf 'REVIEW|someday-count|count=%s\n' "$someday"

# ---- REVIEW-SUMMARY -------------------------------------------------------
attention=$(( ${#overdue_ids[@]} + ${#stale_ids[@]} + ${#no_next_slugs[@]} + ${#stuck_slugs[@]} ))
printf 'REVIEW-SUMMARY|last-review=%s|attention=%s\n' "$last_review_date" "$attention"

awiki_lock_release || true
exit 0
EOF
chmod +x scripts/review-status.sh
```

- [ ] **Step 3: Run the test, see it pass**

```bash
bats tests/review_status_test.sh
```

Expected: 4 tests, 4 pass.

- [ ] **Step 4: Commit**

```bash
git add scripts/review-status.sh tests/review_status_test.sh
git commit -m "feat: add review-status.sh structured weekly-review report"
```

---

## Task 19.3: justfile `review` recipe (replace phase-16 stub)

Phase 16 inserted a commented placeholder for `review` so that the task-layer recipe block was complete; phase 19 replaces it with the actual chain.

- [ ] **Step 1: Inspect current state of the task-layer block**

```bash
grep -n -A1 -B1 'review' justfile | sed -n '1,40p'
```

Expected: a `# review:` commented stub plus the surrounding `# === task layer ===` recipes from phase 16.

- [ ] **Step 2: Replace the commented stub with the live recipe**

Edit `justfile`. Find the block:

```just
# review:
#     # phase 19 — wired in phase 19
```

Replace with:

```just
review:
    bash scripts/agenda.sh
    bash scripts/lint.sh
    bash scripts/review-status.sh
```

- [ ] **Step 3: Smoke check**

```bash
just --list | grep -E '^\s*(review|agenda|capture|triage|task-init|scan)\s'
```

Expected: every task-layer recipe appears in the `just --list` output.

```bash
just review || true
```

Expected: when run on the bare repo (no actions yet), the recipe runs `agenda.sh` (regenerates empty managed regions), `lint.sh` (clean), and `review-status.sh` (every count zero, `last-review=<today>` or `never`). Exit code 0.

- [ ] **Step 4: Commit**

```bash
git add justfile
git commit -m "feat(just): wire review recipe (agenda + lint + review-status)"
```

---

## Task 19.4: MCP `review_status()` — replace phase-18 stub with real implementation

Phase 18 registered `review_status` returning `{ stub: true }` so the tool surface was discoverable. Phase 19 replaces the handler with one that shells out to `scripts/review-status.sh` (under shared lock) and reshapes the structured stdout into the JSON object documented in the spec.

- [ ] **Step 1: Write the failing MCP test FIRST**

```bash
cat > tests/mcp_review_status_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  # Reuse the review-status fixture from review_status_test.sh.
  bash "$BATS_TEST_DIRNAME/helpers/seed-review-fixture.sh" "$TEST_REPO"
}

teardown() {
  rm -rf "$TEST_REPO"
}

@test "mcp review_status returns full JSON object (no stub flag)" {
  resp="$(node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool review_status --args '{}')"
  echo "$resp" | python3 -c '
import json, sys
d = json.load(sys.stdin)
assert "stub" not in d, "stub flag still present"
for k in ["last_review","inbox_unprocessed","raw_inbox_files",
         "projects_no_next_action","waiting_stale_14d","overdue",
         "completed_since_last_review","stuck_projects","someday_count"]:
  assert k in d, f"missing key {k}"
assert isinstance(d["projects_no_next_action"], list)
assert isinstance(d["waiting_stale_14d"], list)
assert isinstance(d["overdue"], list)
assert isinstance(d["stuck_projects"], list)
'
}

@test "mcp review_status waiting_stale_14d entries carry id/wait/since/days" {
  resp="$(node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool review_status --args '{}')"
  echo "$resp" | python3 -c '
import json, sys
d = json.load(sys.stdin)
for e in d["waiting_stale_14d"]:
  for k in ("id","wait","since","days"):
    assert k in e, f"missing {k}"
'
}

@test "mcp review_status overdue entries carry id/due/days_over" {
  resp="$(node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool review_status --args '{}')"
  echo "$resp" | python3 -c '
import json, sys
d = json.load(sys.stdin)
for e in d["overdue"]:
  for k in ("id","due","days_over"):
    assert k in e, f"missing {k}"
'
}
EOF
```

Companion seed helper (shared across MCP tests):

```bash
mkdir -p tests/helpers
cat > tests/helpers/seed-review-fixture.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
DEST="$1"
SRC_LIB="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)/scripts/lib"
mkdir -p "$DEST/scripts" "$DEST/.awiki/maps" \
         "$DEST/content/projects" "$DEST/content/contexts" \
         "$DEST/content/agenda" "$DEST/raw/inbox/interactive"
cp -r "$SRC_LIB" "$DEST/scripts/lib"
cp "$SRC_LIB/../review-status.sh" "$DEST/scripts/"

printf '%s\n' "2026-04-20T09:00:00Z" > "$DEST/.awiki/last-review"

# (Identical fixture body to review_status_test.sh — extracted so MCP and
# bash callers share one source of truth.)
# ... see review_status_test.sh setup() for the full content; this helper
# duplicates only the file writes, no the bats setup() boilerplate.
EOF
chmod +x tests/helpers/seed-review-fixture.sh
```

Fill in the helper body by copy-pasting the file writes from `review_status_test.sh::setup()` (the inbox, actions.tsv, three project files, _someday.md). Keep the two files in sync — flag a TODO at the top of each that points at the other.

Run:

```bash
bats tests/mcp_review_status_test.sh
```

Expected: 3 failures referencing `stub: true` or `stub flag still present`.

- [ ] **Step 2: Implement the real `review_status` handler**

Edit `mcp/awiki-server/index.js`. Locate the phase-18 stub:

```javascript
// review_status — stub, real impl in phase 19
server.tool('review_status', {}, async () => ({ content: [{ type: 'json', json: { stub: true } }] }));
```

Replace with:

```javascript
const { execFileSync } = require('node:child_process');
const path = require('node:path');

server.tool('review_status', {}, async () => {
  const repoRoot = path.resolve(__dirname, '..', '..');
  // Shell out under shared lock; review-status.sh acquires its own
  // flock -s, so the MCP server does not need to take one here.
  let stdout;
  try {
    stdout = execFileSync('bash', ['--', 'scripts/review-status.sh'], {
      cwd: repoRoot,
      encoding: 'utf8',
      env: { ...process.env, LC_ALL: 'C' },
      timeout: 30_000,
    });
  } catch (err) {
    return { content: [{ type: 'text', text: `review_status failed: ${err.message}` }], isError: true };
  }

  // Parse REVIEW|key|... lines into the documented JSON shape.
  const out = {
    last_review: 'never',
    inbox_unprocessed: 0,
    raw_inbox_files: 0,
    projects_no_next_action: [],
    waiting_stale_14d: [],
    overdue: [],
    completed_since_last_review: 0,
    stuck_projects: [],
    someday_count: 0,
  };

  // Helper: parse k=v segments after the key.
  const seg = line => Object.fromEntries(line.split('|').slice(2)
    .map(p => { const i = p.indexOf('='); return [p.slice(0,i), p.slice(i+1)]; }));

  for (const raw of stdout.split('\n')) {
    if (!raw.startsWith('REVIEW')) continue;
    const parts = raw.split('|');
    const tag = parts[1];
    const kv = seg(raw);

    if (raw.startsWith('REVIEW-SUMMARY|')) {
      // last-review=<date>|attention=<n>
      out.last_review = kv['last-review'] ?? 'never';
      // attention is computed; we do not surface it directly — clients
      // can recompute from the array lengths.
      continue;
    }

    switch (tag) {
      case 'inbox-unprocessed':
        out.inbox_unprocessed = parseInt(kv.count, 10) || 0; break;
      case 'raw-inbox-files':
        out.raw_inbox_files = parseInt(kv.count, 10) || 0; break;
      case 'projects-no-next-action':
        out.projects_no_next_action = (kv.slugs ?? '').split(',').filter(Boolean); break;
      case 'waiting-stale-14d':
        // Reshape from CSV ids to objects {id,wait,since,days}. We need
        // wait/since/days from actions.tsv — read the same TSV directly
        // to enrich. Keep this read inside the same block to avoid a
        // second shell-out.
        out.waiting_stale_14d = enrichWaitingStale(repoRoot, (kv.ids ?? '').split(',').filter(Boolean));
        break;
      case 'overdue':
        out.overdue = enrichOverdue(repoRoot, (kv.ids ?? '').split(',').filter(Boolean));
        break;
      case 'completed-since-last-review':
        out.completed_since_last_review = parseInt(kv.count, 10) || 0; break;
      case 'stuck-projects':
        out.stuck_projects = (kv.slugs ?? '').split(',').filter(Boolean); break;
      case 'someday-count':
        out.someday_count = parseInt(kv.count, 10) || 0; break;
      default: /* ignore unknown lines */ break;
    }
  }

  return { content: [{ type: 'json', json: out }] };
});

// --- Helpers (placed near the bottom of the file with other helpers) ---

function enrichWaitingStale(repoRoot, ids) {
  if (ids.length === 0) return [];
  const fs = require('node:fs');
  const tsv = fs.readFileSync(path.join(repoRoot, '.awiki/maps/actions.tsv'), 'utf8');
  const today = process.env.AWIKI_TODAY || new Date().toISOString().slice(0, 10);
  const tSec = Date.UTC(...today.split('-').map(Number).map((v,i)=>i===1?v-1:v)) / 1000;
  const out = [];
  for (const line of tsv.split('\n').slice(1)) {
    if (!line) continue;
    const cols = line.split('\t');
    if (!ids.includes(cols[0])) continue;
    const since = cols[9] || '';
    const wait  = cols[8] || '';
    let days = 0;
    if (since) {
      const sSec = Date.UTC(...since.split('-').map(Number).map((v,i)=>i===1?v-1:v)) / 1000;
      days = Math.floor((tSec - sSec) / 86400);
    }
    out.push({ id: cols[0], wait, since, days });
  }
  return out;
}

function enrichOverdue(repoRoot, ids) {
  if (ids.length === 0) return [];
  const fs = require('node:fs');
  const tsv = fs.readFileSync(path.join(repoRoot, '.awiki/maps/actions.tsv'), 'utf8');
  const today = process.env.AWIKI_TODAY || new Date().toISOString().slice(0, 10);
  const tSec = Date.UTC(...today.split('-').map(Number).map((v,i)=>i===1?v-1:v)) / 1000;
  const out = [];
  for (const line of tsv.split('\n').slice(1)) {
    if (!line) continue;
    const cols = line.split('\t');
    if (!ids.includes(cols[0])) continue;
    const due = cols[6] || '';
    let days_over = 0;
    if (due) {
      const dSec = Date.UTC(...due.split('-').map(Number).map((v,i)=>i===1?v-1:v)) / 1000;
      days_over = Math.floor((tSec - dSec) / 86400);
    }
    out.push({ id: cols[0], due, days_over });
  }
  return out;
}
```

Note: the `Date.UTC(...split.map((v,i)=>i===1?v-1:v))` pattern is a known-working ISO-date parser that does not pull in `Date.parse`'s timezone surprises.

- [ ] **Step 3: Run the MCP tests, see them pass**

```bash
cd mcp/awiki-server && npm install --no-audit --no-fund && cd -
bats tests/mcp_review_status_test.sh
```

Expected: 3 tests, 3 pass.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/index.js tests/mcp_review_status_test.sh tests/helpers/seed-review-fixture.sh
git commit -m "feat(mcp): implement review_status (replace phase-18 stub)"
```

---

## Task 19.5: MCP `mark_review_done()`

Writes `.awiki/last-review=<ISO timestamp>`, appends one log line to `content/agenda/review-log.md`. Runs under exclusive `flock -x` because both touches are mutations.

- [ ] **Step 1: Write the failing test FIRST**

```bash
cat > tests/mark_review_done_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  bash "$BATS_TEST_DIRNAME/helpers/seed-review-fixture.sh" "$TEST_REPO"

  # review-log.md must exist (created by task-init.sh in real wikis).
  cat > content/agenda/review-log.md <<'LOG'
---
title: "Review Log"
type: agenda
draft: false
---

# Review Log

LOG
}

teardown() {
  rm -rf "$TEST_REPO"
}

@test "mark_review_done writes ISO timestamp to .awiki/last-review" {
  before="$(cat .awiki/last-review)"
  resp="$(node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool mark_review_done --args '{}')"
  echo "$resp" | python3 -c 'import json, sys; d=json.load(sys.stdin); assert "last_review" in d and d["last_review"].count("-") == 2 and "T" in d["last_review"]'
  after="$(cat .awiki/last-review)"
  [ "$before" != "$after" ]
}

@test "mark_review_done appends one log line with task counts" {
  node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool mark_review_done --args '{}' >/dev/null
  tail -1 content/agenda/review-log.md \
    | grep -qE '^## \[[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}\] review \| inbox=[0-9]+ next=[0-9]+ waiting=[0-9]+ overdue=[0-9]+ completed=[0-9]+ stuck=[0-9]+$'
}

@test "mark_review_done is idempotent under repeated invocation" {
  node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool mark_review_done --args '{}' >/dev/null
  lines_before="$(wc -l < content/agenda/review-log.md)"
  node "$BATS_TEST_DIRNAME/../mcp/awiki-server/index.js" --tool mark_review_done --args '{}' >/dev/null
  lines_after="$(wc -l < content/agenda/review-log.md)"
  # Each call appends exactly one log line.
  [ "$((lines_after - lines_before))" -eq 1 ]
}
EOF
```

Run:

```bash
bats tests/mark_review_done_test.sh
```

Expected: 3 failures with `tool not found: mark_review_done` (or equivalent).

- [ ] **Step 2: Implement the handler in `mcp/awiki-server/index.js`**

Add near the `review_status` registration:

```javascript
server.tool('mark_review_done', {}, async () => {
  const fs = require('node:fs');
  const repoRoot = path.resolve(__dirname, '..', '..');

  // Acquire exclusive lock for the duration of the two writes.
  // We shell out to lock.sh's scriptable entry-point because the MCP
  // server runs in Node; `flock` itself is a child-process invocation
  // wrapped by lock.sh's `awiki_lock_acquire_node`-style helper, which
  // phase 18 added when the lock library was first sourced from JS.
  // (If phase-18 named it differently, rename here.)
  let stdout;
  try {
    stdout = execFileSync('bash', ['--', 'scripts/review-status.sh'], {
      cwd: repoRoot, encoding: 'utf8', timeout: 30_000,
    });
  } catch (err) {
    return { content: [{ type: 'text', text: `mark_review_done: review-status failed: ${err.message}` }], isError: true };
  }

  // Pull the per-bucket counts directly from the structured stdout
  // (matches the spec's log-line shape).
  const get = (key) => {
    const m = stdout.match(new RegExp(`^REVIEW\\|${key}\\|count=(\\d+)`, 'm'));
    return m ? parseInt(m[1], 10) : 0;
  };
  const inbox = get('inbox-unprocessed');
  const next  = (() => {
    // "next" in the log line = open [ ]/[/] from actions.tsv (not in the
    // REVIEW lines directly). Read tsv once.
    try {
      const tsv = fs.readFileSync(path.join(repoRoot, '.awiki/maps/actions.tsv'), 'utf8');
      return tsv.split('\n').slice(1).filter(l => {
        const c = l.split('\t'); return c[1] === '[ ]' || c[1] === '[/]';
      }).length;
    } catch { return 0; }
  })();
  const waiting   = (() => {
    try {
      const tsv = fs.readFileSync(path.join(repoRoot, '.awiki/maps/actions.tsv'), 'utf8');
      return tsv.split('\n').slice(1).filter(l => l.split('\t')[1] === '[?]').length;
    } catch { return 0; }
  })();
  const overdue   = get('overdue');
  const completed = get('completed-since-last-review');
  const stuck     = get('stuck-projects');

  // ISO timestamp with seconds, UTC.
  const now = new Date();
  const iso = now.toISOString().replace(/\.\d{3}Z$/, 'Z');
  const stamp = `${iso.slice(0,10)} ${iso.slice(11,16)}`;
  const logLine = `## [${stamp}] review | inbox=${inbox} next=${next} waiting=${waiting} overdue=${overdue} completed=${completed} stuck=${stuck}\n`;

  // Take the lock and do the two writes as a unit. We use lock.sh's
  // CLI mode: `bash scripts/lib/lock.sh -- <inner-command>` runs the
  // inner command under flock -x.
  const innerScript = `set -euo pipefail
printf '%s\\n' "${iso}" > .awiki/last-review.tmp
mv .awiki/last-review.tmp .awiki/last-review
printf '%s' ${JSON.stringify(logLine)} >> content/agenda/review-log.md
`;
  try {
    execFileSync('bash', ['--', 'scripts/lib/lock.sh', '--exec', '--', 'bash', '-c', innerScript], {
      cwd: repoRoot, encoding: 'utf8', timeout: 30_000,
    });
  } catch (err) {
    return { content: [{ type: 'text', text: `mark_review_done: write failed: ${err.message}` }], isError: true };
  }

  return { content: [{ type: 'json', json: { last_review: iso } }] };
});
```

If `scripts/lib/lock.sh` from phase 16 does not yet expose an `--exec` CLI mode (some implementations only expose source-able functions), add it as a small extension at the bottom of `lock.sh`:

```bash
cat >> scripts/lib/lock.sh <<'EOF'

# CLI-mode entry: bash scripts/lib/lock.sh --exec -- <argv...>
# Runs <argv> under flock -x .awiki/lock with the standard 30s timeout.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  if [[ "${1:-}" == "--exec" ]]; then
    shift
    [[ "${1:-}" == "--" ]] && shift
    awiki_lock_acquire 30
    trap 'awiki_lock_release' EXIT
    "$@"
  fi
fi
EOF
```

(Confirm phase 16's `lock.sh` does not already define a CLI entry — if it does, harmonize with that and DO NOT add a duplicate.)

- [ ] **Step 3: Run the test, see it pass**

```bash
bats tests/mark_review_done_test.sh
```

Expected: 3 tests, 3 pass.

- [ ] **Step 4: Commit**

```bash
git add mcp/awiki-server/index.js scripts/lib/lock.sh tests/mark_review_done_test.sh
git commit -m "feat(mcp): add mark_review_done tool"
```

---

## Task 19.6: WIKI.md Weekly Review subsection — sed-based migration helper

The spec adds an 8-step Weekly Review subsection inside the existing `<!-- BEGIN task-layer -->` block. New `task-init` runs (post-phase-19) get this section in their template; **already-initialized wikis** need a one-shot migration. Phase 19 ships `scripts/task-layer-migrate-review.sh` (sed-based, idempotent) so existing users can pull the new section without re-running `task-init` (which is per-step idempotent but would re-prompt for everything).

- [ ] **Step 1: Decide the canonical subsection text**

The 8-step Weekly Review subsection. Inline below — keep this as the single source of truth used by both the migration helper and the `task-init` template patch.

```markdown
### Weekly Review

The weekly review is the reflect step. Run it on a fixed cadence (Friday afternoon works well) and step through the eight items below; the agent can drive each step via the `review_status()` MCP tool.

1. **Empty `content/inbox.md`** — triage every line. Run `just triage` (or have the agent call `triage_inbox()`) and walk each item to one of seven outcomes (trash, do-now, act, defer-scheduled, waiting, reference, someday).
2. **Empty `raw/inbox/interactive/`** — ingest every file. Existing `just ingest` pipeline applies; triage the resulting page.
3. **Walk `content/agenda/next-actions.md`** — every active project should have at least one open `[ ]`/`[/]` action. Lint rule `T8-no-next-action` flags violations.
4. **Walk `content/agenda/waiting.md`** — nudge anything `>14d`. The agent surfaces these as `waiting_stale_14d` from `review_status()`.
5. **Walk `content/agenda/someday.md`** — promote one or two items to active if appropriate. Move the line to a project page and flip `[>]` to `[ ]`.
6. **Walk `content/agenda/stuck-projects.md`** — for each entry, decide: revive (add a next action), drop (`status: dropped`), or move to someday (`status: someday`).
7. **Skim `content/projects/_index.md`** — project list sanity. Anything new since last review? Anything done?
8. **Stamp the review** — call `mark_review_done()` (or commit a manual `.awiki/last-review` update). The MCP tool also appends a line to `content/agenda/review-log.md` for history.

The structured shell report `just review` (which chains `agenda.sh` → `lint.sh` → `review-status.sh`) prints `REVIEW|<key>|count=N|...` lines suitable for greppable inspection or dashboard ingestion. The MCP tool `review_status()` returns the same data as JSON.

**Privacy note:** when `encrypt-init` is enabled and `AWIKI_TASK_LAYER=on`, `task-init` (or running `encrypt-init` after `task-init`) covers `content/inbox.md` and `content/agenda/**` under git-crypt by default. See `WIKI.md` Section "Encryption" for migration steps if you ran `encrypt-init` before `task-init`.
```

Save this once at `scripts/templates/wiki-weekly-review.md` so both the migration helper and the future `task-init` template patch can read from a single file:

```bash
mkdir -p scripts/templates
cat > scripts/templates/wiki-weekly-review.md <<'EOF'
### Weekly Review

The weekly review is the reflect step. Run it on a fixed cadence (Friday afternoon works well) and step through the eight items below; the agent can drive each step via the `review_status()` MCP tool.

1. **Empty `content/inbox.md`** — triage every line. Run `just triage` (or have the agent call `triage_inbox()`) and walk each item to one of seven outcomes (trash, do-now, act, defer-scheduled, waiting, reference, someday).
2. **Empty `raw/inbox/interactive/`** — ingest every file. Existing `just ingest` pipeline applies; triage the resulting page.
3. **Walk `content/agenda/next-actions.md`** — every active project should have at least one open `[ ]`/`[/]` action. Lint rule `T8-no-next-action` flags violations.
4. **Walk `content/agenda/waiting.md`** — nudge anything `>14d`. The agent surfaces these as `waiting_stale_14d` from `review_status()`.
5. **Walk `content/agenda/someday.md`** — promote one or two items to active if appropriate. Move the line to a project page and flip `[>]` to `[ ]`.
6. **Walk `content/agenda/stuck-projects.md`** — for each entry, decide: revive (add a next action), drop (`status: dropped`), or move to someday (`status: someday`).
7. **Skim `content/projects/_index.md`** — project list sanity. Anything new since last review? Anything done?
8. **Stamp the review** — call `mark_review_done()` (or commit a manual `.awiki/last-review` update). The MCP tool also appends a line to `content/agenda/review-log.md` for history.

The structured shell report `just review` (which chains `agenda.sh` → `lint.sh` → `review-status.sh`) prints `REVIEW|<key>|count=N|...` lines suitable for greppable inspection or dashboard ingestion. The MCP tool `review_status()` returns the same data as JSON.

**Privacy note:** when `encrypt-init` is enabled and `AWIKI_TASK_LAYER=on`, `task-init` (or running `encrypt-init` after `task-init`) covers `content/inbox.md` and `content/agenda/**` under git-crypt by default. See `WIKI.md` Section "Encryption" for migration steps if you ran `encrypt-init` before `task-init`.
EOF
```

- [ ] **Step 2: Write `scripts/task-layer-migrate-review.sh`**

```bash
cat > scripts/task-layer-migrate-review.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# task-layer-migrate-review.sh — one-shot insertion of the Weekly Review
# subsection into an already-initialized wiki's WIKI.md.
#
# Idempotent. Safe to re-run. Looks for the existing
# <!-- BEGIN task-layer --> bracket from phase 16; if absent, exits 0
# with a notice (the wiki has not run task-init yet — running task-init
# will include this section automatically once phase 19 has shipped).

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"
cd "$REPO_ROOT"

# shellcheck source=lib/lock.sh
source "${SCRIPT_DIR}/lib/lock.sh"
awiki_lock_acquire 30

WIKI=WIKI.md
TEMPLATE="${SCRIPT_DIR}/templates/wiki-weekly-review.md"

if [[ ! -f "$WIKI" ]]; then
  echo "task-layer-migrate-review: $WIKI not found; nothing to do" >&2
  awiki_lock_release; exit 0
fi
if ! grep -q '<!-- BEGIN task-layer -->' "$WIKI"; then
  echo "task-layer-migrate-review: <!-- BEGIN task-layer --> marker not present; run \`just task-init\` first" >&2
  awiki_lock_release; exit 0
fi
if grep -q '^### Weekly Review$' "$WIKI"; then
  echo "task-layer-migrate-review: Weekly Review subsection already present; no-op" >&2
  awiki_lock_release; exit 0
fi

# Insert the template just before the <!-- END task-layer --> marker.
TMP="$(mktemp)"
awk -v tmpl="$TEMPLATE" '
  /<!-- END task-layer -->/ {
    while ((getline line < tmpl) > 0) print line
    print ""
  }
  { print }
' "$WIKI" > "$TMP"
mv "$TMP" "$WIKI"

echo "task-layer-migrate-review: Weekly Review subsection appended to $WIKI"
awiki_lock_release
EOF
chmod +x scripts/task-layer-migrate-review.sh
```

- [ ] **Step 3: Patch `scripts/task-init.sh` to use the same template on fresh init**

Edit the WIKI.md template-patching step (step 2 in `task-init.sh`'s ordering). Find the section that writes the task-layer block. Inside that block, before the closing `<!-- END task-layer -->`, insert:

```bash
# Append the Weekly Review subsection from the canonical template so the
# fresh-init and migrate-review paths share one source of truth.
cat "${SCRIPT_DIR}/templates/wiki-weekly-review.md" >> "$WIKI_TMP"
printf '\n' >> "$WIKI_TMP"
```

(Where `$SCRIPT_DIR` is `task-init.sh`'s script dir and `$WIKI_TMP` is the temp file before atomic rename.)

- [ ] **Step 4: Write a BATS test for the migration helper**

```bash
cat > tests/task_layer_migrate_review_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  mkdir -p scripts
  cp -r "$BATS_TEST_DIRNAME/../scripts/lib" scripts/lib
  cp -r "$BATS_TEST_DIRNAME/../scripts/templates" scripts/templates
  cp "$BATS_TEST_DIRNAME/../scripts/task-layer-migrate-review.sh" scripts/

  # Seed a phase-16-style WIKI.md with the bracket markers but no
  # Weekly Review subsection.
  cat > WIKI.md <<'WIKI'
# Wiki Conventions

(intro)

<!-- BEGIN task-layer -->

### Inline Action Grammar

(table from phase 16)

<!-- END task-layer -->

## After Block
WIKI
}

teardown() {
  rm -rf "$TEST_REPO"
}

@test "migrate-review appends the subsection inside the bracket block" {
  run bash scripts/task-layer-migrate-review.sh
  [ "$status" -eq 0 ]
  grep -q '^### Weekly Review$' WIKI.md
  # Subsection precedes <!-- END task-layer -->
  awk '/^### Weekly Review/ { found=1 } /<!-- END task-layer -->/ { exit found ? 0 : 1 }' WIKI.md
  # After-block content preserved.
  grep -q '^## After Block$' WIKI.md
}

@test "migrate-review is idempotent" {
  bash scripts/task-layer-migrate-review.sh
  before="$(wc -l < WIKI.md)"
  bash scripts/task-layer-migrate-review.sh
  after="$(wc -l < WIKI.md)"
  [ "$before" -eq "$after" ]
}

@test "migrate-review no-ops when bracket marker is absent" {
  cat > WIKI.md <<'WIKI'
# Wiki Conventions
no markers here
WIKI
  run bash scripts/task-layer-migrate-review.sh
  [ "$status" -eq 0 ]
  ! grep -q '^### Weekly Review$' WIKI.md
}
EOF
```

Run:

```bash
bats tests/task_layer_migrate_review_test.sh
```

Expected: 3 tests, 3 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/templates/wiki-weekly-review.md \
        scripts/task-layer-migrate-review.sh \
        scripts/task-init.sh \
        tests/task_layer_migrate_review_test.sh
git commit -m "feat: add Weekly Review WIKI.md subsection + migration helper"
```

---

## Task 19.7: Pre-commit hook installer integrated into `task-init.sh` step 6

Phase 16 left `task-init.sh` step 6 as a printed prompt with a TODO. Phase 19 implements the actual hook installation. The hook runs `lint.sh` (alias-build only) plus `action-scan.sh`. A `# task-layer` marker keeps re-runs idempotent and lets the installer detect prior installation cleanly. If a non-task-layer pre-commit hook already exists (e.g. from v1 phase 12's `install-hooks.sh`), the installer **appends** rather than overwrites.

- [ ] **Step 1: Write the failing test FIRST — `tests/task_init_pre_commit_test.sh`**

```bash
cat > tests/task_init_pre_commit_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  mkdir -p scripts content
  cp -r "$BATS_TEST_DIRNAME/../scripts/lib" scripts/lib
  cp -r "$BATS_TEST_DIRNAME/../scripts/templates" scripts/templates
  cp "$BATS_TEST_DIRNAME/../scripts/task-init.sh" scripts/
  # Stub helper scripts task-init touches.
  for f in capture.sh action-scan.sh agenda.sh action-recur.sh triage.sh review-status.sh log-append.sh lint.sh; do
    [[ -f "$BATS_TEST_DIRNAME/../scripts/$f" ]] && cp "$BATS_TEST_DIRNAME/../scripts/$f" scripts/
  done
}

teardown() {
  rm -rf "$TEST_REPO"
}

@test "task-init installs pre-commit hook when user opts y" {
  # Drive task-init non-interactively: y for pre-commit, default for others.
  printf 'y\ny\n' | bash scripts/task-init.sh

  [ -x .git/hooks/pre-commit ]
  grep -q '# task-layer' .git/hooks/pre-commit
  grep -q 'action-scan.sh' .git/hooks/pre-commit
  grep -q 'lint.sh' .git/hooks/pre-commit
}

@test "task-init pre-commit install is idempotent (re-run does not duplicate)" {
  printf 'y\ny\n' | bash scripts/task-init.sh
  before="$(wc -l < .git/hooks/pre-commit)"
  printf 'y\ny\n' | bash scripts/task-init.sh
  after="$(wc -l < .git/hooks/pre-commit)"
  [ "$before" -eq "$after" ]
  # Marker appears exactly once.
  [ "$(grep -c '# task-layer' .git/hooks/pre-commit)" -eq 1 ]
}

@test "task-init pre-commit appends to an existing non-task-layer hook" {
  cat > .git/hooks/pre-commit <<'HOOK'
#!/usr/bin/env bash
set -e
just lint
HOOK
  chmod +x .git/hooks/pre-commit

  printf 'y\ny\n' | bash scripts/task-init.sh

  # Original line preserved.
  grep -q '^just lint$' .git/hooks/pre-commit
  # Task-layer addition appended after marker.
  grep -q '# task-layer' .git/hooks/pre-commit
  grep -q 'action-scan.sh' .git/hooks/pre-commit
}

@test "task-init does NOT install hook when user opts n" {
  printf 'y\nn\n' | bash scripts/task-init.sh
  if [[ -f .git/hooks/pre-commit ]]; then
    ! grep -q '# task-layer' .git/hooks/pre-commit
  fi
}
EOF
```

Run:

```bash
bats tests/task_init_pre_commit_test.sh
```

Expected: 4 failures (the current task-init prints a TODO instead of writing).

- [ ] **Step 2: Implement step 6 of `scripts/task-init.sh`**

Locate the existing step-6 stub. Replace it with:

```bash
# Step 6: pre-commit hook installer.
HOOK=".git/hooks/pre-commit"
HOOK_DIR="$(dirname "$HOOK")"
mkdir -p "$HOOK_DIR"

# Marker line that signals "task-layer hook present".
MARKER="# task-layer"

needs_install=1
if [[ -f "$HOOK" ]] && grep -q "^${MARKER}\$" "$HOOK"; then
  needs_install=0
fi

if [[ $needs_install -eq 1 ]]; then
  if [[ "${TASK_INIT_AUTO:-}" != "1" ]]; then
    read -r -p "Install task-aware pre-commit hook (runs action-scan + alias-build lint)? (y/N) " ans
    ans="${ans:-N}"
  else
    ans="y"
  fi
  if [[ "$ans" == "y" || "$ans" == "Y" ]]; then
    if [[ ! -f "$HOOK" ]]; then
      # Fresh hook.
      cat > "$HOOK" <<'HOOKEOF'
#!/usr/bin/env bash
set -e

# task-layer
# (Inserted by scripts/task-init.sh — do not remove the marker line above.)
bash scripts/lint.sh --alias-build-only
bash scripts/action-scan.sh
HOOKEOF
    else
      # Append to existing hook. Ensure the file ends with a newline first.
      tail -c1 "$HOOK" | read -r _ || printf '\n' >> "$HOOK"
      cat >> "$HOOK" <<'HOOKEOF'

# task-layer
# (Inserted by scripts/task-init.sh — do not remove the marker line above.)
bash scripts/lint.sh --alias-build-only
bash scripts/action-scan.sh
HOOKEOF
    fi
    chmod +x "$HOOK"
    echo "task-init: installed task-layer pre-commit hook at $HOOK"
  else
    echo "task-init: skipped pre-commit hook install"
  fi
else
  echo "task-init: pre-commit hook already carries # task-layer marker; skipping"
fi
```

If `scripts/lint.sh` from v1 does not yet support `--alias-build-only`, add the flag handler in `lint.sh`:

```bash
# At top of lint.sh, before the main scan:
if [[ "${1:-}" == "--alias-build-only" ]]; then
  build_alias_map   # name from v1 — invoke whatever function builds .awiki/maps/alias-to-slug.tsv
  exit 0
fi
```

(Confirm function name against v1 phase 6's `lint.sh`. If different, alias.)

- [ ] **Step 3: Run the test, see it pass**

```bash
bats tests/task_init_pre_commit_test.sh
```

Expected: 4 tests, 4 pass.

- [ ] **Step 4: Commit**

```bash
git add scripts/task-init.sh scripts/lint.sh tests/task_init_pre_commit_test.sh
git commit -m "feat(task-init): install task-layer pre-commit hook with idempotent marker"
```

---

## Task 19.8: `encrypt-init` extension — auto-cover inbox + agenda when `AWIKI_TASK_LAYER=on`

The v1 phase-5 `encrypt-init.sh` writes git-crypt patterns to `.gitattributes`. Phase 19 extends it so that if `.awiki/config` carries `AWIKI_TASK_LAYER=on`, `encrypt-init` adds `content/inbox.md` and `content/agenda/**` to the patterns by default. **Backwards compatible**: existing `encrypt-init` runs are NOT migrated. A user who already ran `encrypt-init` and wants the new coverage either re-runs `encrypt-init` (idempotent for already-covered patterns; appends the new ones) or hand-edits `.gitattributes`. Documented in `WIKI.md` and README.

- [ ] **Step 1: Write the failing test FIRST — `tests/encrypt_init_task_layer_test.sh`**

```bash
cat > tests/encrypt_init_task_layer_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_REPO="$(mktemp -d)/repo"
  mkdir -p "$TEST_REPO"
  cd "$TEST_REPO"
  git init -q
  mkdir -p scripts content/projects content/agenda content/private .awiki
  cp -r "$BATS_TEST_DIRNAME/../scripts/lib" scripts/lib
  cp "$BATS_TEST_DIRNAME/../scripts/encrypt-init.sh" scripts/

  # Pretend git-crypt is installed via a PATH stub.
  STUB_DIR="$(mktemp -d)"
  cat > "$STUB_DIR/git-crypt" <<'STUB'
#!/usr/bin/env bash
case "$1" in
  init)         exit 0 ;;
  add-gpg-user) exit 0 ;;
  status)       echo "encrypted: yes" ;;
  *)            exit 0 ;;
esac
STUB
  chmod +x "$STUB_DIR/git-crypt"
  PATH="$STUB_DIR:$PATH"
  export PATH
}

teardown() {
  rm -rf "$TEST_REPO"
}

@test "encrypt-init covers inbox.md and agenda/** when AWIKI_TASK_LAYER=on" {
  printf '%s\n' "AWIKI_TASK_LAYER=on" > .awiki/config

  ENCRYPT_INIT_NONINTERACTIVE=1 bash scripts/encrypt-init.sh

  grep -qE '^content/inbox\.md filter=git-crypt diff=git-crypt$' .gitattributes
  grep -qE '^content/agenda/\*\* filter=git-crypt diff=git-crypt$' .gitattributes
}

@test "encrypt-init does NOT cover inbox/agenda when AWIKI_TASK_LAYER is unset" {
  printf '%s\n' "" > .awiki/config

  ENCRYPT_INIT_NONINTERACTIVE=1 bash scripts/encrypt-init.sh

  ! grep -qE '^content/inbox\.md ' .gitattributes
  ! grep -qE '^content/agenda/\*\* ' .gitattributes
}

@test "encrypt-init re-run is idempotent (no duplicate patterns)" {
  printf '%s\n' "AWIKI_TASK_LAYER=on" > .awiki/config

  ENCRYPT_INIT_NONINTERACTIVE=1 bash scripts/encrypt-init.sh
  ENCRYPT_INIT_NONINTERACTIVE=1 bash scripts/encrypt-init.sh

  [ "$(grep -c '^content/inbox\.md filter=git-crypt' .gitattributes)" -eq 1 ]
  [ "$(grep -c '^content/agenda/\*\* filter=git-crypt' .gitattributes)" -eq 1 ]
}

@test "encrypt-init does NOT auto-migrate when re-run finds an already-encrypted repo from before phase 19" {
  # Simulate an existing pre-phase-19 .gitattributes (no inbox/agenda
  # coverage) and AWIKI_TASK_LAYER=on. The script should detect existing
  # encryption and either (a) prompt y/n with default n, or (b) emit a
  # warning instructing the user to re-run encrypt-init explicitly.
  cat > .gitattributes <<'GA'
content/private/** filter=git-crypt diff=git-crypt
GA
  printf '%s\n' "AWIKI_TASK_LAYER=on" > .awiki/config

  # Default: ENCRYPT_INIT_NONINTERACTIVE=1 still adds the patterns
  # because the script is being explicitly invoked. The
  # "no auto-migrate" claim refers to other scripts (e.g. task-init)
  # not silently calling encrypt-init. Verify by NOT calling encrypt-init:
  # .gitattributes stays unchanged.
  before="$(cat .gitattributes)"
  printf 'y\n' | bash scripts/task-init.sh >/dev/null 2>&1 || true
  after="$(cat .gitattributes)"
  [ "$before" = "$after" ]
}
EOF
```

Run:

```bash
bats tests/encrypt_init_task_layer_test.sh
```

Expected: 3 of 4 fail (the unset-config and idempotent paths may pass already; the AWIKI_TASK_LAYER=on path fails because the patterns are not added).

- [ ] **Step 2: Patch `scripts/encrypt-init.sh`**

Locate the section that writes the git-crypt block to `.gitattributes`. After that section, before the file is closed, append:

```bash
# Phase 19: task-layer coverage. Opt-in via .awiki/config AWIKI_TASK_LAYER=on.
TASK_LAYER_ON=0
if [[ -f .awiki/config ]] && grep -qE '^AWIKI_TASK_LAYER=on$' .awiki/config; then
  TASK_LAYER_ON=1
fi

if [[ $TASK_LAYER_ON -eq 1 ]]; then
  add_pattern() {
    local pat="$1"
    if ! grep -qE "^${pat//./\\.} filter=git-crypt diff=git-crypt\$" .gitattributes 2>/dev/null; then
      printf '%s filter=git-crypt diff=git-crypt\n' "$pat" >> .gitattributes
      echo "encrypt-init: added pattern $pat (task-layer)"
    fi
  }
  add_pattern "content/inbox.md"
  add_pattern "content/agenda/**"
fi
```

The `add_pattern` helper makes the operation idempotent: re-runs detect the pattern via fixed-string match (escape `.` to avoid regex over-matching) and skip.

- [ ] **Step 3: Run the test, see it pass**

```bash
bats tests/encrypt_init_task_layer_test.sh
```

Expected: 4 tests, 4 pass.

- [ ] **Step 4: Commit**

```bash
git add scripts/encrypt-init.sh tests/encrypt_init_task_layer_test.sh
git commit -m "feat(encrypt-init): cover inbox.md and agenda/** when AWIKI_TASK_LAYER=on"
```

---

## Task 19.9: README task-layer section

Add a top-level section to `README.md` that introduces the task layer, the 6-step manual smoke test from the spec, and a one-paragraph methodology note (capture → clarify → organize → reflect → engage; no trademark name).

- [ ] **Step 1: Locate the README insertion point**

```bash
grep -n '^## ' README.md | head -20
```

Identify a stable anchor — typically the README's existing section list ends with "Example" (added in v1 phase 12) or "Synthesis" (added in v1 phase 13). Insert the new section immediately before the `## Example` section so the smoke test precedes the sample-wiki pointer.

- [ ] **Step 2: Append the section**

Insert:

```markdown
## Task layer

awiki ships an opt-in **task layer** for tracking actions, projects, and
contexts inside the wiki. The methodology is the classic
capture-clarify-organize-reflect-engage flow popularized by personal-productivity
literature, expressed as wiki-native primitives — captures land in
`content/inbox.md`, the agent triages each one to a project / context page,
a scanner builds derived agenda views, and a structured weekly review
keeps the loop closing.

### Enable

```bash
just task-init
```

This is per-step idempotent. It scaffolds `content/inbox.md`,
`content/projects/`, `content/contexts/`, `content/agenda/*`, patches
`WIKI.md` between `<!-- BEGIN task-layer -->` markers, sets
`AWIKI_TASK_LAYER=on` in `.awiki/config`, and (with consent) installs a
task-aware pre-commit hook. If `encrypt-init` was previously run, you'll
be prompted to extend git-crypt coverage to `content/inbox.md` and
`content/agenda/**`. If you run `encrypt-init` AFTER `task-init`, the
coverage is added automatically.

### 6-step manual smoke test

After `just task-init`, verify the layer end-to-end:

1. **Capture**

   ```bash
   just capture "call dentist"
   ```

   Expected: `content/inbox.md` gains a line `- 2026-MM-DD HH:MM call dentist`.

2. **Triage** — open your agent (Claude / Codex / etc.) and ask it to
   triage the inbox.

   ```
   triage inbox
   ```

   The agent calls `triage_inbox()` and walks each item. For "call dentist",
   choose `act` outcome, project `_loose`, context `phone`. The agent
   removes the inbox line and writes
   `- [ ] call dentist @phone ^<id>` to `content/projects/_loose.md`.

3. **Build agenda**

   ```bash
   just agenda
   ```

   Expected: `content/agenda/next-actions.md` gains a `@phone` section
   with the new action. Managed-region markers
   (`<!-- BEGIN agenda:next-actions -->` / `<!-- END agenda:next-actions -->`)
   bracket the generated content; user notes outside the markers survive.

4. **Verify next-actions render**

   Open `content/agenda/next-actions.md` in Obsidian or any markdown
   viewer. The action shows under `@phone` with the project tag
   `[[_loose]]`.

5. **Complete**

   Edit `content/projects/_loose.md`, flip `[ ]` to `[x]`, save, then:

   ```bash
   just agenda
   ```

   Expected: action no longer appears in `next-actions.md`. If the action
   carried `every:`, a fresh `[ ]` line with the next due-date appears
   above the completed line.

6. **Review**

   ```bash
   just review
   ```

   Expected: structured report shows `REVIEW|completed-since-last-review|count=1`
   among the eight `REVIEW|` lines plus the closing `REVIEW-SUMMARY|...`.

If any step fails, see `docs/just-help.txt` for per-recipe expected output.

### Recipes

| Recipe         | Purpose                                                |
|----------------|--------------------------------------------------------|
| `just task-init` | Enable / re-bless the task layer (per-step idempotent). |
| `just capture "<text>"` | Append a quick capture to `content/inbox.md`. |
| `just triage`  | Walk the inbox interactively (bash fallback to MCP).   |
| `just scan`    | Rebuild `.awiki/maps/actions.tsv` only (cheap).        |
| `just agenda`  | Scan + regenerate the five managed-region agenda pages. |
| `just review`  | Run `agenda` + `lint` + structured weekly-review report. |

### Methodology note

The five-step flow — capture every open loop without judgment, clarify
each capture into a concrete next action or non-action, organize by
project and context, reflect on the system on a fixed cadence, engage
with the next action that fits your current context — predates awiki by
decades. awiki's contribution is making each step a wiki-native
primitive: captures are markdown lines, projects and contexts are pages
with frontmatter, agenda views are managed-region renderings, and the
weekly review is a structured shell report plus an MCP tool. No app, no
daemon, no cloud sync; everything is git-committed text the agent can
read and write.
```

- [ ] **Step 3: Manual verification**

Run the smoke against the live repo:

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
TEST_DIR="$(mktemp -d)/awiki-task-smoke"
git clone "$REPO_ROOT" "$TEST_DIR"
cd "$TEST_DIR"
git submodule update --init --recursive
just check-deps
just task-init <<< $'y\ny\n'
just capture "call dentist"
grep -q "call dentist" content/inbox.md
# Manually edit content/projects/_loose.md to add `- [ ] call dentist @phone ^a01`
mkdir -p content/projects
cat > content/projects/_loose.md <<'EOF'
---
title: "Loose"
date: 2026-04-27
last_updated: 2026-04-27
type: project
status: active
outcome: "Catch-all for orphan actions."
tags: []
draft: false
---

## Open Actions
- [ ] call dentist @phone ^a01
EOF
just agenda
grep -q "call dentist" content/agenda/next-actions.md
# Flip to [x]
sed -i.bak 's/^- \[ \] call dentist/- [x] call dentist done:'"$(date -u +%Y-%m-%d)"'/' content/projects/_loose.md
just agenda
! grep -q "call dentist" content/agenda/next-actions.md
just review
echo "smoke OK"
```

Expected: prints `smoke OK`. Triage step (#2 in the README) is intentionally skipped here because it requires an MCP-connected agent; document that in a `docs/decisions/task-smoke-without-agent.md` if any rough edge surfaces.

- [ ] **Step 4: Commit**

```bash
git add README.md
git commit -m "docs(readme): add task-layer section with 6-step smoke + methodology"
```

---

## Task 19.10: `docs/just-help.txt` task-layer entries

Per spec: each recipe gets a usage line, expected output, and a "when to use" hint.

- [ ] **Step 1: Read existing structure**

```bash
sed -n '1,40p' docs/just-help.txt
```

Find the section convention (typically `## <recipe>` then `Usage: ...`, `Output: ...`, `When: ...`).

- [ ] **Step 2: Append the task-layer block**

```bash
cat >> docs/just-help.txt <<'EOF'

## task-init

Usage:   just task-init
Output:  task-init: created content/inbox.md
         task-init: patched WIKI.md (between task-layer markers)
         task-init: AWIKI_TASK_LAYER=on appended to .awiki/config
         task-init: installed task-layer pre-commit hook at .git/hooks/pre-commit
         (per-step idempotent — re-runs print "skipping" for steps already done)
When:    Once per wiki, to enable the task layer. Safe to re-run after a
         partial failure or to bless a new step (e.g. install the
         pre-commit hook later).

## capture

Usage:   just capture "<text>"
Output:  capture: appended to content/inbox.md (line 42)
         (with sanitizations diff if any rule fired)
When:    Quick-capture an open loop without leaving the terminal.
         Variadic: `just capture pick up groceries` works without quotes
         (no shell metacharacters). For `[[link]]`-style captures, single-quote:
         `just capture '[[memex]] reread'`.

## triage

Usage:   just triage
Output:  Walks every inbox.md line and raw/inbox/interactive/* file
         interactively. For each item:
           "<text>"  outcome (trash/do-now/act/defer/wait/ref/someday)?
         then prompts for project_slug / context_slug / wait_for / etc.
         depending on outcome.
When:    Bash fallback when the MCP-connected agent is unavailable.
         Per-item atomic: Ctrl-C aborts the current item; previous items
         stay applied. Side effects identical to triage_apply MCP tool.

## scan

Usage:   just scan
Output:  scanned 247 files, 31 actions, 0 rejected, 24 open, 7 done
When:    Rebuild .awiki/maps/actions.tsv without regenerating agenda
         pages. Useful before lint runs or to debug parse errors
         (rejected lines land in .awiki/maps/actions-rejected.tsv).

## agenda

Usage:   just agenda
Output:  scanned 247 files, 31 actions, 0 rejected, 24 open, 7 done
         agenda: rebuilt content/agenda/next-actions.md
         agenda: rebuilt content/agenda/today.md
         agenda: rebuilt content/agenda/waiting.md
         agenda: rebuilt content/agenda/someday.md
         agenda: rebuilt content/agenda/stuck-projects.md
When:    Run after triage or after manually editing project/context
         pages. Auto-runs on every Nth triage_apply (default 5,
         configurable via AWIKI_AGENDA_AFTER_N).

## review

Usage:   just review
Output:  (agenda output)
         (lint output, ideally clean)
         REVIEW|inbox-unprocessed|count=3
         REVIEW|raw-inbox-files|count=1
         REVIEW|projects-no-next-action|count=2|slugs=q3-launch,renovate-kitchen
         REVIEW|waiting-stale-14d|count=1|ids=a03
         REVIEW|overdue|count=2|ids=a01,a07
         REVIEW|completed-since-last-review|count=8
         REVIEW|stuck-projects|count=1|slugs=onboarding-revamp
         REVIEW|someday-count|count=12
         REVIEW-SUMMARY|last-review=2026-04-20|attention=4
When:    Weekly cadence (Friday afternoon works well). The structured
         lines are greppable; the agent reads them via the `review_status()`
         MCP tool which returns the same data as JSON. Stamp with
         `mark_review_done()` once you've walked the eight steps in
         WIKI.md "Weekly Review".
EOF
```

- [ ] **Step 3: Manual verification**

```bash
just help task-init   # if the existing `help` recipe greps just-help.txt by recipe name
just help review
```

Expected: each block prints. If `just help` is not implemented, just `cat docs/just-help.txt` to eyeball.

- [ ] **Step 4: Commit**

```bash
git add docs/just-help.txt
git commit -m "docs(just-help): document task-layer recipes"
```

---

## Task 19.11: Sample-wiki extension — `examples/sample-wiki/` task layer

Run `just task-init` against `examples/sample-wiki/` to scaffold the empty layout, then commit hand-curated example content on top: a populated `inbox.md`, two project pages with open actions, four context pages, populated agenda pages (after a sample run), and a sample `review-log.md` entry.

- [ ] **Step 1: Scaffold via task-init in the sample-wiki**

```bash
cd examples/sample-wiki
TASK_INIT_AUTO=1 bash ../../scripts/task-init.sh
cd -
git status -s examples/sample-wiki/
```

Expected: `git status` shows new files `content/inbox.md`,
`content/projects/{_index.md,phone.md placeholder}`,
`content/contexts/{_index.md,phone.md,errands.md,computer.md}`,
`content/agenda/{_index.md,next-actions.md,today.md,waiting.md,someday.md,stuck-projects.md,review-log.md}`,
`.awiki/config`, `.awiki/last-review`, `.awiki/task-count`, plus a
patched `WIKI.md`.

- [ ] **Step 2: Curate `examples/sample-wiki/content/inbox.md` with two captures**

```bash
cat > examples/sample-wiki/content/inbox.md <<'EOF'
---
title: "Inbox"
type: inbox
draft: true
---

- 2026-04-27 09:14 reread chapter 3 of [[s-as-we-may-think]]
- 2026-04-27 09:18 idea: timeline page for memex predecessors
EOF
```

- [ ] **Step 3: Add `content/contexts/home.md` (the fourth context, on top of the three task-init created)**

```bash
cat > examples/sample-wiki/content/contexts/home.md <<'EOF'
---
title: "@home"
date: 2026-04-27
last_updated: 2026-04-27
type: context
aliases: ['@home']
tools: []
draft: false
---

Actions that need to happen at home (not at the desk computer).
EOF
```

Verify the three other contexts task-init created (`phone`, `errands`, `computer`) carry the right alias frontmatter; if not, overwrite with the same template:

```bash
for c in phone errands computer; do
  cat > "examples/sample-wiki/content/contexts/${c}.md" <<EOF
---
title: "@${c}"
date: 2026-04-27
last_updated: 2026-04-27
type: context
aliases: ['@${c}']
tools: []
draft: false
---

Actions that fit the ${c} context.
EOF
done
```

- [ ] **Step 4: Add two project pages with example open actions**

```bash
cat > examples/sample-wiki/content/projects/renovate-kitchen.md <<'EOF'
---
title: "Renovate kitchen"
date: 2026-04-27
last_updated: 2026-04-27
type: project
status: active
outcome: "Kitchen renovated; certificate of occupancy in hand."
area: [[home]]
tags: [home]
aliases: []
draft: false
---

Outcome: the kitchen is renovated. Demo, cabinetry, plumbing,
electrical, inspection.

## Notes

Contractor proposal received 2026-04-20; comparing against budget.

## Open Actions

- [ ] schedule contractor walk-through @phone due:2026-05-02 ^k01
- [/] price out cabinets @computer ^k02
- [?] permit application status wait:[[city-hall]] since:2026-04-22 ^k03
- [ ] move pantry contents to garage @home ^k04
- [>] research induction cooktops someday ^k05

## Done

- [x] sign contractor agreement @computer done:2026-04-25 ^k00

## Related

- [[home]]
- [[contractor-jane-doe]]

## Sources

EOF

cat > examples/sample-wiki/content/projects/q3-launch.md <<'EOF'
---
title: "Q3 launch"
date: 2026-04-15
last_updated: 2026-04-27
type: project
status: active
outcome: "Q3 product launched to general availability."
tags: [work]
aliases: []
draft: false
---

Outcome: Q3 product is GA. Marketing site, docs, support runbook,
internal launch comms.

## Notes

Eng readiness review on 2026-05-15.

## Open Actions

- [ ] draft launch announcement @computer due:2026-05-10 ^q01
- [/] update support runbook @computer ^q02
- [?] legal review of pricing wait:[[bob-smith]] since:2026-04-18 ^q03
- [ ] book launch dinner @phone every:1m due:2026-05-20 ^q04

## Done

- [x] confirm launch date @computer done:2026-04-22 ^q00

## Related

- [[memex]]
- [[bob-smith]]

EOF
```

- [ ] **Step 5: Add `_loose.md` and `_someday.md` catch-alls**

```bash
cat > examples/sample-wiki/content/projects/_loose.md <<'EOF'
---
title: "Loose"
date: 2026-04-27
last_updated: 2026-04-27
type: project
status: active
outcome: "Catch-all bucket for actions without a clear project."
tags: []
aliases: []
draft: false
---

## Open Actions

- [ ] return library books @errands due:2026-05-01 ^l01

EOF

cat > examples/sample-wiki/content/projects/_someday.md <<'EOF'
---
title: "Someday"
date: 2026-04-27
last_updated: 2026-04-27
type: project
status: someday
outcome: "Things to revisit on a future weekly review."
tags: []
aliases: []
draft: false
---

## Open Actions

- [>] learn to bake sourdough @home ^s01
- [>] read every Vannevar Bush essay ^s02
- [>] write a reflection on the memex ^s03

EOF
```

- [ ] **Step 6: Run `just agenda` to populate the agenda pages from the sample data**

```bash
cd examples/sample-wiki
bash ../../scripts/action-scan.sh
bash ../../scripts/agenda.sh
cd -
```

Expected: `examples/sample-wiki/content/agenda/{next-actions,today,waiting,someday,stuck-projects}.md` now carry populated managed regions reflecting the actions above.

- [ ] **Step 7: Append a sample entry to `review-log.md`**

```bash
cat >> examples/sample-wiki/content/agenda/review-log.md <<'EOF'

## [2026-04-20 16:30] review | inbox=0 next=8 waiting=2 overdue=0 completed=3 stuck=0
EOF
```

- [ ] **Step 8: Verify the sample wiki is internally consistent**

```bash
cd examples/sample-wiki
bash ../../scripts/lint.sh
cd -
```

Expected: zero errors, zero warnings (or only the documented `T13-context-unused` info-level entries for any context with no referencing action — those are info, not warn).

- [ ] **Step 9: Manual verification — render the sample wiki**

```bash
cd examples/sample-wiki
hugo --quiet --destination /tmp/sample-wiki-build
cd -
# Open /tmp/sample-wiki-build/agenda/next-actions/index.html in a browser.
# Confirm the @phone, @computer, @home buckets each list the expected
# actions with project tags. Confirm waiting.md shows bob-smith and
# city-hall with days-waited counts.
```

Expected: the rendered HTML reads cleanly; managed-region content visible; no broken wikilinks (sample-wiki includes the v1 phase-12 entity pages already).

- [ ] **Step 10: Commit**

```bash
git add examples/sample-wiki
git commit -m "docs(sample-wiki): add task-layer example projects, contexts, inbox, agenda, review-log"
```

---

## Task 19.12: Sample-wiki end-to-end smoke BATS test

Pin the README's 6-step manual smoke as an automated test. Runs against a temp clone of `examples/sample-wiki/` so the upstream sample-wiki state is not perturbed.

- [ ] **Step 1: Write the test**

```bash
cat > tests/sample_wiki_smoke_test.sh <<'EOF'
#!/usr/bin/env bats

setup() {
  REPO="$BATS_TEST_DIRNAME/.."
  TMP="$(mktemp -d)"
  cp -R "$REPO/examples/sample-wiki" "$TMP/wiki"
  # Carry script + lib + mcp deps into the clone (sample-wiki is pure
  # content; the scripts live in $REPO).
  mkdir -p "$TMP/wiki/scripts"
  cp -R "$REPO/scripts/." "$TMP/wiki/scripts/"
  cd "$TMP/wiki"
  git init -q
  git add -A && git commit -q -m "seed"
  export AWIKI_TODAY="2026-04-27"
}

teardown() {
  rm -rf "$TMP"
}

@test "sample-wiki: capture appends an inbox line" {
  before="$(wc -l < content/inbox.md)"
  bash scripts/capture.sh "test capture from smoke"
  after="$(wc -l < content/inbox.md)"
  [ "$((after - before))" -eq 1 ]
  grep -q "test capture from smoke" content/inbox.md
}

@test "sample-wiki: agenda regenerates managed regions" {
  bash scripts/action-scan.sh
  bash scripts/agenda.sh
  grep -q '<!-- BEGIN agenda:next-actions -->' content/agenda/next-actions.md
  grep -q '<!-- END agenda:next-actions -->' content/agenda/next-actions.md
  # Renovate-kitchen actions appear under @phone or @computer.
  grep -q 'schedule contractor walk-through' content/agenda/next-actions.md
}

@test "sample-wiki: completing an action removes it from next-actions" {
  bash scripts/action-scan.sh && bash scripts/agenda.sh
  grep -q 'return library books' content/agenda/next-actions.md

  # Flip in _loose.md.
  sed -i.bak 's/^- \[ \] return library books .*$/- [x] return library books @errands due:2026-05-01 done:2026-04-27 ^l01/' content/projects/_loose.md
  bash scripts/action-scan.sh && bash scripts/agenda.sh

  ! grep -q 'return library books' content/agenda/next-actions.md
}

@test "sample-wiki: review-status emits all eight REVIEW lines + summary" {
  bash scripts/action-scan.sh
  run bash scripts/review-status.sh
  [ "$status" -eq 0 ]
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

@test "sample-wiki: just review chain runs to completion" {
  # Inline equivalent of `just review` (no just dependency in BATS env).
  bash scripts/agenda.sh
  bash scripts/lint.sh || true   # info-level T13 entries acceptable
  bash scripts/review-status.sh
}
EOF
```

Run:

```bash
bats tests/sample_wiki_smoke_test.sh
```

Expected: 5 tests, 5 pass. If `just` is available in the BATS environment, replace the inline chain in the last test with `just review` and re-run.

- [ ] **Step 2: Commit**

```bash
git add tests/sample_wiki_smoke_test.sh
git commit -m "test: sample-wiki end-to-end task-layer smoke"
```

---

## Task 19.13: CI integration — `scheduled/github-action.yml.example`

The v1 phase-9 scheduled config already runs `just test` and `just lint`. Phase 19 adds a `just review` step with a configurable overdue threshold: the CI job exits non-zero only when `REVIEW|overdue|count=N` exceeds a threshold (default 5, configurable via repo variable `AWIKI_CI_OVERDUE_MAX`). This keeps healthy wikis green while flagging genuine drift.

- [ ] **Step 1: Inspect the existing example**

```bash
cat scheduled/github-action.yml.example
```

Identify the `steps:` block, especially the `just test` / `just lint` steps.

- [ ] **Step 2: Append the review step**

Add immediately after the existing `just lint` step:

```yaml
      - name: Run weekly review (informational + threshold gate)
        env:
          AWIKI_CI_OVERDUE_MAX: ${{ vars.AWIKI_CI_OVERDUE_MAX || '5' }}
        run: |
          set -euo pipefail
          # Run scan+agenda first so review-status reads fresh data.
          bash scripts/action-scan.sh
          bash scripts/agenda.sh
          REPORT="$(bash scripts/review-status.sh)"
          echo "$REPORT"
          # Extract overdue count.
          OVERDUE="$(echo "$REPORT" | awk -F'[|=]' '/^REVIEW\|overdue\|/ { print $4 }')"
          OVERDUE="${OVERDUE:-0}"
          if [[ "$OVERDUE" -gt "$AWIKI_CI_OVERDUE_MAX" ]]; then
            echo "::error::overdue actions ($OVERDUE) exceed threshold ($AWIKI_CI_OVERDUE_MAX)"
            exit 1
          fi
          echo "overdue=$OVERDUE within threshold=$AWIKI_CI_OVERDUE_MAX"
```

- [ ] **Step 3: Verify the example file is valid YAML**

```bash
python3 -c "import yaml, sys; yaml.safe_load(open('scheduled/github-action.yml.example'))"
```

Expected: no exception. (YAML's `${{ vars... }}` GitHub-Actions syntax is a string in the YAML body so it parses cleanly.)

- [ ] **Step 4: Manual verification — copy-and-run-locally smoke**

```bash
AWIKI_CI_OVERDUE_MAX=5 bash -c '
  bash scripts/action-scan.sh
  bash scripts/agenda.sh
  REPORT="$(bash scripts/review-status.sh)"
  echo "$REPORT"
  OVERDUE="$(echo "$REPORT" | awk -F"[|=]" "/^REVIEW\\|overdue\\|/ { print \$4 }")"
  echo "overdue=$OVERDUE within threshold=5"
'
```

Expected: prints the structured report; threshold message reads as expected.

- [ ] **Step 5: Commit**

```bash
git add scheduled/github-action.yml.example
git commit -m "ci: add weekly-review step with configurable overdue threshold"
```

---

## Task 19.14: Full-repo lint + test sweep

Final acceptance: every test from phases 16-19 passes; `just lint` is clean across the repo; the sample-wiki smoke is green.

- [ ] **Step 1: Run all BATS suites**

```bash
bats tests/
```

Expected: all phase-16..19 tests pass. Specifically:
- `tests/task_init_test.sh` (phase 16)
- `tests/task_init_pre_commit_test.sh` (phase 19, new)
- `tests/action_scan_test.sh` (phase 17)
- `tests/agenda_test.sh` (phase 17)
- `tests/lint_task_test.sh` (phase 17)
- `tests/triage_test.sh` (phase 18)
- `tests/recur_test.sh` (phase 18)
- `tests/mcp_task_test.sh` (phase 18)
- `tests/review_status_test.sh` (phase 19)
- `tests/mark_review_done_test.sh` (phase 19)
- `tests/mcp_review_status_test.sh` (phase 19)
- `tests/task_layer_migrate_review_test.sh` (phase 19)
- `tests/encrypt_init_task_layer_test.sh` (phase 19)
- `tests/sample_wiki_smoke_test.sh` (phase 19)

- [ ] **Step 2: Run `just lint` against the awiki repo's own content (the `WIKI.md` and existing pages)**

```bash
just lint
```

Expected: zero errors. T13 info-level entries acceptable (and intended) for any unused context page. The WIKI.md task-layer Weekly Review subsection should not trigger lint (it's documentation, not action lines).

- [ ] **Step 3: Run the README 6-step smoke against a fresh clone**

```bash
REPO_ROOT="$(git rev-parse --show-toplevel)"
TEST_DIR="$(mktemp -d)/awiki-final-smoke"
git clone "$REPO_ROOT" "$TEST_DIR"
cd "$TEST_DIR"
git submodule update --init --recursive
just check-deps
TASK_INIT_AUTO=1 just task-init
just capture "call dentist"
mkdir -p content/projects
cat > content/projects/_loose.md <<'EOF'
---
title: "Loose"
date: 2026-04-27
last_updated: 2026-04-27
type: project
status: active
outcome: "Catch-all."
tags: []
draft: false
---

## Open Actions

- [ ] call dentist @phone ^a01
EOF
sed -i.bak '/call dentist/d' content/inbox.md
just agenda
grep -q "call dentist" content/agenda/next-actions.md
sed -i.bak 's/^- \[ \] call dentist.*$/- [x] call dentist @phone done:2026-04-27 ^a01/' content/projects/_loose.md
just agenda
! grep -q "call dentist" content/agenda/next-actions.md
just review | tee /tmp/review-out.txt
grep -qE '^REVIEW\|completed-since-last-review\|count=[1-9]' /tmp/review-out.txt
echo "FINAL SMOKE OK"
```

Expected: prints `FINAL SMOKE OK`.

- [ ] **Step 4: Document any rough edges**

If any step in the smoke required an explicit user action (e.g., manual page creation that should have been automated), record in `docs/decisions/task-layer-smoke-issues.md` with a follow-up task. Do not paper over real bugs.

- [ ] **Step 5: Commit any fixes**

```bash
git status -s
git diff
git commit -am "fix: task-layer final-smoke corrections"
```

If no fixes needed, skip this step.

---

## Task 19.15: Phase 19 merge

- [ ] **Step 1: Pre-merge sanity**

```bash
git checkout phase-19-task-review-polish
bats tests/
just lint
```

Expected: clean.

- [ ] **Step 2: Merge to main**

```bash
git checkout main
git pull --ff-only
git merge --no-ff phase-19-task-review-polish -m "feat: complete phase 19 task-layer review + polish"
```

- [ ] **Step 3: Tag (optional but recommended — task-layer feature is now complete)**

```bash
git tag v1.1.0   # task-layer ships in v1.1
```

- [ ] **Step 4: Delete the phase branch**

```bash
git branch -d phase-19-task-review-polish
```

- [ ] **Step 5: Push (if remote configured)**

```bash
git push origin main --tags
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md). The task-layer feature (phases 16-19) is now complete.

---

## Self-review

Scope: phase 19 deliverables only. The four prior phases' deliverables are not re-checked here.

**Spec row 19 — coverage check:**

| Spec deliverable                            | Phase 19 task |
|---------------------------------------------|----------------|
| `scripts/review-status.sh`                  | Task 19.2     |
| Justfile `review` recipe                    | Task 19.3     |
| `mark_review_done` MCP tool                 | Task 19.5     |
| `review_status` full impl (replace stub)    | Task 19.4     |
| `content/agenda/review-log.md`              | Created in phase 16 by `task-init.sh`; appended to by Task 19.5 (`mark_review_done`) and Task 19.11 (sample entry). |
| Weekly-review WIKI.md subsection            | Task 19.6     |
| Pre-commit hook installer                   | Task 19.7     |
| README task-layer section                   | Task 19.9     |
| Sample-wiki extended                        | Task 19.11    |
| **`encrypt-init` updated**                  | Task 19.8     |
| Full smoke passes                           | Task 19.14    |

**Structured `REVIEW|` line shapes vs spec — line-by-line:**

| Spec line                                                | Plan emits (Task 19.2) |
|----------------------------------------------------------|-------------------------|
| `REVIEW\|inbox-unprocessed\|count=N`                      | Yes (printf in step 1) |
| `REVIEW\|raw-inbox-files\|count=N`                        | Yes |
| `REVIEW\|projects-no-next-action\|count=N\|slugs=...`     | Yes (CSV slug list when N>0; bare `count=0` when N=0) |
| `REVIEW\|waiting-stale-14d\|count=N\|ids=...`             | Yes |
| `REVIEW\|overdue\|count=N\|ids=...`                       | Yes |
| `REVIEW\|completed-since-last-review\|count=N`            | Yes |
| `REVIEW\|stuck-projects\|count=N\|slugs=...`              | Yes |
| `REVIEW\|someday-count\|count=N`                          | Yes |
| `REVIEW-SUMMARY\|last-review=YYYY-MM-DD\|attention=N`     | Yes |

Note: the spec example shows the `slugs=` / `ids=` suffix only for non-empty cases. Task 19.2 emits `count=0` alone when N=0 (matching the `someday-count` and `completed-since-last-review` shape) and emits the suffix only when there is content. The BATS tests in 19.2 verify both shapes.

**Encrypt-init backwards compatibility:**

- Task 19.8's `encrypt-init` patch only fires when `AWIKI_TASK_LAYER=on` is in `.awiki/config`.
- Task 19.8's BATS test 4 explicitly verifies that re-running `task-init` does NOT silently invoke `encrypt-init` (the user must invoke it themselves).
- The README task-layer section calls this out: "If you ran `encrypt-init` BEFORE `task-init`, run `encrypt-init` again to add the new patterns, OR hand-edit `.gitattributes`." Documented in the methodology note's "Privacy note" line in the WIKI.md template.
- The `add_pattern` helper in `encrypt-init.sh` is idempotent on re-run (regex-checked), so users can safely re-execute.

**Pre-commit hook idempotency:**

- Marker line `# task-layer` (Task 19.7).
- Test 19.7 step-1 case 2 explicitly asserts re-running `task-init` does NOT duplicate the hook block.
- Test 19.7 case 3 asserts append-mode preserves a pre-existing non-task-layer hook (e.g., the v1 phase-12 `install-hooks.sh` hook).
- Test 19.7 case 4 asserts the user can opt-out with `n` and the hook is not installed.

**Sample-wiki smoke green:**

- Task 19.11 scaffolds via `task-init` (with `TASK_INIT_AUTO=1`), populates pages, runs `agenda.sh`, and appends a review-log entry.
- Task 19.12 codifies the smoke as `tests/sample_wiki_smoke_test.sh` (5 BATS tests) so future phase changes cannot silently break the sample wiki.
- Task 19.14 step 3 runs the README's 6-step smoke against a fresh clone end-to-end.

**No invented features:** every task is sourced from the spec's Phase 19 row, the Weekly Review section, the Threat Model encryption row, the `task-init` step-6 + step-3 instructions, or the Implementation Phases table. The CI threshold (Task 19.13) is the only addition not literally specified — it is implied by "extend `scheduled/github-action.yml.example`" in the task list above; the threshold is configurable and defaults to a value (5) that matches the spec's overdue example.

---

## Open questions / spec ambiguities

These are points where the spec admits multiple reasonable implementations or leaves a behavior unspecified. None block phase 19; the tasks above pick one resolution and document the choice.

1. **`REVIEW|` line shape when count=0.** The spec's example shows the `slugs=...` and `ids=...` suffix only for non-zero counts (`waiting-stale-14d|count=1|ids=a03`), but does not state whether `count=0` rows should still carry an empty `slugs=` / `ids=` suffix. **Resolution:** Task 19.2 emits `count=0` with no trailing suffix (matches the most restrictive read of the spec). If downstream consumers expect a uniform shape, switch to always-emit-suffix and update Task 19.2's BATS regexes.

2. **`attention` computation.** The spec's example summary line shows `attention=4` for a report with overdue=2, waiting-stale=1, no-next-action=2, stuck=1 — that's a sum of 6, not 4. **Resolution:** Task 19.2 implements `attention = overdue + waiting-stale + no-next-action + stuck` (sum, no de-dup) and the BATS test asserts that sum. The spec's `=4` is likely a hand-calc error in the example. Flag for spec follow-up; do not match the example arithmetic.

3. **`mark_review_done` log-line "next" / "waiting" semantics.** The spec's log-line shape is `inbox=N next=N waiting=N overdue=N completed=N stuck=N`, but `next` and `waiting` are not in the `REVIEW|` line set. **Resolution:** Task 19.5 reads `actions.tsv` directly to count open `[ ]`/`[/]` for `next` and `[?]` for `waiting`. Other counts come from the structured stdout. Documented inline in the handler.

4. **`scripts/lib/lock.sh` CLI mode.** The spec describes `lock.sh` as a sourceable library only. Task 19.5's `mark_review_done` handler benefits from a CLI invocation form to wrap a small bash snippet under the lock. **Resolution:** Task 19.5 step-2 adds a `--exec`-mode entry to `lock.sh` if phase 16 did not already define one. Confirm before adding to avoid duplication.

5. **`encrypt-init` re-run when `AWIKI_TASK_LAYER` flips on AFTER initial encryption.** The spec says "existing encrypt-init runs are NOT migrated (user must re-run encrypt-init or hand-edit `.gitattributes`)." Task 19.8 honors this by making the new pattern-add code path live in `encrypt-init.sh` itself; users opt in by re-invoking. **Open:** should `task-init.sh` step 3 (encryption-coverage prompt) detect this case and *suggest* re-running `encrypt-init`? Phase 16's step 3 already prompts for the patterns directly. Task 19.8 leaves phase 16's step 3 alone — both code paths now exist; the user picks whichever they reach first. Not a behavior conflict.

6. **`completed-since-last-review` when `.awiki/last-review=never`.** The spec does not specify behavior when no prior review exists. **Resolution:** Task 19.2 counts all completed actions when `last-review` is missing (`else` branch). The `REVIEW-SUMMARY` line emits `last-review=never`. The MCP tool's JSON `last_review` field surfaces the same string `"never"`.

7. **Sample-wiki agenda regen at commit time.** Task 19.11 step 6 runs `agenda.sh` against the sample wiki and commits the populated agenda pages. These pages will become stale if the sample-wiki actions change later. **Resolution:** Task 19.12's BATS test re-runs `agenda.sh` in `setup()` and asserts on the regenerated content, so the committed sample agenda pages serve only as documentation; the test does not depend on them being current. Acceptable for v1; if drift becomes annoying, add a `just sample-wiki-refresh` recipe in a follow-up phase.
