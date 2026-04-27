# Template Update Mechanism — Design

**Date:** 2026-04-27
**Status:** Approved (pending user written-spec review)
**Type:** Feature on awiki template repository
**Depends on:** [LLM Wiki Scaffold](2026-04-27-llm-wiki-scaffold-design.md)

## Summary

Mechanism for repositories that have already bootstrapped from the awiki template to pull subsequent updates (new scripts, MCP server fixes, deploy templates, schema changes, new bootstrap steps) without losing per-domain customizations. Detached-template + manifest model: the bootstrapped repo records template provenance (`{repo, ref, version, commit, applied_migrations, deleted, bootstrap_steps_done}`), and a `just template-update` recipe fetches a fresh template, syncs by per-path strategy, runs versioned migrations, prompts for new bootstrap steps, and writes everything onto a dedicated update branch for review.

## Goals

- **Detached.** No shared git history between template and bootstrapped repos. Privacy-safe for encrypted wikis.
- **Manifest-driven.** Per-path strategy declared in `template.manifest.toml` shipping with template.
- **Three-way merge for hybrid files.** `WIKI.md`, `hugo.toml`, `content/_index.md`, etc. resolved via `git merge-file` against cached ancestor.
- **Versioned migrations.** Mechanical (`*.sh`) and LLM-assisted (`*.prompt.md`) migrations run in order; idempotent; tracked in `applied_migrations[]`.
- **Re-runnable bootstrap steps.** New BOOTSTRAP steps detected by ID and replayed on update.
- **Auditable.** Default = dry-run plan. `--apply` writes to dedicated branch, one commit per phase.
- **Bash-first.** No external toolchain (no copier/cruft/python). Reuses git's merge engine.

## Non-goals

- Hosted update service.
- Cross-template migrations (e.g., from a different scaffold).
- Auto-merge of conflicts (user resolves via standard `<<<<<<<` markers).
- Theme submodule version management (preserved; user upgrades manually).
- Backward-merge from bootstrapped repos into template (one-way only).

## Decisions (from brainstorm)

- **Distribution:** detached + manifest. Not git-remote, not copier/cruft.
- **Versioning:** SHA-pinned, optional human-readable tag.
- **Hybrid file strategy:** 3-way merge via `git merge-file` with cached ancestor.
- **Migrations:** mechanical bash + optional LLM prompt files. Both numbered.
- **Fetch source:** URL or local path in `.awiki/template.json`.
- **User-deleted files:** tracked explicitly in `.awiki/template.json.deleted[]`.
- **New bootstrap steps:** detected by step ID; re-prompted on update.
- **Safety:** dry-run by default; `--apply` runs on dedicated branch with per-phase commits.

---

## Architecture

Five components in the bootstrapped repo:

1. **`.awiki/template.json`** — pinned provenance. Single source of truth for "what template state this repo is at". Tracked in git.
2. **Template cache** — `.awiki/template-cache/<sha>/` holds last-applied template tree (the merge ancestor). Plus `_fetch/` transient working dir during update. Gitignored.
3. **`template.manifest.toml`** — ships at template root. Declares per-path strategy and BOOTSTRAP step ordering.
4. **`scripts/template-update.sh`** — orchestrator with phases: preflight → fetch → plan → apply (sync, migrations, bootstrap-steps, provenance). Each phase = one commit on update branch.
5. **`migrations/NNNN-*.sh|.prompt.md`** — versioned. Mechanical bash and/or agent-facing prompt markdown. Numbering monotonic, never renumbered.

Layer boundaries: fetch is pure git ops, plan is pure read+diff, apply mutates only on `awiki-template-update/<sha>` branch.

---

## `.awiki/template.json` schema

```json
{
  "schema_version": 1,
  "repo": "https://github.com/jmcarbo/awiki",
  "ref": "main",
  "version": "1.0.0",
  "commit": "abc123def456...",
  "applied_migrations": ["0001", "0002", "0003"],
  "deleted": [
    { "path": "scripts/ingest-audio.sh", "reason": "no audio sources" }
  ],
  "bootstrap_steps_done": [
    { "id": "domain", "skipped": false },
    { "id": "wire-qmd-mcp", "skipped": true, "reason": "user declined" }
  ]
}
```

Tracked in git so update history is auditable.

---

## Manifest schema

`template.manifest.toml` at template root, versioned with template:

```toml
schema_version = 1
template_version = "1.0.0"

[strategies]
overwrite     = ["scripts/**", "layouts/**", "mcp/awiki-server/**", "deploy/**", "scheduled/**", "BOOTSTRAP.md", "README.md", "justfile", "tests/**"]
preserve      = ["content/**", "raw/**", ".obsidian/workspace*", "secrets/**", "themes/**"]
three_way     = ["WIKI.md", "hugo.toml", "content/_index.md", "content/log.md", ".gitignore", ".gitattributes", "CLAUDE.md", "AGENTS.md"]
template_only = ["template.manifest.toml", "migrations/**"]

[new_file_default]
strategy = "prompt"   # overwrite | skip | mark-as-user-deleted

[bootstrap]
ordered_steps = [
  "dep-check", "domain", "wiki-name", "privacy", "track-processed",
  "theme", "publish-log", "patch-identity", "install-qmd",
  "wire-qmd-mcp", "wire-awiki-mcp", "log-init", "stage-commit", "smoke-test"
]
```

### Strategy semantics

| strategy        | behavior                                                                         |
|-----------------|----------------------------------------------------------------------------------|
| `overwrite`     | Template wins. User edits lost. (Pure template-owned files.)                     |
| `preserve`      | User wins. Template version ignored. (User-owned files.)                         |
| `three_way`     | `git merge-file` with cached ancestor as base. Conflicts emit `<<<<<<<` markers. |
| `template_only` | Same as overwrite, but suppressed from plan output (infra plumbing).             |

`new_file_default = prompt`: files appearing in new template that match no glob get an inline prompt during plan apply. User chooses overwrite | skip | mark-as-user-deleted.

### Conflict path

If `git merge-file` returns nonzero, file left with markers; plan summary lists conflicts; user resolves before apply finalizes via `--continue`. Migration runner checks for unresolved markers (`grep -rE '^<<<<<<< '`) before proceeding.

---

## Update flow

CLI:

```
just template-update [--ref <sha-or-tag>] [--source <url-or-path>] [--apply] [--continue] [--abort] [--status] [--skip-migration NNNN]
```

Default = dry-run plan. `--apply` mutates.

### Phase 0 — preflight

- Verify `git status` clean (no unstaged, no untracked-that-matter). Halt otherwise. No `--force` flag.
- Verify on `main` (or user's default branch). Halt if already on `awiki-template-update/*`.
- Read `.awiki/template.json` → pinned `{repo, commit_old}`.

### Phase 1 — fetch

- `git clone --depth 50 <source> .awiki/template-cache/_fetch` (or `git fetch` if cache exists).
- Resolve `--ref` → `commit_new`. Default = `origin/main` HEAD.
- If `commit_new == commit_old`, exit 0 with "already up to date".
- Snapshot ancestor tree expected at `.awiki/template-cache/<commit_old>/`. Bootstrap MUST seed this directory at install time. If missing, halt with retrofit instruction.

### Phase 2 — plan (always; dry-run prints + exits)

Diff `commit_old..commit_new` paths against NEW manifest. Categorize:

- `overwrites[]` — listed for transparency.
- `preserves[]` — no-op, listed.
- `three_way[]` — predict conflicts via `git merge-file --diff3 -p ... | grep -c '<<<<<<<'`.
- `template_only[]` — silent.
- `new_files[]` — in new + missing in old + present in current repo: prompt strategy.
- `deletions_in_template[]` — gone in new; would be removed unless user has modified locally.
- `user_deleted[]` — present in `template_old + new` but missing locally: confirm skip per `.awiki/template.json.deleted[]`.

Migrations: scan `migrations/` for files with index > max(`applied_migrations`).
Bootstrap steps: `bootstrap.ordered_steps` IDs not in `bootstrap_steps_done[]`.

Plan emitted as structured stdout (`PLAN|<phase>|<action>|<path>|<note>`) plus human summary. Exit 0 if no `--apply`.

### Phase 3 — apply (only with `--apply`)

Create branch `awiki-template-update/<short-commit_new>`. Halt if exists.

**Commit A — sync:**
- Apply `overwrite`, `three_way` (merge-file), prompted `new_files`, `template_only`.
- Process `deletions_in_template[]` (remove unless locally modified — if modified, prompt).
- Skip `user_deleted[]` per recorded list.
- Stage + commit `chore(template): sync to <version> (<short-sha>)`.

**Halt for conflicts:** if any `<<<<<<<` markers present after sync, stop. User resolves, runs `just template-update --continue`.

**Commit B — migrations:**
- Run mechanical (`*.sh`) in numeric order. Each gets `--dry-run` first to preview, then real run.
- LLM migrations (`*.prompt.md`) staged into `.awiki/pending-prompts/<NNNN>-<slug>.md`; not executed.
- Stage migration outputs + update `applied_migrations[]` in `.awiki/template.json` (mechanical only — LLM IDs added later by agent).
- Commit `chore(template): run migrations N..M`.

**Commit C — bootstrap-steps:**
- For each new step ID, prompt user inline (re-using BOOTSTRAP.md prompt body extracted between `<!-- bootstrap-step: ID -->` markers).
- Run associated commands.
- Append `{id, skipped}` to `bootstrap_steps_done[]`.
- Commit `chore(template): bootstrap steps <ids>`.

**Commit D — provenance:**
- Update `.awiki/template.json` `commit/version`.
- Move `.awiki/template-cache/_fetch` → `.awiki/template-cache/<commit_new>/`. Drop older cache dirs (keep latest two).
- Commit `chore(template): pin to <version>`.

Print: "Update branch ready. Review with `git diff main`. Merge: `git switch main && git merge --no-ff awiki-template-update/<sha>`. Pending LLM migrations: `.awiki/pending-prompts/`."

### `--continue` / `--abort`

- `--continue` resumes after conflict resolution at next phase. Detects current state via existing commits on update branch.
- `--abort`: `git checkout main && git branch -D awiki-template-update/<sha>`, clean `_fetch`. `.awiki/template.json` untouched (still on old pin).

---

## Migrations

Two kinds. Numbered sequence shared. Numbering: four-digit zero-padded, monotonic, never renumbered. Gaps OK.

### Mechanical (`migrations/NNNN-<slug>.sh`)

- bash 4+, `#!/usr/bin/env bash`, `set -euo pipefail`. Same conventions as awiki scripts.
- Idempotent. Re-run = no-op.
- Required `--dry-run` flag — prints intended changes only.
- Runner injects env: `AWIKI_REPO_ROOT`, `AWIKI_TEMPLATE_OLD_VERSION`, `AWIKI_TEMPLATE_NEW_VERSION`.
- Operates on user content (`content/`, `raw/`, `WIKI.md`, etc.). MUST NOT touch `.awiki/`.
- Failure (nonzero exit) → halt update at Commit B. User fixes, runs `just template-update --continue`.
- Skip via `--skip-migration NNNN`. Skipped IDs recorded as `{id: "NNNN", skipped: true, reason}` in `applied_migrations[]` — never re-attempted unless manually unskipped.
- Examples: rename frontmatter field across `content/**/*.md`, add new section to `WIKI.md`, move file location, patch `hugo.toml` key.

### LLM-assisted (`migrations/NNNN-<slug>.prompt.md`)

Markdown with frontmatter:

```yaml
---
id: 0007-rewrite-sources-blocks
requires: [agent]
scope_glob: "content/synthesis/**/*.md"
---
```

Body = instructions for agent. Runner does NOT execute. Stages copy into `.awiki/pending-prompts/<NNNN>-<slug>.md` with resolved file list appended.

After update merge, agent picks up next session: `WIKI.md` workflow updated to say:

> On startup, check `.awiki/pending-prompts/`. If non-empty, surface pending migrations to user, work through one at a time, delete file when done, append `<NNNN>` to `applied_migrations[]` in `.awiki/template.json`.

Lint warns on aged pending prompts (>14d).

Update tool refuses to advance pin if any LLM migration in scope is still pending — avoids skipping over.

---

## Bootstrap integration

### Step IDs

Each numbered BOOTSTRAP step gets an ID comment. Lint enforces presence:

```markdown
### Step 9b. Wire awiki MCP server
<!-- bootstrap-step: wire-awiki-mcp -->
Ask: "Wire awiki wiki-ops MCP server (ingest/lint/query/update_catalog)? (y/N)". On `y`, run `cd mcp/awiki-server && npm install && cd ../..` then `bash scripts/wire-awiki-mcp.sh`.
```

ID list canonical in `template.manifest.toml.bootstrap.ordered_steps`.

### New BOOTSTRAP step (last, before smoke-test)

```
N. Seed template provenance.
   Run: bash scripts/template-init.sh
   - Writes .awiki/template.json with {repo, ref, version, commit, applied_migrations, deleted, bootstrap_steps_done}.
   - Snapshots current template tree to .awiki/template-cache/<commit>/ as merge ancestor.
   - Records all currently-completed step IDs into bootstrap_steps_done[].
```

`scripts/template-init.sh` is idempotent. Re-running rebuilds cache from current pin.

### Update tool re-runs new steps

Phase 3 Commit C detects step IDs in template's BOOTSTRAP not in `bootstrap_steps_done[]`. For each: extract step body between `<!-- bootstrap-step: ID -->` markers, replay prompt + execute commands. Append ID to list on success. User can decline; recorded as `{id, skipped: true}`.

Re-run a step manually: `just bootstrap-step <id>`.

---

## Files + recipes

### New scripts

| script                          | purpose                                                                                  |
|---------------------------------|------------------------------------------------------------------------------------------|
| `scripts/template-update.sh`    | Orchestrator. Args: `--ref`, `--source`, `--apply`, `--continue`, `--abort`, `--status`, `--skip-migration NNNN`. |
| `scripts/template-init.sh`      | Seeds `.awiki/template.json` + cache. Run by BOOTSTRAP.                                  |
| `scripts/template-plan.sh`      | Pure read; emits plan as `PLAN|...` stdout. Reused by update tool + future MCP exposure. |
| `scripts/template-merge.sh`     | Wraps `git merge-file --diff3` for `three_way` strategy. Conflict detection.             |
| `scripts/template-step.sh <id>` | Replay one bootstrap step by ID.                                                         |
| `scripts/template-retrofit.sh`  | Retrofit pre-v1 repos (Phase B in rollout).                                              |

### New files in template root

- `template.manifest.toml`
- `migrations/` (empty in v1)
- `migrations/README.md` (author guide)

### Justfile additions

```just
# === template ===
template-update *args:
    bash scripts/template-update.sh {{args}}

template-status:
    bash scripts/template-update.sh --status

template-init:
    bash scripts/template-init.sh

bootstrap-step id:
    bash scripts/template-step.sh {{id}}
```

### State files

```
.awiki/
├── template.json                  # tracked
├── template-cache/                # gitignored
│   ├── <commit_old>/              # ancestor for next merge
│   └── _fetch/                    # transient during update
└── pending-prompts/               # tracked once written; cleared by agent
    └── NNNN-<slug>.md
```

### Lint additions (`scripts/lint.sh`)

- All BOOTSTRAP step blocks must have `<!-- bootstrap-step: <id> -->`.
- All IDs in `template.manifest.toml.bootstrap.ordered_steps` must exist in BOOTSTRAP.
- All migration files match `migrations/NNNN-*.sh|prompt.md` (zero-padded, slug kebab).
- Warn on `.awiki/pending-prompts/*.md` older than 14 days.
- Warn if `.awiki/template.json.commit` doesn't resolve in current `template-cache/`.

### Tests

`tests/template-update.bats` cases:

- Clean update (no conflicts, no migrations, no new steps).
- Three-way conflict (user-edited hybrid file).
- User-deleted file recorded then preserved across update.
- New file inline prompt: each of `overwrite | skip | mark-as-user-deleted`.
- Mechanical migration runs idempotently (re-run = no-op).
- LLM migration staged into `pending-prompts/`, not executed.
- Declined bootstrap step recorded as `skipped`.
- Dry-run produces no diffs in working tree.
- `--continue` after conflict resolution finishes remaining phases.
- `--abort` cleans branch + cache.
- Failed mechanical migration halts; `--continue` after fix proceeds.

CI workflow does end-to-end: bootstrap from previous tag → mutate template → run `template-update --apply` → verify diff matches expected.

---

## Edge cases + error handling

| case                              | handling                                                                                                          |
|-----------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Encrypted repo, locked            | `three_way` on encrypted file errors loudly: "File X is encrypted; manual merge required after `git-crypt unlock`." User runs from unlocked checkout. |
| Theme submodule                   | `preserve` strategy; never touched. Theme bumps = manual `git submodule update --remote`. Documented in CHANGELOG. |
| Manifest schema mismatch          | New template ships `schema_version=2` while pin is `1`. Tool refuses; points at `migrations/SCHEMA-v1-to-v2.md`; runs via `--schema-upgrade` flag. |
| Diverged user fork                | `--source` accepts fork. Treated as new template. User responsible for fork drift.                                |
| Repo not bootstrapped             | No `.awiki/template.json`. `template-update` errors: "Run `just template-init` first."                            |
| Aborted update                    | `--abort` deletes branch, cleans `_fetch`. `.awiki/template.json` untouched.                                      |
| Stale `_fetch` from prior crash   | Detected at fetch start. Asks "Reuse or refetch?" — default refetch.                                              |
| Partial migration failure         | Halt at Commit B. `applied_migrations[]` includes all up to N. User fixes N+1 source or `--skip-migration NNNN`.  |
| Dirty working tree                | Phase 0 halts. No `--force`. User stashes or commits.                                                             |
| Concurrent update branches        | Existing `awiki-template-update/<other-sha>` halts: "Resolve or `--abort` first."                                 |
| Hugo theme version skew           | New shortcode requires newer theme. Build fails. Migration prompt staged: "Bump theme submodule, verify build."   |
| Migration touches `.awiki/`       | Linted: migration runner rejects scripts that write inside `.awiki/`. Schema migrations are blessed exception (run via `--schema-upgrade`). |

---

## Rollout in awiki template

### Phase A — Land mechanism (template-side)

Add manifest, scripts, recipes, lint rules, BOOTSTRAP step IDs. Tag awiki release `v1.0.0`. This is the floor version: only repos bootstrapped from `>=v1.0.0` get clean update path.

### Phase B — Retrofit script for pre-v1 repos

`scripts/template-retrofit.sh` ships in v1.0.0. Detects bootstrapped repo without `.awiki/template.json`, asks user for awiki commit/version their repo was cloned from, runs `template-init.sh` against that ref. Backfills `bootstrap_steps_done[]` from heuristics:

- `mcp/awiki-server/` config present in agent harness → `wire-awiki-mcp` done.
- `git-crypt status` clean → `privacy` done.
- `.awiki/qmd-status=ok` → `install-qmd` done.

Heuristics conservative — when uncertain, prompt user. Document in `docs/template-update.md` migration guide.

### Phase C — Self-test

`examples/sample-wiki/` as fixture. CI runs: bootstrap from v1.0.0 → mutate template → run `template-update --apply` → verify diff matches expected. BATS tests cover unit behavior; CI workflow does end-to-end.

### Phase D — MCP exposure (post-v1)

Add `template_status`, `template_plan`, `template_apply` tools to awiki MCP server. Lets agent surface "your template is N versions behind, here's the plan" without user invoking CLI. Deferred until v1.x — CLI-first.

### Phase E — Migration ergonomics

After 5+ real migrations shipped, evaluate need for: parallel migration execution, migration test harness, dry-run report formatter. YAGNI until proven.

### Doc updates

- `README.md` — mention update story.
- `WIKI.md` — agent-facing workflow for `pending-prompts/`.
- `BOOTSTRAP.md` — new `template-init` step + per-step IDs.
- `docs/template-update.md` — full user guide.
- `docs/decisions/template-update.md` — ADR summarizing this spec.

---

## Open questions

None at this time. Migration ergonomics (Phase E) deliberately deferred.

## Implementation phasing summary

1. Manifest schema + parser.
2. `template-init.sh` + `.awiki/template.json` writer + cache seeding.
3. `template-plan.sh` (pure read, structured stdout).
4. `template-merge.sh` wrapper around `git merge-file`.
5. `template-update.sh` orchestrator: phases 0–1 (preflight + fetch).
6. `template-update.sh` Phase 2 (plan emit).
7. `template-update.sh` Phase 3 Commit A (sync).
8. `template-update.sh` Phase 3 Commit B (mechanical migrations + LLM staging).
9. `template-update.sh` Phase 3 Commit C (bootstrap steps with marker extraction).
10. `template-update.sh` Phase 3 Commit D (provenance) + `--continue` / `--abort`.
11. BOOTSTRAP.md step ID comments + `template-init` step.
12. Lint additions.
13. `template-retrofit.sh` for pre-v1 repos.
14. BATS tests + CI end-to-end.
15. WIKI.md workflow update for pending-prompts handling.
16. Docs (`docs/template-update.md`, ADR, README mention, CHANGELOG entry).

Detailed plan in companion `docs/superpowers/plans/` after spec approval.
