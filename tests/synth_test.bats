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
