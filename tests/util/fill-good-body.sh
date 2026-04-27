#!/usr/bin/env bash
# Drops a known-good generated region between BEGIN/END markers in the
# given page. Used only by the e2e test so we exercise the full pipeline
# without invoking a real LLM.
set -euo pipefail
PAGE="$1"

python3 - "$PAGE" <<'PY'
import sys, re

path = sys.argv[1]
with open(path) as f:
    text = f.read()

body = """
## TL;DR
- Demo finding 1. [[s-demo-1]]
- Demo finding 2. [[s-demo-2]]
- Demo finding 3. [[s-demo-3]]

## Key Findings
- Demo source 1 covers topic A. [[s-demo-1]]
- Demo source 2 covers topic B. [[s-demo-2]]
- Demo source 3 covers topic C. [[s-demo-3]]
- Sources 1 and 2 overlap on subtopic X. [[s-demo-1]]
- Source 3 raises subtopic Y. [[s-demo-3]]

## Open Questions
- How do these three demo sources relate beyond surface topics?
- What is missing from the corpus?

## Evidence
> "The quick brown fox jumps over the lazy dog" — [[s-demo-1]]
> "verbatim quote text from demo source 2" — [[s-demo-2]]
> "verbatim quote text from demo source 3" — [[s-demo-3]]
"""

text = re.sub(
    r"(<!-- BEGIN GENERATED [^>]*-->)\n[\s\S]*?(<!-- END GENERATED -->)",
    lambda m: m.group(1) + body + m.group(2),
    text,
    count=1,
)
with open(path, "w") as f:
    f.write(text)
PY
