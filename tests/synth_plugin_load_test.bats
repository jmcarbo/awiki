#!/usr/bin/env bats

setup() {
  PLUGINS_DIR="tests/fixtures/synth-plugins-fixture"
  export AWIKI_SYNTH_PLUGINS_DIR="$PLUGINS_DIR"
}

@test "loader parses briefing manifest" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load briefing && echo "$SYNTH_PLUGIN_NAME|$SYNTH_PLUGIN_OUTPUT_TYPE|$SYNTH_PLUGIN_MIN_SOURCES|$SYNTH_PLUGIN_OUTPUT_SUBTYPE"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"briefing|synthesis|2|briefing"* ]]
}

@test "loader exposes required_sections as newline-separated string" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load briefing && printf -- "%s" "$SYNTH_PLUGIN_REQUIRED_SECTIONS"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"## TL;DR"* ]]
  [[ "$output" == *"## Evidence"* ]]
}

@test "loader rejects plugin name with leading hyphen" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load -bad-name'
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid plugin name"* ]]
}

@test "loader rejects manifest missing required_sections" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load missing'
  [ "$status" -ne 0 ]
  [[ "$output" == *"required_sections"* ]]
}

@test "loader rejects directory-form plugins in phase 13" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load dirform'
  [ "$status" -ne 0 ]
  [[ "$output" == *"directory-form plugins not yet supported"* ]]
}

@test "loader supplies output_subtype default = name" {
  TMP="$(mktemp -d)"
  cat > "$TMP/nodefault.md" <<EOF2
---
name: nodefault
description: x
output_type: synthesis
required_sections: ["## A"]
min_sources: 1
---
EOF2
  AWIKI_SYNTH_PLUGINS_DIR="$TMP" run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load nodefault && echo "$SYNTH_PLUGIN_OUTPUT_SUBTYPE"'
  [ "$status" -eq 0 ]
  [[ "$output" == "nodefault" ]]
}

@test "loader rejects unknown plugin" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load nonexistent'
  [ "$status" -ne 0 ]
  [[ "$output" == *"plugin not found"* ]]
}
