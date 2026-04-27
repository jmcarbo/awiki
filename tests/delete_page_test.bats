#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities" "$WORK/scripts"
  cp "$REPO_ROOT/scripts/log-append.sh" "$WORK/scripts/log-append.sh"
  cat > "$WORK/content/entities/foo.md" <<E
---
title: "Foo"
type: entity
---

Body.
E
  cat > "$WORK/content/entities/bar.md" <<E
---
title: "Bar"
type: entity
---

References [[foo]].
E
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  cd "$WORK"
}
teardown() { rm -rf "$WORK"; }

@test "delete-page removes file" {
  bash "$REPO_ROOT/scripts/delete-page.sh" foo
  [ ! -f content/entities/foo.md ]
}

@test "delete-page marks wikilinks as broken" {
  bash "$REPO_ROOT/scripts/delete-page.sh" foo
  run grep -F 'broken: was [[foo]]' content/entities/bar.md
  [ "$status" -eq 0 ]
}

@test "delete-page rejects unknown slug" {
  run bash "$REPO_ROOT/scripts/delete-page.sh" nonexistent
  [ "$status" -eq 2 ]
}
