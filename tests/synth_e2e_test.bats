#!/usr/bin/env bats

# End-to-end synthesis smoke test:
#   - drop 3 sources tagged `demo` directly under content/sources/ (the
#     ingest.sh path moves files but the agent normally writes the source
#     pages; we simulate the post-agent state),
#   - scaffold a briefing via synth.sh new,
#   - stub-fill the BEGIN..END region with a known-good body,
#   - finalize, lint, and (when hugo is present) render to memory,
#   - confirm scripts/update-catalog.sh surfaces the synthesis page.

setup() {
  REPO_TOP="$(git rev-parse --show-toplevel)"
  TMP="$(mktemp -d)"
  export TMP REPO_TOP
  cp -r "$REPO_TOP/scripts" "$TMP/scripts"
  cp -r "$REPO_TOP/synthesis-plugins" "$TMP/synthesis-plugins"
  cp "$REPO_TOP/justfile" "$TMP/justfile"
  cp "$REPO_TOP/WIKI.md" "$TMP/WIKI.md"
  cp "$REPO_TOP/template.manifest.toml" "$TMP/template.manifest.toml"
  cp "$REPO_TOP/hugo.toml" "$TMP/hugo.toml" 2>/dev/null || true
  mkdir -p "$TMP/content/sources" "$TMP/content/synthesis" "$TMP/raw/inbox/interactive" "$TMP/.awiki"
  # log placeholder (log-append.sh appends to content/log.md).
  cat > "$TMP/content/log.md" <<'L'
---
title: Log
type: log
draft: true
---

L
  # Catalog seed — update-catalog.sh refreshes in place.
  cat > "$TMP/content/catalog.md" <<'C'
---
title: Catalog
type: catalog
draft: false
---

# Catalog
C
  pushd "$TMP" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$TMP"
}

@test "e2e: 3 sources, synth briefing, stub-fill, finalize, lint, catalog" {
  # Drop 3 sources tagged `demo` directly into content/sources/.
  for i in 1 2 3; do
    cat > "content/sources/s-demo-$i.md" <<EOF
---
title: "Demo Source $i"
date: 2026-04-27
last_updated: 2026-04-27
type: source
tags: [demo]
aliases: []
sources: []
draft: false
---

Lead paragraph for demo source $i.

The quick brown fox jumps over the lazy dog. This is verbatim quote text from demo source $i.
EOF
  done

  # Scaffold the briefing.
  # NOTE: --slugs is used (not --tag) so the synthesis page's own auto-applied
  # `tags: [demo, briefing]` doesn't drift the scope hash between scaffold and
  # finalize. The same slugs list is what a tag-scoped scaffold would resolve
  # to anyway (just demo-tagged sources). This is a known scaffold-tagging
  # quirk, deliberately worked-around in this e2e test rather than papered
  # over in synth.sh.
  run bash scripts/synth.sh new briefing demo --slugs=s-demo-1,s-demo-2,s-demo-3
  [ "$status" -eq 0 ]
  [ -f "content/synthesis/demo-briefing.md" ]

  # Stub-fill the body with a known-good generated region.
  bash "$REPO_TOP/tests/util/fill-good-body.sh" content/synthesis/demo-briefing.md

  # Finalize.
  run bash scripts/synth.sh finalize demo-briefing
  [ "$status" -eq 0 ]

  # Lint clean (or only S7 info bullets).
  run bash scripts/lint.sh
  # Allow exit 0 (errors=0,warnings=0) or exit 1 (warnings only — orphan info etc).
  [[ "$status" -eq 0 || "$status" -eq 1 ]]
  [[ "$output" != *"LINT|ERROR"* ]]

  # hugo --renderToMemory clean (skip if hugo absent; fixture lacks themes).
  if command -v hugo >/dev/null 2>&1 && [[ -d themes ]]; then
    run hugo --renderToMemory --quiet
    [ "$status" -eq 0 ]
  fi

  # Catalog contains the synthesis entry.
  run bash scripts/update-catalog.sh
  [ "$status" -eq 0 ]
  grep -q "demo-briefing" content/catalog.md
}
