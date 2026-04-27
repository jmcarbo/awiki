#!/usr/bin/env bash
# Stub capture.sh for BATS smoke. Reads sanitized text on stdin (the MCP
# server passes AWIKI_CAPTURE_PRESANITIZED=1) and appends a timestamped line
# to content/inbox.md. No flock here — the real script takes its own lock,
# but the MCP boundary does NOT double-lock capture.
set -euo pipefail
text="$(cat)"
printf -- '- 2026-04-27T12:00:00Z %s\n' "$text" >> content/inbox.md
