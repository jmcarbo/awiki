#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
  mkdir -p .awiki content/sources content/entities raw/_git-cache
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > content/log.md
  export AWIKI_REPO_ROOT="$WORK"
  # Build a tiny local fixture repo
  seed="$WORK/seed"
  mkdir -p "$seed"
  printf "# Hello\n\nbody\n" > "$seed/README.md"
  bash "$BATS_TEST_DIRNAME/util/build-git-fixture.sh" "$seed" "$WORK/repo"
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "ingest-git: rejects missing arg" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh"
  [ "$status" -eq 1 ]
}

@test "ingest-git: --dry-run on local fixture exits 0 with plan output" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "PLAN|"
  echo "$output" | grep -q "repo_key=local-repo"
}

@test "ingest-git: --dry-run reports added=1 on first run with single README" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "added=1"
  echo "$output" | grep -q "modified=0"
  echo "$output" | grep -q "removed=0"
}

@test "ingest-git: --paths=docs/ excludes README" {
  mkdir -p "$WORK/repo/docs"
  printf "# Doc\n\nbody\n" > "$WORK/repo/docs/intro.md"
  git -C "$WORK/repo" add -A && git -C "$WORK/repo" commit -q -m more
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run --paths=docs/
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "added=1"
}

@test "ingest-git: full run writes one source page from README" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ "$status" -eq 0 ]
  [ -f content/sources/git-repo-readme.md ]
  grep -q "^type: source" content/sources/git-repo-readme.md
  grep -q "^git_repo: repo" content/sources/git-repo-readme.md
}

@test "ingest-git: full run honors --private routing" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  mkdir -p content/private/sources content/private/entities
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --private
  [ "$status" -eq 0 ]
  [ -f content/private/sources/git-repo-readme.md ]
  ! [ -f content/sources/git-repo-readme.md ]
  grep -q "private" content/private/sources/git-repo-readme.md
}

@test "ingest-git: writes repo entity page with sources list" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ -f content/entities/repo-repo.md ]
  grep -q "^type: entity" content/entities/repo-repo.md
  grep -q "^git_url:" content/entities/repo-repo.md
  grep -q "^git_sha:" content/entities/repo-repo.md
  grep -q "\[\[git-repo-readme\]\]" content/entities/repo-repo.md
}

@test "ingest-git: removes derived page when upstream file deleted" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  mkdir -p "$WORK/repo/docs"
  printf "# Doc 1\n\nbody text\n" > "$WORK/repo/docs/d1.md"
  printf "# Doc 2\n\nbody text\n" > "$WORK/repo/docs/d2.md"
  git -C "$WORK/repo" add -A && git -C "$WORK/repo" commit -q -m two
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ -f content/sources/git-repo-docs-d1.md ]
  [ -f content/sources/git-repo-docs-d2.md ]
  git -C "$WORK/repo" rm -q docs/d2.md
  git -C "$WORK/repo" commit -q -m rm
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  ! [ -f content/sources/git-repo-docs-d2.md ]
  [ -f raw/_originals/git/local-repo/git-repo-docs-d2.md ]
}

@test "ingest-git: writes state JSON with file map after run" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ -f .awiki/git-state/local-repo.json ]
  python3 -c "
import json, sys
o = json.load(open('.awiki/git-state/local-repo.json'))
assert o['schema'] == 1
assert o['repo_key'] == 'local-repo'
assert 'README.md' in o['files']
"
}

@test "ingest-git: rerun with no changes is no-op (state head_sha unchanged)" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  before_sha="$(python3 -c "import json; print(json.load(open('.awiki/git-state/local-repo.json'))['head_sha'])")"
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  after_sha="$(python3 -c "import json; print(json.load(open('.awiki/git-state/local-repo.json'))['head_sha'])")"
  [ "$before_sha" = "$after_sha" ]
}
