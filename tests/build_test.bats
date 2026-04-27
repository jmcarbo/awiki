#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/content/entities" "$WORK/.awiki/maps"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: []
aliases: [Effoh]
sources: []
draft: false
---

Sees [[bar]].
E
  cat > "$WORK/content/entities/bar.md" <<E
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

Sees [[foo]] and [[Effoh|alias-display]].
E
  export AWIKI_REPO_ROOT="$WORK"
  cd "$WORK"
}

teardown() {
  rm -rf "$WORK"
}

@test "build emits slug map" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh" --maps-only
  run cat .awiki/maps/slug-to-path.tsv
  [[ "$output" == *"foo	entities/foo.md"* ]]
  [[ "$output" == *"bar	entities/bar.md"* ]]
}

@test "build rewrites wikilinks in build-content" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/foo.md
  [[ "$output" == *"[Bar](/entities/bar/)"* ]]
}

@test "build resolves alias wikilink" {
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/bar.md
  [[ "$output" == *"[alias-display](/entities/foo/)"* ]]
}

@test "alias list with quoted comma is not split" {
  cat > "$WORK/content/entities/qux.md" <<E
---
title: "Qux"
aliases: ["A", "B, with comma"]
draft: false
---

body
E
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh" --maps-only
  run cat .awiki/maps/alias-to-slug.tsv
  [[ "$output" == *"A	qux"* ]]
  [[ "$output" == *"B, with comma	qux"* ]]
  # Must not produce a bogus 'with comma' or '"B' fragment.
  [[ "$output" != *"with comma	qux"* || "$output" == *"B, with comma	qux"* ]]
  # Three lines of qux would mean we mis-split: count 'qux' lines.
  qux_count=$(grep -c '	qux$' .awiki/maps/alias-to-slug.tsv || true)
  [[ "$qux_count" -eq 2 ]]
}

@test "single-quoted title is stripped" {
  cat > "$WORK/content/entities/sq.md" <<E
---
title: 'Single Quoted'
aliases: []
draft: false
---

body
E
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh" --maps-only
  run cat .awiki/maps/slug-to-title.tsv
  [[ "$output" == *"sq	Single Quoted"* ]]
  [[ "$output" != *"'Single Quoted'"* ]]
}

@test "code block contents preserved (wikilinks not rewritten inside fenced code)" {
  cat > "$WORK/content/entities/code.md" <<'E'
---
title: "Code"
aliases: []
draft: false
---

Outside [[bar]] gets rewritten.

```
Inside fenced [[bar]] stays literal.
```

Inline `[[bar]]` also stays literal.
E
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh"
  run cat .awiki/build-content/entities/code.md
  [[ "$output" == *"Outside [Bar](/entities/bar/) gets rewritten."* ]]
  [[ "$output" == *"Inside fenced [[bar]] stays literal."* ]]
  [[ "$output" == *'Inline `[[bar]]` also stays literal.'* ]]
}

@test "_index.md is skipped from slug map" {
  mkdir -p "$WORK/content/section"
  cat > "$WORK/content/section/_index.md" <<E
---
title: "Section"
draft: false
---

intro
E
  bash "$BATS_TEST_DIRNAME/../scripts/build.sh" --maps-only
  run cat .awiki/maps/slug-to-path.tsv
  [[ "$output" != *"_index"* ]]
  run cat .awiki/maps/slug-to-title.tsv
  [[ "$output" != *"_index	"* ]]
}
