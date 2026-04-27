# Synthesis Generator Framework — Design

**Date:** 2026-04-27
**Status:** Draft (pending user written-spec review)
**Type:** Feature on top of awiki v1 (post-master-plan addition)
**Depends on:** Master plan phases 1, 2, 3, 6, 8

## Summary

Pluggable synthesis layer for awiki: turn curated wiki page-sets into derived
artifacts (briefing, mindmap, timeline, study-guide) the way Google NotebookLM
turns a notebook of sources into "Briefing Doc / Mind Map / Study Guide /
Timeline" outputs — but vendor-neutral, agent-driven, citation-disciplined,
and persistent-as-markdown.

Plugins are filesystem-discovered single-file (or directory) descriptors under
`synthesis-plugins/`. Each plugin defines a prompt template and an output
schema. The user (or agent) invokes a plugin against a scope (tag, slugs, or
saved query); the framework scaffolds a synthesis page with managed-region
markers and emits the prompt. The agent fills the region. Lint then enforces
citation hygiene, evidence-quote substring matches, and marker integrity.

A `## Feedback` channel inside each synthesis page lets the user add binding
refinement directives that flow into the next regeneration prompt.

Ships 4 plugins in v1: `briefing`, `mindmap`, `timeline`, `study-guide`. All
text-output, agent-driven (no API keys, no TTS). Audio/video deferred.

## Goals

- Bring NotebookLM's notebook-as-derivative-engine pattern to awiki without
  coupling to any vendor.
- Synthesis pages are first-class wiki citizens: persistent, regenerable,
  catalog-listed, lintable, wikilinkable.
- Anti-hallucination spine: every claim cites `[[slug]]`, every page emits an
  `## Evidence` section with verbatim quoted spans, lint substring-checks
  quotes against source bodies.
- Iterative human refinement: `## Feedback` section + `synth refine` CLI;
  curated by user, binding on the next regen.
- Extensibility: `synthesis-plugins/` is a discovery directory. User-authored
  plugins work day one (no registration step).
- Reuses existing scripts/lint/MCP/catalog/log infrastructure. No parallel
  pipelines.

## Non-goals (v1)

- Audio Overview / podcast generation. Deferred — needs TTS API key model and
  separate spec.
- Video Overview / narrated decks. Deferred for the same reason.
- Vendor-coupled implementations (no NotebookLM API, no proprietary
  embeddings).
- Notebook sharing / multi-user notebooks.
- Real-time chat-with-sources. The existing agent Query workflow in `WIKI.md`
  already covers this surface.
- Section-anchor wikilinks (`[[slug#section]]`) as the citation primitive.
  Page-level only in v1.

## In-scope (delivered across phases 13-15)

- Orchestrator script `scripts/synth.sh` (subcommands: `new`, `regen`,
  `finalize`, `list`, `resolve`, `refine`).
- Plugin loader supporting single-file (`synthesis-plugins/<name>.md`) and
  directory (`synthesis-plugins/<name>/`) forms.
- Four shipped plugins: `briefing`, `mindmap`, `timeline`, `study-guide`.
- Synthesis page format with frontmatter scope, BEGIN/END managed-region
  markers, persistent `## Notes` and `## Feedback` sections.
- Lint rules L1-L8 enforcing marker integrity, required sections,
  evidence-quote substring match, citation slug existence, scope drift,
  hand-edit detection, feedback metrics.
- MCP server tools `list_synth_plugins()` and `synthesize(plugin, scope,
  topic_slug)`.
- justfile recipes `synth`, `synth-regen`, `synth-refine`, `synth-list`.
- Optional Anki export helper for `study-guide` outputs.
- Mermaid syntax validation post-hook for `mindmap` outputs.
- WIKI.md, README.md, just-help, sample-wiki updates.

---

## Architecture

Adds one new layer (`synthesis-plugins/`) and extends three existing ones
(`scripts/`, `mcp/awiki-server/`, `content/synthesis/`).

```
            ┌────────────────────────────────────────────────────────┐
            │ User / Agent                                           │
            │   - just synth <plugin> [--tag=X | --slugs=...]        │
            │   - MCP synthesize(plugin, scope) tool                 │
            │   - Conversational: "agent, briefing on memex"         │
            └─────────────────────────┬──────────────────────────────┘
                                      │
                                      ▼
            ┌────────────────────────────────────────────────────────┐
            │ scripts/synth.sh    (orchestrator, mechanical only)    │
            │   1. resolve plugin (synthesis-plugins/<name>.md)      │
            │   2. resolve/create scope page in content/synthesis/   │
            │   3. read scope → page-set                             │
            │   4. emit prompt bundle for agent (stdout)             │
            │   5. agent writes between markers                      │
            │   6. update catalog, log-append, optional lint         │
            └─────────────────────────┬──────────────────────────────┘
                                      │
                                      ▼
            ┌────────────────────────────────────────────────────────┐
            │ synthesis-plugins/  (plugin registry)                  │
            │   briefing.md, mindmap.md, timeline.md, study-guide.md │
            │   single-file form (frontmatter + prompt body);        │
            │   loader also accepts <name>/ directory form (v2)      │
            └─────────────────────────┬──────────────────────────────┘
                                      │
                                      ▼
            ┌────────────────────────────────────────────────────────┐
            │ content/synthesis/<slug>.md  (synthesis page)          │
            │   - frontmatter: type, plugin, scope, last_generated   │
            │   - body: lead, ## Notes, ## Feedback                  │
            │   - <!-- BEGIN GENERATED --> ... <!-- END --> region   │
            │   - ## Evidence section w/ quoted spans                │
            └────────────────────────────────────────────────────────┘
```

### Touch points on existing layers

- `WIKI.md` Section 6 (Output Formats): document synthesis pages now produced
  via plugin registry. Existing `synthesis|deck|chart|canvas` enum gains
  optional frontmatter `plugin: <name>`.
- `WIKI.md` Section 7 (Lint Checklist): add L1-L8 to mechanical list.
- `scripts/lint.sh` sources `scripts/lint-synth.sh` for synth-specific rules;
  new `--only=synth` flag.
- `scripts/update-catalog.sh` recognizes synthesis pages with `plugin:` and
  groups them under a "Synthesis" catalog section.
- `mcp/awiki-server` gains `list_synth_plugins()` and `synthesize(...)` tools.
- `justfile` gets `synth`, `synth-regen`, `synth-refine`, `synth-list` recipes.
- BOOTSTRAP: no new prompt; plugins ship pre-loaded. Optional final-step hint
  prints "Try `just synth briefing --tag=foo` once you've ingested 3+ sources."

### Generation execution model

Hybrid (per the framework choice):

- **Text plugins (all 4 v1 plugins)** — agent-driven. Plugin = prompt
  template. Orchestrator scaffolds + emits prompt; agent generates content
  via its existing harness; agent calls `finalize`. No LLM calls in scripts;
  no API key needed.
- **Binary plugins (post-v1)** — script-driven with TTS/render API. Reserved
  for audio/video. The directory plugin form already supports this via
  `post.sh`.

---

## Plugin Contract

### Single-file form (v1 default — all 4 shipped plugins use this form)

`synthesis-plugins/<name>.md`:

```yaml
---
name: briefing
description: One-page exec summary of a curated source set.
version: 1
output_type: synthesis              # maps to content/synthesis/ frontmatter type
output_subtype: briefing            # advisory; surfaces in catalog grouping
min_sources: 2
max_sources: 50                     # plugin refuses if scope exceeds; agent narrows
required_sections:                  # lint enforces these headings inside markers
  - "## TL;DR"
  - "## Key Findings"
  - "## Open Questions"
  - "## Evidence"
post_hook: null                     # optional path; null for v1 plugins (briefing)
---

# Prompt

You are generating a one-page briefing from {{scope_description}}.

Pages in scope:
{{#pages}}
- [[{{slug}}]] ({{type}}) — {{lead_paragraph}}
{{/pages}}

Produce markdown with these sections:
1. **TL;DR** — 3 bullets, ≤25 words each. Each bullet ends with `[[slug]]`.
2. **Key Findings** — 5-8 bullets. Group related claims. Cite `[[slug]]` per claim.
3. **Open Questions** — 2-4 items the source set raises but doesn't answer.
4. **Evidence** — for each Key Finding, one verbatim quote ≤30 words:

   > "exact quote text" — [[slug]]

Constraints:
- Inline citations use page-level wikilinks only.
- Evidence quotes MUST appear verbatim in the cited source's body.
- If a finding can't be evidenced from the scope, omit it.
- Output goes between the BEGIN GENERATED / END GENERATED markers in the
  target page. Do not modify content outside the markers.

{{#feedback}}
## Human Feedback (binding)

The user has provided the following refinement directives. Treat each bullet
as a binding constraint on this regeneration:

{{feedback}}

If any bullet conflicts with a plugin-required section or invariant
(citations, evidence rules, marker discipline), keep the invariant and
surface the conflict under "## Open Questions" or equivalent.
{{/feedback}}
```

**Required manifest fields:** `name`, `description`, `output_type`,
`required_sections`, `min_sources`. All others optional with documented
defaults.

### Directory form (v2-ready, loader accepts but no v1 plugin uses it)

```
synthesis-plugins/<name>/
├── plugin.yaml      # same fields as single-file frontmatter
├── prompt.md        # template body
├── post.sh          # optional — runs after agent writes file
└── schema.json      # optional — JSON-schema for output validation
```

Loader picks form by checking `synthesis-plugins/<name>.md` first, then
`synthesis-plugins/<name>/plugin.yaml`. Name collision (both forms present
for same name) = lint error.

### Discovery

`scripts/synth.sh list` scans `synthesis-plugins/` and prints
`name | description | output_type` rows. MCP `list_synth_plugins()` returns
the same data structured.

---

## Synthesis Page Format

A synthesis page has four content zones, of which only one is machine-managed.

`content/synthesis/<topic>-<plugin>.md`:

```markdown
---
title: "Memex History — Briefing"
date: 2026-04-27
last_updated: 2026-04-27
last_generated: 2026-04-27T14:32:11Z
type: synthesis
plugin: briefing
scope:
  tag: memex                          # OR slugs: [s-as-we-may-think, ...]
                                      # OR query: "..." (qmd-search expression)
  exclude_tags: [draft]               # optional
  min_last_updated: 2025-01-01        # optional — freshness filter on inputs
  types: [source, entity]             # optional — restrict input page types
tags: [memex, briefing]
aliases: []
sources: []                           # populated by synth.sh from resolved scope
draft: false
---

Lead paragraph — written once by user/agent, NOT regenerated.
Describes what this notebook is for. Survives regen.

## Notes

User-authored notes. Free-form. Survives regen. Cite sources inline if you
want. Treated as additional context (knowledge), not as instructions.

## Feedback

- Tighten TL;DR to 15 words per bullet, current length too verbose.
- Add coverage of `[[c-associative-trails]]` — under-cited so far.
- Drop the "Open Questions" section, not useful for this notebook.

<!-- BEGIN GENERATED plugin=briefing scope_hash=a3f9c2 -->

## TL;DR
- ...claim... [[s-as-we-may-think]]
- ...claim... [[e-vannevar-bush]]
- ...claim... [[c-associative-trails]]

## Key Findings
- ...

## Open Questions
- ...

## Evidence
> "exact verbatim quote from source" — [[s-as-we-may-think]]
> "another quote" — [[e-vannevar-bush]]

<!-- END GENERATED -->
```

### Zone rules

| Zone | Lifetime | Edited by |
|------|----------|-----------|
| Frontmatter | persistent; `last_generated`/`sources` rewritten on regen | agent + synth.sh |
| Lead paragraph | persistent | user/agent (initial), preserved on regen |
| `## Notes` | persistent | user; agent treats as context |
| `## Feedback` | persistent; bullets curated by user | user (`synth.sh refine` appends) |
| Generated region (between markers) | rewritten on regen | agent only; user edits warned by lint |

### Frontmatter rules

- `plugin:` required. Pins page to one plugin. Changing plugin = create new
  page.
- `scope:` required. One of `tag`, `slugs`, `query`. Optional filters:
  `exclude_tags`, `min_last_updated`, `types`.
- `last_updated:` (existing wiki convention) — bumped on any edit to the
  page, by user or agent. Used by general lint rules (>90d staleness).
- `last_generated:` (synth-specific) — ISO-8601 UTC. Stamped by
  `synth.sh finalize` / `accept-stage` only. Tracks when the generated
  region was last produced. Distinct from `last_updated`: a user editing
  `## Feedback` bumps `last_updated` but NOT `last_generated`.
- `sources:` populated from resolved scope at gen-time. Hand-edits are
  overwritten on regen.

### Marker rules

- Markers MUST appear once each, BEGIN before END. Lint error otherwise.
- `scope_hash=<6-char>` in BEGIN marker = first 6 hex chars of
  `sha256(sorted(resolved_slug_list).join("\n"))`. Drift detection (lint
  L5). 6 chars is sufficient because the comparison is against the
  page's own previous hash, not a global namespace.
- Region between markers = exclusively machine-managed. Lint warns on edits
  inside (L6).
- Plugin prompt explicitly instructs agent NOT to touch content outside
  markers.

### Naming convention

`<topic>-<plugin>.md` (e.g., `memex-briefing.md`, `memex-mindmap.md`). Same
topic + different plugins = separate pages, cross-linkable via wikilinks.

---

## `synth.sh` Orchestrator

Mechanical only: resolves plugin, resolves scope, writes scaffold, emits
prompt for agent. Does NOT call any LLM.

`#!/usr/bin/env bash`, `set -euo pipefail`, bash 4+, idempotent where
possible.

### CLI surface

```
synth.sh new <plugin> <topic-slug> [--tag=X | --slugs=a,b,c | --query="..."]
                                   [--exclude-tags=X,Y] [--min-last-updated=YYYY-MM-DD]
                                   [--types=source,entity]
synth.sh regen <synthesis-page-slug> [--force] [--stage]
synth.sh accept-stage <synthesis-page-slug>
synth.sh finalize <synthesis-page-slug>
synth.sh list
synth.sh resolve <synthesis-page-slug>
synth.sh refine <synthesis-page-slug> "<note>"
```

### `synth.sh new` — first-time generation

1. Validate plugin exists in `synthesis-plugins/`. Exit 1 if missing.
2. Resolve scope from CLI args. Build slug list. Apply `min_sources` /
   `max_sources` from manifest. Exit 2 if out of range (print actionable
   count).
3. Build target path: `content/synthesis/<topic-slug>-<plugin>.md`. Exit 3 if
   exists (suggest `regen`).
4. Scaffold page: write frontmatter (incl. resolved scope, empty
   `last_generated`), lead-paragraph placeholder, `## Notes` heading, BEGIN
   and END markers (empty between them) with computed `scope_hash`. Do NOT
   create empty `## Feedback` (created lazily on first `refine`).
5. Emit prompt bundle to stdout: plugin prompt template with
   `{{scope_description}}` and `{{pages}}` interpolated (latter populated by
   single-pass scan of resolved pages' frontmatter + lead paragraphs).
   Include `{{feedback}}` block only if `## Feedback` non-empty (won't be on
   `new`).
6. Append `log-append.sh synth-scaffold "<plugin> <topic>"`.
7. Exit 0. Caller (agent or shell) consumes the prompt, generates content,
   writes between markers, then runs `synth.sh finalize <slug>`.

### `synth.sh finalize <slug>`

Called by the agent after writing generated content into the markers.

1. Validate marker integrity (exactly one BEGIN, one END, BEGIN before END).
2. Run scoped lint: `lint.sh --only=synth --file=<path>` — checks required
   sections, evidence-quote substrings, citation-slug existence.
3. On clean lint: stamp `last_generated` to now-UTC, re-hash scope, update
   BEGIN marker `scope_hash`, populate frontmatter `sources:` from resolved
   slug list.
4. Append `log-append.sh synth "<plugin> <topic>"`.
5. Increment `.awiki/ingest-count` (synthesis counts toward auto-lint
   cadence).
6. Suggest catalog update (`scripts/update-catalog.sh`).
7. Exit 0. Lint failures = exit 6 with structured output, agent retries.

### `synth.sh regen`

1. Read target page frontmatter. Validate `plugin:` and `scope:` present.
2. Hand-edit detection: compare working-tree to last committed version
   (`git show HEAD:<path>`). If the diff intersects the generated region
   between BEGIN/END markers → suspect manual edit. Exit 4 unless `--force`
   or `--stage`. Diffs that touch only `## Notes`, `## Feedback`, lead
   paragraph, or frontmatter (excluding `last_generated` / `sources`) do
   NOT trigger this. Mtime alone is NOT used: `synth refine` and ordinary
   note-taking bump mtime without touching markers and must not produce a
   false positive. If the file is untracked or has no committed prior
   version, skip this check (treat as fresh).
3. Re-resolve scope. Compute new `scope_hash`. If unchanged AND no
   `min_last_updated` push, prompt "no scope drift, regen anyway? (y/N)" —
   non-interactive callers must pass `--force`.
4. `--stage` mode: write to `content/synthesis/.staged/<slug>.md`; print diff
   path; do NOT touch live page until `synth.sh accept-stage <slug>`. The
   staged file inherits the live page's frontmatter and outside-marker
   content; only the generated region differs.
5. Default mode: clear region between markers, scaffold updated header
   (scope_hash etc.), then emit prompt bundle (with `{{feedback}}`
   interpolation if `## Feedback` non-empty). Agent fills, calls `finalize`.

### `synth.sh accept-stage <slug>`

1. Verify `content/synthesis/.staged/<slug>.md` exists. Exit 7 if missing.
2. Run `lint.sh --only=synth --file=<staged-path>`. On lint failure, leave
   staged file in place, exit 6.
3. Replace live `content/synthesis/<slug>.md` with staged file.
4. Delete staged file.
5. Stamp `last_generated`, populate `sources:`, append log entry. (Same
   tail as `finalize`.)
6. Exit 0.

### `synth.sh refine <slug> "<note>"`

1. Read target page; locate `## Feedback` section. Create it (above markers,
   below `## Notes` if present) if absent.
2. Append `- <note>` as a bullet. Idempotent: skip if exact-duplicate bullet
   already present.
3. Print appended bullet for confirmation. Do NOT trigger regen
   automatically.

### Exit codes

| Code | Meaning |
|------|---------|
| 0 | OK |
| 1 | Plugin missing/invalid |
| 2 | Scope resolution failure (too few/many sources, invalid filter) |
| 3 | Target page already exists (use `regen`) |
| 4 | Hand-edit detected since last_generated; pass `--force` or `--stage` |
| 5 | Marker integrity failure |
| 6 | Lint failure during finalize |
| 7 | Stage-mode write failure |

### justfile recipes

```just
# === synthesis ===
synth plugin topic *args:
    bash scripts/synth.sh new {{plugin}} {{topic}} {{args}}

synth-regen slug *args:
    bash scripts/synth.sh regen {{slug}} {{args}}

synth-refine slug *note:
    bash scripts/synth.sh refine {{slug}} "{{note}}"

synth-list:
    bash scripts/synth.sh list
```

`*args` and `*note` (variadic) follow the same quoting convention as
existing recipes (`commit`, `search`, `log`); `docs/just-help.txt` documents
quoting in every example.

### MCP tools (`mcp/awiki-server`)

- `list_synth_plugins()` — returns array of plugin manifests (parsed from
  `synthesis-plugins/`).
- `synthesize(plugin, scope_descriptor, topic_slug)` — wraps `synth.sh new`,
  returns scope resolution + prompt bundle. Agent generates content, calls
  `finalize_synthesis(topic_slug)` which wraps `synth.sh finalize`.
- All shell-outs use `execFileSync` with argument arrays (no shell
  interpolation). Tool inputs validated by JSON Schema. Path traversal in
  `topic_slug` rejected (must match `^[a-z0-9-]+$`).

### Conversational flow (no CLI)

Agent reads `WIKI.md` synthesis section, knows the pattern: call
`list_synth_plugins`, pick plugin, scaffold via `synthesize`, fill body,
call `finalize_synthesis`. Same outcome as the CLI path.

---

## Lint Rules + Anti-Hallucination

Six new mechanical rules + two metric rules. All emit
`LINT|<level>|<file>|<msg>` per existing convention. Implemented in
`scripts/lint-synth.sh`, sourced by `lint.sh`.

| # | Rule | Level | What it catches |
|---|------|-------|-----------------|
| L1 | Marker integrity | error | Missing/duplicate/swapped BEGIN/END markers in `content/synthesis/*.md` pages with `plugin:` frontmatter. |
| L2 | Required sections present | error | Each plugin's `required_sections` must appear inside the generated region. |
| L3 | Evidence quote substring | error | Each `> "quote" — [[slug]]` line: load source, normalize whitespace, assert quote substring of body. Hallucinated/paraphrased = error w/ closest-fuzzy-match suggestion. |
| L4 | Citation slug in scope | error | Every `[[slug]]` in the generated region resolves AND is in resolved scope. Catches agent citing out-of-scope pages. |
| L5 | Scope drift | warning | Recompute `scope_hash`. If differs, "N new sources, M removed since last regen — consider `just synth-regen`". |
| L6 | Hand-edit inside markers | warning | If file mtime > `last_generated` AND diff (vs parent commit) intersects the generated region, "manual edit will be lost on regen. Move to `## Notes` or `--force` to acknowledge." |
| L7 | Feedback count metric | info | `feedback_count=N` reported in synth-lint summary. |
| L8 | Out-of-scope feedback ref | warning | A `## Feedback` bullet mentions a `[[slug]]` not in resolved scope → "feedback references out-of-scope page; widen scope or remove bullet". |

### L3 implementation detail (anti-hallucination spine)

1. Parse each line matching `^>\s+"(.+)"\s+—\s+\[\[([a-z0-9-]+)\]\]$`.
2. Resolve `<slug>` to its file path via existing slug→path map
   (`.awiki/maps/slug-to-path.tsv`).
3. Load source body. Strip frontmatter. Collapse whitespace runs to single
   space.
4. Apply same collapse to the quote.
5. `grep -F` substring check. Match = OK. Miss = error.
6. On miss: `python3 -c "import difflib; print(difflib.get_close_matches(...))"`
   over source paragraphs, surface top suggestion in lint message.

Edge cases:

- Quote contains a wikilink `[[other-slug]]`: rewrite to display text before
  substring check.
- Quote crosses paragraphs: handled by whitespace collapse.
- Smart-vs-straight quotes: lint normalizes both source and quote to
  straight quotes before compare. Plugin prompt instructs straight-quote
  output.

### Existing lint integration

- Synth lint runs as part of `just lint` (no opt-in).
- `lint.sh --only=synth` runs only synth rules; called by
  `synth.sh finalize`.
- `lint.sh --fix` for synth: only mechanical fix is normalizing
  smart→straight quotes inside generated regions. Marker repair,
  missing-section repair, evidence-quote correction = NOT in `--fix`
  (require regen).

### Semantic lint additions in `WIKI.md`

- Synthesis page review: agent re-reads its own evidence section after
  generation; cross-checks claim ↔ quote semantic alignment (quote
  substantively supports claim, not just topically related). Agent-driven,
  not mechanical.

---

## Iterative Refinement (Human-in-Loop)

Two channels for human steering, plus an implicit third (`## Notes`). All
persist across regens; all are git-tracked.

### Channel A — `## Feedback` section (persistent, curated)

Markdown section in the synthesis page, outside the generated markers.
Bullets are binding refinement directives.

Rules:

- Lives outside markers — preserved automatically.
- Plugin loader injects this section verbatim into the next regen prompt as
  a `{{feedback}}` block, with explicit instruction: "Treat each bullet as
  a binding constraint. If a bullet contradicts a plugin-required section,
  surface conflict and proceed with the plugin requirement."
- User curates: edits, removes resolved items, adds new ones. No expiry —
  entries apply until user removes them.
- Lint L7 reports `feedback_count`; if >20, emit warning suggesting scope
  refactor or page split.

### Channel B — `synth.sh refine` (ad-hoc CLI append)

`just synth-refine memex-briefing "add a section comparing Bush's vision to Engelbart's NLS"`

Behavior already specified in the orchestrator section. Multiple `refine`
calls batch into one regen. User runs `synth-regen` when ready.

### Channel C (implicit) — direct edit to `## Notes`

Already designed: free-form notes section outside markers. Agent reads it as
additional context but treats it as user-authored knowledge, NOT as
instructions. Notes inform synthesis content; Feedback shapes synthesis
style/scope.

### Why this design

- Survives regen — feedback in markdown, not ephemeral CLI state.
- Auditable — git diff shows every refinement directive's lifecycle.
- Composable — multiple refinements batch into one regen.
- Unambiguous — Feedback (instructions) and Notes (content) are separate
  sections with different agent semantics.

---

## Plugin Specs (v1 Set)

### `briefing`

**Purpose:** One-page exec summary. NotebookLM "Briefing Doc" equivalent.

**Manifest highlights:**

```yaml
name: briefing
output_subtype: briefing
min_sources: 2
max_sources: 50
required_sections:
  - "## TL;DR"
  - "## Key Findings"
  - "## Open Questions"
  - "## Evidence"
post_hook: null
```

**Output structure:**

- TL;DR — 3 bullets, ≤25 words each, one citation per bullet.
- Key Findings — 5-8 bullets, grouped where related, citation per claim.
- Open Questions — 2-4 items the source set raises but doesn't answer.
- Evidence — verbatim quote per Key Finding, ≤30 words.

**Prompt notes:** Bias toward primary-source claims (frontmatter
`type: source`) over derivative pages (`type: synthesis`); reduces
echo-chamber risk when scope spans both layers.

### `mindmap`

**Purpose:** Visual concept graph rendered via Mermaid (Hugo + Obsidian both
render natively).

**Manifest highlights:**

```yaml
name: mindmap
output_subtype: mindmap
min_sources: 3
max_sources: 30
required_sections:
  - "## Mindmap"
  - "## Legend"
  - "## Pages"
  - "## Evidence"
post_hook: scripts/synth-mindmap-validate.sh
```

**Output structure:**

- Mindmap — single fenced ` ```mermaid ` block, `mindmap` directive. Root =
  topic. Branches = clusters. Leaves = display text from page frontmatter.
  Max depth 3, max ~40 nodes (Mermaid renders poorly past that).
- Legend — bullet list mapping cluster names to brief descriptions.
- Pages — `display-text → [[slug]]` rows for navigation (Mermaid `mindmap`
  doesn't support markdown-link nodes; this section bridges that).
- Evidence — per cluster, one quote justifying the cluster's coherence.

**post_hook:** `synth-mindmap-validate.sh` runs mermaid CLI (`@mermaid-js/mermaid-cli`,
`mmdc`) in `--dry-run` to validate syntax. Optional dep — falls back to
regex sanity check (balanced braces, valid `mindmap` keyword) if `mmdc` not
installed. Failure = finalize exit 6.

### `timeline`

**Purpose:** Chronological extraction across the source set. NotebookLM
"Timeline" equivalent.

**Manifest highlights:**

```yaml
name: timeline
output_subtype: timeline
min_sources: 3
max_sources: 50
required_sections:
  - "## Timeline"
  - "## Themes"
  - "## Evidence"
render: mermaid                     # alternative: table
```

**Output structure:**

- Timeline — Mermaid `timeline` block (default) OR fallback markdown table.
  Entries: `<year>: <event one-liner> [[slug]]`. Mermaid mode groups by
  decade.
- Themes — 3-5 bullets identifying recurring patterns across the chronology.
  Each bullet cites 2+ slugs.
- Evidence — verbatim quotes anchoring each theme.

**Date extraction:** Plugin prompt instructs agent to source dates from page
bodies, NOT just `date:` frontmatter (which is page-creation date, not
event date). Uncertain dates marked `c.<year>` or `<year>?`.

### `study-guide`

**Purpose:** Active-recall study aid. NotebookLM "Study Guide" + flashcards
equivalent.

**Manifest highlights:**

```yaml
name: study-guide
output_subtype: study-guide
min_sources: 1                      # single-source study common (textbook chapter)
max_sources: 30
required_sections:
  - "## Concept Checklist"
  - "## Short-Answer Questions"
  - "## Flashcards"
  - "## Suggested Deep-Dives"
  - "## Evidence"
post_hook: null
```

**Output structure:**

- Concept Checklist — bullet list of must-know concepts, each linked
  `[[slug]]`. Reader self-marks `[ ]` / `[x]`.
- Short-Answer Questions — 5-10 questions, each with a hidden answer
  (`<details>` block — renders collapsibly in both Hugo and Obsidian) and
  `[[slug]]` source citation.
- Flashcards — Anki-importable format: `Q: ...` / `A: ...` blocks separated
  by `---`. `scripts/synth-export-anki.sh` converts these to Anki `.apkg`
  (optional dep: `genanki` Python lib).
- Suggested Deep-Dives — 3-5 follow-up questions/sources.
- Evidence — quotes backing the answers in Short-Answer.

### Common across all 4 plugins

- All emit `## Evidence`.
- All forbid prose paraphrase outside cited claims.
- All use page-level wikilinks (no section anchors in v1).
- All inject `{{feedback}}` block when `## Feedback` non-empty.
- All ship as single-file plugins under `synthesis-plugins/`. Only `mindmap`
  has a non-null `post_hook`.

---

## Testing

### Phase 13 (Synth core)

- `synth_test.bats` — fixture wiki under `tests/fixtures/wiki-synth/` with 3
  sources tagged `memex`.
  - `synth.sh new briefing memex --tag=memex` → creates page, scope
    resolves to 3 slugs, `last_generated` empty, markers present, prompt
    emitted to stdout.
  - `synth.sh new briefing memex` again → exit 3 (already exists).
  - `synth.sh new briefing too-narrow --tag=nonexistent` → exit 2 (zero
    sources).
  - `synth.sh resolve memex-briefing` → prints the 3 slugs.
  - `synth.sh finalize memex-briefing` after writing valid content into
    markers → stamps `last_generated`, populates `sources:` frontmatter,
    exit 0.
  - `synth.sh finalize` with broken markers → exit 5.
- Plugin loader unit test — manifest parsing, missing required field
  detection, single-file vs directory form precedence, name collision
  detection.

### Phase 14 (Lint + remaining plugins)

- `lint_synth_test.bats`:
  - L1: double BEGIN → error.
  - L2: missing `## Evidence` → error.
  - L3: hallucinated quote → error w/ suggestion. Smart-quote source +
    straight-quote claim still matches.
  - L4: out-of-scope citation → error.
  - L5: add new tagged source, scope_hash mismatch → warning.
  - L6: `touch` page after `last_generated`, lint warns when diff
    intersects markers.
- Plugin scaffold tests — `synth.sh new <plugin>` for each of mindmap /
  timeline / study-guide produces correct manifest-driven scaffold.
- Mermaid post-hook — fixture with invalid mermaid → post_hook exits
  non-zero, `finalize` exits 6.

### Phase 15 (Refinement + MCP)

- `synth_refine_test.bats` — `synth.sh refine <slug> "<note>"` appends
  bullet, creates `## Feedback` if absent, idempotent on exact duplicates,
  preserves user-edited bullets.
- L7/L8 lint tests — feedback count surfaced; out-of-scope feedback slug →
  warning.
- MCP server tests (Node) — `list_synth_plugins` returns expected 4
  manifests; `synthesize` invokes orchestrator and returns scope resolution.
- Anki export — fixture study-guide page → `synth-export-anki.sh` produces
  parseable `.apkg`. Skipped if `genanki` not installed; documented.
- End-to-end smoke test: ingest 3 sources tagged `demo`, run
  `just synth briefing demo --tag=demo`, agent fills body, `just lint`
  clean, `hugo --renderToMemory` clean, page appears in catalog under
  Synthesis section.

### CI

Existing `scheduled/github-action.yml.example` (phase 9) covers `just test`
and `just lint`; no CI changes needed. Mermaid post-hook is optional, so
lint failures from a missing `mmdc` install are downgraded to warnings in
CI per existing soft-dep convention.

---

## Threat Model / Privacy Considerations

| Risk | Mitigation |
|------|------------|
| Plugin prompt injection via crafted source content | Plugins are static template files in the repo; sources are interpolated as data, not as prompt instructions. `WIKI.md` cautions agents to treat source content as quoted material, not as commands. |
| Synthesis page leaks private content from scope | Existing `tags: [private]` lint rule applies to synthesis pages. Lint warns if any resolved source is `private` but the synthesis page is not under `content/private/`. |
| Hallucinated evidence quotes | L3 substring check is hard-fail. No way to ship a synthesis page with an unverifiable quote past `finalize`. |
| Out-of-scope citations | L4 hard-fail. Agent can't cite a page outside the resolved scope. |
| MCP `synthesize` arg injection | `topic_slug` validated against `^[a-z0-9-]+$`. Plugin name validated against directory listing. Scope descriptor JSON-schema validated. All shell-outs use `execFileSync` with argument arrays. |
| User-authored plugin runs untrusted code | Single-file plugins are prompt-only — no code execution. Directory-form plugins with `post.sh` ARE arbitrary code. WIKI.md warns: "Treat third-party `synthesis-plugins/<name>/post.sh` as untrusted shell code; review before adding." |
| Mermaid CLI download supply-chain | `mmdc` is optional. If installed, user installs from npm (their choice). Lint fallback (regex check) exists for users who don't want the npm dep. |
| Anki export leaks data | `synth-export-anki.sh` writes locally; no upload. User imports manually. |

---

## Dependencies & Compatibility

Additions to the existing `Dependencies & Compatibility` table:

| Tool | Required? | Min version | Notes |
|------|-----------|-------------|-------|
| `python3` (existing) | yes | 3.8+ | also used by L3 fuzzy-match (`difflib` is stdlib). |
| `@mermaid-js/mermaid-cli` (`mmdc`) | optional | 10+ | `mindmap` post-hook validation. Falls back to regex sanity check if absent. |
| `genanki` (Python) | optional | 0.13+ | `study-guide` Anki export. Skipped if absent. |

No new required tools.

---

## Implementation Phases (extends master plan)

Appended to the existing 12-phase plan as phases 13-15.

| # | Phase | Depends on | Deliverable |
|---|-------|------------|-------------|
| 13 | Synth core | 1, 2, 3, 6 | `scripts/synth.sh` (`new`/`regen`/`finalize`/`list`/`resolve`/`refine`), plugin loader (single-file form), `briefing` plugin shipped, scope resolution, marker scaffolding, `## Notes` preservation, BATS tests for orchestrator. |
| 14 | Synth lint + remaining plugins | 13 | `lint-synth.sh` (L1-L6), `mindmap` / `timeline` / `study-guide` plugins, mermaid post-hook, `--only=synth` flag, fuzzy-match suggestion in evidence-quote errors. |
| 15 | Refinement + MCP integration | 13, 14, 8 | `## Feedback` channel + prompt template injection, lint L7/L8, `list_synth_plugins` + `synthesize` MCP tools, `synth-export-anki.sh` (optional), WIKI.md synthesis-workflow doc, `examples/sample-wiki/` synthesis demo. |

**Spike absorption:** Mermaid `mindmap` rendering quality at >20 nodes
(phase 14 front); Anki export reliability via `genanki` (phase 15 front).
Failure of either degrades gracefully (mindmap → nested-list rendering;
Anki export → out of v1 if `genanki` integration breaks).

**Definition of done per phase.** Same as master plan: (a) plan tasks check
off, (b) phase tests pass, (c) phase smoke test in README runs clean, (d)
merged to `main` with green pre-commit lint.

**No deferrals beyond phases 13-15.** Audio/video overview, multilingual
generation, section-anchor citations, plugin marketplace, etc. are out of
scope for this spec. Future synthesis features get their own specs.

---

## Documentation Updates

Additive across phases 13-15:

- `WIKI.md` Section 6 (Output Formats) — synthesis-via-plugin workflow.
- `WIKI.md` Section 7 (Lint Checklist) — L1-L8 in mechanical list.
- `WIKI.md` new Section 9 — synthesis workflow (scaffold → generate →
  finalize → refine → regen).
- `README.md` smoke-test section — synthesis step added.
- `docs/just-help.txt` — `synth`, `synth-regen`, `synth-list`,
  `synth-refine` recipes documented.
- `examples/sample-wiki/` — pre-rendered synthesis page
  (`memex-briefing.md`) demonstrating the format.
