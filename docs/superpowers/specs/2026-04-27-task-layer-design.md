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
- Obsidian-friendly: status markers render correctly in vanilla Obsidian.
  Tail metadata uses `key:value` (Dataview-style), which the Obsidian
  Tasks plugin can read **only when its "Dataview format" option is
  enabled**; the plugin's default emoji syntax (📅, ⏳, 🔁, ✅) is NOT
  supported and not required. Block-IDs (`^id`) provide stable
  references. Phase 17 spike confirms Dataview-mode interop and pins
  plugin version.
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

- `scripts/task-init.sh` (per-step-idempotent enable script).
- `scripts/capture.sh` (append to `content/inbox.md`, with input
  sanitization).
- `scripts/lib/action-grammar.sh` (shared bash library: one regex set
  for action lines + tail metadata; sourced by scanner, lint, recur,
  and triage scripts so the parser cannot drift).
- `scripts/lib/lock.sh` (advisory `flock` wrapper around
  `.awiki/lock`; sourced by every mutating script and by the MCP
  server's mutating handlers).
- `scripts/action-scan.sh` (single-pass scanner → `.awiki/maps/actions.tsv`
  + `.awiki/maps/actions-rejected.tsv` for malformed lines that lint
  consumes).
- `scripts/agenda.sh` (regenerates five managed-region agenda pages
  via temp-file + atomic rename).
- `scripts/action-recur.sh` (re-emit on `[x] every:N` completion).
- `scripts/triage.sh` (bash fallback; MCP tool is primary).
- `scripts/review-status.sh` (structured weekly-review report).
- MCP server extension: `capture`, `triage_inbox`, `triage_apply`,
  `list_actions`, `rebuild_agenda`, `review_status`,
  `mark_review_done`.
- WIKI.md additions: page-kind enum entries, system-page entries,
  inline-grammar table, triage workflow, weekly-review workflow, lint
  rules T1-T14. All bracketed for clean removal.
- Lint rules T1-T14 (mechanical + semantic).
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
│   ├── lib/
│   │   ├── action-grammar.sh        # shared parser; sourced by all task scripts
│   │   └── lock.sh                  # flock wrapper around .awiki/lock
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
│   ├── lock                         # flock target (gitignored)
│   ├── last-review                  # ISO timestamp
│   ├── task-count                   # triage counter (auto-rebuild trigger)
│   └── maps/
│       ├── actions.tsv              # scanner output (gitignored)
│       └── actions-rejected.tsv     # malformed action lines (gitignored)
└── tests/
    ├── action_scan_test.sh
    ├── agenda_test.sh
    ├── triage_test.sh
    ├── recur_test.sh
    ├── lint_task_test.sh
    ├── task_init_test.sh
    └── mcp_task_test.sh
```

`task-init.sh` is **per-step idempotent** — see "`task-init` Workflow"
below. Each step detects existing state and skips its own work; partial
failures can be re-run safely.

---

## Concurrency & Atomicity

The task layer has multiple writers — `triage_apply` (MCP), `agenda.sh`,
`action-recur.sh`, `capture.sh`, `task-init.sh`, the optional pre-commit
hook, and the user editing in Obsidian. All share `content/**/*.md` and
`.awiki/maps/*.tsv`. Without coordination, races corrupt managed regions
and produce duplicate `^id`s.

### Advisory lock

- `.awiki/lock` is a single global advisory lock. Every mutating script
  sources `scripts/lib/lock.sh`, which wraps the body in `flock -x` on
  this file (timeout 30s, exit 7 on contention).
- The MCP server acquires the same lock around any mutating tool
  invocation (`capture`, `triage_apply`, `rebuild_agenda`,
  `mark_review_done`). Read-only tools (`triage_inbox`, `list_actions`,
  `review_status`) take a shared `flock -s` so they see consistent
  state but don't block each other.
- The lock is per-repo (path-scoped). Two checkouts of the same wiki
  can run independently.
- The pre-commit hook runs `action-scan.sh` only (read-only against
  `content/`); it takes the shared lock.

### Atomic file writes

- All managed-region rewrites (`agenda.sh`, `action-recur.sh`) write to
  `<file>.tmp.<pid>` and `mv` atomically into place. POSIX rename is
  atomic on the same filesystem.
- `triage_apply` and `capture` modify a single file at a time using the
  same temp+rename pattern.
- If the user edits the file in Obsidian during regen, Obsidian
  re-reads on `mv` (file-watcher event); user's in-flight buffer state
  may be lost if they had unsaved edits inside the managed region.
  Documented in WIKI.md task workflow.

### Auto-rebuild scheduling

- `triage_apply` increments `.awiki/task-count` **after** its own
  mutations commit. If the threshold is hit, it enqueues an
  `agenda.sh` run that fires *after* the current handler returns and
  releases the lock. No nested invocation — the handler returns to the
  caller first; the rebuild runs in a follow-up tool call slot, not
  inline.
- The bash equivalent: `triage.sh` runs `agenda.sh` as the last step
  before exiting, after closing the lock.

### Recurrence chain integrity

- `action-recur.sh` reads the canonical `actions.tsv` (built under
  lock) and refuses to mint a `^<base>~<n>` whose `<n>` already exists
  in the chain. This makes "fast double `[x]`" idempotent because the
  scan output the recur script reads is the snapshot taken under
  the same lock.

---

## Schema Additions to `WIKI.md`

All additions are bracketed by `<!-- BEGIN task-layer -->` /
`<!-- END task-layer -->` so the layer can be removed cleanly.

### `type` enum extension

Add to **page kinds** (Section 2 of `WIKI.md`):
- `project` — outcome requiring multiple actions; tracked to completion.
- `context` — situation/tool/place where actions can happen
  (`phone`, `errands`, `computer`, `home`, ...).

Areas of responsibility ("home", "career", "health") are NOT a new
page kind. They reuse existing `type: entity`. Project frontmatter
references them via `area: [[entity-slug]]`.

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

### Single-line constraint

Action lines are **strictly single-line**. Multi-line wrapped or
indented continuation is not parsed and not supported. Lint rule
`T15-action-continuation` warns if a checkbox line is followed by an
indented non-blank line on a page that contains other action lines.

### Status markers (render in vanilla Obsidian)

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

Obsidian convention. Stable cross-link target.

**Format:** `^[a-z0-9]{8}` (8-char base32, ~1.1 trillion namespace).
Probability of birthday collision below 1e-4 up to ~470k IDs — well
above any expected wiki lifetime.

**Minting:** `triage_apply`, `action-recur.sh`, and `lint --fix` mint
new IDs by sampling random base32 then verifying the candidate does
not appear in the current `actions.tsv` (built under lock). Up to 5
retries; failure aborts with exit 8 (effectively unreachable).

**User-typed IDs:** allowed if they match `^[a-z0-9]{3,16}$` AND do
NOT contain `~`. The `~` separator is reserved for recurrence chains
(see Recurrence). Lint rule `T14-bad-id-shape` errors on user-typed
IDs containing `~` or violating the length range.

**Recurrence chain:** instances after the chain head are named
`^<base>~<n>` where `<n>` ≥ 2 (the head has no suffix). `~` is
unambiguous because user-typed IDs cannot contain it. Lint rule `T2`
(duplicate ID) treats `^<base>` and `^<base>~<n>` as distinct
identities, but flags repeated `<n>` values within the same chain.

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

**Capture sanitization** (applies to both MCP tool and `capture.sh`):

| Pattern | Action |
|---------|--------|
| Newlines / `\r` / NUL / control chars (except tab → space) | Reject; exit 4 / MCP error. |
| Length > 2000 chars | Truncate, append `…`, log warning. |
| `<!--` or `-->` substrings | Replace with `< !--` / `--  >` (space-padded) so they cannot terminate a managed region or become a real HTML comment. |
| `[[` and `]]` (wikilink open/close) | Replace with `[ [` / `] ]` (space-padded) so an attacker cannot inject a phantom wikilink that lint will resolve. Captures that intentionally reference a wiki page must be edited post-triage on the destination page. |
| `^[a-z0-9]{4,}` block-ID-shaped tokens | Prefix with backslash (`\^abc123`) so the scanner does not treat them as block IDs. |
| Markdown checkbox prefix `[ ]`/`[/]`/etc. at line start | Reject — captures are not actions; line begins with `- <ISO-datetime> ` and the user's text follows. |

The reject and replace rules are documented with examples in
`docs/just-help.txt` and on `capture --help`.

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
| `reference` | Becomes a wiki page: existing ingest pattern — agent proposes `<page_type, slug>` to `triage_apply`; MCP server validates `slug` matches `^[a-z0-9][a-z0-9-]{0,63}$` AND `page_type ∈ {entity, concept, topic, source}` AND the resolved path stays under `content/<page_type>s/`. On valid input, agent writes the page body via existing tools. Logged. |
| `someday` | `- [>] <text> ^<id>` on relevant project page or `content/projects/_someday.md` catch-all. Logged. |

`_loose.md` and `_someday.md` are normal project pages (`type: project`,
`status: active` and `status: someday` respectively); the underscore
prefix is convention for "catch-all". Created lazily on first use, not
by `task-init`.

Each outcome:
1. Removes original capture line from `inbox.md` (or moves file from
   `raw/inbox/interactive/` → `raw/processed/`).
2. Logs via `scripts/log-append.sh triage "<outcome> | <slug>"`.
3. Increments `.awiki/task-count`. At threshold (resolved env →
   `.awiki/config` → default 5), `agenda.sh` auto-rebuilds via the
   deferred-enqueue path described in Concurrency & Atomicity.

### Triage MCP tool surface

```
triage_inbox()
  → returns: [{ source: "inbox.md" | "raw/inbox/...", line_or_path: ...,
                text: ..., id: ... }, ...]

triage_apply(id, outcome, params)
  outcome ∈ {trash, do-now, act, defer-scheduled, waiting,
             reference, someday}
  params: { project_slug?,                  // ^[a-z0-9][a-z0-9_-]{0,63}$ (underscore allowed for _loose, _someday)
            context_slug?,                  // ^[a-z0-9][a-z0-9-]{0,63}$
            wait_for?,                      // ^[a-z0-9][a-z0-9-]{0,63}$ (entity slug, no [[ ]])
            page_type?,                     // ∈ {entity, concept, topic, source} (reference outcome only)
            ref_slug?,                      // ^[a-z0-9][a-z0-9-]{0,63}$ (reference outcome only)
            due?,                            // YYYY-MM-DD
            defer? }                         // YYYY-MM-DD
  → writes pages, removes/moves source, logs,
    returns: { ok, actions_taken: [...], created_pages: [...], updated_pages: [...] }
```

**Slug validation:** every slug param is regex-checked at the MCP
boundary. After regex pass, the server resolves the destination path
(`content/projects/<slug>.md`, etc.) using `path.resolve` and rejects
the call if the resolved path does not have the expected parent
directory as prefix. This blocks `..`-traversal even if the regex
is loosened in the future.

**Date validation:** `due:` and `defer:` must be valid ISO calendar
dates (parsed via `Date.UTC`); rejects `2026-02-30`, `2026-13-01`, etc.

`triage_apply` is the single mutation entry-point. Bash fallback:
`scripts/triage.sh <id> <outcome> [k=v ...]` invokes the same logic for
cold-start / no-MCP situations.

---

## Scanner + Agenda Generation

### `scripts/action-scan.sh`

Single pass over `content/**/*.md`. Sources `scripts/lib/action-grammar.sh`
for the canonical action-line regex (the same library lint sources, so
the two parsers cannot drift). Outputs `.awiki/maps/actions.tsv` and
`.awiki/maps/actions-rejected.tsv` (both gitignored).

`actions.tsv` columns:

```
id  status  text  file  line  context  due  defer  wait  since  every  done  priority  est  project  source_kind
```

- `project` column resolved by walking up: action's containing file's
  frontmatter `type:` — if `project`, that slug; else `""`.
- `context` resolved from inline `@<slug>` token, normalized via
  context alias map. Empty if no context.
- `source_kind` ∈ `{public, private}`. `private` if the file path
  matches any pattern from `.gitattributes` git-crypt section
  (typically `content/private/**`, `raw/processed/private/**`) OR if
  the file's frontmatter contains `tags: [private]`. Used by
  `agenda.sh` for the privacy filter (see below).
- Lines without a valid `[STATUS]` checkbox marker are skipped from
  `actions.tsv` and instead emitted to `actions-rejected.tsv` with
  reason codes (bad-status, bad-key, bad-date, etc.) for lint to
  consume — single parse, two consumers.
- Multi-line continuation: action lines are strictly single-line.
  Indented non-blank lines following a checkbox line on a page that
  contains other actions are emitted to `actions-rejected.tsv` with
  reason `continuation`; lint rule `T15` warns.
- Lines under `## Done` heading on project pages still scanned (state
  = `[x]`); used for "completed since last review" delta.

**Inbox-line ID synthesis** (used by `triage_inbox` / `triage_apply`):

- For inbox lines: `id = inbox-<sha1(line-text)[:10]>`. Stable as long
  as the line text doesn't change. If the user edits the inbox file
  between `triage_inbox()` and `triage_apply(...)`, the ID may no
  longer resolve — `triage_apply` returns error `stale-id` and asks
  the agent to re-call `triage_inbox()`.
- For files in `raw/inbox/interactive/`: `id = file-<sha1(relpath)[:10]>`.
- For action lines on content pages: `id = <^id>` (the block-ID).

Idempotent. Stdout = scan summary
(`scanned N files, M actions, R rejected, K open, L done`). Exit 0
clean, 1 if duplicate `^id` detected (lint surfaces same).

### `scripts/agenda.sh`

Reads `actions.tsv`, regenerates the five managed-region pages via
temp-file + atomic rename (see Concurrency & Atomicity). Each
generated file's frontmatter `last_updated` is rewritten to the
current date on every regen (the staleness-rule exemption noted in
schema is a backup; this is the primary mechanism).

**Privacy filter** (mandatory):

- Actions where `source_kind=private` are excluded from generated
  agenda pages by default.
- For each agenda page, if any private actions were excluded, append
  inside the managed region:
  ```
  > _N action(s) hidden — origin under private/encrypted path._
  ```
  Counts only; no slugs, no text.
- Override: setting `AWIKI_AGENDA_INCLUDE_PRIVATE=1` in `.awiki/config`
  inlines private actions into agenda pages **only if** the agenda
  pages are themselves under the same encryption envelope (i.e.
  `task-init` was run after `encrypt-init` and `.gitattributes`
  covers `content/agenda/**` — see `task-init` step 3a). Otherwise
  the override is rejected and the script exits 5.

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
- Auto: `triage_apply` increments `.awiki/task-count`; at threshold,
  enqueues `agenda.sh` (see Concurrency & Atomicity). Counter resets.
  Threshold resolution: `AWIKI_AGENDA_AFTER_N` env var if set,
  otherwise the value in `.awiki/config`, otherwise default 5.
  Mirrors v1's `AWIKI_LINT_AFTER_N` lookup chain.
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
   line with same `^<base>` and a higher `~<n>` index.
2. `action-recur.sh` re-emits an open copy on the same page,
   immediately above the `[x]` line:

   ```
   - [ ] water plants @home every:1w due:2026-05-11 ^a05~2
   - [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
   ```
3. New `^id` = `<base>~<n>`, where `<base>` is the chain head's ID
   (no `~`) and `<n>` is monotonically increasing within the chain
   starting at 2. Lint enforces uniqueness across chain (`T2`) and
   shape (`T14`).
4. **`every:` interval semantics:**
   - `Nd` / `daily` / `Nw` / `weekly`: pure day arithmetic on UTC
     calendar dates. No DST adjustment (dates carry no time).
   - `Nm` / `monthly`: month-add with **last-day clamp**. `done:2026-01-31, every:1m → 2026-02-28`. `done:2026-03-15, every:1m → 2026-04-15`. Never produces an invalid date.
   - The `every:` value is read from the **completed line being
     processed**, not from the chain head. If the user edits the
     interval mid-chain (e.g., changes `every:1w` to `every:2w` on
     instance `^a05~3`), the next emit uses `2w`. Documented in
     WIKI.md.
5. Computed `due:` = `done` + interval. If `done:` is missing (user
   forgot to stamp), use today. `lint --fix` populates missing
   `done:` before recur runs in the canonical pipeline.
6. **Race protection** (see Concurrency & Atomicity): recur runs
   under the same lock as the scanner snapshot, so multiple `[x]`
   flips committed in one file edit produce one re-emit per chain
   per scanner pass — never two.
7. Dry-run mode `action-recur.sh --dry-run` prints the diff without
   writing — used by lint and `--fix` workflows.

**Recurrence cap:**
- `action-recur.sh` **refuses to emit** (exits 6, no write) if the
  chain has already produced ≥200 instances on one page.
- Lint rule `T12-recur-chain-cap` warns at 150, errors at 200. The
  hard refuse-to-emit and the lint error are independent: the
  refusal prevents new growth; the lint error surfaces the cap
  violation if a chain somehow reached 200 by other means
  (e.g., manual user edits).

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

All task-layer rules fire **only on lines matching the action grammar**
(`scripts/lib/action-grammar.sh`). Stray `@mentions`, `[ ]` literals
inside fenced code blocks, and prose `^abc` tokens do not trigger
these rules.

| Level | Rule | Trigger |
|-------|------|---------|
| error | `T1-bad-status` | Checkbox marker not in `{ ,/,?,>,x,-}`. |
| error | `T2-dup-id` | Same exact `^id` (literal match including any `~<n>` suffix) appears on ≥2 action lines. The recur chain shape `^<base>` + `^<base>~<n>` is permitted as long as each instance ID is unique. Within a chain, `<n>` values must be unique. |
| error | `T3-bad-key` | Tail key not in allowed set (`due,defer,wait,since,every,done,priority,est`). |
| error | `T4-bad-date` | `due:`/`defer:`/`since:`/`done:` value not a valid `YYYY-MM-DD` calendar date. |
| error | `T5-waiting-no-person` | `[?]` line without `wait:`. |
| error | `T6-bad-context` | Action line carries `@<slug>` referencing a context page that does not exist. T6 fires only on lines matching the action grammar (NOT on prose `@mentions`). |
| error | `T7-managed-region-edited` | Hand-edit detected inside `<!-- BEGIN agenda:... -->` / `<!-- END -->`. |
| warn | `T8-no-next-action` | `type: project, status: active` page has zero open `[ ]`/`[/]` actions. **Exempt:** `_loose.md`, `_someday.md`, and any project page with `status: someday` or `status: done`. |
| warn | `T9-waiting-stale` | `[?]` line where `since:` > 14 days ago. |
| warn | `T10-overdue` | `[ ]`/`[/]` line where `due:` < today. |
| warn | `T11-stale-someday` | `[>]` line whose enclosing page hasn't been touched in 90+ days. |
| warn | `T12-recur-chain-cap` | Recur chain at ≥150 (warn at 150, error at 200). Independent of `action-recur.sh`'s refuse-to-emit at 200. |
| info | `T13-context-unused` | `type: context` page with zero referencing actions. |
| error | `T14-bad-id-shape` | Block-ID does not match `^[a-z0-9]{3,16}$`, OR contains `~` outside the recurrence-chain shape `^<base>~<digits>`. |
| warn | `T15-action-continuation` | Indented non-blank line follows a checkbox line on a page that contains other actions (multi-line wrapped action — not supported). |

`lint.sh --fix` mechanical fixes (task-aware):
- Normalize date format (`2026/4/27` → `2026-04-27`).
- Auto-stamp missing `done:` on `[x]` lines (= today). Fidelity loss
  acknowledged: if a user manually flipped to `[x]` days ago and only
  now runs lint, the stamp will say today, not the actual completion
  date. The MCP `triage_apply` path always stamps `done:` at flip
  time, so this fidelity loss only affects pure-Obsidian editing
  without periodic lint.
- Auto-stamp missing `since:` on `[?]` lines (= today). Same fidelity
  caveat. `triage_apply` for the `waiting` outcome always stamps
  `since:` at the moment of the flip.
- Sort tail keys to canonical order:
  `@context due defer wait since every priority est done`.
- Mint missing `^id` (random base32, 8 chars; collision-checked
  against current `actions.tsv`, retries up to 5×) — preserves
  existing IDs.

---

## MCP Server Additions

`mcp/awiki-server/index.js` extended. New tools:

| Tool | Args | Returns | Side effects |
|------|------|---------|--------------|
| `capture` | `{text}` | `{appended:true, line, timestamp}` | Appends to `content/inbox.md`. |
| `triage_inbox` | `{}` | `[{id, source, text, captured_at, ...}]` | Read-only scan of `inbox.md` lines + `raw/inbox/interactive/` files. |
| `triage_apply` | `{id, outcome, params}` | `{ok, actions_taken[], created_pages[], updated_pages[], stale_id?}` | Mutates origin + destination files; logs; increments `task-count`. Returns `{ok:false, stale_id:true}` if the `id` no longer resolves (caller should re-run `triage_inbox()`). |
| `list_actions` | `{filter?: {status?, context?, project?, due_before?, wait?, overdue?}}` | `[action]` | Read-only; calls `action-scan.sh` if scan map stale. |
| `rebuild_agenda` | `{}` | `{rebuilt:[<file>], duration_ms}` | Runs `action-scan.sh` + `agenda.sh`. |
| `review_status` | `{}` | (see Weekly Review JSON above) | Read-only. |
| `mark_review_done` | `{}` | `{last_review:<ISO>}` | Writes `.awiki/last-review`, appends `review-log.md`. |

### Security

- All shell-outs continue using `execFileSync` with arg arrays
  (no shell interpolation).
- **Per-arg validation at the MCP boundary**, applied before any
  filesystem touch:
  - `id`: `^[a-z0-9~-]{1,32}$` (allows `inbox-...`, `file-...`, plain
    block-ID, recurrence-chain IDs).
  - `outcome` ∈ exact enum (no superset, no case variants).
  - `project_slug`: `^[a-z0-9_][a-z0-9_-]{0,63}$` (underscore-prefix
    allowed for `_loose`, `_someday`).
  - `context_slug`, `wait_for`, `ref_slug`: `^[a-z0-9][a-z0-9-]{0,63}$`.
  - `page_type` ∈ `{entity, concept, topic, source}`.
  - `due`, `defer`: valid `YYYY-MM-DD` calendar date (parsed via
    `Date.UTC`, rejects 2026-02-30, 2026-13-01, etc.).
- **Path-resolution guard:** after regex pass, the server resolves
  every destination path (`content/projects/<project_slug>.md`, etc.)
  via `path.resolve` and rejects the call unless the resolved path
  starts with the expected canonical parent (`<repo-root>/content/projects/`,
  `<repo-root>/content/contexts/`, `<repo-root>/content/<page_type>s/`).
  Catches traversal even if regex is loosened.
- **`capture` text sanitization:** see "Capture sanitization" table
  in the Capture + Triage Workflow section. The MCP boundary applies
  the same rules as `capture.sh`. Specifically: control chars and
  embedded newlines rejected outright; `<!--`/`-->`/`[[`/`]]`
  neutralized; block-ID-shaped tokens backslash-escaped.
- **Concurrency:** every mutating tool acquires `flock -x .awiki/lock`
  (timeout 30s); read-only tools take `flock -s`. See Concurrency &
  Atomicity.
- **Log-line safety:** before writing a log entry, `log-append.sh`
  replaces `|`, `\n`, and `\r` in any caller-supplied substring with
  `_`, ` `, ` ` respectively. Prevents log poisoning via slugs that
  contain pipe or newline.
- All seven tools live in the same stdio-only server. No new
  transport, no network listener.

### Wiring

Existing `scripts/wire-awiki-mcp.sh` already registers the server; new
tools auto-exposed once `mcp/awiki-server` is rebuilt
(`cd mcp/awiki-server && npm install`). `task-init` does NOT re-wire —
prints the rebuild instruction; user runs once.

---

## `task-init` Workflow

`scripts/task-init.sh` is **per-step idempotent**: each step checks
its own state and skips work that is already done, so partial-failure
re-runs are safe. There is no binary "already enabled" gate.

Steps:

1. **Pages:** for each path in the create list, write it only if the
   file does not already exist (`[ -f "$f" ] && skip`). Files:
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
     with `aliases: ['@phone']` etc.
2. **Patch `WIKI.md`:** if the `<!-- BEGIN task-layer -->` marker is
   absent, append the templated section (page-kind enum entries,
   system-page entries, inline-grammar table, triage workflow,
   weekly-review workflow, lint rules T1-T15). If the marker is
   present but the contents differ from the template, print a warning
   and skip — user has customized; manual reconciliation required.
3. **Encryption coverage:** detect `encrypt-init` state by reading
   `.gitattributes`. If git-crypt section is present, prompt:
   "Add `content/inbox.md` and `content/agenda/**` to git-crypt
   patterns? (Y/n)". On `Y`, append the patterns atomically. On
   `n`, print warning that agenda pages will exclude private actions
   (privacy filter).
4. **Patch `.awiki/config`:** if keys absent, append
   `AWIKI_AGENDA_AFTER_N=5` and `AWIKI_TASK_LAYER=on`. Existing
   user-set values preserved.
5. **State files:** create `.awiki/last-review=<today>` and
   `.awiki/task-count=0` only if missing.
6. **Pre-commit hook:** if no `git/hooks/pre-commit` exists OR the
   existing hook does not contain the `# task-layer` marker, prompt:
   "Install task-aware pre-commit hook (runs action-scan + lint)? (y/N)".
   On `y`, writes / appends with marker.
7. **MCP rebuild prompt:** if `mcp/awiki-server/index.js` is already
   wired (`.mcp.json` or `.codex/config.toml` mentions `awiki`),
   prints "rebuild server: `cd mcp/awiki-server && npm install`".
   If not wired, prints how to wire.
8. **Log:** `scripts/log-append.sh task-init "enabled"`. Logged once
   per first-time enable; idempotent re-runs append a `task-init|noop`
   line instead.
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
`capture.sh` joins with spaces. Single-quote any text containing shell
metacharacters: `just capture '[[link]] and $vars'` — without quoting,
the user's shell may glob, expand, or split the input before `just`
sees it.

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
| `recur_test.sh` | `[x] every:1w done:2026-04-27` → emits `[ ] every:1w due:2026-05-04 ^<base>~2`. Idempotent: re-run does not double-emit. `every:1m` last-day clamp covered (e.g. 2026-01-31 → 2026-02-28). Cap at 200 (refuse-to-emit). Concurrent flips race-tested via `flock` contention. |
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
| `inbox.md` leaks unprocessed personal capture | `inbox.md` ships `draft: true` (Hugo skips). `task-init` step 3 prompts user to extend `.gitattributes` git-crypt patterns to cover `content/inbox.md` if `encrypt-init` already ran. v1's `encrypt-init` is also extended (phase 19) so it includes `content/inbox.md` and `content/agenda/**` by default if `task-init` has run. |
| Agenda pages leak private actions to public render | `action-scan.sh` flags actions whose origin file is under encrypted patterns or carries `tags: [private]` as `source_kind=private`; `agenda.sh` excludes them from generated pages by default and emits "_N action(s) hidden_" placeholder. Override `AWIKI_AGENDA_INCLUDE_PRIVATE=1` only honored when agenda directory itself is encrypted. |
| `triage_apply` writes outside intended directory | All slug params regex-validated AND path-resolution-guarded against canonical parent directories. `..`-traversal rejected even if regex loosens. |
| `capture` injects markdown / wikilinks / managed-region markers | Sanitization at both MCP and `capture.sh` entrypoints: control chars rejected, `<!--`/`-->`/`[[`/`]]` neutralized with space-padding, block-ID-shaped tokens backslash-escaped, length capped at 2000 chars. |
| Concurrent triage / agenda regen / Obsidian edit corrupts files | `flock -x .awiki/lock` around every mutating script and MCP handler; managed-region writes via temp+atomic rename; user edits inside managed regions during regen window may be lost (documented). |
| Wait-for points to a non-existent entity | Lint rule `T6`-style (extended to `wait:`) flags broken `[[entity-slug]]`; existing wikilink lint catches. |
| Recurrence runs unbounded | `action-recur.sh` refuses to emit at ≥200 per chain (exit 6, no write). Lint `T12` warns at 150, errors at 200, independently. |
| Block-ID collision under random-mint | 8-char base32 (1.1T namespace) + collision-check-and-retry on mint against current `actions.tsv`. Birthday collision <1e-4 up to ~470k IDs. |
| User-typed `^a-2` collides with recurrence chain | Recurrence chain uses `~` separator (`^a~2`). `~` in user-typed IDs is forbidden by `T14`. |
| Log poisoning via `\|` or `\n` in slug | `log-append.sh` neutralizes `\|`, `\n`, `\r` in any caller-supplied substring. |
| Agent triage rewrites `inbox.md` without consent | `triage_apply` always logs. Each outcome shown to user before commit. Bash fallback prints diff and prompts before writing. |

## Dependencies & Compatibility

No new tool requirements beyond v1. MCP server extension reuses Node
20+. Scanner / agenda / recur / triage scripts use the bash 4+ already
required by v1. `flock` is required (POSIX util-linux on Linux,
`flock` from Homebrew on macOS — already present on macOS via
`util-linux` / `coreutils` in most awiki user setups; v1's
dependency-check is extended in phase 16 to verify it).

## Implementation Phases

| # | Phase | Depends on | Deliverable |
|---|-------|------------|-------------|
| 16 | Schema + scaffold | v1 phases 1-3, 6 | `task-init.sh`, `scripts/lib/action-grammar.sh`, `scripts/lib/lock.sh`, dep-check extended for `flock`, page-type enum extension in `WIKI.md`, project/context/inbox/agenda system pages, `capture.sh` with sanitization, justfile recipes (`task-init`, `capture`), `<!-- BEGIN task-layer -->` block, BATS `task_init_test.sh`. |
| 17 | Scanner + agenda | 16 | `action-scan.sh` (uses shared grammar lib, emits both `actions.tsv` and `actions-rejected.tsv`, tags `source_kind`), `agenda.sh` (privacy filter, atomic rename, `last_updated` rewrite), managed-region pages with markers, justfile recipes `agenda` + `scan`, lint rules **T1-T7, T14, T15** (mechanical + grammar + scoping rules), `--fix` for date/key/id (8-char + retry), BATS `action_scan_test.sh` + `agenda_test.sh` + `lint_task_test.sh`. **Spike:** Obsidian Tasks plugin Dataview-mode interop — confirms behavior and pins plugin version. Smoke 1-4 passes. |
| 18 | Triage + recurrence + MCP | 16, 17, v1 phase 8 | `mcp/awiki-server` extended with seven tools (full slug+path+date validation, capture sanitization, lock acquisition), `triage.sh` bash fallback, `action-recur.sh` (clamp arithmetic, `~`-separator IDs, refuse-to-emit cap), lint rules T8-T13 (semantic; T8 exemption for `_loose`/`_someday`), `log-append.sh` poisoning protection, BATS `triage_test.sh` + `recur_test.sh` + `mcp_task_test.sh`. Smoke 5-6 passes. |
| 19 | Review + polish | 18 | `review-status.sh`, justfile `review` recipe, `mark_review_done` MCP tool, `agenda/review-log.md`, weekly-review WIKI.md subsection, pre-commit hook installer (with task-layer marker), README task-layer section, sample-wiki extended with example projects/contexts/inbox, **`encrypt-init` updated** to include `content/inbox.md` and `content/agenda/**` by default when `AWIKI_TASK_LAYER=on`. Full smoke passes. |

**Definition of done per phase:** matches v1 spec — (a) phase tasks
check off, (b) phase tests pass, (c) phase smoke runs clean,
(d) merged to `main` with green pre-commit lint.

**Spike absorption:** one unverified assumption — Obsidian Tasks
plugin Dataview-mode interop. Phase 17 starts with a 1-day spike:
install Obsidian + Tasks plugin against a fixture vault, enable the
plugin's "Dataview format" option, confirm status markers + key:value
tail metadata render and toggle as expected, AND confirm the default
emoji format does NOT silently mis-parse our key:value tokens. Pin
plugin version in `.obsidian/community-plugins.json`. If interop is
not viable, downgrade Goals: "renders correctly in vanilla Obsidian"
without plugin-compat claim.

**Migration:** none — opt-in feature on top of v1, not modifying
existing phases. Existing wikis adopt by running `just task-init`.
