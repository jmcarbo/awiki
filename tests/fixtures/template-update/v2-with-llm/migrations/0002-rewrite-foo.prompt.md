---
id: 0002-rewrite-foo
requires: [agent]
scope_glob: "content/**/*.md"
risk: medium
---

Rewrite each `## Foo` heading to `## Foo (renamed)`. After each file, run `just lint`.
