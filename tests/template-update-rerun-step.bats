#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  echo ".awiki/template-cache/" > .gitignore
  git add .gitignore .awiki/template.json
  git -c user.email=a@b -c user.name=t commit -q -m "post-init"
}
teardown() { rm -rf "$TMP"; }

@test "--rerun-bootstrap-step: refuses if state file present" {
  cd "$TMP"
  mkdir -p .awiki/template-cache/_fetch
  echo '{}' > .awiki/template-cache/_fetch/.update-state.json
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "in.progress|update branch"
}

@test "--rerun-bootstrap-step: refuses if pending prompts" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo x > .awiki/pending-prompts/0001-x.md
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "pending"
}

@test "--rerun-bootstrap-step: refuses if any awiki-template-update branch exists" {
  cd "$TMP"
  git branch awiki-template-update/leftover
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain --non-interactive
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "update branch|in.progress"
}

@test "--rerun-bootstrap-step: creates rerun branch and updates content_hash" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --rerun-bootstrap-step domain --non-interactive
  [ "$status" -eq 0 ] || { echo "STATUS=$status"; echo "$output"; false; }
  CUR=$(git rev-parse --abbrev-ref HEAD)
  [[ "$CUR" =~ ^awiki-template-update/rerun-domain- ]]
  # template.json updated for the step.
  python3 -c "
import json
d = json.load(open('.awiki/template.json'))
m = [s for s in d.get('bootstrap_steps_done', []) if s.get('id') == 'domain']
assert m and m[0].get('status') == 'applied' and m[0].get('content_hash','').startswith('sha256:'), m
"
}
