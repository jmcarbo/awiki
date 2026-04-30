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
