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

### 4.6 Synthesis

1. Pick a plugin from `synthesis-plugins/`. Available: `briefing`, `mindmap`,
   `timeline`, `study-guide`. `just synth-list` prints the catalog with
   `<name> | <output_subtype> | <description>`.
2. `just synth <plugin> <topic-slug> --tag=<tag>` (or `--slugs=a,b,c` or
   `--query="..."`) scaffolds `content/synthesis/<topic>-<plugin>.md` and
   emits the prompt bundle on stdout. Quote the topic slug if it contains
   underscores; the orchestrator validates `^[a-z0-9][a-z0-9-]*$`. Plugin
   name constraint: `^[a-z][a-z0-9-]*$`. The orchestrator fails closed if
   any source is tagged `private` and the target page is not under
   `content/private/` (pass `--allow-private` to acknowledge intentional
   declassification).
3. The agent (you) reads the bundle, generates content **only between the
   BEGIN GENERATED and END GENERATED markers**, then calls
   `just synth-finalize <slug>`. Finalize runs `lint.sh --only=synth
   --file=<path>` and stamps `last_generated`.
4. Lint discipline (synth-namespaced rules):
   - **S1 marker integrity** — exactly one BEGIN, one END, BEGIN before
     END. Hard error.
   - **S2 required sections** — every section the plugin manifest's
     `required_sections` lists must appear inside the markers. Hard error.
   - **S3 evidence quote substring** — every `> "quote" — [[slug]]` line
     must appear verbatim in the cited source's body (after Unicode NFC
     normalization, zero-width strip, hyphen-variant collapse, smart→
     straight quotes, NBSP→space, whitespace collapse, and wikilink-to-
     title rewrite for inlined `[[other-slug]]` references). Hard error.
     If you draft a quote that fails S3, do NOT paraphrase to make it pass
     — go back to the source and copy the exact span.
   - **S4 citation in scope** — every `[[slug]]` inside markers must
     resolve and be in the page's resolved scope. Hard error.
   - **S5 scope drift** — warning if the recomputed `scope_hash` differs
     from the BEGIN-marker value. Skipped for `query`-scoped pages
     (qmd is non-deterministic). Resolution: `just synth-regen <slug>`.
   - **S6 hand-edit inside markers** — warning if frontmatter
     `last_updated` advanced past `last_generated` AND the working-tree
     diff vs HEAD intersects the BEGIN..END region. Diffs to lead
     paragraph, `## Notes`, `## Feedback`, or non-`last_generated`/
     `sources` frontmatter do NOT trigger. If you intentionally edited
     inside markers, run `synth-regen` to re-stamp.
   - **S9 aggregate evidence words** — sum of words across all
     `> "..."` lines is hard-capped by the plugin manifest's
     `max_evidence_total_words` (default 500; `study-guide` is 300 to
     defend against accidental near-reproduction of a single source).
   - `just lint --only=synth` runs only synth rules. `just lint
     --only=synth --fix` applies S3 normalization (zero-width strip,
     hyphen→ASCII, smart→straight quotes) inside generated regions only.
     Marker repair, missing-section repair, S9 reductions are NOT in
     `--fix` (they need regen).
5. Plugin-specific notes:
   - **`briefing`** — TL;DR / Key Findings / Open Questions / Evidence.
     Bias toward `type: source` over derivative pages to reduce echo.
   - **`mindmap`** — `## Mindmap` is a fenced ```mermaid``` block with
     the `mindmap` directive. Mermaid mindmap leaves cannot be markdown
     links, so the `## Pages` section bridges display text → wikilinks.
     Max depth 3, max ~40 nodes. The `synth-mindmap-validate.sh`
     post-hook runs `mmdc --dry-run` if installed (regex fallback
     otherwise). Post-hook is gated by `ALLOW_PLUGIN_POST_HOOKS=1` in
     `.awiki/config`.
   - **`timeline`** — `render: mermaid|table` config. Mermaid mode
     groups by decade. **Source dates from page bodies**, not just
     frontmatter `date:` (which is page-creation date). Mark uncertain
     dates `c.<year>` or `<year>?`.
   - **`study-guide`** — Concept Checklist / Short-Answer Questions
     (`<details>` blocks) / Flashcards (Anki-importable Q:/A: separated
     by `---`) / Suggested Deep-Dives / Evidence. `min_sources: 1` so a
     single textbook chapter is a valid scope; the tighter
     `max_evidence_total_words: 300` cap protects against
     near-reproduction.
6. To refresh later: `just synth-regen <slug>`. The orchestrator re-runs
   the privacy check, refuses if it detects a manual edit inside the
   marker region (pass `--force` to override or `--stage` to write to
   `.staged/<slug>.md` for review). After review, promote with
   `just synth-accept-stage <slug>`.
7. Plugin post-hooks (third-party `synthesis-plugins/<name>/post.sh`)
   are arbitrary shell code. Default is `ALLOW_PLUGIN_POST_HOOKS=0` —
   the orchestrator refuses to invoke any post-hook until you flip the
   flag. Review the hook source first; treat unfamiliar plugin
   directories as untrusted.

#### Feedback channel

Synthesis pages carry a `## Feedback` section outside the BEGIN/END
markers. Three refinement channels:

- **Channel A — `## Feedback` bullets (curated, persistent).** User edits
  this section directly in markdown. On the next regen, the orchestrator
  parses each bullet and injects it into the prompt as a fenced literal
  block:

  ````
  ```text
  Tighten TL;DR to 15 words per bullet.
  Add coverage of [[c-associative-trails]] — under-cited so far.
  ```
  ````

  The fence guarantees that bullet content (including any string that
  mimics a marker comment) renders as inert text — it cannot be confused
  with the live page's BEGIN/END markers.

- **Channel B — `synth.sh refine` (CLI append).**
  `just synth-refine <slug> "<note>"` appends a bullet to `## Feedback`,
  idempotent on exact duplicates. Multiple refines batch into one regen;
  `just synth-regen <slug>` triggers the next pass.

- **Channel C — `## Notes` (free-form).** Free user notes outside markers.
  The agent reads `## Notes` as additional context (knowledge), NOT as
  instructions. Use Notes for content; use Feedback for binding directives
  that shape style/scope.

#### MCP tools

Three synthesis tools surface via the awiki MCP server (alongside the
four existing ingest/lint/query/update tools):

- `list_synth_plugins()` — returns `{plugins, errors}` with one record
  per plugin under `synthesis-plugins/`. No arguments. Use to discover
  available plugins before calling `synthesize`.
- `synthesize(plugin, scope_descriptor, topic_slug)` — wraps
  `synth.sh new`. Returns `{prompt_bundle, resolved_slugs, target_path}`
  on success. On scope-resolution failure, returns
  `{error: "scope_resolution_failed", reason, suggested_action}` (NOT an
  MCP-level error). The agent receives the prompt, fills the body
  between BEGIN/END markers in `target_path`, then calls
  `finalize_synthesis`.
- `finalize_synthesis(topic_slug)` — wraps `synth.sh finalize`.
  Validates markers, runs synth lint, stamps `last_generated`,
  populates frontmatter `sources:`. On marker/lint failure, returns a
  structured error payload; agent corrects and retries.

All three tools enforce strict argument validation:

- `plugin` matches `^[a-z][a-z0-9-]*$`.
- `topic_slug` matches `^[a-z0-9][a-z0-9-]*$` (no leading hyphen —
  defeats flag-injection).
- `scope_descriptor` validates against
  `mcp/awiki-server/schemas/scope.json` (oneOf tag/slugs/query, with
  optional exclude_tags/min_last_updated/types filters).
- The resolved manifest path is `realpath`-checked against
  `synthesis-plugins/` to defeat symlink-swap attacks.

All justfile recipes use `--` to terminate flag parsing before positional
args.

## 5. Inbox Queues

- `raw/inbox/interactive/` — single source, supervised.
- `raw/inbox/batch/` — folder dump, unattended; agent writes `synthesis/batch-<date>.md` summary at end.
- `raw/inbox/checkpoint/` — staged proposals under `.staged/`, user approves before commit.

## 6. Output Formats

Markdown, comparison table, Marp slide deck (`type: deck`), Matplotlib chart (`type: chart`), Mermaid diagram (inline in markdown), Obsidian canvas (`type: canvas`). All filed under `content/synthesis/`.

- Synthesis pages may carry `plugin: <name>` frontmatter pointing to a
  manifest under `synthesis-plugins/`. Phase 13 ships `briefing`; phase 14
  adds `mindmap`, `timeline`, `study-guide`. Required sections per plugin
  are enforced by lint S2.

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
- S1 (error): synthesis page has exactly one BEGIN/END marker pair, BEGIN before END.
- S2 (error): synthesis page contains every heading from its plugin's required_sections list inside the markers.
- S3 (error): every `> "quote" — [[slug]]` evidence line is a verbatim substring of the cited source body, after Unicode normalization, zero-width strip, hyphen-variant collapse, smart→straight quote conversion, NBSP→space, whitespace collapse, and wikilink-to-title rewrite. Fuzzy suggestion provided on miss.
- S4 (error): every `[[slug]]` inside the generated region is in the page's resolved scope.
- S5 (warning): recomputed scope_hash differs from BEGIN-marker value. Skipped for query-scoped pages.
- S6 (warning): last_updated > last_generated AND working-tree diff vs HEAD intersects the generated region. Frontmatter-driven (not mtime).
- S7 (info): `feedback_count=N` reported in synth-lint summary. Warning when >20 (suggests scope refactor or page split).
- S8 (warning): `## Feedback` bullet contains `[[slug]]` not in the page's resolved scope. Either widen the scope or remove the bullet.
- S9 (error): sum of evidence-quote words ≤ plugin manifest's max_evidence_total_words (default 500; study-guide 300).

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
