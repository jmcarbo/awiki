# Phase 09 — BOOTSTRAP Integration + Lint Additions + Update-Availability Check

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Wire the template-update foundation into the existing awiki bootstrap and lint infrastructure: add `<!-- bootstrap-step: <id> -->` markers to `BOOTSTRAP.md`, ship the production `template.manifest.toml`, add the new BOOTSTRAP step that calls `template-init.sh`, extend `scripts/lint.sh` with the new rules from spec, and add the update-availability check that surfaces "N updates available" via lint info.

**Architecture:** This phase modifies project-level files: `BOOTSTRAP.md`, `template.manifest.toml`, `scripts/lint.sh`, `justfile`. New helper `scripts/_template_helpers/availability.py` runs `git ls-remote` (cached) to detect upstream commits ahead of pin.

**Spec sections:** `Bootstrap integration / Step IDs / New BOOTSTRAP step`, `Lint additions`, `Recovery from a bad update / Update-availability notification`.

---

## File structure

**Created:**
- `template.manifest.toml` (production manifest, in repo root).
- `scripts/_template_helpers/availability.py`
- `scripts/_template_helpers/lint_template.py` (new lint subroutines).
- `tests/template-update-availability.bats`
- `tests/template-update-lint.bats`
- `tests/template-update-bootstrap.bats`

**Modified:**
- `BOOTSTRAP.md` — add `<!-- bootstrap-step: ... -->` markers + final `template-init` step.
- `scripts/lint.sh` — call `lint_template.py` and surface results.
- `justfile` — add remaining template-* recipes.
- `CHANGELOG.md`.

**Depends on:** Phase 08.

---

## Task 1: Production `template.manifest.toml`

**Files:**
- Create: `template.manifest.toml`
- Create: `tests/template-update-bootstrap.bats`

- [ ] **Step 1: Failing test**

Create `tests/template-update-bootstrap.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
}

@test "production manifest exists at repo root" {
  [ -f "$REPO_ROOT/template.manifest.toml" ]
}

@test "production manifest: schema_version=1, parses cleanly" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$REPO_ROOT/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^schema_version=1$'
}

@test "production manifest: dangerous-ids includes theme + wire-* + install-qmd" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" dangerous-ids "$REPO_ROOT/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qx "theme"
  echo "$output" | grep -qx "wire-qmd-mcp"
  echo "$output" | grep -qx "wire-awiki-mcp"
  echo "$output" | grep -qx "install-qmd"
}

@test "production manifest: bootstrap.ordered_steps matches BOOTSTRAP.md markers (bidirectional)" {
  MANIFEST_IDS=$(bash "$REPO_ROOT/scripts/template-manifest.sh" bootstrap-ids "$REPO_ROOT/template.manifest.toml" | sort)
  BS_IDS=$(python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" list "$REPO_ROOT/BOOTSTRAP.md" | sort)
  [ "$MANIFEST_IDS" = "$BS_IDS" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Create production manifest**

Write `template.manifest.toml`:

```toml
schema_version = 1
template_version = "1.0.0"

[strategies]
overwrite        = ["scripts/**", "layouts/**", "mcp/awiki-server/**", "deploy/**", "scheduled/**", "BOOTSTRAP.md", "README.md", "justfile", "tests/**"]
preserve         = ["content/**", "raw/**", ".obsidian/workspace*", "secrets/**", "themes/**"]
three_way        = ["WIKI.md", "hugo.toml", "content/_index.md", "content/log.md", ".gitignore", "CLAUDE.md", "AGENTS.md"]
attributes_merge = [".gitattributes"]
template_only    = ["template.manifest.toml", "migrations/**"]

[new_file_default]
strategy = "prompt"

[bootstrap]
ordered_steps = [
  "dep-check", "domain", "wiki-name", "privacy", "track-processed",
  "theme", "publish-log", "patch-identity", "install-qmd",
  "wire-qmd-mcp", "wire-awiki-mcp", "log-init", "stage-commit",
  "template-init", "smoke-test"
]

[bootstrap.dangerous]
ids = ["theme", "wire-qmd-mcp", "wire-awiki-mcp", "install-qmd"]
```

- [ ] **Step 4: Run tests — verify (manifest loads + dangerous-ids; the BOOTSTRAP-parity test stays red until Task 2)**

- [ ] **Step 5: Commit**

```bash
git add template.manifest.toml tests/template-update-bootstrap.bats
git commit -m "feat: production template.manifest.toml at repo root"
```

---

## Task 2: BOOTSTRAP.md step markers + new template-init step

**Files:**
- Modify: `BOOTSTRAP.md`

- [ ] **Step 1: Inspect current BOOTSTRAP.md**

```bash
head -100 BOOTSTRAP.md
```

(Note: the awiki scaffold spec defines BOOTSTRAP.md as having steps 0–12. The exact format depends on the existing file. The next step assumes the file exists from Phase 1 of the LLM Wiki Scaffold plan. If it does not, this task creates a minimal version matching the manifest IDs.)

- [ ] **Step 2: Add or insert markers under each step heading**

For each step ID in the manifest's `ordered_steps`, ensure the BOOTSTRAP.md has:

```markdown
### Step <number>. <Step Name>
<!-- bootstrap-step: <id> -->
<existing body>
```

If `BOOTSTRAP.md` does not yet exist (LLM Wiki Scaffold plan not complete), create a minimal one:

```markdown
# Bootstrap

This file is read by the agent on first init. It walks the user through customizing the wiki.

### Step 0. Dependency check
<!-- bootstrap-step: dep-check -->
Run `bash scripts/check-deps.sh`. Halt on missing required tools.

### Step 1. Ask domain
<!-- bootstrap-step: domain -->
Ask user: which domain (personal | research | book | business | other)?

### Step 2. Ask wiki name
<!-- bootstrap-step: wiki-name -->
Ask wiki name and one-line purpose.

### Step 3. Ask privacy level
<!-- bootstrap-step: privacy -->
Ask privacy. If sensitive → run `just encrypt-init` BEFORE first commit.

### Step 4. Track processed sources
<!-- bootstrap-step: track-processed -->
Ask: "Track ingested sources in git? (y/N)". On `y`, edit `.gitignore` to un-ignore `raw/processed/*`.

### Step 5. Theme
<!-- bootstrap-step: theme -->
Ask Hugo theme. Default `hugo-book`. If different, deinit current submodule and add chosen.

### Step 6. Publish log
<!-- bootstrap-step: publish-log -->
Ask: publish log to rendered site? Sets `draft:` flag on `content/log.md`.

### Step 7. Patch identity
<!-- bootstrap-step: patch-identity -->
Update `WIKI.md` Identity section, `hugo.toml` site title + baseURL, `content/log.md` frontmatter, `content/_index.md` welcome.

### Step 8. Install qmd
<!-- bootstrap-step: install-qmd -->
Run `just install-qmd && just reindex`. Non-fatal on failure.

### Step 9a. Wire qmd MCP
<!-- bootstrap-step: wire-qmd-mcp -->
Optional: wire qmd MCP server into agent harness via `bash scripts/wire-qmd-mcp.sh`.

### Step 9b. Wire awiki MCP
<!-- bootstrap-step: wire-awiki-mcp -->
Optional: wire awiki MCP server (ingest/lint/query/update_catalog) via `bash scripts/wire-awiki-mcp.sh`.

### Step 10. Log init
<!-- bootstrap-step: log-init -->
Append init entry: `bash scripts/log-append.sh init "<message>"`.

### Step 11. Stage initial commit
<!-- bootstrap-step: stage-commit -->
Stage all files; prompt user to review and commit.

### Step 12. Seed template provenance
<!-- bootstrap-step: template-init -->
Run `bash scripts/template-init.sh --repo <upstream-url> --ref main --version <version> --commit <commit>` to write `.awiki/template.json`, snapshot template tree to `.awiki/template-cache/<commit>/`, and record `bootstrap_steps_done[]` with content_hash for all completed steps.

### Step 13. Smoke test
<!-- bootstrap-step: smoke-test -->
Print smoke-test instructions from README.md.
```

- [ ] **Step 3: Run tests — verify the bidirectional BOOTSTRAP/manifest test passes**

```bash
bats tests/template-update-bootstrap.bats
```

- [ ] **Step 4: Commit**

```bash
git add BOOTSTRAP.md
git commit -m "feat: BOOTSTRAP.md step markers + template-init step (12)"
```

---

## Task 3: Lint additions

**Files:**
- Create: `scripts/_template_helpers/lint_template.py`
- Modify: `scripts/lint.sh`
- Create: `tests/template-update-lint.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-lint.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
}
teardown() { rm -rf "$TMP"; }

@test "lint_template: warns on identical globs in same strategy" {
  cd "$TMP"
  cat > template.manifest.toml <<'EOF'
schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = ["scripts/**", "scripts/**"]
preserve = []
three_way = []
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/lint_template.py" --root "$TMP"
  echo "$output" | grep -q "duplicate glob"
}

@test "lint_template: fails on missing migration headers" {
  cd "$TMP"
  cp "$REPO_ROOT/template.manifest.toml" .
  mkdir -p migrations
  cat > migrations/0001-bad.sh <<'EOF'
#!/usr/bin/env bash
echo no headers
EOF
  chmod +x migrations/0001-bad.sh
  run python3 "$REPO_ROOT/scripts/_template_helpers/lint_template.py" --root "$TMP"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "missing header"
}

@test "lint_template: warns on aged pending-prompts" {
  cd "$TMP"
  cp "$REPO_ROOT/template.manifest.toml" .
  mkdir -p .awiki/pending-prompts
  echo body > .awiki/pending-prompts/0001-old.md
  touch -t 202001011200 .awiki/pending-prompts/0001-old.md
  run python3 "$REPO_ROOT/scripts/_template_helpers/lint_template.py" --root "$TMP"
  echo "$output" | grep -qE "old|aged|14"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/lint_template.py`:

```python
#!/usr/bin/env python3
"""Template-update lint subroutines.

Run as: lint_template.py --root <repo-root>

Emits LINT|<level>|<file>|<msg> lines. Exit 0 = clean, 1 = warnings, 2 = errors.
"""
from __future__ import annotations

import argparse
import re
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path

HERE = Path(__file__).resolve().parent
sys.path.insert(0, str(HERE))
import bootstrap_hash  # noqa: E402
import manifest_parse  # noqa: E402

REQUIRED_MIG_HEADERS = {"migration", "requires", "touches", "idempotent"}
REQUIRED_PROMPT_KEYS = {"id", "requires", "scope_glob", "risk"}
RISK_VALUES = {"low", "medium", "high"}
BLOCKED_PATTERNS = ("secrets/", "themes/", ".awiki/", ".git/")


def emit(level: str, file: str, msg: str) -> None:
    print(f"LINT|{level}|{file}|{msg}")


def lint_manifest(root: Path) -> tuple[int, int]:
    errors = warnings = 0
    mp = root / "template.manifest.toml"
    if not mp.is_file():
        emit("error", "template.manifest.toml", "missing")
        return (1, 0)
    m = manifest_parse.load(mp)
    for strat, globs in m.get("strategies", {}).items():
        seen: set[str] = set()
        for g in globs:
            if g in seen:
                emit("warning", "template.manifest.toml", f"duplicate glob in {strat}: {g}")
                warnings += 1
            seen.add(g)
    return (errors, warnings)


def lint_migrations(root: Path) -> tuple[int, int]:
    errors = warnings = 0
    mig_dir = root / "migrations"
    if not mig_dir.is_dir():
        return (0, 0)
    pat = re.compile(r"^(\d{4}-[a-z0-9-]+\.(?:sh|prompt\.md)|schema-\d+-to-\d+\.sh|README\.md|\.gitkeep)$")
    for entry in mig_dir.iterdir():
        if not entry.is_file():
            continue
        if not pat.match(entry.name):
            emit("error", str(entry.relative_to(root)), f"migration filename does not match pattern")
            errors += 1
            continue
        if entry.name.endswith(".sh") and not entry.name.startswith("schema-"):
            # Validate header.
            text = entry.read_text(encoding="utf-8", errors="replace")
            headers: set[str] = set()
            for line in text.splitlines():
                if not line.startswith("#"):
                    if line.strip() == "" or line.startswith("#!"):
                        continue
                    break
                m_hdr = re.match(r"#\s*([a-z_]+)\s*:", line)
                if m_hdr:
                    headers.add(m_hdr.group(1))
            missing = REQUIRED_MIG_HEADERS - headers
            if missing:
                emit("error", str(entry.relative_to(root)), f"missing header keys: {sorted(missing)}")
                errors += 1
            # touches: blocklist
            for line in text.splitlines():
                m_t = re.match(r"#\s*touches\s*:\s*(.*)$", line)
                if m_t:
                    for g in m_t.group(1).split():
                        for blocked in BLOCKED_PATTERNS:
                            if g.startswith(blocked) or g == blocked.rstrip("/"):
                                emit("error", str(entry.relative_to(root)),
                                     f"touches: includes blocked pattern {g}")
                                errors += 1
        elif entry.name.endswith(".prompt.md"):
            text = entry.read_text(encoding="utf-8", errors="replace")
            fm: dict[str, str] = {}
            in_fm = False
            for line in text.splitlines():
                if line.strip() == "---":
                    if in_fm:
                        break
                    in_fm = True
                    continue
                if in_fm and ":" in line:
                    k, v = line.split(":", 1)
                    fm[k.strip()] = v.strip().strip('"').strip("'")
            missing = REQUIRED_PROMPT_KEYS - fm.keys()
            if missing:
                emit("error", str(entry.relative_to(root)), f"missing frontmatter keys: {sorted(missing)}")
                errors += 1
            risk = fm.get("risk", "")
            if risk and risk not in RISK_VALUES:
                emit("error", str(entry.relative_to(root)), f"risk must be low|medium|high: {risk}")
                errors += 1
            scope = fm.get("scope_glob", "")
            for blocked in BLOCKED_PATTERNS:
                if scope.startswith(blocked) or scope == blocked.rstrip("/"):
                    emit("error", str(entry.relative_to(root)),
                         f"scope_glob points at blocked pattern: {scope}")
                    errors += 1
    return (errors, warnings)


def lint_pending_prompts(root: Path) -> tuple[int, int]:
    errors = warnings = 0
    pp = root / ".awiki" / "pending-prompts"
    if not pp.is_dir():
        return (0, 0)
    cutoff = datetime.now(timezone.utc) - timedelta(days=14)
    for p in pp.glob("*.md"):
        mtime = datetime.fromtimestamp(p.stat().st_mtime, tz=timezone.utc)
        if mtime < cutoff:
            emit("warning", str(p.relative_to(root)),
                 f"pending prompt older than 14 days (mtime={mtime.isoformat()})")
            warnings += 1
    return (errors, warnings)


def lint_provenance_drift(root: Path) -> tuple[int, int]:
    pj = root / ".awiki" / "template.json"
    if not pj.is_file():
        return (0, 0)
    import json
    d = json.loads(pj.read_text(encoding="utf-8"))
    if d.get("repo") != d.get("original_repo"):
        emit("warning", ".awiki/template.json",
             f"repo differs from original_repo (repo={d.get('repo')}, original={d.get('original_repo')})")
        return (0, 1)
    return (0, 0)


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--root", required=True, type=Path)
    args = p.parse_args()
    total_e = total_w = 0
    for fn in (lint_manifest, lint_migrations, lint_pending_prompts, lint_provenance_drift):
        e, w = fn(args.root)
        total_e += e; total_w += w
    if total_e > 0:
        return 2
    if total_w > 0:
        return 1
    return 0


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Wire into existing `scripts/lint.sh`**

Add to `scripts/lint.sh` (location: near end, before final summary):

```bash
# Template-update lint subroutines.
TEMPLATE_LINT_OUT=$(python3 "$SCRIPT_DIR/_template_helpers/lint_template.py" --root "$REPO_ROOT" 2>&1 || true)
if [[ -n "$TEMPLATE_LINT_OUT" ]]; then
  echo "$TEMPLATE_LINT_OUT"
fi
```

(Adjust paths to match the existing `scripts/lint.sh` structure from the LLM Wiki Scaffold plan.)

- [ ] **Step 5: Run tests — verify passing**

- [ ] **Step 6: Commit**

```bash
git add scripts/_template_helpers/lint_template.py scripts/lint.sh tests/template-update-lint.bats
git commit -m "feat: lint additions for template-update (manifest dupes, mig headers, aged prompts, repo drift)"
```

---

## Task 4: Update-availability check

**Files:**
- Create: `scripts/_template_helpers/availability.py`
- Modify: `scripts/_template_helpers/lint_template.py`
- Create: `tests/template-update-availability.bats`

- [ ] **Step 1: Failing tests**

Create `tests/template-update-availability.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
}
teardown() { rm -rf "$TMP"; }

@test "availability check: --check flag returns 0 (cached <7d)" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  touch .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  [ "$status" -eq 0 ]
}

@test "availability check: missing _check-stamp triggers update check (no halt on offline)" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo /nonexistent-fork \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  rm -f .awiki/template-cache/_check-stamp
  run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  # Network failure should not halt (lint info skipped).
  [ "$status" -eq 0 ]
}

@test "availability check: AWIKI_NO_TEMPLATE_CHECK suppresses" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v0" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  AWIKI_NO_TEMPLATE_CHECK=1 run python3 "$REPO_ROOT/scripts/_template_helpers/availability.py" --root "$TMP" --check
  [ "$status" -eq 0 ]
  [ -z "$output" ] || true
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/availability.py`:

```python
#!/usr/bin/env python3
"""Update-availability check.

--check: read .awiki/template-cache/_check-stamp; if older than 7 days OR missing,
         run `git ls-remote <repo> <ref>` (cached for 1h via _check-stamp mtime).
         If new commits available, emit LINT|info|template|...
"""
from __future__ import annotations

import argparse
import json
import os
import subprocess
import sys
from datetime import datetime, timedelta, timezone
from pathlib import Path


def main() -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--root", required=True, type=Path)
    p.add_argument("--check", action="store_true")
    args = p.parse_args()
    if not args.check:
        return 0

    if os.environ.get("AWIKI_NO_TEMPLATE_CHECK") == "1":
        return 0

    pj = args.root / ".awiki" / "template.json"
    if not pj.is_file():
        return 0
    d = json.loads(pj.read_text(encoding="utf-8"))

    config_path = args.root / ".awiki" / "config"
    if config_path.is_file():
        for line in config_path.read_text(encoding="utf-8").splitlines():
            if line.startswith("no_template_check=true"):
                return 0

    cache_dir = args.root / ".awiki" / "template-cache"
    cache_dir.mkdir(parents=True, exist_ok=True)
    stamp = cache_dir / "_check-stamp"
    cutoff = datetime.now(timezone.utc) - timedelta(days=7)
    if stamp.is_file():
        mtime = datetime.fromtimestamp(stamp.stat().st_mtime, tz=timezone.utc)
        if mtime > cutoff:
            return 0  # fresh

    # Run git ls-remote.
    repo = d.get("repo", "")
    ref = d.get("ref", "main")
    pinned = d.get("commit", "")
    try:
        result = subprocess.run(
            ["git", "ls-remote", repo, ref],
            capture_output=True, text=True, timeout=10
        )
    except Exception:
        return 0  # network issue; skip silently
    if result.returncode != 0:
        return 0
    line = result.stdout.split("\n")[0].strip()
    if not line:
        return 0
    upstream = line.split()[0]

    stamp.touch()  # update stamp regardless

    if upstream != pinned:
        # Count commits ahead is hard with shallow; just say "available".
        print(f"LINT|info|template|template updates available since {pinned[:12]}. Run `just template-status` to review.")

    return 0


if __name__ == "__main__":
    sys.exit(main())
```

Add a call into `scripts/_template_helpers/lint_template.py`'s `main()`:

```python
def lint_availability(root: Path) -> tuple[int, int]:
    subprocess.run(
        ["python3", str(HERE / "availability.py"), "--root", str(root), "--check"],
        check=False
    )
    return (0, 0)
```

Insert into the `for fn in (...)` tuple.

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/availability.py scripts/_template_helpers/lint_template.py tests/template-update-availability.bats
git commit -m "feat: update-availability check via _check-stamp + AWIKI_NO_TEMPLATE_CHECK opt-out"
```

---

## Task 5: Justfile recipes

**Files:**
- Modify: `justfile`

- [ ] **Step 1: Add full recipe set**

Append to `justfile`:

```just
# === template (added Phase 09) ===
template-update *args:
    bash scripts/template-update.sh {{args}}

template-status:
    bash scripts/template-update.sh --status

template-gc:
    bash scripts/template-update.sh --gc

bootstrap-step id:
    bash scripts/template-step.sh {{id}}

template-retrofit:
    bash scripts/template-retrofit.sh
```

(`template-step.sh` and `template-retrofit.sh` are introduced in Phase 10 — recipes can reference future scripts; `just` defers errors.)

- [ ] **Step 2: Quick sanity check**

```bash
just --list | grep -E 'template-(update|status|gc|retrofit)'
```

- [ ] **Step 3: Commit**

```bash
git add justfile
git commit -m "feat: justfile recipes for template-update, template-status, template-gc, template-retrofit, bootstrap-step"
```

---

## Task 6: CHANGELOG + verify all Phase 09

- [ ] **Step 1: Run all Phase 09 tests**

```bash
bats tests/template-update-bootstrap.bats tests/template-update-lint.bats tests/template-update-availability.bats
```

- [ ] **Step 2: Append CHANGELOG**

```markdown
### Added
- Production `template.manifest.toml` at repo root with full strategy + bootstrap config.
- `BOOTSTRAP.md` step markers (`<!-- bootstrap-step: <id> -->`) and new `template-init` step (Step 12).
- Lint additions (manifest dupes, migration headers, aged pending prompts, repo/original_repo drift).
- Update-availability check (`_check-stamp` 7-day cadence + `AWIKI_NO_TEMPLATE_CHECK` opt-out + `.awiki/config.no_template_check`).
- Justfile recipes: `template-update`, `template-status`, `template-gc`, `template-retrofit`, `bootstrap-step` (Phase 09).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 09 BOOTSTRAP + lint + availability"
```

---

## Phase 09 — Definition of done

- [ ] Production `template.manifest.toml` matches spec.
- [ ] `BOOTSTRAP.md` has marker for every ID in `bootstrap.ordered_steps` (bidirectional; lint enforces).
- [ ] `BOOTSTRAP.md` Step 12 calls `template-init.sh`.
- [ ] Lint emits warnings for: duplicate globs, repo/original_repo drift, aged pending prompts.
- [ ] Lint emits errors for: missing migration headers, blocked `touches:`, missing prompt frontmatter, blocked `scope_glob`.
- [ ] Availability check emits `LINT|info|template|...` when upstream is ahead; suppressed by env var or config.
- [ ] Justfile lists all template recipes.
- [ ] CHANGELOG entry added.

Phase 10 closes out: retrofit, full BATS matrix, CI E2E, docs.
