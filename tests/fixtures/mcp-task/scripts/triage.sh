#!/usr/bin/env bash
# Stub triage.sh for BATS smoke. Sleeps optionally to simulate lock-held work,
# then prints the TRIAGE-RESULT|<json> trailer the MCP server expects.
set -euo pipefail
sleep "${AWIKI_TRIAGE_SLEEP:-0}"
id="$1"; shift
outcome="$1"; shift
printf 'TRIAGE-RESULT|{"actions_taken":["%s"],"created_pages":[],"updated_pages":["content/inbox.md"]}\n' "$outcome"
