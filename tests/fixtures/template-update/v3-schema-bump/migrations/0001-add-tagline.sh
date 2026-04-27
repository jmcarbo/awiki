#!/usr/bin/env bash
# migration: 0001-add-tagline
# requires: agent=false
# touches: WIKI.md
# idempotent: yes
set -euo pipefail
if [[ "${1:-}" == "--dry-run" ]]; then
  echo "would append tagline to WIKI.md"
  exit 0
fi
grep -q "## Tagline" WIKI.md || echo -e "\n## Tagline\n" >> WIKI.md
