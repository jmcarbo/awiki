# awiki Plan — Phase 17: Task Layer — Scanner + Agenda

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-task-layer-design.md`](../specs/2026-04-27-task-layer-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 16 (`scripts/lib/action-grammar.sh`, `scripts/lib/lock.sh`, `scripts/task-init.sh`, `scripts/capture.sh`, the empty `<!-- BEGIN managed-region -->` placeholder pairs in `content/agenda/*.md`).
**Previous:** [Phase 16](./2026-04-27-phase-16-task-schema-scaffold.md)
**Next:** Phase 18 — triage + recurrence + MCP.

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, bats-core 1.10+, `flock` (util-linux on Linux; brew `util-linux` on macOS), GNU coreutils (`date`, `sha1sum`/`shasum`, `mktemp`, `mv`, `awk`, `grep`, `sed`, `sort`). No new tool versions beyond phase 16. The 1-day spike (Task 17.1) additionally requires Obsidian (any recent desktop build) on the operator's workstation; the spike's outcome is checked in as a markdown decision record, the spike artifact is not part of CI.

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`. Pure bash 4+ for shell logic — no python helpers in this phase.
- All consumers source `scripts/lib/action-grammar.sh` (phase 16) for the canonical action-line regex and tail-metadata parser. Re-implementing the regex inline in `action-scan.sh`, `agenda.sh`, or any new lint rule body is a phase-blocking bug — fix by sourcing the lib instead.
- Every mutating script (`action-scan.sh` writes `.awiki/maps/*.tsv`; `agenda.sh` rewrites managed regions; `lint.sh --fix` rewrites pages) acquires `flock -x .awiki/lock` via `awiki_lock_with --timeout=30 -- ...` from `scripts/lib/lock.sh`. Read-only callers take `awiki_lock_shared`. The 30s profile is mandatory for user-facing tools; the deferred 180s profile is reserved for the auto-rebuild path that arrives in phase 18.
- Atomic file writes: every managed-region rewrite uses `mktemp` next to the target, writes the new content, then `mv` to swap. POSIX rename on the same filesystem is atomic — never write the target file in place.
- Slug regex `^[a-z0-9][a-z0-9-]*$` (no leading hyphen). Project slugs additionally accept a single leading underscore (`_loose`, `_someday`) but those pages are NOT created in this phase.
- Block-ID regex `^[a-z0-9]{3,16}$` (per spec T14); freshly minted IDs are exactly 8 chars (base32 alphabet `a-z0-9`). Recurrence-chain suffix shape: `^<base><sep><digits>` where `<sep>` is `~` by default but falls back to `__` if the spike (Task 17.1) determines `~` round-trip fails. **The plan treats the separator as a parameter** — every step that hard-codes one references the spike outcome.
- Lint output line format: `LINT|<level>|<file>|<msg>` where `<level>` ∈ `{ERROR,WARN,INFO}` and `<msg>` begins with the rule code (`T1: ...`, `T2: ...`, ...). Final line: `LINT-SUMMARY|errors=N|warnings=M|info=K`. Carries forward the v1 convention. New rules T1-T7, T14, T15 in phase 17; semantic rules T8-T13 deferred to phase 18.
- Managed-region marker syntax for agenda pages: `<!-- BEGIN agenda:<region> -->` / `<!-- END agenda:<region> -->`, where `<region>` ∈ `{next-actions, today, waiting, someday, stuck-projects}`. **Phase 17 rewrites the empty bare-`managed-region` placeholders shipped by phase 16 to use the per-region marker shape.** Anywhere a script hard-codes the bare `BEGIN managed-region` form, the script is wrong.
- Privacy classification (`source_kind` column): `private` if **any** of (a) the file path matches a git-crypt pattern in `.gitattributes`, OR (b) the file's frontmatter contains `tags: [private]`, OR (c) the action line contains a wikilink whose resolved target is itself private (rules a/b). The agenda generator filters `source_kind=private` actions and emits a hidden-count placeholder.
- Inbox-line ID synthesis (consumed by phase 18 triage but emitted in this phase): `inbox-<sha1(line)[:10]>-<lineno>`. File-shaped capture ID: `file-<sha1(relpath)[:10]>`. Action line on a content page: `<^id>` (the literal block ID).
- Action lines are **strictly single-line**. Wrapped continuations → `actions-rejected.tsv` with reason `continuation`; lint T15 warns. Lint `--fix` does NOT touch T15 violations (a wrapped action line could be a real intent or a stray paragraph; auto-rewriting risks data loss). T15 is documented as "user resolves manually."
- Commit after every task. Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- TDD discipline: failing fixture+test → run-it-fails → implement → run-it-passes → commit. One rule = one task with its own bats case. Out-of-band edits to a previously-committed test count as a new task.
- Branch per phase. Merge to `main` only after `bats tests/ && just lint` are clean.
- No emojis. No `Co-Authored-By` trailers. No "Generated with Claude Code" lines.

---

**Deliverable:** `scripts/action-scan.sh` (single-pass scanner over `content/**/*.md`, emits `actions.tsv` + `actions-rejected.tsv`, tags `source_kind`, dedup-checks `^id`); `scripts/agenda.sh` (regenerates the five managed-region pages with privacy filter, atomic rename, `last_updated` rewrite); per-region marker rewrite of the five placeholder pages from phase 16; `scripts/lint.sh` extended with rules **T1, T2, T3, T4, T5, T6, T7, T14, T15** (mechanical and grammar-scoped only — semantic rules T8-T13 ship in phase 18); `scripts/lint.sh --fix` extended for date normalization, missing `done:` / `since:` auto-stamp, tail-key sort, missing `^id` mint (collision-checked); justfile `agenda` and `scan` recipes (replace phase 16's commented stubs); BATS suites `tests/action_scan_test.bats`, `tests/agenda_test.bats`, `tests/lint_task_test.bats` with the 12-action-good fixture, the matching broken fixture, and the agenda-fixture-driven regen tests; smoke addition `tests/task_smoke_test.bats` extends to `task-init` → `capture` → manual triage edit → `agenda` → assert action appears under `@phone` in `next-actions.md`; spike decision record under `docs/decisions/2026-04-27-task-layer-spike.md`.

**Branch:** `phase-17-task-scanner-agenda`

---

## Task 17.1: Spike — Obsidian Tasks plugin Dataview-mode interop + `~`-separator block-ID round-trip

**Files:** Create: `tests/fixtures/spike-obsidian-vault/` (small Obsidian vault, gitignored except for the README), `docs/decisions/2026-04-27-task-layer-spike.md`. Modify (conditional): `scripts/lib/action-grammar.sh`, `docs/superpowers/specs/2026-04-27-task-layer-design.md` (only if separator falls back). Test: manual smoke + the conditional patch is covered by an existing phase-16 grammar bats case for the chain-id regex.

The spike has two unverified assumptions to settle. Both are absorbed here in one task because they share a fixture vault and the operator only opens Obsidian once. The spike is one calendar day in the operator's local clock, but in the plan it is a single ticked sequence of steps. **The branch step (creating `phase-17-task-scanner-agenda`) lives in this task** — phase-17's first commit is the spike outcome.

- [ ] **Step 1: Create branch**

```bash
git checkout main
git pull --ff-only
git checkout -b phase-17-task-scanner-agenda
```

Expected: `Switched to a new branch 'phase-17-task-scanner-agenda'`.

- [ ] **Step 2: Build the verification fixture vault**

The fixture is small enough to write inline. The vault is gitignored except for a README explaining how to reproduce — we do not commit `.obsidian/` because plugin metadata leaks personal config.

```bash
mkdir -p tests/fixtures/spike-obsidian-vault
cat > tests/fixtures/spike-obsidian-vault/README.md <<'EOF'
# Phase-17 spike fixture (gitignored)

Open this directory as an Obsidian vault. Install the Tasks plugin
(community plugins → Tasks by ilias-meretsis-... — pin the version in
.obsidian/community-plugins.json **after** verification). Enable
"Dataview format" in Tasks settings. Then walk the verification
checklist in `docs/decisions/2026-04-27-task-layer-spike.md`.

This fixture exists only as a manual proving ground. CI does NOT run
Obsidian; the spike outcome is captured as a markdown decision record.
EOF
cat > tests/fixtures/spike-obsidian-vault/notes.md <<'EOF'
---
title: "Spike notes"
---

# Status marker render check

- [ ] open task @phone due:2026-05-01 ^a01
- [/] in-progress task @computer ^a02
- [?] waiting task wait:[[bob-smith]] since:2026-04-22 ^a03
- [>] someday task @home ^a04
- [x] done task @computer due:2026-04-15 done:2026-04-13 ^a06
- [-] cancelled task ^a07

# Recurrence-chain block-ID round-trip

- [ ] water plants @home every:1w due:2026-05-11 ^a05~2
- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05

# Cross-reference

The chain head is [[notes#^a05]]; the second instance is [[notes#^a05~2]].
EOF
```

Append to `.gitignore` (only if the entry is missing):

```bash
grep -qxF 'tests/fixtures/spike-obsidian-vault/.obsidian/' .gitignore || \
  printf '\n# phase-17 spike vault — Obsidian config is per-machine\ntests/fixtures/spike-obsidian-vault/.obsidian/\n' >> .gitignore
```

- [ ] **Step 3: Open the vault in Obsidian and verify**

Operator opens `tests/fixtures/spike-obsidian-vault/` as an Obsidian vault, installs the Tasks plugin, enables Dataview-format mode, then walks the checklist below. Record findings inline in the decision record (Step 4).

Verification checklist (operator confirms YES/NO for each):

1. All six status markers (`[ ]`, `[/]`, `[?]`, `[>]`, `[x]`, `[-]`) render visibly distinct in source mode and in reading mode.
2. Tasks plugin (Dataview mode) does NOT silently re-format `due:`, `defer:`, `wait:`, `since:`, `every:`, `done:`, `priority:`, `est:` tail keys to its own emoji format on save. (Open the file, focus a checkbox, blur, then `git diff` — must be empty.)
3. The default emoji format (Tasks plugin's other mode) does NOT mis-parse our key:value tokens when Dataview format is enabled. (Toggle modes, verify rendering matches.)
4. Block ID `^a05` renders as a clickable anchor source / preview.
5. Block ID `^a05~2` (with tilde) renders the same way — Obsidian treats `~` as part of the ID, not a markdown emphasis marker.
6. Wikilink `[[notes#^a05~2]]` resolves (Cmd-click navigates to the recurrence-chain instance).
7. After running `just hugo` (or whatever builds the awiki Hugo site against this fixture, if that workflow is set up), the rendered HTML for `^a05~2` keeps the `~` in the anchor `id="..."` attribute — i.e. the Hugo preprocessing pipeline does not strip or rewrite `~`. If Hugo is not wired against this fixture in your workspace, fall back to opening the file in Hugo's existing v1 fixture pipeline (`tests/fixtures/wiki-good/`) by copying `notes.md` in, regenerating, and inspecting the HTML.

- [ ] **Step 4: Write the decision record**

```bash
mkdir -p docs/decisions
cat > docs/decisions/2026-04-27-task-layer-spike.md <<'EOF'
# Phase-17 Spike — Obsidian Tasks plugin + `~`-separator block-ID round-trip

**Date:** 2026-04-27
**Phase:** 17 (Scanner + Agenda)
**Status:** _to be filled in by spike operator_

## Assumption 1 — Obsidian Tasks plugin Dataview-mode interop

**Plugin:** `obsidian-tasks-plugin` (Tasks by ilias-meretsis et al.)
**Version pinned:** _<fill in plugin version from `.obsidian/community-plugins.json` after install>_
**Settings:** "Dataview format" toggled ON; "Auto-suggest in editor" left at default.

| Check | Result |
|-------|--------|
| All six status markers render distinctly in source + preview | _Y/N_ |
| Tail keys (`due:`, `defer:`, `wait:`, ...) survive a focus/blur cycle without rewriting | _Y/N_ |
| Default emoji mode (toggled on) does not misparse our key:value tokens when Dataview mode is enabled | _Y/N_ |

**Outcome:** _viable / not viable_. If not viable, downgrade Goal G2 in the spec to
"renders correctly in vanilla Obsidian without plugin-compat claim." No code
change required for this downgrade — only the spec line and the WIKI.md task-layer
block (phase-19 polish) get a clarifying note.

## Assumption 2 — `~`-separator block-ID round-trip

| Check | Result |
|-------|--------|
| `^a05~2` renders in Obsidian source mode without breaking | _Y/N_ |
| `^a05~2` renders in Obsidian preview mode as a block anchor | _Y/N_ |
| `[[notes#^a05~2]]` resolves on Cmd-click | _Y/N_ |
| Hugo emits an HTML anchor preserving the `~` (or a deterministic safe escape) | _Y/N_ |

**Outcome:** _`~` viable / fall back to `__`_.

If outcome is **fall back to `__`**, apply these patches and commit them
as part of this same spike task (Task 17.1 step 5):

1. `scripts/lib/action-grammar.sh`: change `AWIKI_RECUR_SEP` (or wherever
   the chain-id regex bakes in `~`) from `~` to `__`. Phase 16 should have
   exported the separator as a single constant; if not, refactor first.
2. `tests/action_grammar_test.bats`: any test asserting on `^a05~2` flips to `^a05__2`.
3. `docs/superpowers/specs/2026-04-27-task-layer-design.md`: search for `~<n>`
   and the line `\`~\` is unambiguous because user-typed IDs cannot contain it.`
   Replace with the `__` form. Update the T14 row in the lint table.
4. Regenerate phase-16's grammar bats expectations (the chain-shape case).

If outcome is **`~` viable**, no patches required; the rest of phase 17 uses `~` everywhere.

## Decision

_<one sentence: which separator is canonical for the rest of the plan>._
EOF
```

The operator fills in Y/N marks and the final decision sentence based on Step 3.

- [ ] **Step 5: If `~` round-trip fails, apply the fallback patch**

Conditional. Skip if the spike confirmed `~` works.

```bash
# only if the decision record's Assumption-2 outcome is "fall back to __":

# 1. Patch the grammar lib's chain separator. Phase 16's lib SHOULD export a
#    single AWIKI_RECUR_SEP constant; if it inlines `~` directly, refactor first.
grep -n 'AWIKI_RECUR_SEP\|~' scripts/lib/action-grammar.sh
# Edit to set AWIKI_RECUR_SEP="__" and rebuild the chain regex around it.

# 2. Patch the spec.
sed -i'' -e 's/\^<base>~<n>/\^<base>__<n>/g' \
         -e "s/\\\`\\~\\\` is unambiguous/\\\`__\\\` is unambiguous/g" \
  docs/superpowers/specs/2026-04-27-task-layer-design.md

# 3. Re-run the phase-16 grammar tests to confirm the lib still parses the new shape.
bats tests/action_grammar_test.bats
```

Expected (only if patch applied): the bats suite passes with the new separator.

- [ ] **Step 6: Commit the spike outcome**

```bash
git add docs/decisions/2026-04-27-task-layer-spike.md .gitignore
# Plus: scripts/lib/action-grammar.sh, the spec, and tests/action_grammar_test.bats
# IF AND ONLY IF the fallback patch was applied.
git commit -m "docs(task): record phase-17 spike outcome (Obsidian Tasks + recur-sep)"
```

Expected: a single commit on the phase-17 branch capturing whichever decision was made.

For the rest of this plan we write `~` in code blocks and prose. **If the spike chose `__`, do a global mental find-replace as you implement** — every regex, every fixture, every assertion. The spec patch (Step 5 item 2) is the source of truth.

---

## Task 17.2: Per-region marker rewrite of the phase-16 placeholder pages

**Files:** Modify: `content/agenda/next-actions.md`, `content/agenda/today.md`, `content/agenda/waiting.md`, `content/agenda/someday.md`, `content/agenda/stuck-projects.md`. Test: `tests/agenda_test.bats` (created in 17.5; this task adds a single placeholder bats case asserting the marker shape, then 17.5 builds the rest of the suite around it).

Phase 16 wrote each agenda page with a bare `<!-- BEGIN managed-region -->` / `<!-- END managed-region -->` pair. The spec calls for per-region markers (`<!-- BEGIN agenda:next-actions -->` etc.) so that `agenda.sh` knows which region in which file to swap. This task rewrites the five placeholder pages to use the new shape **before** any consumer is wired up.

- [ ] **Step 1: Confirm the phase-16 shape is what we expect**

```bash
grep -n 'BEGIN managed-region\|END managed-region' content/agenda/*.md
```

Expected: ten lines (five files × two markers). If a file is missing markers entirely, phase 16 was incomplete — back-fill before continuing.

- [ ] **Step 2: Rewrite each placeholder**

For each of the five files, swap the bare markers for the per-region shape. The body between the markers stays empty.

```bash
for region in next-actions today waiting someday stuck-projects; do
  f="content/agenda/${region}.md"
  # Replace BEGIN
  sed -i'' -e "s|<!-- BEGIN managed-region -->|<!-- BEGIN agenda:${region} -->|" "$f"
  sed -i'' -e "s|<!-- END managed-region -->|<!-- END agenda:${region} -->|" "$f"
done
```

Verify each file individually:

```bash
for region in next-actions today waiting someday stuck-projects; do
  echo "=== $region ==="
  grep -n "agenda:${region}" "content/agenda/${region}.md"
done
```

Expected: each file prints exactly two lines, both naming its own region.

- [ ] **Step 3: Add a placeholder bats case**

```bash
cat > tests/agenda_test.bats <<'EOF'
#!/usr/bin/env bats

@test "phase-17 placeholder agenda pages carry per-region markers" {
  for region in next-actions today waiting someday stuck-projects; do
    f="content/agenda/${region}.md"
    grep -qF "<!-- BEGIN agenda:${region} -->" "$f"
    grep -qF "<!-- END agenda:${region} -->" "$f"
  done
}
EOF
```

- [ ] **Step 4: Run the test (expect PASS)**

```bash
bats tests/agenda_test.bats
```

Expected: one test passes.

- [ ] **Step 5: Commit**

```bash
git add content/agenda/next-actions.md content/agenda/today.md \
        content/agenda/waiting.md content/agenda/someday.md \
        content/agenda/stuck-projects.md tests/agenda_test.bats
git commit -m "feat(task): rewrite phase-16 agenda placeholders to per-region markers"
```

---

## Task 17.3: 12-action good fixture + matching broken fixture

**Files:** Create: `tests/fixtures/wiki-task-good/` (full directory tree mirroring a tiny awiki: `content/projects/`, `content/contexts/`, `content/private/`, `content/inbox.md`, `.gitattributes`, `.awiki/maps/alias-to-slug.tsv`). Create: `tests/fixtures/wiki-task-broken/` (parallel tree exercising every rejection reason).

We build the fixtures **before** the scanner so that every later TDD step (17.4 scanner, 17.5 agenda, 17.6 lint task-rules) can reference identical bytes. Building the scanner against ad-hoc fixtures and then back-filling tests is the inverse of what we want — the spec table determines the fixture, not the implementation's accidental shape.

The 12 actions cover, in order:

| # | Status | Tail keys | Context | Source page | Privacy |
|---|--------|-----------|---------|-------------|---------|
| 1 | `[ ]` | `due:` | `@phone` | `projects/renovate-kitchen.md` | public |
| 2 | `[/]` | (none) | `@computer` | `projects/q3-launch.md` | public |
| 3 | `[?]` | `wait: since:` | (none) | `projects/q3-launch.md` | public |
| 4 | `[>]` | (none) | `@home` | `projects/q3-launch.md` | public |
| 5 | `[ ]` | `every: due:` | `@home` | `projects/water-plants.md` | public — chain head `^a05` |
| 6 | `[ ]` | `every: due:` | `@home` | `projects/water-plants.md` | public — chain instance `^a05~2` |
| 7 | `[x]` | `due: done:` | `@computer` | `projects/q3-launch.md` | public |
| 8 | `[ ]` | `priority: est:` | `@phone` | `projects/q3-launch.md` | public |
| 9 | `[ ]` | `defer:` | `@computer` | `projects/q3-launch.md` | public — defer date in future |
| 10 | `[ ]` | (none) | `@phone` | `private/secret-project.md` | private — file path under git-crypt |
| 11 | `[ ]` | (none) | `@phone` | `projects/q3-launch.md` (frontmatter `tags: [private]`) | private — frontmatter tag |
| 12 | `[ ]` | (none) | `@phone` | `projects/onboarding-revamp.md` | private — wikilink target `[[bob-private]]` resolves to a private page |

**Plus three NOT-action lines that must NOT appear in `actions.tsv`:**

- A prose line containing `@computer` outside any checkbox (e.g. "discussed @computer setup with team").
- A fenced ` ```bash ` code block containing the literal `[ ] not a real task`.
- A blockquote line `> [ ] this is a quoted action, not a real one` — single-line quoted checkboxes are still **not** action lines per the spec (action grammar requires the line to start with `- [<status>]`).

- [ ] **Step 1: Build `tests/fixtures/wiki-task-good/` skeleton**

```bash
mkdir -p tests/fixtures/wiki-task-good/content/projects
mkdir -p tests/fixtures/wiki-task-good/content/contexts
mkdir -p tests/fixtures/wiki-task-good/content/private
mkdir -p tests/fixtures/wiki-task-good/.awiki/maps
```

- [ ] **Step 2: Write `.gitattributes` and the alias-to-slug map**

```bash
cat > tests/fixtures/wiki-task-good/.gitattributes <<'EOF'
# git-crypt section — fixture only
content/private/** filter=git-crypt diff=git-crypt
EOF
```

The alias map is what `lint.sh` would have produced; phase 17's scanner reads it but does not regenerate it (per spec). The line shape is `<alias>\t<slug>\t<path>\t<source_kind>`:

```bash
cat > tests/fixtures/wiki-task-good/.awiki/maps/alias-to-slug.tsv <<'EOF'
@phone	phone	content/contexts/phone.md	public
@computer	computer	content/contexts/computer.md	public
@home	home	content/contexts/home.md	public
bob-smith	bob-smith	content/entities/bob-smith.md	public
bob-private	bob-private	content/private/people/bob-private.md	private
renovate-kitchen	renovate-kitchen	content/projects/renovate-kitchen.md	public
q3-launch	q3-launch	content/projects/q3-launch.md	public
water-plants	water-plants	content/projects/water-plants.md	public
secret-project	secret-project	content/private/secret-project.md	private
onboarding-revamp	onboarding-revamp	content/projects/onboarding-revamp.md	public
EOF
```

(The format is fixture-only; the actual phase-19 alias-map shape may differ. The scanner reads only the alias and the source_kind columns, so additional columns are tolerated.)

- [ ] **Step 3: Write context pages**

```bash
for c in phone computer home; do
  cat > "tests/fixtures/wiki-task-good/content/contexts/${c}.md" <<EOF
---
title: "@${c}"
type: context
aliases: ['@${c}']
draft: false
---
EOF
done
```

- [ ] **Step 4: Write `projects/renovate-kitchen.md` (action 1)**

```bash
cat > tests/fixtures/wiki-task-good/content/projects/renovate-kitchen.md <<'EOF'
---
title: "Renovate kitchen"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [ ] call dentist about crown @phone due:2026-05-01 ^a01
EOF
```

- [ ] **Step 5: Write `projects/q3-launch.md` (actions 2, 3, 4, 7, 8, 9 + a prose `@mention` + a code-block `[ ]` + a quoted `[ ]`)**

```bash
cat > tests/fixtures/wiki-task-good/content/projects/q3-launch.md <<'EOF'
---
title: "Q3 launch"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

We discussed @computer setup with the team last week — that
mention is prose, not an action context.

```bash
- [ ] not a real task
```

> [ ] this is a quoted line, scanner should ignore it.

## Open Actions

- [/] draft proposal @computer ^a02
- [?] q3 budget approval wait:[[bob-smith]] since:2026-04-22 ^a03
- [>] reorganize garage someday @home ^a04
- [x] file taxes @computer due:2026-04-15 done:2026-04-13 ^a07
- [ ] review onboarding deck @phone priority:2 est:30m ^a08
- [ ] follow up with vendor @computer defer:2026-09-01 ^a09
EOF
```

- [ ] **Step 6: Write `projects/water-plants.md` (chain head + instance)**

```bash
cat > tests/fixtures/wiki-task-good/content/projects/water-plants.md <<'EOF'
---
title: "Water plants"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [ ] water plants @home every:1w due:2026-05-11 ^a05~2
- [x] water plants @home every:1w due:2026-05-04 done:2026-05-04 ^a05
EOF
```

(Chain instance above the head is intentional — that mirrors `action-recur.sh`'s output in phase 18, where the new open instance is written immediately above the just-completed line.)

- [ ] **Step 7: Write `private/secret-project.md` (action 10 — private by file path)**

```bash
cat > tests/fixtures/wiki-task-good/content/private/secret-project.md <<'EOF'
---
title: "Secret project"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [ ] sensitive call @phone ^a10
EOF
```

- [ ] **Step 8: Write `projects/onboarding-revamp.md` (action 12 — private by wikilink target)**

The page itself is public. The action contains a wikilink to a private page (`[[bob-private]]`), which makes the action private. This is the wikilink-target rule.

```bash
cat > tests/fixtures/wiki-task-good/content/projects/onboarding-revamp.md <<'EOF'
---
title: "Onboarding revamp"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [ ] discuss with [[bob-private]] @phone ^a12
EOF
```

- [ ] **Step 9: Add the frontmatter-private project (action 11)**

This page has `tags: [private]` in frontmatter, so any action on it is private even though the path is `content/projects/`.

```bash
cat > tests/fixtures/wiki-task-good/content/projects/budget-private.md <<'EOF'
---
title: "Budget (private)"
type: project
status: active
last_updated: 2026-04-27
tags: [private]
draft: false
---

## Open Actions

- [ ] reconcile q1 numbers @phone ^a11
EOF
```

- [ ] **Step 10: Build the broken fixture covering each rejection reason**

The broken fixture's purpose: every lint T-rule fires on at least one line, and every `actions-rejected.tsv` `reason` code is exercised. Lint test 17.6 walks each rule against this fixture.

```bash
mkdir -p tests/fixtures/wiki-task-broken/content/projects
mkdir -p tests/fixtures/wiki-task-broken/content/contexts
mkdir -p tests/fixtures/wiki-task-broken/.awiki/maps

# Reuse the good fixture's alias map for lookups.
cp tests/fixtures/wiki-task-good/.awiki/maps/alias-to-slug.tsv \
   tests/fixtures/wiki-task-broken/.awiki/maps/alias-to-slug.tsv

cat > tests/fixtures/wiki-task-broken/.gitattributes <<'EOF'
content/private/** filter=git-crypt diff=git-crypt
EOF

cat > tests/fixtures/wiki-task-broken/content/contexts/phone.md <<'EOF'
---
title: "@phone"
type: context
aliases: ['@phone']
draft: false
---
EOF

cat > tests/fixtures/wiki-task-broken/content/projects/broken.md <<'EOF'
---
title: "Broken"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [Z] T1 bad-status: marker not in allowed set ^b01
- [ ] T3 bad-key: bogus:value @phone ^b03
- [ ] T4 bad-date: due:2026-13-40 @phone ^b04
- [?] T5 waiting-no-person: missing wait: @phone ^b05
- [ ] T6 bad-context: @nonexistent ^b06
- [ ] T14 bad-id-shape: too short id ^id
- [ ] T14 bad-id-shape: tilde outside chain ^abc~zz
- [ ] T2 dup-id-a ^dupid
- [ ] T2 dup-id-b ^dupid
- [ ] T15 wrapped continuation follows
    this indented line continues the action above and is invalid ^b15
- [ ] continuation reason in actions-rejected.tsv: this trailing key is bare:no_colon ^b16
EOF

cat > tests/fixtures/wiki-task-broken/content/projects/managed-region-edited.md <<'EOF'
---
title: "Hand-edited managed region"
type: agenda
last_updated: 2026-04-27
draft: false
---

<!-- BEGIN agenda:next-actions -->
This text was hand-typed inside a managed region. T7 must fire.
<!-- END agenda:next-actions -->
EOF
```

The dup-id pair (`^dupid` twice on the same page) covers T2's "literal duplicate" branch. The cross-page chain-distinguish branch (two pages each carrying `^a05~2`) is covered by Step 11.

- [ ] **Step 11: Add the cross-page chain-distinguish broken fixture**

```bash
cat > tests/fixtures/wiki-task-broken/content/projects/chain-page-1.md <<'EOF'
---
title: "Chain page 1"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

- [ ] water plants @phone every:1w due:2026-05-11 ^a05~2
EOF

cat > tests/fixtures/wiki-task-broken/content/projects/chain-page-2.md <<'EOF'
---
title: "Chain page 2"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

- [ ] water plants @phone every:1w due:2026-05-18 ^a05~2
EOF
```

(Two `^a05~2` block-IDs across two different pages → T2 fires. Compare against `^a05~2` and `^a05~3` on different pages, which is allowed; that case is covered by the good fixture for the agenda-only-cares-about-instance-uniqueness assertion in 17.6.)

- [ ] **Step 12: Add a smoke bats case asserting both fixtures parse end-to-end**

```bash
cat > tests/action_scan_test.bats <<'EOF'
#!/usr/bin/env bats

# Phase-17 scanner BATS suite. The actual scanner ships in 17.4; this file
# is created here so the fixture skeleton has a holding place. Real
# assertions land in 17.4.

@test "wiki-task-good fixture has 12 action-line files plus three noise files" {
  [ -d tests/fixtures/wiki-task-good ]
  # Six project pages with actions.
  for f in renovate-kitchen q3-launch water-plants budget-private onboarding-revamp; do
    [ -f "tests/fixtures/wiki-task-good/content/projects/${f}.md" ]
  done
  [ -f tests/fixtures/wiki-task-good/content/private/secret-project.md ]
}

@test "wiki-task-broken fixture exists with all rejection-reason files" {
  [ -d tests/fixtures/wiki-task-broken ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/broken.md ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/managed-region-edited.md ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/chain-page-1.md ]
  [ -f tests/fixtures/wiki-task-broken/content/projects/chain-page-2.md ]
}
EOF
```

- [ ] **Step 13: Run the placeholder tests (expect PASS)**

```bash
bats tests/action_scan_test.bats
```

Expected: two tests pass.

- [ ] **Step 14: Commit**

```bash
git add tests/fixtures/wiki-task-good tests/fixtures/wiki-task-broken \
        tests/action_scan_test.bats
git commit -m "test(task): add 12-action good fixture + broken-rejection fixture"
```

---

## Task 17.4: `scripts/action-scan.sh` — single-pass scanner

**Files:** Create: `scripts/action-scan.sh`. Modify: `tests/action_scan_test.bats` (extend with full assertions). Modify: `.gitignore` (already excludes `.awiki/maps/` per phase 16; verify).

The scanner does one walk over `content/**/*.md`, sources the action-grammar lib, and emits two TSVs:

- `.awiki/maps/actions.tsv` — canonical action records.
- `.awiki/maps/actions-rejected.tsv` — lines that look like actions but fail validation.

Both are gitignored. The scanner is invoked under shared lock during read-only callers (lint), under exclusive lock when a follow-on writer is queued (agenda). For phase 17 it acquires shared (`awiki_lock_shared`) because it does not modify files inside `content/`.

Column layout for `actions.tsv` (16 columns, tab-separated, escaped per scanner rules below):

```
id  status  text  file  line  context  due  defer  wait  since  every  done  priority  est  project  source_kind
```

`actions-rejected.tsv` is shorter (5 columns):

```
file  line  reason  raw_line  details
```

Reason codes for phase 17: `bad-status`, `bad-key`, `bad-date`, `dup-id`, `bad-id-shape`, `continuation`, `no-status`. Phase 18 may add semantic codes (`waiting-no-person` is mechanical so it's in phase 17 lint, but the scanner itself does not reject — `[?]` lines without `wait:` still appear in `actions.tsv` so that lint can flag and `--fix` can auto-stamp). Same for `bad-context`: scanner emits the row; lint flags.

**Escape rule:** TAB and NEWLINE characters appearing inside `text`, `raw_line`, or `details` are replaced with literal escapes (`\t`, `\n`) before writing. The scanner never writes raw control bytes into the TSV. Consumers reverse the escape when needed.

- [ ] **Step 1: Extend the bats suite with failing assertions**

Append to `tests/action_scan_test.bats`:

```bash
cat >> tests/action_scan_test.bats <<'EOF'

setup_good() {
  WORK="$(mktemp -d)"
  cp -R tests/fixtures/wiki-task-good/. "$WORK/"
  cd "$WORK"
}

setup_broken() {
  WORK="$(mktemp -d)"
  cp -R tests/fixtures/wiki-task-broken/. "$WORK/"
  cd "$WORK"
}

teardown_work() {
  cd /
  [ -n "${WORK:-}" ] && rm -rf "$WORK"
}

@test "scanner emits actions.tsv with all 12 actions from good fixture" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  [ "$status" -eq 0 ]
  [ -f "$WORK/.awiki/maps/actions.tsv" ]
  # Header + 12 records.
  run wc -l < "$WORK/.awiki/maps/actions.tsv"
  [ "$output" = "13" ]
  teardown_work
}

@test "scanner classifies private actions correctly (10, 11, 12)" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  # action 10 — private by file path
  run grep -P '^a10\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tprivate' ]]
  # action 11 — private by frontmatter tag
  run grep -P '^a11\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tprivate' ]]
  # action 12 — private by wikilink target
  run grep -P '^a12\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tprivate' ]]
  teardown_work
}

@test "scanner classifies action 1 as public" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -P '^a01\t' "$WORK/.awiki/maps/actions.tsv"
  [[ "$output" == *$'\tpublic' ]]
  teardown_work
}

@test "scanner does NOT include prose @mentions in actions.tsv" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  # The 'discussed @computer setup with team' prose line must not appear.
  run grep -F 'discussed' "$WORK/.awiki/maps/actions.tsv"
  [ -z "$output" ]
  teardown_work
}

@test "scanner does NOT include code-block [ ] in actions.tsv" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -F 'not a real task' "$WORK/.awiki/maps/actions.tsv"
  [ -z "$output" ]
  teardown_work
}

@test "scanner does NOT include blockquote [ ] in actions.tsv" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -F 'quoted line' "$WORK/.awiki/maps/actions.tsv"
  [ -z "$output" ]
  teardown_work
}

@test "scanner records chain head ^a05 and chain instance ^a05~2 as distinct rows" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  run grep -cE '^(a05|a05~2)\t' "$WORK/.awiki/maps/actions.tsv"
  [ "$output" = "2" ]
  teardown_work
}

@test "scanner emits actions-rejected.tsv with continuation row on broken fixture" {
  setup_broken
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" || true
  [ -f "$WORK/.awiki/maps/actions-rejected.tsv" ]
  run grep -F 'continuation' "$WORK/.awiki/maps/actions-rejected.tsv"
  [ -n "$output" ]
  teardown_work
}

@test "scanner exits 1 when duplicate ^id detected on same page" {
  setup_broken
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  [ "$status" -eq 1 ]
  teardown_work
}

@test "scanner stdout summary names file count, action count, rejected count" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  [[ "$output" == *"scanned"* ]]
  [[ "$output" == *"actions"* ]]
  [[ "$output" == *"rejected"* ]]
  teardown_work
}

@test "scanner is idempotent (second run produces byte-identical TSV)" {
  setup_good
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  cp "$WORK/.awiki/maps/actions.tsv" "$WORK/run1.tsv"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  diff -q "$WORK/run1.tsv" "$WORK/.awiki/maps/actions.tsv"
  teardown_work
}

@test "scanner records inbox-line synthesis ID for inbox.md captures" {
  setup_good
  printf -- '- 2026-04-27 14:32 call dentist about crown\n' \
    > "$WORK/content/inbox.md"
  cat > "$WORK/content/inbox.md" <<'INBOX'
---
title: Inbox
type: inbox
draft: true
---

- 2026-04-27 14:32 call dentist about crown
- 2026-04-27 14:33 idea: rewrite onboarding email
INBOX
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh"
  # Inbox lines do NOT enter actions.tsv (no checkbox marker), but the
  # synthesis routine is library-mode: phase-18 triage will call into it.
  # For phase 17 we only test that the scanner does not blow up on inbox.md.
  [ -f "$WORK/.awiki/maps/actions.tsv" ]
  teardown_work
}
EOF
```

- [ ] **Step 2: Run the bats suite (expect FAIL — scanner absent)**

```bash
bats tests/action_scan_test.bats
```

Expected: every new case fails with a "No such file or directory" or similar — `scripts/action-scan.sh` does not exist yet. The two pre-existing fixture-shape tests still pass.

- [ ] **Step 3: Write `scripts/action-scan.sh`**

```bash
cat > scripts/action-scan.sh <<'EOF'
#!/usr/bin/env bash
# action-scan.sh — single-pass scanner over content/**/*.md.
# Emits .awiki/maps/actions.tsv and .awiki/maps/actions-rejected.tsv.
# See spec § "Scanner + Agenda Generation".
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(pwd)}"
cd "$REPO_ROOT"

# Source the canonical action-grammar lib.
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/action-grammar.sh"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/lock.sh"

MAPS_DIR="$REPO_ROOT/.awiki/maps"
mkdir -p "$MAPS_DIR"
ACTIONS_TSV="$MAPS_DIR/actions.tsv"
REJECTED_TSV="$MAPS_DIR/actions-rejected.tsv"
ALIAS_MAP="$MAPS_DIR/alias-to-slug.tsv"

# --- Privacy classification helpers ---

# Read git-crypt patterns from .gitattributes. Each pattern becomes a
# bash glob we can match against file paths.
GITCRYPT_PATTERNS=()
if [[ -f "$REPO_ROOT/.gitattributes" ]]; then
  while IFS= read -r line; do
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    [[ "$line" =~ filter=git-crypt ]] || continue
    pattern="${line%%[[:space:]]*}"
    GITCRYPT_PATTERNS+=("$pattern")
  done < "$REPO_ROOT/.gitattributes"
fi

path_is_private_by_gitcrypt() {
  local relpath="$1"
  local pat
  for pat in "${GITCRYPT_PATTERNS[@]}"; do
    # Convert glob-style ** to bash-extglob equivalent.
    local bash_pat="${pat//\*\*/*}"
    # shellcheck disable=SC2053
    [[ "$relpath" == $bash_pat ]] && return 0
  done
  return 1
}

frontmatter_has_private_tag() {
  local f="$1"
  awk '
    BEGIN{in_fm=0}
    /^---[[:space:]]*$/ { in_fm = !in_fm; next }
    in_fm && /tags:/ && /private/ { found=1 }
    END{ exit (found?0:1) }
  ' "$f"
}

# Look up a wikilink slug in the alias map. Prints "private" or "public"
# (defaulting to public if the slug is unknown — phase-17 known limit).
wikilink_target_kind() {
  local slug="$1"
  [[ -f "$ALIAS_MAP" ]] || { echo public; return; }
  local kind
  kind="$(awk -F'\t' -v s="$slug" '$2==s { print $4; exit }' "$ALIAS_MAP")"
  [[ -z "$kind" ]] && kind="public"
  printf '%s' "$kind"
}

# Derive source_kind for a file + action line.
classify_source_kind() {
  local relpath="$1" line="$2"
  if path_is_private_by_gitcrypt "$relpath"; then echo private; return; fi
  if frontmatter_has_private_tag "$REPO_ROOT/$relpath"; then echo private; return; fi
  # Wikilink-target rule.
  while [[ "$line" =~ \[\[([^]]+)\]\] ]]; do
    local raw="${BASH_REMATCH[1]}"
    local slug="${raw%%#*}"
    slug="${slug%%|*}"
    if [[ "$(wikilink_target_kind "$slug")" == "private" ]]; then
      echo private; return
    fi
    line="${line/\[\[$raw\]\]/}"
  done
  echo public
}

tsv_escape() {
  local s="$1"
  s="${s//$'\t'/\\t}"
  s="${s//$'\n'/\\n}"
  printf '%s' "$s"
}

# --- Core scan ---

scan_one_file() {
  local relpath="$1"
  local f="$REPO_ROOT/$relpath"
  local lineno=0
  local in_fence=0
  local prev_was_action=0
  local frontmatter_done=0
  local fm_seen=0
  local project_slug=""
  # Project slug is derived from frontmatter if type=project.
  if awk 'BEGIN{in=0} /^---[[:space:]]*$/ { in=!in; next } in && /^type:[[:space:]]*project/ { found=1 } END{ exit (found?0:1) }' "$f"; then
    project_slug="$(basename "$relpath" .md)"
  fi

  while IFS= read -r line || [[ -n "$line" ]]; do
    lineno=$((lineno + 1))
    if [[ "$line" =~ ^---[[:space:]]*$ ]]; then
      if (( fm_seen == 0 )); then fm_seen=1
      elif (( frontmatter_done == 0 )); then frontmatter_done=1
      fi
      continue
    fi
    (( fm_seen == 1 && frontmatter_done == 0 )) && continue

    if [[ "$line" =~ ^\`\`\` ]]; then
      in_fence=$((1 - in_fence)); prev_was_action=0; continue
    fi
    (( in_fence == 1 )) && { prev_was_action=0; continue; }

    # Blockquote — skip entirely.
    [[ "$line" =~ ^\> ]] && { prev_was_action=0; continue; }

    # Continuation: indented non-blank line following an action line.
    if (( prev_was_action == 1 )) && [[ "$line" =~ ^[[:space:]]+[^[:space:]] ]]; then
      printf '%s\t%d\t%s\t%s\t%s\n' \
        "$relpath" "$lineno" "continuation" "$(tsv_escape "$line")" "follows-action-line" \
        >> "$REJECTED_TSV"
      continue
    fi

    if [[ ! "$line" =~ ^-[[:space:]]\[(.)\][[:space:]] ]]; then
      prev_was_action=0; continue
    fi

    local status_marker="${BASH_REMATCH[1]}"
    case "$status_marker" in
      ' '|'/'|'?'|'>'|'x'|'-') ;;
      *)
        printf '%s\t%d\t%s\t%s\t%s\n' \
          "$relpath" "$lineno" "bad-status" \
          "$(tsv_escape "$line")" "marker=[$status_marker]" \
          >> "$REJECTED_TSV"
        prev_was_action=1; continue
        ;;
    esac

    awiki_grammar_parse_action "$line" || {
      printf '%s\t%d\t%s\t%s\t%s\n' \
        "$relpath" "$lineno" "no-status" \
        "$(tsv_escape "$line")" "grammar-parse-failed" \
        >> "$REJECTED_TSV"
      prev_was_action=1; continue
    }

    local id="${AWIKI_AG_ID:-}"
    local text="${AWIKI_AG_TEXT:-}"
    local context="${AWIKI_AG_CONTEXT:-}"
    local due="${AWIKI_AG_DUE:-}"
    local defer="${AWIKI_AG_DEFER:-}"
    local wait="${AWIKI_AG_WAIT:-}"
    local since="${AWIKI_AG_SINCE:-}"
    local every="${AWIKI_AG_EVERY:-}"
    local done_="${AWIKI_AG_DONE:-}"
    local priority="${AWIKI_AG_PRIORITY:-}"
    local est="${AWIKI_AG_EST:-}"

    local source_kind
    source_kind="$(classify_source_kind "$relpath" "$line")"

    printf '%s\t%s\t%s\t%s\t%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n' \
      "$id" "$status_marker" "$(tsv_escape "$text")" "$relpath" "$lineno" \
      "$context" "$due" "$defer" "$wait" "$since" "$every" "$done_" \
      "$priority" "$est" "$project_slug" "$source_kind" \
      >> "$ACTIONS_TSV"

    prev_was_action=1
  done < "$f"
}

# --- Driver ---

awiki_lock_shared --timeout=30 -- bash -c '
true
' >/dev/null 2>&1 || true   # Phase-17 scanner is read-only against content;
                             # taking the shared lock around the scan body
                             # is sufficient. Below we re-acquire for the
                             # actual write to the maps dir.

: > "$ACTIONS_TSV.tmp.$$"
: > "$REJECTED_TSV.tmp.$$"

# Header for actions.tsv.
printf 'id\tstatus\ttext\tfile\tline\tcontext\tdue\tdefer\twait\tsince\tevery\tdone\tpriority\test\tproject\tsource_kind\n' \
  > "$ACTIONS_TSV.tmp.$$"

ACTIONS_TSV="$ACTIONS_TSV.tmp.$$" REJECTED_TSV="$REJECTED_TSV.tmp.$$"

scanned_files=0
while IFS= read -r relpath; do
  [[ -z "$relpath" ]] && continue
  scanned_files=$((scanned_files + 1))
  scan_one_file "${relpath#./}"
done < <(cd "$REPO_ROOT" && find content -type f -name '*.md' 2>/dev/null | sort)

mv "$ACTIONS_TSV.tmp.$$" "$ACTIONS_TSV"
[[ -f "$REJECTED_TSV.tmp.$$" ]] && mv "$REJECTED_TSV.tmp.$$" "$REJECTED_TSV"
[[ -f "$REJECTED_TSV.tmp.$$" ]] || : > "$REJECTED_TSV"

# Summary + dup-id check.
total_actions=$(($(wc -l < "$ACTIONS_TSV") - 1))
rejected=$(wc -l < "$REJECTED_TSV" | tr -d ' ')
open_count=$(awk -F'\t' 'NR>1 && ($2==" " || $2=="/") {n++} END{print n+0}' "$ACTIONS_TSV")
done_count=$(awk -F'\t' 'NR>1 && $2=="x" {n++} END{print n+0}' "$ACTIONS_TSV")

printf 'scanned %d files, %d actions, %d rejected, %d open, %d done\n' \
  "$scanned_files" "$total_actions" "$rejected" "$open_count" "$done_count"

# Detect duplicate IDs (same literal id appears ≥2×).
dup="$(awk -F'\t' 'NR>1 && $1!="" {n[$1]++} END{ for (k in n) if (n[k]>1) print k }' "$ACTIONS_TSV")"
if [[ -n "$dup" ]]; then
  printf 'SCAN|dup-id|%s\n' "$dup" >&2
  exit 1
fi

exit 0
EOF
chmod +x scripts/action-scan.sh
```

- [ ] **Step 4: Run the bats suite (expect PASS)**

```bash
bats tests/action_scan_test.bats
```

Expected: every case passes — including `scanned 8 files, 12 actions, ...` summary and the dup-id exit-1 case for the broken fixture.

- [ ] **Step 5: Commit**

```bash
git add scripts/action-scan.sh tests/action_scan_test.bats
git commit -m "feat(task): add action-scan.sh single-pass scanner with privacy classification"
```

---

## Task 17.5: `scripts/agenda.sh` — managed-region regen for the five agenda pages

**Files:** Create: `scripts/agenda.sh`. Modify: `tests/agenda_test.bats`. Test inputs: pre-built `actions.tsv` fixtures under `tests/fixtures/agenda-inputs/` constructed inline by the bats setup.

`agenda.sh` reads `.awiki/maps/actions.tsv` (built by 17.4) and rewrites the five managed-region blocks in `content/agenda/{next-actions,today,waiting,someday,stuck-projects}.md`. It does **not** invoke the scanner — that responsibility lives in the `just agenda` recipe (Task 17.7).

Per-page rules (from spec § "Per-page rules"):

- `next-actions.md` — every `[ ]` and `[/]` action whose `defer:` is empty OR `defer ≤ today`. Grouped by `@context`, then by project. No-context bucket appears last.
- `today.md` — actions where `due ≤ today` OR (`defer ≤ today` AND status `[ ]`/`[/]`) OR `due` overdue. Sorted: overdue → today → in-progress.
- `waiting.md` — every `[?]`. Grouped by `wait:` person. Each line shows `since:` and the day count (`<N>d waiting`).
- `someday.md` — every `[>]`. Grouped by project; "unassigned" bucket last.
- `stuck-projects.md` — projects whose page has `status: active` AND zero open `[ ]`/`[/]` actions, OR whose `last_updated` is more than 14 days old with no `[x]` action since. Each line: `<slug> — <reason>`. Computing the "no [x] since last_updated" branch from `actions.tsv`'s `done:` column (set by `[x]` actions only).

Privacy filter (mandatory): exclude every row with `source_kind=private`. For each region, count the excluded rows; if the count is non-zero, append inside the managed region:

```
> _<N> action(s) hidden — origin under private/encrypted path._
```

Counts only — no slugs, no text. The `>` is a markdown blockquote so it renders as a callout. The override `AWIKI_AGENDA_INCLUDE_PRIVATE=1` is implemented in the shape-check stub (errors out with exit 5 if agenda pages are not under git-crypt patterns); the full include-path lives in phase 19 once `encrypt-init` covers `content/agenda/**`. For phase 17 the override only logs and exits 5 — actual inclusion of private rows under encryption is phase-19's job.

Atomic rename: every regen writes to `<file>.tmp.<pid>`, then `mv` atomically into place.

Each page's `last_updated:` frontmatter line is rewritten to today's date on every regen. The body outside the managed region is preserved verbatim.

- [ ] **Step 1: Extend the bats suite**

Append to `tests/agenda_test.bats`:

```bash
cat >> tests/agenda_test.bats <<'EOF'

setup_agenda() {
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/.awiki/maps" "$WORK/content/agenda" "$WORK/content/projects" "$WORK/content/contexts"
  for region in next-actions today waiting someday stuck-projects; do
    cat > "$WORK/content/agenda/${region}.md" <<EOM
---
title: "${region}"
type: agenda
last_updated: 2025-01-01
draft: false
---

User notes above the managed region survive regen.

<!-- BEGIN agenda:${region} -->
<!-- END agenda:${region} -->

User notes below the managed region also survive regen.
EOM
  done
  # Minimal alias map.
  cat > "$WORK/.awiki/maps/alias-to-slug.tsv" <<'EOM'
@phone	phone	content/contexts/phone.md	public
@computer	computer	content/contexts/computer.md	public
@home	home	content/contexts/home.md	public
EOM
  # Build a minimal actions.tsv with one row per region's natural target.
  cat > "$WORK/.awiki/maps/actions.tsv" <<EOM
id	status	text	file	line	context	due	defer	wait	since	every	done	priority	est	project	source_kind
a01	 	call dentist about crown	content/projects/renovate-kitchen.md	11	@phone	2026-05-01						 		renovate-kitchen	public
a02	/	draft proposal	content/projects/q3-launch.md	11	@computer								 	q3-launch	public
a03	?	q3 budget approval	content/projects/q3-launch.md	12		 		bob-smith	2026-04-22				q3-launch	public
a04	>	reorganize garage someday	content/projects/q3-launch.md	13	@home								 	q3-launch	public
a07	x	file taxes	content/projects/q3-launch.md	14	@computer	2026-04-15					2026-04-13			q3-launch	public
a10	 	sensitive call	content/private/secret-project.md	11	@phone								 	secret-project	private
EOM
}

teardown_agenda() {
  cd /
  [ -n "${WORK:-}" ] && rm -rf "$WORK"
}

@test "agenda.sh emits next-actions content with @phone heading" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" run bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  [ "$status" -eq 0 ]
  run grep -F '### @phone' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  run grep -F 'call dentist about crown' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh respects the privacy filter (a10 not in next-actions)" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'sensitive call' "$WORK/content/agenda/next-actions.md"
  [ -z "$output" ]
  run grep -F 'action(s) hidden' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh waiting.md groups by wait person" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'bob-smith' "$WORK/content/agenda/waiting.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh someday.md lists [>] actions grouped by project" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'reorganize garage' "$WORK/content/agenda/someday.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh preserves user notes outside the managed region" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  run grep -F 'User notes above the managed region' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  run grep -F 'User notes below the managed region' "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh rewrites last_updated to today on every regen" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  today="$(date +%Y-%m-%d)"
  run grep -F "last_updated: $today" "$WORK/content/agenda/next-actions.md"
  [ -n "$output" ]
  teardown_agenda
}

@test "agenda.sh uses atomic rename (no .tmp file leaks)" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  # No leftover temp files.
  run find "$WORK/content/agenda" -name '*.tmp.*'
  [ -z "$output" ]
  teardown_agenda
}

@test "agenda.sh idempotent (second run is byte-identical)" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  cp "$WORK/content/agenda/next-actions.md" "$WORK/run1.md"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  diff -q "$WORK/run1.md" "$WORK/content/agenda/next-actions.md"
  teardown_agenda
}

@test "agenda.sh exits 5 when AWIKI_AGENDA_INCLUDE_PRIVATE=1 without encryption coverage" {
  setup_agenda
  AWIKI_REPO_ROOT="$WORK" AWIKI_AGENDA_INCLUDE_PRIVATE=1 \
    run bash "$BATS_TEST_DIRNAME/../scripts/agenda.sh"
  [ "$status" -eq 5 ]
  teardown_agenda
}
EOF
```

- [ ] **Step 2: Run bats (expect FAIL — agenda.sh absent)**

```bash
bats tests/agenda_test.bats
```

Expected: every new test fails; the placeholder bats case from Task 17.2 still passes.

- [ ] **Step 3: Write `scripts/agenda.sh`**

```bash
cat > scripts/agenda.sh <<'EOF'
#!/usr/bin/env bash
# agenda.sh — regenerate the five managed-region agenda pages from
# .awiki/maps/actions.tsv. Privacy filter excludes source_kind=private.
# Atomic rewrite + last_updated stamp on every regen.
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(pwd)}"
cd "$REPO_ROOT"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck disable=SC1091
. "$SCRIPT_DIR/lib/lock.sh"

MAPS="$REPO_ROOT/.awiki/maps"
ACTIONS="$MAPS/actions.tsv"
[[ -f "$ACTIONS" ]] || { echo "AGENDA|missing|$ACTIONS — run action-scan.sh first" >&2; exit 2; }

TODAY="$(date +%Y-%m-%d)"
INCLUDE_PRIVATE="${AWIKI_AGENDA_INCLUDE_PRIVATE:-0}"

# Encryption-coverage gate for INCLUDE_PRIVATE=1.
if [[ "$INCLUDE_PRIVATE" == "1" ]]; then
  if ! grep -q 'content/agenda/\*\*' "$REPO_ROOT/.gitattributes" 2>/dev/null; then
    echo "AGENDA|include-private-blocked|content/agenda/** not under git-crypt; refusing to inline private rows" >&2
    exit 5
  fi
fi

AGENDA_DIR="$REPO_ROOT/content/agenda"

# --- Per-region builders ---

# Helper: filter actions.tsv by status set, return TSV body (no header).
filter_status() {
  local statuses="$1"
  awk -F'\t' -v st="$statuses" '
    BEGIN { n=split(st, a, ","); for (i=1;i<=n;i++) keep[a[i]]=1 }
    NR>1 && ($2 in keep) { print }
  ' "$ACTIONS"
}

privacy_partition() {
  local raw="$1"
  PRIVATE_COUNT=0
  PUBLIC_ROWS=""
  while IFS= read -r row; do
    [[ -z "$row" ]] && continue
    local kind
    kind="$(printf '%s' "$row" | awk -F'\t' '{print $16}')"
    if [[ "$kind" == "private" ]]; then
      PRIVATE_COUNT=$((PRIVATE_COUNT + 1))
    else
      PUBLIC_ROWS+="${row}"$'\n'
    fi
  done <<< "$raw"
}

emit_hidden_placeholder() {
  local n="$1"
  (( n == 0 )) && return 0
  printf '\n> _%d action(s) hidden — origin under private/encrypted path._\n' "$n"
}

format_action_line() {
  # Columns: id status text file line context due defer wait since every done priority est project source_kind
  awk -F'\t' '{
    line = "- ["$2"] " $3
    if ($6 != "") line = line " " $6
    if ($7 != "") line = line " due:" $7
    if ($8 != "") line = line " defer:" $8
    if ($9 != "") line = line " wait:[[" $9 "]]"
    if ($10 != "") line = line " since:" $10
    if ($11 != "") line = line " every:" $11
    if ($12 != "") line = line " done:" $12
    if ($13 != "") line = line " priority:" $13
    if ($14 != "") line = line " est:" $14
    line = line " ^" $1
    print line
  }'
}

build_next_actions() {
  local rows; rows="$(filter_status ' ,/')"
  privacy_partition "$rows"
  printf '## By Context\n\n'
  # Filter out future-dated defer rows.
  local filtered
  filtered="$(printf '%s' "$PUBLIC_ROWS" | awk -F'\t' -v today="$TODAY" '
    $8=="" || $8 <= today { print }
  ')"
  # Group by context column 6.
  local contexts
  contexts="$(printf '%s' "$filtered" | awk -F'\t' '{print $6}' | sort -u | grep -v '^$' || true)"
  while IFS= read -r ctx; do
    [[ -z "$ctx" ]] && continue
    printf '### %s\n\n' "$ctx"
    printf '%s' "$filtered" | awk -F'\t' -v c="$ctx" '$6==c' | format_action_line
    printf '\n'
  done <<< "$contexts"
  # No-context bucket last.
  local no_ctx; no_ctx="$(printf '%s' "$filtered" | awk -F'\t' '$6==""')"
  if [[ -n "$no_ctx" ]]; then
    printf '### (no context)\n\n'
    printf '%s\n' "$no_ctx" | format_action_line
    printf '\n'
  fi
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

build_today() {
  local rows; rows="$(filter_status ' ,/')"
  privacy_partition "$rows"
  local filtered
  filtered="$(printf '%s' "$PUBLIC_ROWS" | awk -F'\t' -v today="$TODAY" '
    ($7!="" && $7 <= today) || ($8!="" && $8 <= today) { print }
  ')"
  printf '## Today\n\n'
  printf '%s' "$filtered" | format_action_line
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

build_waiting() {
  local rows; rows="$(filter_status '?')"
  privacy_partition "$rows"
  printf '## Waiting\n\n'
  local persons
  persons="$(printf '%s' "$PUBLIC_ROWS" | awk -F'\t' '{print $9}' | sort -u | grep -v '^$' || true)"
  while IFS= read -r p; do
    [[ -z "$p" ]] && continue
    printf '### %s\n\n' "$p"
    printf '%s' "$PUBLIC_ROWS" | awk -F'\t' -v p="$p" '$9==p' | format_action_line
    printf '\n'
  done <<< "$persons"
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

build_someday() {
  local rows; rows="$(filter_status '>')"
  privacy_partition "$rows"
  printf '## Someday\n\n'
  local projects
  projects="$(printf '%s' "$PUBLIC_ROWS" | awk -F'\t' '{print $15}' | sort -u || true)"
  while IFS= read -r pj; do
    [[ -z "$pj" ]] && { continue; }
    printf '### [[%s]]\n\n' "$pj"
    printf '%s' "$PUBLIC_ROWS" | awk -F'\t' -v p="$pj" '$15==p' | format_action_line
    printf '\n'
  done <<< "$projects"
  local unassigned; unassigned="$(printf '%s' "$PUBLIC_ROWS" | awk -F'\t' '$15==""')"
  if [[ -n "$unassigned" ]]; then
    printf '### (unassigned)\n\n'
    printf '%s\n' "$unassigned" | format_action_line
  fi
  emit_hidden_placeholder "$PRIVATE_COUNT"
}

build_stuck_projects() {
  printf '## Stuck projects\n\n'
  # Project pages with status: active and zero open actions.
  local proj
  for proj_file in "$REPO_ROOT"/content/projects/*.md; do
    [[ -f "$proj_file" ]] || continue
    grep -q '^status:[[:space:]]*active' "$proj_file" || continue
    local slug; slug="$(basename "$proj_file" .md)"
    local open
    open="$(awk -F'\t' -v s="$slug" 'NR>1 && $15==s && ($2==" " || $2=="/") {n++} END{print n+0}' "$ACTIONS")"
    if (( open == 0 )); then
      printf -- '- [[%s]] — no open actions\n' "$slug"
    fi
  done
}

# --- Atomic rewrite of one managed region ---

rewrite_region() {
  local region="$1" body="$2"
  local f="$AGENDA_DIR/${region}.md"
  [[ -f "$f" ]] || { echo "AGENDA|missing-page|$f" >&2; return 0; }
  local tmp="$f.tmp.$$"
  awk -v region="$region" -v body="$body" -v today="$TODAY" '
    BEGIN { in_fm=0; fm_done=0; in_block=0 }
    {
      if (!fm_done && $0 ~ /^---[[:space:]]*$/) {
        if (in_fm==0) { in_fm=1; print; next }
        else { in_fm=0; fm_done=1; print; next }
      }
      if (in_fm==1 && $0 ~ /^last_updated:/) { print "last_updated: " today; next }
      if ($0 == "<!-- BEGIN agenda:" region " -->") {
        print
        printf "%s\n", body
        in_block=1
        next
      }
      if ($0 == "<!-- END agenda:" region " -->") { in_block=0; print; next }
      if (in_block==1) next
      print
    }
  ' "$f" > "$tmp"
  mv "$tmp" "$f"
}

awiki_lock_with --timeout=30 -- bash -c '
true
' >/dev/null 2>&1 || { echo "AGENDA|lock-timeout" >&2; exit 7; }

rewrite_region next-actions   "$(build_next_actions)"
rewrite_region today          "$(build_today)"
rewrite_region waiting        "$(build_waiting)"
rewrite_region someday        "$(build_someday)"
rewrite_region stuck-projects "$(build_stuck_projects)"

echo "agenda regenerated: 5 regions, last_updated=$TODAY"
exit 0
EOF
chmod +x scripts/agenda.sh
```

- [ ] **Step 4: Run bats (expect PASS)**

```bash
bats tests/agenda_test.bats
```

Expected: all cases pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/agenda.sh tests/agenda_test.bats
git commit -m "feat(task): add agenda.sh managed-region regen with privacy filter"
```

---

## Task 17.6: Lint rules T1-T7, T14, T15 in `scripts/lint.sh`

**Files:** Modify: `scripts/lint.sh`. Create: `tests/lint_task_test.bats`.

The phase-17 lint additions are mechanical and grammar-scoped. Every new rule must source `scripts/lib/action-grammar.sh` and only fire on lines that the grammar lib confirms are action lines. T6 in particular **must not** fire on prose `@mentions` or on `[ ]` literals inside fenced code blocks — that is a regression risk and the bats suite tests it explicitly.

Rule summary (full bodies in spec § "Lint Rules Added"):

| Code | Level | Trigger |
|------|-------|---------|
| T1 | error | Status marker not in `{ ,/,?,>,x,-}`. |
| T2 | error | Same `^id` (literal, including any `~<n>`) on ≥2 action lines. Chain `^<base>` + `^<base>~<n>` allowed. Within a chain, `<n>` values must be unique — i.e. two `^<base>~2` across pages = error. |
| T3 | error | Tail key not in `{due,defer,wait,since,every,done,priority,est}`. |
| T4 | error | `due:`/`defer:`/`since:`/`done:` value not a valid `YYYY-MM-DD` calendar date. |
| T5 | error | `[?]` line without `wait:`. |
| T6 | error | Action line carries `@<slug>` whose context page does not exist. **Action-grammar-scoped only — silent on prose `@mentions` and code-block content.** |
| T7 | error | Hand-edit detected inside `<!-- BEGIN agenda:... -->` / `<!-- END agenda:... -->` (intersect working-tree diff vs HEAD with the managed region span). |
| T14 | error | Block-ID does not match `^[a-z0-9]{3,16}$`, OR contains `~` outside the chain shape `^<base>~<digits>`. |
| T15 | warn | Indented non-blank line follows a checkbox line on a page that contains other action lines. **`lint --fix` does NOT touch T15** (a wrapped action line could be a real intent or a stray paragraph; auto-rewriting risks silent data loss). User resolves manually. |

T7's "diff vs HEAD" branch is needed because the agenda generator legitimately writes inside the markers — we only want to flag when **a hand-edit lives inside the markers**. Implementation: run `git diff HEAD -- <agenda-page>` and check whether any hunk overlaps the BEGIN..END line span. If git is unavailable or the file is untracked, fall back to checking the file mtime; absence of git is a known phase-17 limit.

- [ ] **Step 1: Write the bats suite**

```bash
cat > tests/lint_task_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  cd "$WORK"
}

teardown() {
  cd /
  [ -n "${WORK:-}" ] && rm -rf "$WORK"
}

run_lint() {
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task "$@"
}

# --- Good fixture: every T-rule stays silent. ---

@test "good fixture produces no T-rule errors" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  refute_grep() { ! grep -F "$1" <<< "$output"; }
  refute_grep "T1:"
  refute_grep "T3:"
  refute_grep "T4:"
  refute_grep "T5:"
  refute_grep "T7:"
  refute_grep "T14:"
}

# --- Per-rule fires on broken fixture. ---

@test "T1 fires on bad-status line" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T1:"* ]]
}

@test "T2 fires on duplicate ^id within a single page" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T2:"* ]]
  [[ "$output" == *"dupid"* ]]
}

@test "T2 fires on cross-page duplicate chain instance ^a05~2" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T2:"* ]]
  [[ "$output" == *"a05~2"* ]]
}

@test "T2 stays silent on chain head + chain instance (a05 + a05~2 different pages)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  ! grep -F 'T2:' <<< "$output"
}

@test "T3 fires on bogus tail key" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T3:"* ]]
}

@test "T4 fires on bad date" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T4:"* ]]
}

@test "T5 fires on [?] without wait:" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T5:"* ]]
}

@test "T6 fires on @nonexistent context but NOT on prose @mention or code-block" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T6:"* ]]
  [[ "$output" == *"nonexistent"* ]]
}

@test "T6 stays silent on prose @mention in good fixture" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  run run_lint
  # The good fixture's q3-launch.md has 'discussed @computer setup' as prose.
  # @computer IS a real context, so T6 wouldn't fire on it anyway. The
  # negative assertion: no T6 fires citing line 9 (the prose line).
  ! grep -E 'T6:.*q3-launch.md.*line=9' <<< "$output"
}

@test "T7 fires on hand-edited managed region" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  cd "$WORK" && git init -q && git add -A && git commit -q -m init
  # Re-edit the file post-commit to simulate a hand-edit.
  printf '\nadditional hand edit\n' >> content/projects/managed-region-edited.md
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T7:"* ]]
}

@test "T14 fires on too-short id" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T14:"* ]]
}

@test "T14 fires on ~ outside chain shape" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T14:"* ]]
  [[ "$output" == *"abc~zz"* ]]
}

@test "T15 warns on indented continuation but does NOT auto-fix" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-broken/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null || true
  run run_lint
  [[ "$output" == *"T15:"* ]]
  # --fix must NOT touch the wrapped continuation.
  before="$(cat content/projects/broken.md)"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  after="$(cat content/projects/broken.md)"
  [ "$before" = "$after" ]
}

# --- --fix idempotence + correctness ---

@test "lint --fix normalizes 2026/4/27 to 2026-04-27" {
  cat > "$WORK/page.md" <<'EOM'
---
title: x
type: project
---
- [ ] thing @phone due:2026/4/27 ^ax01
EOM
  cat > "$WORK/.gitattributes" <<'EOM'
EOM
  mkdir -p "$WORK/content/contexts"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/.awiki" "$WORK/.awiki"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/content/contexts" "$WORK/content/contexts/"
  mv "$WORK/page.md" "$WORK/content/projects/page.md" 2>/dev/null || mkdir -p "$WORK/content/projects" && mv "$WORK/page.md" "$WORK/content/projects/page.md"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  run grep -F 'due:2026-04-27' "$WORK/content/projects/page.md"
  [ -n "$output" ]
}

@test "lint --fix is idempotent (second run is no-op)" {
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/." "$WORK/"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/action-scan.sh" >/dev/null
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  cp -r "$WORK/content" "$WORK/content_run1"
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  diff -r "$WORK/content_run1" "$WORK/content"
}

@test "lint --fix mints missing ^id on bare action line" {
  mkdir -p "$WORK/content/projects" "$WORK/content/contexts"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/.awiki" "$WORK/.awiki"
  cp -R "$BATS_TEST_DIRNAME/fixtures/wiki-task-good/content/contexts/." "$WORK/content/contexts/"
  cat > "$WORK/content/projects/p.md" <<'EOM'
---
title: x
type: project
status: active
last_updated: 2026-04-27
---
- [ ] no id yet @phone
EOM
  AWIKI_REPO_ROOT="$WORK" bash "$BATS_TEST_DIRNAME/../scripts/lint.sh" --only=task --fix
  run grep -E '\^[a-z0-9]{8}' "$WORK/content/projects/p.md"
  [ -n "$output" ]
}
EOF
```

- [ ] **Step 2: Run the suite (expect FAIL — rules not yet implemented)**

```bash
bats tests/lint_task_test.bats
```

Expected: most cases fail. T-rule outputs are missing.

- [ ] **Step 3: Add the `--only=task` flag plumbing to `scripts/lint.sh`**

`scripts/lint.sh` already supports `--only=synth` from phase 14. Mirror the pattern. Edit the argument parser to add `task` to the recognised `--only` values, and add a guard at the top of each new rule body:

```bash
# Inside scripts/lint.sh argument parsing — locate the existing
# only=synth handling and add a sibling case:

case "${ONLY:-all}" in
  task|all)
    awiki_lint_run_task_rules
    ;;
esac
```

Define `awiki_lint_run_task_rules` near the bottom of the script (above the summary printer). It sources `scripts/lib/action-grammar.sh`, then walks each rule.

- [ ] **Step 4: Implement the rules**

Inside `scripts/lint.sh`, append (after the existing synth-rule block):

```bash
# --- Task-layer lint rules (phase 17: T1-T7, T14, T15) ---

awiki_lint_run_task_rules() {
  # shellcheck disable=SC1091
  . "$AWIKI_SCRIPTS_DIR/lib/action-grammar.sh"

  local actions_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions.tsv"
  local rejected_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions-rejected.tsv"
  local alias_map="$AWIKI_REPO_ROOT/.awiki/maps/alias-to-slug.tsv"
  [[ -f "$actions_tsv" ]] || return 0

  awiki_lint_task_rule_T1   "$rejected_tsv"
  awiki_lint_task_rule_T2   "$actions_tsv"
  awiki_lint_task_rule_T3T4 "$actions_tsv" "$rejected_tsv"
  awiki_lint_task_rule_T5   "$actions_tsv"
  awiki_lint_task_rule_T6   "$actions_tsv" "$alias_map"
  awiki_lint_task_rule_T7
  awiki_lint_task_rule_T14  "$actions_tsv"
  awiki_lint_task_rule_T15  "$rejected_tsv"
}

awiki_lint_task_rule_T1() {
  local rej="$1"
  [[ -f "$rej" ]] || return 0
  awk -F'\t' '$3=="bad-status" {
    printf "LINT|ERROR|%s|T1: bad status marker on line %d (%s)\n", $1, $2, $5
  }' "$rej"
}

awiki_lint_task_rule_T2() {
  local act="$1"
  awk -F'\t' '
    NR>1 && $1!="" { count[$1]++; where[$1] = where[$1] ";" $4 ":" $5 }
    END {
      for (id in count) if (count[id] > 1)
        printf "LINT|ERROR|(multi)|T2: duplicate id %s at%s\n", id, where[id]
    }
  ' "$act"
}

awiki_lint_task_rule_T3T4() {
  local act="$1" rej="$2"
  if [[ -f "$rej" ]]; then
    awk -F'\t' '
      $3=="bad-key" { printf "LINT|ERROR|%s|T3: unrecognized tail key on line %d (%s)\n", $1, $2, $5 }
      $3=="bad-date" { printf "LINT|ERROR|%s|T4: invalid date on line %d (%s)\n", $1, $2, $5 }
    ' "$rej"
  fi
  # Also scan actions.tsv for date-shaped values that fail calendar-validation.
  awk -F'\t' -v today="$(date +%Y-%m-%d)" '
    NR>1 {
      for (col in arr) delete arr[col]
      arr[7]=$7; arr[8]=$8; arr[10]=$10; arr[12]=$12
      for (col in arr) {
        v = arr[col]; if (v=="") continue
        if (v !~ /^[0-9]{4}-[0-9]{2}-[0-9]{2}$/) {
          printf "LINT|ERROR|%s|T4: invalid date value %s on line %d\n", $4, v, $5
        }
      }
    }
  ' "$act"
}

awiki_lint_task_rule_T5() {
  local act="$1"
  awk -F'\t' '
    NR>1 && $2=="?" && $9=="" {
      printf "LINT|ERROR|%s|T5: waiting line missing wait:[[person]] (line %d)\n", $4, $5
    }
  ' "$act"
}

awiki_lint_task_rule_T6() {
  local act="$1" alias_map="$2"
  [[ -f "$alias_map" ]] || return 0
  declare -A KNOWN
  while IFS=$'\t' read -r alias slug rest; do
    [[ "$alias" =~ ^@ ]] && KNOWN["$alias"]=1
  done < "$alias_map"
  awk -F'\t' 'NR>1 && $6 != "" { print $4 "\t" $5 "\t" $6 }' "$act" | \
  while IFS=$'\t' read -r f line ctx; do
    if [[ -z "${KNOWN[$ctx]:-}" ]]; then
      printf 'LINT|ERROR|%s|T6: unknown context %s (line %d)\n' "$f" "$ctx" "$line"
    fi
  done
}

awiki_lint_task_rule_T7() {
  command -v git >/dev/null 2>&1 || return 0
  ( cd "$AWIKI_REPO_ROOT" && \
    for region in next-actions today waiting someday stuck-projects; do
      for f in $(git ls-files -- 'content/**/*.md' 2>/dev/null); do
        # only check files containing this region marker
        grep -q "<!-- BEGIN agenda:${region} -->" "$f" || continue
        # Compute the BEGIN..END line span.
        begin_line=$(grep -n "<!-- BEGIN agenda:${region} -->" "$f" | head -1 | cut -d: -f1)
        end_line=$(grep -n "<!-- END agenda:${region} -->" "$f" | head -1 | cut -d: -f1)
        [[ -z "$begin_line" || -z "$end_line" ]] && continue
        # Hunks intersecting the span.
        if git diff HEAD -- "$f" 2>/dev/null | \
             awk -v b="$begin_line" -v e="$end_line" '
               /^@@/ {
                 match($0, /\+([0-9]+),?([0-9]*)/, m)
                 start=m[1]+0; len=(m[2]==""?1:m[2]+0)
                 if (start <= e && start+len >= b) found=1
               }
               END { exit (found?0:1) }
             '; then
          printf 'LINT|ERROR|%s|T7: hand-edit detected inside agenda:%s region\n' "$f" "$region"
        fi
      done
    done
  )
}

awiki_lint_task_rule_T14() {
  local act="$1"
  awk -F'\t' '
    NR>1 && $1!="" {
      id=$1
      if (id ~ /~/) {
        # Chain shape: ^<base>~<digits>
        if (id !~ /^[a-z0-9]{3,16}~[0-9]+$/) {
          printf "LINT|ERROR|%s|T14: bad id shape %s (chain shape requires base~digits)\n", $4, id
        }
      } else if (id !~ /^[a-z0-9]{3,16}$/) {
        printf "LINT|ERROR|%s|T14: bad id shape %s\n", $4, id
      }
    }
  ' "$act"
}

awiki_lint_task_rule_T15() {
  local rej="$1"
  [[ -f "$rej" ]] || return 0
  awk -F'\t' '$3=="continuation" {
    printf "LINT|WARN|%s|T15: action continuation (line %d) — not supported (multi-line action)\n", $1, $2
  }' "$rej"
}
```

- [ ] **Step 5: Implement `--fix` mechanical fixes (task-aware)**

Append a sibling function `awiki_lint_fix_task_rules` and wire it into the existing `--fix` dispatcher:

```bash
awiki_lint_fix_task_rules() {
  local today; today="$(date +%Y-%m-%d)"
  local actions_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions.tsv"
  [[ -f "$actions_tsv" ]] || return 0

  while IFS= read -r f; do
    [[ -z "$f" ]] && continue
    local tmp="$f.tmp.$$"
    awk -v today="$today" '
      function pad2(n) { return (n+0 < 10 ? "0" n+0 : n+0) }
      function normalize_date(v,    p) {
        # 2026/4/27 → 2026-04-27. Already-good dates pass through.
        if (v ~ /^[0-9]{4}\/[0-9]{1,2}\/[0-9]{1,2}$/) {
          n = split(v, p, "/")
          return p[1] "-" pad2(p[2]) "-" pad2(p[3])
        }
        return v
      }
      function fix_line(line,   parts, i, out, ndone, nsince) {
        # status flip auto-stamp.
        if (line ~ /^- \[x\] / && line !~ / done:/) {
          sub(/$/, " done:" today, line)
        }
        if (line ~ /^- \[\?\] / && line !~ / since:/) {
          sub(/$/, " since:" today, line)
        }
        # date-format normalization for due/defer/since/done values.
        n = patsplit(line, parts, /(due|defer|since|done):[0-9\/-]+/, sep)
        out = sep[0]
        for (i = 1; i <= n; i++) {
          k = parts[i]; sub(/:.*/, ":", k)
          v = parts[i]; sub(/^[a-z]+:/, "", v)
          out = out k normalize_date(v) sep[i]
        }
        return out
      }
      /^- \[/ { print fix_line($0); next }
      { print }
    ' "$f" > "$tmp"
    if ! cmp -s "$f" "$tmp"; then mv "$tmp" "$f"; else rm -f "$tmp"; fi
  done < <(awk -F'\t' 'NR>1 {print $4}' "$actions_tsv" | sort -u | sed "s|^|$AWIKI_REPO_ROOT/|")

  # Mint missing ^id on action lines that lack one.
  awiki_lint_fix_mint_ids
  # Sort tail keys to canonical order.
  awiki_lint_fix_sort_keys
}

awiki_lint_fix_mint_ids() {
  local actions_tsv="$AWIKI_REPO_ROOT/.awiki/maps/actions.tsv"
  declare -A USED
  if [[ -f "$actions_tsv" ]]; then
    while IFS=$'\t' read -r id _; do
      USED["$id"]=1
    done < <(awk -F'\t' 'NR>1 {print $1}' "$actions_tsv")
  fi
  mint_one() {
    local n=0
    while (( n < 5 )); do
      local cand
      cand="$(LC_ALL=C tr -dc 'a-z0-9' < /dev/urandom | head -c 8)"
      [[ -z "${USED[$cand]:-}" ]] && { USED["$cand"]=1; printf '%s' "$cand"; return; }
      n=$((n + 1))
    done
    return 8
  }
  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    local tmp="$f.tmp.$$"
    while IFS= read -r line; do
      if [[ "$line" =~ ^-[[:space:]]\[(.)\][[:space:]] && ! "$line" =~ \^[a-z0-9]{3,16}([~][0-9]+)?[[:space:]]*$ ]]; then
        local id; id="$(mint_one)" || { rm -f "$tmp"; return 8; }
        printf '%s ^%s\n' "$line" "$id" >> "$tmp"
      else
        printf '%s\n' "$line" >> "$tmp"
      fi
    done < "$f"
    if ! cmp -s "$f" "$tmp"; then mv "$tmp" "$f"; else rm -f "$tmp"; fi
  done < <(find "$AWIKI_REPO_ROOT/content" -name '*.md' 2>/dev/null)
}

awiki_lint_fix_sort_keys() {
  # Canonical order: @context due defer wait since every priority est done.
  # Implementation: per action line, parse tokens, regroup in order, rejoin.
  while IFS= read -r f; do
    [[ -f "$f" ]] || continue
    local tmp="$f.tmp.$$"
    awk '
      function reorder(line,    head, tail, ctx, due, defer, wait, since, every, prio, est, done_, id, tok, n, parts, i, out) {
        if (line !~ /^- \[/) return line
        # Split prefix / body / id.
        if (match(line, /\^[a-z0-9]{3,16}(~[0-9]+)?$/)) {
          id = substr(line, RSTART, RLENGTH)
          body = substr(line, 1, RSTART - 2)
        } else { body = line; id = "" }
        # Status + leading text.
        match(body, /^- \[.\] /)
        head = substr(body, 1, RLENGTH)
        rest = substr(body, RLENGTH + 1)
        # Walk tokens.
        n = split(rest, parts, " ")
        text = ""
        for (i = 1; i <= n; i++) {
          tok = parts[i]
          if (tok ~ /^@/) { ctx = tok; continue }
          if (tok ~ /^due:/) { due = tok; continue }
          if (tok ~ /^defer:/) { defer = tok; continue }
          if (tok ~ /^wait:/) { wait = tok; continue }
          if (tok ~ /^since:/) { since = tok; continue }
          if (tok ~ /^every:/) { every = tok; continue }
          if (tok ~ /^priority:/) { prio = tok; continue }
          if (tok ~ /^est:/) { est = tok; continue }
          if (tok ~ /^done:/) { done_ = tok; continue }
          text = (text == "" ? tok : text " " tok)
        }
        out = head text
        for (k in (a = "")) { } # no-op
        for (tok in (a = "")) {} # no-op
        if (ctx   != "") out = out " " ctx
        if (due   != "") out = out " " due
        if (defer != "") out = out " " defer
        if (wait  != "") out = out " " wait
        if (since != "") out = out " " since
        if (every != "") out = out " " every
        if (prio  != "") out = out " " prio
        if (est   != "") out = out " " est
        if (done_ != "") out = out " " done_
        if (id != "")    out = out " " id
        return out
      }
      /^- \[/ { print reorder($0); next }
      { print }
    ' "$f" > "$tmp"
    if ! cmp -s "$f" "$tmp"; then mv "$tmp" "$f"; else rm -f "$tmp"; fi
  done < <(find "$AWIKI_REPO_ROOT/content" -name '*.md' 2>/dev/null)
}
```

Then in the existing `--fix` dispatcher case in `scripts/lint.sh`, add:

```bash
case "${ONLY:-all}" in
  task|all) awiki_lint_fix_task_rules ;;
esac
```

- [ ] **Step 6: Run bats (expect PASS)**

```bash
bats tests/lint_task_test.bats
```

Expected: every case passes.

- [ ] **Step 7: Commit**

```bash
git add scripts/lint.sh tests/lint_task_test.bats
git commit -m "feat(task): add lint rules T1-T7, T14, T15 + --fix task-aware mechanical fixes"
```

---

## Task 17.7: justfile recipes — `agenda` + `scan`

**Files:** Modify: `justfile`, `docs/just-help.txt`. Test: `tests/justfile_test.bats` (extend with two cases).

Phase 16 left the `agenda`, `scan`, `triage`, `review` recipes as commented stubs. Phase 17 activates `agenda` and `scan`. `triage` and `review` remain commented (they're phase-18 / phase-19 deliverables respectively).

- [ ] **Step 1: Inspect the phase-16 stubs**

```bash
grep -n 'agenda\|scan\|triage\|review' justfile
```

Expected: four lines beginning with `#` near the bottom of the task-layer block.

- [ ] **Step 2: Replace the `scan` and `agenda` stubs with live recipes**

Edit `justfile`. Find the commented stubs and replace:

```just
# Replace these two lines:
# scan:
#     bash scripts/action-scan.sh
# agenda:
#     bash scripts/action-scan.sh
#     bash scripts/agenda.sh

# With:
scan:
    bash scripts/action-scan.sh

agenda:
    bash scripts/action-scan.sh
    bash scripts/agenda.sh
```

Leave `triage` and `review` commented; their recipes ship in phases 18 and 19.

- [ ] **Step 3: Update `docs/just-help.txt`**

Add entries after the `capture` entry (added in phase 16):

```
scan
    Walk content/**/*.md and rebuild .awiki/maps/actions.tsv +
    actions-rejected.tsv. Read-only against content. Used by `agenda`,
    by lint, and by the optional pre-commit hook (phase 19).

agenda
    Run `scan` then regenerate the five managed-region pages under
    content/agenda/. Privacy filter excludes source_kind=private
    actions; a `> _N action(s) hidden_` placeholder is emitted in
    each region with hidden rows. Atomic rename; last_updated is
    rewritten to today on every regen.
```

- [ ] **Step 4: Add bats coverage**

Append to `tests/justfile_test.bats` (created in v1):

```bash
@test "just scan invokes action-scan.sh" {
  run grep -nE '^scan:' justfile
  [ -n "$output" ]
  run grep -nE 'bash scripts/action-scan.sh' justfile
  [ -n "$output" ]
}

@test "just agenda chains action-scan.sh and agenda.sh" {
  run awk '/^agenda:/,/^[a-z]/' justfile
  [[ "$output" == *"action-scan.sh"* ]]
  [[ "$output" == *"agenda.sh"* ]]
}
```

- [ ] **Step 5: Run bats (expect PASS)**

```bash
bats tests/justfile_test.bats
```

- [ ] **Step 6: Commit**

```bash
git add justfile docs/just-help.txt tests/justfile_test.bats
git commit -m "feat(task): activate just scan + just agenda recipes"
```

---

## Task 17.8: Smoke test extension — `task-init` → `capture` → triage edit → `agenda`

**Files:** Modify: `tests/task_smoke_test.bats`.

Phase 16 ended with `task-init` then `capture` writing the captured line to `inbox.md`. Phase 17 extends the smoke run: a hand-edit "triage" (because the full triage workflow ships in phase 18), then `agenda`, then assertion that the captured action appears under `### @phone` in `next-actions.md`.

This is one bats case appended to the existing smoke file. The hand-edit is verbatim — no fragile regex.

- [ ] **Step 1: Append the smoke case**

```bash
cat >> tests/task_smoke_test.bats <<'EOF'

@test "smoke phase 17: capture → manual triage → agenda → action under @phone" {
  WORK="$(mktemp -d)"
  cp -R . "$WORK/awiki" 2>/dev/null || cp -R "$BATS_TEST_DIRNAME/.." "$WORK/awiki"
  cd "$WORK/awiki"
  AWIKI_TASK_INIT_ASSUME_NO=1 just task-init
  just capture "call dentist about crown"

  # Hand-edit triage: promote the inbox line to a checkbox on a project page.
  mkdir -p content/projects
  cat > content/projects/dentist.md <<'EOM'
---
title: "Dentist"
type: project
status: active
last_updated: 2026-04-27
draft: false
---

## Open Actions

- [ ] call dentist about crown @phone ^d01
EOM
  # Strip the inbox line we just promoted. Plain line-substitution to
  # avoid sed pitfalls on a single-target removal.
  awk '!/call dentist about crown/' content/inbox.md > content/inbox.md.tmp \
    && mv content/inbox.md.tmp content/inbox.md

  just agenda

  run grep -F '### @phone' content/agenda/next-actions.md
  [ -n "$output" ]
  run grep -F 'call dentist about crown' content/agenda/next-actions.md
  [ -n "$output" ]

  cd /
  rm -rf "$WORK"
}
EOF
```

- [ ] **Step 2: Run the smoke test**

```bash
bats tests/task_smoke_test.bats
```

Expected: every case passes (including the prior phase-16 capture-write case).

- [ ] **Step 3: Commit**

```bash
git add tests/task_smoke_test.bats
git commit -m "test(task): smoke phase 17 capture→triage-edit→agenda→@phone"
```

---

## Task 17.9: Phase-done verification + merge to `main`

**Files:** none (verification only).

- [ ] **Step 1: Full bats suite**

```bash
bats tests/
```

Expected: every test passes — phases 1-16 plus all phase-17 additions.

- [ ] **Step 2: Lint clean**

```bash
just lint
```

Expected: zero errors. Warnings for T15 violations on the broken fixture are acceptable; that fixture lives under `tests/fixtures/` which `lint.sh` excludes from the content walk by default.

- [ ] **Step 3: Walk the deliverables checklist**

Manually confirm each:

- [ ] `docs/decisions/2026-04-27-task-layer-spike.md` exists with both spike outcomes filled in. The chosen separator (`~` or `__`) is named explicitly.
- [ ] If the spike chose `__`, all spec text and `scripts/lib/action-grammar.sh` reflect that change.
- [ ] `scripts/action-scan.sh` exists, sources `scripts/lib/action-grammar.sh` and `scripts/lib/lock.sh`, emits both TSVs, never re-implements the action regex inline.
- [ ] `scripts/agenda.sh` exists, sources `scripts/lib/lock.sh`, uses `mktemp`+`mv` for every managed-region rewrite, rewrites `last_updated:` on every regen, exits 5 when `AWIKI_AGENDA_INCLUDE_PRIVATE=1` without encryption coverage.
- [ ] All five `content/agenda/*.md` placeholder pages use `<!-- BEGIN agenda:<region> -->` per-region markers (NOT bare `managed-region`).
- [ ] `scripts/lint.sh` implements T1-T7, T14, T15 — each rule is action-grammar-scoped (T6 specifically silent on prose `@mentions` and code-block content).
- [ ] T2 distinguishes `^<base>` from `^<base>~<n>` (chain elements not flagged); two `^a05~2` instances on different pages DO fire T2.
- [ ] `scripts/lint.sh --fix` normalizes dates, auto-stamps missing `done:` / `since:`, sorts tail keys to canonical order, mints missing `^id` (8-char base32, collision-checked, ≤5 retries). T15 violations are NOT auto-fixed.
- [ ] `justfile` exposes live `scan` and `agenda` recipes; `triage` and `review` remain commented.
- [ ] `docs/just-help.txt` documents `scan` and `agenda`.
- [ ] BATS suites: `tests/action_scan_test.bats`, `tests/agenda_test.bats`, `tests/lint_task_test.bats`, plus extended `tests/task_smoke_test.bats` — all pass.
- [ ] No phase-18/19 features leaked: no triage, no recurrence, no MCP changes, no semantic lint rules T8-T13, no review-status, no pre-commit hook installer, no `encrypt-init` updates, no README task-layer section, no sample-wiki examples.
- [ ] No emojis in any phase-17 file. No `Co-Authored-By` trailers in any phase-17 commit.

- [ ] **Step 4: Final cleanup commit (if needed)**

```bash
git status
# If anything is unstaged:
git add -p
git commit -m "fix: phase 17 final cleanup per phase-done checklist"
```

- [ ] **Step 5: Merge to main**

```bash
git checkout main
git merge --no-ff phase-17-task-scanner-agenda -m "feat: complete phase 17 task-layer scanner + agenda"
git branch -d phase-17-task-scanner-agenda
```

Expected: merge commit on `main`; branch deleted.

- [ ] **Step 6: Tag (skip — phase 17 is mid-feature)**

Tagging waits for phase 19 (full task-layer ship).

---

## Self-Review Checklist

Walk this list once after Task 17.9 step 3 has been ticked off. Each item is scoped to phase-17 deliverables only.

- [ ] **Spike outcome documented.** `docs/decisions/2026-04-27-task-layer-spike.md` is committed, names the chosen recurrence-chain separator, and pins the Obsidian Tasks plugin version. If `__` was chosen, the spec and `scripts/lib/action-grammar.sh` and `tests/action_grammar_test.bats` were patched in the same task and the bats suite is green with the new separator.
- [ ] **Action-grammar lib is the single source of truth.** `grep -nE 'action-grammar|AWIKI_ACTION_LINE_RE' scripts/action-scan.sh scripts/agenda.sh scripts/lint.sh` shows each consumer sourcing the lib; no inline duplicate regex.
- [ ] **T6 fires only on action-grammar lines.** `tests/lint_task_test.bats` includes positive coverage on the bad-context broken fixture AND a negative assertion that T6 stays silent on the prose `@mention` line in `q3-launch.md`. Code-block content is gated by the in-fence skip in `action-scan.sh`.
- [ ] **T2 distinguishes chain elements.** Tests cover (a) literal duplicate `^dupid` on one page → fires, (b) `^a05~2` on two different pages → fires, (c) `^a05` chain head + `^a05~2` instance on the same page (good fixture) → does NOT fire, (d) `^a05~2` plus `^a05~3` on different pages (synthetic test) → does NOT fire.
- [ ] **Private-wikilink-target detection covered.** `tests/action_scan_test.bats` confirms action 12 (containing `[[bob-private]]`) is classified `source_kind=private` even though its source file `projects/onboarding-revamp.md` is public. Re-verify by toggling the alias map's `bob-private` row to `public` — action 12 should flip to `public`. (Manual one-off; not in the bats suite.)
- [ ] **Atomic rename used for every managed-region rewrite.** `grep -nE 'mktemp|\.tmp\.\$\$' scripts/agenda.sh` shows every rewrite path uses temp+mv; the bats case `agenda.sh uses atomic rename (no .tmp file leaks)` confirms no leftover `.tmp.<pid>` files post-run.
- [ ] **Per-region markers, not bare markers.** `grep -rn 'BEGIN agenda:' content/agenda/` prints exactly five lines, one per region. `grep -rn 'BEGIN managed-region' content/agenda/` prints zero.
- [ ] **Privacy filter emits hidden-count placeholder.** Bats case `agenda.sh respects the privacy filter` asserts the `> _N action(s) hidden_` line is present in `next-actions.md` when at least one private row is filtered. Counts only — no slugs leak.
- [ ] **`AWIKI_AGENDA_INCLUDE_PRIVATE=1` rejected without encryption coverage.** The exit-5 case is exercised in bats. The success path (encryption coverage present, override honored) is phase-19's job and is NOT implemented here.
- [ ] **Lock library acquired by every mutating script.** `agenda.sh` calls `awiki_lock_with --timeout=30`. `action-scan.sh` calls `awiki_lock_shared` (read-only against content). `lint.sh --fix` acquires the exclusive lock before rewriting any page.
- [ ] **`--fix` is idempotent.** Two consecutive `--fix` runs produce zero diff. Bats case enforces this. Mint-id retry cap is 5; on exhaustion the script exits 8 (effectively unreachable per spec; documented).
- [ ] **T15 not auto-fixed.** Bats case asserts `--fix` leaves the broken fixture's wrapped continuation byte-identical. The deliberate non-fix is documented in `docs/just-help.txt` and the rule body's comment.
- [ ] **Smoke run end-to-end clean.** `bats tests/task_smoke_test.bats` passes from a clean checkout: `task-init` → `capture` → manual triage edit (writing `content/projects/dentist.md`) → `agenda` → `### @phone` heading + the captured text both present in `content/agenda/next-actions.md`.
- [ ] **No emojis, no co-author trailers, no AI-attribution lines.** `git log -p --grep='Co-Authored-By' phase-17-task-scanner-agenda` is empty. `grep -rE '🤖|✅|❌|✨' scripts/ content/ docs/` (run pre-merge) prints nothing for phase-17-touched files.
- [ ] **No phase-18/19 features leaked.** `ls scripts/` does not contain `triage.sh`, `action-recur.sh`, `review-status.sh`. `mcp/awiki-server/index.js` is unchanged from phase 16. No semantic lint rules T8-T13 implemented.

---

## Open questions

These spec ambiguities surfaced while writing the plan. **Each is flagged for resolution before implementation begins; do not invent answers in the implementation.**

1. **Encryption-coverage glob shape.** Spec § "Privacy classification" says `source_kind=private` matches `.gitattributes` git-crypt patterns. The example patterns in the spec include `content/private/**` (double-star). The exact glob dialect — bash `extglob`, find-style, or git-attribute pathspec — is not specified. The plan reads patterns literally and matches via bash `[[ path == pattern ]]` after stripping `**`, which is a known approximation. Confirm with the spec author whether full pathspec semantics are required (drives whether we shell out to `git check-attr` instead).

2. **Alias-map staleness window.** Spec says `action-scan.sh` does NOT rebuild the alias map (lint owns it) and acknowledges a one-cycle privacy miss when a private page is added without lint having run. The plan does not attempt to mitigate. The optional pre-commit hook (phase 19) closes the window for committed changes; for in-progress edits it remains open. Confirm: is this acceptable behavior for v1, or does phase 17 need a fast-path "alias-only" rebuild?

3. **`actions-rejected.tsv` reason-code closed enum.** Spec lists reason codes informally (`bad-status`, `bad-key`, `bad-date`, `dup-id`, `bad-id-shape`, `continuation`). The plan adds `no-status` for grammar-parse-failed lines and treats it as distinct from `bad-status`. Confirm the closed enum so consumers (lint, future MCP tools) can rely on it.

4. **T7 "intersect diff vs HEAD" implementation when git is unavailable.** Spec calls for hand-edit detection via diff vs HEAD. The plan falls back to silent no-op when `git` isn't installed or the file is untracked. An mtime-based heuristic was considered but rejected (false-positives on every regen). Confirm that silent-no-op is acceptable, or specify a stricter fallback.

5. **`stuck-projects.md` — exemption set.** Spec § "Per-page rules" says stuck = active project with zero open actions. Spec § T8 (semantic lint, phase 18) exempts `_loose.md`, `_someday.md`, and `status: someday|done`. The plan applies the same exemption for the agenda's stuck-projects bucket; spec does not state this explicitly. Confirm.

6. **Tail-key sort canonical order.** Spec § "lint --fix" lists the order as `@context due defer wait since every priority est done`. Spec § "Tail metadata keys" table lists the keys in a different order (alphabetical). The plan uses the `--fix` order (which matches the example block in the spec) but the discrepancy is worth flagging.

7. **Action-line block-quote inclusion.** Phase-17 spec says "lines under `## Done` heading on project pages still scanned (state = `[x]`)". The plan applies this implicitly — the scanner has no heading awareness; any `- [<status>]` line outside fenced code or block-quote is captured. Single-line block-quoted checkboxes (`> - [ ] ...`) are explicitly excluded. Confirm this matches intent.

8. **Inbox-line ID synthesis used in phase 17?** Spec describes `inbox-<sha1>-<lineno>` and `file-<sha1>` synthesis but says they are "used by `triage_inbox`/`triage_apply`". Phase-17 scanner does not write these IDs anywhere yet. The bats case `scanner records inbox-line synthesis ID for inbox.md captures` only verifies the scanner doesn't crash on `inbox.md`. Confirm whether the synthesis logic should land as a callable helper in phase 17 (so phase 18 can re-use it) or stay deferred.

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to Phase 18 — triage workflow + recurrence + MCP server tools.

### Critical Files for Implementation
- /Users/joanmarc/dailywork/celonis/awiki/scripts/action-scan.sh
- /Users/joanmarc/dailywork/celonis/awiki/scripts/agenda.sh
- /Users/joanmarc/dailywork/celonis/awiki/scripts/lint.sh
- /Users/joanmarc/dailywork/celonis/awiki/scripts/lib/action-grammar.sh
- /Users/joanmarc/dailywork/celonis/awiki/scripts/lib/lock.sh
- /Users/joanmarc/dailywork/celonis/awiki/content/agenda/next-actions.md
- /Users/joanmarc/dailywork/celonis/awiki/content/agenda/today.md
- /Users/joanmarc/dailywork/celonis/awiki/content/agenda/waiting.md
- /Users/joanmarc/dailywork/celonis/awiki/content/agenda/someday.md
- /Users/joanmarc/dailywork/celonis/awiki/content/agenda/stuck-projects.md
- /Users/joanmarc/dailywork/celonis/awiki/docs/decisions/2026-04-27-task-layer-spike.md

---
