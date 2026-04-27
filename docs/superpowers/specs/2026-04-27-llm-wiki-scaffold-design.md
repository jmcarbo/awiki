# LLM Wiki Scaffold — Design

**Date:** 2026-04-27
**Status:** Approved (pending user written-spec review)
**Type:** Template / scaffold repository

## Summary

A reusable template repository for building personal LLM-maintained knowledge bases ("wikis"). Users clone the repo per domain (personal, research, book, business, etc.), open any LLM agent (Claude Code, Codex, OpenCode), and the agent walks them through bootstrap, then operates the wiki via a documented schema and a thin layer of helper scripts.

The system is domain-agnostic, multi-agent compatible, Hugo-renderable, Obsidian-friendly, and has search via `qmd` (qntx-labs fork) bootstrapped from day one.

## Goals

- **Scaffold, not application.** Cloned per use.
- **Multi-agent.** Same schema works for any agent with a system-prompt convention (CLAUDE.md / AGENTS.md).
- **Mixed tooling.** Browse via Obsidian, render via Hugo, search via qmd, navigate via terminal.
- **Schema as primary contract.** Conventions documented in one file, scripts enforce mechanics, agent does semantic work.
- **Privacy-aware.** Encryption hooks (git-crypt / age) opt-in for sensitive domains.

## Non-goals

- Hosted SaaS or multi-tenant deployment.
- Full RAG pipeline with embedding store. (Index-first; qmd handles vector escalation.)
- MCP server for wiki ops. (Future work; not blocking.)
- Auto-deploy / CI hosting. (User wires their own.)

---

## Architecture

Three layers per the LLM Wiki pattern:

1. **Raw sources** (`raw/`) — immutable input documents. Agent reads, never modifies.
2. **Wiki** (`content/`) — LLM-generated markdown pages. Hugo serves, Obsidian browses.
3. **Schema** (`WIKI.md`, with stub `CLAUDE.md` and `AGENTS.md` pointing to it) — conventions and workflows the agent must follow.

Plus:
- **Scripts** (`scripts/`) — thin bash helpers for bookkeeping (file moves, log appends, lint, qmd index, encryption).
- **justfile** — recipe entry-point exposing common workflows.
- **Bootstrap** (`BOOTSTRAP.md`) — agent reads on first init; customizes schema per domain.

---

## Repository Layout

```
awiki/
├── BOOTSTRAP.md                # agent reads on first init
├── WIKI.md                     # schema (primary contract)
├── CLAUDE.md                   # stub pointing to WIKI.md (cross-platform safe)
├── AGENTS.md                   # stub pointing to WIKI.md
├── README.md                   # human-facing overview
├── .gitignore
├── .gitattributes              # git-crypt patterns (commented out)
├── hugo.toml                   # Hugo config
├── justfile                    # recipe entry-point
├── themes/                     # git submodule(s)
├── content/                    # Hugo-served wiki pages (LLM-owned)
│   ├── _index.md               # Hugo home
│   ├── catalog.md              # content catalog (LLM-maintained)
│   ├── log.md                  # chronological log
│   ├── entities/
│   ├── concepts/
│   ├── topics/
│   ├── sources/
│   └── synthesis/
├── raw/
│   ├── inbox/
│   │   ├── interactive/        # supervised one-at-a-time
│   │   ├── batch/              # unsupervised bulk
│   │   └── checkpoint/         # staged pipeline w/ approvals
│   ├── processed/              # post-ingest (mirrors source path)
│   └── assets/                 # images from clipped articles
├── scripts/
│   ├── ingest.sh
│   ├── lint.sh
│   ├── install-qmd.sh
│   ├── qmd-index.sh
│   ├── encrypt-init.sh
│   └── log-append.sh
├── tests/
│   ├── fixtures/
│   ├── lint_test.sh
│   ├── ingest_test.sh
│   ├── log_append_test.sh
│   └── schema_test.sh
└── docs/
    ├── just-help.txt           # extended justfile help
    └── superpowers/specs/      # design docs
```

Key constraints:
- `raw/` lives outside `content/` so Hugo never publishes raw sources.
- `raw/inbox/` is gitignored (sources may be sensitive / copyrighted).
- `raw/processed/` is tracked → version history of what was ingested.
- Wikilinks `[[page-slug]]` resolve via Hugo theme that supports them; Obsidian native.
- Slugs are unique across `content/`. Lint enforces.

---

## Schema (`WIKI.md`)

The schema is the primary agent contract. Sections:

### 1. Identity
Filled by BOOTSTRAP: `wiki_name`, `domain`, one-line purpose.

### 2. Page Conventions
- Filenames: `kebab-case.md`. Folder = type.
- Required frontmatter (YAML, single block serving both Hugo and Obsidian):

```yaml
---
title: "Vannevar Bush"
date: 2026-04-27
last_updated: 2026-04-27
type: entity                    # entity|concept|topic|source|synthesis|deck|chart|canvas|log|catalog
tags: [memex, computing-history]
aliases: [Bush, V. Bush]
sources: ["[[s-as-we-may-think]]"]
draft: false
---
```

- Body structure: lead paragraph (≤100 words) → sections → `## Related` (wikilinks) → `## Sources`.
- Frontmatter `sources: ["[[slug]]"]` stores wikilinks as YAML strings — they are NOT rendered as links by Obsidian or Hugo (frontmatter is data). Body `## Sources` section mirrors them as real links for human reading. Lint extracts both and validates parity.
- `type` enum splits into **page kinds** (`entity`, `concept`, `topic`, `source`, `synthesis`, `deck`, `chart`, `canvas`) and **system pages** (`log`, `catalog`). System pages have separate validation rules and are exempt from folder-per-type and orphan checks.

### 3. Wikilink Rules
- `[[page-slug]]` or `[[page-slug|display]]`.
- Lint flags broken links and slug collisions.

### 4. Workflows

**Ingest:**
1. Read source.
2. Propose summary + page-update plan.
3. Write `content/sources/<slug>.md`.
4. Update affected entity/concept/topic pages.
5. Update `content/catalog.md`.
6. Append log entry via `scripts/log-append.sh`.

**Query:**
1. Read `catalog.md`.
2. If insufficient (>100 sources or vector-needed query), shell to `qmd search`.
3. Drill into pages.
4. Answer with `[[…]]` citations.
5. If novel synthesis, file under `content/synthesis/` and log.

**Lint:**
1. `just lint` → captures structured output.
2. Agent groups issues by theme.
3. Proposes fixes per group; user approves.
4. `log-append.sh lint "<summary>"`.

### 5. Inbox Queues
Three modes distinguished by path under `raw/inbox/`:

- **interactive/** — single source, supervised. Default.
- **batch/** — folder dump, unattended; agent writes `synthesis/batch-<date>.md` summary at end.
- **checkpoint/** — staged proposals under `.staged/`, user approves before commit.

### 6. Output Formats
Markdown, comparison table, Marp slide deck, Matplotlib chart, Mermaid diagram, Obsidian canvas. All filed under `content/synthesis/` (the `synthesis/` folder is a **container** for derived/output artifacts, not a single page type). Frontmatter `type` reflects the artifact: `synthesis` (markdown analysis), `deck` (Marp), `chart` (image+caption page), `canvas` (Obsidian canvas json sidecar). The folder-per-type rule (Section 2) is relaxed for the synthesis container; lint allows mixed types under `content/synthesis/`.

### 7. Lint Checklist
Mechanical: broken wikilinks, missing frontmatter fields, stale `last_updated` (>90d), duplicate aliases, empty pages, dangling/missing catalog entries.
Semantic (agent-driven): contradictions, stale claims vs newer sources, missing concept pages.

### 8. qmd Usage
- Heuristic: shell to `qmd search` when wiki >~100 sources OR query needs vector match.
- Fallback to `catalog.md` + `grep -r` if qmd unavailable.

---

## Bootstrap (`BOOTSTRAP.md`)

Agent reads on first session. Steps:

0. **Dependency check.** Run a check script: bash 4+, git, just, hugo. Optional: git-crypt, age, qmd toolchain. Print OS-specific install hints for missing required tools; halt if any required tool absent.
1. Ask domain (personal / research / book / business / other).
2. Ask wiki name, one-line purpose.
3. Ask privacy level → if sensitive, run `just encrypt-init` BEFORE any first commit. Verify `git status` shows expected encrypted-vs-cleartext patterns.
4. Ask: "Track ingested sources in git? (y/N)" — on `y`, remove `raw/processed/` from `.gitignore`. On `N`, sources stay local.
5. Ask Hugo theme preference (default: `hugo-book`).
6. Ask: "Publish log to rendered site? (y/N)" — controls `draft:` flag on `content/log.md`.
7. Patch `WIKI.md` Identity section, `hugo.toml` site title, `content/log.md` frontmatter.
8. Run `just install-qmd` then `just reindex`. Treat install failure as non-fatal; record status in `.awiki/qmd-status`.
9. Optionally wire qmd MCP server into agent harness.
10. Append init entry to `log.md`.
11. Stage initial commit, prompt user to review.

**Agent schema files** — `CLAUDE.md` and `AGENTS.md` are NOT symlinks (Windows / corporate git configs lose them). They are committed stub files containing a single line: `> Schema is in [WIKI.md](./WIKI.md). Read it before responding.` plus a one-line summary of WIKI.md sections. `lint.sh` checks the stubs are present and that WIKI.md exists; `WIKI.md` itself is the canonical source.

---

## Scripts

All scripts are **bash 4+** (`#!/usr/bin/env bash`, `set -euo pipefail`). macOS ships bash 3.2; BOOTSTRAP's dependency check requires `bash --version` ≥ 4 and prints `brew install bash` on macOS if missing. Scripts are idempotent where possible, exit nonzero on error, log to stderr.

### `scripts/ingest.sh <source-path>`
- Accepts: absolute path, or path relative to repo root, under `raw/inbox/`. Bare slug rejected.
- Resolves queue mode from path (interactive / batch / checkpoint).
- Moves source `raw/inbox/...` → `raw/processed/<original-relative-path>`.
- Appends log entry via `log-append.sh`.
- Increments `.awiki/ingest-count`.
- If `.awiki/ingest-count` ≥ threshold (`AWIKI_LINT_AFTER_N` env var, default 5 from `.awiki/config`), runs `scripts/lint.sh` and resets counter. Lint failure is logged but does NOT block the ingest move (sources are already in `raw/processed/` by this point); exit code 4 surfaces the warning to caller.
- Calls `qmd-index.sh` at end if qmd installed (incremental).
- Does NOT call LLM — agent does the semantic work in subsequent turns.
- Exit codes: 0 ok, 1 missing/invalid arg, 2 path not in inbox, 3 already processed, 4 ok-but-lint-failed, 5 qmd reindex failed (non-fatal warning).

### `scripts/lint.sh [--fix] [--hugo-check]`
- Emits `LINT|<level>|<file>|<msg>` lines.
- Final `LINT-SUMMARY|errors=N|warnings=M|info=K`.
- `--fix`: mechanical fixes only (add missing `last_updated`, normalize wikilink case, frontmatter key order, trailing whitespace).
- `--hugo-check`: runs `hugo --renderToMemory` to catch template breakage.
- Exit: 0 clean, 1 warnings, 2 errors.

### `scripts/install-qmd.sh`
- Idempotent. Skips if `qmd` on PATH.
- Clones `qntx-labs/qmd` to `~/.local/share/qmd-src`, builds per upstream README, symlinks binary to `~/.local/bin/qmd`.
- Verifies `qmd --version`.

### `scripts/qmd-index.sh`
- Runs `qmd index content/`.
- Cache at `.qmd/` (gitignored).

### `scripts/encrypt-init.sh [--age]`
- Default: git-crypt. Initializes, writes `.gitattributes` patterns for `raw/processed/private/**`, `content/private/**`, `secrets/**`.
- `--age`: age-based, asymmetric. Generates keypair under `secrets/`.
- Idempotent.

### `scripts/log-append.sh <action> <message...>`
- Appends `## [<ISO-datetime>] <action> | <message>` to `content/log.md`.

### Optional hooks
- `git/hooks/pre-commit` → runs `lint.sh`. Installed by BOOTSTRAP if user opts in.

---

## justfile

Recipe entry-point. `just` lists recipes.

```just
default:
    @just --list

# === ingest ===
ingest path:
    bash scripts/ingest.sh {{path}}

ingest-batch-list:                   # list files in batch inbox for agent to iterate
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

# === hugo ===
serve:
    hugo server -D

build:
    hugo --minify

# === bootstrap ===
init:
    @echo "Open agent. Say: 'init wiki'. Agent reads BOOTSTRAP.md."

install-qmd:
    bash scripts/install-qmd.sh

encrypt-init:
    bash scripts/encrypt-init.sh

# === git ===
status:
    git status -s

commit message:
    git add -A && git commit -m "{{message}}"

# === help ===
help:
    @cat docs/just-help.txt
```

`docs/just-help.txt` documents each recipe in detail, queue semantics, common workflows.

**Argument quoting convention:** recipes that accept multi-word args (`commit message:`, `search query:`, `log action *message:`) require the caller to quote: `just commit "fix: foo"`, `just search "memex history"`. `docs/just-help.txt` shows this in every example. `*message` (variadic) collects remaining args; `log-append.sh` joins with spaces.

---

## Hugo + Obsidian Integration

Single `content/` directory works in both.

### Frontmatter
YAML, serves both. See Page Conventions above.

### Wikilinks

Hugo's bundled goldmark does NOT ship a wikilink extension. Authoring uses `[[page-slug]]` for Obsidian compatibility; Hugo rendering requires explicit handling.

**Strategy (chosen approach — pending spike confirmation):**
1. Author wikilinks `[[slug]]` and `[[slug|display]]` natively for Obsidian.
2. `scripts/build.sh` (or `just build`) preprocesses `content/` into `.awiki/build-content/` rewriting `[[slug]]` → `[Display](/<section>/<slug>/)` using a slug→path map built by `lint.sh`. Hugo runs against the preprocessed copy.
3. Live preview (`just serve`) wraps `hugo server` with a watcher that rewrites on change, OR uses a `_markup/render-link.html` render-hook plus a custom shortcode `{{< wl "slug" >}}` for the small set of cases where preprocessing is impractical.

**Spike required before phase 3 implementation:** confirm preprocessing approach against `hugo-book` theme; alt path = pin a community module like `hugo-wikilinks`. Decision recorded in `docs/decisions/wikilink-rendering.md` after spike.

**Day-one fallback:** if Hugo can't render wikilinks yet, pages still render with `[[slug]]` as inline literal text — Obsidian remains primary browser. Hugo rendering is non-blocking for v1.

Theme default: `hugo-book` (configurable in BOOTSTRAP).

### Obsidian vault config (committed)
- `.obsidian/app.json` — `attachmentFolderPath: raw/assets/`.
- `.obsidian/hotkeys.json` — `Ctrl+Shift+D` → "Download attachments for current file".
- `.obsidian/community-plugins.json` — recommend Dataview, Marp.
- `.obsidian/workspace*` and `.obsidian/cache` gitignored.

### Hugo config (`hugo.toml`)
- `baseURL`, `title` patched by BOOTSTRAP.
- `contentDir = "content"`.
- Goldmark wikilink extension enabled.
- `disableKinds = ["taxonomy"]` toggle in BOOTSTRAP.

### Special pages
- `content/_index.md` — Hugo home.
- `content/catalog.md` — LLM-maintained content catalog (replaces ambiguous `index.md`).
- `content/log.md` — chronological log.

---

## qmd Integration

Source: https://github.com/qntx-labs/qmd

### Install (one-time, non-blocking)
`just install-qmd` → `scripts/install-qmd.sh`. BOOTSTRAP runs it but treats failure as non-fatal:
- On success: writes `.awiki/qmd-status=ok`, proceeds to reindex.
- On failure: writes `.awiki/qmd-status=missing`, prints warning, continues bootstrap. Agent uses `grep -r` fallback per WIKI.md until user fixes.
- BOOTSTRAP detects toolchain. qmd build prerequisites (Go / Rust / etc.) and the build command itself are determined by reading the upstream README at install time — script must surface upstream errors verbatim, not pretend success.
- macOS PATH check: if `~/.local/bin` not on PATH, BOOTSTRAP prints a one-liner to add it to user's shell rc.

### Index
- `.qmd/` at repo root, gitignored.
- Indexes `content/**` only.
- `just reindex` for full refresh.
- `ingest.sh` calls reindex at end (incremental if supported).

### MCP option
- qmd ships an MCP server. BOOTSTRAP asks if user wants it wired into agent harness; if yes, patches `.mcp.json` (Claude Code) or equivalent for other agents.

### Fallback
- If `qmd` unavailable, agent falls back to `catalog.md` + `grep -r`. Documented in `WIKI.md`.

---

## Privacy & Encryption

Default `.gitignore`:
```
# build artifacts
public/
.hugo_build.lock
.qmd/
.awiki/

# inbox: source files often sensitive / copyrighted
raw/inbox/

# default-private until user opts in (BOOTSTRAP / encrypt-init flips these)
raw/processed/
content/private/**
raw/processed/private/**

# secrets — deny all, allow only public keys
secrets/**
!secrets/*.pub

# obsidian local state
.obsidian/workspace*
.obsidian/cache

# OS / editor
.DS_Store
*.swp
```

**Important defaults:**
- `raw/processed/` is **gitignored by default** to prevent accidental commit of copyrighted/sensitive sources. BOOTSTRAP step 3 asks: "Track ingested sources in git? (y/N)". On `y`, BOOTSTRAP removes the `raw/processed/` line from `.gitignore`. Note implications (copyright, history-rewrite cost if revoked).
- `content/private/**` and `raw/processed/private/**` are **gitignored until `encrypt-init` runs**. `encrypt-init` removes those lines and adds matching git-crypt patterns to `.gitattributes` atomically.
- `secrets/**` denies all by default with explicit `!secrets/*.pub` allowlist for committable public keys (e.g. age recipient).

`raw/processed/` is tracked. If sensitive, encrypt.

### Encryption choices (opt-in via `just encrypt-init`)

- **None** — public wiki.
- **git-crypt** (recommended for personal/health/journal) — symmetric key, transparent en/decrypt at git level. Patterns cover `raw/processed/private/**`, `content/private/**`, `secrets/**`. Key exported to `secrets/.git-crypt-key` (gitignored).
- **age** (`--age`) — asymmetric, file-level, manual. Keypair under `secrets/`. Public key committed.

Lint warns if a page has `tags: [private]` or filename suggests private but path is not under `private/`.

---

## Lint Workflow

### Triggers (all selectable, all wired)
1. **Manual** — `just lint`.
2. **Auto after N ingests** — `ingest.sh` increments `.awiki/ingest-count`; runs lint at threshold (BOOTSTRAP default: 5). Counter resets.
3. **Git pre-commit hook** — installed by BOOTSTRAP if user opts in.
4. **Scheduled** — `WIKI.md` documents wiring via `loop`/`schedule` skills or external cron.

### Output
Structured `LINT|<level>|<file>|<msg>` lines, final `LINT-SUMMARY`.

### `--fix` (mechanical only)
Add missing `last_updated`, normalize wikilink case, frontmatter key order, trailing whitespace.

### Semantic (agent-driven)
Contradictions, stale claims, missing concept pages — handled in lint review workflow (agent reads lint output + relevant pages, proposes fixes per group).

---

## Log + Catalog Conventions

### `content/log.md`
- Append-only, chronological.
- `## [YYYY-MM-DD HH:MM] <action> | <message>`.
- Actions logged by default: `init`, `ingest`, `ingest-failed`, `synthesis`, `lint`, `encrypt`, `manual`.
- **`query` action is NOT logged by default** (high volume, metadata leak — when did user ask what?). Opt-in via `AWIKI_LOG_QUERIES=1` in `.awiki/config`.
- Body (optional): bullets with affected pages.
- Parseable: `grep "^## \[" content/log.md | tail -10`.
- **Privacy:** `log.md` ships with `draft: true` frontmatter so Hugo skips it on `hugo build` by default. BOOTSTRAP asks "Publish log to rendered site? (y/N)"; on `n`, draft stays true. Even when published, log entries are summary-level (no source contents).

### `content/catalog.md`
- LLM-maintained content catalog.
- Sections by type, alphabetical within.
- One line per page: `[[slug]] — one-line summary. <metadata>`.
- Updated on every ingest.
- Lint warns on dangling / missing entries.

### `content/_index.md`
- Hugo home page. Wiki landing.
- Brief overview, links to top-level sections, recent log entries (rendered via Hugo shortcode at build time, not embedded).
- **Maintenance:** written once by BOOTSTRAP. Agent does NOT auto-update on ingest. Manual edit only.

### Section indexes (`content/entities/_index.md`, etc.)
- **Deferred to v2.** Not part of v1 scaffold. Hugo lists pages in a section via theme defaults; explicit section indexes optional.
- If a user creates one manually, lint validates it like any other page (frontmatter, no broken wikilinks). No special workflow.

---

## Testing

### Script tests (`tests/`)
- `lint_test.sh` — fixture wiki under `tests/fixtures/wiki-broken/` with known issues.
- `ingest_test.sh` — drop fixture source into mock inbox, assert moved + logged.
- `log_append_test.sh` — assert format, idempotence.
- `schema_test.sh` — parse all `content/**/*.md` frontmatter, validate per-`type` required fields.

Framework: plain bash + `assert.sh` helper. BOOTSTRAP detects `bats-core` if installed and uses it.

### CI
Out of scope for scaffold. `WIKI.md` documents a sample GitHub Actions workflow.

### Manual smoke test (in README)
1. Drop sample source into `raw/inbox/interactive/sample.md`.
2. `just ingest raw/inbox/interactive/sample.md` → confirm move + log.
3. `just lint` → confirm clean (or expected warnings).
4. `just serve` → confirm Hugo renders.
5. `just search "test"` → confirm qmd responds.

### Hugo build check
`scripts/lint.sh --hugo-check` runs `hugo --renderToMemory` to catch template breakage.

---

## Threat Model / Privacy Considerations

Consolidated risks and mitigations (cross-references the more detailed sections):

| Risk | Mitigation in v1 |
|------|------------------|
| User commits sensitive content before `encrypt-init` runs | `content/private/**` and `raw/processed/private/**` gitignored by default; `encrypt-init` flips them atomically with git-crypt patterns. |
| `raw/processed/` leaks copyrighted/PII sources | Gitignored by default; opt-in via BOOTSTRAP question. |
| Secrets accidentally committed | `secrets/**` gitignored with `!secrets/*.pub` allowlist. |
| Hugo publishes private content | `content/private/**` gitignored pre-encrypt; render-time check via `lint.sh --hugo-check` warns on `tags: [private]` outside private paths. |
| `log.md` leaks query / activity metadata | `query` not logged by default; `log.md` has `draft: true` so Hugo skips. |
| Agent moves files into `private/` without consent | WIKI.md mandates user confirmation before any move into `private/`. |
| Lint / build runs on encrypted (locked) tree | Documented: lint and Hugo build assume decrypted working tree; CI must unlock or skip. |

## Dependencies & Compatibility

| Tool | Required? | Min version | Notes |
|------|-----------|-------------|-------|
| `bash` | yes | 4+ | macOS ships 3.2; install via Homebrew. |
| `git` | yes | 2.30+ | |
| `just` | yes | 1.13+ | brew/cargo/apt. |
| `hugo` | yes | 0.120+ | extended edition for SCSS-using themes. |
| `qmd` (qntx-labs fork) | optional | n/a | non-blocking; install failure logs warning. |
| `git-crypt` | optional | 0.7+ | required only if user picks git-crypt encryption. |
| `age` | optional | 1.0+ | required only if user picks age encryption. |
| `bats-core` | optional | 1.10+ | recommended for tests; fallback to plain bash + `assert.sh`. |

OS support: macOS, Linux primary. Windows via WSL2 (native git symlinks unreliable; spec uses stub files instead).

BOOTSTRAP step 0 (added): runs a dependency check, prints OS-specific install hints for missing required tools, halts on missing required deps.

## Spike Requirements (before implementation phase 3+)

1. **Hugo wikilink rendering** — confirm preprocessing approach against `hugo-book`. Output: `docs/decisions/wikilink-rendering.md` with chosen mechanism, version pins, and fallback. Blocker for phase 3 (Hugo render).
2. **qmd build** — clone `qntx-labs/qmd`, follow upstream README on macOS + Linux, verify `qmd index` and `qmd search` work. Output: pinned toolchain in `install-qmd.sh`. Blocker for phase 4 (qmd integration).

## Implementation Phases

Spec is sliced into independent plans. Phase 1 is safe to start immediately; later phases depend on the spike outcomes above.

1. **Skeleton** — repo layout, WIKI.md, BOOTSTRAP.md, stub CLAUDE.md/AGENTS.md, README.md, `.gitignore`, `.gitattributes`, justfile (recipe stubs only), empty content + raw dirs with `.gitkeep`.
2. **Scripts core** — `log-append.sh`, `ingest.sh`, `lint.sh` (mechanical checks only, no `--hugo-check`), tests for each. `.awiki/config`, `.awiki/ingest-count`.
3. **Hugo render** — pin theme, resolve wikilink approach (post-spike), `hugo.toml`, build/serve recipes. *Blocked on spike 1.*
4. **qmd integration** — `install-qmd.sh`, `qmd-index.sh`, fallback to grep, MCP option. *Blocked on spike 2.*
5. **Encryption** — `encrypt-init.sh` (git-crypt + age paths), `.gitattributes` patterns, atomic `.gitignore` flip.
6. **Tests & lint polish** — `tests/` framework finalized, `lint.sh --fix`, `lint.sh --hugo-check` (post phase 3), pre-commit hook installer.
7. **Polish** — Obsidian vault config commit, `docs/just-help.txt`, README smoke test, scheduled-lint docs.

## Open Items / Future Work

- MCP server for native wiki ops (escalate when index + qmd insufficient).
- Pre-built scheduled-lint config for common cron environments.
- Auto-deploy templates (Netlify, Cloudflare Pages, GitHub Pages).
- Vision-aware ingest for image-heavy sources (workflow documented; tooling deferred).
- PDF/audio ingest helpers (currently agent uses ad-hoc tools; `scripts/ingest-pdf.sh` candidate).
- Section indexes (deferred from v1).
- Slug rename / page deletion workflow (currently manual; auto-fix candidate).
- Alias resolution for wikilinks (`[[Bush]]` → `vannevar-bush.md`); requires lint-built alias map.
