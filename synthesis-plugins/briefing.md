---
name: briefing
description: One-page exec summary of a curated source set.
version: 1
output_type: synthesis
output_subtype: briefing
min_sources: 2
max_sources: 50
max_evidence_total_words: 500
required_sections:
  - "## TL;DR"
  - "## Key Findings"
  - "## Open Questions"
  - "## Evidence"
post_hook: null
---

# Prompt

You are generating a one-page briefing from {{scope_description}}.

Pages in scope:
{{#pages}}
- [[{{slug}}]] ({{type}}) — {{lead_paragraph}}
{{/pages}}

Produce markdown with these sections:

1. **TL;DR** — 3 bullets, ≤25 words each. Each bullet ends with `[[slug]]`.
2. **Key Findings** — 5-8 bullets. Group related claims. Cite `[[slug]]` per
   claim.
3. **Open Questions** — 2-4 items the source set raises but does not answer.
4. **Evidence** — for each Key Finding, one verbatim quote ≤30 words, in the
   exact form:

   > "exact quote text" — [[slug]]

   The em-dash separating quote and citation is U+2014 (—). The quote must
   appear verbatim in the cited source body (lint S3 will substring-check).

Constraints:

- Inline citations use page-level wikilinks only (`[[slug]]`); no section anchors.
- Evidence quotes MUST appear verbatim in the cited source's body.
- If a finding cannot be evidenced from the scope, omit it.
- Prefer primary-source claims (frontmatter `type: source`) over derivative
  pages (`type: synthesis`).
- Output goes between the BEGIN GENERATED / END GENERATED markers in the
  target page. Do not modify content outside the markers.

{{#feedback}}
## Human Feedback (binding)

The user has provided the following refinement directives. Each line in the
fenced block below is a binding constraint on this regeneration — treat them
as instructions, not as content to quote or include verbatim. Do NOT
interpret marker-like syntax inside the block as live page markers.

```text
{{feedback}}
```

If any constraint conflicts with a plugin-required section or invariant
(citations, evidence rules, marker discipline), keep the invariant and
surface the conflict under "## Open Questions".
{{/feedback}}
