#!/usr/bin/env bash
set -euo pipefail

HOOK=".git/hooks/pre-commit"
cat > "$HOOK" <<'HOOKEOF'
#!/usr/bin/env bash
set -e
just lint
HOOKEOF
chmod +x "$HOOK"
echo "INSTALLED|$HOOK"
