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
