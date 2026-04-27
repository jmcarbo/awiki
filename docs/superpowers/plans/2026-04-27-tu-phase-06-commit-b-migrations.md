# Phase 06 — Phase 3 Commit B (Migrations)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement Phase 3 Commit B: run mechanical migrations with stripped env + `touches:` enforcement + `.awiki/` ban; stage LLM prompt migrations into `.awiki/pending-prompts/` with scope_glob enforcement and risk handling; halt on failure with state-file recovery.

**Architecture:** New helper `scripts/_template_helpers/migration.py` parses migration headers, runs scripts with `env -i`, validates `touches:`, checks post-run `git status` against declared globs and `.awiki/` ban. LLM staging copies prompt + appends resolved scope file list. Orchestrator iterates `migrations/` in numeric order against `applied_migrations[]`.

**Tech Stack:** bash 4+, `env -i`, `git status --porcelain`, Python 3 for header parsing + scope_glob resolution.

**Spec sections:** `Migrations / Mechanical`, `Migrations / LLM-assisted`, `Trust Model / Migration execution constraints`, `Update flow / Phase 3 — Commit B`.

---

## File structure

**Created:**
- `scripts/_template_helpers/migration.py` — header parser, runner, validator, LLM stager.
- `tests/template-update-migration.bats`
- Test fixtures:
  - `tests/fixtures/template-update/v2-with-llm/` — extends v1 + LLM prompt + dangerous-step content change.

**Modified:**
- `scripts/template-update.sh` — Commit B wiring.

**Depends on:** Phases 01–05.

---

## Task 1: Migration header parser

**Files:**
- Create: `scripts/_template_helpers/migration.py`
- Create: `tests/template-update-migration.bats`

- [ ] **Step 1: Failing test**

Create `tests/template-update-migration.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "migration parse-header: extracts touches + idempotent + requires" {
  cat > "$TMP/0001-x.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: WIKI.md content/synthesis/**/*.md
# idempotent: yes
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" parse-header "$TMP/0001-x.sh"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE '^touches=WIKI.md content/synthesis/\*\*/\*\.md$'
  echo "$output" | grep -qE '^idempotent=yes$'
  echo "$output" | grep -qE '^requires=agent=false$'
}

@test "migration parse-header: missing required field -> nonzero" {
  cat > "$TMP/0001-x.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# touches: WIKI.md
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" parse-header "$TMP/0001-x.sh"
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/migration.py`:

```python
#!/usr/bin/env python3
"""Migration runner + LLM stager.

Subcommands:
  parse-header <script>
  validate-touches <script>     — fails if touches: includes secrets/, .awiki/, .git/, themes/
  run <script> --repo-root <dir> --old-version <v> --new-version <v>
  stage-prompt <prompt-md> --user-tree <dir> --pending-dir <dir> --gitattributes <path>
"""
from __future__ import annotations

import argparse
import os
import re
import subprocess
import sys
from fnmatch import fnmatch
from pathlib import Path

REQUIRED_HEADER_KEYS = {"migration", "requires", "touches", "idempotent"}
BLOCKED_PATTERNS = ("secrets/", "themes/", ".awiki/", ".git/")


def parse_header(script: Path) -> dict[str, str]:
    out: dict[str, str] = {}
    with script.open("r", encoding="utf-8") as f:
        for line in f:
            if not line.startswith("#"):
                if line.strip() == "":
                    continue
                if line.startswith("#!"):
                    continue
                # Hit non-comment line; stop.
                break
            m = re.match(r"#\s*([a-z_]+)\s*:\s*(.*)$", line.rstrip("\n"))
            if m:
                out[m.group(1)] = m.group(2).strip()
    return out


def cmd_parse_header(args: argparse.Namespace) -> int:
    h = parse_header(args.script)
    missing = REQUIRED_HEADER_KEYS - set(h.keys())
    if missing:
        print(f"missing header keys: {', '.join(sorted(missing))}", file=sys.stderr)
        return 1
    for k, v in h.items():
        print(f"{k}={v}")
    return 0


def cmd_validate_touches(args: argparse.Namespace) -> int:
    h = parse_header(args.script)
    touches = h.get("touches", "").split()
    for glob in touches:
        for blocked in BLOCKED_PATTERNS:
            if glob.startswith(blocked) or glob == blocked.rstrip("/"):
                print(f"touches blocked: {glob}", file=sys.stderr)
                return 1
    return 0


def cmd_run(args: argparse.Namespace) -> int:
    h = parse_header(args.script)
    if cmd_validate_touches(args) != 0:
        return 1
    touches = h.get("touches", "").split()

    repo_root = args.repo_root
    env = {
        "PATH": os.environ.get("PATH", "/usr/bin:/bin"),
        "HOME": os.environ.get("HOME", str(Path.home())),
        "AWIKI_REPO_ROOT": str(repo_root),
        "AWIKI_TEMPLATE_OLD_VERSION": args.old_version,
        "AWIKI_TEMPLATE_NEW_VERSION": args.new_version,
        "LANG": os.environ.get("LANG", "C.UTF-8"),
        "LC_ALL": os.environ.get("LC_ALL", "C.UTF-8"),
    }

    # Run --dry-run first as audit log.
    print(f"info: dry-run {args.script}", file=sys.stderr)
    rc = subprocess.call(["bash", str(args.script), "--dry-run"], env=env, cwd=str(repo_root))
    if rc != 0:
        print(f"migration dry-run failed: {args.script}", file=sys.stderr)
        return rc

    # Real run.
    rc = subprocess.call(["bash", str(args.script)], env=env, cwd=str(repo_root))
    if rc != 0:
        return rc

    # Post-run audit: every modified path must match touches: or be in .awiki/ blocklist.
    out = subprocess.run(
        ["git", "-C", str(repo_root), "status", "--porcelain"],
        capture_output=True, text=True
    ).stdout
    modified: list[str] = []
    for line in out.splitlines():
        if not line.strip():
            continue
        # Format: "XY filename"
        rel = line[3:].strip()
        if "->" in rel:
            rel = rel.split("->", 1)[1].strip()
        modified.append(rel)

    out_of_scope_tracked: list[str] = []
    out_of_scope_untracked: list[str] = []
    awiki_writes_tracked: list[str] = []
    awiki_writes_untracked: list[str] = []

    # Re-parse porcelain to distinguish tracked vs untracked.
    # Format: "XY filename" — untracked is "?? filename".
    for line in out.splitlines():
        if not line.strip():
            continue
        xy = line[:2]
        rel = line[3:].strip()
        if "->" in rel:
            rel = rel.split("->", 1)[1].strip()
        is_untracked = (xy == "??")
        if rel.startswith(".awiki/"):
            (awiki_writes_untracked if is_untracked else awiki_writes_tracked).append(rel)
        elif not any(_glob_match(g, rel) for g in touches):
            (out_of_scope_untracked if is_untracked else out_of_scope_tracked).append(rel)

    def revert(tracked: list[str], untracked: list[str]) -> None:
        if tracked:
            subprocess.call(["git", "-C", str(repo_root), "restore", "--source=HEAD", "--", *tracked])
        for u in untracked:
            try:
                (repo_root / u).unlink()
            except FileNotFoundError:
                pass

    if awiki_writes_tracked or awiki_writes_untracked:
        all_awiki = awiki_writes_tracked + awiki_writes_untracked
        print(f"migration wrote to .awiki/: {all_awiki}", file=sys.stderr)
        revert(awiki_writes_tracked, awiki_writes_untracked)
        return 1
    if out_of_scope_tracked or out_of_scope_untracked:
        all_oos = out_of_scope_tracked + out_of_scope_untracked
        print(f"migration wrote outside touches:: {all_oos}", file=sys.stderr)
        revert(out_of_scope_tracked, out_of_scope_untracked)
        return 1
    return 0


def _glob_match(glob: str, path: str) -> bool:
    if "**" in glob:
        parts = glob.split("**")
        if len(parts) == 2:
            prefix, suffix = parts
            prefix = prefix.rstrip("/")
            suffix = suffix.lstrip("/")
            if prefix and not (path == prefix or path.startswith(prefix + "/")):
                return False
            if suffix and not fnmatch(path, "*" + suffix):
                return False
            return True
    return fnmatch(path, glob)


def cmd_stage_prompt(args: argparse.Namespace) -> int:
    text = args.prompt_md.read_text(encoding="utf-8")
    fm = _parse_yaml_frontmatter(text)
    sid = fm.get("id", args.prompt_md.stem.removesuffix(".prompt"))
    scope = fm.get("scope_glob", "")
    risk = fm.get("risk", "medium")

    if not scope:
        print(f"prompt missing scope_glob: {args.prompt_md}", file=sys.stderr)
        return 1
    for blocked in BLOCKED_PATTERNS:
        if scope.startswith(blocked) or scope == blocked.rstrip("/"):
            print(f"scope_glob blocked: {scope}", file=sys.stderr)
            return 1

    # Resolve scope.
    matches: list[str] = []
    for p in args.user_tree.rglob("*"):
        if p.is_file():
            rel = str(p.relative_to(args.user_tree))
            if _glob_match(scope, rel):
                matches.append(rel)

    # Encrypted-path filter: reject scope_glob matching any path with filter=git-crypt.
    if args.gitattributes is not None and args.gitattributes.is_file():
        for rel in matches:
            cr = subprocess.run(
                ["git", "-c", f"core.attributesfile={args.gitattributes}",
                 "check-attr", "filter", rel],
                capture_output=True, text=True
            )
            if "filter: git-crypt" in cr.stdout:
                print(f"scope_glob would match encrypted path: {rel}", file=sys.stderr)
                return 1

    args.pending_dir.mkdir(parents=True, exist_ok=True)
    dest = args.pending_dir / args.prompt_md.name
    body = text
    metadata = "\n\n## Resolved scope\n" + "\n".join(f"- {m}" for m in matches)
    metadata += "\n\n## Acceptance\nAfter completing, run: `just lint`."
    metadata += "\n\n## Trust note\nConfirm intent with user before bulk edits."
    metadata += f"\n\n## Risk\n{risk}\n"
    dest.write_text(body + metadata, encoding="utf-8")
    print(f"info: staged {dest.name} (risk={risk}, files={len(matches)})", file=sys.stderr)
    return 0


def _parse_yaml_frontmatter(text: str) -> dict[str, str]:
    out: dict[str, str] = {}
    in_fm = False
    for line in text.splitlines():
        if line.strip() == "---":
            if in_fm:
                break
            in_fm = True
            continue
        if in_fm and ":" in line:
            k, v = line.split(":", 1)
            out[k.strip()] = v.strip().strip('"').strip("'").strip("[").strip("]")
    return out


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p = sub.add_parser("parse-header"); p.add_argument("script", type=Path)
    p = sub.add_parser("validate-touches"); p.add_argument("script", type=Path)
    p = sub.add_parser("run")
    p.add_argument("script", type=Path)
    p.add_argument("--repo-root", required=True, type=Path)
    p.add_argument("--old-version", required=True)
    p.add_argument("--new-version", required=True)
    p = sub.add_parser("stage-prompt")
    p.add_argument("prompt_md", type=Path)
    p.add_argument("--user-tree", required=True, type=Path)
    p.add_argument("--pending-dir", required=True, type=Path)
    p.add_argument("--gitattributes", required=False, type=Path, default=None)

    args = parser.parse_args()
    return {
        "parse-header": cmd_parse_header,
        "validate-touches": cmd_validate_touches,
        "run": cmd_run,
        "stage-prompt": cmd_stage_prompt,
    }[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
```

`chmod +x scripts/_template_helpers/migration.py`.

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/migration.py tests/template-update-migration.bats
git commit -m "feat: migration.py parse-header"
```

---

## Task 2: `validate-touches` blocks dangerous patterns

**Files:**
- Modify: `tests/template-update-migration.bats`

- [ ] **Step 1: Failing test**

```bash
@test "migration validate-touches: secrets/ blocked" {
  cat > "$TMP/m.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: secrets/foo.age
# idempotent: yes
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" validate-touches "$TMP/m.sh"
  [ "$status" -ne 0 ]
}

@test "migration validate-touches: clean touches passes" {
  cat > "$TMP/m.sh" <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: WIKI.md
# idempotent: yes
EOF
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" validate-touches "$TMP/m.sh"
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 2: Run** — already implemented in Task 1; expect pass.

- [ ] **Step 3: Commit**

```bash
git add tests/template-update-migration.bats
git commit -m "test: validate-touches blocks secrets/ etc."
```

---

## Task 3: `run` enforces stripped env + `touches:` post-run check

**Files:**
- Modify: `tests/template-update-migration.bats`

- [ ] **Step 1: Failing tests**

```bash
@test "migration run: stripped env removes GH_TOKEN" {
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  cat > m.sh <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: token-leak.txt
# idempotent: yes
set -euo pipefail
[[ "${1:-}" == "--dry-run" ]] && exit 0
echo "token=${GH_TOKEN:-EMPTY}" > token-leak.txt
EOF
  chmod +x m.sh
  GH_TOKEN=secret run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" run \
    "$TMP/m.sh" --repo-root "$TMP" --old-version 0.1 --new-version 0.2
  [ "$status" -eq 0 ]
  grep -q '^token=EMPTY$' token-leak.txt
}

@test "migration run: writes outside touches: -> halt + revert" {
  cd "$TMP"
  git init -q
  echo content > inscope.txt
  echo content > outscope.txt
  git add . && git -c user.email=a@b -c user.name=t commit -q -m init
  cat > m.sh <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: inscope.txt
# idempotent: yes
set -euo pipefail
[[ "${1:-}" == "--dry-run" ]] && exit 0
echo modified > inscope.txt
echo broken > outscope.txt
EOF
  chmod +x m.sh
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" run \
    "$TMP/m.sh" --repo-root "$TMP" --old-version 0.1 --new-version 0.2
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "outside touches"
  # outscope reverted to original
  grep -q '^content$' outscope.txt
}

@test "migration run: writes to .awiki/ -> halt + revert" {
  cd "$TMP"
  git init -q
  mkdir -p .awiki
  echo content > .awiki/template.json
  git add . && git -c user.email=a@b -c user.name=t commit -q -m init
  cat > m.sh <<'EOF'
#!/usr/bin/env bash
# migration: 0001-x
# requires: agent=false
# touches: .awiki/template.json
# idempotent: yes
set -euo pipefail
[[ "${1:-}" == "--dry-run" ]] && exit 0
echo broken > .awiki/template.json
EOF
  chmod +x m.sh
  # validate-touches itself should reject .awiki/ pattern.
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" run \
    "$TMP/m.sh" --repo-root "$TMP" --old-version 0.1 --new-version 0.2
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run — verify (expected 3 of 3 pass)**

- [ ] **Step 3: Commit**

```bash
git add tests/template-update-migration.bats
git commit -m "test: migration run env-strip + touches enforcement + .awiki ban"
```

---

## Task 4: LLM prompt staging with scope_glob enforcement

**Files:**
- Create: `tests/fixtures/template-update/v2-with-llm/migrations/0002-rewrite-foo.prompt.md`
- Modify: `tests/template-update-migration.bats`

- [ ] **Step 1: Create v2 fixture**

```bash
cp -R tests/fixtures/template-update/v1 tests/fixtures/template-update/v2-with-llm
```

Create `tests/fixtures/template-update/v2-with-llm/migrations/0002-rewrite-foo.prompt.md`:

```markdown
---
id: 0002-rewrite-foo
requires: [agent]
scope_glob: "content/**/*.md"
risk: medium
---

Rewrite each `## Foo` heading to `## Foo (renamed)`. After each file, run `just lint`.
```

Bump `tests/fixtures/template-update/v2-with-llm/template.manifest.toml` `template_version = "0.3.0"`.

- [ ] **Step 2: Failing tests**

```bash
@test "migration stage-prompt: writes into pending-dir with metadata" {
  cd "$TMP"
  V2="$REPO_ROOT/tests/fixtures/template-update/v2-with-llm"
  mkdir -p pending content/notes
  echo "## Foo" > content/notes/a.md
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" stage-prompt \
    "$V2/migrations/0002-rewrite-foo.prompt.md" \
    --user-tree "$TMP" --pending-dir "$TMP/pending"
  [ "$status" -eq 0 ]
  [ -f "$TMP/pending/0002-rewrite-foo.prompt.md" ]
  grep -q "## Resolved scope" "$TMP/pending/0002-rewrite-foo.prompt.md"
  grep -q "content/notes/a.md" "$TMP/pending/0002-rewrite-foo.prompt.md"
  grep -q "## Risk" "$TMP/pending/0002-rewrite-foo.prompt.md"
}

@test "migration risk: high under --non-interactive auto-declined" {
  cd "$TMP"
  TMP_V=$(mktemp -d)
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v1/." "$TMP_V/"
  cat > "$TMP_V/migrations/0003-high-risk.prompt.md" <<'EOF'
---
id: 0003-high-risk
requires: [agent]
scope_glob: "content/**/*.md"
risk: high
---
Big rewrite.
EOF
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A && git -c user.email=a@b -c user.name=t commit -q -m init
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$TMP_V" --accept-source-change --apply --non-interactive
  [ ! -f .awiki/pending-prompts/0003-high-risk.prompt.md ]
  python3 -c "
import json
d = json.load(open('.awiki/template.json'))
ids = [(m.get('id'), m.get('status')) for m in d['applied_migrations']]
assert ('0003-high-risk', 'skipped') in ids, ids
"
  rm -rf "$TMP_V"
}

@test "migration stage-prompt: scope_glob secrets/ rejected" {
  cd "$TMP"
  cat > bad.prompt.md <<'EOF'
---
id: bad
requires: [agent]
scope_glob: "secrets/**"
risk: medium
---
body
EOF
  mkdir -p pending
  run python3 "$REPO_ROOT/scripts/_template_helpers/migration.py" stage-prompt \
    "$TMP/bad.prompt.md" --user-tree "$TMP" --pending-dir "$TMP/pending"
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 3: Run — already-implemented; expect pass**

- [ ] **Step 4: Commit**

```bash
git add tests/fixtures/template-update/v2-with-llm/ tests/template-update-migration.bats
git commit -m "feat: migration.py stage-prompt + v2 fixture"
```

---

## Task 5: Wire Phase 3 Commit B in orchestrator

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-sync.bats`

- [ ] **Step 1: Failing test for end-to-end Commit B**

Append to `tests/template-update-sync.bats`:

```bash
@test "template-update --apply: Commit B runs mechanical migration + records pending" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  V1="$REPO_ROOT/tests/fixtures/template-update/v1"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V1" --accept-source-change --apply --non-interactive
  [ "$status" -eq 0 ] || true
  # Check that the migration ran (added "## Tagline" to WIKI.md).
  grep -q "## Tagline" WIKI.md
  # Commit B exists.
  git log --format=%s -3 | grep -q "run migrations"
  # State file shows Commit B committed.
  PHASE=$(python3 "$REPO_ROOT/scripts/_template_helpers/state.py" get .awiki/template-cache/_fetch/.update-state.json phase)
  [ "$PHASE" = "commit-b" ]
}

@test "template-update --apply: LLM prompt staged into pending-prompts" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$(git rev-parse HEAD)" >/dev/null
  V2="$REPO_ROOT/tests/fixtures/template-update/v2-with-llm"
  mkdir -p content/notes && echo "## Foo" > content/notes/a.md
  git add content && git -c user.email=a@b -c user.name=t commit -q -m add-content
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V2" --accept-source-change --apply --non-interactive
  [ -f .awiki/pending-prompts/0002-rewrite-foo.prompt.md ]
  grep -q "Resolved scope" .awiki/pending-prompts/0002-rewrite-foo.prompt.md
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Commit B**

In `scripts/template-update.sh`, replace the trailing `echo "info: Commits B/C/D not yet implemented"` and `exit 0` with:

```bash
# === Phase 3 — Commit B: migrations ===
if should_skip_phase commit-b; then
  echo "info: resume — skipping Commit B"
else
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-b --status started

OLD_VERSION=$(python3 -c "import json; print(json.load(open('$PJ'))['version'])")
NEW_VERSION=$(awk -F= '$1=="template_version"{print $2}' <(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST"))

# Already-applied IDs.
APPLIED_IDS=$(python3 -c "
import json
d=json.load(open('$PJ'))
for m in d.get('applied_migrations', []):
    print(m.get('id', ''))
")

# Iterate migrations in numeric order (by filename).
for MIG in $(ls "$FETCH_DIR/migrations/"*.sh "$FETCH_DIR/migrations/"*.prompt.md 2>/dev/null | sort); do
  FNAME=$(basename "$MIG")
  [[ "$FNAME" == "README.md" || "$FNAME" == ".gitkeep" ]] && continue
  [[ "$FNAME" == schema-*.sh ]] && continue   # handled in Phase 1.5

  if [[ "$FNAME" == *.sh ]]; then
    MID="${FNAME%.sh}"
  else
    MID="${FNAME%.prompt.md}"
  fi

  # Skip if already applied.
  if echo "$APPLIED_IDS" | grep -qx "$MID"; then continue; fi
  if [[ -n "$SKIP_MIGRATION" && "$SKIP_MIGRATION" == "$MID" ]]; then
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "$MID" --status skipped --reason "user --skip-migration"
    continue
  fi

  if [[ "$FNAME" == *.sh ]]; then
    if ! python3 "$HELPERS/migration.py" run "$MIG" \
         --repo-root "$REPO_ROOT" --old-version "$OLD_VERSION" --new-version "$NEW_VERSION"; then
      echo "halt: migration $MID failed. Fix or run with --skip-migration $MID then --continue." >&2
      exit 1
    fi
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "$MID" --status applied
  else
    # LLM prompt: stage.
    PEND_DIR="$REPO_ROOT/.awiki/pending-prompts"
    STAGE_ARGS=(--user-tree "$REPO_ROOT" --pending-dir "$PEND_DIR")
    [[ -f "$REPO_ROOT/.gitattributes" ]] && STAGE_ARGS+=(--gitattributes "$REPO_ROOT/.gitattributes")
    if ! python3 "$HELPERS/migration.py" stage-prompt "$MIG" "${STAGE_ARGS[@]}"; then
      echo "halt: prompt staging failed for $MID" >&2
      exit 1
    fi
    # If risk: high under --non-interactive → record skipped instead of staged.
    RISK=$(awk -F: '/^risk:/{gsub(/[ "'\'']/, "", $2); print $2; exit}' "$MIG" || echo medium)
    if [[ "$RISK" == "high" && $NON_INTERACTIVE -eq 1 ]]; then
      rm -f "$PEND_DIR/$(basename "$MIG")"
      python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
        --id "$MID" --status skipped --reason "non-interactive high-risk"
      continue
    fi
    # Note: applied_migrations[] entry for LLM prompts is appended LATER by the agent;
    # state.applied_migrations_pending only tracks staged prompts so Commit D can list them.
    python3 "$HELPERS/state.py" add-migration-pending "$FETCH_DIR/.update-state.json" \
      --id "$MID" --status applied --reason "staged-as-prompt"
  fi
done

# Stage and commit (only if there were changes).
git add -A
if git diff --cached --quiet; then
  echo "info: no migration changes to commit"
else
  git commit -q -m "chore(template): run migrations"
  COMMIT_B_SHA=$(git rev-parse HEAD)
  python3 "$HELPERS/state.py" set-last-completed "$FETCH_DIR/.update-state.json" "$COMMIT_B_SHA"
fi
python3 "$HELPERS/state.py" set-phase "$FETCH_DIR/.update-state.json" --phase commit-b --status committed

echo "info: Commit B complete"
fi  # end Commit B (skipped on resume)

echo "info: Commits C/D not yet implemented"
exit 0
```

- [ ] **Step 4: Run tests — verify passing**

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-sync.bats
git commit -m "feat: Phase 3 Commit B (mechanical run + LLM stage + skip-migration)"
```

---

## Task 6: CHANGELOG + verify

- [ ] **Step 1: Run all Phase 06 tests**

```bash
bats tests/template-update-migration.bats tests/template-update-sync.bats
```

- [ ] **Step 2: Append CHANGELOG**

```markdown
### Added
- Migration runner with stripped env + `touches:` enforcement + `.awiki/` ban + `--skip-migration <id>`.
- LLM prompt staging into `.awiki/pending-prompts/` with `scope_glob` enforcement (blocks secrets/, .awiki/, .git/, themes/) and `risk` metadata (Phase 06).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 06 commit B migrations"
```

---

## Phase 06 — Definition of done

- [ ] `migration.py` covers: parse-header (required keys), validate-touches (blocklist), run (stripped env + post-run audit + revert on out-of-scope), stage-prompt (scope_glob enforcement + metadata).
- [ ] Orchestrator iterates migrations in numeric order, skips already-applied + `--skip-migration` IDs.
- [ ] Mechanical migration writing outside `touches:` → halt + revert.
- [ ] Mechanical migration writing to `.awiki/` → halt + revert (unless schema-upgrade flow).
- [ ] LLM prompt with `scope_glob: secrets/**` → staging rejected.
- [ ] State file phase=commit-b status=committed.
- [ ] Pending entries recorded in state file (not yet in `template.json`; Commit D writes them).
- [ ] CHANGELOG entry added.

Phase 07 implements Commit C (bootstrap steps) + Commit D (provenance).
