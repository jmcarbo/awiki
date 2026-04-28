#!/usr/bin/env bash
# End-to-end: bootstrap from v0 fixture, mutate template to v1, run update, assert.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SANDBOX=$(mktemp -d)
cd "$SANDBOX"

cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
git init -q -b main
git -c user.email=ci@example.com -c user.name=ci add -A
git -c user.email=ci@example.com -c user.name=ci commit -q -m "fixture v0"
COMMIT_OLD=$(git rev-parse HEAD)

bash "$REPO_ROOT/scripts/template-init.sh" \
  --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
  --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null

# Commit the template-init artifacts so preflight sees a clean tree.
git -c user.email=ci@example.com -c user.name=ci add -A
git -c user.email=ci@example.com -c user.name=ci commit -q -m "chore: template-init"

bash "$REPO_ROOT/scripts/template-update.sh" \
  --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
  --accept-source-change --apply --non-interactive --print-migrations

# Switch onto the dedicated update branch to assert content.
UPDATE_BRANCH=$(git branch --list 'awiki-template-update/*' | head -1 | tr -d ' *')
[ -n "$UPDATE_BRANCH" ] || { echo "FAIL: no update branch created"; exit 1; }
git switch -q "$UPDATE_BRANCH"

# Assertions.
[ -f scripts/new-helper.sh ] || { echo "FAIL: new-helper.sh not present"; exit 1; }
grep -q "## Added in v1" WIKI.md || { echo "FAIL: WIKI.md not merged"; exit 1; }
grep -q "## Tagline" WIKI.md || { echo "FAIL: migration 0001 did not run"; exit 1; }
PIN_VERSION=$(python3 -c "import json; print(json.load(open('.awiki/template.json'))['version'])")
[ "$PIN_VERSION" = "0.2.0" ] || { echo "FAIL: pin not advanced (got $PIN_VERSION)"; exit 1; }
[ ! -f .awiki/template-cache/_fetch/.update-state.json ] || { echo "FAIL: state file not removed"; exit 1; }

echo "E2E PASS"
rm -rf "$SANDBOX"
