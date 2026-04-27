---
name: timeline
description: Chronological extraction across the source set; mermaid or markdown table.
version: 1
output_type: synthesis
output_subtype: timeline
min_sources: 3
max_sources: 50
max_evidence_total_words: 500
required_sections:
  - "## Timeline"
  - "## Themes"
  - "## Evidence"
render: mermaid
---

# Prompt

You are generating a chronological timeline from {{scope_description}}.

Pages in scope:
{{#pages}}
- [[{{slug}}]] ({{type}}) — {{lead_paragraph}}
{{/pages}}

Produce markdown with these sections:

1. **Timeline** — render mode = `{{render}}`.
   - If `mermaid`: a single fenced ```mermaid``` block starting with the
     `timeline` keyword. Group entries by decade as Mermaid `section`
     headings (e.g., `section 1940s`). Within each section, one entry per
     line: `<year>: <event one-liner> [[<slug>]]`.
   - If `table`: a markdown table with columns `Year | Event | Source`. The
     `Source` column contains a `[[<slug>]]` wikilink.
   Date discipline:
   - Source dates from the **page body**, not just frontmatter `date:`
     (which is page-creation date, not event date).
   - Uncertain dates are marked `c.<year>` (circa) or `<year>?`.
   - One entry per distinct event. Do not duplicate the same event under
     two slugs; pick the more authoritative source.
2. **Themes** — 3-5 bullets identifying recurring patterns across the
   chronology. Each bullet cites at least 2 slugs.
3. **Evidence** — verbatim quotes (≤30 words) anchoring each theme:

   > "exact quote text" — [[slug]]

Constraints:
- Inline citations are page-level wikilinks.
- Evidence quotes MUST appear verbatim in the cited source's body.
- Output goes between BEGIN GENERATED and END GENERATED markers.

{{#feedback}}
## Human Feedback (binding)

```text
{{feedback}}
```
{{/feedback}}
