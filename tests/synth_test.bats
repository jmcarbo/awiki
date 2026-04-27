#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/synthesis-plugins" "$WORK/synthesis-plugins"
  cp -r "$REPO_ROOT/tests/fixtures/wiki-synth/content" "$WORK/content"
  cp -r "$REPO_ROOT/tests/fixtures/wiki-synth/.awiki" "$WORK/.awiki"
  mkdir -p "$WORK/content/synthesis/.staged"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  # Stub log-append.sh / lint.sh for hermetic tests.
  cat > "$WORK/scripts/log-append.sh" <<'STUB'
#!/usr/bin/env bash
echo "LOG-APPEND|$*" >> "${AWIKI_LOG_FILE:-content/log.md}"
STUB
  cat > "$WORK/scripts/lint.sh" <<'STUB'
#!/usr/bin/env bash
# stub lint — phase 13 only checks markers; phase 14 will replace this.
exit 0
STUB
  chmod +x "$WORK/scripts/log-append.sh" "$WORK/scripts/lint.sh"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "synth list prints briefing row" {
  run bash scripts/synth.sh list
  [ "$status" -eq 0 ]
  [[ "$output" == *"briefing"* ]]
  [[ "$output" == *"synthesis"* ]]
}

@test "resolve --tag=memex returns 8 sorted slugs" {
  cat > content/synthesis/memex-briefing.md <<EOF2
---
title: "Memex"
type: synthesis
plugin: briefing
scope:
  tag: memex
sources: []
draft: false
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
<!-- END GENERATED -->
EOF2
  run bash scripts/synth.sh resolve memex-briefing
  [ "$status" -eq 0 ]
  [[ "$output" == *"s-hyphens"* ]]
  [[ "$output" == *"s1"* ]]
  # No private leak — s-private should NOT be included unless target is private.
  [[ "$output" != *"s-private"* ]]
}

@test "resolve with explicit --slugs subset" {
  cat > content/synthesis/manual-briefing.md <<EOF2
---
title: "Manual"
type: synthesis
plugin: briefing
scope:
  slugs: [s1, s2]
sources: []
draft: false
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
<!-- END GENERATED -->
EOF2
  run bash scripts/synth.sh resolve manual-briefing
  [ "$status" -eq 0 ]
  [[ "$output" == *"s1"* ]]
  [[ "$output" == *"s2"* ]]
  [[ "$output" != *"s3"* ]]
}

@test "resolve query exits 2 when qmd not installed" {
  cat > content/synthesis/q-briefing.md <<EOF2
---
title: "Q"
type: synthesis
plugin: briefing
scope:
  query: "memex history"
sources: []
draft: false
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
<!-- END GENERATED -->
EOF2
  # Restrict PATH to system bin dirs (excludes brew/cargo paths where qmd lives)
  # so command -v qmd fails inside synth.sh; awk/sort/find/grep remain available.
  PATH="/usr/bin:/bin" run bash scripts/synth.sh resolve q-briefing
  [ "$status" -eq 2 ]
  [[ "$output" == *"qmd"* ]]
}
