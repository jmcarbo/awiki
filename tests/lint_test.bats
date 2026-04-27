#!/usr/bin/env bats

@test "lint detects broken wikilink" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [[ "$output" == *"LINT|ERROR"*"foo.md"*"broken wikilink"* ]]
}

@test "lint detects empty page" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [[ "$output" == *"LINT|WARN"*"empty.md"* ]]
}

@test "lint exits 2 on errors" {
  run bash scripts/lint.sh tests/fixtures/wiki-broken/content
  [ "$status" -eq 2 ]
}

@test "lint exits 0 on clean wiki" {
  CLEAN="$(mktemp -d)/content"
  mkdir -p "$CLEAN/entities"
  cat > "$CLEAN/entities/foo.md" <<EOF2
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Foo references [[bar]] across the wiki for connectivity testing purposes.
EOF2
  cat > "$CLEAN/entities/bar.md" <<EOF2
---
title: "Bar"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Bar references [[foo]] for connectivity testing in the lint suite.
EOF2
  run bash scripts/lint.sh "$CLEAN"
  [ "$status" -eq 0 ]
}
