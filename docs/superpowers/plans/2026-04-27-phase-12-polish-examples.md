# awiki Plan — Phase 12: Polish & Examples

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** all prior phases
**Previous:** [Phase 11](./2026-04-27-phase-11-multimodal-ingest.md)
**Next:** —

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** `examples/sample-wiki/` reference, full README smoke test verified, pre-commit hook installer, Obsidian vault config.

**Branch:** `phase-12-polish`
**Depends on:** all prior phases.

## Task 12.1: Branch + Obsidian vault config

```bash
git checkout -b phase-12-polish
```

- [ ] **Step 1: Write `.obsidian/app.json`**

```bash
mkdir -p .obsidian
cat > .obsidian/app.json <<'EOF'
{
  "attachmentFolderPath": "raw/assets/",
  "alwaysUpdateLinks": true,
  "useMarkdownLinks": false
}
EOF
```

- [ ] **Step 2: Write `.obsidian/hotkeys.json`**

```bash
cat > .obsidian/hotkeys.json <<'EOF'
{
  "editor:download-attachments": [{"modifiers": ["Mod", "Shift"], "key": "D"}]
}
EOF
```

- [ ] **Step 3: Write `.obsidian/community-plugins.json`**

```bash
cat > .obsidian/community-plugins.json <<'EOF'
["dataview", "obsidian-marp"]
EOF
```

- [ ] **Step 4: Commit**

```bash
git add .obsidian
git commit -m "chore: add Obsidian vault config (attachment path, hotkeys, plugins)"
```

## Task 12.2: Pre-commit hook installer

- [ ] **Step 1: Write `scripts/install-hooks.sh`**

```bash
cat > scripts/install-hooks.sh <<'EOF'
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
EOF
chmod +x scripts/install-hooks.sh
```

- [ ] **Step 2: Add `just install-hooks` recipe**

Edit `justfile`, add under `# === bootstrap / setup ===`:

```just
install-hooks:
    bash scripts/install-hooks.sh
```

- [ ] **Step 3: Commit**

```bash
git add scripts/install-hooks.sh justfile
git commit -m "feat: add pre-commit hook installer"
```

## Task 12.3: `examples/sample-wiki/`

- [ ] **Step 1: Create reference wiki**

Create `examples/sample-wiki/` with:
- 2 entity pages (`vannevar-bush`, `claude-shannon`)
- 2 concept pages (`memex`, `information-theory`)
- 1 source page (`s-as-we-may-think`)
- `catalog.md`, `_index.md`, `log.md`

Each page has full frontmatter and at least one wikilink.

- [ ] **Step 2: README pointer**

Append to README:

> ## Example
>
> Browse `examples/sample-wiki/` for a tiny reference wiki with full frontmatter, wikilinks, and catalog.

- [ ] **Step 3: Commit**

```bash
git add examples README.md
git commit -m "docs: add examples/sample-wiki reference"
```

## Task 12.4: Verify full smoke test

- [ ] **Step 1: Fresh-clone smoke test (manual)**

```bash
cd /tmp
git clone /Users/joanmarc/dailywork/celonis/awiki test-clone
cd test-clone
git submodule update --init --recursive
just check-deps
just install-hooks
echo "# Sample\nA test source about caves." > raw/inbox/interactive/sample.md
just ingest raw/inbox/interactive/sample.md
just lint
just build
just test
```

Expected: every step succeeds. Document any rough edges in `docs/decisions/smoke-test-issues.md` and fix.

- [ ] **Step 2: Commit fixes**

```bash
git commit -am "fix: smoke test corrections"
```

## Task 12.5: Phase 12 merge

```bash
git checkout main
git merge --no-ff phase-12-polish -m "feat: complete phase 12 polish & examples"
git branch -d phase-12-polish
git tag v1.0.0
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to —.
