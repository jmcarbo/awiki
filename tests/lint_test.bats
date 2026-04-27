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

@test "lint --fix adds missing last_updated" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/needs-fix.md" <<EOF2
---
title: "Needs Fix"
date: 2026-01-01
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body referencing [[needs-fix]] for self-connectivity, sufficient length.
EOF2
  bash scripts/lint.sh --fix "$TMP" || true
  run grep '^last_updated:' "$TMP/entities/needs-fix.md"
  [ "$status" -eq 0 ]
}

@test "lint warns on tags: [private] outside private/ path" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/leaky.md" <<E
---
title: "Leaky"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: [private]
aliases: []
sources: []
draft: false
---

Body content sufficient length for non-empty check.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|WARN"*"leaky.md"*"private tag outside private path"* ]]
}

@test "lint --fix is a no-op for files lacking date: field" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  PAGE="$TMP/entities/no-date.md"
  cat > "$PAGE" <<EOF2
---
title: "No Date"
type: entity
tags: []
aliases: []
sources: []
draft: false
---

Body referencing [[no-date]] for self-connectivity, sufficient length.
EOF2
  # Force mtime into the past so we can detect any modification.
  touch -t 200001010000 "$PAGE"
  MTIME_BEFORE="$(stat -f %m "$PAGE" 2>/dev/null || stat -c %Y "$PAGE")"
  run bash scripts/lint.sh --fix "$TMP"
  # No FIX line should be emitted for this file.
  [[ "$output" != *"FIX|"*"no-date.md"* ]]
  MTIME_AFTER="$(stat -f %m "$PAGE" 2>/dev/null || stat -c %Y "$PAGE")"
  [ "$MTIME_BEFORE" = "$MTIME_AFTER" ]
}
