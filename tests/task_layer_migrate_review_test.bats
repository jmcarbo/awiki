#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  mkdir -p "$WORK/scripts"
  cp -r "$REPO_ROOT/scripts/lib" "$WORK/scripts/lib"
  cp -r "$REPO_ROOT/scripts/templates" "$WORK/scripts/templates"
  cp "$REPO_ROOT/scripts/task-layer-migrate-review.sh" "$WORK/scripts/"
  chmod +x "$WORK/scripts/task-layer-migrate-review.sh"
  cd "$WORK"
  git init -q
  git config user.email t@t.t
  git config user.name t

  # Seed a phase-16-style WIKI.md with the bracket markers but no
  # Weekly Review subsection.
  cat > WIKI.md <<'WIKI'
# Wiki Conventions

(intro)

<!-- BEGIN task-layer -->

### Inline Action Grammar

(table from phase 16)

<!-- END task-layer -->

## After Block
WIKI
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

@test "migrate-review appends the subsection inside the bracket block" {
  run bash scripts/task-layer-migrate-review.sh
  [ "$status" -eq 0 ]
  run grep -q '^### Weekly Review$' WIKI.md
  [ "$status" -eq 0 ]
  # Subsection precedes <!-- END task-layer -->
  awk '/^### Weekly Review$/ { found=1 } /^<!-- END task-layer -->$/ { exit found ? 0 : 1 }' WIKI.md
  # After-block content preserved.
  run grep -q '^## After Block$' WIKI.md
  [ "$status" -eq 0 ]
}

@test "migrate-review is idempotent" {
  bash scripts/task-layer-migrate-review.sh
  before="$(wc -l < WIKI.md)"
  bash scripts/task-layer-migrate-review.sh
  after="$(wc -l < WIKI.md)"
  [ "$before" -eq "$after" ]
}

@test "migrate-review no-ops when bracket marker is absent" {
  cat > WIKI.md <<'WIKI'
# Wiki Conventions
no markers here
WIKI
  run bash scripts/task-layer-migrate-review.sh
  [ "$status" -eq 0 ]
  run grep -q '^### Weekly Review$' WIKI.md
  [ "$status" -ne 0 ]
}

@test "migrate-review no-ops when WIKI.md missing" {
  rm -f WIKI.md
  run bash scripts/task-layer-migrate-review.sh
  [ "$status" -eq 0 ]
}
