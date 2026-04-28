#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp "$REPO_ROOT/justfile" "$WORK/justfile"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\nAWIKI_STALE_DAYS=90\nAWIKI_LOG_QUERIES=0\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "data-init creates content/datasets/ and data/ directories" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/datasets ]
  [ -d data ]
}

@test "data-init is idempotent on directory creation" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/datasets ]
  [ -d data ]
}

@test "data-init creates .gitkeep in each new directory" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -f content/datasets/.gitkeep ]
  [ -f data/.gitkeep ]
}

@test "data-init restores .gitkeep when dir pre-exists without it" {
  mkdir -p content/datasets data
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -f content/datasets/.gitkeep ]
  [ -f data/.gitkeep ]
}

@test "data-init appends AWIKI_DATA_LAYER=on if absent" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_DATA_LAYER=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init does not duplicate AWIKI_DATA_LAYER on re-run" {
  run bash scripts/data-init.sh
  run bash scripts/data-init.sh
  run grep -c '^AWIKI_DATA_LAYER=' .awiki/config
  [[ "$output" == "1" ]]
}

@test "data-init appends AWIKI_DATASET_INLINE_MAX_ROWS=500 default" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_DATASET_INLINE_MAX_ROWS=500$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init appends AWIKI_DATASET_INLINE_MAX_BYTES=51200 default" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_DATASET_INLINE_MAX_BYTES=51200$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init preserves user-set AWIKI_DATASET_INLINE_MAX_ROWS" {
  echo 'AWIKI_DATASET_INLINE_MAX_ROWS=1000' >> .awiki/config
  run bash scripts/data-init.sh
  run grep -c '^AWIKI_DATASET_INLINE_MAX_ROWS=' .awiki/config
  [[ "$output" == "1" ]]
  run grep -E '^AWIKI_DATASET_INLINE_MAX_ROWS=1000$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init inserts data-layer block in WIKI.md" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run grep -F '<!-- BEGIN data-layer -->' WIKI.md
  [ "$status" -eq 0 ]
  run grep -F '<!-- END data-layer -->' WIKI.md
  [ "$status" -eq 0 ]
  run grep -F 'Page kinds' WIKI.md
  [ "$status" -eq 0 ]
}

@test "data-init does not duplicate data-layer block on re-run" {
  bash scripts/data-init.sh
  bash scripts/data-init.sh
  run grep -c -F '<!-- BEGIN data-layer -->' WIKI.md
  [[ "$output" == "1" ]]
}

@test "data-init refreshes data-layer block on re-run when template changes" {
  bash scripts/data-init.sh
  # Mutate the inserted block; re-run must restore from template.
  sed -i.bak 's/Page kinds/Page kinds: MUTATED/' WIKI.md
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  run grep -F 'Page kinds' WIKI.md
  [ "$status" -eq 0 ]
  run grep -c -F 'Page kinds: MUTATED' WIKI.md
  [[ "$output" == "0" ]]
}

@test "data-init is byte-idempotent on WIKI.md re-runs" {
  bash scripts/data-init.sh
  cp WIKI.md WIKI.md.snapshot
  bash scripts/data-init.sh
  bash scripts/data-init.sh
  run cmp -s WIKI.md WIKI.md.snapshot
  [ "$status" -eq 0 ]
}

@test "data-init refuses to patch WIKI.md with BEGIN but no END marker" {
  # Seed a malformed WIKI.md: BEGIN marker, then user content, no END.
  printf -- '%s\n' '<!-- BEGIN data-layer -->' 'user content that must survive' > WIKI.md
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  # User content must still be present.
  run grep -F 'user content that must survive' WIKI.md
  [ "$status" -eq 0 ]
  # BEGIN marker still exactly once.
  run grep -c -F '<!-- BEGIN data-layer -->' WIKI.md
  [[ "$output" == "1" ]]
  # Block was not appended again.
  run grep -c -F 'Page kind: `dataset`' WIKI.md
  [[ "$output" == "0" ]]
}

@test "just data-init recipe runs the script" {
  run just data-init
  [ "$status" -eq 0 ]
  [ -d content/datasets ]
  [ -d data ]
  run grep -E '^AWIKI_DATA_LAYER=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init creates content/charts/ and assets/charts/ and static/vendor/vega/" {
  run bash scripts/data-init.sh
  [ "$status" -eq 0 ]
  [ -d content/charts ]
  [ -d assets/charts ]
  [ -d static/vendor/vega ]
}

@test "data-init appends AWIKI_CHART_OBSIDIAN_PREVIEW=on default" {
  run bash scripts/data-init.sh
  run grep -E '^AWIKI_CHART_OBSIDIAN_PREVIEW=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "data-init extends git-crypt patterns when .gitattributes has git-crypt section" {
  printf '*.secret filter=git-crypt diff=git-crypt\n' > .gitattributes
  run bash scripts/data-init.sh <<<"y"
  run grep -F 'data/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  run grep -F 'assets/charts/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
}

@test "data-init does not duplicate git-crypt patterns on re-run" {
  printf '*.secret filter=git-crypt diff=git-crypt\n' > .gitattributes
  bash scripts/data-init.sh <<<"y"
  bash scripts/data-init.sh <<<"y"
  run grep -c -F 'data/private/** filter=git-crypt diff=git-crypt' .gitattributes
  [[ "$output" == "1" ]]
}

@test "data-init skips git-crypt extension when no git-crypt section" {
  printf '*.txt text\n' > .gitattributes
  run bash scripts/data-init.sh
  run grep -F 'data/private/**' .gitattributes
  [ "$status" -ne 0 ]
}
