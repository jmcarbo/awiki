#!/usr/bin/env bash
# Alias for: just template-update --rerun-bootstrap-step <id>
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
[[ $# -eq 1 ]] || { echo "usage: template-step.sh <id>" >&2; exit 2; }
exec bash "$SCRIPT_DIR/template-update.sh" --rerun-bootstrap-step "$1"
