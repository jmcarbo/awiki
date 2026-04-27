# awiki Plan — Phase 4: qmd Integration

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 1
**Previous:** [Phase 03](./2026-04-27-phase-03-hugo-render.md)
**Next:** [Phase 05](./2026-04-27-phase-05-encryption.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** `install-qmd.sh`, `qmd-index.sh`, MCP wiring option, `.awiki/qmd-status` flag, grep fallback in WIKI.md.

**Branch:** `phase-4-qmd-integration`

## Task 4.1: Spike — confirm qmd build

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-4-qmd-integration
```

- [ ] **Step 2: Clone qmd locally + build**

```bash
git clone https://github.com/qntx-labs/qmd /tmp/qmd-spike
cd /tmp/qmd-spike
cat README.md   # determine toolchain + build cmd
# Run upstream-documented build steps
cd -
```

- [ ] **Step 3: Document outcome in `docs/decisions/qmd-install.md`**

Capture: language, build command, install target, macOS + Linux verification.

- [ ] **Step 4: Commit**

```bash
git add docs/decisions/qmd-install.md
git commit -m "docs: record qmd install decision after spike"
```

## Task 4.2: `scripts/install-qmd.sh`

**Files:** Create: `scripts/install-qmd.sh`, `tests/install_qmd_test.bats`

- [ ] **Step 1: Write the test**

```bash
cat > tests/install_qmd_test.bats <<'EOF'
#!/usr/bin/env bats

@test "install-qmd is idempotent if qmd already on PATH" {
  if ! command -v qmd >/dev/null 2>&1; then skip "qmd not installed"; fi
  run bash scripts/install-qmd.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"already installed"* ]]
}

@test "install-qmd writes status file on success" {
  run bash scripts/install-qmd.sh
  if [ "$status" -eq 0 ]; then
    [ -f .awiki/qmd-status ]
    [[ "$(cat .awiki/qmd-status)" =~ ^(ok|missing)$ ]]
  fi
}
EOF
```

- [ ] **Step 2: Run test (FAIL expected)**

Run: `bats tests/install_qmd_test.bats`

- [ ] **Step 3: Write `scripts/install-qmd.sh` (using spike outcome)**

```bash
cat > scripts/install-qmd.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

mkdir -p .awiki
STATUS_FILE=".awiki/qmd-status"

if command -v qmd >/dev/null 2>&1; then
  echo "qmd already installed at: $(command -v qmd)"
  echo "ok" > "$STATUS_FILE"
  exit 0
fi

SRC_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/qmd-src"
mkdir -p "$(dirname "$SRC_DIR")"

if [[ ! -d "$SRC_DIR/.git" ]]; then
  git clone https://github.com/qntx-labs/qmd "$SRC_DIR"
fi
( cd "$SRC_DIR" && git pull --ff-only )

# Build per upstream — pinned via docs/decisions/qmd-install.md.
# Replace with the actual command from spike outcome.
( cd "$SRC_DIR" && make build ) || {
  echo "missing" > "$STATUS_FILE"
  echo "qmd build failed; agent will use grep fallback" >&2
  exit 1
}

mkdir -p "$HOME/.local/bin"
ln -sf "$SRC_DIR/qmd" "$HOME/.local/bin/qmd"

if ! command -v qmd >/dev/null 2>&1; then
  echo "Add ~/.local/bin to PATH:"
  echo '  export PATH="$HOME/.local/bin:$PATH"'
fi

echo "ok" > "$STATUS_FILE"
echo "qmd installed at $HOME/.local/bin/qmd"
EOF
chmod +x scripts/install-qmd.sh
```

Note: replace `make build` with the actual command from the spike.

- [ ] **Step 4: Run test (PASS expected)**

Run: `bats tests/install_qmd_test.bats`

- [ ] **Step 5: Commit**

```bash
git add scripts/install-qmd.sh tests/install_qmd_test.bats
git commit -m "feat: add install-qmd script + status flag"
```

## Task 4.3: `scripts/qmd-index.sh`

**Files:** Create: `scripts/qmd-index.sh`

- [ ] **Step 1: Write `scripts/qmd-index.sh`**

```bash
cat > scripts/qmd-index.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if ! command -v qmd >/dev/null 2>&1; then
  echo "QMD-INDEX|skip|reason=qmd-not-installed" >&2
  exit 0
fi

mkdir -p .qmd
qmd index content/
echo "QMD-INDEX|ok"
EOF
chmod +x scripts/qmd-index.sh
```

- [ ] **Step 2: Smoke test**

Run: `just reindex`
Expected: either `QMD-INDEX|ok` or `QMD-INDEX|skip` — no crash.

- [ ] **Step 3: Commit**

```bash
git add scripts/qmd-index.sh
git commit -m "feat: add qmd-index script (no-op if qmd missing)"
```

## Task 4.4: MCP wiring helper

**Files:** Create: `scripts/wire-qmd-mcp.sh`

- [ ] **Step 1: Write `scripts/wire-qmd-mcp.sh`**

```bash
cat > scripts/wire-qmd-mcp.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

if ! command -v qmd >/dev/null 2>&1; then
  echo "qmd not installed; cannot wire MCP server" >&2
  exit 1
fi

# Detect harness — prefer codex if both present (last-write wins)
TARGET=""
if [[ -d .claude || -f CLAUDE.md ]]; then TARGET="claude"; fi
if [[ -d .codex ]]; then TARGET="codex"; fi

case "$TARGET" in
  claude)
    CONFIG="./.mcp.json"   # Claude Code reads project-level MCP from repo root
    if [[ ! -f "$CONFIG" ]]; then echo '{"mcpServers": {}}' > "$CONFIG"; fi
    python3 - "$CONFIG" <<'PY'
import json, sys
path = sys.argv[1]
data = json.load(open(path))
data.setdefault("mcpServers", {})["qmd"] = {
    "command": "qmd",
    "args": ["mcp", "--root", "content/"]
}
json.dump(data, open(path, "w"), indent=2)
PY
    echo "WIRED|claude|$CONFIG"
    exit 0
    ;;
  codex)
    CONFIG=".codex/config.toml"
    mkdir -p .codex
    if ! grep -q '^\[mcp.servers.qmd\]' "$CONFIG" 2>/dev/null; then
      cat >> "$CONFIG" <<'TOML'

[mcp.servers.qmd]
command = "qmd"
args = ["mcp", "--root", "content/"]
TOML
    fi
    echo "WIRED|codex|$CONFIG"
    exit 0
    ;;
  *)
    echo "No agent harness detected (.claude or .codex). Run from a configured project." >&2
    exit 1
    ;;
esac
EOF
chmod +x scripts/wire-qmd-mcp.sh
```

- [ ] **Step 2: Commit**

```bash
git add scripts/wire-qmd-mcp.sh
git commit -m "feat: add MCP wiring helper for qmd (claude/codex detection)"
```

## Task 4.5: Phase 4 merge

```bash
bats tests/
git checkout main
git merge --no-ff phase-4-qmd-integration -m "feat: complete phase 4 qmd integration"
git branch -d phase-4-qmd-integration
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 05](./2026-04-27-phase-05-encryption.md).
