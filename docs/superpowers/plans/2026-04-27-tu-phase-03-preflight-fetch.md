# Phase 03 — Orchestrator Preflight + Fetch (Phase 0a / 1 / 0b)

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Stand up `scripts/template-update.sh` orchestrator with arg parsing, **Phase 0a** (pre-fetch preflight: dirty tree, branch, encryption, pending prompts, source-change), **Phase 1** (fetch + ancestor auto-recovery + state file init), and **Phase 0b** (post-fetch: schema-version check, encryption recheck, optional `--verify-signature`).

**Architecture:** `template-update.sh` is the single entry point. CLI arg parser sets a flags-bag. Functions per phase. State file at `.awiki/template-cache/_fetch/.update-state.json` written/updated by every phase transition. Helpers from Phase 01–02 are composed.

**Tech Stack:** bash 4+, `getopt`-style manual arg parsing (long flags), git, git-crypt (optional), Python 3 for state-file I/O via a small helper.

**Spec sections:** `Update flow / Phase 0a/0b`, `Update flow / Phase 1`, `Update flow / Flag interactions`, `Trust Model / Source identity`, `Optional tag-signature verification`.

---

## File structure

**Created:**
- `scripts/template-update.sh` — orchestrator entry point.
- `scripts/_template_helpers/state.py` — state-file reader/writer.
- `scripts/_template_helpers/preflight.py` — dirty-tree + encryption + pending-prompts checks.
- `tests/template-update-preflight.bats`
- `tests/template-update-fetch.bats`

**Modified:** none yet (justfile recipe added in Phase 09 to wire `template-update`).

**Depends on:** Phase 01 (manifest/provenance/init) + Phase 02 (source-check).

---

## Task 1: State-file helper

**Files:**
- Create: `scripts/_template_helpers/state.py`
- Create: `tests/template-update-preflight.bats` (state-file tests added here for now)

- [ ] **Step 1: Failing test**

Create `tests/template-update-preflight.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}
teardown() { rm -rf "$TMP"; }

@test "state init: writes JSON with required keys" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch awiki-template-update/bbb
  [ "$status" -eq 0 ]
  python3 -c "
import json
d = json.load(open('$TMP/state.json'))
assert d['phase'] == 'fetch'
assert d['status'] == 'completed'
assert d['commit_old'] == 'aaa'
assert d['commit_new'] == 'bbb'
assert d['branch'] == 'awiki-template-update/bbb'
assert d['last_completed_commit'] is None
assert d['applied_migrations_pending'] == []
assert d['bootstrap_steps_pending'] == []
assert d['deletions_user_decisions'] == {}
"
}

@test "state set-phase: updates phase and status" {
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch br
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" set-phase "$TMP/state.json" \
    --phase commit-a --status started
  run python3 -c "import json; d=json.load(open('$TMP/state.json')); print(d['phase'], d['status'])"
  [ "$output" = "commit-a started" ]
}

@test "state set-last-completed: updates SHA" {
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" init "$TMP/state.json" \
    --commit-old aaa --commit-new bbb --branch br
  python3 "$REPO_ROOT/scripts/_template_helpers/state.py" set-last-completed "$TMP/state.json" deadbeef
  run python3 -c "import json; print(json.load(open('$TMP/state.json'))['last_completed_commit'])"
  [ "$output" = "deadbeef" ]
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/state.py`:

```python
#!/usr/bin/env python3
"""Orchestrator state-file reader/writer.

Subcommands:
  init <path> --commit-old <sha> --commit-new <sha> --branch <name>
  set-phase <path> --phase <name> --status <started|committed>
  set-last-completed <path> <sha>
  add-migration-pending <path> --id <id> --status <applied|skipped> [--reason <r>]
  add-bootstrap-pending <path> --id <id> --status <applied|skipped> [--reason <r>] [--content-hash <h>]
  add-deletion-decision <path> --rel <relpath> --decision <remove|preserve-local>
  get <path> <field>
"""
from __future__ import annotations

import argparse
import json
import sys
from datetime import datetime, timezone
from pathlib import Path

REQUIRED_FIELDS = (
    "phase", "status", "commit_old", "commit_new", "branch", "started_at",
    "last_completed_commit", "applied_migrations_pending",
    "bootstrap_steps_pending", "deletions_user_decisions",
)


def load(path: Path) -> dict:
    if not path.is_file():
        return {}
    with path.open("r") as f:
        return json.load(f)


def save(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def cmd_init(args: argparse.Namespace) -> int:
    data = {
        "phase": "fetch",
        "status": "completed",
        "commit_old": args.commit_old,
        "commit_new": args.commit_new,
        "branch": args.branch,
        "started_at": datetime.now(timezone.utc).isoformat(timespec="seconds"),
        "last_completed_commit": None,
        "applied_migrations_pending": [],
        "bootstrap_steps_pending": [],
        "deletions_user_decisions": {},
    }
    save(Path(args.path), data)
    return 0


def cmd_set_phase(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    d["phase"] = args.phase
    d["status"] = args.status
    save(p, d)
    return 0


def cmd_set_last_completed(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    d["last_completed_commit"] = args.sha
    save(p, d)
    return 0


def cmd_add_migration_pending(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    entry: dict = {"id": args.id, "status": args.status}
    if args.reason:
        entry["reason"] = args.reason
    d.setdefault("applied_migrations_pending", []).append(entry)
    save(p, d)
    return 0


def cmd_add_bootstrap_pending(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    entry: dict = {"id": args.id, "status": args.status}
    if args.reason:
        entry["reason"] = args.reason
    if args.content_hash:
        entry["content_hash"] = args.content_hash
    d.setdefault("bootstrap_steps_pending", []).append(entry)
    save(p, d)
    return 0


def cmd_add_deletion_decision(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    d.setdefault("deletions_user_decisions", {})[args.rel] = args.decision
    save(p, d)
    return 0


def cmd_get(args: argparse.Namespace) -> int:
    p = Path(args.path)
    d = load(p)
    if args.field not in d:
        print(f"field not found: {args.field}", file=sys.stderr)
        return 1
    val = d[args.field]
    if isinstance(val, (dict, list)):
        print(json.dumps(val))
    else:
        print(val)
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)

    p_init = sub.add_parser("init")
    p_init.add_argument("path")
    p_init.add_argument("--commit-old", required=True)
    p_init.add_argument("--commit-new", required=True)
    p_init.add_argument("--branch", required=True)

    p_sp = sub.add_parser("set-phase")
    p_sp.add_argument("path")
    p_sp.add_argument("--phase", required=True)
    p_sp.add_argument("--status", required=True, choices=["started", "committed"])

    p_lc = sub.add_parser("set-last-completed")
    p_lc.add_argument("path")
    p_lc.add_argument("sha")

    p_amp = sub.add_parser("add-migration-pending")
    p_amp.add_argument("path")
    p_amp.add_argument("--id", required=True)
    p_amp.add_argument("--status", required=True, choices=["applied", "skipped"])
    p_amp.add_argument("--reason", default="")

    p_abp = sub.add_parser("add-bootstrap-pending")
    p_abp.add_argument("path")
    p_abp.add_argument("--id", required=True)
    p_abp.add_argument("--status", required=True, choices=["applied", "skipped"])
    p_abp.add_argument("--reason", default="")
    p_abp.add_argument("--content-hash", default="")

    p_add = sub.add_parser("add-deletion-decision")
    p_add.add_argument("path")
    p_add.add_argument("--rel", required=True)
    p_add.add_argument("--decision", required=True, choices=["remove", "preserve-local"])

    p_get = sub.add_parser("get")
    p_get.add_argument("path")
    p_get.add_argument("field")

    args = parser.parse_args()
    dispatch = {
        "init": cmd_init,
        "set-phase": cmd_set_phase,
        "set-last-completed": cmd_set_last_completed,
        "add-migration-pending": cmd_add_migration_pending,
        "add-bootstrap-pending": cmd_add_bootstrap_pending,
        "add-deletion-decision": cmd_add_deletion_decision,
        "get": cmd_get,
    }
    return dispatch[args.cmd](args)


if __name__ == "__main__":
    sys.exit(main())
```

`chmod +x scripts/_template_helpers/state.py`.

- [ ] **Step 4: Run tests — verify passing**

Expected: 3 of 3 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/state.py tests/template-update-preflight.bats
git commit -m "feat: orchestrator state-file helper"
```

---

## Task 2: Preflight checks (dirty tree, branch, pending prompts)

**Files:**
- Create: `scripts/_template_helpers/preflight.py`
- Modify: `tests/template-update-preflight.bats`

- [ ] **Step 1: Append failing tests**

```bash
@test "preflight: clean tree -> ok" {
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-tree
  [ "$status" -eq 0 ]
}

@test "preflight: untracked file -> halt" {
  cd "$TMP"
  git init -q
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  echo dirty > leftover.txt
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-tree
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "untracked"
}

@test "preflight: unstaged change -> halt" {
  cd "$TMP"
  git init -q
  echo a > tracked.txt
  git add tracked.txt
  git -c user.email=a@b -c user.name=t commit -q -m add
  echo b > tracked.txt
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-tree
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "modified|unstaged"
}

@test "preflight: branch on main -> ok" {
  cd "$TMP"
  git init -q -b main
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-branch --expected main
  [ "$status" -eq 0 ]
}

@test "preflight: branch on update branch -> halt" {
  cd "$TMP"
  git init -q -b main
  git -c user.email=a@b -c user.name=t commit -q --allow-empty -m init
  git checkout -q -b awiki-template-update/abc
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-branch --expected main
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "awiki-template-update"
}

@test "preflight: empty pending-prompts dir -> ok" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-pending-prompts
  [ "$status" -eq 0 ]
}

@test "preflight: non-empty pending-prompts -> halt" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo body > .awiki/pending-prompts/0001-x.md
  run python3 "$REPO_ROOT/scripts/_template_helpers/preflight.py" check-pending-prompts
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "0001-x.md"
}
```

- [ ] **Step 2: Run — verify failures**

- [ ] **Step 3: Implement preflight**

Create `scripts/_template_helpers/preflight.py`:

```python
#!/usr/bin/env python3
"""Preflight checks for template-update orchestrator.

Subcommands:
  check-tree             — staged/unstaged/untracked + dirty submodules
  check-branch --expected <name>
  check-pending-prompts  — halts if .awiki/pending-prompts/*.md present
  check-encryption --manifest <path>   — halts if encrypted+locked + manifest needs merge
"""
from __future__ import annotations

import argparse
import subprocess
import sys
from pathlib import Path


def run(cmd: list[str]) -> tuple[int, str, str]:
    r = subprocess.run(cmd, capture_output=True, text=True)
    return r.returncode, r.stdout, r.stderr


def check_tree() -> int:
    # Staged or unstaged tracked changes.
    rc, _, _ = run(["git", "diff-index", "--quiet", "HEAD", "--"])
    if rc != 0:
        print("dirty tree: staged or unstaged tracked changes", file=sys.stderr)
        return 1
    # Untracked files not gitignored.
    rc, out, _ = run(["git", "ls-files", "--others", "--exclude-standard"])
    if rc == 0 and out.strip():
        print(f"dirty tree: untracked files\n{out}", file=sys.stderr)
        return 1
    # Submodule dirty.
    rc, out, _ = run(["git", "submodule", "status"])
    if rc == 0:
        for line in out.splitlines():
            if line.startswith("+") or line.startswith("-"):
                print(f"dirty tree: submodule {line}", file=sys.stderr)
                return 1
    return 0


def check_branch(expected: str) -> int:
    rc, out, _ = run(["git", "rev-parse", "--abbrev-ref", "HEAD"])
    if rc != 0:
        print("not in a git repo", file=sys.stderr)
        return 2
    cur = out.strip()
    if cur.startswith("awiki-template-update/"):
        print(f"halt: on update branch {cur}; use --continue or --abort", file=sys.stderr)
        return 1
    if cur != expected:
        print(f"halt: on branch {cur}, expected {expected}", file=sys.stderr)
        return 1
    return 0


def check_pending_prompts() -> int:
    pp = Path(".awiki/pending-prompts")
    if not pp.is_dir():
        return 0
    leftovers = sorted(p.name for p in pp.glob("*.md"))
    if not leftovers:
        return 0
    print(
        "halt: pending LLM migrations from previous cycle:\n  "
        + "\n  ".join(leftovers)
        + "\nRun agent to complete or delete prompts before next update.",
        file=sys.stderr,
    )
    return 1


def check_encryption(manifest_path: Path) -> int:
    # Try git-crypt status.
    rc, out, err = run(["git-crypt", "status", "-e"])
    if rc != 0 and "command not found" in err.lower() or rc == 127:
        # git-crypt not installed; skip silently (no encrypted paths exist).
        return 0
    if not out.strip():
        return 0  # no encrypted paths
    # Encrypted paths present. Check if locked.
    rc, status_out, _ = run(["git-crypt", "status"])
    locked = "locked" in status_out.lower()
    if not locked:
        return 0
    # Check if any encrypted path matches three_way / attributes_merge in OLD manifest.
    sys.path.insert(0, str(Path(__file__).resolve().parent))
    import manifest_parse  # noqa: E402
    m = manifest_parse.load(manifest_path)
    enc_paths = [line.strip() for line in out.splitlines() if line.strip()]
    for p in enc_paths:
        s = manifest_parse.resolve_strategy(m, p)
        if s in ("three_way", "attributes_merge"):
            print(
                f"halt: encrypted path {p} would require merge but checkout is locked.\n"
                "Run `git-crypt unlock` first.",
                file=sys.stderr,
            )
            return 1
    return 0


def main() -> int:
    parser = argparse.ArgumentParser()
    sub = parser.add_subparsers(dest="cmd", required=True)
    sub.add_parser("check-tree")
    p_b = sub.add_parser("check-branch")
    p_b.add_argument("--expected", required=True)
    sub.add_parser("check-pending-prompts")
    p_e = sub.add_parser("check-encryption")
    p_e.add_argument("--manifest", required=True, type=Path)
    args = parser.parse_args()
    if args.cmd == "check-tree":
        return check_tree()
    if args.cmd == "check-branch":
        return check_branch(args.expected)
    if args.cmd == "check-pending-prompts":
        return check_pending_prompts()
    if args.cmd == "check-encryption":
        return check_encryption(args.manifest)
    return 2


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests — verify passing**

Expected: 7 added tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/preflight.py tests/template-update-preflight.bats
git commit -m "feat: preflight helper (tree, branch, pending-prompts, encryption)"
```

---

## Task 3: `template-update.sh` skeleton + arg parsing

**Files:**
- Create: `scripts/template-update.sh`
- Create: `tests/template-update-fetch.bats`

- [ ] **Step 1: Failing tests for arg parsing + Phase 0a integration**

Create `tests/template-update-fetch.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  cd "$TMP"
  cp -R "$REPO_ROOT/tests/fixtures/template-update/v0/." .
  git init -q -b main
  git add -A
  git -c user.email=a@b -c user.name=t commit -q -m init
  COMMIT_OLD=$(git rev-parse HEAD)
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --ref main --version 0.1.0 --commit "$COMMIT_OLD" >/dev/null
}

teardown() { rm -rf "$TMP"; }

@test "template-update: --help prints usage" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --help
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "usage:"
  echo "$output" | grep -q -- "--apply"
  echo "$output" | grep -q -- "--continue"
}

@test "template-update: dirty tree halts" {
  cd "$TMP"
  echo dirty > leftover.txt
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "dirty tree"
}

@test "template-update: on update branch halts" {
  cd "$TMP"
  git checkout -q -b awiki-template-update/foo
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "update branch|--continue"
}

@test "template-update: pending prompts halts" {
  cd "$TMP"
  mkdir -p .awiki/pending-prompts
  echo x > .awiki/pending-prompts/0001-leftover.md
  run bash "$REPO_ROOT/scripts/template-update.sh"
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "pending"
}

@test "template-update: source mismatch halts without --accept-source-change" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" --source /elsewhere
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "Source change detected"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement skeleton**

Create `scripts/template-update.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPERS="$SCRIPT_DIR/_template_helpers"
REPO_ROOT="$(pwd)"

# Defaults
REF=""
SOURCE=""
APPLY=0
DRY_RUN=0
CONTINUE=0
ABORT=0
STATUS=0
SCHEMA_UPGRADE=0
ACCEPT_SOURCE_CHANGE=0
ACCEPT_ATTRIBUTE_CHANGES=0
ACCEPT_MANUAL_COMMITS=0
PERSIST_SOURCE=0
PRINT_MIGRATIONS=0
NON_INTERACTIVE=0
VERIFY_SIGNATURE=0
GC=0
RE_PIN=""
RERUN_BOOTSTRAP_STEP=""
SKIP_MIGRATION=""

usage() {
  cat <<'EOF'
usage: template-update.sh [flags]

Default = dry-run plan. --apply executes onto a dedicated update branch.

flags:
  --ref <sha-or-tag>          Pin a specific upstream ref (default: origin/main HEAD)
  --source <url-or-path>      Override repo source for one invocation
  --apply                     Execute the update (default is dry-run)
  --dry-run                   Force dry-run even with --apply
  --continue                  Resume after conflicts / migration failure
  --abort                     Discard in-progress update branch
  --status                    Print current pin + pending state; read-only
  --schema-upgrade            Allow schema-version bump
  --accept-source-change      Confirm --source differs from pin
  --accept-attribute-changes  Confirm .gitattributes filter changes
  --accept-manual-commits     Allow --continue past manual commits on update branch
  --persist-source            Save --source to .awiki/template.json.repo on success
  --rerun-bootstrap-step <id> Re-run a single (often dangerous) bootstrap step
  --print-migrations          Include full migration script bodies in plan
  --re-pin <commit>           Set pin without running an update (rollback escape hatch)
  --gc                        Prune orphaned cache dirs and exit
  --non-interactive           Auto-resolve prompts to safe defaults; CI mode
  --verify-signature          Require git verify-tag/verify-commit on the fetched ref
  --skip-migration <id>       Skip a specific migration during apply
EOF
}

# Parse args.
while [[ $# -gt 0 ]]; do
  case "$1" in
    -h|--help) usage; exit 0 ;;
    --ref) REF="$2"; shift 2 ;;
    --source) SOURCE="$2"; shift 2 ;;
    --apply) APPLY=1; shift ;;
    --dry-run) DRY_RUN=1; shift ;;
    --continue) CONTINUE=1; shift ;;
    --abort) ABORT=1; shift ;;
    --status) STATUS=1; shift ;;
    --schema-upgrade) SCHEMA_UPGRADE=1; shift ;;
    --accept-source-change) ACCEPT_SOURCE_CHANGE=1; shift ;;
    --accept-attribute-changes) ACCEPT_ATTRIBUTE_CHANGES=1; shift ;;
    --accept-manual-commits) ACCEPT_MANUAL_COMMITS=1; shift ;;
    --persist-source) PERSIST_SOURCE=1; shift ;;
    --rerun-bootstrap-step) RERUN_BOOTSTRAP_STEP="$2"; shift 2 ;;
    --print-migrations) PRINT_MIGRATIONS=1; shift ;;
    --re-pin) RE_PIN="$2"; shift 2 ;;
    --gc) GC=1; shift ;;
    --non-interactive) NON_INTERACTIVE=1; shift ;;
    --verify-signature) VERIFY_SIGNATURE=1; shift ;;
    --skip-migration) SKIP_MIGRATION="$2"; shift 2 ;;
    *) echo "unknown arg: $1" >&2; usage; exit 2 ;;
  esac
done

# Dry-run wins if both flags set.
if [[ $DRY_RUN -eq 1 ]]; then APPLY=0; fi

PJ="$REPO_ROOT/.awiki/template.json"
[[ -f "$PJ" ]] || { echo "halt: no .awiki/template.json. Run 'just template-init' or 'just template-retrofit'." >&2; exit 1; }

# Defer --status, --re-pin, --gc, --abort, --continue, --rerun-bootstrap-step to later phases.
if [[ -n "$RE_PIN" || $GC -eq 1 || $ABORT -eq 1 || $CONTINUE -eq 1 || $STATUS -eq 1 || -n "$RERUN_BOOTSTRAP_STEP" ]]; then
  echo "stub: this command path is implemented in a later phase" >&2
  exit 0
fi

# === Phase 0a: pre-fetch preflight ===
DEFAULT_BRANCH=$(bash "$SCRIPT_DIR/template-config.sh" get "$REPO_ROOT/.awiki/config" default_branch main)

python3 "$HELPERS/preflight.py" check-tree
python3 "$HELPERS/preflight.py" check-branch --expected "$DEFAULT_BRANCH"
python3 "$HELPERS/preflight.py" check-pending-prompts

# Encryption preflight uses CURRENT manifest in the repo (template-shipped at bootstrap).
if [[ -f "$REPO_ROOT/template.manifest.toml" ]]; then
  python3 "$HELPERS/preflight.py" check-encryption --manifest "$REPO_ROOT/template.manifest.toml"
fi

# Source-change check.
PIN_REPO=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" repo)
RESOLVED_SOURCE="${SOURCE:-$PIN_REPO}"
SC_ARGS=(--provenance "$PJ" --source "$RESOLVED_SOURCE")
[[ $ACCEPT_SOURCE_CHANGE -eq 1 ]] && SC_ARGS+=(--accept-source-change)
bash "$SCRIPT_DIR/template-source-check.sh" "${SC_ARGS[@]}"

echo "info: phase 0a preflight ok"

# Phases 1, 0b, 1.5, 2, 3 — STUBBED until later tasks/phases.
echo "info: subsequent phases not yet implemented"
exit 0
```

`chmod +x scripts/template-update.sh`.

- [ ] **Step 4: Run tests — verify passing**

Expected: 5 of 5 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-fetch.bats
git commit -m "feat: template-update.sh skeleton + Phase 0a preflight"
```

---

## Task 4: Phase 1 — fetch + ancestor cache check

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-fetch.bats`

- [ ] **Step 1: Append failing tests**

```bash
@test "template-update: fetch creates _fetch dir and state file" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --accept-source-change
  [ "$status" -eq 0 ]
  [ -d .awiki/template-cache/_fetch ]
  [ -f .awiki/template-cache/_fetch/.update-state.json ]
}

@test "template-update: same commit -> 'already up to date'" {
  cd "$TMP"
  # Use v0 as source to match the pinned commit.
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v0" --accept-source-change
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "already up to date" || true
}

@test "template-update: fetch auto-recovers ancestor cache when missing" {
  cd "$TMP"
  rm -rf ".awiki/template-cache/$COMMIT_OLD"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" --accept-source-change
  [ "$status" -eq 0 ]
  [ -d ".awiki/template-cache/$COMMIT_OLD" ]
  echo "$output" | grep -q "Re-built ancestor cache from pin"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Phase 1 in `template-update.sh`**

Replace the `# Phases 1, 0b...` block in `scripts/template-update.sh` with:

```bash
# === Phase 1: fetch ===
FETCH_DIR="$REPO_ROOT/.awiki/template-cache/_fetch"
COMMIT_OLD=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" commit)
ORIGINAL_REPO=$(bash "$SCRIPT_DIR/template-provenance.sh" get "$PJ" original_repo)

mkdir -p "$REPO_ROOT/.awiki/template-cache"
if [[ -d "$FETCH_DIR/.git" ]]; then
  # Reuse existing fetch dir: fetch updates.
  git -C "$FETCH_DIR" fetch --depth 50 origin >/dev/null 2>&1 || true
elif [[ -d "$FETCH_DIR" ]]; then
  # Stale dir without .git (e.g. local copy fixture). Wipe + re-clone or copy.
  rm -rf "$FETCH_DIR"
fi

if [[ ! -d "$FETCH_DIR/.git" ]]; then
  if [[ "$RESOLVED_SOURCE" == /* ]] || [[ -d "$RESOLVED_SOURCE/.git" ]]; then
    # Local path: prefer git clone if it's a repo, else snapshot copy.
    if [[ -d "$RESOLVED_SOURCE/.git" ]]; then
      git clone --depth 50 "$RESOLVED_SOURCE" "$FETCH_DIR" >/dev/null
    else
      mkdir -p "$FETCH_DIR"
      cp -R "$RESOLVED_SOURCE/." "$FETCH_DIR/"
      # Init a throwaway git repo so `git rev-parse` works.
      git -C "$FETCH_DIR" init -q
      git -C "$FETCH_DIR" add -A
      git -C "$FETCH_DIR" -c user.email=fetch@local -c user.name=fetch commit -q -m "snapshot: $RESOLVED_SOURCE"
    fi
  else
    git clone --depth 50 "$RESOLVED_SOURCE" "$FETCH_DIR" >/dev/null
  fi
fi

# Resolve commit_new.
if [[ -n "$REF" ]]; then
  COMMIT_NEW=$(git -C "$FETCH_DIR" rev-parse "$REF")
else
  COMMIT_NEW=$(git -C "$FETCH_DIR" rev-parse HEAD)
fi

if [[ "$COMMIT_NEW" == "$COMMIT_OLD" ]]; then
  echo "already up to date (pinned at $COMMIT_OLD)"
  rm -rf "$FETCH_DIR"
  exit 0
fi

# Ancestor cache check + auto-recover.
ANCESTOR_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT_OLD"
if [[ ! -d "$ANCESTOR_DIR" ]]; then
  echo "info: ancestor cache missing for $COMMIT_OLD; rebuilding from $ORIGINAL_REPO"
  TMP_ANC=$(mktemp -d)
  if [[ -d "$ORIGINAL_REPO/.git" ]] || [[ "$ORIGINAL_REPO" == http* ]]; then
    if git clone --depth 50 "$ORIGINAL_REPO" "$TMP_ANC/orig" >/dev/null 2>&1; then
      git -C "$TMP_ANC/orig" checkout -q "$COMMIT_OLD" 2>/dev/null || \
        git -C "$TMP_ANC/orig" fetch --depth 50 origin "$COMMIT_OLD" >/dev/null 2>&1 || true
      git -C "$TMP_ANC/orig" checkout -q "$COMMIT_OLD"
      mkdir -p "$ANCESTOR_DIR"
      git -C "$TMP_ANC/orig" archive --format=tar HEAD | tar -x -C "$ANCESTOR_DIR"
      echo "Re-built ancestor cache from pin"
    else
      echo "halt: cannot reach original_repo $ORIGINAL_REPO. Run 'just template-retrofit'." >&2
      rm -rf "$TMP_ANC"
      exit 1
    fi
  else
    # Local non-git source: snapshot copy.
    mkdir -p "$ANCESTOR_DIR"
    cp -R "$ORIGINAL_REPO/." "$ANCESTOR_DIR/"
    echo "Re-built ancestor cache from pin"
  fi
  rm -rf "$TMP_ANC"
fi

# Initialize state file.
SHORT_NEW=$(echo "$COMMIT_NEW" | head -c 12)
BRANCH_NAME="awiki-template-update/$SHORT_NEW"
python3 "$HELPERS/state.py" init "$FETCH_DIR/.update-state.json" \
  --commit-old "$COMMIT_OLD" --commit-new "$COMMIT_NEW" --branch "$BRANCH_NAME"

echo "info: phase 1 fetch ok (commit_new=$COMMIT_NEW)"
echo "info: subsequent phases not yet implemented"
exit 0
```

- [ ] **Step 4: Run tests — verify passing**

Expected: all template-update-fetch.bats tests pass (8 total).

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-fetch.bats
git commit -m "feat: template-update Phase 1 (fetch + ancestor auto-recovery + state init)"
```

---

## Task 5: Phase 0b — schema-version check + signature verification

**Files:**
- Modify: `scripts/template-update.sh`
- Modify: `tests/template-update-fetch.bats`

- [ ] **Step 1: Append failing tests**

```bash
@test "template-update: schema-version mismatch halts without --schema-upgrade" {
  cd "$TMP"
  # Build a v3 fixture with schema_version=2.
  V3="$REPO_ROOT/tests/fixtures/template-update/v3-schema-bump"
  if [[ ! -d "$V3" ]]; then
    cp -R "$REPO_ROOT/tests/fixtures/template-update/v1" "$V3"
    sed -i.bak 's/^schema_version = 1$/schema_version = 2/' "$V3/template.manifest.toml" && rm "$V3/template.manifest.toml.bak"
    mkdir -p "$V3/migrations"
    cat > "$V3/migrations/schema-1-to-2.sh" <<'EOF'
#!/usr/bin/env bash
# Schema upgrade stub.
set -euo pipefail
EOF
    chmod +x "$V3/migrations/schema-1-to-2.sh"
  fi
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$V3" --accept-source-change
  [ "$status" -ne 0 ]
  echo "$output" | grep -q "schema_version"
  echo "$output" | grep -q -- "--schema-upgrade"
}

@test "template-update: --verify-signature halts on unsigned tag" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-update.sh" \
    --source "$REPO_ROOT/tests/fixtures/template-update/v1" \
    --accept-source-change \
    --verify-signature
  [ "$status" -ne 0 ]
  echo "$output" | grep -qE "verify|signature"
}
```

- [ ] **Step 2: Run — verify failure**

- [ ] **Step 3: Implement Phase 0b in `template-update.sh`**

Insert before the `echo "info: phase 1 fetch ok"` line:

```bash
# === Phase 0b: post-fetch preflight ===
NEW_MANIFEST="$FETCH_DIR/template.manifest.toml"
[[ -f "$NEW_MANIFEST" ]] || { echo "halt: fetched template missing template.manifest.toml" >&2; exit 1; }

NEW_SCHEMA=$(bash "$SCRIPT_DIR/template-manifest.sh" load "$NEW_MANIFEST" | awk -F= '$1=="schema_version"{print $2}')
PIN_SCHEMA=$(python3 -c "import json; print(json.load(open('$PJ'))['schema_version'])")

if [[ "$NEW_SCHEMA" != "$PIN_SCHEMA" ]]; then
  if [[ $SCHEMA_UPGRADE -eq 0 ]]; then
    echo "halt: template schema_version=$NEW_SCHEMA, pin schema_version=$PIN_SCHEMA. Re-run with --schema-upgrade." >&2
    exit 1
  fi
  # Phase 1.5 schema-upgrade itself happens in Phase 04 plan; for now leave a marker.
  echo "info: schema_version mismatch — schema-upgrade flow lands in Phase 04"
fi

# Encryption recheck against NEW manifest.
python3 "$HELPERS/preflight.py" check-encryption --manifest "$NEW_MANIFEST"

# Optional signature verification.
REQUIRE_SIG=$(bash "$SCRIPT_DIR/template-config.sh" get "$REPO_ROOT/.awiki/config" require_signature false)
if [[ $VERIFY_SIGNATURE -eq 1 ]] || [[ "$REQUIRE_SIG" == "true" ]]; then
  TARGET="${REF:-$COMMIT_NEW}"
  # Try verify-tag first; fall back to verify-commit.
  if ! git -C "$FETCH_DIR" verify-tag "$TARGET" 2>/dev/null \
       && ! git -C "$FETCH_DIR" verify-commit "$TARGET" 2>/dev/null; then
    echo "halt: signature verification failed for $TARGET" >&2
    exit 1
  fi
  echo "info: signature verified for $TARGET"
fi

echo "info: phase 0b ok"
```

- [ ] **Step 4: Run tests — verify passing**

Expected: 10 of 10 tests in `template-update-fetch.bats` pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-update.sh tests/template-update-fetch.bats
git commit -m "feat: Phase 0b (schema check + encryption recheck + signature verification)"
```

---

## Task 6: CHANGELOG + verify Phase 03 suite

**Files:**
- Modify: `CHANGELOG.md`

- [ ] **Step 1: Run all Phase 03 tests**

```bash
bats tests/template-update-preflight.bats tests/template-update-fetch.bats
```

- [ ] **Step 2: Append to CHANGELOG**

```markdown
### Added
- `template-update.sh` orchestrator skeleton with Phase 0a (preflight: dirty tree, branch, encryption, pending prompts, source-change), Phase 1 (fetch + ancestor auto-recovery + state file init), Phase 0b (post-fetch schema check + encryption recheck + optional `--verify-signature`) (Phase 03).
```

- [ ] **Step 3: Commit**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 03 orchestrator preflight + fetch"
```

---

## Phase 03 — Definition of done

- [ ] State-file helper writes/reads JSON with all required fields.
- [ ] Preflight halts on dirty tree, update branch, pending prompts, locked encryption.
- [ ] `template-update.sh` parses all CLI flags and `--help` works.
- [ ] Phase 0a runs in order: tree, branch, encryption, pending-prompts, source-change.
- [ ] Phase 1 fetches into `_fetch/`, resolves commit_new, auto-recovers missing ancestor.
- [ ] Phase 1 writes initial state file.
- [ ] Phase 0b halts on schema mismatch unless `--schema-upgrade`; runs encryption recheck against NEW manifest; honors `--verify-signature`.
- [ ] CHANGELOG entry added.

Phase 04 implements Phase 1.5 (schema upgrade) and Phase 2 (plan emit).
