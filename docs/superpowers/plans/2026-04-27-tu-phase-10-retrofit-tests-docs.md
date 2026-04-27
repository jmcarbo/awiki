# Phase 10 — Retrofit, BATS Matrix, CI E2E, WIKI.md Workflow, Docs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Close out the template-update feature: ship `template-retrofit.sh` for pre-v1 repos; complete the full BATS test matrix from spec including security + recovery cases not yet covered; add an end-to-end CI workflow; document the agent-facing pending-prompts handling in `WIKI.md`; write user-facing `docs/template-update.md`, ADR, `migrations/README.md` author guide, README mention, CHANGELOG release entry. Tag `v1.0.0`.

**Architecture:** Retrofit script detects bootstrapped repos without `.awiki/template.json` and prompts the user (with heuristic suggestions) to provide upstream commit/version, then calls `template-init.sh`. Tests fill remaining gaps in security, recovery, and edge case categories from spec. CI workflow snapshots a v0 fixture, mutates to v1, runs `template-update --apply --non-interactive --print-migrations`, asserts diff matches expected.

**Spec sections:** `Rollout in awiki template / Phase B Retrofit`, `Tests / Security cases / Recovery cases / Edge cases`, `WIKI.md workflow update`, `Doc updates`.

---

## File structure

**Created:**
- `scripts/template-retrofit.sh`
- `scripts/template-step.sh` (alias-runner for `bootstrap-step` recipe).
- `tests/template-update-retrofit.bats`
- `tests/template-update-e2e.sh` (CI runner).
- `.github/workflows/template-update-e2e.yml` (or update existing CI).
- `docs/template-update.md`
- `docs/decisions/template-update.md` (ADR).
- `migrations/README.md` (author guide).

**Modified:**
- `WIKI.md` — add agent-facing pending-prompts workflow.
- `README.md` — mention update story.
- `CHANGELOG.md` — release entry for v1.0.0.

**Depends on:** Phase 09.

---

## Task 1: `template-step.sh` alias runner

**Files:**
- Create: `scripts/template-step.sh`

- [ ] **Step 1: Implement (no separate test — exercised via existing `bootstrap-step` recipe + Phase 08 rerun tests)**

Create `scripts/template-step.sh`:

```bash
#!/usr/bin/env bash
# Alias for: just template-update --rerun-bootstrap-step <id>
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[[ $# -eq 1 ]] || { echo "usage: template-step.sh <id>" >&2; exit 2; }
exec bash "$SCRIPT_DIR/template-update.sh" --rerun-bootstrap-step "$1"
```

`chmod +x scripts/template-step.sh`.

- [ ] **Step 2: Sanity check**

```bash
bash scripts/template-step.sh --help 2>&1 | head -3   # should error nicely (no id given)
```

- [ ] **Step 3: Commit**

```bash
git add scripts/template-step.sh
git commit -m "feat: scripts/template-step.sh aliases --rerun-bootstrap-step"
```

---

## Task 2: `template-retrofit.sh`

**Files:**
- Create: `scripts/template-retrofit.sh`
- Create: `tests/template-update-retrofit.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-retrofit.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT=$(git rev-parse HEAD)
}
teardown() { rm -rf "$TMP"; }

@test "retrofit: no .awiki/template.json -> prompts and runs template-init" {
  cd "$TMP"
  [ ! -f .awiki/template.json ]
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive
  [ "$status" -eq 0 ]
  [ -f .awiki/template.json ]
}

@test "retrofit: existing template.json -> no-op with info" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$COMMIT" >/dev/null
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo url --ref main --version 0.1.0 --commit "$COMMIT" --non-interactive
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "already"
}

@test "retrofit: heuristic suggests wire-awiki-mcp from .awiki/qmd-status" {
  cd "$TMP"
  mkdir -p .awiki
  echo "ok" > .awiki/qmd-status
  run bash "$REPO_ROOT/scripts/template-retrofit.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$COMMIT" \
    --non-interactive --heuristics-only
  echo "$output" | grep -q "install-qmd"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/template-retrofit.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(pwd)"

REPO=""
REF="main"
VERSION=""
COMMIT=""
NON_INTERACTIVE=0
HEURISTICS_ONLY=0

usage() {
  cat <<EOF
usage: template-retrofit.sh --repo <url> --ref <ref> --version <v> --commit <sha>
                            [--non-interactive] [--heuristics-only]

Detects a bootstrapped repo without .awiki/template.json and seeds it.
EOF
  exit 2
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --commit) COMMIT="$2"; shift 2 ;;
    --non-interactive) NON_INTERACTIVE=1; shift ;;
    --heuristics-only) HEURISTICS_ONLY=1; shift ;;
    *) usage ;;
  esac
done

PJ="$REPO_ROOT/.awiki/template.json"
if [[ -f "$PJ" ]]; then
  echo "info: $PJ already exists; nothing to retrofit"
  exit 0
fi

# Heuristics: which steps look already-done?
echo "Detecting already-completed bootstrap steps:"
HEURISTIC_HITS=()
[[ -f "$REPO_ROOT/.awiki/qmd-status" ]] && {
  echo "  - install-qmd: yes (.awiki/qmd-status exists)"
  HEURISTIC_HITS+=("install-qmd")
}
if [[ -f "$REPO_ROOT/.gitattributes" ]] && grep -q "filter=git-crypt" "$REPO_ROOT/.gitattributes"; then
  echo "  - privacy=git-crypt: yes (.gitattributes references git-crypt)"
  HEURISTIC_HITS+=("privacy")
fi
[[ -f "$REPO_ROOT/secrets/age-key.txt" ]] && {
  echo "  - privacy=age: yes (secrets/age-key.txt exists)"
  HEURISTIC_HITS+=("privacy")
}
if grep -q "awiki-server" "$REPO_ROOT/.mcp.json" 2>/dev/null \
   || grep -q "awiki-server" "$REPO_ROOT/.cursor/mcp.json" 2>/dev/null; then
  echo "  - wire-awiki-mcp: yes (mcp config references awiki-server)"
  HEURISTIC_HITS+=("wire-awiki-mcp")
fi

if [[ $HEURISTICS_ONLY -eq 1 ]]; then
  exit 0
fi

[[ -z "$REPO" || -z "$VERSION" || -z "$COMMIT" ]] && usage

# Run template-init.
bash "$SCRIPT_DIR/template-init.sh" --repo "$REPO" --ref "$REF" --version "$VERSION" --commit "$COMMIT"

# If non-interactive, the heuristic hits are recorded as done by template-init's
# "scan BOOTSTRAP.md and record all step IDs" path. (template-init currently records
# every step ID present in BOOTSTRAP.md; that's correct for a freshly-bootstrapped repo.
# For pre-v1 repos that completed only some steps, the user-confirm pass below patches
# the result.)
if [[ $NON_INTERACTIVE -eq 0 ]]; then
  echo "Confirm each step status (y/n/skip):"
  STEPS=$(python3 "$SCRIPT_DIR/_template_helpers/bootstrap_hash.py" list "$REPO_ROOT/BOOTSTRAP.md")
  for SID in $STEPS; do
    HIT=""
    for h in "${HEURISTIC_HITS[@]:-}"; do
      [[ "$h" == "$SID" ]] && HIT=" (heuristic suggests yes)"
    done
    read -r -p "  $SID$HIT [y/n] " ans </dev/tty
    if [[ "$ans" =~ ^[Nn] ]]; then
      # Mark as skipped in template.json by editing.
      python3 -c "
import json
p='$PJ'
d=json.load(open(p))
for s in d.get('bootstrap_steps_done', []):
    if s.get('id') == '$SID':
        s['status'] = 'skipped'
        s['reason'] = 'retrofit: user said no'
        break
json.dump(d, open(p,'w'), indent=2); open(p,'a').write('\n')
"
    fi
  done
fi

echo "info: retrofit complete"
```

`chmod +x scripts/template-retrofit.sh`.

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-retrofit.sh tests/template-update-retrofit.bats
git commit -m "feat: template-retrofit.sh seeds pre-v1 repos with heuristics + confirms"
```

---

## Task 3: WIKI.md agent-facing pending-prompts workflow

**Files:**
- Modify: `WIKI.md`

- [ ] **Step 1: Append section to WIKI.md**

Add to the bottom of `WIKI.md` (or merge into existing Workflows section):

```markdown
## 14. Pending Template Migrations (LLM-assisted)

When a template update stages LLM migrations, prompt files appear under `.awiki/pending-prompts/`. On startup, follow this workflow:

1. List `.awiki/pending-prompts/*.md`. Show each (with `risk:` from frontmatter) to the user.
2. For each prompt:
   - Read its body and the `## Resolved scope` file list.
   - **Surface the full prompt body to the user.** Confirm intent before bulk edits.
   - If `risk: high`: require explicit "I have reviewed" confirmation.
   - If user accepts: perform edits across resolved files; run `just lint` after; commit `chore(template): apply LLM migration <id>`; delete the prompt file; append `{id, status: "applied"}` to `.awiki/template.json.applied_migrations[]`.
   - If user declines: append `{id, status: "skipped", reason: "user declined"}` to `applied_migrations[]`; delete the prompt file.
3. Lint will warn on prompts older than 14 days. Resolve or document why deferred.

Failure to surface prompts to the user before acting violates the Trust Model. Always wait for explicit user confirmation.
```

- [ ] **Step 2: Commit**

```bash
git add WIKI.md
git commit -m "docs: WIKI.md — pending-prompts workflow with user-confirm gate"
```

---

## Task 4: User-facing `docs/template-update.md`

**Files:**
- Create: `docs/template-update.md`

- [ ] **Step 1: Write doc**

Create `docs/template-update.md`:

```markdown
# Template Updates

This wiki was bootstrapped from the [awiki template](https://github.com/jmcarbo/awiki). When the template ships new scripts, MCP server fixes, deploy templates, schema changes, or new bootstrap steps, you can pull them in with `just template-update`.

## Quick start

```bash
# Read pending status (no mutation):
just template-status

# Preview what an update would do (default = dry-run):
just template-update

# Apply onto a dedicated review branch:
just template-update --apply

# Review the branch:
git diff main

# Merge:
git switch main && git merge --no-ff awiki-template-update/<sha>
```

## Trust model

`just template-update` executes scripts shipped from the upstream template (`migrations/*.sh`) and stages instructions for your agent (`migrations/*.prompt.md`). Both are **trusted code** — review before running.

- `--source` defaults to the URL recorded at bootstrap (`.awiki/template.json.repo`). Changing it requires `--accept-source-change` and a confirmation prompt.
- `--print-migrations` shows full migration script bodies in the dry-run plan.
- `--verify-signature` requires the upstream tag/commit be GPG-signed (you provide the trust roots in `~/.gnupg/`).
- Optional: set `require_signature=true` in `.awiki/config` to make signature checks mandatory.

If you don't trust the source, don't run the update.

## Recovery from a bad update

### Soft revert (keep new pin, retry without one migration)

```bash
git revert <merge-commit>
just template-update --skip-migration <id> --apply
```

### Hard re-pin (downgrade)

```bash
git revert -m 1 <merge-commit>
just template-update --re-pin <previous-commit>
git add .awiki/template.json && git commit -m "chore(template): re-pin to <prev-version>"
```

## Multi-machine wikis

Same wiki cloned to multiple machines: pull updates on one machine, push, then `git pull` on the others. The first `just template-update` on a fresh machine auto-recovers `.awiki/template-cache/<pin>/` from upstream — no manual seeding required. If the upstream is unreachable, run `just template-retrofit` to re-seed.

## Pending LLM migrations

Some updates ship as agent prompts under `.awiki/pending-prompts/`. After merging, your agent should surface each prompt to you before acting (see `WIKI.md` §14). Lint warns on prompts older than 14 days.

## Common flags

| flag                              | use                                                                    |
|-----------------------------------|------------------------------------------------------------------------|
| `--apply`                         | Execute (default = dry-run plan)                                       |
| `--continue`                      | Resume after conflicts / migration failure                             |
| `--abort`                         | Discard in-progress update branch                                      |
| `--non-interactive`               | Auto-resolve prompts to safe defaults; CI mode                         |
| `--print-migrations`              | Show full migration bodies in plan                                     |
| `--accept-source-change`          | Confirm `--source` differs from pinned `repo`                          |
| `--accept-attribute-changes`      | Confirm `.gitattributes` filter changes                                |
| `--accept-manual-commits`         | Bypass `--continue` HEAD check after manual edits on update branch     |
| `--skip-migration <id>`           | Skip a specific failing migration                                      |
| `--rerun-bootstrap-step <id>`     | Re-run a single (often dangerous) bootstrap step                       |
| `--re-pin <commit>`               | Roll back the recorded pin (rebuilds cache from upstream)              |
| `--gc`                            | Prune orphaned cache dirs                                              |
| `--verify-signature`              | Require signed tag/commit                                              |

## Updating from a pre-v1 repo

If your wiki was bootstrapped before awiki v1.0.0, run:

```bash
just template-retrofit
```

This walks you through naming your original awiki commit/version and seeds `.awiki/template.json` with content_hashes for completed bootstrap steps. From then on, regular `just template-update` works.
```

- [ ] **Step 2: Commit**

```bash
git add docs/template-update.md
git commit -m "docs: docs/template-update.md user guide"
```

---

## Task 5: ADR + migrations/README.md author guide

**Files:**
- Create: `docs/decisions/template-update.md`
- Create: `migrations/README.md`

- [ ] **Step 1: Write ADR**

Create `docs/decisions/template-update.md`:

```markdown
# ADR: Template Update Mechanism

**Status:** Accepted
**Date:** 2026-04-27

## Context

The awiki repo is a template/scaffold cloned per domain. Users customize `WIKI.md` Identity, `hugo.toml`, `content/_index.md`, `.gitignore`, encryption setup, etc. As the template evolves (new scripts, MCP server fixes, schema changes), users need a way to pull updates without losing their customizations.

## Decision

Adopt a **detached-template + manifest** model:

- Bootstrapped repo records `.awiki/template.json` (pinned `commit`, `version`, `original_repo`).
- Updates fetch a fresh template, sync per-path strategy from `template.manifest.toml`, run versioned migrations, replay new bootstrap steps, write everything onto a dedicated `awiki-template-update/<sha>` branch.
- Default = dry-run; `--apply` mutates only on the dedicated branch with one commit per phase (Sync, Migrations, Bootstrap-steps, Provenance).
- `git merge-file --diff3` for hybrid files; user resolves conflicts via standard `<<<<<<<` markers.

## Alternatives considered

1. **Git remote + merge.** Rejected: merges noisy; user history mixes with template; encrypted repos shouldn't share history with public template.
2. **Copier / cruft.** Rejected: adds Python toolchain dependency; forces jinja restructure of all customizable files; awiki ethos is bash-first.

## Consequences

- Implementation is bash + Python helpers (no third-party dependencies beyond stdlib).
- Migration scripts and LLM prompts are trusted code — users review with `--print-migrations` and the in-flow user-confirm gate for prompts.
- Recovery flows (`--continue`, `--abort`, `--re-pin`) are first-class. `--non-interactive` enables CI.
- Rollout requires a retrofit script for repos bootstrapped before v1.0.0.

## See

- Spec: `docs/superpowers/specs/2026-04-27-template-update-design.md`
- Plans: `docs/superpowers/plans/2026-04-27-template-update-master-plan.md`
- User guide: `docs/template-update.md`
```

- [ ] **Step 2: Write migrations/README.md**

Create `migrations/README.md`:

```markdown
# Migrations

Migration scripts run during `just template-update --apply` to evolve user content alongside template changes. Two kinds:

- **Mechanical** (`NNNN-<slug>.sh`) — bash, idempotent, runs with stripped env.
- **LLM-assisted** (`NNNN-<slug>.prompt.md`) — staged into `.awiki/pending-prompts/`, agent picks up next session.

Numbering: zero-padded 4-digit, monotonic, never renumbered. Gaps OK.

Schema upgrades use `schema-NN-to-MM.sh` (separate sequence; see spec §Schema upgrades).

## Mechanical migration template

```bash
#!/usr/bin/env bash
# migration: 0007-rewrite-sources-blocks
# requires: agent=false
# touches: content/synthesis/**/*.md WIKI.md
# idempotent: yes
set -euo pipefail

if [[ "${1:-}" == "--dry-run" ]]; then
  echo "would rewrite ## Sources blocks"
  exit 0
fi

# Real run.
find content/synthesis -name '*.md' -exec sed -i.bak 's/^## Sources$/## Sources\n/' {} \;
find content/synthesis -name '*.bak' -delete
```

### Required header keys

| key            | meaning                                                                  |
|----------------|--------------------------------------------------------------------------|
| `migration`    | Slug matching filename (without `.sh`).                                  |
| `requires`     | `agent=true` or `agent=false`. Mechanical = `false`.                     |
| `touches`      | Space-separated globs. Runner enforces post-run `git status` matches.    |
| `idempotent`   | `yes` / `no`. Should always be `yes` for v1.                             |

### Banned `touches:` patterns

- `secrets/`, `themes/`, `.awiki/`, `.git/` — runner halts pre-run.
- Schema upgrades bypass `.awiki/` ban via `--schema-upgrade` flag.

### Stripped env

Runner clears `GH_TOKEN`, `GITHUB_TOKEN`, etc. Migrations cannot rely on auth tokens.

## LLM prompt template

```markdown
---
id: 0008-reformat-tags
requires: [agent]
scope_glob: "content/**/*.md"
risk: medium
---

Rewrite each `tags: [a, b]` line to `tags:\n  - a\n  - b`. Run `just lint` after to verify frontmatter parses.
```

### Required frontmatter keys

| key          | meaning                                                                  |
|--------------|--------------------------------------------------------------------------|
| `id`         | Slug (full filename minus `.prompt.md`).                                 |
| `requires`   | `[agent]`.                                                               |
| `scope_glob` | Path pattern. MUST NOT match `secrets/`, `.awiki/`, `.git/`, `themes/`.  |
| `risk`       | `low | medium | high`. `high` auto-declines under `--non-interactive`.   |

### Authoring guidelines

1. **Be specific.** Resolved file list is appended to the staged prompt; agent operates on that list, not your glob.
2. **No `secrets/`.** Runner rejects.
3. **Idempotent intent.** If applied twice, second pass should no-op or be safe.
4. **State acceptance criteria.** Most prompts end with "After completing, run `just lint`."

## Numbering rules

- Strictly linear. Authors MUST NOT ship a migration that depends on a skippable predecessor.
- If `0008` requires `0007`'s effect, document it in `0008`'s header and have it self-detect missing precondition (exit nonzero with clear error).
- Skipped migrations cannot be retroactively un-skipped automatically — user must edit `.awiki/template.json`.
```

- [ ] **Step 3: Commit**

```bash
git add docs/decisions/template-update.md migrations/README.md
git commit -m "docs: ADR + migrations/README.md author guide"
```

---

## Task 6: README.md mention

**Files:**
- Modify: `README.md`

- [ ] **Step 1: Add update story to README**

Insert into README.md (near the bottom, before any contributing/license sections):

```markdown
## Pulling template updates

Once your wiki is bootstrapped, you can pull awiki template updates with:

```bash
just template-update          # dry-run plan
just template-update --apply  # execute on dedicated review branch
```

See [docs/template-update.md](docs/template-update.md) for the full guide, trust model, and recovery flows.
```

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: README — mention just template-update"
```

---

## Task 7: CI E2E workflow

**Files:**
- Create: `tests/template-update-e2e.sh`
- Create: `.github/workflows/template-update-e2e.yml`

- [ ] **Step 1: E2E runner script**

Create `tests/template-update-e2e.sh`:

```bash
#!/usr/bin/env bash
# End-to-end: bootstrap from v0 fixture, mutate template to v1, run update, assert.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX=$(mktemp -d)
cd "$SANDBOX"

cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
git init -q -b main
git -c user.email=ci@example.com -c user.name=ci add -A
git -c user.email=ci@example.com -c user.name=ci commit -q -m "fixture v0"
COMMIT_OLD=$(git rev-parse HEAD)

bash "$REPO_ROOT/scripts/template-init.sh" \
  --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
  --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null

bash "$REPO_ROOT/scripts/template-update.sh" \
  --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
  --accept-source-change --apply --non-interactive --print-migrations

# Assertions.
[ -f scripts/new-helper.sh ] || { echo "FAIL: new-helper.sh not present"; exit 1; }
grep -q "## Added in v1" WIKI.md || { echo "FAIL: WIKI.md not merged"; exit 1; }
grep -q "## Tagline" WIKI.md || { echo "FAIL: migration 0001 did not run"; exit 1; }
PIN_VERSION=$(python3 -c "import json; print(json.load(open('.awiki/template.json'))['version'])")
[ "$PIN_VERSION" = "0.2.0" ] || { echo "FAIL: pin not advanced (got $PIN_VERSION)"; exit 1; }
[ ! -f .awiki/template-cache/_fetch/.update-state.json ] || { echo "FAIL: state file not removed"; exit 1; }

echo "E2E PASS"
rm -rf "$SANDBOX"
```

`chmod +x tests/template-update-e2e.sh`.

- [ ] **Step 2: GitHub Actions workflow**

Create `.github/workflows/template-update-e2e.yml`:

```yaml
name: template-update E2E

on:
  pull_request:
    paths:
      - 'scripts/template-*'
      - 'scripts/_template_helpers/**'
      - 'template.manifest.toml'
      - 'BOOTSTRAP.md'
      - 'tests/**'
      - '.github/workflows/template-update-e2e.yml'
  push:
    branches: [main]

jobs:
  e2e:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Set up Python 3.11
        uses: actions/setup-python@v5
        with:
          python-version: '3.11'
      - name: Install bats
        run: sudo apt-get update && sudo apt-get install -y bats git-crypt
      - name: Run BATS suite
        run: bats tests/template-*.bats
      - name: Run E2E
        run: bash tests/template-update-e2e.sh
```

- [ ] **Step 3: Run locally**

```bash
bash tests/template-update-e2e.sh
```

Expected: `E2E PASS`.

- [ ] **Step 4: Commit**

```bash
git add tests/template-update-e2e.sh .github/workflows/template-update-e2e.yml
git commit -m "test: end-to-end template-update CI workflow"
```

---

## Task 8: Final BATS matrix verification

**Files:** none — verification.

- [ ] **Step 1: Run every template-update BATS file**

```bash
bats tests/template-manifest.bats \
     tests/template-config.bats \
     tests/template-bootstrap-hash.bats \
     tests/template-provenance.bats \
     tests/template-init.bats \
     tests/template-plan.bats \
     tests/template-merge.bats \
     tests/template-attr-audit.bats \
     tests/template-source-check.bats \
     tests/template-update-preflight.bats \
     tests/template-update-fetch.bats \
     tests/template-update-plan.bats \
     tests/template-update-sync.bats \
     tests/template-update-migration.bats \
     tests/template-update-bootstrap-step.bats \
     tests/template-update-commit-d.bats \
     tests/template-update-abort.bats \
     tests/template-update-continue.bats \
     tests/template-update-repin.bats \
     tests/template-update-rerun-step.bats \
     tests/template-update-gc-status.bats \
     tests/template-update-bootstrap.bats \
     tests/template-update-lint.bats \
     tests/template-update-availability.bats \
     tests/template-update-retrofit.bats
```

Expected: all pass.

- [ ] **Step 2: Verify no orphaned state in fixtures**

```bash
find tests/fixtures/template-update -name "_fetch" -o -name ".update-state.json"
```

Expected: empty output.

---

## Task 9: Final CHANGELOG + tag v1.0.0

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Promote `[Unreleased]` to `[1.0.0]`**

Edit `CHANGELOG.md`. Replace `## [Unreleased]` with:

```markdown
## [1.0.0] - 2026-04-27

First release with the template-update mechanism. Repos bootstrapped from this version (or later) get the clean update path. Pre-v1 repos use `just template-retrofit` to seed `.awiki/template.json` once.

### Added
- Template update mechanism: detached-template + manifest model with per-path strategies (`overwrite`/`preserve`/`three_way`/`attributes_merge`/`template_only`), versioned migrations (mechanical bash + LLM prompt), three-way merge for hybrid files, dedicated update branch with phase-per-commit (Sync, Migrations, Bootstrap-steps, Provenance), state file for `--continue`/`--abort` recovery, multi-machine ancestor cache auto-recovery, `--re-pin` rollback, `--rerun-bootstrap-step` for dangerous steps, `--gc` cache cleanup, `--non-interactive` for CI.
- Trust model: pinned `repo`/`original_repo`, `--accept-source-change` gate, stripped env for migrations, `touches:` post-run enforcement, `scope_glob` enforcement for LLM prompts, optional `--verify-signature` GPG check.
- Lint additions for manifest dupes, migration headers, aged pending prompts, repo/original_repo drift, and update-availability detection via `_check-stamp`.
- BOOTSTRAP step IDs (HTML comment markers) + new `template-init` step seeding `.awiki/template.json` and `.awiki/template-cache/<commit>/`.
- Retrofit script for pre-v1 repos.
- BATS test suite covering happy / security / recovery / edge cases.
- CI E2E workflow.
- Docs: `docs/template-update.md`, `docs/decisions/template-update.md` (ADR), `migrations/README.md` (author guide), README mention, WIKI.md agent-facing pending-prompts workflow.

### Migration

If your wiki was bootstrapped before this release, run `just template-retrofit` once to seed `.awiki/template.json` with content_hashes for completed bootstrap steps. From then on, regular `just template-update` works.
```

- [ ] **Step 2: Commit + tag**

```bash
git add CHANGELOG.md
git commit -m "release: v1.0.0 — template update mechanism"
git tag -a v1.0.0 -m "v1.0.0: template update mechanism"
```

(Don't push tag without user confirmation.)

---

## Phase 10 — Definition of done

- [ ] `template-retrofit.sh` seeds pre-v1 repos with heuristic-assisted prompts.
- [ ] `template-step.sh` aliases `--rerun-bootstrap-step`.
- [ ] WIKI.md §14 documents agent-facing pending-prompts workflow with user-confirm gate.
- [ ] `docs/template-update.md` user guide published.
- [ ] ADR + `migrations/README.md` author guide published.
- [ ] README mentions `just template-update`.
- [ ] CI E2E workflow runs full BATS suite + end-to-end fixture round-trip.
- [ ] All BATS test files green.
- [ ] CHANGELOG promoted to `[1.0.0]`; tag created (push deferred to user).

## Master plan — Definition of done

After Phase 10 merges:

- [ ] All 26 spec implementation steps shipped.
- [ ] `just template-update --status` works on a freshly bootstrapped repo.
- [ ] `tests/template-update-e2e.sh` passes locally and in CI.
- [ ] `examples/sample-wiki/` (if maintained) updated to v1.0.0-bootstrapped state.
- [ ] Lint clean across awiki repo with new rules active.
- [ ] Tag `v1.0.0` created.

After user pushes the tag, awiki has a stable update story. Repos bootstrapped from `>=v1.0.0` get the clean path; older repos can retrofit.
