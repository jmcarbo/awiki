# awiki Plan — Phase 1: Skeleton

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** —
**Previous:** —
**Next:** [Phase 02](./2026-04-27-phase-02-scripts-core.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** Repo layout with empty content/raw dirs, schema docs, gitignore, gitattributes, justfile stubs, README, dep-check script.

**Branch:** `phase-1-skeleton`

## Task 1.1: Branch + directory layout

**Files:**
- Create: directory structure under repo root.

- [ ] **Step 1: Create branch**

```bash
git checkout -b phase-1-skeleton
```

- [ ] **Step 2: Create directories**

```bash
mkdir -p content/{entities,concepts,topics,sources,synthesis} \
         raw/{inbox/{interactive,batch,checkpoint},processed,assets} \
         scripts \
         tests/fixtures \
         docs/decisions \
         themes \
         deploy \
         scheduled \
         examples \
         mcp \
         .awiki
```

- [ ] **Step 3: Add `.gitkeep` to every empty dir**

```bash
find content raw scripts tests docs themes deploy scheduled examples mcp .awiki \
     -type d -empty -exec touch {}/.gitkeep \;
```

- [ ] **Step 4: Seed system pages (`_index.md`, `catalog.md`, `log.md`)**

```bash
cat > content/_index.md <<'EOF'
---
title: "awiki"
type: section-index
draft: false
---

# Welcome

This wiki was scaffolded from the awiki template. After bootstrap, this paragraph will be replaced by your wiki's purpose and entry-point links.

- [Catalog](/catalog/) — full content listing.
- [Log](/log/) — chronological activity log.
EOF

cat > content/catalog.md <<'EOF'
---
title: "Catalog"
type: catalog
draft: false
---

# Catalog

LLM-maintained index of every page. Updated on every ingest.

## Entities

(empty)

## Concepts

(empty)

## Topics

(empty)

## Sources

(empty)

## Synthesis

(empty)
EOF

cat > content/log.md <<'EOF'
---
title: "Log"
type: log
draft: true
---

# Log

Append-only chronological record. One H2 per entry. BOOTSTRAP appends the first entry.
EOF
```

- [ ] **Step 5: Verify layout**

Run: `find content raw scripts tests docs themes deploy scheduled examples mcp -type d | sort && ls content/`

Expected: tree matches spec's Repository Layout; `_index.md`, `catalog.md`, `log.md` present.

- [ ] **Step 6: Raw originals dir**

```bash
mkdir -p raw/processed/_originals
touch raw/processed/_originals/.gitkeep
```

- [ ] **Step 7: Commit**

```bash
git add content raw scripts tests docs themes deploy scheduled examples mcp .awiki layouts 2>/dev/null || true
mkdir -p layouts/shortcodes
touch layouts/shortcodes/.gitkeep
git add content raw scripts tests docs themes deploy scheduled examples mcp .awiki layouts
git commit -m "chore: scaffold awiki directory layout with seeded system pages"
```

## Task 1.2: `.gitignore`

**Files:** Create: `.gitignore`

- [ ] **Step 1: Write `.gitignore`**

```
# build artifacts
public/
.hugo_build.lock
.qmd/
.awiki/*
!.awiki/.gitkeep

# inbox: source files often sensitive / copyrighted
raw/inbox/*
!raw/inbox/interactive/.gitkeep
!raw/inbox/batch/.gitkeep
!raw/inbox/checkpoint/.gitkeep

# default-private until user opts in via BOOTSTRAP / encrypt-init
raw/processed/*
!raw/processed/.gitkeep
content/private/**
raw/processed/private/**
raw/processed/_originals/**

# secrets — deny all, allow only public keys
secrets/**
!secrets/*.pub

# obsidian local state
.obsidian/workspace*
.obsidian/cache

# OS / editor
.DS_Store
*.swp
*.swo
```

- [ ] **Step 2: Commit**

```bash
git add .gitignore
git commit -m "chore: add default .gitignore with private-by-default patterns"
```

## Task 1.3: `.gitattributes`

**Files:** Create: `.gitattributes`

- [ ] **Step 1: Write `.gitattributes`**

```
# git-crypt patterns activated by scripts/encrypt-init.sh
# Uncomment after running `just encrypt-init`:
# raw/processed/private/** filter=git-crypt diff=git-crypt
# raw/processed/_originals/** filter=git-crypt diff=git-crypt
# content/private/** filter=git-crypt diff=git-crypt
# secrets/** filter=git-crypt diff=git-crypt

# end-of-line normalization
* text=auto
*.sh text eol=lf
*.toml text eol=lf
*.md text eol=lf
```

- [ ] **Step 2: Commit**

```bash
git add .gitattributes
git commit -m "chore: add .gitattributes with EOL normalization and commented git-crypt stubs"
```

## Task 1.4: `WIKI.md` schema

**Files:** Create: `WIKI.md`

- [ ] **Step 1: Write `WIKI.md`**

````markdown
# WIKI Schema

This document is the contract every agent (Claude Code, Codex, OpenCode, …) must follow when operating this wiki. Read it in full before responding to any user request that touches the wiki.

## 1. Identity

<!-- BOOTSTRAP fills this section. Do not edit manually after bootstrap. -->
- **wiki_name:** `<unset>`
- **domain:** `<unset>`
- **purpose:** `<unset>`

## 2. Page Conventions

- Filenames: `kebab-case.md`. Folder = page kind.
- Required frontmatter (YAML, single block):

```yaml
---
title: "Vannevar Bush"
date: 2026-04-27
last_updated: 2026-04-27
type: entity                    # entity|concept|topic|source|synthesis|deck|chart|canvas|log|catalog|section-index
tags: [memex, computing-history]
aliases: [Bush, V. Bush]
sources: ["[[s-as-we-may-think]]"]
draft: false
---
```

- Body structure: lead paragraph (≤100 words) → sections → `## Related` (wikilinks) → `## Sources`.
- `type` enum splits into **page kinds** (`entity`, `concept`, `topic`, `source`, `synthesis`, `deck`, `chart`, `canvas`) and **system pages** (`log`, `catalog`, `section-index`). System pages have separate validation; orphan-check exempts them.
- Wikilinks in frontmatter `sources:` are YAML strings, NOT rendered as links by Obsidian or Hugo. The body `## Sources` section mirrors them as real links.

## 3. Wikilink Rules

- `[[page-slug]]` or `[[page-slug|display]]`.
- Slugs are filenames without `.md`, unique across `content/`.
- Aliases (frontmatter `aliases:`) resolve via the alias map built by `lint.sh`. `[[Bush]]` resolves to `vannevar-bush.md` if uniquely aliased; collisions are lint errors.
- Lint flags broken links and slug collisions.

## 4. Workflows

### 4.1 Ingest

1. Determine queue mode from path under `raw/inbox/{interactive,batch,checkpoint}/`.
2. Run `bash scripts/ingest.sh <source-path>` — moves source, appends log, increments counter.
3. Read the source (Read tool / vision tool as needed).
4. Per queue mode:
   - **interactive:** propose 3-5 takeaways + page-update plan. Wait for user confirmation.
   - **batch:** proceed with default emphasis (full summary, all entities/concepts).
   - **checkpoint:** write proposal to `raw/inbox/checkpoint/.staged/<slug>.md` and stop.
5. Write `content/sources/<slug>.md` (frontmatter + summary + extracted facts).
6. Update affected `entities/`, `concepts/`, `topics/` pages (create if needed).
7. Update `content/catalog.md` — add or modify entries.
8. Update affected section indexes' descriptions if and only if the section's purpose materially changed.
9. Verify no broken wikilinks introduced (run `just lint` if many pages touched).

### 4.2 Query

1. Read `content/catalog.md`.
2. If wiki has >100 sources or query needs vector match: shell to `qmd search "<query>"`.
3. Otherwise: drill into pages directly via wikilinks from catalog entries.
4. Answer with `[[…]]` citations to source/entity/concept pages.
5. If the answer represents novel synthesis, file under `content/synthesis/<slug>.md` with `type: synthesis` and append a `synthesis` log entry.

### 4.3 Lint

1. Run `just lint` (or `just lint-fix` for mechanical auto-fixes).
2. Read structured `LINT|<level>|<file>|<msg>` output.
3. Group issues by theme (broken links, orphans, contradictions, coverage gaps).
4. Propose fixes per group; user approves per group.
5. After fixes applied, run `bash scripts/log-append.sh lint "<summary>"`.

### 4.4 Rename / Delete

- Rename a slug: `bash scripts/rename.sh <old-slug> <new-slug>`. Updates all wikilinks, catalog, and aliases.
- Delete a page: `bash scripts/delete-page.sh <slug>`. Removes file, prunes wikilinks (replaced with plain text + `<!-- broken: was [[slug]] -->` marker), updates catalog. Lint surfaces the markers for review.

## 5. Inbox Queues

- `raw/inbox/interactive/` — single source, supervised.
- `raw/inbox/batch/` — folder dump, unattended; agent writes `synthesis/batch-<date>.md` summary at end.
- `raw/inbox/checkpoint/` — staged proposals under `.staged/`, user approves before commit.

## 6. Output Formats

Markdown, comparison table, Marp slide deck (`type: deck`), Matplotlib chart (`type: chart`), Mermaid diagram (inline in markdown), Obsidian canvas (`type: canvas`). All filed under `content/synthesis/`.

## 7. Lint Checklist

Mechanical (`scripts/lint.sh`):
- Broken wikilinks.
- Missing required frontmatter fields per `type`.
- Stale `last_updated` (>90d, configurable via `AWIKI_STALE_DAYS`).
- Duplicate aliases across pages.
- Empty pages (<50 chars body).
- Catalog dangling/missing entries.
- Slug uniqueness.
- Privacy: `tags: [private]` outside `**/private/` paths.

Semantic (agent-driven, post-lint review):
- Contradictions between pages.
- Stale claims vs newer sources.
- Missing concept pages mentioned across sources.

## 8. qmd Usage

- Heuristic: shell to `qmd search` when wiki >~100 sources or query needs vector match.
- Fallback: `catalog.md` + `grep -r` if `.awiki/qmd-status=missing`.
- Reindex: automatic after each ingest (incremental); manual via `just reindex`.

## 9. Privacy Guarantees

- Agent NEVER moves files into a `private/` path without explicit user confirmation.
- Lint (`scripts/lint.sh`) warns on `tags: [private]` outside `**/private/`.
- `log.md` ships with `draft: true`; opt-in publishing via BOOTSTRAP.
- `query` action NOT logged unless `AWIKI_LOG_QUERIES=1` in `.awiki/config`.

## 10. Conventions Recap (one-liners)

- Slug = filename without `.md`. Lowercase kebab-case.
- One source = one page in `content/sources/<slug>.md`.
- Catalog updated on every ingest.
- Section indexes updated only when section's purpose changes.
- All structural changes go through scripts; no direct file moves by agent for sources.
````

- [ ] **Step 2: Commit**

```bash
git add WIKI.md
git commit -m "docs: add WIKI.md schema"
```

## Task 1.5: `BOOTSTRAP.md`

**Files:** Create: `BOOTSTRAP.md`

- [ ] **Step 1: Write `BOOTSTRAP.md`**

````markdown
# Bootstrap

Run this once when the user first opens an agent in a fresh clone of the awiki template. Walk through each step in order. Do NOT skip steps; later steps depend on earlier ones.

## Step 0: Dependency check

Run `bash scripts/check-deps.sh`. If it exits non-zero, surface the printed install hints and halt. Re-run after the user installs missing tools.

## Step 1: Domain

Ask the user one of:

- A. Personal — health, goals, journals, self-improvement.
- B. Research — deep topic over weeks/months (papers, articles).
- C. Reading — book, series, course; characters, themes, plot.
- D. Business / team — Slack, meetings, project docs.
- E. Other — let user describe.

Record the answer.

## Step 2: Wiki name + purpose

Ask: "What's the wiki called?" — kebab-case identifier.
Ask: "One-line purpose?" — single sentence, ≤120 chars.

## Step 3: Encryption decision

Ask:

- A. None — public wiki, nothing sensitive.
- B. git-crypt — symmetric, transparent. Recommended for personal/health/journal.
- C. age — asymmetric, manual.

If A: continue.
If B: run `bash scripts/encrypt-init.sh`.
If C: run `bash scripts/encrypt-init.sh --age`.

After encrypt-init, run `git status` and verify expected encrypted-vs-cleartext patterns before any commit.

## Step 4: Track ingested sources?

Ask: "Track ingested sources in git? (y/N)". Default N.

If y: edit `.gitignore` and remove these two lines:
```
raw/processed/*
!raw/processed/.gitkeep
```
The `_originals/` and `private/` exceptions remain ignored (still privacy-protected).

Note to user: tracking sources may include copyrighted material. History-rewrite cost is non-trivial if revoked.

## Step 5: Hugo theme

Ask: "Hugo theme? (default: hugo-book)"

The default `hugo-book` submodule is already present from the template. Skip the add step if the user accepts the default. If the user picks an alternative:

```bash
git submodule deinit -f themes/hugo-book
git rm -f themes/hugo-book
rm -rf .git/modules/themes/hugo-book
git submodule add <theme-url> themes/<theme-name>
```

After switching, also rewrite `theme = "hugo-book"` in `hugo.toml` to the chosen theme name.

## Step 6: Publish log?

Ask: "Publish log to rendered site? (y/N)". Default N.

- N → leave `content/log.md` frontmatter `draft: true`.
- y → set `content/log.md` frontmatter `draft: false`.

## Step 7: Patch identity

Edit:
- `WIKI.md` Identity section: fill `wiki_name`, `domain`, `purpose` from steps 1-2.
- `hugo.toml`: set `title`, `baseURL` (ask if not known yet — placeholder fine).
- `content/_index.md`: set `title`, write a one-paragraph wiki landing.
- `content/log.md`: frontmatter from step 6.

## Step 8: Install qmd

Run `just install-qmd`. Treat failure as non-fatal:
- On success: confirm `.awiki/qmd-status=ok`, run `just reindex`.
- On failure: `.awiki/qmd-status=missing`, print warning, continue. Agent uses `grep -r` fallback per WIKI.md.

If `~/.local/bin` is not on PATH, print:

```
Add this to ~/.zshrc or ~/.bashrc:
export PATH="$HOME/.local/bin:$PATH"
```

## Step 9a: Wire qmd MCP server (optional)

Ask: "Wire qmd MCP server into your agent harness? (y/N)" — only if `.awiki/qmd-status=ok`.

If y: run `bash scripts/wire-qmd-mcp.sh`. Print verification instructions.

## Step 9b: Wire awiki wiki-ops MCP server (optional)

Ask: "Wire awiki wiki-ops MCP server (ingest/lint/query/update_catalog)? (y/N)".

If y, install Node deps and wire:

```bash
( cd mcp/awiki-server && npm install --silent )
bash scripts/wire-awiki-mcp.sh
```

The wire script registers the awiki server in:
- Claude Code: project-level `./.mcp.json` (created if absent).
- Codex: `./.codex/config.toml` `[mcp.servers.awiki]` block.

Verify registration by listing tools in the next agent session. The MCP tools are: `ingest_source`, `lint`, `query_wiki`, `update_catalog`.

## Step 10: Initial log entry

```bash
bash scripts/log-append.sh init "wiki '$WIKI_NAME' initialized for domain '$DOMAIN'"
```

## Step 11: Initial commit

Stage:
```bash
git add -A
git status
```

Show user the file list. Ask: "Stage all and commit? (y/N)". On y:

```bash
git commit -m "chore: initialize wiki '$WIKI_NAME'"
```

## Step 12: Smoke test prompt

Tell the user:

> Bootstrap complete. Try the smoke test in README.md to verify everything works:
> 1. Drop a sample source into `raw/inbox/interactive/sample.md`.
> 2. `just ingest raw/inbox/interactive/sample.md`.
> 3. `just lint`.
> 4. `just serve`.
> 5. `just search "test"` (if qmd installed).
````

- [ ] **Step 2: Commit**

```bash
git add BOOTSTRAP.md
git commit -m "docs: add BOOTSTRAP.md agent-driven init walkthrough"
```

## Task 1.6: Stub `CLAUDE.md` and `AGENTS.md`

**Files:** Create: `CLAUDE.md`, `AGENTS.md`

- [ ] **Step 1: Write `CLAUDE.md`**

```markdown
# Claude Code Instructions

> Schema is in [WIKI.md](./WIKI.md). Read it before responding.

This wiki follows the awiki template. The full schema (page conventions, workflows, lint rules, privacy guarantees) lives in `WIKI.md`. Always consult it before any wiki operation.

For first-time initialization, read [BOOTSTRAP.md](./BOOTSTRAP.md) and follow it step by step.
```

- [ ] **Step 2: Write `AGENTS.md` (identical body)**

```markdown
# Agent Instructions

> Schema is in [WIKI.md](./WIKI.md). Read it before responding.

This wiki follows the awiki template. The full schema (page conventions, workflows, lint rules, privacy guarantees) lives in `WIKI.md`. Always consult it before any wiki operation.

For first-time initialization, read [BOOTSTRAP.md](./BOOTSTRAP.md) and follow it step by step.
```

- [ ] **Step 3: Commit**

```bash
git add CLAUDE.md AGENTS.md
git commit -m "docs: add CLAUDE.md and AGENTS.md stubs pointing to WIKI.md"
```

## Task 1.7: `README.md`

**Files:** Create: `README.md`

- [ ] **Step 1: Write `README.md`**

````markdown
# awiki

A template repository for building personal LLM-maintained wikis. Domain-agnostic. Multi-agent (Claude Code, Codex, OpenCode). Hugo-renderable. Obsidian-friendly. Search via qmd.

## Quick start

```bash
git clone <this-repo> mywiki
cd mywiki
just                          # see available commands
```

Open your agent (Claude Code, Codex, OpenCode) in this directory. Say:

> init wiki

The agent reads `BOOTSTRAP.md` and walks through customization (domain, name, encryption, theme, qmd install). Done.

## Smoke test (post-bootstrap)

```bash
echo "# Sample\nA test source about caves." > raw/inbox/interactive/sample.md
just ingest raw/inbox/interactive/sample.md   # agent processes via WIKI.md flow
just lint                                     # validate wiki integrity
just serve                                    # local Hugo preview
just search "cave"                            # qmd search (if installed)
```

## Layout

- `content/` — LLM-owned wiki pages (Hugo-served).
- `raw/` — immutable source documents.
- `WIKI.md` — schema (primary contract for agents).
- `BOOTSTRAP.md` — first-run walkthrough.
- `scripts/` — bookkeeping helpers.
- `justfile` — recipe entry-point.

Full architecture: see `docs/superpowers/specs/2026-04-27-llm-wiki-scaffold-design.md`.

## Common commands

| Recipe | Purpose |
|--------|---------|
| `just ingest <path>` | Process a source from inbox into wiki. |
| `just lint` | Validate wiki integrity. |
| `just lint-fix` | Auto-fix mechanical lint issues. |
| `just serve` | Local Hugo preview at http://localhost:1313. |
| `just build` | Build static site to `public/`. |
| `just search "<q>"` | Search wiki via qmd. |
| `just reindex` | Refresh qmd index. |
| `just encrypt-init` | Set up git-crypt for sensitive content. |
| `just rename <old> <new>` | Rename a slug, updating all wikilinks. |
| `just delete <slug>` | Remove a page, marking broken wikilinks. |
| `just test` | Run BATS test suite. |
| `just help` | Extended help. |

## Dependencies

| Tool | Required? |
|------|-----------|
| bash 4+ | yes |
| git 2.30+ | yes |
| just 1.13+ | yes |
| hugo 0.120+ extended | yes |
| bats-core 1.10+ | yes (for `just test`) |
| qmd (qntx-labs fork) | optional, recommended |
| git-crypt 0.7+ | optional (encryption path) |
| age 1.0+ | optional (encryption path) |

Run `bash scripts/check-deps.sh` to verify.

## License

Choose your own per-clone. Template ships without a LICENSE file.
````

- [ ] **Step 2: Commit**

```bash
git add README.md
git commit -m "docs: add README with quick start and smoke test"
```

## Task 1.8: Skeleton `justfile`

**Files:** Create: `justfile`, `docs/just-help.txt`

- [ ] **Step 1: Write `justfile` with all recipe stubs**

```just
set shell := ["bash", "-uc"]

default:
    @just --list

# === ingest ===
ingest path:
    bash scripts/ingest.sh {{path}}

ingest-batch-list:
    @find raw/inbox/batch -type f | sort

# === maintenance ===
lint:
    bash scripts/lint.sh

lint-fix:
    bash scripts/lint.sh --fix

reindex:
    bash scripts/qmd-index.sh

search query:
    qmd search "{{query}}"

log action *message:
    bash scripts/log-append.sh {{action}} {{message}}

rename old new:
    bash scripts/rename.sh {{old}} {{new}}

delete slug:
    bash scripts/delete-page.sh {{slug}}

# === hugo ===
serve:
    bash scripts/serve.sh

build:
    bash scripts/build.sh

# === bootstrap / setup ===
init:
    @echo "Open agent. Say: 'init wiki'. Agent reads BOOTSTRAP.md."

install-qmd:
    bash scripts/install-qmd.sh

encrypt-init:
    bash scripts/encrypt-init.sh

check-deps:
    bash scripts/check-deps.sh

# === git ===
status:
    git status -s

commit message:
    git add -A && git commit -m "{{message}}"

# === tests ===
test:
    bats tests/

# === help ===
help:
    @cat docs/just-help.txt
```

- [ ] **Step 2: Write `docs/just-help.txt`**

```
awiki — recipe reference

INGEST
  just ingest <path>            Process source from inbox.
                                Path must be under raw/inbox/{interactive,batch,checkpoint}/.
                                Example: just ingest raw/inbox/interactive/article.md
  just ingest-batch-list        List files queued in raw/inbox/batch/ for agent iteration.

MAINTENANCE
  just lint                     Validate wiki integrity. Output: LINT|<level>|<file>|<msg>.
  just lint-fix                 Auto-apply mechanical fixes only (frontmatter, whitespace).
  just reindex                  Refresh qmd index over content/.
  just search "<query>"         Hybrid BM25+vector search via qmd. Quote multi-word.
  just log <action> <message>   Append timestamped entry to content/log.md.
                                Example: just log manual "reviewed catalog"
  just rename <old> <new>       Rename a slug, updating all wikilinks.
                                Example: just rename foo-bar foobar
  just delete <slug>            Remove a page, marking broken wikilinks for review.

HUGO
  just serve                    Watch + preprocess + hugo server. http://localhost:1313.
  just build                    Preprocess + hugo build → public/.

SETUP
  just init                     Reminds you to open an agent and say "init wiki".
  just install-qmd              Install qmd from qntx-labs fork (idempotent).
  just encrypt-init             Set up git-crypt or age for sensitive content.
  just check-deps               Verify required tools installed.

GIT
  just status                   Short git status.
  just commit "<message>"       Stage all + commit. Quote multi-word messages.
                                Example: just commit "feat: add memex page"

TESTS
  just test                     Run BATS test suite under tests/.

HELP
  just help                     This text.

QUEUE SEMANTICS
  raw/inbox/interactive/        Default; agent supervises one-at-a-time.
  raw/inbox/batch/              Unattended bulk; agent writes batch synthesis at end.
  raw/inbox/checkpoint/         Staged; agent writes proposal to .staged/, user approves.

ARGUMENT QUOTING
  Multi-word args MUST be quoted: `just commit "fix: foo"`, `just search "memex history"`.
  Variadic recipes (log, rename) accept space-separated remaining args; quote if any contain spaces.
```

- [ ] **Step 3: Verify justfile syntax**

Run: `just --list`

Expected: prints recipe list, no errors.

- [ ] **Step 4: Commit**

```bash
git add justfile docs/just-help.txt
git commit -m "feat: add justfile recipe stubs and extended help"
```

## Task 1.9: `scripts/check-deps.sh`

**Files:** Create: `scripts/check-deps.sh`, `tests/check_deps_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/check_deps_test.bats <<'EOF'
#!/usr/bin/env bats

@test "check-deps exits 0 when all required deps present" {
  # Assumes test env has bash/git/just/hugo/bats; smoke test.
  run bash scripts/check-deps.sh
  [ "$status" -eq 0 ]
}

@test "check-deps prints OS hint when missing tool" {
  # Run with a fake PATH that excludes git.
  run env PATH="/usr/bin:/bin" AWIKI_FAKE_MISSING=git bash scripts/check-deps.sh
  [ "$status" -ne 0 ]
  [[ "$output" == *"git"* ]]
}
EOF
```

- [ ] **Step 2: Run test to verify it fails**

Run: `bats tests/check_deps_test.bats`
Expected: FAIL — `scripts/check-deps.sh` does not yet exist.

- [ ] **Step 3: Write `scripts/check-deps.sh`**

```bash
cat > scripts/check-deps.sh <<'EOF'
#!/usr/bin/env bash
set -uo pipefail

OS="$(uname -s)"
ERRORS=0

check() {
  local cmd="$1" min_version="$2" install_macos="$3" install_linux="$4"
  if [[ -n "${AWIKI_FAKE_MISSING:-}" && "$cmd" == "$AWIKI_FAKE_MISSING" ]]; then
    echo "MISSING|$cmd|min=$min_version" >&2
    case "$OS" in
      Darwin) echo "  install: $install_macos" >&2 ;;
      Linux)  echo "  install: $install_linux" >&2 ;;
    esac
    ERRORS=$((ERRORS + 1))
    return
  fi
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "MISSING|$cmd|min=$min_version" >&2
    case "$OS" in
      Darwin) echo "  install: $install_macos" >&2 ;;
      Linux)  echo "  install: $install_linux" >&2 ;;
    esac
    ERRORS=$((ERRORS + 1))
    return
  fi
  echo "OK|$cmd"
}

check_bash_version() {
  local major="${BASH_VERSION%%.*}"
  if [[ "$major" -lt 4 ]]; then
    echo "MISSING|bash|need=4+|have=$BASH_VERSION" >&2
    case "$OS" in
      Darwin) echo "  install: brew install bash" >&2 ;;
      Linux)  echo "  install: bash 4+ should be default; check distro packages" >&2 ;;
    esac
    ERRORS=$((ERRORS + 1))
    return
  fi
  echo "OK|bash|$BASH_VERSION"
}

check_bash_version
check git "2.30" "brew install git" "apt install git"
check just "1.13" "brew install just" "cargo install just"
check hugo "0.120" "brew install hugo" "see https://gohugo.io/installation/"
check bats "1.10" "brew install bats-core" "apt install bats"

# Optional tools — warn but do not fail.
for opt in qmd git-crypt age entr fswatch pdftotext; do
  if command -v "$opt" >/dev/null 2>&1; then
    echo "OK|$opt (optional)"
  else
    echo "OPTIONAL-MISSING|$opt"
  fi
done

if [[ "$ERRORS" -gt 0 ]]; then
  echo "DEPS-SUMMARY|errors=$ERRORS" >&2
  exit 1
fi
echo "DEPS-SUMMARY|errors=0"
EOF
chmod +x scripts/check-deps.sh
```

- [ ] **Step 4: Run test to verify pass**

Run: `bats tests/check_deps_test.bats`
Expected: 2 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/check-deps.sh tests/check_deps_test.bats
git commit -m "feat: add check-deps script + BATS test"
```

## Task 1.10: Phase 1 smoke test + merge

- [ ] **Step 1: Verify directory tree, files, gitignore patterns**

Run:
```bash
ls -la
just --list
bash scripts/check-deps.sh
```
Expected: layout matches spec; recipes listed; deps OK or fixable.

- [ ] **Step 2: Update task list and merge**

```bash
git checkout main
git merge --ff-only phase-1-skeleton || git merge --no-ff phase-1-skeleton -m "feat: complete phase 1 skeleton"
git branch -d phase-1-skeleton
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 02](./2026-04-27-phase-02-scripts-core.md).
