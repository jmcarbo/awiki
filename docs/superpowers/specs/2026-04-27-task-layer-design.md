# Task Layer — Design

**Date:** 2026-04-27
**Status:** Draft (pending user written-spec review)
**Type:** Feature on top of awiki v1 (post-master-plan addition)
**Depends on:** Master plan phases 1-3, 6, 8

## Summary

Adds an opt-in action-management layer to awiki. The methodology is the
classic capture → clarify → organize → reflect → engage flow popularized by
personal-productivity literature, expressed as wiki-native primitives.

Users capture quick thoughts to `content/inbox.md` and file-shaped sources
(PDF, voice memo, email export) to the existing `raw/inbox/interactive/`.
The agent triages each item to one of seven outcomes — trash, do-now, act,
defer-scheduled, waiting, reference, someday — using a `triage_apply` MCP
tool. Actions live as inline checkboxes on project / context pages with
status markers and key:value tail metadata; a single source of truth on
the origin page. A scanner builds derived agenda views (`next-actions`,
`today`, `waiting`, `someday`, `stuck-projects`) under `content/agenda/`
with managed-region markers. Recurrence (`every:1w`) re-emits open copies
on completion. Weekly review is surfaced via `review_status()` MCP tool
backed by a structured report.

The layer is opt-in via `just task-init`; infrastructure (scripts, lint
rules, MCP tool code) ships with awiki, but pages, schema additions, and
state files are only created when the user runs `task-init`.

## Goals

- Capture-clarify-organize-reflect-engage flow as wiki-native primitives.
  No external app, no daemon, no cloud sync.
- One source of truth: action lines live on origin pages (project /
  context). Agenda views are derived artifacts.
- Schema-first: `type: project` and `type: context` join the page-kind
  enum; `inbox` and `agenda` join system pages. Inline-checkbox grammar
  documented in `WIKI.md`. Lint enforces.
- Agent-friendly: triage and review are MCP tools with structured I/O and
  deterministic side effects. Bash fallbacks exist for cold-start.
- Obsidian-friendly: status markers + tail-metadata grammar interoperable
  with the Obsidian Tasks plugin defaults (without requiring it).
  Block-IDs (`^id`) provide stable references.
- Hugo-friendly: agenda pages render as readable lists; managed-region
  markers mirror the synthesis spec convention.
- Opt-in: zero footprint until `just task-init` runs. Reversible by
  deleting the bracketed `<!-- BEGIN task-layer -->` block in `WIKI.md`
  and the layer's pages.

## Non-goals

- External calendar / Google Tasks / Todoist sync. Future spec.
- Push notifications, mobile app, daemon. Capture is git-committed text.
- Time-tracking, pomodoro, billable hours.
- Multi-user assignment with state machines. Single user;
  "delegate" = "I'm waiting on a person".
- Section-anchor wikilinks for action references (matches synthesis
  spec's same non-goal).
- Cron-style recurrence. Only `every:Nd|Nw|Nm` and named cadences
  (`daily`, `weekly`, `monthly`).
- GUI or kanban views.

## In-scope (delivered across phases 16-19)

- `scripts/task-init.sh` (idempotent enable script).
- `scripts/capture.sh` (append to `content/inbox.md`).
- `scripts/action-scan.sh` (single-pass scanner → `.awiki/maps/actions.tsv`).
- `scripts/agenda.sh` (regenerates five managed-region agenda pages).
- `scripts/action-recur.sh` (re-emit on `[x] every:N` completion).
- `scripts/triage.sh` (bash fallback; MCP tool is primary).
- `scripts/review-status.sh` (structured weekly-review report).
- MCP server extension: `capture`, `triage_inbox`, `triage_apply`,
  `list_actions`, `rebuild_agenda`, `review_status`,
  `mark_review_done`.
- WIKI.md additions: page-kind enum entries, system-page entries,
  inline-grammar table, triage workflow, weekly-review workflow, lint
  rules T1-T13. All bracketed for clean removal.
- Lint rules T1-T13 (mechanical + semantic).
- Justfile recipes: `task-init`, `capture`, `triage`, `agenda`, `scan`,
  `review`.
- Pre-commit hook variant that runs `action-scan.sh`.
- BATS tests: scan, agenda, triage, recur, lint, init, MCP.
- Sample wiki extended with example projects, contexts, inbox.

---

## Repository Additions

```
awiki/
├── content/
│   ├── inbox.md                     # capture stream (system page)
│   ├── projects/
│   │   ├── _index.md                # section index (page-list shortcode)
│   │   └── <slug>.md                # type: project
│   ├── contexts/
│   │   ├── _index.md
│   │   └── <slug>.md                # type: context (phone, errands, ...)
│   └── agenda/
│       ├── _index.md
│       ├── next-actions.md          # generated, managed-region
│       ├── today.md                 # generated
│       ├── waiting.md               # generated
│       ├── someday.md               # generated
│       ├── stuck-projects.md        # generated
│       └── review-log.md            # append-only review history
├── scripts/
│   ├── task-init.sh                 # one-shot enable
│   ├── capture.sh                   # append line to content/inbox.md
│   ├── triage.sh                    # bash fallback
│   ├── action-scan.sh               # scan content/**/*.md → actions.tsv
│   ├── agenda.sh                    # regen agenda/*.md from scan output
│   ├── action-recur.sh              # re-emit on [x] every:N (idempotent)
│   └── review-status.sh             # structured review report
├── mcp/awiki-server/
│   └── (extended) capture, triage_inbox, triage_apply, list_actions,
│       rebuild_agenda, review_status, mark_review_done
├── .awiki/
│   ├── last-review                  # ISO timestamp
│   ├── task-count                   # triage counter (auto-rebuild trigger)
│   └── maps/
│       └── actions.tsv              # scanner output (gitignored)
└── tests/
    ├── action_scan_test.sh
    ├── agenda_test.sh
    ├── triage_test.sh
    ├── recur_test.sh
    ├── lint_task_test.sh
    ├── task_init_test.sh
    └── mcp_task_test.sh
```

`task-init.sh` is idempotent. Re-running on an already-initialized wiki
prints "task layer already enabled, nothing to do" and exits 0.

---

## Schema Additions to `WIKI.md`

All additions are bracketed by `<!-- BEGIN task-layer -->` /
`<!-- END task-layer -->` so the layer can be removed cleanly.

### `type` enum extension

Add to **page kinds** (Section 2):
- `project` — outcome requiring multiple actions; tracked to completion.
- `context` — situation/tool/place where actions can happen
  (`phone`, `errands`, `computer`, `home`, ...).

Add to **system pages**:
- `inbox` — single page `content/inbox.md`, append-only capture stream.
  Orphan-check exempt.
- `agenda` — generated pages under `content/agenda/`.
  Managed-region; orphan-check exempt; `last_updated` exempt from
  staleness rule (rebuilt on cadence).

### Project frontmatter

```yaml
---
title: "Renovate kitchen"
date: 2026-04-27
last_updated: 2026-04-27
type: project
status: active            # active | someday | done | dropped
outcome: "Kitchen renovated; certificate of occupancy."
area: [[home]]            # optional wikilink to area entity
tags: [home]
aliases: []
draft: false
---
```

Body structure: outcome statement → `## Notes` → `## Open Actions`
(canonical inline-checkbox section) → `## Done` (completed actions,
optional) → `## Related` → `## Sources`. Scanner discovers checkboxes
under any heading; `## Open Actions` is convention.

### Context frontmatter

```yaml
---
title: "@phone"
date: 2026-04-27
last_updated: 2026-04-27
type: context
aliases: ['@phone']        # canonical alias; lint enforces parity
                           # between filename slug and @-prefixed alias
tools: ["AirPods", "Skype: 415-555-..."]
draft: false
---
```

Slug convention: `phone`, `errands`, `home`, `computer`. Wikilink form:
`[[@phone]]` resolves to `content/contexts/phone.md` via the alias map
built by `scripts/lint.sh`.

### Inbox frontmatter

```yaml
---
title: "Inbox"
type: inbox
draft: true               # never publish
---
```

Body = chronological capture lines, no checkbox markers (captures are
not yet actions):

```
- 2026-04-27 14:32 call dentist about crown
- 2026-04-27 14:33 idea: rewrite onboarding email
- 2026-04-27 15:10 [[s-as-we-may-think]] reread chapter 3
```

Triage promotes a line to a checkbox on a project / context page, or to
a new wiki page (reference outcome).

---

## Inline Action Grammar

Canonical line:

```
- [STATUS] <text> [@context] [key:value ...] [^id]
```

### Status markers (Obsidian Tasks compatible)

| Marker | Meaning |
|--------|---------|
| `[ ]` | open / next |
| `[/]` | in-progress |
| `[?]` | waiting (delegated) |
| `[>]` | someday / deferred-indefinite |
| `[x]` | done |
| `[-]` | cancelled |

### Tail metadata keys

key:value, lowercase, ASCII.

| Key | Format | Meaning |
|-----|--------|---------|
| `due:` | `YYYY-MM-DD` | hard date — overdue if past |
| `defer:` | `YYYY-MM-DD` | hidden from `next-actions` until date |
| `wait:` | `[[entity-slug]]` | person/team blocking; required if `[?]` |
| `since:` | `YYYY-MM-DD` | when waiting started; auto-stamped on flip to `[?]` |
| `every:` | `Nd`/`Nw`/`Nm`/`daily`/`weekly`/`monthly` | recurrence on completion |
| `done:` | `YYYY-MM-DD` | auto-stamped on flip to `[x]` |
| `priority:` | `1`-`3` | optional; lower = higher |
| `est:` | `Nm`/`Nh` | optional time estimate |

### Context

Wikilink-style tag, written `@phone` inline (resolves via context-page
alias). Actions can carry zero or one context.

### Block-ID `^abc123`

Obsidian convention. Stable cross-link target. `triage_apply` and
`action-recur.sh` mint when promoting from inbox; lint warns on
duplicate IDs (`T2`).

### Examples

```markdown
- [ ] call dentist about crown @phone due:2026-05-01 ^a01
- [/] draft proposal @computer ^a02
- [?] q3 budget approval wait:[[bob-smith]] since:2026-04-22 ^a03
- [>] reorganize garage someday @home ^a04
- [ ] water plants @home every:1w due:2026-05-04 ^a05
- [x] file taxes @computer due:2026-04-15 done:2026-04-13 ^a06
```

Actions live on project pages, context pages, or area-entity pages.
Scanner discovers regardless of heading.

---

## Capture + Triage Workflow

### Capture channels

**Quick capture → `content/inbox.md`:**
- Mobile/desktop user appends free-text line (Obsidian quick-capture
  plugin or just typing).
- MCP tool `capture(text)` appends `- <ISO-datetime> <text>` to
  `inbox.md`.
- Justfile recipe: `just capture "call dentist"` → `scripts/capture.sh`.
  This is a new script (separate from `log-append.sh`) because log and
  inbox have different semantics — log is event history, inbox is an
  open-loop queue.

**File-shaped capture → `raw/inbox/interactive/`:**
- Existing `ingest.sh` pipeline. PDF, voice memo, email export.
- Triage outcomes (below) still apply.

### Seven triage outcomes

Single workflow regardless of capture channel. Agent calls
`triage_inbox()` to get the unprocessed list, then walks one item at a
time, asking the user, applying outcomes via `triage_apply(...)`.

| Outcome | Effect |
|---------|--------|
| `trash` | Strikethrough line in `inbox.md` (or move file to `raw/inbox/.trash/`). Logged. |
| `do-now` | (≤2 min items.) Agent + user execute live. Append `- [x] <text> done:<today>` to a project / context page. Logged. |
| `act` | Promote to `- [ ] <text> @<context> ^<id>` under chosen project page (or new project). Asks user for context. Logged. |
| `defer-scheduled` | `- [ ] <text> @<context> due:<date> ^<id>` or `defer:<date>`. Logged. |
| `waiting` | `- [?] <text> wait:[[<person>]] since:<today> ^<id>` placed on chosen project page (or `content/projects/_loose.md` if user picks "no project"). Logged. |
| `reference` | Becomes a wiki page: existing ingest pattern — agent writes `content/<entities|concepts|topics|sources>/<slug>.md`. No checkbox. Logged. |
| `someday` | `- [>] <text> ^<id>` on relevant project page or `content/projects/_someday.md` catch-all. Logged. |

`_loose.md` and `_someday.md` are normal project pages (`type: project`,
`status: active` and `status: someday` respectively); the underscore
prefix is convention for "catch-all". Created lazily on first use, not
by `task-init`.

Each outcome:
1. Removes original capture line from `inbox.md` (or moves file from
   `raw/inbox/interactive/` → `raw/processed/`).
2. Logs via `scripts/log-append.sh triage "<outcome> | <slug>"`.
3. Increments `.awiki/task-count`. At threshold (default 5,
   `AWIKI_AGENDA_AFTER_N` env var), `agenda.sh` auto-rebuilds.

### Triage MCP tool surface

```
triage_inbox()
  → returns: [{ source: "inbox.md" | "raw/inbox/...", line_or_path: ...,
                text: ..., id: ... }, ...]

triage_apply(id, outcome, params)
  outcome ∈ {trash, do-now, act, defer-scheduled, waiting,
             reference, someday}
  params: { project_slug?,                  // kebab-case, no leading "@"
            context_slug?,                  // kebab-case, no leading "@"
            due?,                            // YYYY-MM-DD
            defer?,                          // YYYY-MM-DD
            wait_for?,                       // entity slug, no [[ ]]
            page_type? }                     // for reference outcome
  → writes pages, removes/moves source, logs,
    returns: { success, actions_taken: [...] }
```

`triage_apply` is the single mutation entry-point. Bash fallback:
`scripts/triage.sh <id> <outcome> [k=v ...]` invokes the same logic for
cold-start / no-MCP situations.

---

## Scanner + Agenda Generation

### `scripts/action-scan.sh`

Single pass over `content/**/*.md`. Output: `.awiki/maps/actions.tsv`
(gitignored). Columns:

```
id  status  text  file  line  context  due  defer  wait  since  every  done  priority  est  project
```

- `project` column resolved by walking up: action's containing file's
  frontmatter `type:` — if `project`, that slug; else `""`.
- `context` resolved from inline `@<slug>` token, normalized via
  context alias map. Empty if no context.
- Lines without `[STATUS]` skipped.
- Lines under `## Done` heading on project pages still scanned (state
  = `[x]`); used for "completed since last review" delta.

Idempotent. Stdout = scan summary
(`scanned N files, M actions, K open, L done`). Exit 0 clean, 1 if
duplicate `^id` detected (lint surfaces same).

### `scripts/agenda.sh`

Reads `actions.tsv`, regenerates the five managed-region pages.

**Managed-region marker** (matches synthesis spec convention):

```markdown
<!-- BEGIN agenda:next-actions -->
## By Context

### @phone
- [ ] call dentist about crown — [[renovate-kitchen]] due:2026-05-01 ^a01

### @computer
- [/] draft proposal — [[q3-launch]] ^a02
<!-- END agenda:next-actions -->
```

Agent must NOT hand-edit between markers (lint rule `T7`, mirrors
synthesis L6 hand-edit detection). Anything outside markers is preserved
(user notes).

### Per-page rules

- `next-actions.md` — every `[ ]` and `[/]` action without a future
  `defer:`. Grouped by `@context`, then by project. No-context bucket
  last.
- `today.md` — actions where `due ≤ today` OR (`defer:` exists AND
  `defer ≤ today` AND status `[ ]`/`[/]`) OR `due` overdue. Sorted:
  overdue → today → in-progress.
- `waiting.md` — every `[?]`. Grouped by `wait:` person. Each line
  shows `since:` and days waited.
- `someday.md` — every `[>]`. Grouped by project (or "unassigned").
- `stuck-projects.md` — projects with `status: active` AND zero open
  `[ ]`/`[/]` actions, OR `last_updated > 14d` with no completed
  action since. List shows project slug + reason.

### Rebuild triggers

- Manual: `just agenda`.
- Auto: `triage_apply` increments `.awiki/task-count`; at threshold
  (default 5, env `AWIKI_AGENDA_AFTER_N`), runs `agenda.sh`. Counter
  resets.
- MCP: `rebuild_agenda()` callable mid-conversation.
- Pre-commit hook (optional, set by `task-init`): runs
  `action-scan.sh` only (cheap), warns on duplicate IDs.

---

## Recurrence

`scripts/action-recur.sh` runs after any `[x]` flip on an action carrying
`every:`. Triggered:
- by `triage_apply` when `do-now` outcome stamps `[x]` on a recurring
  line, OR
- by user manually editing `[ ]` → `[x]` and running `just agenda`
  (scanner detects, runs recur pass before generation), OR
- by MCP `rebuild_agenda()`.

### Behavior (idempotent)

1. Scanner emits a `recur-needed` row for each `[x] every:N done:<date>`
   whose computed-next-due-date is **not yet present** as an open `[ ]`
   line with same `^id-N` prefix.
2. `action-recur.sh` re-emits an open copy on the same page,
   immediately above the `[x]` line:

   ```
   - [ ] water plants @home every:1w due:2026-05-11 ^a05-2
   - [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
   ```
3. New `^id` = `<original-id>-<n>`, monotonically increasing per
   recurrence chain. Lint enforces uniqueness across chain.
4. Computed `due:` = `done` + interval. If no `done:` on completion
   (user forgot), use today.
5. Dry-run mode `action-recur.sh --dry-run` prints the diff without
   writing — used by lint and `--fix` workflows.

Recurrence cap: refuses to emit if the chain has already produced ≥200
instances on one page (sanity bound; user splits chain).

---

## Weekly Review

### Doc

WIKI.md gains a "Weekly Review" subsection under Workflows. Steps:

1. Empty `inbox.md` (triage every line).
2. Empty `raw/inbox/interactive/` (ingest every file).
3. Walk `agenda/next-actions.md` — every project should have at least
   one `[ ]`/`[/]` next action.
4. Walk `agenda/waiting.md` — nudge anything `>14d`.
5. Walk `agenda/someday.md` — promote one or two to active if
   appropriate.
6. Walk `agenda/stuck-projects.md` — decide: revive / drop / move to
   someday.
7. Skim `content/projects/_index.md` — project list sanity.
8. Stamp review.

### Recipe

```just
review:
    bash scripts/agenda.sh
    bash scripts/lint.sh
    bash scripts/review-status.sh
```

`review-status.sh` prints structured report:

```
REVIEW|inbox-unprocessed|count=3
REVIEW|raw-inbox-files|count=1
REVIEW|projects-no-next-action|count=2|slugs=q3-launch,renovate-kitchen
REVIEW|waiting-stale-14d|count=1|ids=a03
REVIEW|overdue|count=2|ids=a01,a07
REVIEW|completed-since-last-review|count=8
REVIEW|stuck-projects|count=1|slugs=onboarding-revamp
REVIEW|someday-count|count=12
REVIEW-SUMMARY|last-review=2026-04-20|attention=4
```

### MCP tool

`review_status()` returns the same data as a JSON object:

```json
{
  "last_review": "2026-04-20",
  "inbox_unprocessed": 3,
  "raw_inbox_files": 1,
  "projects_no_next_action": ["q3-launch", "renovate-kitchen"],
  "waiting_stale_14d": [
    {"id":"a03","wait":"bob-smith","since":"2026-04-10","days":17}
  ],
  "overdue": [{"id":"a01","due":"2026-04-25","days_over":2}],
  "completed_since_last_review": 8,
  "stuck_projects": ["onboarding-revamp"],
  "someday_count": 12
}
```

Agent uses this to walk user through the review; on completion, agent
calls `mark_review_done()` which writes `.awiki/last-review` (ISO
timestamp) and appends to `content/agenda/review-log.md`:

```
## [2026-04-27 17:42] review | inbox=0 next=14 waiting=2 overdue=0 completed=8 stuck=0
```

---

## Lint Rules Added

Extends `scripts/lint.sh`. New `LINT|<level>|<file>|<msg>` lines:

| Level | Rule | Trigger |
|-------|------|---------|
| error | `T1-bad-status` | Checkbox marker not in `{ ,/,?,>,x,-}`. |
| error | `T2-dup-id` | Same `^id` appears on ≥2 action lines. Recur chain pattern `^<base>-<n>` is exempt as long as each `<n>` is unique within the chain and each chain element appears at most once. |
| error | `T3-bad-key` | Tail key not in allowed set (`due,defer,wait,since,every,done,priority,est`). |
| error | `T4-bad-date` | `due:`/`defer:`/`since:`/`done:` value not `YYYY-MM-DD`. |
| error | `T5-waiting-no-person` | `[?]` line without `wait:`. |
| error | `T6-bad-context` | `@<slug>` references context page that does not exist. |
| error | `T7-managed-region-edited` | Hand-edit detected inside `<!-- BEGIN agenda:... -->` / `<!-- END -->`. |
| warn | `T8-no-next-action` | `type: project, status: active` page has zero open `[ ]`/`[/]` actions. |
| warn | `T9-waiting-stale` | `[?]` line where `since:` > 14 days ago. |
| warn | `T10-overdue` | `[ ]`/`[/]` line where `due:` < today. |
| warn | `T11-stale-someday` | `[>]` line whose enclosing page hasn't been touched in 90+ days. |
| warn | `T12-recur-chain-cap` | Recur chain at ≥150 (warn at 150, error at 200). |
| info | `T13-context-unused` | `type: context` page with zero referencing actions. |

`lint.sh --fix` mechanical fixes (task-aware):
- Normalize date format (`2026/4/27` → `2026-04-27`).
- Auto-stamp missing `done:` on `[x]` lines (= today).
- Auto-stamp missing `since:` on `[?]` lines (= today).
- Sort tail keys to canonical order:
  `@context due defer wait since every priority est done`.
- Mint missing `^id` (random base32, 6 chars) — preserves existing IDs.

---

## MCP Server Additions

`mcp/awiki-server/index.js` extended. New tools:

| Tool | Args | Returns | Side effects |
|------|------|---------|--------------|
| `capture` | `{text}` | `{appended:true, line, timestamp}` | Appends to `content/inbox.md`. |
| `triage_inbox` | `{}` | `[{id, source, text, captured_at, ...}]` | Read-only scan of `inbox.md` lines + `raw/inbox/interactive/` files. |
| `triage_apply` | `{id, outcome, params}` | `{ok, actions_taken[], created_pages[], updated_pages[]}` | Mutates origin + destination files; logs; increments `task-count`. |
| `list_actions` | `{filter?: {status?, context?, project?, due_before?, wait?, overdue?}}` | `[action]` | Read-only; calls `action-scan.sh` if scan map stale. |
| `rebuild_agenda` | `{}` | `{rebuilt:[<file>], duration_ms}` | Runs `action-scan.sh` + `agenda.sh`. |
| `review_status` | `{}` | (see Weekly Review JSON above) | Read-only. |
| `mark_review_done` | `{}` | `{last_review:<ISO>}` | Writes `.awiki/last-review`, appends `review-log.md`. |

### Security

- All shell-outs continue using `execFileSync` with arg arrays
  (no shell interpolation). Per-arg validation at the MCP boundary:
  `id` matches `^[a-z0-9-]+$`, dates ISO-only, slugs kebab-case,
  outcome ∈ enum.
- `triage_apply` rejects `params.project_slug` if the path resolves
  outside `content/projects/`. Same for `context_slug`.
- `capture` truncates input >2000 chars (one inbox line cap), strips
  control chars; embedded newlines rejected outright.
- All seven tools live in the same stdio-only server. No new transport.

### Wiring

Existing `scripts/wire-awiki-mcp.sh` already registers the server; new
tools auto-exposed once `mcp/awiki-server` is rebuilt
(`cd mcp/awiki-server && npm install`). `task-init` does NOT re-wire —
prints the rebuild instruction; user runs once.

---

## `task-init` Workflow

`scripts/task-init.sh` (idempotent). Run by user via `just task-init`.
Steps:

1. **Pre-check.** Refuses if `content/inbox.md` already exists AND
   `WIKI.md` already declares `type: project` — prints
   "task layer already enabled, nothing to do" and exits 0.
2. **Create pages:**
   - `content/inbox.md` (frontmatter + empty body).
   - `content/projects/_index.md` (section index, `page-list` shortcode).
   - `content/contexts/_index.md` (section index).
   - `content/agenda/_index.md`.
   - `content/agenda/{next-actions,today,waiting,someday,stuck-projects}.md`
     — each with empty managed-region pair + minimal frontmatter
     (`type: agenda`, `last_updated: <today>`, `draft: false`).
   - `content/agenda/review-log.md` — frontmatter `type: agenda`, body
     header only.
   - Three starter context pages: `content/contexts/{phone,errands,computer}.md`
     with `aliases: ['@phone']` etc. User edits and extends.
3. **Patch `WIKI.md`:** appends pre-templated "Action Layer" section
   (page-kind enum entries, system-page entries, inline-grammar table,
   triage workflow, weekly-review workflow, lint rules T1-T13). Section
   bracketed by `<!-- BEGIN task-layer -->` / `<!-- END task-layer -->`
   so it can be removed cleanly.
4. **Patch `.awiki/config`:** sets `AWIKI_AGENDA_AFTER_N=5`,
   `AWIKI_TASK_LAYER=on`.
5. **Initialize state:** `.awiki/last-review=<today>`,
   `.awiki/task-count=0`.
6. **Optional pre-commit hook:** asks
   "Install task-aware pre-commit hook (runs action-scan + lint)? (y/N)".
   On `y`, writes `git/hooks/pre-commit` (or appends to existing).
7. **Optional MCP rebuild prompt:** if `mcp/awiki-server` already
   wired, prints "rebuild server: `cd mcp/awiki-server && npm install`".
   If not wired, prints how to wire.
8. **Log:** `scripts/log-append.sh task-init "enabled"`.
9. **Smoke instructions:** prints
   "Try: `just capture 'pick up groceries'` then `just agenda`".

**Reversal:** documented but not scripted. `WIKI.md` task block is
`<!-- BEGIN task-layer -->` bracketed; user deletes block, removes
the layer's pages and state files. No `task-deinit.sh` in v1 (YAGNI).

---

## justfile Additions

```just
# === task layer ===
task-init:
    bash scripts/task-init.sh

capture *text:
    bash scripts/capture.sh {{text}}

triage:
    @echo "Open agent. Say: 'triage inbox'. Agent calls triage_inbox MCP tool."

agenda:
    bash scripts/action-scan.sh
    bash scripts/agenda.sh

scan:
    bash scripts/action-scan.sh

review:
    bash scripts/agenda.sh
    bash scripts/lint.sh
    bash scripts/review-status.sh
```

Quoting convention extends the existing rule:
`just capture "pick up groceries"`. Variadic `*text` collects all args;
`capture.sh` joins with spaces.

`docs/just-help.txt` extended with a task-layer section: each recipe +
when to use + what to expect.

---

## Testing

BATS tests under `tests/` (matches existing pattern):

| Test | Asserts |
|------|---------|
| `action_scan_test.sh` | Fixture wiki with 12 actions of varied status/metadata → `actions.tsv` has correct rows, columns, IDs. Duplicate ID detection. Lines without status skipped. |
| `agenda_test.sh` | Given fixture `actions.tsv`, `agenda.sh` produces expected `next-actions.md` / `today.md` / `waiting.md` / `someday.md` / `stuck-projects.md`. Managed-region content swappable; outside-region content preserved across rebuilds. |
| `triage_test.sh` | All seven outcomes: input inbox line + apply outcome → assert origin removed, destination written, log appended, `task-count` incremented. |
| `recur_test.sh` | `[x] every:1w done:2026-04-27` → emits `[ ] every:1w due:2026-05-04 ^id-2`. Idempotent: re-run does not double-emit. Cap at 200. |
| `lint_task_test.sh` | Each T1-T13 rule fires on a hand-crafted bad fixture and stays silent on a good fixture. `--fix` corrections work and are idempotent. |
| `task_init_test.sh` | Empty repo + `task-init.sh` → expected pages exist, `WIKI.md` patched between markers, `.awiki/config` updated. Re-run = no-op. |
| `mcp_task_test.sh` | Spins up `awiki-server` over stdio, exercises `capture`, `triage_inbox`, `triage_apply` (one outcome), `list_actions`, `rebuild_agenda`, `review_status`, `mark_review_done`. Verifies arg validation rejects malformed inputs. |

Fixtures under `tests/fixtures/wiki-task-good/` and `wiki-task-broken/`.
Reused by lint and scanner tests.

CI: `scheduled/github-action.yml.example` already runs `just test` and
`just lint` — picks up new tests automatically. Pinned tool versions in
the v1 Dependencies & Compatibility table extended with Node version
(already present for MCP server).

### Manual smoke test (added to README task-layer section)

1. `just task-init`
2. `just capture "call dentist"`
3. Open agent, ask: "triage inbox" → agent calls MCP tool, walks user;
   user chooses `act`, project `_loose`, context `phone`.
4. `just agenda` → confirm `content/agenda/next-actions.md` lists the
   action under `@phone` with project `[[_loose]]`.
5. Mark `[x]` on the action line in `content/projects/_loose.md`, run
   `just agenda` → confirm action removed from `next-actions.md`.
6. `just review` → structured report shows `completed-since-last-review|count=1`.

---

## Threat Model / Privacy Considerations

Reuses awiki v1's encryption + privacy posture. Task-layer-specific
risks:

| Risk | Mitigation |
|------|------------|
| `inbox.md` leaks unprocessed personal capture | `inbox.md` ships `draft: true` (Hugo skips). Covered by encryption patterns when `encrypt-init` run. Same path-traversal protections as `log.md`. |
| `triage_apply` writes outside intended directory | Server validates `project_slug`/`context_slug` resolve under `content/projects/` and `content/contexts/`. Path traversal rejected. |
| `capture` MCP tool used to inject newlines / break frontmatter | Embedded newlines rejected at MCP boundary; control chars stripped; line truncated at 2000 chars. |
| Wait-for points to a person entity that doesn't exist | Lint rule `T6`-style (extended to `wait:`) flags broken `[[entity-slug]]`. Existing wikilink lint catches. |
| Recurrence runs unbounded | Cap at 200 instances per chain. Lint warns at 150. |
| Agenda pages leak privacy via aggregation | Agenda pages are `draft: false` by default but list only action text + slugs (no body content). User can flip `draft: true` per page if a wiki ships sensitive action wording. |
| Agent triage rewrites user's inbox.md without consent | `triage_apply` always logs. Each outcome shown to user before commit. Bash fallback prints diff and prompts before writing. |

## Dependencies & Compatibility

No new tool requirements beyond v1. MCP server extension reuses Node
20+ already required. Scanner / agenda / recur / triage scripts are
pure bash 4+. No new optional tools.

## Implementation Phases

| # | Phase | Depends on | Deliverable |
|---|-------|------------|-------------|
| 16 | Schema + scaffold | v1 phases 1-3, 6 | `task-init.sh`, page-type enum extension in `WIKI.md`, project/context/inbox/agenda system pages, `capture.sh`, justfile recipes (`task-init`, `capture`), `<!-- BEGIN task-layer -->` block, BATS `task_init_test.sh`. |
| 17 | Scanner + agenda | 16 | `action-scan.sh`, `agenda.sh`, managed-region pages with markers, justfile recipes `agenda` + `scan`, lint rules T1-T7 (mechanical), `--fix` for date/key/id, BATS `action_scan_test.sh` + `agenda_test.sh` + `lint_task_test.sh`. Smoke 1-4 passes. |
| 18 | Triage + recurrence + MCP | 16, 17, v1 phase 8 | `mcp/awiki-server` extended with seven tools, `triage.sh` bash fallback, `action-recur.sh`, recur cap, lint rules T8-T13 (semantic), BATS `triage_test.sh` + `recur_test.sh` + `mcp_task_test.sh`. Smoke 5-6 passes. |
| 19 | Review + polish | 18 | `review-status.sh`, justfile `review` recipe, `mark_review_done` MCP tool, `agenda/review-log.md`, weekly-review WIKI.md subsection, pre-commit hook installer, README task-layer section, sample-wiki extended with example projects/contexts/inbox. Full smoke passes. |

**Definition of done per phase:** matches v1 spec — (a) phase tasks
check off, (b) phase tests pass, (c) phase smoke runs clean,
(d) merged to `main` with green pre-commit lint.

**Spike absorption:** one unverified assumption — Obsidian Tasks plugin
compatibility. Phase 17 starts with a 1-day spike: install Obsidian +
Tasks plugin against a fixture vault, confirm status markers + tail
metadata render and toggle as expected. Pin plugin version in
`.obsidian/community-plugins.json`.

**Migration:** none — opt-in feature on top of v1, not modifying
existing phases. Existing wikis adopt by running `just task-init`.
