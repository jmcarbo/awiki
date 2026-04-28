#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "cache-rotate: keeps current + previous; deletes older" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa .awiki/template-cache/bbb .awiki/template-cache/ccc
  # Make ccc oldest, aaa newest by mtime.
  touch -t 202001011200 .awiki/template-cache/ccc
  touch -t 202101011200 .awiki/template-cache/bbb
  touch -t 202201011200 .awiki/template-cache/aaa
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/aaa ]
  [ -d .awiki/template-cache/bbb ]
  [ ! -d .awiki/template-cache/ccc ]
}

@test "cache-rotate: ignores non-sha entries (_fetch, _check-stamp)" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa .awiki/template-cache/_fetch
  touch .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/_fetch ]
  [ -f .awiki/template-cache/_check-stamp ]
}

@test "cache-rotate: with only current, no-op" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/aaa
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/aaa ]
}

@test "cache-rotate: missing cache-dir is ok" {
  cd "$TMP"
  run python3 "$REPO_ROOT/scripts/_template_helpers/cache_rotate.py" \
    --cache-dir .awiki/template-cache --current aaa
  [ "$status" -eq 0 ]
}

@test "template-update --apply: Commit D writes template.json once + rotates cache" {
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT_OLD=$(git rev-parse HEAD)
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null
  git add .awiki
  git -c user.email=a@b -c user.name=t commit -q -m bootstrap
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --non-interactive
  [ "$status" -eq 0 ] || { echo "STATUS=$status"; echo "$output"; false; }
  # Pin advanced.
  PIN_VERSION=$(python3 -c "import json; print(json.load(open('.awiki/template.json'))['version'])")
  [ "$PIN_VERSION" = "0.2.0" ]
  # applied_migrations updated.
  python3 -c "import json; d=json.load(open('.awiki/template.json')); ids=[m['id'] for m in d['applied_migrations']]; assert '0001-add-tagline' in ids, ids"
  # State file removed.
  [ ! -f .awiki/template-cache/_fetch/.update-state.json ]
  [ ! -d .awiki/template-cache/_fetch ]
  # Cache rotated: only current + 1 previous (between 1 and 2 SHA dirs).
  COUNT=$(ls .awiki/template-cache/ | grep -v '^_' | wc -l | tr -d ' ')
  [[ "$COUNT" =~ ^[12]$ ]]
  # Last commit is provenance pin.
  git log --format=%s -1 | grep -q "pin to"
}

@test "template-update --apply --persist-source: updates repo, original_repo unchanged" {
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  git add .awiki
  git -c user.email=a@b -c user.name=t commit -q -m bootstrap
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --persist-source --non-interactive
  [ "$status" -eq 0 ] || { echo "STATUS=$status"; echo "$output"; false; }
  REPO_URL=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json repo)
  [ "$REPO_URL" = "$V1" ]
  ORIG=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json original_repo)
  [ "$ORIG" = "$REPO_ROOT/tests/fixtures/template-update/v0" ]
}
