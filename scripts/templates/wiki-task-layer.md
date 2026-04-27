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

The structured shell report `just review` (which chains `agenda.sh` -> `lint.sh` -> `review-status.sh`) prints `REVIEW|<key>|count=N|...` lines suitable for greppable inspection or dashboard ingestion. The MCP tool `review_status()` returns the same data as JSON.

**Privacy note:** when `encrypt-init` is enabled and `AWIKI_TASK_LAYER=on`, `task-init` (or running `encrypt-init` after `task-init`) covers `content/inbox.md` and `content/agenda/**` under git-crypt by default. See `WIKI.md` Section "Encryption" for migration steps if you ran `encrypt-init` before `task-init`.

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
