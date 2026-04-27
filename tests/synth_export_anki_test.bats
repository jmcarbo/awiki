#!/usr/bin/env bats

setup() {
  REPO_TOP="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  export TMP REPO_TOP
  mkdir -p "$TMP/content/synthesis" "$TMP/scripts" "$TMP/.awiki/exports"
  cp "$REPO_TOP/scripts/synth-export-anki.sh" "$TMP/scripts/"
  cp "$REPO_TOP/scripts/synth-export-anki.py" "$TMP/scripts/"
  chmod +x "$TMP/scripts/synth-export-anki.sh" "$TMP/scripts/synth-export-anki.py"
  cat > "$TMP/content/synthesis/textbook-study-guide.md" <<'MD'
---
title: "Textbook Study Guide"
date: 2026-04-27
last_updated: 2026-04-27
last_generated: 2026-04-27T00:00:00Z
type: synthesis
plugin: study-guide
scope:
  slugs: [s-textbook-ch1]
tags: []
aliases: []
sources: []
draft: false
---

Lead.

<!-- BEGIN GENERATED plugin=study-guide scope_hash=abc123 -->

## Concept Checklist
- [ ] [[s-textbook-ch1]] photosynthesis basics

## Short-Answer Questions
- Q1 [[s-textbook-ch1]]

## Flashcards

Q: What is photosynthesis?
A: The conversion of light energy into chemical energy in plants.

---

Q: What is the byproduct of photosynthesis?
A: Oxygen.

## Suggested Deep-Dives
- [[s-textbook-ch1]]

## Evidence
> "Photosynthesis converts light into chemical energy" — [[s-textbook-ch1]]

<!-- END GENERATED -->
MD
}

teardown() {
  rm -rf "$TMP"
}

@test "synth-export-anki.sh produces a parseable .apkg when genanki is present" {
  if ! python3 -c 'import genanki' 2>/dev/null; then
    skip "genanki not installed; install via: pip install genanki"
  fi
  cd "$TMP"
  run bash scripts/synth-export-anki.sh content/synthesis/textbook-study-guide.md
  [ "$status" -eq 0 ]
  [ -f .awiki/exports/textbook-study-guide.apkg ]
  # An .apkg is a zip; confirm it unzips and contains collection.anki2.
  unzip -l .awiki/exports/textbook-study-guide.apkg | grep -q collection.anki2
}

@test "synth-export-anki.sh exits 0 with stderr note when genanki absent" {
  if python3 -c 'import genanki' 2>/dev/null; then
    skip "genanki IS installed; cannot test absence"
  fi
  cd "$TMP"
  run bash scripts/synth-export-anki.sh content/synthesis/textbook-study-guide.md
  [ "$status" -eq 0 ]
  [[ "$stderr" == *"genanki not installed"* ]] || [[ "$output" == *"genanki not installed"* ]]
}
