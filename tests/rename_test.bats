#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  WORK="$(mktemp -d)/repo"
  mkdir -p "$WORK/content/entities" "$WORK/scripts"
  # rename.sh shells `bash scripts/log-append.sh ...` from cwd; mirror that script.
  cp "$REPO_ROOT/scripts/log-append.sh" "$WORK/scripts/log-append.sh"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
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
  cd "$WORK"
}

teardown() { rm -rf "$WORK"; }

@test "rename moves file and updates wikilinks" {
  bash "$REPO_ROOT/scripts/rename.sh" foo foo-renamed
  [ -f content/entities/foo-renamed.md ]
  [ ! -f content/entities/foo.md ]
  run grep -F '[[foo-renamed]]' content/entities/bar.md
  [ "$status" -eq 0 ]
}
