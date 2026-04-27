# awiki Plan — Phase 5: Encryption

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 1, 2
**Previous:** [Phase 04](./2026-04-27-phase-04-qmd-integration.md)
**Next:** [Phase 06](./2026-04-27-phase-06-section-indexes.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** `encrypt-init.sh` (git-crypt + age), atomic `.gitignore` flip, `.gitattributes` patterns, lint privacy checks.

**Branch:** `phase-5-encryption`
**Depends on:** Phase 1, 2.

## Task 5.1: Branch + `scripts/encrypt-init.sh` (git-crypt path)

**Files:** Create: `scripts/encrypt-init.sh`

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-5-encryption
```

- [ ] **Step 2: Write `scripts/encrypt-init.sh`**

```bash
cat > scripts/encrypt-init.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

MODE="git-crypt"
if [[ "${1:-}" == "--age" ]]; then MODE="age"; fi

case "$MODE" in
  git-crypt)
    if ! command -v git-crypt >/dev/null 2>&1; then
      echo "git-crypt not installed. Install: brew install git-crypt (mac) / apt install git-crypt (linux)" >&2
      exit 1
    fi
    if [[ -d .git/git-crypt ]]; then
      echo "git-crypt already initialized" >&2
      exit 0
    fi
    git-crypt init

    # Activate patterns in .gitattributes (portable across BSD/GNU sed via awk)
    awk '
      /^# raw\/processed\/private\// { sub(/^# /, ""); print; next }
      /^# raw\/processed\/_originals\// { sub(/^# /, ""); print; next }
      /^# content\/private\// { sub(/^# /, ""); print; next }
      /^# secrets\// { sub(/^# /, ""); print; next }
      { print }
    ' .gitattributes > .gitattributes.tmp && mv .gitattributes.tmp .gitattributes

    # Atomically remove the gitignore lines for private paths so encrypted commits work
    awk '
      /^content\/private\/\*\*$/ { next }
      /^raw\/processed\/private\/\*\*$/ { next }
      /^raw\/processed\/_originals\/\*\*$/ { next }
      { print }
    ' .gitignore > .gitignore.tmp && mv .gitignore.tmp .gitignore

    mkdir -p secrets
    git-crypt export-key secrets/.git-crypt-key
    chmod 600 secrets/.git-crypt-key

    bash scripts/log-append.sh encrypt "git-crypt initialized; key at secrets/.git-crypt-key"
    echo "ENCRYPT-OK|mode=git-crypt|key=secrets/.git-crypt-key"
    echo "STORE the key securely (password manager). Anyone with this key can read encrypted paths."
    ;;
  age)
    if ! command -v age >/dev/null 2>&1; then
      echo "age not installed. Install: brew install age / apt install age" >&2
      exit 1
    fi
    mkdir -p secrets
    if [[ -f secrets/age.key ]]; then
      echo "age key already exists at secrets/age.key" >&2
      exit 0
    fi
    age-keygen -o secrets/age.key
    grep '^# public key:' secrets/age.key | sed 's/^# public key: //' > secrets/age.pub
    chmod 600 secrets/age.key

    bash scripts/log-append.sh encrypt "age keypair generated"
    echo "ENCRYPT-OK|mode=age|key=secrets/age.key|pub=secrets/age.pub"
    echo "Encrypt sensitive files: age -R secrets/age.pub -o file.age file"
    echo "Decrypt: age -d -i secrets/age.key file.age"
    ;;
esac
EOF
chmod +x scripts/encrypt-init.sh
```

- [ ] **Step 3: Test (manual, requires git-crypt installed)**

Run in a throwaway clone:
```bash
just encrypt-init
git status
git-crypt status content/private/test.md  # should show encrypted
```

- [ ] **Step 4: Commit**

```bash
git add scripts/encrypt-init.sh
git commit -m "feat: add encrypt-init script (git-crypt + age modes)"
```

## Task 5.2: Lint privacy checks

**Files:** Modify: `scripts/lint.sh`

- [ ] **Step 1: Add test**

Append to `tests/lint_test.bats`:

```bash
@test "lint warns on tags: [private] outside private/ path" {
  TMP="$(mktemp -d)/content"
  mkdir -p "$TMP/entities"
  cat > "$TMP/entities/leaky.md" <<E
---
title: "Leaky"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: [private]
aliases: []
sources: []
draft: false
---

Body content sufficient length for non-empty check.
E
  run bash scripts/lint.sh "$TMP"
  [[ "$output" == *"LINT|WARN"*"leaky.md"*"private tag outside private path"* ]]
}
```

- [ ] **Step 2: Run test (FAIL)**

- [ ] **Step 3: Modify `scripts/lint.sh` — add privacy check**

Inside the per-page-checks loop, after the empty-page check, add:

```bash
  # Parse the tags: line as a YAML list, check for an exact 'private' token.
  has_private_tag=0
  while IFS= read -r line; do
    if [[ "$line" =~ ^tags:[[:space:]]*\[(.*)\][[:space:]]*$ ]]; then
      raw="${BASH_REMATCH[1]}"
      IFS=',' read -ra parts <<< "$raw"
      for p in "${parts[@]}"; do
        token="$(echo "$p" | sed -E 's/^[ "'\'']+|[ "'\'']+$//g')"
        if [[ "$token" == "private" ]]; then has_private_tag=1; fi
      done
    fi
  done < "$page"

  if [[ "$has_private_tag" -eq 1 && "$page" != *"/private/"* ]]; then
    echo "LINT|WARN|$page|private tag outside private path"
    WARNS=$((WARNS + 1))
  fi
```

- [ ] **Step 4: Run test (PASS)**

- [ ] **Step 5: Commit**

```bash
git add scripts/lint.sh tests/lint_test.bats
git commit -m "feat: lint warns on private tag outside private path"
```

## Task 5.3: Phase 5 merge

```bash
bats tests/
git checkout main
git merge --no-ff phase-5-encryption -m "feat: complete phase 5 encryption"
git branch -d phase-5-encryption
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 06](./2026-04-27-phase-06-section-indexes.md).
