#!/usr/bin/env bats

setup() {
  TMP="$(mktemp -d)"
  cat > "$TMP/source.md" <<'SRC'
---
title: "Sample"
type: source
---

Knowledge work in 1945 hinged on a researcher's ability to traverse
their own associative trails through stored sources.

The memex would compress decades of literature into a desk-sized device.
SRC

  cat > "$TMP/quote.txt" <<'QUOTE'
knowledge work in 1945 hinged on a researchers ability
QUOTE
}

teardown() { rm -rf "$TMP"; }

@test "fuzzy helper finds closest paragraph" {
  run python3 scripts/lint-synth-fuzzy.py "$TMP/source.md" "$TMP/quote.txt"
  [ "$status" -eq 0 ]
  [[ "$output" == *"associative trails"* ]] || [[ "$output" == *"researcher"* ]]
}

@test "fuzzy helper exits 0 with empty stdout when no close match" {
  printf "completely unrelated alien text xyzzy\n" > "$TMP/quote.txt"
  run python3 scripts/lint-synth-fuzzy.py "$TMP/source.md" "$TMP/quote.txt"
  [ "$status" -eq 0 ]
}

@test "fuzzy helper rejects missing source file" {
  run python3 scripts/lint-synth-fuzzy.py "$TMP/nope.md" "$TMP/quote.txt"
  [ "$status" -ne 0 ]
}

@test "fuzzy helper does not interpret quote content as code" {
  # Quote contains characters that would be hostile in a python -c context.
  printf '"; import os; os.system("touch /tmp/awiki-pwn-%s") #' "$$" > "$TMP/quote.txt"
  rm -f /tmp/awiki-pwn-$$
  run python3 scripts/lint-synth-fuzzy.py "$TMP/source.md" "$TMP/quote.txt"
  [ "$status" -eq 0 ]
  [ ! -f /tmp/awiki-pwn-$$ ]
}
