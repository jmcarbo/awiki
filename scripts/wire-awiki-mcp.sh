#!/usr/bin/env bash
set -euo pipefail

if [[ ! -f mcp/awiki-server/index.js ]]; then
  echo "MCP server not built. Run: cd mcp/awiki-server && npm install" >&2
  exit 1
fi

TARGET=""
if [[ -d .claude || -f CLAUDE.md ]]; then TARGET="claude"; fi
if [[ -d .codex ]]; then TARGET="codex"; fi

case "$TARGET" in
  claude)
    CONFIG="./.mcp.json"
    [[ -f "$CONFIG" ]] || echo '{"mcpServers": {}}' > "$CONFIG"
    python3 - "$CONFIG" <<'PY'
import json, sys, os
path = sys.argv[1]
data = json.load(open(path))
data.setdefault("mcpServers", {})["awiki"] = {
    "command": "node",
    "args": [os.path.abspath("mcp/awiki-server/index.js")]
}
json.dump(data, open(path, "w"), indent=2)
PY
    echo "WIRED|claude|$CONFIG"
    ;;
  codex)
    CONFIG=".codex/config.toml"
    mkdir -p .codex
    if ! grep -q '^\[mcp.servers.awiki\]' "$CONFIG" 2>/dev/null; then
      ABS_PATH="$(cd mcp/awiki-server && pwd)/index.js"
      cat >> "$CONFIG" <<TOML

[mcp.servers.awiki]
command = "node"
args = ["$ABS_PATH"]
TOML
    fi
    echo "WIRED|codex|$CONFIG"
    ;;
  *)
    echo "No agent harness detected (.claude or .codex). Run from a configured project." >&2
    exit 1
    ;;
esac
