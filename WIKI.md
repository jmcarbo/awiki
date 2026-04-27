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

**Vision-aware ingest:** when a source contains images, the agent operates in two passes:
1. **Text pass:** Read the markdown body alone via the Read tool.
2. **Image pass:** Use the Read tool to view referenced images one at a time. Claude Code handles `![alt](path.png)` markdown image refs natively; for Codex / OpenCode use their equivalent vision tool.
3. **Integrate:** combine notes from both passes when writing `content/sources/<slug>.md` and any entity/concept pages affected.

No script needed — the agent decides when image content is load-bearing. For dense visual sources (slides, infographics), the agent should default to image-pass; for text-with-decorative-images, text-pass alone is sufficient.

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

### 4.5 Scheduled lint

If the user has installed one of the configs in `scheduled/`, lint runs automatically on a cadence. Lint output is captured to logs (`/tmp/awiki-lint.{out,err}` for launchd; `journalctl --user -u awiki-lint` for systemd; the Actions run log for CI). Agent should treat scheduled lint failures as the next-session priority.

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
