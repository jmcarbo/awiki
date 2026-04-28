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

@test "transform: rewrites relative md link to wikilink" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  printf '%s' '{"docs/foo.md":"git-x-docs-foo","README.md":"git-x-readme"}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  grep -q "\[\[git-x-docs-foo|foo\]\]" out.md
  grep -q "\[\[git-x-readme|README\]\]" out.md
}

@test "transform: does not rewrite inside fenced code" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  printf '%s' '{"docs/foo.md":"git-x-docs-foo","docs/not-rewritten.md":"git-x-docs-not-rewritten"}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  ! grep -q "git-x-docs-not-rewritten" out.md
  grep -q "fake" out.md
}

@test "transform: rewrites reference-style links" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  printf '%s' '{"docs/foo.md":"git-x-docs-foo"}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  grep -q "git-x-docs-foo" out.md
}

@test "transform: leaves link plain when target slug not in map" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  echo '{}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  grep -q "\[foo\](./foo.md)" out.md
}

@test "transform: copies referenced image to asset dir + rewrites path" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  upstream_root="$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed"
  cp "$upstream_root/docs/intro.md" upstream.md
  echo '{}' | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x" \
    --upstream-root "$upstream_root"
  [ -f "$WORK/_assets/git-x/docs/img/arch.png" ]
  grep -q "_assets/git-x/docs/img/arch.png" out.md
}

@test "transform: warns on missing image ref but exits 0" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  printf -- '---\ntitle: T\n---\n\n# T\n\n![missing](./nope.png)\n' > upstream.md
  echo '{}' | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath README.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x" \
    --upstream-root "$WORK"
  grep -q "missing" out.md
}
