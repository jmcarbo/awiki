// Drops a known-good generated region between BEGIN/END markers in the
// given page. Used by MCP server tests to exercise finalize() without
// invoking a real LLM. The body uses fixture sources s1/s2/s3 from
// tests/fixtures/wiki-synth/ and quotes verbatim text that S3 will accept.

import { readFileSync, writeFileSync } from "node:fs";

export function fillGoodBody(pagePath) {
  const text = readFileSync(pagePath, "utf8");
  const body = `
## TL;DR
- Bush proposed the memex in 1945. [[s1]]
- Engelbart augmented intellect. [[s2]]
- Nelson coined hypertext. [[s3]]

## Key Findings
- The memex was framed as a desk-sized analog device. [[s1]]
- Engelbart built the first practical hypertext system. [[s2]]
- Nelson introduced project Xanadu. [[s3]]
- Three early figures shaped associative-trail thinking. [[s1]]
- Bush's essay anticipated linked documents. [[s1]]

## Open Questions
- How do these three figures relate beyond surface topics?
- What did Xanadu attempt that the modern web does not?

## Evidence
> "A 1945 essay by Vannevar Bush proposing the memex" — [[s1]]

`;

  const replaced = text.replace(
    /(<!-- BEGIN GENERATED [^>]*-->)\n[\s\S]*?(<!-- END GENERATED -->)/,
    (m, begin, end) => `${begin}\n${body}${end}`,
  );
  writeFileSync(pagePath, replaced);
}

if (import.meta.url === `file://${process.argv[1]}`) {
  if (process.argv.length < 3) {
    console.error("usage: fill-good-body.mjs <page-path>");
    process.exit(1);
  }
  fillGoodBody(process.argv[2]);
}
