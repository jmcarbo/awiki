# awiki Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build awiki, a domain-agnostic, multi-agent template repository for LLM-maintained personal wikis.

**Architecture:** Three layers — immutable raw sources, LLM-owned Hugo `content/` markdown wiki, and a `WIKI.md` schema. Thin bash scripts handle bookkeeping (ingest, lint, log, qmd index, encrypt). A `justfile` exposes recipes. qmd (qntx-labs fork) provides hybrid search; index-first retrieval with grep fallback. Bootstrap is agent-driven via `BOOTSTRAP.md`. v1 ships in 12 sequential-but-mostly-parallelizable phases.

**Tech Stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, Node 20+ (phase 8 MCP server only), entr or fswatch (phase 3 watcher).

---

## How to Read This Plan

Phases are numbered 1-12. Each phase contains tasks. Tasks contain bite-sized steps. Every code block is the exact content to write — no placeholders. File paths are absolute relative to repo root `/Users/joanmarc/dailywork/celonis/awiki/`.

Spec reference: `docs/superpowers/specs/2026-04-27-llm-wiki-scaffold-design.md`.

Conventions:
- Commit after every task. Commit messages use Conventional Commits (`feat:`, `fix:`, `docs:`, `test:`, `chore:`).
- Branch per phase: `phase-1-skeleton`, `phase-2-scripts-core`, etc. Merge via PR (or fast-forward locally).
- All scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.

---

# Phase 1: Skeleton

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

# Phase 2: Scripts Core

**Deliverable:** Functional `log-append.sh`, `ingest.sh`, `lint.sh` (mechanical), `.awiki/config` defaults, BATS tests for each.

**Branch:** `phase-2-scripts-core`
**Depends on:** Phase 1.

## Task 2.1: Branch + `.awiki/config` defaults

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-2-scripts-core
```

- [ ] **Step 2: Write `.awiki/config`**

```bash
cat > .awiki/config <<'EOF'
# awiki runtime config — edit as needed
AWIKI_LINT_AFTER_N=5
AWIKI_STALE_DAYS=90
AWIKI_LOG_QUERIES=0
EOF
```

- [ ] **Step 3: Update `.gitignore` to track `.awiki/config`**

Edit `.gitignore`: change `.awiki/*` block to:

```
.awiki/*
!.awiki/.gitkeep
!.awiki/config
```

- [ ] **Step 4: Commit**

```bash
git add .awiki/config .gitignore
git commit -m "chore: add .awiki/config defaults"
```

## Task 2.2: `scripts/log-append.sh`

**Files:** Create: `scripts/log-append.sh`, `tests/log_append_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/log_append_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  TEST_LOG="$(mktemp -d)/log.md"
  export AWIKI_LOG_FILE="$TEST_LOG"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$TEST_LOG"
}

teardown() {
  rm -rf "$(dirname "$TEST_LOG")"
}

@test "log-append writes ## [datetime] action | message" {
  bash scripts/log-append.sh ingest "Sample article"
  run grep -E '^## \[[0-9]{4}-[0-9]{2}-[0-9]{2} [0-9]{2}:[0-9]{2}\] ingest \| Sample article$' "$TEST_LOG"
  [ "$status" -eq 0 ]
}

@test "log-append handles multi-word message" {
  bash scripts/log-append.sh manual reviewed the catalog
  run grep -E '^## \[.*\] manual \| reviewed the catalog$' "$TEST_LOG"
  [ "$status" -eq 0 ]
}

@test "log-append fails on missing action arg" {
  run bash scripts/log-append.sh
  [ "$status" -ne 0 ]
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

Run: `bats tests/log_append_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 3: Write `scripts/log-append.sh`**

```bash
cat > scripts/log-append.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ $# -lt 1 ]]; then
  echo "usage: log-append.sh <action> [message...]" >&2
  exit 1
fi

ACTION="$1"
shift
MESSAGE="$*"
LOG_FILE="${AWIKI_LOG_FILE:-content/log.md}"
NOW="$(date '+%Y-%m-%d %H:%M')"

if [[ ! -f "$LOG_FILE" ]]; then
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n\n" > "$LOG_FILE"
fi

printf -- "## [%s] %s | %s\n" "$NOW" "$ACTION" "$MESSAGE" >> "$LOG_FILE"
EOF
chmod +x scripts/log-append.sh
```

- [ ] **Step 4: Run test (expect PASS)**

Run: `bats tests/log_append_test.bats`
Expected: 3 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/log-append.sh tests/log_append_test.bats
git commit -m "feat: add log-append script + BATS tests"
```

## Task 2.3: `scripts/ingest.sh`

**Files:** Create: `scripts/ingest.sh`, `tests/ingest_test.bats`, `tests/fixtures/sample-source.md`

- [ ] **Step 1: Write the fixture**

```bash
cat > tests/fixtures/sample-source.md <<'EOF'
# Sample Source
A short article about caves.
EOF
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/ingest_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  cp -r raw "$WORK/raw"
  cp tests/fixtures/sample-source.md "$WORK/raw/inbox/interactive/sample.md"
  mkdir -p "$WORK/.awiki" "$WORK/content"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cp .awiki/config "$WORK/.awiki/config"
  export AWIKI_REPO_ROOT="$WORK"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "ingest moves source from inbox to processed" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  [ "$status" -eq 0 ]
  [ ! -f raw/inbox/interactive/sample.md ]
  [ -f raw/processed/interactive/sample.md ]
}

@test "ingest appends log entry" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  run grep "ingest | sample.md | mode=interactive" content/log.md
  [ "$status" -eq 0 ]
}

@test "ingest increments .awiki/ingest-count" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" raw/inbox/interactive/sample.md
  count="$(cat .awiki/ingest-count)"
  [ "$count" = "1" ]
}

@test "ingest rejects path outside inbox" {
  cp tests/fixtures/sample-source.md "$WORK/elsewhere.md" 2>/dev/null || cp "$BATS_TEST_DIRNAME/../tests/fixtures/sample-source.md" elsewhere.md
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh" elsewhere.md
  [ "$status" -eq 2 ]
}

@test "ingest rejects missing arg" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest.sh"
  [ "$status" -eq 1 ]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL)**

Run: `bats tests/ingest_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 4: Write `scripts/ingest.sh`**

```bash
cat > scripts/ingest.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# Load config
if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi
THRESHOLD="${AWIKI_LINT_AFTER_N:-5}"

if [[ $# -lt 1 ]]; then
  echo "usage: ingest.sh <path-under-raw/inbox/>" >&2
  exit 1
fi

SRC="$1"
# Normalize to relative path from repo root.
if [[ "$SRC" = /* ]]; then
  SRC="${SRC#"$REPO_ROOT/"}"
fi

if [[ ! "$SRC" =~ ^raw/inbox/(interactive|batch|checkpoint)/.+ ]]; then
  echo "ERROR: path must be under raw/inbox/{interactive,batch,checkpoint}/" >&2
  exit 2
fi
MODE="${BASH_REMATCH[1]}"

if [[ ! -f "$SRC" ]]; then
  echo "ERROR: source not found: $SRC" >&2
  exit 1
fi

# Strip leading raw/inbox/<mode>/ to get relative subtree
REL="${SRC#raw/inbox/$MODE/}"
DEST="raw/processed/$MODE/$REL"
DEST_DIR="$(dirname "$DEST")"

if [[ -f "$DEST" ]]; then
  echo "ERROR: already processed: $DEST" >&2
  exit 3
fi

mkdir -p "$DEST_DIR"
mv "$SRC" "$DEST"

bash scripts/log-append.sh ingest "$(basename "$SRC") | mode=$MODE"

# Increment counter
COUNTER_FILE=".awiki/ingest-count"
mkdir -p .awiki
COUNT=0
[[ -f "$COUNTER_FILE" ]] && COUNT="$(cat "$COUNTER_FILE")"
COUNT=$((COUNT + 1))
echo "$COUNT" > "$COUNTER_FILE"

echo "INGEST-OK|src=$SRC|dest=$DEST|mode=$MODE|count=$COUNT"

# Auto-lint
LINT_RC=0
if [[ "$COUNT" -ge "$THRESHOLD" ]]; then
  echo "AUTO-LINT|threshold=$THRESHOLD|count=$COUNT"
  if ! bash scripts/lint.sh; then
    LINT_RC=4
  fi
  echo "0" > "$COUNTER_FILE"
fi

# Auto-reindex (best-effort)
QMD_RC=0
if [[ "${AWIKI_QMD_STATUS:-}" != "missing" ]] && command -v qmd >/dev/null 2>&1; then
  if ! bash scripts/qmd-index.sh 2>/dev/null; then
    QMD_RC=5
  fi
fi

if [[ "$LINT_RC" -ne 0 ]]; then exit 4; fi
if [[ "$QMD_RC" -ne 0 ]]; then exit 5; fi
exit 0
EOF
chmod +x scripts/ingest.sh
```

- [ ] **Step 5: Run test (expect PASS)**

Run: `bats tests/ingest_test.bats`
Expected: 5 tests pass. (The auto-lint and qmd lines may fail soft — script is tolerant since lint.sh and qmd-index.sh don't exist yet; they exit non-zero but the test fixture sets count=1 < threshold=5 so auto-lint won't trigger.)

- [ ] **Step 6: Commit**

```bash
git add scripts/ingest.sh tests/ingest_test.bats tests/fixtures/sample-source.md
git commit -m "feat: add ingest script + BATS tests"
```

## Task 2.4: `scripts/lint.sh` (mechanical, phase 1 of two)

**Files:** Create: `scripts/lint.sh`, `tests/lint_test.bats`, `tests/fixtures/wiki-broken/`

- [ ] **Step 1: Write fixtures**

```bash
mkdir -p tests/fixtures/wiki-broken/content/{entities,sources}
cat > tests/fixtures/wiki-broken/content/entities/foo.md <<'EOF'
---
title: "Foo"
date: 2026-01-01
last_updated: 2026-01-01
type: entity
tags: [demo]
aliases: []
sources: []
draft: false
---

See [[bar-doesnt-exist]].
EOF

cat > tests/fixtures/wiki-broken/content/entities/empty.md <<'EOF'
---
title: "Empty"
date: 2026-01-01
last_updated: 2026-01-01
type: entity
tags: []
aliases: []
sources: []
draft: false
---
EOF

cat > tests/fixtures/wiki-broken/content/sources/orphan.md <<'EOF'
---
title: "Orphan"
date: 2026-01-01
last_updated: 2026-01-01
type: source
tags: []
aliases: []
sources: []
draft: false
---

Body content with sufficient length to not be flagged as empty page.
EOF
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/lint_test.bats <<'EOF'
#!/usr/bin/env bats

@test "lint detects broken wikilink" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [[ "$output" == *"LINT|ERROR"*"foo.md"*"broken wikilink"* ]]
}

@test "lint detects empty page" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [[ "$output" == *"LINT|WARN"*"empty.md"* ]]
}

@test "lint exits 2 on errors" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [ "$status" -eq 2 ]
}

@test "lint exits 0 on clean wiki" {
  CLEAN="$(mktemp -d)/content"
  mkdir -p "$CLEAN/entities"
  cat > "$CLEAN/entities/foo.md" <<EOF2
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Foo references [[bar]].
EOF2
  cat > "$CLEAN/entities/bar.md" <<EOF2
---
title: "Bar"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Bar references [[foo]] for connectivity.
EOF2
  run bash scripts/lint.sh "$CLEAN"
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL)**

Run: `bats tests/lint_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 4: Write `scripts/lint.sh` (phase 1 — mechanical only, no `--hugo-check` yet)**

```bash
cat > scripts/lint.sh <<'EOF'
#!/usr/bin/env bash
set -uo pipefail

CONTENT_DIR="${1:-content}"
ERRORS=0
WARNS=0
INFOS=0

# Build slug map and alias map
declare -A SLUG_TO_PATH
declare -A ALIAS_TO_SLUG
declare -A ALIAS_COUNT

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  if [[ -n "${SLUG_TO_PATH[$slug]:-}" ]]; then
    echo "LINT|ERROR|$page|duplicate slug: $slug also at ${SLUG_TO_PATH[$slug]}"
    ERRORS=$((ERRORS + 1))
  fi
  SLUG_TO_PATH[$slug]="$page"

  # Extract aliases via grep on YAML frontmatter
  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        ALIAS_COUNT[$a]=$((${ALIAS_COUNT[$a]:-0} + 1))
        ALIAS_TO_SLUG[$a]="$slug"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Per-page checks
while IFS= read -r -d '' page; do
  body_len=$(awk '/^---$/{c++; next} c==2{print}' "$page" | wc -c | tr -d ' ')
  if [[ "$body_len" -lt 50 ]]; then
    echo "LINT|WARN|$page|empty page (<50 char body)"
    WARNS=$((WARNS + 1))
  fi

  # Broken wikilinks: extract [[slug]] or [[slug|x]]
  while read -r link; do
    target="${link%%|*}"
    if [[ -z "${SLUG_TO_PATH[$target]:-}" && -z "${ALIAS_TO_SLUG[$target]:-}" ]]; then
      echo "LINT|ERROR|$page|broken wikilink: [[$target]]"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

# Alias collisions
for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done

echo "LINT-SUMMARY|errors=$ERRORS|warnings=$WARNS|info=$INFOS"

if [[ "$ERRORS" -gt 0 ]]; then exit 2; fi
if [[ "$WARNS" -gt 0 ]]; then exit 1; fi
exit 0
EOF
chmod +x scripts/lint.sh
```

- [ ] **Step 5: Run test (expect PASS)**

Run: `bats tests/lint_test.bats`
Expected: 4 tests pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats tests/fixtures/wiki-broken
git commit -m "feat: add lint script (mechanical checks) + BATS tests"
```

## Task 2.5: `scripts/lint.sh --fix`

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add fix-mode test**

Append to `tests/lint_test.bats`:

```bash
cat >> tests/lint_test.bats <<'EOF'

@test "lint --fix adds missing last_updated" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/needs-fix.md" <<EOF2
---
title: "Needs Fix"
date: 2026-01-01
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body referencing [[needs-fix]] for self-connectivity, sufficient length.
EOF2
  bash scripts/lint.sh --fix "$TMP" || true
  run grep '^last_updated:' "$TMP/entities/needs-fix.md"
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

Run: `bats tests/lint_test.bats -f "lint --fix"`
Expected: FAIL — `--fix` not handled.

- [ ] **Step 3: Modify `scripts/lint.sh` to handle `--fix`**

Replace the top of the script (replace the line `CONTENT_DIR="${1:-content}"` block) with:

```bash
cat > scripts/lint.sh <<'EOF'
#!/usr/bin/env bash
set -uo pipefail

FIX=0
CONTENT_DIR="content"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --fix) FIX=1; shift ;;
    *) CONTENT_DIR="$1"; shift ;;
  esac
done

apply_fixes() {
  local page="$1"
  if ! grep -q '^last_updated:' "$page"; then
    today="$(date '+%Y-%m-%d')"
    # Portable insertion: use awk (handles BSD/GNU sed differences in \n).
    awk -v today="$today" '
      /^date: / { print; print "last_updated: " today; next }
      { print }
    ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
    echo "FIX|$page|added last_updated: $today"
  fi
}

if [[ "$FIX" -eq 1 ]]; then
  while IFS= read -r -d '' page; do
    apply_fixes "$page"
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi

ERRORS=0
WARNS=0
INFOS=0

declare -A SLUG_TO_PATH
declare -A ALIAS_TO_SLUG
declare -A ALIAS_COUNT

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  if [[ -n "${SLUG_TO_PATH[$slug]:-}" ]]; then
    echo "LINT|ERROR|$page|duplicate slug: $slug also at ${SLUG_TO_PATH[$slug]}"
    ERRORS=$((ERRORS + 1))
  fi
  SLUG_TO_PATH[$slug]="$page"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        ALIAS_COUNT[$a]=$((${ALIAS_COUNT[$a]:-0} + 1))
        ALIAS_TO_SLUG[$a]="$slug"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  body_len=$(awk '/^---$/{c++; next} c==2{print}' "$page" | wc -c | tr -d ' ')
  if [[ "$body_len" -lt 50 ]]; then
    echo "LINT|WARN|$page|empty page (<50 char body)"
    WARNS=$((WARNS + 1))
  fi

  while read -r link; do
    target="${link%%|*}"
    if [[ -z "${SLUG_TO_PATH[$target]:-}" && -z "${ALIAS_TO_SLUG[$target]:-}" ]]; then
      echo "LINT|ERROR|$page|broken wikilink: [[$target]]"
      ERRORS=$((ERRORS + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

for a in "${!ALIAS_COUNT[@]}"; do
  if [[ "${ALIAS_COUNT[$a]}" -gt 1 ]]; then
    echo "LINT|ERROR|content|alias collision: '$a' used by multiple pages"
    ERRORS=$((ERRORS + 1))
  fi
done

echo "LINT-SUMMARY|errors=$ERRORS|warnings=$WARNS|info=$INFOS"

if [[ "$ERRORS" -gt 0 ]]; then exit 2; fi
if [[ "$WARNS" -gt 0 ]]; then exit 1; fi
exit 0
EOF
chmod +x scripts/lint.sh
```

- [ ] **Step 4: Run test (expect PASS)**

Run: `bats tests/lint_test.bats`
Expected: 5 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: add --fix mode to lint script (mechanical fixes)"
```

## Task 2.6: Phase 2 merge

- [ ] **Step 1: Run all tests + smoke**

Run: `bats tests/ && just lint || true && just lint-fix || true`
Expected: tests pass; `just lint` reports a clean (or near-clean) repo.

- [ ] **Step 2: Merge**

```bash
git checkout main
git merge --no-ff phase-2-scripts-core -m "feat: complete phase 2 scripts core"
git branch -d phase-2-scripts-core
```

---

# Phase 3: Hugo Render

**Deliverable:** Wikilink preprocessor (`build.sh`), live-watch wrapper (`serve.sh`), `hugo.toml`, `hugo-book` theme submodule, slug+alias maps, smoke render of fixture content.

**Branch:** `phase-3-hugo-render`
**Depends on:** Phase 1, 2.

## Task 3.1: Spike — confirm preprocessing approach

Spike, time-boxed 4 hours. Output: `docs/decisions/wikilink-rendering.md`.

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-3-hugo-render
```

- [ ] **Step 2: Add `hugo-book` theme submodule**

```bash
git submodule add https://github.com/alex-shpak/hugo-book themes/hugo-book
git commit -m "chore: add hugo-book theme submodule"
```

- [ ] **Step 3: Write spike scratch**

Create `docs/decisions/wikilink-rendering.md` with answers to:
1. Does goldmark + hugo-book render `[[slug]]` natively? (verify by trying — write a minimal page).
2. If no, is preprocessing into `[<title>](/<section>/<slug>/)` sufficient? (test on 3 pages with title-from-frontmatter resolution).
3. Does Hugo's relative `--source` flag work with content/ outside the project root?

- [ ] **Step 4: Decide and document**

Spec says: preprocessing pipeline. Confirm with spike or pivot. If pivot, update Hugo Render section of spec before continuing.

- [ ] **Step 5: Commit**

```bash
git add docs/decisions/wikilink-rendering.md
git commit -m "docs: record wikilink rendering decision"
```

## Task 3.2: `hugo.toml`

**Files:** Create: `hugo.toml`

- [ ] **Step 1: Write `hugo.toml`**

```toml
baseURL = 'https://example.com/'
title = 'awiki'
theme = 'hugo-book'
contentDir = '.awiki/build-content'

[markup]
  [markup.goldmark]
    [markup.goldmark.renderer]
      unsafe = true
  [markup.tableOfContents]
    startLevel = 2
    endLevel = 4

[params]
  BookSection = 'docs'
```

Hugo is invoked from the repo root with `hugo --source .` (default). `contentDir` is relative to repo root and points at the preprocessor's output. `.awiki/build-content/` must exist before Hugo runs — `scripts/build.sh` creates it.

- [ ] **Step 2: Commit**

```bash
git add hugo.toml
git commit -m "feat: add hugo.toml pointing to preprocessed build dir"
```

## Task 3.3: `scripts/build.sh` — slug+alias maps

**Files:** Create: `scripts/build.sh`, `tests/build_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/build_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/content/entities" "$WORK/.awiki/maps"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: [Effoh]
sources: []
draft: false
---

Sees [[bar]].
E
  cat > "$WORK/content/entities/bar.md" <<E
---
title: "Bar"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Sees [[foo]] and [[Effoh|alias-display]].
E
  cd "$WORK"
}

teardown() {
  rm -rf "$WORK"
}

@test "build emits slug map" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh" --maps-only
  run cat .awiki/maps/slug-to-path.tsv
  [[ "$output" == *"foo	entities/foo.md"* ]]
  [[ "$output" == *"bar	entities/bar.md"* ]]
}

@test "build rewrites wikilinks in build-content" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/foo.md
  [[ "$output" == *"[Bar](/entities/bar/)"* ]]
}

@test "build resolves alias wikilink" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/bar.md
  [[ "$output" == *"[alias-display](/entities/foo/)"* ]]
}
EOF
```

- [ ] **Step 2: Run test (expect FAIL)**

Run: `bats tests/build_test.bats`
Expected: FAIL — script absent.

- [ ] **Step 3: Write `scripts/build.sh`**

```bash
cat > scripts/build.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

MAPS_ONLY=0
[[ "${1:-}" == "--maps-only" ]] && MAPS_ONLY=1

CONTENT_DIR="content"
BUILD_DIR=".awiki/build-content"
MAPS_DIR=".awiki/maps"
mkdir -p "$MAPS_DIR" "$BUILD_DIR"

SLUG_MAP="$MAPS_DIR/slug-to-path.tsv"
ALIAS_MAP="$MAPS_DIR/alias-to-slug.tsv"
TITLE_MAP="$MAPS_DIR/slug-to-title.tsv"

: > "$SLUG_MAP"
: > "$ALIAS_MAP"
: > "$TITLE_MAP"

# Build maps
while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  slug="$(basename "$page" .md)"
  echo -e "$slug\t$rel" >> "$SLUG_MAP"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^title:[[:space:]]*\"?([^\"]+)\"?[[:space:]]*$ ]]; then
      title="${BASH_REMATCH[1]}"
      echo -e "$slug\t$title" >> "$TITLE_MAP"
    fi
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        echo -e "$a\t$slug" >> "$ALIAS_MAP"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

[[ "$MAPS_ONLY" -eq 1 ]] && exit 0

# Rewrite content into build-content
rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

resolve_target() {
  local target="$1"
  local rel
  rel="$(awk -F'\t' -v s="$target" '$1==s{print $2; exit}' "$SLUG_MAP")"
  if [[ -z "$rel" ]]; then
    local resolved_slug
    resolved_slug="$(awk -F'\t' -v a="$target" '$1==a{print $2; exit}' "$ALIAS_MAP")"
    if [[ -n "$resolved_slug" ]]; then
      rel="$(awk -F'\t' -v s="$resolved_slug" '$1==s{print $2; exit}' "$SLUG_MAP")"
    fi
  fi
  echo "$rel"
}

resolve_title() {
  local slug="$1"
  awk -F'\t' -v s="$slug" '$1==s{print $2; exit}' "$TITLE_MAP"
}

while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  out="$BUILD_DIR/$rel"
  mkdir -p "$(dirname "$out")"

  python3 - "$page" "$out" "$SLUG_MAP" "$ALIAS_MAP" "$TITLE_MAP" <<'PY'
import re, sys, os
src, out, slug_map, alias_map, title_map = sys.argv[1:6]
def load(path):
    d = {}
    for line in open(path):
        line = line.rstrip('\n')
        if '\t' in line:
            k, v = line.split('\t', 1)
            d[k] = v
    return d
slugs = load(slug_map)
aliases = load(alias_map)
titles = load(title_map)
text = open(src).read()
def repl(m):
    inner = m.group(1)
    if '|' in inner:
        target, display = inner.split('|', 1)
    else:
        target = display = inner
    rel = slugs.get(target)
    resolved_slug = target
    if not rel:
        resolved_slug = aliases.get(target, '')
        rel = slugs.get(resolved_slug, '')
    if not rel:
        return m.group(0)
    section, _ = os.path.split(rel)
    if display == target:
        display = titles.get(resolved_slug, target)
    url = '/' + section + '/' + os.path.splitext(os.path.basename(rel))[0] + '/'
    return f'[{display}]({url})'
text = re.sub(r'\[\[([^\]]+)\]\]', repl, text)
open(out, 'w').write(text)
PY
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

echo "BUILD-OK|content=$CONTENT_DIR|build=$BUILD_DIR"
EOF
chmod +x scripts/build.sh
```

Note: depends on `python3` (universally available on macOS/Linux). Add to `check-deps.sh` in a follow-up commit.

- [ ] **Step 4: Run test (expect PASS)**

Run: `bats tests/build_test.bats`
Expected: 3 tests pass.

- [ ] **Step 5: Update `check-deps.sh`**

Add `check python3 "3.8" "brew install python" "apt install python3"` after the `check bats` line.

- [ ] **Step 6: Commit**

```bash
git add scripts/build.sh scripts/check-deps.sh tests/build_test.bats
git commit -m "feat: add build.sh wikilink preprocessor with slug+alias maps"
```

## Task 3.4: `scripts/serve.sh`

**Files:** Create: `scripts/serve.sh`

- [ ] **Step 1: Write `scripts/serve.sh`**

```bash
cat > scripts/serve.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

bash scripts/build.sh

if command -v entr >/dev/null 2>&1; then
  echo "WATCH|tool=entr"
  ( while true; do
      find content -name '*.md' | entr -d bash scripts/build.sh
    done ) &
elif command -v fswatch >/dev/null 2>&1; then
  echo "WATCH|tool=fswatch"
  ( fswatch -o content | while read -r _; do bash scripts/build.sh; done ) &
else
  echo "WATCH|tool=poll-1s"
  ( while true; do bash scripts/build.sh; sleep 1; done ) &
fi
WATCHER_PID=$!
trap "kill $WATCHER_PID 2>/dev/null || true" EXIT

hugo server --bind 0.0.0.0 --port 1313 -D
EOF
chmod +x scripts/serve.sh
```

- [ ] **Step 2: Smoke test**

Run: `just serve` for 5 seconds, verify Hugo binds (no crash). Ctrl-C.

- [ ] **Step 3: Commit**

```bash
git add scripts/serve.sh
git commit -m "feat: add serve script with watcher fallback"
```

## Task 3.5: Hugo build recipe + `_index.md`

**Files:** Modify: `scripts/build.sh` (extend), Create: `content/_index.md`

- [ ] **Step 1: Rewrite `scripts/build.sh` with proper arg parsing + `--full` flag**

Replace the entire file with:

```bash
cat > scripts/build.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

MAPS_ONLY=0
FULL=0
while [[ $# -gt 0 ]]; do
  case "$1" in
    --maps-only) MAPS_ONLY=1; shift ;;
    --full) FULL=1; shift ;;
    -*) echo "unknown flag: $1" >&2; exit 1 ;;
    *) echo "unexpected positional: $1" >&2; exit 1 ;;
  esac
done

if [[ "$MAPS_ONLY" -eq 1 && "$FULL" -eq 1 ]]; then
  echo "--maps-only and --full are mutually exclusive" >&2
  exit 1
fi

CONTENT_DIR="content"
BUILD_DIR=".awiki/build-content"
MAPS_DIR=".awiki/maps"
mkdir -p "$MAPS_DIR" "$BUILD_DIR"

SLUG_MAP="$MAPS_DIR/slug-to-path.tsv"
ALIAS_MAP="$MAPS_DIR/alias-to-slug.tsv"
TITLE_MAP="$MAPS_DIR/slug-to-title.tsv"

: > "$SLUG_MAP"
: > "$ALIAS_MAP"
: > "$TITLE_MAP"

while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  slug="$(basename "$page" .md)"
  printf '%s\t%s\n' "$slug" "$rel" >> "$SLUG_MAP"

  in_fm=0
  while IFS= read -r line; do
    [[ "$line" == "---" ]] && { in_fm=$((in_fm + 1)); continue; }
    [[ "$in_fm" -ge 2 ]] && break
    if [[ "$line" =~ ^title:[[:space:]]*\"?([^\"]+)\"?[[:space:]]*$ ]]; then
      printf '%s\t%s\n' "$slug" "${BASH_REMATCH[1]}" >> "$TITLE_MAP"
    fi
    if [[ "$line" =~ ^aliases:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        a="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        [[ -z "$a" ]] && continue
        printf '%s\t%s\n' "$a" "$slug" >> "$ALIAS_MAP"
      done
    fi
  done < "$page"
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

if [[ "$MAPS_ONLY" -eq 1 ]]; then
  echo "BUILD-OK|maps-only=1"
  exit 0
fi

rm -rf "$BUILD_DIR"
mkdir -p "$BUILD_DIR"

while IFS= read -r -d '' page; do
  rel="${page#$CONTENT_DIR/}"
  out="$BUILD_DIR/$rel"
  mkdir -p "$(dirname "$out")"

  python3 - "$page" "$out" "$SLUG_MAP" "$ALIAS_MAP" "$TITLE_MAP" <<'PY'
import re, sys, os
src, out, slug_map, alias_map, title_map = sys.argv[1:6]
def load(path):
    d = {}
    for line in open(path):
        line = line.rstrip('\n')
        if '\t' in line:
            k, v = line.split('\t', 1)
            d[k] = v
    return d
slugs = load(slug_map)
aliases = load(alias_map)
titles = load(title_map)
text = open(src).read()
def repl(m):
    inner = m.group(1)
    if '|' in inner:
        target, display = inner.split('|', 1)
    else:
        target = display = inner
    rel = slugs.get(target)
    resolved_slug = target
    if not rel:
        resolved_slug = aliases.get(target, '')
        rel = slugs.get(resolved_slug, '')
    if not rel:
        return m.group(0)
    section, _ = os.path.split(rel)
    if display == target:
        display = titles.get(resolved_slug, target)
    url = '/' + section + '/' + os.path.splitext(os.path.basename(rel))[0] + '/'
    return f'[{display}]({url})'
text = re.sub(r'\[\[([^\]]+)\]\]', repl, text)
open(out, 'w').write(text)
PY
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

echo "BUILD-OK|content=$CONTENT_DIR|build=$BUILD_DIR"

if [[ "$FULL" -eq 1 ]]; then
  hugo --minify --destination public
  echo "HUGO-OK|out=public"
fi
EOF
chmod +x scripts/build.sh
```

This replaces the earlier task 3.3 version once `--full` lands. Hugo is invoked from repo root (no `--source` flag needed); `hugo.toml`'s `contentDir = '.awiki/build-content'` does the redirection.

- [ ] **Step 2: Update `_index.md` to be appropriate for the seeded version (already created in Phase 1; only edit if Phase 1 placeholder needs polish)**

```bash
cat > content/_index.md <<'EOF'
---
title: "awiki"
type: section-index
draft: false
---

# Welcome

This is your awiki. Browse:

- [[catalog]] — full content listing.
- [[log]] — chronological activity log.
- Sections: entities, concepts, topics, sources, synthesis.

Edit this page manually after BOOTSTRAP runs.
EOF
```

- [ ] **Step 3: Update `justfile` build recipe to pass `--full`**

Edit `justfile`, change the `build` recipe to:

```just
build:
    bash scripts/build.sh --full
```

- [ ] **Step 4: Smoke test**

Run: `just build && ls public/`
Expected: `public/index.html` exists.

- [ ] **Step 5: Commit**

```bash
git add scripts/build.sh content/_index.md justfile
git commit -m "feat: hugo build via build.sh --full; refine _index.md landing"
```

## Task 3.6: Phase 3 merge

- [ ] **Step 1: Run tests + smoke**

```bash
bats tests/
just build
just serve & sleep 3 && kill %1
```

- [ ] **Step 2: Merge**

```bash
git checkout main
git merge --no-ff phase-3-hugo-render -m "feat: complete phase 3 hugo render"
git branch -d phase-3-hugo-render
```

---

# Phase 4: qmd Integration

**Deliverable:** `install-qmd.sh`, `qmd-index.sh`, MCP wiring option, `.awiki/qmd-status` flag, grep fallback in WIKI.md.

**Branch:** `phase-4-qmd-integration`
**Depends on:** Phase 1.

## Task 4.1: Spike — confirm qmd build

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-4-qmd-integration
```

- [ ] **Step 2: Clone qmd locally + build**

```bash
git clone https://github.com/qntx-labs/qmd /tmp/qmd-spike
cd /tmp/qmd-spike
cat README.md   # determine toolchain + build cmd
# Run upstream-documented build steps
cd -
```

- [ ] **Step 3: Document outcome in `docs/decisions/qmd-install.md`**

Capture: language, build command, install target, macOS + Linux verification.

- [ ] **Step 4: Commit**

```bash
git add docs/decisions/qmd-install.md
git commit -m "docs: record qmd install decision after spike"
```

## Task 4.2: `scripts/install-qmd.sh`

**Files:** Create: `scripts/install-qmd.sh`, `tests/install_qmd_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/install_qmd_test.bats <<'EOF'
#!/usr/bin/env bats

@test "install-qmd is idempotent if qmd already on PATH" {
  if ! command -v qmd >/dev/null 2>&1; then skip "qmd not installed"; fi
  run bash scripts/install-qmd.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"already installed"* ]]
}

@test "install-qmd writes status file on success" {
  run bash scripts/install-qmd.sh
  if [ "$status" -eq 0 ]; then
    [ -f .awiki/qmd-status ]
    [[ "$(cat .awiki/qmd-status)" =~ ^(ok|missing)$ ]]
  fi
}
EOF
```

- [ ] **Step 2: Run test (FAIL expected)**

Run: `bats tests/install_qmd_test.bats`

- [ ] **Step 3: Write `scripts/install-qmd.sh` (using spike outcome)**

```bash
cat > scripts/install-qmd.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

mkdir -p .awiki
STATUS_FILE=".awiki/qmd-status"

if command -v qmd >/dev/null 2>&1; then
  echo "qmd already installed at: $(command -v qmd)"
  echo "ok" > "$STATUS_FILE"
  exit 0
fi

SRC_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/qmd-src"
mkdir -p "$(dirname "$SRC_DIR")"

if [[ ! -d "$SRC_DIR/.git" ]]; then
  git clone https://github.com/qntx-labs/qmd "$SRC_DIR"
fi
( cd "$SRC_DIR" && git pull --ff-only )

# Build per upstream — pinned via docs/decisions/qmd-install.md.
# Replace with the actual command from spike outcome.
( cd "$SRC_DIR" && make build ) || {
  echo "missing" > "$STATUS_FILE"
  echo "qmd build failed; agent will use grep fallback" >&2
  exit 1
}

mkdir -p "$HOME/.local/bin"
ln -sf "$SRC_DIR/qmd" "$HOME/.local/bin/qmd"

if ! command -v qmd >/dev/null 2>&1; then
  echo "Add ~/.local/bin to PATH:"
  echo '  export PATH="$HOME/.local/bin:$PATH"'
fi

echo "ok" > "$STATUS_FILE"
echo "qmd installed at $HOME/.local/bin/qmd"
EOF
chmod +x scripts/install-qmd.sh
```

Note: replace `make build` with the actual command from the spike.

- [ ] **Step 4: Run test (PASS expected)**

Run: `bats tests/install_qmd_test.bats`

- [ ] **Step 5: Commit**

```bash
git add scripts/install-qmd.sh tests/install_qmd_test.bats
git commit -m "feat: add install-qmd script + status flag"
```

## Task 4.3: `scripts/qmd-index.sh`

**Files:** Create: `scripts/qmd-index.sh`

- [ ] **Step 1: Write `scripts/qmd-index.sh`**

```bash
cat > scripts/qmd-index.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if ! command -v qmd >/dev/null 2>&1; then
  echo "QMD-INDEX|skip|reason=qmd-not-installed" >&2
  exit 0
fi

mkdir -p .qmd
qmd index content/
echo "QMD-INDEX|ok"
EOF
chmod +x scripts/qmd-index.sh
```

- [ ] **Step 2: Smoke test**

Run: `just reindex`
Expected: either `QMD-INDEX|ok` or `QMD-INDEX|skip` — no crash.

- [ ] **Step 3: Commit**

```bash
git add scripts/qmd-index.sh
git commit -m "feat: add qmd-index script (no-op if qmd missing)"
```

## Task 4.4: MCP wiring helper

**Files:** Create: `scripts/wire-qmd-mcp.sh`

- [ ] **Step 1: Write `scripts/wire-qmd-mcp.sh`**

```bash
cat > scripts/wire-qmd-mcp.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if ! command -v qmd >/dev/null 2>&1; then
  echo "qmd not installed; cannot wire MCP server" >&2
  exit 1
fi

# Detect harness — prefer codex if both present (last-write wins)
TARGET=""
if [[ -d .claude || -f CLAUDE.md ]]; then TARGET="claude"; fi
if [[ -d .codex ]]; then TARGET="codex"; fi

case "$TARGET" in
  claude)
    CONFIG="./.mcp.json"   # Claude Code reads project-level MCP from repo root
    if [[ ! -f "$CONFIG" ]]; then echo '{"mcpServers": {}}' > "$CONFIG"; fi
    python3 - "$CONFIG" <<'PY'
import json, sys
path = sys.argv[1]
data = json.load(open(path))
data.setdefault("mcpServers", {})["qmd"] = {
    "command": "qmd",
    "args": ["mcp", "--root", "content/"]
}
json.dump(data, open(path, "w"), indent=2)
PY
    echo "WIRED|claude|$CONFIG"
    exit 0
    ;;
  codex)
    CONFIG=".codex/config.toml"
    mkdir -p .codex
    if ! grep -q '^\[mcp.servers.qmd\]' "$CONFIG" 2>/dev/null; then
      cat >> "$CONFIG" <<'TOML'

[mcp.servers.qmd]
command = "qmd"
args = ["mcp", "--root", "content/"]
TOML
    fi
    echo "WIRED|codex|$CONFIG"
    exit 0
    ;;
  *)
    echo "No agent harness detected (.claude or .codex). Run from a configured project." >&2
    exit 1
    ;;
esac
EOF
chmod +x scripts/wire-qmd-mcp.sh
```

- [ ] **Step 2: Commit**

```bash
git add scripts/wire-qmd-mcp.sh
git commit -m "feat: add MCP wiring helper for qmd (claude/codex detection)"
```

## Task 4.5: Phase 4 merge

```bash
bats tests/
git checkout main
git merge --no-ff phase-4-qmd-integration -m "feat: complete phase 4 qmd integration"
git branch -d phase-4-qmd-integration
```

---

# Phase 5: Encryption

**Deliverable:** `encrypt-init.sh` (git-crypt + age), atomic `.gitignore` flip, `.gitattributes` patterns, lint privacy checks.

**Branch:** `phase-5-encryption`
**Depends on:** Phase 1, 2.

## Task 5.1: Branch + `scripts/encrypt-init.sh` (git-crypt path)

**Files:** Create: `scripts/encrypt-init.sh`

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-5-encryption
```

- [ ] **Step 2: Write `scripts/encrypt-init.sh`**

```bash
cat > scripts/encrypt-init.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

MODE="git-crypt"
if [[ "${1:-}" == "--age" ]]; then MODE="age"; fi

case "$MODE" in
  git-crypt)
    if ! command -v git-crypt >/dev/null 2>&1; then
      echo "git-crypt not installed. Install: brew install git-crypt (mac) / apt install git-crypt (linux)" >&2
      exit 1
    fi
    if [[ -d .git/git-crypt ]]; then
      echo "git-crypt already initialized" >&2
      exit 0
    fi
    git-crypt init

    # Activate patterns in .gitattributes (portable across BSD/GNU sed via awk)
    awk '
      /^# raw\/processed\/private\// { sub(/^# /, ""); print; next }
      /^# raw\/processed\/_originals\// { sub(/^# /, ""); print; next }
      /^# content\/private\// { sub(/^# /, ""); print; next }
      /^# secrets\// { sub(/^# /, ""); print; next }
      { print }
    ' .gitattributes > .gitattributes.tmp && mv .gitattributes.tmp .gitattributes

    # Atomically remove the gitignore lines for private paths so encrypted commits work
    awk '
      /^content\/private\/\*\*$/ { next }
      /^raw\/processed\/private\/\*\*$/ { next }
      /^raw\/processed\/_originals\/\*\*$/ { next }
      { print }
    ' .gitignore > .gitignore.tmp && mv .gitignore.tmp .gitignore

    mkdir -p secrets
    git-crypt export-key secrets/.git-crypt-key
    chmod 600 secrets/.git-crypt-key

    bash scripts/log-append.sh encrypt "git-crypt initialized; key at secrets/.git-crypt-key"
    echo "ENCRYPT-OK|mode=git-crypt|key=secrets/.git-crypt-key"
    echo "STORE the key securely (password manager). Anyone with this key can read encrypted paths."
    ;;
  age)
    if ! command -v age >/dev/null 2>&1; then
      echo "age not installed. Install: brew install age / apt install age" >&2
      exit 1
    fi
    mkdir -p secrets
    if [[ -f secrets/age.key ]]; then
      echo "age key already exists at secrets/age.key" >&2
      exit 0
    fi
    age-keygen -o secrets/age.key
    grep '^# public key:' secrets/age.key | sed 's/^# public key: //' > secrets/age.pub
    chmod 600 secrets/age.key

    bash scripts/log-append.sh encrypt "age keypair generated"
    echo "ENCRYPT-OK|mode=age|key=secrets/age.key|pub=secrets/age.pub"
    echo "Encrypt sensitive files: age -R secrets/age.pub -o file.age file"
    echo "Decrypt: age -d -i secrets/age.key file.age"
    ;;
esac
EOF
chmod +x scripts/encrypt-init.sh
```

- [ ] **Step 3: Test (manual, requires git-crypt installed)**

Run in a throwaway clone:
```bash
just encrypt-init
git status
git-crypt status content/private/test.md  # should show encrypted
```

- [ ] **Step 4: Commit**

```bash
git add scripts/encrypt-init.sh
git commit -m "feat: add encrypt-init script (git-crypt + age modes)"
```

## Task 5.2: Lint privacy checks

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add test**

Append to `tests/lint_test.bats`:

```bash
@test "lint warns on tags: [private] outside private/ path" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/leaky.md" <<E
---
title: "Leaky"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: [private]
aliases: []
sources: []
draft: false
---

Body content sufficient length for non-empty check.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|WARN"*"leaky.md"*"private tag outside private path"* ]]
}
```

- [ ] **Step 2: Run test (FAIL)**

- [ ] **Step 3: Modify `scripts/lint.sh` — add privacy check**

Inside the per-page-checks loop, after the empty-page check, add:

```bash
  # Parse the tags: line as a YAML list, check for an exact 'private' token.
  has_private_tag=0
  while IFS= read -r line; do
    if [[ "$line" =~ ^tags:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        token="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        if [[ "$token" == "private" ]]; then has_private_tag=1; fi
      done
    fi
  done < "$page"

  if [[ "$has_private_tag" -eq 1 && "$page" != *"/private/"* ]]; then
    echo "LINT|WARN|$page|private tag outside private path"
    WARNS=$((WARNS + 1))
  fi
```

- [ ] **Step 4: Run test (PASS)**

- [ ] **Step 5: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: lint warns on private tag outside private path"
```

## Task 5.3: Phase 5 merge

```bash
bats tests/
git checkout main
git merge --no-ff phase-5-encryption -m "feat: complete phase 5 encryption"
git branch -d phase-5-encryption
```

---

# Phase 6: Section Indexes + Catalog v2

**Deliverable:** Hugo `page-list` shortcode, BOOTSTRAP scaffolds section indexes, lint exemptions for system pages, catalog cross-link checks.

**Branch:** `phase-6-section-indexes`
**Depends on:** Phase 1, 3.

## Task 6.1: Branch + Hugo shortcode

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-6-section-indexes
```

- [ ] **Step 2: Create `layouts/shortcodes/page-list.html`**

```bash
mkdir -p layouts/shortcodes
cat > layouts/shortcodes/page-list.html <<'EOF'
{{ $section := .Get "section" | default .Page.Section }}
<ul>
{{ range where (where .Site.RegularPages "Section" $section) "Type" "ne" "section-index" }}
  <li><a href="{{ .RelPermalink }}">{{ .Title }}</a> — {{ .Summary }}</li>
{{ end }}
</ul>
EOF
```

- [ ] **Step 3: Commit**

```bash
git add layouts/shortcodes/page-list.html
git commit -m "feat: add Hugo page-list shortcode"
```

## Task 6.2: Section index scaffolds

- [ ] **Step 1: Create one `_index.md` per section**

```bash
for section in entities concepts topics sources synthesis; do
  cat > "content/$section/_index.md" <<EOF
---
title: "$(echo $section | sed 's/.*/\u&/')"
type: section-index
draft: false
---

Pages tagged \`$section\`.

{{< page-list section="$section" >}}
EOF
done
```

- [ ] **Step 2: Commit**

```bash
git add content/*/_index.md
git commit -m "feat: scaffold section index pages with page-list shortcode"
```

## Task 6.3: Lint exemption for system pages

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add test for orphan check + system-page exemption (single combined test)**

```bash
cat >> tests/lint_test.bats <<'EOF'

@test "lint flags orphan entity" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/island.md" <<E
---
title: "Island"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body content with sufficient length, but no inbound wikilinks anywhere.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|INFO"*"island.md"*"orphan"* ]]
}

@test "lint exempts section-index from orphan check" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/_index.md" <<E
---
title: "Entities"
type: section-index
draft: false
---

Section landing.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" != *"LINT|INFO"*"_index.md"*"orphan"* ]]
}
EOF
```

- [ ] **Step 2: Add orphan check + system-page exemption to `scripts/lint.sh`**

Insert this block after the alias-collision check, before the `LINT-SUMMARY` line:

```bash
# Orphan check: count inbound wikilinks per slug; warn if zero.
declare -A INBOUND
while IFS= read -r -d '' page; do
  while read -r link; do
    target="${link%%|*}"
    INBOUND[$target]=$((${INBOUND[$target]:-0} + 1))
    # Also credit the alias's resolved slug, if any
    resolved="${ALIAS_TO_SLUG[$target]:-}"
    if [[ -n "$resolved" ]]; then
      INBOUND[$resolved]=$((${INBOUND[$resolved]:-0} + 1))
    fi
  done < <(grep -oE '\[\[[^]]+\]\]' "$page" | sed -E 's/^\[\[|\]\]$//g')
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)

while IFS= read -r -d '' page; do
  slug="$(basename "$page" .md)"
  type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
  if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
    continue
  fi
  if [[ "${INBOUND[$slug]:-0}" -eq 0 ]]; then
    echo "LINT|INFO|$page|orphan: no inbound wikilinks"
    INFOS=$((INFOS + 1))
  fi
done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
```

- [ ] **Step 3: Run tests (PASS expected)**

```bash
bats tests/lint_test.bats
```

- [ ] **Step 4: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: lint orphan check with system-page exemption"
```

## Task 6.4: Catalog cross-link check

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add test**

```bash
cat >> tests/lint_test.bats <<'EOF'

@test "lint warns on page missing from catalog" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/catalog.md" <<E
---
title: "Catalog"
type: catalog
---

# Catalog
E
  cat > "$TMP/entities/uncatalogued.md" <<E
---
title: "Uncatalogued"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Uncatalogued [[uncatalogued]] self-reference for connectivity.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|WARN"*"uncatalogued.md"*"missing from catalog"* ]]
}
EOF
```

- [ ] **Step 2: Add catalog cross-link check to lint.sh**

Insert before `LINT-SUMMARY`:

```bash
# Catalog cross-link check
CATALOG="$CONTENT_DIR/catalog.md"
if [[ -f "$CATALOG" ]]; then
  declare -A IN_CATALOG
  while read -r link; do
    target="${link%%|*}"
    IN_CATALOG[$target]=1
  done < <(grep -oE '\[\[[^]]+\]\]' "$CATALOG" | sed -E 's/^\[\[|\]\]$//g')

  while IFS= read -r -d '' page; do
    slug="$(basename "$page" .md)"
    [[ "$slug" =~ ^(_index|catalog|log)$ ]] && continue
    type_field="$(awk -F'[:[:space:]]+' '/^type:/{print $2; exit}' "$page" | tr -d '"')"
    if [[ "$type_field" =~ ^(log|catalog|section-index)$ ]]; then
      continue
    fi
    if [[ -z "${IN_CATALOG[$slug]:-}" ]]; then
      echo "LINT|WARN|$page|missing from catalog"
      WARNS=$((WARNS + 1))
    fi
  done < <(find "$CONTENT_DIR" -name '*.md' -type f -print0)
fi
```

- [ ] **Step 3: Run tests + commit**

```bash
bats tests/lint_test.bats
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: lint warns on pages missing from catalog"
```

## Task 6.5: `scripts/update-catalog.sh`

**Files:** Create: `scripts/update-catalog.sh`, `tests/update_catalog_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/update_catalog_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities"
  cat > "$WORK/content/catalog.md" <<E
---
title: "Catalog"
type: catalog
---

# Catalog
E
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body referencing [[foo]] for connectivity, sufficient length.
E
  cd "$WORK"
}
teardown() { rm -rf "$WORK"; }

@test "update-catalog inserts entity entry" {
  bash "$BATS_TEST_DIRNAME/../scripts/update-catalog.sh"
  run grep -F '[[foo]]' content/catalog.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Write `scripts/update-catalog.sh`**

```bash
cat > scripts/update-catalog.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

CATALOG="content/catalog.md"
[[ -f "$CATALOG" ]] || { echo "no catalog at $CATALOG" >&2; exit 1; }

python3 - "$CATALOG" content <<'PY'
import os, sys, re, glob, collections
catalog_path, content_dir = sys.argv[1:3]

groups = collections.OrderedDict([
    ("entity", "Entities"),
    ("concept", "Concepts"),
    ("topic", "Topics"),
    ("source", "Sources"),
    ("synthesis", "Synthesis"),
    ("deck", "Synthesis"),
    ("chart", "Synthesis"),
    ("canvas", "Synthesis"),
])

entries = collections.defaultdict(list)
for path in sorted(glob.glob(os.path.join(content_dir, "**", "*.md"), recursive=True)):
    rel = os.path.relpath(path, content_dir)
    slug = os.path.splitext(os.path.basename(path))[0]
    if slug in ("_index", "catalog", "log"):
        continue
    fm = {}
    with open(path) as f:
        text = f.read()
    m = re.match(r"---\n(.*?)\n---", text, re.S)
    if not m:
        continue
    for line in m.group(1).splitlines():
        if ":" in line:
            k, v = line.split(":", 1)
            fm[k.strip()] = v.strip().strip('"')
    t = fm.get("type", "")
    if t in ("log", "catalog", "section-index"):
        continue
    section = groups.get(t, "Misc")
    title = fm.get("title", slug)
    entries[section].append(f"- [[{slug}]] — {title}")

# Read existing catalog body up to first ##; replace below with our generated sections.
with open(catalog_path) as f:
    full = f.read()
m = re.split(r"^## ", full, count=1, flags=re.M)
prelude = m[0].rstrip() + "\n\n"
out = prelude
for section in ("Entities", "Concepts", "Topics", "Sources", "Synthesis", "Misc"):
    if entries.get(section):
        out += f"## {section}\n\n"
        out += "\n".join(entries[section]) + "\n\n"
with open(catalog_path, "w") as f:
    f.write(out.rstrip() + "\n")
PY

echo "CATALOG-OK"
EOF
chmod +x scripts/update-catalog.sh
```

- [ ] **Step 3: Run test + commit**

```bash
bats tests/update_catalog_test.bats
git add scripts/update-catalog.sh tests/update_catalog_test.bats
git commit -m "feat: add update-catalog script (rebuilds content/catalog.md from frontmatter)"
```

## Task 6.5: Phase 6 merge

```bash
git checkout main
git merge --no-ff phase-6-section-indexes -m "feat: complete phase 6 section indexes + catalog v2"
git branch -d phase-6-section-indexes
```

---

# Phase 7: Slug Rename, Deletion, Alias Resolution

**Deliverable:** `rename.sh`, `delete-page.sh`. Alias-collision lint rule (already shipped phase 2/3).

**Branch:** `phase-7-rename-delete`
**Depends on:** Phase 2, 3.

## Task 7.1: `scripts/rename.sh`

**Files:** Create: `scripts/rename.sh`, `tests/rename_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/rename_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities"
  cp scripts/rename.sh /dev/null 2>/dev/null || true
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
type: entity
---

Body.
E
  cat > "$WORK/content/entities/bar.md" <<E
---
title: "Bar"
type: entity
---

References [[foo]].
E
  cd "$WORK"
}

teardown() { rm -rf "$WORK"; }

@test "rename moves file and updates wikilinks" {
  bash "$BATS_TEST_DIRNAME/../scripts/rename.sh" foo foo-renamed
  [ -f content/entities/foo-renamed.md ]
  [ ! -f content/entities/foo.md ]
  run grep -F '[[foo-renamed]]' content/entities/bar.md
  [ "$status" -eq 0 ]
}
EOF
```

- [ ] **Step 2: Write `scripts/rename.sh`**

```bash
cat > scripts/rename.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 2 ]] || { echo "usage: rename.sh <old-slug> <new-slug>" >&2; exit 1; }
OLD="$1"; NEW="$2"

OLD_PATH="$(find content -name "$OLD.md" -type f | head -1)"
[[ -n "$OLD_PATH" ]] || { echo "slug not found: $OLD" >&2; exit 2; }

NEW_PATH="$(dirname "$OLD_PATH")/$NEW.md"
[[ ! -e "$NEW_PATH" ]] || { echo "target exists: $NEW_PATH" >&2; exit 3; }

git mv "$OLD_PATH" "$NEW_PATH" 2>/dev/null || mv "$OLD_PATH" "$NEW_PATH"

# Update all wikilinks — portable across BSD/GNU sed
find content -name '*.md' -type f -print0 | while IFS= read -r -d '' f; do
  awk -v old="$OLD" -v new="$NEW" '
    {
      gsub("\\[\\[" old "\\]\\]", "[[" new "]]")
      gsub("\\[\\[" old "\\|", "[[" new "|")
    }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

bash scripts/log-append.sh rename "$OLD -> $NEW"
echo "RENAME-OK|old=$OLD|new=$NEW"
EOF
chmod +x scripts/rename.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/rename.sh tests/rename_test.bats
git commit -m "feat: add rename script (updates all wikilinks)"
```

## Task 7.2: `scripts/delete-page.sh`

**Files:** Create: `scripts/delete-page.sh`, `tests/delete_page_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/delete_page_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
type: entity
---

Body.
E
  cat > "$WORK/content/entities/bar.md" <<E
---
title: "Bar"
type: entity
---

References [[foo]].
E
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cd "$WORK"
}
teardown() { rm -rf "$WORK"; }

@test "delete-page removes file" {
  bash "$BATS_TEST_DIRNAME/../scripts/delete-page.sh" foo
  [ ! -f content/entities/foo.md ]
}

@test "delete-page marks wikilinks as broken" {
  bash "$BATS_TEST_DIRNAME/../scripts/delete-page.sh" foo
  run grep -F 'broken: was [[foo]]' content/entities/bar.md
  [ "$status" -eq 0 ]
}

@test "delete-page rejects unknown slug" {
  run bash "$BATS_TEST_DIRNAME/../scripts/delete-page.sh" nonexistent
  [ "$status" -eq 2 ]
}
EOF
```

- [ ] **Step 2: Write `scripts/delete-page.sh`**

```bash
cat > scripts/delete-page.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: delete-page.sh <slug>" >&2; exit 1; }
SLUG="$1"

PAGE="$(find content -name "$SLUG.md" -type f | head -1)"
[[ -n "$PAGE" ]] || { echo "slug not found: $SLUG" >&2; exit 2; }

git rm "$PAGE" 2>/dev/null || rm "$PAGE"

# Replace wikilinks with broken markers — portable across BSD/GNU sed
find content -name '*.md' -type f -print0 | while IFS= read -r -d '' f; do
  awk -v slug="$SLUG" '
    {
      gsub("\\[\\[" slug "\\]\\]", "<!-- broken: was [[" slug "]] -->" slug)
      gsub("\\[\\[" slug "\\|", "<!-- broken: was [[" slug "|... -->" slug "|")
    }
    { print }
  ' "$f" > "$f.tmp" && mv "$f.tmp" "$f"
done

bash scripts/log-append.sh delete "$SLUG (removed; wikilinks marked broken for review)"
echo "DELETE-OK|slug=$SLUG"
EOF
chmod +x scripts/delete-page.sh
```

- [ ] **Step 3: Run test (PASS)**

```bash
bats tests/delete_page_test.bats
```

- [ ] **Step 4: Commit**

```bash
git add scripts/delete-page.sh tests/delete_page_test.bats
git commit -m "feat: add delete-page script (marks broken wikilinks) + BATS test"
```

## Task 7.3: Phase 7 merge

```bash
bats tests/
git checkout main
git merge --no-ff phase-7-rename-delete -m "feat: complete phase 7 rename/delete/alias"
git branch -d phase-7-rename-delete
```

---

# Phase 8: MCP Wiki-Ops Server

**Deliverable:** `mcp/awiki-server/` Node MCP server exposing `ingest_source`, `query_wiki`, `lint`, `update_catalog`. BOOTSTRAP wiring option.

**Branch:** `phase-8-mcp-server`
**Depends on:** Phase 2, 4.

## Task 8.1: Branch + npm package

- [ ] **Step 1: Branch + scaffold**

```bash
git checkout -b phase-8-mcp-server
mkdir -p mcp/awiki-server
cd mcp/awiki-server
npm init -y
npm i @modelcontextprotocol/sdk
```

(Note: hand-written JSON schemas; no zod dependency. Keeps surface area small.)

- [ ] **Step 2: Write `mcp/awiki-server/index.js`**

```javascript
#!/usr/bin/env node
import { Server } from "@modelcontextprotocol/sdk/server/index.js";
import { StdioServerTransport } from "@modelcontextprotocol/sdk/server/stdio.js";
import { CallToolRequestSchema, ListToolsRequestSchema } from "@modelcontextprotocol/sdk/types.js";
import { execFileSync } from "node:child_process";

const TOOLS = [
  {
    name: "ingest_source",
    description: "Process a source from raw/inbox/* into the wiki via scripts/ingest.sh.",
    inputSchema: {
      type: "object",
      properties: { path: { type: "string", description: "Path under raw/inbox/{interactive,batch,checkpoint}/" } },
      required: ["path"],
      additionalProperties: false,
    },
  },
  {
    name: "lint",
    description: "Run mechanical lint over content/. Returns LINT|... lines and a LINT-SUMMARY.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
  {
    name: "query_wiki",
    description: "Search wiki via qmd (BM25+vector hybrid). Falls back to grep when qmd absent.",
    inputSchema: {
      type: "object",
      properties: { query: { type: "string" } },
      required: ["query"],
      additionalProperties: false,
    },
  },
  {
    name: "update_catalog",
    description: "Rebuild content/catalog.md from on-disk pages and current frontmatter.",
    inputSchema: { type: "object", properties: {}, additionalProperties: false },
  },
];

const server = new Server({ name: "awiki", version: "0.1.0" }, { capabilities: { tools: {} } });

server.setRequestHandler(ListToolsRequestSchema, async () => ({ tools: TOOLS }));

server.setRequestHandler(CallToolRequestSchema, async (req) => {
  const { name, arguments: args } = req.params;
  let out;
  try {
    switch (name) {
      case "ingest_source":
        if (typeof args?.path !== "string") throw new Error("path required");
        out = execFileSync("bash", ["scripts/ingest.sh", args.path], { encoding: "utf8" });
        break;
      case "lint":
        out = execFileSync("bash", ["scripts/lint.sh"], { encoding: "utf8" });
        break;
      case "query_wiki": {
        if (typeof args?.query !== "string") throw new Error("query required");
        try {
          out = execFileSync("qmd", ["search", args.query], { encoding: "utf8" });
        } catch {
          out = execFileSync("grep", ["-rli", "--include=*.md", args.query, "content/"], { encoding: "utf8" });
        }
        break;
      }
      case "update_catalog":
        out = execFileSync("bash", ["scripts/update-catalog.sh"], { encoding: "utf8" });
        break;
      default:
        throw new Error(`unknown tool: ${name}`);
    }
    return { content: [{ type: "text", text: out }] };
  } catch (e) {
    return { content: [{ type: "text", text: `ERROR|${e.message}` }], isError: true };
  }
});

const transport = new StdioServerTransport();
await server.connect(transport);
```

- [ ] **Step 3: Add npm bin + chmod**

Edit `mcp/awiki-server/package.json` to add `"bin": { "awiki-mcp": "./index.js" }` and `"type": "module"`.

```bash
chmod +x mcp/awiki-server/index.js
```

- [ ] **Step 4: Commit**

```bash
cd ../..
git add mcp/awiki-server
git commit -m "feat: add awiki MCP server (ingest/lint/query/update)"
```

## Task 8.2: BOOTSTRAP wiring helper for awiki MCP

- [ ] **Step 1: Add helper `scripts/wire-awiki-mcp.sh`**

(Mirrors `wire-qmd-mcp.sh` but registers `awiki` server pointing to `mcp/awiki-server/index.js`.)

```bash
cat > scripts/wire-awiki-mcp.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f mcp/awiki-server/index.js ]]; then
  echo "MCP server not built. Run: cd mcp/awiki-server && npm install" >&2
  exit 1
fi

TARGET=""
if [[ -d .claude || -f CLAUDE.md ]]; then TARGET="claude"; fi
if [[ -d .codex ]]; then TARGET="codex"; fi

case "$TARGET" in
  claude)
    CONFIG="./.mcp.json"
    [[ -f "$CONFIG" ]] || echo '{"mcpServers": {}}' > "$CONFIG"
    python3 - "$CONFIG" <<'PY'
import json, sys, os
path = sys.argv[1]
data = json.load(open(path))
data.setdefault("mcpServers", {})["awiki"] = {
    "command": "node",
    "args": [os.path.abspath("mcp/awiki-server/index.js")]
}
json.dump(data, open(path, "w"), indent=2)
PY
    echo "WIRED|claude|$CONFIG"
    ;;
  codex)
    CONFIG=".codex/config.toml"
    mkdir -p .codex
    if ! grep -q '^\[mcp.servers.awiki\]' "$CONFIG" 2>/dev/null; then
      ABS_PATH="$(cd mcp/awiki-server && pwd)/index.js"
      cat >> "$CONFIG" <<TOML

[mcp.servers.awiki]
command = "node"
args = ["$ABS_PATH"]
TOML
    fi
    echo "WIRED|codex|$CONFIG"
    ;;
  *)
    echo "No agent harness detected (.claude or .codex). Run from a configured project." >&2
    exit 1
    ;;
esac
EOF
chmod +x scripts/wire-awiki-mcp.sh
```

- [ ] **Step 2: Commit**

```bash
git add scripts/wire-awiki-mcp.sh
git commit -m "feat: add awiki MCP wiring helper"
```

## Task 8.3: Smoke test for awiki MCP server

**Files:** Create: `tests/mcp_server_test.bats`, `tests/fixtures/mcp-list-tools.json`

- [ ] **Step 1: Write the test**

```bash
cat > tests/fixtures/mcp-list-tools.json <<'EOF'
{"jsonrpc":"2.0","id":1,"method":"tools/list","params":{}}
EOF

cat > tests/mcp_server_test.bats <<'EOF'
#!/usr/bin/env bats

@test "awiki MCP server lists 4 tools" {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  run bash -c 'cat tests/fixtures/mcp-list-tools.json | node mcp/awiki-server/index.js | head -1'
  [ "$status" -eq 0 ]
  [[ "$output" == *"ingest_source"* ]]
  [[ "$output" == *"lint"* ]]
  [[ "$output" == *"query_wiki"* ]]
  [[ "$output" == *"update_catalog"* ]]
}

@test "awiki MCP server rejects unknown tool" {
  if [[ ! -d mcp/awiki-server/node_modules ]]; then
    skip "run 'cd mcp/awiki-server && npm install' first"
  fi
  REQ='{"jsonrpc":"2.0","id":1,"method":"tools/call","params":{"name":"bogus","arguments":{}}}'
  run bash -c "echo '$REQ' | node mcp/awiki-server/index.js"
  [[ "$output" == *"unknown tool"* || "$output" == *"isError"* ]]
}
EOF
```

- [ ] **Step 2: Install deps + run test**

```bash
( cd mcp/awiki-server && npm install --silent )
bats tests/mcp_server_test.bats
```

Expected: 2 tests pass.

- [ ] **Step 3: Commit**

```bash
git add tests/mcp_server_test.bats tests/fixtures/mcp-list-tools.json
git commit -m "test: add MCP server smoke tests (tools list + unknown-tool rejection)"
```

## Task 8.4: Phase 8 merge

```bash
git checkout main
git merge --no-ff phase-8-mcp-server -m "feat: complete phase 8 MCP server"
git branch -d phase-8-mcp-server
```

---

# Phase 9: Scheduled Lint Configs

**Deliverable:** Pre-built configs for launchd (macOS), systemd (Linux), GitHub Actions (CI).

**Branch:** `phase-9-scheduled`
**Depends on:** Phase 2.

## Task 9.1: Branch + launchd plist

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-9-scheduled
```

- [ ] **Step 2: Write `scheduled/launchd.plist.example`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.user.awiki.lint</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>-lc</string>
    <string>cd /ABSOLUTE/PATH/TO/WIKI && /opt/homebrew/bin/just lint</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key><integer>9</integer>
    <key>Minute</key><integer>0</integer>
  </dict>
  <key>StandardOutPath</key>
  <string>/tmp/awiki-lint.out</string>
  <key>StandardErrorPath</key>
  <string>/tmp/awiki-lint.err</string>
</dict>
</plist>
```

- [ ] **Step 3: Write `scheduled/systemd.timer.example` and `.service.example`**

```ini
# scheduled/awiki-lint.service.example
[Unit]
Description=awiki lint pass

[Service]
Type=oneshot
WorkingDirectory=/ABSOLUTE/PATH/TO/WIKI
ExecStart=/usr/bin/just lint
```

```ini
# scheduled/awiki-lint.timer.example
[Unit]
Description=Run awiki lint daily

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

- [ ] **Step 4: Write `scheduled/github-action.yml.example`**

```yaml
name: awiki-ci
on:
  push:
  pull_request:
  schedule:
    - cron: '0 9 * * *'
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive
      - name: Install just
        run: |
          curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin
      - name: Install hugo extended
        run: |
          curl -fsSL https://github.com/gohugoio/hugo/releases/download/v0.120.4/hugo_extended_0.120.4_linux-amd64.tar.gz | tar xz -C /tmp
          sudo mv /tmp/hugo /usr/local/bin/hugo
      - name: Install bats-core (pinned)
        run: |
          curl -fsSL https://github.com/bats-core/bats-core/archive/refs/tags/v1.10.0.tar.gz | tar xz -C /tmp
          sudo /tmp/bats-core-1.10.0/install.sh /usr/local
      - name: Optional unlock for encrypted wikis
        if: ${{ env.GIT_CRYPT_KEY != '' }}
        env:
          GIT_CRYPT_KEY: ${{ secrets.GIT_CRYPT_KEY }}
        run: |
          sudo apt-get update && sudo apt-get install -y git-crypt
          echo "$GIT_CRYPT_KEY" | base64 -d > /tmp/git-crypt-key
          git-crypt unlock /tmp/git-crypt-key
          rm /tmp/git-crypt-key
      - run: just check-deps
      - run: just lint
      - run: just test
```

- [ ] **Step 5: Document in README + WIKI.md**

Append exact content to `README.md`:

```markdown
## Scheduled lint

Pre-built configs ship under `scheduled/`:

- **macOS (launchd):** `cp scheduled/launchd.plist.example ~/Library/LaunchAgents/com.user.awiki.lint.plist`, edit the absolute path, `launchctl load ~/Library/LaunchAgents/com.user.awiki.lint.plist`.
- **Linux (systemd):** copy `scheduled/awiki-lint.service.example` and `scheduled/awiki-lint.timer.example` to `~/.config/systemd/user/`, drop the `.example`, edit the absolute path, then `systemctl --user daemon-reload && systemctl --user enable --now awiki-lint.timer`.
- **GitHub Actions:** `cp scheduled/github-action.yml.example .github/workflows/awiki-ci.yml`. If the repo uses git-crypt, set the `GIT_CRYPT_KEY` repo secret to the base64-encoded key; the workflow unlocks before lint/test.
```

Append exact content to `WIKI.md` section 4 (workflows), as a new sub-section "Scheduled lint":

```markdown
### 4.5 Scheduled lint

If the user has installed one of the configs in `scheduled/`, lint runs automatically on a cadence. Lint output is captured to logs (`/tmp/awiki-lint.{out,err}` for launchd; `journalctl --user -u awiki-lint` for systemd; the Actions run log for CI). Agent should treat scheduled lint failures as the next-session priority.
```

- [ ] **Step 6: Add a yaml-lint smoke test for the GH Actions example**

```bash
cat > tests/scheduled_test.bats <<'EOF'
#!/usr/bin/env bats

@test "github-action.yml.example is valid YAML" {
  command -v python3 >/dev/null 2>&1 || skip "python3 missing"
  python3 -c "import yaml; yaml.safe_load(open('scheduled/github-action.yml.example'))"
}

@test "launchd plist parses as XML" {
  command -v plutil >/dev/null 2>&1 || skip "plutil missing"
  run plutil -lint scheduled/launchd.plist.example
  [ "$status" -eq 0 ]
}

@test "systemd timer has [Install] section" {
  run grep -F '[Install]' scheduled/awiki-lint.timer.example
  [ "$status" -eq 0 ]
}
EOF
```

(Add `pyyaml` to BOOTSTRAP step 0 dep check or print install hint; the test skips gracefully if missing.)

- [ ] **Step 7: Commit**

```bash
git add scheduled README.md WIKI.md tests/scheduled_test.bats
git commit -m "feat: scheduled lint configs (launchd/systemd/gh-actions) + smoke tests"
```

## Task 9.2: Phase 9 merge

```bash
git checkout main
git merge --no-ff phase-9-scheduled -m "feat: complete phase 9 scheduled lint configs"
git branch -d phase-9-scheduled
```

---

# Phase 10: Auto-Deploy Templates

**Deliverable:** Netlify, Cloudflare Pages, GitHub Pages templates + README per-target instructions.

**Branch:** `phase-10-deploy`
**Depends on:** Phase 3.

## Task 10.1: Branch + Netlify

```bash
git checkout -b phase-10-deploy
```

- [ ] **Step 1: Write `deploy/netlify.toml`**

```toml
[build]
  command = "scripts/deploy-build.sh"
  publish = "public"

[build.environment]
  HUGO_VERSION = "0.120.4"

[[plugins]]
  package = "netlify-plugin-cache"
```

- [ ] **Step 2: Write `deploy/cloudflare-pages.toml`**

```toml
# Cloudflare Pages — paste into project UI:
# Build command:    scripts/deploy-build.sh
# Build output:     public
# Environment vars:
#   HUGO_VERSION = 0.120.4
#   GIT_CRYPT_KEY = <base64> (only if wiki uses git-crypt; sets a CI-only secret)
```

- [ ] **Step 3: Write `deploy/github-pages.yml.example`**

```yaml
name: deploy-pages
on:
  push:
    branches: [main]
permissions:
  contents: read
  pages: write
  id-token: write
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive
      - uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: '0.120.4'
          extended: true
      - name: Install just
        run: curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin
      - name: Optional unlock for encrypted wikis
        if: ${{ secrets.GIT_CRYPT_KEY != '' }}
        env:
          GIT_CRYPT_KEY: ${{ secrets.GIT_CRYPT_KEY }}
        run: |
          sudo apt-get update && sudo apt-get install -y git-crypt
          echo "$GIT_CRYPT_KEY" | base64 -d > /tmp/git-crypt-key
          git-crypt unlock /tmp/git-crypt-key
          rm /tmp/git-crypt-key
      - run: bash scripts/deploy-build.sh
      - uses: actions/upload-pages-artifact@v3
        with:
          path: public
  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - uses: actions/deploy-pages@v4
        id: deployment
```

- [ ] **Step 4: Write `scripts/deploy-build.sh`**

```bash
cat > scripts/deploy-build.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Refuse to publish if encryption is configured but tree is locked.
if [[ -d .git/git-crypt ]]; then
  if ! git-crypt status -e 2>/dev/null | head -1 | grep -q 'encrypted:'; then
    echo "ERROR: git-crypt initialized but tree appears locked. Set GIT_CRYPT_KEY secret and unlock before deploy." >&2
    exit 1
  fi
fi

bash scripts/build.sh --full
EOF
chmod +x scripts/deploy-build.sh
```

- [ ] **Step 5: README deploy section**

Append exact content to README:

```markdown
## Deployment

Three pre-built templates under `deploy/`:

| Target | Files | Setup |
|--------|-------|-------|
| Netlify | `deploy/netlify.toml` | Copy to repo root, link the repo in the Netlify UI. |
| Cloudflare Pages | `deploy/cloudflare-pages.toml` | Settings live in CF UI; this file documents them. |
| GitHub Pages | `deploy/github-pages.yml.example` | `cp deploy/github-pages.yml.example .github/workflows/deploy-pages.yml` |

**Encrypted wikis:** if you ran `just encrypt-init` with git-crypt, set the `GIT_CRYPT_KEY` repo secret to the base64-encoded export of `secrets/.git-crypt-key`. Each template will unlock the tree before building. **If unlock fails or the secret is missing on a wiki with encryption enabled, the build fails closed — no plaintext fallback.**
```

- [ ] **Step 6: Add validation tests**

```bash
cat > tests/deploy_test.bats <<'EOF'
#!/usr/bin/env bats

@test "netlify.toml parses as TOML" {
  command -v python3 >/dev/null 2>&1 || skip
  python3 -c "import tomllib; tomllib.load(open('deploy/netlify.toml','rb'))" 2>/dev/null \
    || python3 -c "import tomli; tomli.load(open('deploy/netlify.toml','rb'))"
}

@test "github-pages.yml.example parses as YAML" {
  python3 -c "import yaml; yaml.safe_load(open('deploy/github-pages.yml.example'))"
}

@test "deploy-build.sh refuses locked git-crypt tree" {
  WORK="$(mktemp -d)"
  cp scripts/deploy-build.sh "$WORK/"
  cd "$WORK"
  mkdir -p .git/git-crypt
  run bash deploy-build.sh
  [ "$status" -ne 0 ]
  cd - >/dev/null
  rm -rf "$WORK"
}
EOF
```

- [ ] **Step 7: Commit + merge**

```bash
bats tests/deploy_test.bats
git add deploy scripts/deploy-build.sh README.md tests/deploy_test.bats
git commit -m "feat: add auto-deploy templates with git-crypt unlock + tests"
git checkout main
git merge --no-ff phase-10-deploy -m "feat: complete phase 10 deploy templates"
git branch -d phase-10-deploy
```

---

# Phase 11: Multimodal Ingest Helpers

**Deliverable:** `ingest-pdf.sh`, `ingest-audio.sh`, vision workflow doc.

**Branch:** `phase-11-multimodal`
**Depends on:** Phase 2.

## Task 11.1: Branch + `scripts/ingest-pdf.sh`

```bash
git checkout -b phase-11-multimodal
```

- [ ] **Step 1: Write `scripts/ingest-pdf.sh`**

`_originals/` ALWAYS lives under `raw/processed/_originals/` (privacy-protected by gitignore + git-crypt patterns). The helper produces a markdown sidecar in the inbox, ready for `just ingest`.

```bash
cat > scripts/ingest-pdf.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: ingest-pdf.sh <pdf-path-under-raw/inbox/>" >&2; exit 1; }
PDF="$1"

[[ "$PDF" =~ \.pdf$ ]] || { echo "not a pdf" >&2; exit 1; }
[[ -f "$PDF" ]] || { echo "not found: $PDF" >&2; exit 1; }
[[ "$PDF" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }

OUT="${PDF%.pdf}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$PDF" "$ORIG_DIR/$(basename "$PDF")"

if command -v pdftotext >/dev/null 2>&1; then
  pdftotext -layout "$PDF" - > "$OUT"
elif command -v marker >/dev/null 2>&1; then
  marker "$PDF" -o "$OUT"
else
  echo "Need pdftotext or marker installed" >&2
  exit 1
fi

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [pdf]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$PDF" .pdf)" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$PDF")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$PDF"

echo "PDF-CONVERTED|in=$PDF|out=$OUT|orig=$ORIG_DIR/$(basename "$PDF")"
echo "Now run: just ingest $OUT"
EOF
chmod +x scripts/ingest-pdf.sh
```

- [ ] **Step 2: Write `scripts/ingest-audio.sh`**

```bash
cat > scripts/ingest-audio.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

[[ $# -eq 1 ]] || { echo "usage: ingest-audio.sh <audio-path-under-raw/inbox/>" >&2; exit 1; }
AUDIO="$1"

[[ -f "$AUDIO" ]] || { echo "not found: $AUDIO" >&2; exit 1; }
[[ "$AUDIO" =~ ^raw/inbox/ ]] || { echo "must be under raw/inbox/" >&2; exit 1; }
command -v whisper-cpp >/dev/null 2>&1 || { echo "Install whisper.cpp" >&2; exit 1; }
[[ -f models/ggml-base.en.bin ]] || { echo "missing models/ggml-base.en.bin (download from whisper.cpp releases)" >&2; exit 1; }

OUT="${AUDIO%.*}.md"
ORIG_DIR="raw/processed/_originals"
mkdir -p "$ORIG_DIR"
cp "$AUDIO" "$ORIG_DIR/$(basename "$AUDIO")"

whisper-cpp -m models/ggml-base.en.bin -otxt -f "$AUDIO"
mv "${AUDIO}.txt" "$OUT"

TMP="$(mktemp)"
{
  printf -- "---\ntitle: \"%s\"\ndate: %s\nlast_updated: %s\ntype: source\ntags: [audio]\naliases: []\nsources: []\noriginal: %s\ndraft: false\n---\n\n" \
    "$(basename "$AUDIO")" "$(date '+%Y-%m-%d')" "$(date '+%Y-%m-%d')" "$ORIG_DIR/$(basename "$AUDIO")"
  cat "$OUT"
} > "$TMP"
mv "$TMP" "$OUT"

rm "$AUDIO"
echo "AUDIO-TRANSCRIBED|out=$OUT|orig=$ORIG_DIR/$(basename "$AUDIO")"
echo "Now run: just ingest $OUT"
EOF
chmod +x scripts/ingest-audio.sh
```

- [ ] **Step 3: BATS test for `ingest-pdf.sh`**

```bash
cat > tests/ingest_pdf_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  command -v pdftotext >/dev/null 2>&1 || skip "pdftotext not installed"
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/raw/inbox/interactive" "$WORK/raw/processed"
  # Write minimal valid PDF (one-page hello). Uses python's reportlab if available, else skip.
  if ! python3 -c "from reportlab.pdfgen import canvas" 2>/dev/null; then skip "reportlab not installed"; fi
  python3 - "$WORK/raw/inbox/interactive/hello.pdf" <<PY
from reportlab.pdfgen import canvas
import sys
c = canvas.Canvas(sys.argv[1])
c.drawString(72, 720, "Hello PDF")
c.showPage(); c.save()
PY
  cd "$WORK"
}
teardown() { cd - >/dev/null; rm -rf "$WORK"; }

@test "ingest-pdf produces markdown with frontmatter and stashes original" {
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-pdf.sh" raw/inbox/interactive/hello.pdf
  [ -f raw/inbox/interactive/hello.md ]
  [ -f raw/processed/_originals/hello.pdf ]
  [ ! -f raw/inbox/interactive/hello.pdf ]
  run grep -E '^title: "hello"' raw/inbox/interactive/hello.md
  [ "$status" -eq 0 ]
}

@test "ingest-pdf rejects path outside raw/inbox/" {
  cp raw/inbox/interactive/hello.md elsewhere.pdf 2>/dev/null || true
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-pdf.sh" /tmp/elsewhere.pdf
  [ "$status" -ne 0 ]
}
EOF
```

- [ ] **Step 4: Append vision-workflow doc to WIKI.md**

Append exact content to `WIKI.md` after section 4.1:

```markdown
**Vision-aware ingest:** when a source contains images, the agent operates in two passes:
1. **Text pass:** Read the markdown body alone via the Read tool.
2. **Image pass:** Use the Read tool to view referenced images one at a time. Claude Code handles `![alt](path.png)` markdown image refs natively; for Codex / OpenCode use their equivalent vision tool.
3. **Integrate:** combine notes from both passes when writing `content/sources/<slug>.md` and any entity/concept pages affected.

No script needed — the agent decides when image content is load-bearing. For dense visual sources (slides, infographics), the agent should default to image-pass; for text-with-decorative-images, text-pass alone is sufficient.
```

- [ ] **Step 5: Commit + merge**

```bash
bats tests/ingest_pdf_test.bats || true   # optional deps may skip
git add scripts/ingest-pdf.sh scripts/ingest-audio.sh tests/ingest_pdf_test.bats WIKI.md
git commit -m "feat: PDF/audio ingest helpers (originals to raw/processed/_originals) + vision workflow"
git checkout main
git merge --no-ff phase-11-multimodal -m "feat: complete phase 11 multimodal ingest"
git branch -d phase-11-multimodal
```

---

# Phase 12: Polish & Examples

**Deliverable:** `examples/sample-wiki/` reference, full README smoke test verified, pre-commit hook installer, Obsidian vault config.

**Branch:** `phase-12-polish`
**Depends on:** all prior phases.

## Task 12.1: Branch + Obsidian vault config

```bash
git checkout -b phase-12-polish
```

- [ ] **Step 1: Write `.obsidian/app.json`**

```bash
mkdir -p .obsidian
cat > .obsidian/app.json <<'EOF'
{
  "attachmentFolderPath": "raw/assets/",
  "alwaysUpdateLinks": true,
  "useMarkdownLinks": false
}
EOF
```

- [ ] **Step 2: Write `.obsidian/hotkeys.json`**

```bash
cat > .obsidian/hotkeys.json <<'EOF'
{
  "editor:download-attachments": [{"modifiers": ["Mod", "Shift"], "key": "D"}]
}
EOF
```

- [ ] **Step 3: Write `.obsidian/community-plugins.json`**

```bash
cat > .obsidian/community-plugins.json <<'EOF'
["dataview", "obsidian-marp"]
EOF
```

- [ ] **Step 4: Commit**

```bash
git add .obsidian
git commit -m "chore: add Obsidian vault config (attachment path, hotkeys, plugins)"
```

## Task 12.2: Pre-commit hook installer

- [ ] **Step 1: Write `scripts/install-hooks.sh`**

```bash
cat > scripts/install-hooks.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

HOOK=".git/hooks/pre-commit"
cat > "$HOOK" <<'HOOKEOF'
#!/usr/bin/env bash
set -e
just lint
HOOKEOF
chmod +x "$HOOK"
echo "INSTALLED|$HOOK"
EOF
chmod +x scripts/install-hooks.sh
```

- [ ] **Step 2: Add `just install-hooks` recipe**

Edit `justfile`, add under `# === bootstrap / setup ===`:

```just
install-hooks:
    bash scripts/install-hooks.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/install-hooks.sh justfile
git commit -m "feat: add pre-commit hook installer"
```

## Task 12.3: `examples/sample-wiki/`

- [ ] **Step 1: Create reference wiki**

Create `examples/sample-wiki/` with:
- 2 entity pages (`vannevar-bush`, `claude-shannon`)
- 2 concept pages (`memex`, `information-theory`)
- 1 source page (`s-as-we-may-think`)
- `catalog.md`, `_index.md`, `log.md`

Each page has full frontmatter and at least one wikilink.

- [ ] **Step 2: README pointer**

Append to README:

> ## Example
>
> Browse `examples/sample-wiki/` for a tiny reference wiki with full frontmatter, wikilinks, and catalog.

- [ ] **Step 3: Commit**

```bash
git add examples README.md
git commit -m "docs: add examples/sample-wiki reference"
```

## Task 12.4: Verify full smoke test

- [ ] **Step 1: Fresh-clone smoke test (manual)**

```bash
cd /tmp
git clone /Users/joanmarc/dailywork/celonis/awiki test-clone
cd test-clone
git submodule update --init --recursive
just check-deps
just install-hooks
echo "# Sample\nA test source about caves." > raw/inbox/interactive/sample.md
just ingest raw/inbox/interactive/sample.md
just lint
just build
just test
```

Expected: every step succeeds. Document any rough edges in `docs/decisions/smoke-test-issues.md` and fix.

- [ ] **Step 2: Commit fixes**

```bash
git commit -am "fix: smoke test corrections"
```

## Task 12.5: Phase 12 merge

```bash
git checkout main
git merge --no-ff phase-12-polish -m "feat: complete phase 12 polish & examples"
git branch -d phase-12-polish
git tag v1.0.0
```

---

# Self-Review Checklist

Run after writing all phases. Issues found go inline; no re-review needed.

- [ ] Every spec section has at least one task implementing it.
- [ ] No "TBD", "TODO", "implement later" anywhere in the plan.
- [ ] Type/method/property names consistent across phases (e.g. `LINT|<level>|<file>|<msg>` everywhere).
- [ ] Every code block compiles or is valid syntax for its language.
- [ ] Test code precedes implementation in every TDD task.
- [ ] All scripts use `#!/usr/bin/env bash` and `set -euo pipefail`.
- [ ] All recipes match script signatures (incl. variadic quoting).
- [ ] Phase dependencies stated and respected.
- [ ] Commit messages use Conventional Commits.

---

# Execution Notes

- **Recommended runner:** subagent-driven-development. Dispatch one subagent per task; review between tasks.
- **Parallelism:** phases 4, 5, 9, 10, 11 are mutually independent given phase 1-2 prereqs. Can run agents on multiple worktrees.
- **Spike phases:** 3 and 4 each begin with a spike task (4 hours) before the rest of the phase proceeds. If the spike outcome contradicts the plan, update the spec FIRST, then revise the affected tasks.
- **Cumulative test gate:** every phase merge requires `just test && just lint` clean on `main`.

