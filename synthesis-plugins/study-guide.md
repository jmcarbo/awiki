---
name: study-guide
description: Active-recall study aid; concept checklist, short-answer Qs, flashcards.
version: 1
output_type: synthesis
output_subtype: study-guide
min_sources: 1
max_sources: 30
max_evidence_total_words: 300
required_sections:
  - "## Concept Checklist"
  - "## Short-Answer Questions"
  - "## Flashcards"
  - "## Suggested Deep-Dives"
  - "## Evidence"
post_hook: null
---

# Prompt

You are generating a study guide from {{scope_description}}.

Pages in scope:
{{#pages}}
- [[{{slug}}]] ({{type}}) — {{lead_paragraph}}
{{/pages}}

Produce markdown with these sections:

1. **Concept Checklist** — bullet list of must-know concepts. Each bullet:
   `- [ ] <concept name> — [[<slug>]]`. Reader self-marks `[ ]`/`[x]`.
2. **Short-Answer Questions** — 5-10 questions. Each question is followed by
   a collapsible answer block (renders in both Hugo and Obsidian):

   ```
   <details>
   <summary>Q1: <question text></summary>

   <answer text> — [[<slug>]]
   </details>
   ```

3. **Flashcards** — Anki-importable. One card per block, blocks separated
   by a line containing only `---`:

   ```
   Q: <front>
   A: <back> [[<slug>]]
   ```

   The `Q:` and `A:` prefixes are load-bearing; `synth-export-anki.sh`
   parses them. Citations belong on the `A:` line.
4. **Suggested Deep-Dives** — 3-5 follow-up questions or specific page
   recommendations the source set raises but doesn't fully answer.
5. **Evidence** — verbatim quotes (≤30 words each) backing the
   short-answer answers:

   > "exact quote text" — [[slug]]

Constraints:
- Citations are page-level wikilinks.
- Evidence quotes MUST appear verbatim in the cited source's body.
- Aggregate cap is **300 words across all evidence quotes** (lint S9). If
  you would exceed it, drop the least load-bearing quote.
- Output goes between BEGIN GENERATED and END GENERATED markers.

{{#feedback}}
## Human Feedback (binding)

```text
{{feedback}}
```
{{/feedback}}
