#!/usr/bin/env bash
# Schema upgrade: 1 → 2.
# Bumps .awiki/template.json.schema_version in the BOOTSTRAPPED repo
# (path is provided via AWIKI_REPO_ROOT, the orchestrator-supplied env var).
set -euo pipefail

REPO="${AWIKI_REPO_ROOT:?AWIKI_REPO_ROOT required}"
PJ="$REPO/.awiki/template.json"

# Bump schema_version in template.json (whitelisted under --schema-upgrade).
python3 -c "
import json
d = json.load(open('$PJ'))
d['schema_version'] = 2
json.dump(d, open('$PJ', 'w'), indent=2)
open('$PJ', 'a').write('\n')
"
echo "schema upgraded 1 → 2"
