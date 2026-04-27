---
name: mindmap
description: Visual concept graph rendered via Mermaid (Hugo + Obsidian native).
version: 1
output_type: synthesis
output_subtype: mindmap
min_sources: 3
max_sources: 30
max_evidence_total_words: 500
required_sections:
  - "## Mindmap"
  - "## Legend"
  - "## Pages"
  - "## Evidence"
post_hook: scripts/synth-mindmap-validate.sh
---

# Prompt

You are generating a Mermaid `mindmap` from {{scope_description}}.

Pages in scope:
{{#pages}}
- [[{{slug}}]] ({{type}}) — {{lead_paragraph}}
{{/pages}}

Produce markdown with these sections:

1. **Mindmap** — exactly one fenced ```mermaid``` block. The block MUST start
   with the word `mindmap` on its own line, then a single root node = the
   notebook topic. Branches = thematic clusters (3-5). Leaves = display text
   from each page's frontmatter `title:`. **Mermaid `mindmap` does not support
   markdown links inside nodes** — therefore leaves are plain text only; the
   `## Pages` section bridges from text back to wikilinks.
   Constraints: max depth 3, max ~40 nodes total. Past 40 the diagram does
   not render legibly; collapse aggressively.
2. **Legend** — one bullet per cluster: `- <cluster name>: <one-sentence
   description of what it groups>`.
3. **Pages** — one row per leaf in the mindmap, in this exact format:
   `<display text> → [[<slug>]]`. This section is the navigation bridge from
   the mermaid leaves back into the wiki.
4. **Evidence** — for each cluster, one verbatim quote (≤30 words) from a
   source page that justifies the cluster's coherence:

   > "exact quote text" — [[slug]]

Constraints:
- Inline citations are page-level wikilinks only.
- Evidence quotes MUST appear verbatim in the cited source's body.
- Leaves in the mermaid block contain plain text only (no `[[...]]`, no
  markdown links).
- Output goes between BEGIN GENERATED and END GENERATED markers. Do not
  modify content outside markers.

{{#feedback}}
## Human Feedback (binding)

```text
{{feedback}}
```
{{/feedback}}
