#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities"
  cat > "$WORK/content/catalog.md" <<E
---
title: "Catalog"
type: catalog
---

# Catalog
E
  cat > "$WORK/content/entities/foo.md" <<E
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

Body referencing [[foo]] for connectivity, sufficient length.
E
  cd "$WORK"
}
teardown() { rm -rf "$WORK"; }

@test "update-catalog inserts entity entry" {
  bash "$BATS_TEST_DIRNAME/../scripts/update-catalog.sh"
  run grep -F '[[foo]]' content/catalog.md
  [ "$status" -eq 0 ]
}
