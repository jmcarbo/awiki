# Template Update Mechanism — Design

**Date:** 2026-04-27
**Status:** Round 2.5 — applied second-pass review findings (1 Critical, 10 Important, 10 Minor)
**Type:** Feature on awiki template repository
**Depends on:** [LLM Wiki Scaffold](2026-04-27-llm-wiki-scaffold-design.md)

## Summary

Mechanism for repositories that have already bootstrapped from the awiki template to pull subsequent updates (new scripts, MCP server fixes, deploy templates, schema changes, new bootstrap steps) without losing per-domain customizations. Detached-template + manifest model: the bootstrapped repo records template provenance in `.awiki/template.json`, and `just template-update` fetches a fresh template, syncs by per-path strategy, runs versioned migrations, prompts for new bootstrap steps, and writes everything onto a dedicated update branch for review.

## Goals

- **Detached.** No shared git history between template and bootstrapped repos. Privacy-safe for encrypted wikis.
- **Manifest-driven.** Per-path strategy declared in `template.manifest.toml` shipping with template.
- **Three-way merge for hybrid files** via `git merge-file` with cached ancestor.
- **Versioned migrations.** Mechanical (`*.sh`) + LLM-assisted (`*.prompt.md`); idempotent; tracked in `applied_migrations[]`.
- **Re-runnable bootstrap steps.** New steps detected by ID and replayed.
- **Auditable.** Default = dry-run plan. `--apply` writes to dedicated branch, one commit per phase.
- **Bash-first.** No external toolchain. Reuses git's merge engine.
- **Trust-explicit.** Source pinned at bootstrap; source changes require user confirmation.
- **Recovery-first.** Explicit state file for `--continue`, `--re-pin` escape hatch for bad updates.

## Non-goals

- Hosted update service.
- Cross-template migrations (from a different scaffold).
- Auto-merge of conflicts.
- Theme submodule version management (preserved; user upgrades manually).
- Backward-merge from bootstrapped repos into template (one-way only).
- Sandboxed migration execution. Migrations are trusted code; users are responsible for source review (see Trust Model).

## Decisions (from brainstorm)

- **Distribution:** detached + manifest. Not git-remote, not copier/cruft.
- **Versioning:** SHA-pinned, optional human-readable tag.
- **Hybrid file strategy:** 3-way merge via `git merge-file` with cached ancestor.
- **Migrations:** mechanical bash + optional LLM prompt files. Both numbered.
- **Fetch source:** URL or local path in `.awiki/template.json`.
- **User-deleted files:** tracked in `.awiki/template.json.deleted[]`.
- **New bootstrap steps:** detected by step ID; re-prompted on update.
- **Safety:** dry-run by default; `--apply` runs on dedicated branch with per-phase commits.

---

## Trust Model

The update mechanism executes code shipped from the template source. This section is load-bearing — implementation MUST honor it.

### Trust assumptions

- **`migrations/NNNN-*.sh`** are trusted bash. Run with full user-shell privileges and access to repo (including unlocked ciphertext when running on an unlocked checkout).
- **`migrations/NNNN-*.prompt.md`** are trusted instructions for the agent. The agent acts on user content under their guidance.
- **Files with `overwrite` strategy** replace user-tracked content directly.

### Source identity

- `.awiki/template.json.repo` is **pinned at bootstrap** to the canonical awiki repo URL (or whatever URL `template-init` was invoked with).
- `.awiki/template.json.original_repo` is set once at bootstrap and never changes. Lets audits detect later source switches.
- `--source <url>` overrides the configured repo for one invocation.
- If the resolved `--source` differs from `.awiki/template.json.repo`, `template-update` halts with:
  ```
  Source change detected:
    pinned : <repo>
    new    : <source>
  This will execute migrations and overwrite tracked files from the new source.
  Re-run with --accept-source-change to proceed.
  ```
- `--accept-source-change` is a one-shot flag (does not persist). Optionally `--persist-source` updates `.awiki/template.json.repo` after success; otherwise pin stays.

### Optional tag-signature verification

Pinning by SHA records identity but does not verify authenticity beyond TLS to the git host. Optional defense: signed tags.

- `--verify-signature` flag invokes `git verify-tag <ref>` (or `git verify-commit <commit>` if `--ref` is a SHA) before treating the fetched tree as trusted. Fails → halt.
- Trust roots: user's `~/.gnupg/` or `git config gpg.ssh.allowedSignersFile`. Spec does not bundle keys.
- Configurable via `.awiki/config.require_signature = true` for users who want it always on.
- v1 ships the flag; defaults off. Document in `docs/template-update.md`. Authors who sign their releases get the audit trail; users opting in get one extra layer.

This is **opt-in** because the canonical awiki upstream may not sign every release in v1; making it mandatory would block legitimate updates.

### Plan-time review surface

Dry-run plan MUST emit, for each new mechanical migration:

```
PLAN|migration|<NNNN>|<filename>|<diffstat>
PLAN|migration-content|<NNNN>|<sha256-of-script>|<lines-of-bash>
```

`--print-migrations` adds full script contents to plan output (verbose; for review). For LLM prompt migrations, the prompt body is **always** printed in the plan (not behind a flag) since it's small and load-bearing.

### Migration execution constraints

- Mechanical migrations run with `set -euo pipefail` and a stripped env: `PATH`, `HOME`, `AWIKI_REPO_ROOT`, `AWIKI_TEMPLATE_OLD_VERSION`, `AWIKI_TEMPLATE_NEW_VERSION`, `LANG`, `LC_ALL` only. Auth tokens (`GH_TOKEN`, `GITHUB_TOKEN`, etc.) are unset before exec.
- Migrations MUST NOT touch `.awiki/`. Schema-upgrade migrations (see §Schema upgrades) are the only blessed exception, gated by `--schema-upgrade`.
- LLM prompt `scope_glob` is enforced by the runner at staging time:
  - Glob MUST be present and non-empty.
  - Glob MUST NOT match any path under `secrets/`, `.git/`, `.awiki/`, `themes/`, or any `.gitattributes`-encrypted path that would be in plaintext only on an unlocked checkout.
  - Runner pre-resolves the glob and writes the resolved file list inside the staged prompt; agent is told to operate on that list, not re-evaluate the glob.

### Pending-prompt review gate

Before the agent acts on a pending prompt, the WIKI.md workflow REQUIRES a user review step:

> Surface the prompt body to the user. Confirm before running. If user declines, append `{id, status: "skipped", reason}` to `applied_migrations[]` and delete the file.

Lint warns if `pending-prompts/*.md` shows agent-modified timestamps without a corresponding `applied_migrations[]` entry (signals an undocumented run).

### Threat model summary

| Threat                                       | Mitigation                                                              |
|----------------------------------------------|-------------------------------------------------------------------------|
| Malicious `--source` URL                     | Source-change confirmation; pinned `original_repo`                      |
| Compromised upstream migrating against repo  | Plan surfaces script content + sha; `--print-migrations` for full body  |
| Migration exfiltrates secrets                | Stripped env removes auth tokens; `touches:` header enforced post-run; users review with `--print-migrations`. **Asymmetry note:** mechanical migrations have no scope_glob equivalent — `touches:` enforcement (below) is the bound. |
| Bash migration writes outside declared scope | `touches:` header parsed and enforced: post-run, `git status --porcelain` MUST report only paths matching the declared globs (plus `.awiki/` ban). Halt + revert otherwise. |
| LLM prompt rewrites secrets                  | Scope-glob enforced at runner; secrets/, .awiki/, themes/, .git/ blocklisted |
| Silent encryption-pattern flip               | `.gitattributes` uses `attributes_merge` strategy with explicit confirm |
| Tag/commit forgery on git host               | Optional `--verify-signature` (or `require_signature = true`) checks `git verify-tag` / `git verify-commit` |

The mechanism is **not** a security boundary against the template author. Users running templates they don't trust should not run this.

---

## Architecture

Five components in the bootstrapped repo:

1. **`.awiki/template.json`** — pinned provenance. Tracked in git.
2. **Template cache** — `.awiki/template-cache/<sha>/` holds last-applied template tree (merge ancestor) + `_fetch/` transient working dir + `_fetch/.update-state.json` orchestrator state. Gitignored.
3. **`template.manifest.toml`** — at template root. Per-path strategy + BOOTSTRAP step ordering.
4. **`scripts/template-update.sh`** — orchestrator. Phases preflight → fetch → plan → apply (sync, migrations, bootstrap-steps, provenance). Each phase = one commit on update branch. State persisted between phases.
5. **`migrations/NNNN-*.sh|.prompt.md`** — versioned. Numbering monotonic, never renumbered.

Layer boundaries: fetch is pure git ops (writes only to `.awiki/template-cache/_fetch/`), plan is pure read+test-merge in scratch dir, apply mutates only on `awiki-template-update/<sha>` branch.

---

## `.awiki/template.json` schema

```json
{
  "schema_version": 1,
  "repo": "https://github.com/jmcarbo/awiki",
  "original_repo": "https://github.com/jmcarbo/awiki",
  "ref": "main",
  "version": "1.0.0",
  "commit": "abc123def456...",
  "applied_migrations": [
    { "id": "0001-add-frontmatter-field", "status": "applied" },
    { "id": "0002-rename-folder", "status": "applied" },
    { "id": "0003-cleanup", "status": "skipped", "reason": "user --skip-migration" },
    { "id": "schema-1-to-2", "status": "applied" },
    { "id": "0007-rewrite-sources-blocks", "status": "applied" }
  ],
  "deleted": [
    { "path": "scripts/ingest-audio.sh", "reason": "no audio sources" }
  ],
  "bootstrap_steps_done": [
    { "id": "domain", "status": "applied", "content_hash": "sha256:abc..." },
    { "id": "wire-qmd-mcp", "status": "skipped", "reason": "user declined" }
  ]
}
```

Tracked in git so update history is auditable. Rationale for new fields:

- **`original_repo`** — tamper-evident pin set at bootstrap; never auto-modified. Lets audits flag later switches.
- **`bootstrap_steps_done[].content_hash`** — sha256 of the step body (text between `<!-- bootstrap-step: ID -->` and the next step marker). On update, if the upstream content_hash changes, the step is treated as new (re-prompted) — see §Content-hash tracking and §Dangerous steps for the full replay rules.
- **`bootstrap_steps_done[].status`** — `"applied"` | `"skipped"`. Replaces earlier `skipped: bool` for consistency with `applied_migrations[]`.
- **`applied_migrations[].id`** — full slug from migration filename (e.g., `0007-rewrite-sources-blocks` for `migrations/0007-rewrite-sources-blocks.sh` or `.prompt.md`). Schema upgrades use `schema-N-to-M` (no zero-padding). Applies uniformly to mechanical, LLM, and schema-upgrade migrations.

All `template.json` writes happen in **Commit D only** (single source of truth at end of cycle). Earlier phases stage data in the orchestrator state file (`.awiki/template-cache/_fetch/.update-state.json`); Commit D reads state and writes once.

---

## Manifest schema

`template.manifest.toml` at template root:

```toml
schema_version = 1
template_version = "1.0.0"

[strategies]
overwrite        = ["scripts/**", "layouts/**", "mcp/awiki-server/**", "deploy/**", "scheduled/**", "BOOTSTRAP.md", "README.md", "justfile", "tests/**"]
preserve         = ["content/**", "raw/**", ".obsidian/workspace*", "secrets/**", "themes/**"]
three_way        = ["WIKI.md", "hugo.toml", "content/_index.md", "content/log.md", ".gitignore", "CLAUDE.md", "AGENTS.md"]
attributes_merge = [".gitattributes"]
template_only    = ["template.manifest.toml", "migrations/**"]

[new_file_default]
strategy = "prompt"   # overwrite | skip | mark-as-user-deleted

[bootstrap]
ordered_steps = [
  "dep-check", "domain", "wiki-name", "privacy", "track-processed",
  "theme", "publish-log", "patch-identity", "install-qmd",
  "wire-qmd-mcp", "wire-awiki-mcp", "log-init", "stage-commit", "smoke-test"
]

[bootstrap.dangerous]
# Steps that mutate non-template state and MUST NOT auto-replay.
# Update tool detects new versions but only runs them on --rerun-bootstrap-step <id>.
ids = ["theme", "wire-qmd-mcp", "wire-awiki-mcp", "install-qmd"]
```

### Strategy semantics

| strategy           | behavior                                                                                            |
|--------------------|-----------------------------------------------------------------------------------------------------|
| `overwrite`        | Template wins. User edits lost.                                                                     |
| `preserve`         | User wins. Template version ignored.                                                                |
| `three_way`        | `git merge-file --diff3` with cached ancestor. Conflicts emit `<<<<<<<` markers.                    |
| `attributes_merge` | Like `three_way` but always blocks for explicit user confirm; runs `git check-attr` audit (below).  |
| `template_only`    | Same as overwrite, but emitted as `PLAN|template_only|...` (silent in human-readable summary, structured stdout still surfaces the diff for transparency). |

### Glob precedence

Multiple strategies may declare overlapping globs (e.g., `content/**` preserves but `content/log.md` is three_way). Resolution rule:

1. **Most-specific glob wins.** Specificity computed in this order, descending:
   1. Longer literal prefix before any `*`.
   2. More path segments (separators).
   3. No `**` beats has-`**`.
   4. Lexicographic order of the glob string (final tie-break, deterministic).
2. **Strategy precedence on cross-strategy ties** (same specificity, different strategy): `template_only` > `attributes_merge` > `three_way` > `overwrite` > `preserve` > `new_file_default`.
3. **Same-strategy ties** (same specificity, same strategy): later-declared in the manifest wins. Authors should avoid this — lint warns on identical globs in the same strategy list.

Example resolutions:
- `content/log.md` matches both `content/**` (preserve) and `content/log.md` (three_way) → `three_way` (longer literal prefix).
- `secrets/key.age` matches `secrets/**` only → `preserve`.
- `scripts/foo.sh` matches `scripts/**` only → `overwrite`.

`scripts/template-plan.sh` MUST emit the resolved strategy per touched path; if a path matches no glob, it falls under `new_file_default`.

### `attributes_merge` strategy (`.gitattributes` and similar)

`.gitattributes` requires special handling because git-crypt evaluates patterns at checkout time — a naive merge can silently flip files between cleartext and ciphertext.

Apply rules for `attributes_merge`:

1. Run `git merge-file --diff3` against ancestor. If conflicts, halt as in `three_way`.
2. After clean merge, run `git check-attr -a -- <every-tracked-path>` against both the OLD and NEW file. Diff the attribute sets.
3. For each path whose `filter` (or `diff`) attribute changed:
   - Emit `PLAN|attribute-change|<path>|<old-attrs>|<new-attrs>`.
   - If any change adds or removes `filter=git-crypt`, halt apply with: "Encryption pattern change detected on N file(s). Re-run with `--accept-attribute-changes` to commit Sync (Commit A)."
4. `--accept-attribute-changes` confirms once; not persisted.

`.gitattributes` is the only file in `attributes_merge` in v1; the strategy exists for future similar files (e.g., `.gitignore` if encryption patterns ever moved there — currently not).

### Conflict path

If `git merge-file` returns nonzero on any `three_way` or `attributes_merge` path, file left with markers; plan summary lists conflicts; user resolves before apply finalizes via `--continue`.

Pre-Commit-A check: `grep -lE '^<<<<<<< ' -- $(echo $three_way_paths $attributes_merge_paths)`. Restricts to the actual merged paths to avoid false positives on wiki content that legitimately documents conflict markers.

---

## Update flow

CLI:

```
just template-update [
  --ref <sha-or-tag>
  --source <url-or-path>
  --apply
  --dry-run
  --continue
  --abort
  --status
  --skip-migration <id>     # full slug, e.g., 0007-rewrite-sources-blocks
  --schema-upgrade
  --accept-source-change
  --accept-attribute-changes
  --accept-manual-commits
  --persist-source
  --rerun-bootstrap-step <id>
  --print-migrations
  --re-pin <commit>
  --gc
  --non-interactive
  --verify-signature
]
```

Default = dry-run plan. `--apply` mutates.

### Flag interactions

- `--apply --dry-run` → `--dry-run` wins (no mutation). Documented behavior. Default (no flag) = dry-run.
- `--continue` without prior failure / state file → halt: "No update in progress."
- `--accept-manual-commits` → bypasses the "branch HEAD ≠ last_completed_commit" halt during `--continue`. Use after manually editing files / committing on the update branch.
- `--non-interactive` → all prompts auto-resolve to safe defaults (see §Non-interactive defaults). Required for CI.
- `--re-pin` is mutually exclusive with `--apply`, `--continue`. Operates on `.awiki/template.json` only; see §Recovery from a bad update.
- `--gc` cleans orphaned cache dirs without running an update.
- `--verify-signature` is independent of all phases; verification runs at fetch time, halts early on failure.

### Phase 0 — preflight

Phase 0 splits into two sub-phases: **0a** runs before fetch (uses only local repo state); **0b** runs after fetch (needs new manifest).

**Phase 0a — pre-fetch preflight (no network, no manifest needed):**
- **Dirty working tree definition.** Halt if any of:
  - `git diff-index --quiet HEAD --` returns nonzero (staged or unstaged tracked changes).
  - `git ls-files --others --exclude-standard` is non-empty (untracked files NOT ignored by `.gitignore`).
  - Submodule (`themes/*`) has uncommitted changes (`git submodule status` shows `+` or `-`).
- **Branch check.** Verify on main (or the branch named in `.awiki/config.default_branch`, defaulting to `main`). Halt if on `awiki-template-update/*` (use `--continue` instead).
- **Encryption preflight.** Run `git-crypt status -e 2>/dev/null || true` (requires git-crypt ≥ 0.6; older versions fall back to `git-crypt status` parsing). If output non-empty AND any encrypted path matches a `three_way` or `attributes_merge` glob in the OLD manifest, run `git-crypt status` to verify lock state. If locked, halt: "Encrypted paths X, Y, Z would require merge. Run `git-crypt unlock` first."
- **Pending prompts gate.** If `.awiki/pending-prompts/*.md` is non-empty AND not invoked with `--continue`, halt: "Pending LLM migrations from previous cycle: <list>. Run agent to complete or delete prompts before next update." (Note: `--continue` resuming a cycle that staged prompts itself is allowed — the gate applies only when starting a NEW cycle.)
- **Read pin.** Load `.awiki/template.json` → `{repo, original_repo, commit_old, schema_version}`.
- **Source-change check.** If `--source` differs from `repo`, halt unless `--accept-source-change`.

**Phase 0b — post-fetch preflight (runs immediately after Phase 1):**
- **Schema-version check.** Read `_fetch/template.manifest.toml.schema_version`. If newer than pin's, run §Schema upgrades flow before continuing. Halt unless `--schema-upgrade`.
- **Encryption recheck.** Re-run encryption preflight against the NEW manifest's `three_way` and `attributes_merge` globs. New paths covering encrypted files must also be on an unlocked checkout.
- **Signature verification (if `--verify-signature` or `require_signature = true`):** Run `git verify-tag <ref>` (tag) or `git verify-commit <commit>` (SHA). Halt on failure.

### Phase 1 — fetch

- Resolve source URL. Default: `repo` from pin. Override: `--source`.
- `git clone --depth 50 <source> .awiki/template-cache/_fetch` (or `git fetch` if `_fetch/` already a clone). Stale `_fetch/` from prior crash detected via `_fetch/.update-state.json` presence; either resumed (`--continue`) or pruned + re-cloned.
- Resolve `--ref` → `commit_new`. Default = `origin/main` HEAD.
- If `commit_new == commit_old`, exit 0 with "already up to date".
- **Ancestor cache check.** `.awiki/template-cache/<commit_old>/` MUST exist for three-way merges. Recovery path:
  - If missing AND `original_repo` resolvable, **auto-recover**: `git clone --depth 50 <repo> tmp/ && git -C tmp checkout <commit_old> && cp -R tmp/. .awiki/template-cache/<commit_old>/ && rm -rf tmp/`. Print "Re-built ancestor cache from pin."
  - If `original_repo` unreachable (offline, repo deleted), halt with retrofit instruction: "Run `just template-retrofit` to re-seed ancestor."
- Write initial state file `_fetch/.update-state.json`:
  ```json
  {
    "phase": "fetch",
    "status": "completed",
    "commit_old": "abc123...",
    "commit_new": "def456...",
    "branch": "awiki-template-update/def456",
    "started_at": "2026-04-27T15:30:00Z",
    "last_completed_commit": null,
    "applied_migrations_pending": [
      { "id": "0007-rewrite-sources-blocks", "status": "applied" }
    ],
    "bootstrap_steps_pending": [
      { "id": "domain", "status": "applied", "content_hash": "sha256:abc..." }
    ],
    "deletions_user_decisions": {
      "scripts/old-helper.sh": "preserve-local"
    }
  }
  ```

  Field shapes:
  - `applied_migrations_pending[]` items match `applied_migrations[]` in `template.json` (id, status, optional reason).
  - `bootstrap_steps_pending[]` items match `bootstrap_steps_done[]` in `template.json` (id, status, optional reason, content_hash).
  - `deletions_user_decisions` is `{path: "remove" | "preserve-local"}`.

### Phase 1.5 — schema upgrade (conditional)

Runs only if Phase 0b detected `schema_version` mismatch AND `--schema-upgrade` was passed.

- **Branch creation happens here**, not later in Phase 3. Branch name `awiki-template-update/<short-commit_new>` (same as regular update). Schema upgrade lands as **Commit 0** on this branch.
- Run `migrations/schema-NN-to-MM.sh` (read from `_fetch/migrations/`) with stripped env, `.awiki/` writes whitelisted only for the duration of this script.
- Stage all changes (under `.awiki/` and elsewhere); commit `chore(template): schema upgrade <NN> → <MM>`.
- Update state file `phase: schema-upgrade`, `status: committed`, `last_completed_commit: <sha>`.
- After commit, re-evaluate Phase 0b checks (schema_version now matches).
- If `--schema-upgrade` was the only action requested (no `--apply`), exit 0 here. The orchestrator normally transitions to Phase 2 plan after schema upgrade, but with no `--apply`, plan emits and exits as usual.

The schema upgrade lives on the update branch like any other commit, preserving the "apply mutates only on `awiki-template-update/<sha>` branch" invariant. If the user `--abort`s, schema upgrade is also discarded.

### Phase 2 — plan (always; dry-run prints + exits)

Plan is **read-only** at the user's repo level. Test-merges happen in a scratch dir (`.awiki/template-cache/_fetch/_scratch-merge/`) which is wiped at end of plan.

Diff `commit_old..commit_new` paths against NEW manifest. Categorize per resolved strategy (§Glob precedence). Categories emitted as structured stdout (formal contract below):

- `overwrites[]` — listed.
- `preserves[]` — no-op, listed.
- `three_way[]` — predict conflicts via test-merge in `_scratch-merge/`. Emit `PLAN|three_way|<path>|conflict-predicted|<count>` or `clean`.
- `attributes_merge[]` — same prediction + attribute diff preview.
- `template_only[]` — diff emitted under `PLAN|template_only|<path>|<note>` (suppressed from human summary; present in structured stdout).
- `new_files[]` — prompt strategy on apply.
- `deletions_in_template[]` — would be removed unless locally modified.
- `user_deleted[]` — confirm skip per `.awiki/template.json.deleted[]`.

Migrations: scan `migrations/` for files with index > max(`applied_migrations`). For each, emit:

```
PLAN|migration|<NNNN>|<filename>|<diffstat>
PLAN|migration-content|<NNNN>|sha256:<hash>|<lines>
```

If `--print-migrations`, follow with full file body framed by `BEGIN-MIGRATION-CONTENT|<NNNN>` … `END-MIGRATION-CONTENT|<NNNN>`.

LLM prompt migrations: full body printed in plan unconditionally.

Bootstrap steps: emit step IDs not in `bootstrap_steps_done[]` OR whose upstream `content_hash` differs from recorded:

```
PLAN|bootstrap-step-new|<id>
PLAN|bootstrap-step-content-changed|<id>|<old-hash>|<new-hash>
PLAN|bootstrap-step-dangerous|<id>|<reason-enum>
```

For dangerous steps (`bootstrap.dangerous.ids`), the plan emits `bootstrap-step-dangerous` and apply does NOT run them automatically. User runs `just template-update --rerun-bootstrap-step <id>` separately.

### Plan output format (formal contract)

All lines pipe-separated. The line-type field (column 2) determines arity. Each line type has fixed column count.

**Escape rules:**
- Pipe `|` inside any field → `%7C`.
- Newline inside any field → `%0A`.
- Percent `%` → `%25`.
- All other bytes literal. UTF-8 throughout.

**Line printing order** (consumers can rely on it):
1. `header` (exactly one).
2. Sync category lines: `overwrite`, `preserve`, `three_way`, `attributes_merge`, `attribute-change`, `template_only`, `new_file`, `deletion-in-template`, `user-deleted` — interleaved alphabetically by `<path>`.
3. Migration lines: `migration` + `migration-content` (paired) per `NNNN`, ascending. Then `migration-prompt` per `NNNN`, ascending. Optional `migration-body` blocks if `--print-migrations`.
4. Bootstrap step lines: one per step ID (different shapes per event).
5. Prompt lines (deferred prompts that apply will surface).
6. `footer` (exactly one).

**Per-event line shapes:**

| line type                       | columns (`PLAN|<type>|...`)                                                       |
|---------------------------------|-----------------------------------------------------------------------------------|
| `header`                        | `<schema-version>|<commit_old>|<commit_new>`                                      |
| `overwrite`                     | `<path>|<note>`                                                                   |
| `preserve`                      | `<path>|<note>`                                                                   |
| `three_way`                     | `<path>|<status>|<conflict-count>` (count empty if status=clean)                  |
| `attributes_merge`              | `<path>|<status>|<conflict-count>`                                                |
| `attribute-change`              | `<path>|<old-attrs>|<new-attrs>`                                                  |
| `template_only`                 | `<path>|<note>`                                                                   |
| `new_file`                      | `<path>|<default-strategy>`                                                       |
| `deletion-in-template`          | `<path>|<locally-modified-bool>`                                                  |
| `user-deleted`                  | `<path>|<recorded-reason>`                                                        |
| `migration`                     | `<NNNN>|<filename>|<diffstat>`                                                    |
| `migration-content`             | `<NNNN>|sha256:<hash>|<lines>`                                                    |
| `migration-prompt`              | `<NNNN>|<filename>|<scope-glob>|<resolved-file-count>|<risk>`                     |
| `migration-body`                | `<NNNN>|begin` then raw body lines (escape rules apply) then `<NNNN>|end`         |
| `bootstrap-step-new`            | `<id>`                                                                            |
| `bootstrap-step-content-changed`| `<id>|<old-hash>|<new-hash>`                                                      |
| `bootstrap-step-dangerous`      | `<id>|<reason-enum>` (enum: `marked-dangerous-new`, `marked-dangerous-changed`)   |
| `prompt`                        | `<phase>|<question-id>|<default-on-apply>`                                        |
| `footer`                        | `errors=<n>|warnings=<n>|prompts=<n>|conflicts=<n>` (kv form for footer only)     |

`<status>` enum: `clean | conflict-predicted | conflict | error`.

Three explicit rows for `bootstrap-step-*` replace the earlier `bootstrap-step|<event>|<extra-cols>` placeholder. Each row has fixed arity.

Footer is the only line using `key=value`; it's a summary aggregation, not a record. Consumers can detect via the literal `footer` line type.

### Phase 3 — apply (only with `--apply`)

Branch: `awiki-template-update/<short-commit_new>`. Halt if exists.

State file maintained in `.awiki/template-cache/_fetch/.update-state.json` throughout. Each phase begins by writing `phase: <name>, status: started, last_completed_commit: <sha>` and ends by writing `status: committed, last_completed_commit: <new-sha>`.

**Commit A — sync:**
- Apply `overwrite`, `three_way` (merge-file), `attributes_merge` (merge-file + check-attr), prompted `new_files`, `template_only`.
- Process `deletions_in_template[]` (remove unless locally modified — if modified, prompt; record decision in state file).
- Skip `user_deleted[]` per recorded list.
- `attributes_merge` blocks for `--accept-attribute-changes` if filter changes detected.
- Stage + commit `chore(template): sync to <version> (<short-sha>)`.

**Halt for conflicts:** any `<<<<<<<` markers in `three_way` or `attributes_merge` paths. User resolves, runs `--continue`.

**Commit B — migrations (mechanical only):**
- Run mechanical (`*.sh`) in numeric order. For each: `--dry-run` first (audit log only), then real run with stripped env (§Trust Model).
- LLM migrations (`*.prompt.md`) staged into `.awiki/pending-prompts/<NNNN>-<slug>.md` with resolved file list appended; agent acts later.
- Migration outputs staged. **`template.json` is NOT updated here** — pending entries kept in state file.
- Commit `chore(template): run migrations N..M (mechanical) + stage M..K (prompts)`.

**Commit C — bootstrap-steps:**
- For each new step ID NOT in `bootstrap.dangerous.ids`, prompt user inline (extract body between markers from `_fetch/BOOTSTRAP.md`); run associated commands; pending entry in state file.
- Dangerous step IDs are skipped here. `--rerun-bootstrap-step <id>` runs one explicitly (separate invocation, separate commit).
- Commit `chore(template): bootstrap steps <ids>`.

**Commit D — provenance (single template.json write):**
- Read state file. Apply pending updates to `.awiki/template.json`:
  - `commit`, `version`, `ref`.
  - Append `applied_migrations[]` entries from state.
  - Append `bootstrap_steps_done[]` entries from state.
  - If `--persist-source`, update `repo` (never `original_repo`).
- Move `.awiki/template-cache/_fetch` → `.awiki/template-cache/<commit_new>/`.
- **Cache retention:** keep exactly two `<sha>/` dirs — `<commit_new>` (just installed) and the previous pin. All older are deleted. Selection is by mtime descending (most recently installed kept). `_check-stamp` and `_fetch/` are not `<sha>/` dirs and are unaffected.
- Delete state file.
- Commit `chore(template): pin to <version>`.

Print: "Update branch ready. Review with `git diff main`. Merge: `git switch main && git merge --no-ff awiki-template-update/<sha>`. Pending LLM migrations: `.awiki/pending-prompts/`."

### `--continue` / `--abort`

- `--continue` reads state file at `_fetch/.update-state.json`. Validates branch HEAD == `last_completed_commit`. If user committed manually since last orchestrator commit, halt: "Manual commits detected on update branch. Re-run with `--accept-manual-commits`." Resumes at `phase` after `last_completed_commit`.
- `--abort`: `git checkout main && git branch -D awiki-template-update/<sha>` (force fine — branch is local + ephemeral). Removes `_fetch/`. `.awiki/template.json` untouched.
- If killed mid-phase (signal/crash): state file shows `status: started`. `--continue` re-runs the phase from scratch (each phase's commit is built atomically; partial work in working tree is discarded via `git reset --hard <last_completed_commit>` before retry).

### Non-interactive defaults

`--non-interactive` resolves prompts as:

| prompt                                       | default                |
|----------------------------------------------|------------------------|
| `new_file_default` decision                  | `skip`                 |
| Locally-modified file deleted in template    | `preserve-local`       |
| Bootstrap step                               | `decline` (skipped)    |
| Pending prompt with `risk: high`             | `decline` (skipped, recorded with `reason: "non-interactive high-risk"`) |
| Manual commits on update branch              | halt (requires `--accept-manual-commits`) |
| Source change                                | halt (requires explicit flag)  |
| Attribute change                             | halt (requires explicit flag)  |
| Schema upgrade                               | halt (requires explicit flag)  |

Used by CI E2E test (Phase C self-test).

---

## Recovery from a bad update

Once `awiki-template-update/<sha>` is merged to main, rolling back requires care because `.awiki/template.json` claims the new pin.

### Soft revert (keep new pin)

If a single migration introduced a regression but most of the update is fine:
1. `git revert <merge-commit>` to undo the merge.
2. Run `just template-update --skip-migration NNNN --apply` to retry without the bad migration.
3. Open issue against template repo.

### Hard re-pin (downgrade)

If the entire update should be undone:

1. `git revert -m 1 <merge-commit>` (reverts merge while keeping main's first parent).
2. `just template-update --re-pin <previous-commit>` — sets `.awiki/template.json.commit` back, rebuilds `.awiki/template-cache/<previous-commit>/` from upstream, drops the now-orphaned `<commit_new>/` cache.
3. Commit the `template.json` change: `chore(template): re-pin to <previous-version> (rolled back)`.

### `--re-pin <commit>` semantics

- Refuses if any `pending-prompts/*.md` exist (state inconsistent).
- Refuses if state file present (`--abort` first).
- Validates `<commit>` resolvable in upstream `repo`.
- Mutates only `.awiki/template.json` and `.awiki/template-cache/`. Does NOT modify tracked content. (Reverting tracked content is the user's git operation, separate.)
- Prints reminder: "If you reverted the merge, content is back to pre-update state. If you didn't revert and only re-pinned, your tree may have new content with an old pin recorded — you probably want to revert too."

### Update-availability notification

Soft notification surfaced via lint:
- `scripts/lint.sh` checks `.awiki/template-cache/_check-stamp` mtime; if older than 7 days OR missing, runs `git ls-remote <repo> <ref>` (cached for 1h) and compares to pinned `commit`. If new commits available, lint emits:
  ```
  LINT|info|template|<n> template updates available since <pin>. Run `just template-status` to review.
  ```
- Suppress with `AWIKI_NO_TEMPLATE_CHECK=1` env or `.awiki/config.no_template_check = true`.
- `--status` extends with CHANGELOG slice: greps upstream `CHANGELOG.md` (if exists) for entries between pinned `commit` and `--ref`, or falls back to `git log --oneline commit_old..commit_new`.

---

## Migrations

Two kinds. Numbered sequence shared. Numbering: four-digit zero-padded, monotonic, never renumbered. Gaps OK.

**Migrations are strictly linear.** Authors MUST NOT ship a migration that depends on a skippable predecessor. If `0008` requires `0007`'s effect, document it in `0008`'s header and have it self-detect missing precondition (exit nonzero with clear error). Skipped migrations cannot be retroactively un-skipped automatically — user must edit `template.json`.

### Mechanical (`migrations/NNNN-<slug>.sh`)

- bash 4+, `#!/usr/bin/env bash`, `set -euo pipefail`. Same conventions as awiki scripts.
- Idempotent. Re-run = no-op.
- Required `--dry-run` flag — prints intended changes only.
- Required header comment block:
  ```bash
  # migration: 0007-rewrite-sources-blocks
  # requires: agent=false
  # touches: content/synthesis/**/*.md WIKI.md
  # idempotent: yes
  ```
  `touches` is **enforced** (see below).
- **Runner injects stripped env:** `PATH`, `HOME`, `AWIKI_REPO_ROOT`, `AWIKI_TEMPLATE_OLD_VERSION`, `AWIKI_TEMPLATE_NEW_VERSION`, `LANG`, `LC_ALL`. All other env (incl. auth tokens) unset.
- **Operates on user content.** MUST NOT touch `.awiki/`. Schema-upgrade migrations bypass via `--schema-upgrade` flag.
- **`touches:` enforcement.** Post-run, runner reads `git status --porcelain` and checks: every modified/added/deleted path matches at least one glob in the migration's `touches:` header. Any path outside declared scope = halt + revert (`git restore --source=HEAD -- <out-of-scope-paths>`) + leave migration unrecorded for retry. The `.awiki/` ban applies regardless of `touches:` (writes to `.awiki/` always fail unless `--schema-upgrade`).
- **Read-only paths blocklist.** Even with `touches:`, migrations cannot modify `secrets/`, `themes/`, `.git/`. These are unconditional. Runner halts pre-run if `touches:` glob would match these paths.
- Failure (nonzero exit) → halt update at Commit B. State file records last successful migration. User fixes upstream OR runs `--skip-migration NNNN` and `--continue`.
- Skip via `--skip-migration NNNN`: recorded as `{id, status: "skipped", reason}` in pending state; runner advances. Never auto-retried.

### LLM-assisted (`migrations/NNNN-<slug>.prompt.md`)

Markdown with required frontmatter:

```yaml
---
id: 0007-rewrite-sources-blocks
requires: [agent]
scope_glob: "content/synthesis/**/*.md"
risk: medium    # low|medium|high — see semantics below
---
```

- `scope_glob` required, validated at staging:
  - Non-empty.
  - MUST NOT match paths under `secrets/`, `.git/`, `.awiki/`, `themes/`, or any `.gitattributes`-encrypted path.
  - Resolved file list written into staged prompt; agent operates on the list, not re-evaluated glob.
- `risk` semantics:
  - `low` — staged silently. WIKI.md workflow surfaces to user before action.
  - `medium` — same as low; UX shows a yellow warning indicator.
  - `high` — auto-declined under `--non-interactive`. In interactive mode, prompts user with a stronger confirmation ("This migration is marked HIGH RISK. Type 'I have reviewed' to continue."). Used for prompts that touch many files or rewrite load-bearing structure.
- Body = instructions for agent.
- Runner stages copy into `.awiki/pending-prompts/<NNNN>-<slug>.md` with metadata block:
  ```
  ## Resolved scope
  - content/synthesis/foo.md
  - content/synthesis/bar.md
  ## Acceptance
  After completing, run: `just lint`.
  ## Trust note
  Confirm intent with user before bulk edits.
  ```
- Lint warns on aged pending prompts (>14d).

### Pending-prompts gating

- Commit D advances pin even if `pending-prompts/` is non-empty — current cycle finishes.
- Phase 0 of NEXT update halts if leftovers exist.
- Agent completion workflow: surface to user, confirm intent, edit files, run `just lint`, delete prompt file, append `{id, status: "applied"}` to `applied_migrations[]`, commit `chore(template): apply LLM migration <NNNN>`. Lifecycle is two commits: prompt added (during update merge), prompt resolved (via agent later).

---

## Schema upgrades

Manifest schema may evolve (e.g., new strategy added, new manifest field, change in plan output). Handled separately from content migrations.

### Naming + location

- Files: `migrations/schema-NN-to-MM.sh` (e.g., `schema-1-to-2.sh`).
- Numbering: matches manifest `schema_version` transitions. Not part of `NNNN-*` content sequence (does not consume those numbers).
- Detected automatically if Phase 0 sees `_fetch/template.manifest.toml.schema_version > pin schema_version`.

### Execution

- User runs `template-update --schema-upgrade`. Runs BEFORE Phase 2 plan (because plan output format may itself depend on schema version).
- Runner whitelists writes to `.awiki/` for the duration of the script. Other constraints (stripped env, no auth tokens) still apply.
- Schema-upgrade script MUST update `.awiki/template.json.schema_version` itself, in-script. No separate provenance commit; the script's own commit (`chore(template): schema upgrade <NN> → <MM>`) carries the change.
- After successful upgrade, `template-update` re-evaluates Phase 0 and continues normally (or exits if user only requested upgrade).
- Recorded in `applied_migrations[]` as `{id: "schema-1-to-2", status: "applied"}` (string ID, not zero-padded).

### Rules for authors

- Schema upgrades MUST be reversible in principle. Document the inverse in the script header.
- MUST NOT depend on a specific content state. Pure infrastructure.

---

## Bootstrap integration

### Step IDs

Each numbered BOOTSTRAP step gets an ID comment between the section header and body:

```markdown
### Step 9b. Wire awiki MCP server
<!-- bootstrap-step: wire-awiki-mcp -->
Ask: "Wire awiki wiki-ops MCP server (...)? (y/N)". On `y`, run `cd mcp/awiki-server && npm install && cd ../..` then `bash scripts/wire-awiki-mcp.sh`.
```

ID list canonical in `template.manifest.toml.bootstrap.ordered_steps`. **Lint flow:** manifest is source of truth; lint fails if any manifest ID lacks a marker in BOOTSTRAP, or if BOOTSTRAP has a marker not in manifest.

### Content-hash tracking

`bootstrap_steps_done[].content_hash` = sha256 of step body (text from `<!-- bootstrap-step: ID -->` to next `### Step` heading, normalized whitespace).

Recorded at bootstrap by `template-init.sh`. Compared on update:
- Step ID present in pin AND content_hash matches → already done, skip.
- Step ID present in pin AND content_hash changed → emit `PLAN|bootstrap-step|<id>|content-changed|<old>|<new>`. Treated like a new step (re-prompted on apply, except dangerous IDs).
- Step ID not in pin → new step. Prompted (except dangerous IDs).

### Dangerous steps

Steps in `bootstrap.dangerous.ids` (theme, wire-qmd-mcp, wire-awiki-mcp, install-qmd) are NEVER auto-replayed. Plan emits `PLAN|bootstrap-step|<id>|dangerous-skipped|<reason>`. User runs explicitly:

```
just template-update --rerun-bootstrap-step wire-awiki-mcp
```

This invocation:
1. Verifies clean tree (Phase 0a definition).
2. Halts if any `awiki-template-update/*` branch exists (resolve or `--abort` first).
3. Halts if `_fetch/.update-state.json` exists (in-progress update; resolve or `--abort` first).
4. Halts if `pending-prompts/` non-empty (state inconsistent).
5. Creates branch `awiki-template-update/rerun-<id>-<timestamp>`.
6. Runs the step (with same env stripping + source-trust rules as a regular update).
7. Updates `bootstrap_steps_done[].content_hash` to current upstream value (Commit D-equivalent: single `template.json` write at end).
8. Commits `chore(template): re-run bootstrap step <id>`.

### New BOOTSTRAP step (last, before smoke-test)

```
N. Seed template provenance.
   Run: bash scripts/template-init.sh
   - Writes .awiki/template.json with {repo, original_repo, ref, version, commit, schema_version, applied_migrations, deleted, bootstrap_steps_done}.
   - Snapshots template tree to .awiki/template-cache/<commit>/ as merge ancestor.
   - Records all currently-completed step IDs into bootstrap_steps_done[] with content_hash.
```

`scripts/template-init.sh` is idempotent. Re-running rebuilds cache from current pin.

### Manual replay

`just bootstrap-step <id>` = alias for `bash scripts/template-step.sh <id>`. Runs the step from local BOOTSTRAP.md (not upstream). Used for manual fixups.

---

## Files + recipes

### New scripts

| script                          | purpose                                                                                  |
|---------------------------------|------------------------------------------------------------------------------------------|
| `scripts/template-update.sh`    | Orchestrator. All flags listed in §Update flow.                                          |
| `scripts/template-init.sh`      | Seeds `.awiki/template.json` + cache. Run by BOOTSTRAP.                                  |
| `scripts/template-plan.sh`      | Pure read; emits formal plan format. Reused by orchestrator + future MCP exposure.       |
| `scripts/template-merge.sh`     | Wraps `git merge-file --diff3` for `three_way` + `attributes_merge` strategies.          |
| `scripts/template-step.sh <id>` | Replay one bootstrap step by ID. Aliased by `just bootstrap-step <id>`.                  |
| `scripts/template-retrofit.sh`  | Retrofit pre-v1 repos (Phase B in rollout).                                              |
| `scripts/template-attr-audit.sh`| `git check-attr` diff helper for `attributes_merge`.                                     |
| `scripts/template-source-check.sh` | Source-change confirmation helper (used by orchestrator).                             |

`--status` prints: current pin (commit + version), `original_repo`, pending-prompts list with ages, count of unapplied template commits at `--ref`, last update branch (if any), CHANGELOG slice between pinned `commit` and `--ref`. Read-only.

### New files in template root

- `template.manifest.toml`
- `migrations/` (empty in v1)
- `migrations/README.md` (author guide; explains naming, idempotency, env, scope_glob, dangerous-step rules)

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

template-retrofit:
    bash scripts/template-retrofit.sh

template-gc:
    bash scripts/template-update.sh --gc
```

### State files

```
.awiki/
├── template.json                  # tracked, written only by Commit D / template-init / --re-pin
├── template-cache/                # gitignored
│   ├── <commit_pinned>/           # ancestor for next merge
│   ├── <commit_previous>/         # kept; pruned at next Commit D
│   ├── _check-stamp               # mtime tracks last availability check
│   └── _fetch/                    # transient during update
│       ├── (template tree)
│       ├── _scratch-merge/        # plan-time scratch; wiped at end of plan
│       └── .update-state.json     # orchestrator state for --continue
├── pending-prompts/               # tracked once written; cleared by agent
│   └── NNNN-<slug>.md
└── config                         # see §.awiki/config schema below
```

### `.awiki/config` schema

Plain `KEY=value` lines, shell-sourceable. Bare values (no quotes); values containing spaces use `"..."`. Comments start with `#`. Booleans: `true`/`false`.

```
# .awiki/config
default_branch=main
no_template_check=false
require_signature=false
ingest_lint_threshold=5
```

Recognized keys (v1):

| key                    | type   | default | meaning                                                  |
|------------------------|--------|---------|----------------------------------------------------------|
| `default_branch`       | string | `main`  | Branch the orchestrator considers "main".                |
| `no_template_check`    | bool   | `false` | Suppresses update-availability lint info.                |
| `require_signature`    | bool   | `false` | Forces `--verify-signature` on every update.             |
| `ingest_lint_threshold`| int    | `5`     | (Pre-existing — used by `scripts/ingest.sh`.)            |

Unknown keys: lint warns. Parser: `awk -F= '!/^#/ && NF==2 {print $1, $2}'` or sourced via `set -a; . .awiki/config; set +a`.

### Lint additions (`scripts/lint.sh`)

- All BOOTSTRAP step blocks must have `<!-- bootstrap-step: <id> -->`.
- All IDs in `template.manifest.toml.bootstrap.ordered_steps` must exist in BOOTSTRAP and vice versa (bidirectional).
- All migration files match `migrations/(NNNN-*\.(sh|prompt\.md)|schema-\d+-to-\d+\.sh)`.
- LLM prompt frontmatter has required keys (`id`, `requires`, `scope_glob`, `risk`).
- LLM prompt `risk` value in enum: `low | medium | high`.
- LLM prompt `scope_glob` does not match secrets/, .awiki/, .git/, themes/.
- Mechanical migration has required header block (`migration:`, `requires:`, `touches:`, `idempotent:`).
- Mechanical migration `touches:` does not include `secrets/`, `themes/`, `.awiki/`, `.git/` patterns.
- Warn on `.awiki/pending-prompts/*.md` older than 14 days.
- Warn on pending-prompt files whose mtime is newer than the most recent `applied_migrations[]` entry referencing the same id (signals undocumented agent run).
- Warn if `.awiki/template.json.commit` doesn't resolve in current `template-cache/`.
- Warn if `.awiki/template.json.repo != .awiki/template.json.original_repo`.
- Warn if `template-cache/_check-stamp` older than 7 days (triggers update-availability check).
- Warn if `_fetch/.update-state.json` exists outside an active `--continue` session (orphaned state).
- Warn if `_fetch/_scratch-merge/` exists (orphaned plan-time scratch — safe to delete).
- Fail if any pending-prompt body references paths outside its declared `scope_glob`.
- Warn if a manifest strategy list contains identical glob entries (same-strategy tie risk).
- Warn on unknown keys in `.awiki/config`.

### Tests

`tests/template-update.bats` cases:

**Happy path:**
- Clean update (no conflicts, no migrations, no new steps).
- Three-way conflict (user-edited hybrid file).
- User-deleted file recorded then preserved across update.
- New file inline prompt: each of `overwrite | skip | mark-as-user-deleted`.
- Mechanical migration runs idempotently (re-run = no-op).
- LLM migration staged into `pending-prompts/`, not executed.
- Declined bootstrap step recorded as `skipped`.
- Dry-run produces no diffs in working tree.
- `--continue` after conflict resolution finishes remaining phases.
- `--abort` cleans branch + cache + state file.
- Failed mechanical migration halts; `--continue` after fix proceeds.

**Security cases:**
- `--source` differs from pin → halt without `--accept-source-change`.
- `--accept-source-change` proceeds; `original_repo` unchanged.
- Mechanical migration writes to `.awiki/` → runner detects, reverts, halts.
- Migration env strip: migration that reads `GH_TOKEN` sees empty.
- LLM prompt with `scope_glob: secrets/**` → staging rejected.
- LLM prompt with body referencing `.git/` → lint fails.
- `.gitattributes` change adds `filter=git-crypt` → halt without `--accept-attribute-changes`.
- `.gitattributes` change removes encryption → audit emitted; halt.
- Mechanical migration writes outside its declared `touches:` glob → runner reverts + halts.
- Mechanical migration `touches:` includes `secrets/` → lint fails (pre-run).
- LLM prompt with `risk: high` under `--non-interactive` → auto-declined and recorded.
- Plan output with `--print-migrations` includes `migration-body` framing for each script.
- `--verify-signature` on unsigned tag → halt.
- `--verify-signature` on validly-signed tag → proceeds.
- `require_signature=true` in `.awiki/config` → forces verification even without flag.

**Recovery cases:**
- Orchestrator killed mid-Commit-A → `--continue` re-runs Commit A from clean state.
- Orchestrator killed mid-Commit-B (after migration 0007 succeeded) → `--continue` resumes at 0008.
- User commits manually on update branch → `--continue` halts without `--accept-manual-commits`.
- User commits manually on update branch + `--accept-manual-commits` → resumes successfully.
- Pin commit not in cache (multi-machine) → auto-recover from `original_repo`.
- `--re-pin <commit>` rolls back pin; cache rebuilt; tree untouched.
- `--re-pin` with pending prompts → refuses.
- `--re-pin` with `original_repo` unreachable (offline) → halt with retrofit instruction.
- `--re-pin` while update in progress (state file present) → refuses.
- `--rerun-bootstrap-step <id>` while in-progress update → refuses with halt message.
- `--rerun-bootstrap-step` on dangerous step with content_hash unchanged → no-op (with confirmation prompt).

**Edge cases:**
- Encryption preflight: locked git-crypt + three_way path → halt.
- Manifest glob precedence: `content/log.md` resolves to `three_way`, not `preserve`.
- `--non-interactive` bootstraps + updates without prompts; CI uses this.
- `--non-interactive` with source change → halts (requires explicit flag).
- Schema upgrade: pin schema=1, template schema=2 → halt; with `--schema-upgrade` runs schema migration first, then proceeds.
- Bootstrap step content_hash changed → re-prompt.
- Dangerous step (`theme`) changed → not auto-replayed; `--rerun-bootstrap-step` runs it.
- `--gc` removes any `<sha>/` cache dir other than the current pin and its immediate previous (matches Commit D retention).

CI workflow (`tests/template-update-e2e.sh`): bootstrap from previous tag → mutate template (add migration + bootstrap step + new file) → run `template-update --apply --non-interactive --print-migrations` → verify diff matches expected fixture.

---

## Edge cases + error handling

| case                              | handling                                                                                                          |
|-----------------------------------|-------------------------------------------------------------------------------------------------------------------|
| Encrypted repo, locked            | Phase 0 preflight halts: "Encrypted paths X, Y, Z would require merge. Run `git-crypt unlock` first."             |
| Theme submodule                   | `preserve` strategy; never touched. Theme bumps = manual `git submodule update --remote`. Documented in CHANGELOG. |
| Manifest schema mismatch          | Halt; user runs `--schema-upgrade`. See §Schema upgrades.                                                         |
| Diverged user fork                | Halt unless `--accept-source-change`. `original_repo` unchanged.                                                  |
| Repo not bootstrapped             | No `.awiki/template.json`. `template-update` errors: "Run `just template-init` or `just template-retrofit`."      |
| Aborted update                    | `--abort` deletes branch, removes `_fetch/`. `.awiki/template.json` untouched.                                    |
| Stale `_fetch` from prior crash   | State file presence detected. Resume via `--continue` or `--abort`. No silent reuse.                              |
| Partial migration failure         | Halt at Commit B. State records last successful. User fixes OR `--skip-migration NNNN`.                           |
| Dirty working tree                | Phase 0 halts. Definition: staged/unstaged tracked + untracked-not-gitignored + dirty submodules.                 |
| Concurrent update branches        | Existing `awiki-template-update/<other-sha>` halts: "Resolve or `--abort` first."                                 |
| Hugo theme version skew           | New shortcode requires newer theme. Build fails. Migration prompt staged: "Bump theme submodule, verify build."   |
| Migration touches `.awiki/`       | Runner detects via post-run `git status .awiki/`; reverts changes; halts.                                         |
| Migration writes outside `touches:`| Runner reads `git status --porcelain`, reverts out-of-scope paths via `git restore --source=HEAD`, halts.        |
| Schema-upgrade flow (Phase 1.5)   | Branch created at schema-upgrade time, schema migration = Commit 0; rest of cycle continues on same branch.       |
| `--continue` after schema upgrade | State file phase = `schema-upgrade`, status = `committed`. `--continue` resumes at Phase 2 plan or Phase 3 apply. |
| Update available, user offline    | Lint info suppressed if `git ls-remote` fails (network error); `_check-stamp` not updated.                       |
| Multi-machine wiki                | Ancestor cache missing on second machine → auto-rebuild from `original_repo` at pinned commit.                    |
| Template repo renamed/moved       | `git fetch` fails. User edits `.awiki/template.json.repo` manually OR uses `--source <new-url> --persist-source`. |
| User wants to fork off            | `.awiki/template.json.repo = "none"` (literal) disables update. `template-update` exits 0 with "updates disabled".|
| Update merged then bad            | See §Recovery from a bad update.                                                                                  |

---

## Rollout in awiki template

### Phase A — Land mechanism (template-side)

Add manifest, scripts, recipes, lint rules, BOOTSTRAP step IDs, content_hash tracking. Tag awiki release `v1.0.0`. Floor version: only repos bootstrapped from `>=v1.0.0` get clean update path.

### Phase B — Retrofit script for pre-v1 repos

`scripts/template-retrofit.sh` ships in v1.0.0. Detects bootstrapped repo without `.awiki/template.json`, asks user for awiki commit/version their repo was cloned from, runs `template-init.sh` against that ref. Backfills `bootstrap_steps_done[]` from heuristics, with explicit prompt for each — heuristics are best-effort:

- `mcp/awiki-server/` referenced in agent harness config → suggest `wire-awiki-mcp` done.
- `git-crypt status` shows `.gitattributes` patterns → suggest `privacy=git-crypt` done.
- `secrets/age-key.txt` exists → suggest `privacy=age` done.
- `.awiki/qmd-status=ok` → suggest `install-qmd` done.

User confirms or overrides each. Document in `docs/template-update.md`.

### Phase C — Self-test

`examples/sample-wiki/` as fixture. CI runs: bootstrap from v1.0.0 → mutate template (add migration, bootstrap step, hybrid-file change) → run `template-update --apply --non-interactive` → verify diff matches expected. BATS tests cover unit behavior; CI workflow does end-to-end.

### Phase D — MCP exposure (post-v1)

Add `template_status`, `template_plan`, `template_apply` tools to awiki MCP server. Lets agent surface "your template is N versions behind, here's the plan." Deferred until v1.x — CLI-first.

### Phase E — Migration ergonomics

After 5+ real migrations shipped, evaluate need for: parallel migration execution, migration test harness, dry-run report formatter. YAGNI until proven.

### Doc updates

- `README.md` — mention update story.
- `WIKI.md` — agent-facing workflow for `pending-prompts/` (with user-confirm gate).
- `BOOTSTRAP.md` — new `template-init` step + per-step IDs.
- `docs/template-update.md` — full user guide. Includes trust model, recovery flow, multi-machine.
- `docs/decisions/template-update.md` — ADR.
- `migrations/README.md` — author guide.

---

## Deferred decisions

(Renamed from "Open questions" — items deliberately punted to implementation or future iteration.)

1. **Plan output line ordering across implementations.** v1 fixes the ordering listed in §"Plan output format". If a future MCP consumer needs random-access, add an index footer.
2. **`--gc` retention policy.** v1 = "keep current pin + immediate previous (exactly two `<sha>/` dirs)." May add age-based (>30 days) or per-machine policy later.
3. **Bootstrap step content-hash normalization.** v1 = whitespace-normalize before sha256. May need to ignore comment-style edits later.
4. **Schema-upgrade migration recording format.** v1 = string ID `schema-N-to-M` in `applied_migrations[]`. May want separate `applied_schema_upgrades[]` later for clarity.
5. **Update-availability check cadence.** v1 = 7-day stale check via `_check-stamp`. May want exponential backoff if user keeps deferring.
6. **Migration dependency declaration.** v1 = strictly linear, authors document inline. If 5+ migrations end up with explicit deps, formalize a `requires_migration:` header.
7. **`touches:` glob granularity.** v1 = top-level glob list. May add per-action declarations (`touches.read`, `touches.write`) for stricter audits.
8. **Signed-tag bundle.** v1 = optional flag, no bundled keys. Future: ship maintainer pubkey via `.awiki/template-trust.gpg` (committed in template, verified by user once).
9. **Phase E migration ergonomics.** Evaluation criteria to be set after 5 real migrations in the wild.

---

## Implementation phasing summary

1. Manifest schema + parser (incl. glob precedence with full tie-break rules).
2. `.awiki/config` parser + writer.
3. `template-init.sh` + `.awiki/template.json` writer + cache seeding + content_hash for bootstrap steps + `original_repo` set-once.
4. `template-plan.sh` (pure read, formal `PLAN|...` format with escape rules + per-event arity, scratch-merge for conflict prediction).
5. `template-merge.sh` wrapper around `git merge-file` (covers `three_way` + `attributes_merge`).
6. `template-attr-audit.sh` for attribute-change detection.
7. `template-source-check.sh` for source-change confirmation.
8. `template-update.sh` orchestrator: Phase 0a (pre-fetch preflight) + Phase 1 (fetch, ancestor auto-recovery, optional `--verify-signature`).
9. `template-update.sh` Phase 0b (post-fetch preflight: schema check, encryption recheck, signature).
10. `template-update.sh` Phase 1.5 (schema-upgrade flow with branch creation + Commit 0).
11. `template-update.sh` Phase 2 (plan emit + scratch test-merges).
12. `template-update.sh` Phase 3 Commit A (sync, including `attributes_merge` flow + `--accept-attribute-changes` gate).
13. `template-update.sh` Phase 3 Commit B (mechanical migrations with stripped env + `touches:` enforcement + `.awiki/` audit + LLM staging with scope_glob enforcement + `risk` handling).
14. `template-update.sh` Phase 3 Commit C (bootstrap steps + content_hash diff + dangerous-step skip).
15. `template-update.sh` Phase 3 Commit D (provenance, single template.json write) + state file lifecycle.
16. `--continue` (with `--accept-manual-commits`) / `--abort` + state-file recovery.
17. `--re-pin` + recovery flow.
18. `--rerun-bootstrap-step <id>` (with full preconditions) + `--gc` + `--non-interactive`.
19. BOOTSTRAP.md step ID comments + `template-init` step.
20. Lint additions (scope_glob, risk, content_hash, original_repo drift, manifest/BOOTSTRAP parity, `.awiki/config` keys, identical-glob warnings).
21. Update-availability check (`_check-stamp`) + `git ls-remote` cached fetch.
22. `template-retrofit.sh` for pre-v1 repos.
23. BATS tests (happy + security + recovery + edge cases) — full matrix.
24. CI E2E (`template-update-e2e.sh`).
25. WIKI.md workflow update for pending-prompts handling (with user-confirm gate + risk handling).
26. Docs (`docs/template-update.md`, ADR, README mention, CHANGELOG entry, `migrations/README.md` author guide).

Detailed plan in companion `docs/superpowers/plans/` after spec approval.
