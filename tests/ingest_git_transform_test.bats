#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "transform: strips upstream frontmatter and emits awiki frontmatter" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/README.md" upstream.md
  echo '{}' | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath README.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  [ -f out.md ]
  head -1 out.md | grep -q '^---'
  grep -q "^type: source" out.md
  grep -q "^provenance: git" out.md
  grep -q "^git_repo: x" out.md
  grep -q "^git_path: README.md" out.md
  ! grep -q "^foo: bar" out.md
  grep -q "Upstream Title" out.md
}
