# Phase 01 — Manifest, Config, Provenance

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Land the foundational data layer for the template-update system: manifest parser with glob-precedence resolver, `.awiki/config` parser, `template.json` schema + reader/writer, bootstrap-step content_hash util, and `template-init.sh` to seed it all at bootstrap time.

**Architecture:** Bash entry-point scripts wrap small Python helpers for TOML/JSON parsing (Python 3.11+ `tomllib` + `json` stdlib). Each helper is single-responsibility. `template-init.sh` orchestrates: read manifest, capture content_hashes, write `template.json`, snapshot template tree to cache.

**Tech Stack:** bash 4+ (`#!/usr/bin/env bash`, `set -euo pipefail`), Python 3.11+ (`tomllib`, `json`, `hashlib`, `pathlib`), git, sha256sum, BATS for tests.

**Spec sections:** `Manifest schema`, `.awiki/config schema`, `.awiki/template.json schema`, `Bootstrap integration / Step IDs / Content-hash tracking / New BOOTSTRAP step`.

---

## File structure

**Created:**
- `scripts/template-manifest.sh` — bash wrapper exposing manifest queries.
- `scripts/template-config.sh` — bash wrapper for `.awiki/config` parsing.
- `scripts/template-provenance.sh` — bash wrapper for `template.json` reads/writes.
- `scripts/template-init.sh` — bootstrap-time seeding script.
- `scripts/_template_helpers/manifest_parse.py` — TOML manifest parser + glob resolver.
- `scripts/_template_helpers/provenance.py` — `template.json` reader/writer (validates schema_version, key shapes).
- `scripts/_template_helpers/bootstrap_hash.py` — extracts step bodies + computes sha256.
- `template.manifest.toml` — at repo root; ships with template.
- `migrations/` — empty dir with `.gitkeep`.
- `migrations/README.md` — author guide stub (filled in Phase 10).
- `tests/template-manifest.bats`
- `tests/template-config.bats`
- `tests/template-provenance.bats`
- `tests/template-bootstrap-hash.bats`
- `tests/template-init.bats`
- `tests/fixtures/template-update/v0/` — minimal template fixture (manifest + BOOTSTRAP.md + a few files).

**Modified:**
- `.gitignore` — add `.awiki/template-cache/`.
- `justfile` — add `template-init` recipe (referenced by BOOTSTRAP step in Phase 09).

---

## Task 1: Repo setup + fixture skeleton

**Files:**
- Create: `tests/fixtures/template-update/v0/template.manifest.toml`
- Create: `tests/fixtures/template-update/v0/BOOTSTRAP.md`
- Create: `tests/fixtures/template-update/v0/scripts/.gitkeep`
- Create: `tests/fixtures/template-update/v0/migrations/.gitkeep`
- Modify: `.gitignore`

- [ ] **Step 1: Add cache dir to `.gitignore`**

Edit `.gitignore` adding:

```gitignore
# template-update cache (gitignored; rebuildable from template.json pin)
.awiki/template-cache/
```

- [ ] **Step 2: Create v0 fixture manifest**

Write `tests/fixtures/template-update/v0/template.manifest.toml`:

```toml
schema_version = 1
template_version = "0.1.0"

[strategies]
overwrite        = ["scripts/**", "BOOTSTRAP.md"]
preserve         = ["content/**", "raw/**"]
three_way        = ["WIKI.md", "hugo.toml"]
attributes_merge = [".gitattributes"]
template_only    = ["template.manifest.toml", "migrations/**"]

[new_file_default]
strategy = "prompt"

[bootstrap]
ordered_steps = ["dep-check", "domain", "stage-commit"]

[bootstrap.dangerous]
ids = []
```

- [ ] **Step 3: Create v0 fixture BOOTSTRAP.md with step markers**

Write `tests/fixtures/template-update/v0/BOOTSTRAP.md`:

```markdown
# Bootstrap

### Step 0. Dependency check
<!-- bootstrap-step: dep-check -->
Run `bash scripts/check-deps.sh`. Halt on missing required tools.

### Step 1. Domain
<!-- bootstrap-step: domain -->
Ask user: which domain (personal | research | book | business | other)?

### Step 11. Stage initial commit
<!-- bootstrap-step: stage-commit -->
`git add . && git commit -m "init: bootstrapped from awiki vX.Y.Z"`.
```

- [ ] **Step 4: Create empty subdirs in fixture**

```bash
mkdir -p tests/fixtures/template-update/v0/scripts tests/fixtures/template-update/v0/migrations
touch tests/fixtures/template-update/v0/scripts/.gitkeep tests/fixtures/template-update/v0/migrations/.gitkeep
```

- [ ] **Step 5: Commit**

```bash
git add .gitignore tests/fixtures/template-update/v0/
git commit -m "test: phase 01 — add v0 template fixture"
```

---

## Task 2: Manifest parser — load and dump

**Files:**
- Create: `scripts/_template_helpers/manifest_parse.py`
- Create: `scripts/template-manifest.sh`
- Test: `tests/template-manifest.bats`

- [ ] **Step 1: Write the failing test**

Write `tests/template-manifest.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
}

@test "manifest load: emits schema_version" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE 'schema_version=1'
}

@test "manifest load: emits template_version" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE 'template_version=0\.1\.0'
}

@test "manifest load: missing file -> nonzero exit" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" load /nonexistent.toml
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run test — verify failure**

```bash
bats tests/template-manifest.bats
```

Expected: 3 tests fail (script does not exist).

- [ ] **Step 3: Write helper**

Create `scripts/_template_helpers/manifest_parse.py`:

```python
#!/usr/bin/env python3
"""Manifest parser for template.manifest.toml.

Subcommands:
  load <path>             — emit shell-sourceable KEY=value
  resolve <path> <relpath> — print resolved strategy for relpath
  bootstrap-ids <path>    — print one ID per line in declared order
  dangerous-ids <path>    — print one ID per line
  has-glob-overlap <path> — exit 1 if same-strategy duplicate globs exist
"""
from __future__ import annotations

import sys
import tomllib
from pathlib import Path


def load(path: Path) -> dict:
    with path.open("rb") as f:
        return tomllib.load(f)


def cmd_load(path: Path) -> int:
    m = load(path)
    print(f"schema_version={m.get('schema_version', 0)}")
    print(f"template_version={m.get('template_version', '')}")
    nfd = m.get("new_file_default", {}).get("strategy", "prompt")
    print(f"new_file_default={nfd}")
    return 0


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: manifest_parse.py <subcmd> <path> [args...]", file=sys.stderr)
        return 2
    subcmd, path_arg = sys.argv[1], Path(sys.argv[2])
    if not path_arg.is_file():
        print(f"manifest not found: {path_arg}", file=sys.stderr)
        return 1
    if subcmd == "load":
        return cmd_load(path_arg)
    print(f"unknown subcmd: {subcmd}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Write bash wrapper**

Create `scripts/template-manifest.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
HELPER="$SCRIPT_DIR/_template_helpers/manifest_parse.py"

exec python3 "$HELPER" "$@"
```

`chmod +x scripts/template-manifest.sh scripts/_template_helpers/manifest_parse.py`.

- [ ] **Step 5: Run test — verify passing**

```bash
bats tests/template-manifest.bats
```

Expected: 3 of 3 pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/template-manifest.sh scripts/_template_helpers/manifest_parse.py tests/template-manifest.bats
git commit -m "feat: manifest parser (load subcmd) for template.manifest.toml"
```

---

## Task 3: Manifest — glob precedence resolver

**Files:**
- Modify: `scripts/_template_helpers/manifest_parse.py`
- Modify: `tests/template-manifest.bats`

- [ ] **Step 1: Add failing tests for `resolve` subcmd**

Append to `tests/template-manifest.bats`:

```bash
@test "manifest resolve: scripts/foo.sh -> overwrite" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" scripts/foo.sh
  [ "$status" -eq 0 ]
  [ "$output" = "overwrite" ]
}

@test "manifest resolve: WIKI.md -> three_way" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" WIKI.md
  [ "$status" -eq 0 ]
  [ "$output" = "three_way" ]
}

@test "manifest resolve: .gitattributes -> attributes_merge" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" .gitattributes
  [ "$status" -eq 0 ]
  [ "$output" = "attributes_merge" ]
}

@test "manifest resolve: content/notes/foo.md -> preserve" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" content/notes/foo.md
  [ "$status" -eq 0 ]
  [ "$output" = "preserve" ]
}

@test "manifest resolve: unknown.txt -> new_file_default (prompt)" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$FIXTURE/template.manifest.toml" random/unknown.txt
  [ "$status" -eq 0 ]
  [ "$output" = "prompt" ]
}

@test "manifest resolve: most-specific glob wins (content/log.md)" {
  # Create a fixture with overlapping globs.
  TMP=$(mktemp -d)
  cat > "$TMP/manifest.toml" <<'EOF'
schema_version = 1
template_version = "0.0.0"
[strategies]
overwrite = []
preserve = ["content/**"]
three_way = ["content/log.md"]
attributes_merge = []
template_only = []
[new_file_default]
strategy = "prompt"
[bootstrap]
ordered_steps = []
[bootstrap.dangerous]
ids = []
EOF
  run bash "$REPO_ROOT/scripts/template-manifest.sh" resolve "$TMP/manifest.toml" content/log.md
  [ "$status" -eq 0 ]
  [ "$output" = "three_way" ]
  rm -rf "$TMP"
}
```

- [ ] **Step 2: Run tests — verify failure**

```bash
bats tests/template-manifest.bats
```

Expected: 6 new tests fail (`unknown subcmd: resolve`).

- [ ] **Step 3: Implement glob resolver**

Edit `scripts/_template_helpers/manifest_parse.py` — add this above `main()`:

```python
import fnmatch
from typing import Optional, Tuple

STRATEGY_PRECEDENCE = [
    "template_only",
    "attributes_merge",
    "three_way",
    "overwrite",
    "preserve",
]


def _glob_specificity(glob: str) -> Tuple[int, int, int, str]:
    """Return a sort key. Higher tuple = more specific."""
    star_idx = glob.find("*")
    literal_prefix_len = len(glob) if star_idx == -1 else star_idx
    segments = glob.count("/") + 1
    no_double_star = 0 if "**" in glob else 1
    # Negate glob string for lexicographic tie-break (later-declared wins
    # is enforced at iteration order; lex is final fallback).
    return (literal_prefix_len, segments, no_double_star, glob)


def _matches(glob: str, relpath: str) -> bool:
    # fnmatch handles * and ? but not **; expand ** to fnmatch's recursive form.
    # Python's fnmatch does not support **; emulate by removing path-segment boundary.
    if "**" in glob:
        # `a/**/c` matches `a/anything/here/c` AND `a/c`.
        # Replace `/**/` with `/` (zero segments) plus `/*/` patterns aren't easy.
        # Simpler: split on `**` and check prefix/suffix containment.
        parts = glob.split("**")
        if len(parts) == 2:
            prefix, suffix = parts
            prefix = prefix.rstrip("/")
            suffix = suffix.lstrip("/")
            if prefix and not (relpath == prefix or relpath.startswith(prefix + "/")):
                return False
            if suffix and not (relpath == suffix or relpath.endswith("/" + suffix) or fnmatch.fnmatch(relpath, "*" + suffix)):
                return False
            return True
        # Fallback: single ** at end
    return fnmatch.fnmatch(relpath, glob)


def resolve_strategy(manifest: dict, relpath: str) -> str:
    """Return resolved strategy for relpath. Most-specific glob wins; cross-strategy
    ties broken by STRATEGY_PRECEDENCE; same-strategy ties broken by later-declared."""
    strategies = manifest.get("strategies", {})
    candidates: list[Tuple[Tuple, str, int]] = []  # (specificity_key, strategy, declared_idx)

    # Walk in STRATEGY_PRECEDENCE order so that for same specificity the higher-precedence
    # strategy gets a higher idx.
    for strategy in STRATEGY_PRECEDENCE:
        globs = strategies.get(strategy, [])
        for idx, glob in enumerate(globs):
            if _matches(glob, relpath):
                candidates.append((_glob_specificity(glob), strategy, idx))

    if not candidates:
        nfd = manifest.get("new_file_default", {}).get("strategy", "prompt")
        return nfd

    # Sort: highest specificity first; ties broken by STRATEGY_PRECEDENCE position;
    # final fallback: later-declared (higher idx) wins.
    def _sort_key(item):
        spec, strat, idx = item
        precedence = STRATEGY_PRECEDENCE.index(strat)
        # Lower precedence index = higher priority. Negate for descending.
        return (spec, -precedence, idx)

    candidates.sort(key=_sort_key, reverse=True)
    return candidates[0][1]


def cmd_resolve(path: Path, relpath: str) -> int:
    m = load(path)
    print(resolve_strategy(m, relpath))
    return 0
```

In `main()` add:

```python
    if subcmd == "resolve":
        if len(sys.argv) < 4:
            print("usage: resolve <manifest> <relpath>", file=sys.stderr)
            return 2
        return cmd_resolve(path_arg, sys.argv[3])
```

- [ ] **Step 4: Run tests — verify passing**

```bash
bats tests/template-manifest.bats
```

Expected: 9 of 9 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/manifest_parse.py tests/template-manifest.bats
git commit -m "feat: manifest glob precedence resolver"
```

---

## Task 4: Manifest — bootstrap-ids, dangerous-ids, glob-overlap detector

**Files:**
- Modify: `scripts/_template_helpers/manifest_parse.py`
- Modify: `tests/template-manifest.bats`

- [ ] **Step 1: Add failing tests**

Append to `tests/template-manifest.bats`:

```bash
@test "manifest bootstrap-ids: prints in declared order" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" bootstrap-ids "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "dep-check" ]
  [ "${lines[1]}" = "domain" ]
  [ "${lines[2]}" = "stage-commit" ]
}

@test "manifest dangerous-ids: empty for v0 fixture" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" dangerous-ids "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
  [ -z "$output" ]
}

@test "manifest has-glob-overlap: clean fixture -> exit 0" {
  run bash "$REPO_ROOT/scripts/template-manifest.sh" has-glob-overlap "$FIXTURE/template.manifest.toml"
  [ "$status" -eq 0 ]
}

@test "manifest has-glob-overlap: duplicate within strategy -> exit 1" {
  TMP=$(mktemp -d)
  cat > "$TMP/manifest.toml" <<'EOF'
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
  run bash "$REPO_ROOT/scripts/template-manifest.sh" has-glob-overlap "$TMP/manifest.toml"
  [ "$status" -ne 0 ]
  rm -rf "$TMP"
}
```

- [ ] **Step 2: Run tests — verify failure**

```bash
bats tests/template-manifest.bats
```

Expected: 4 new tests fail.

- [ ] **Step 3: Implement subcommands**

Edit `scripts/_template_helpers/manifest_parse.py`:

```python
def cmd_bootstrap_ids(path: Path) -> int:
    m = load(path)
    for sid in m.get("bootstrap", {}).get("ordered_steps", []):
        print(sid)
    return 0


def cmd_dangerous_ids(path: Path) -> int:
    m = load(path)
    for sid in m.get("bootstrap", {}).get("dangerous", {}).get("ids", []):
        print(sid)
    return 0


def cmd_has_glob_overlap(path: Path) -> int:
    m = load(path)
    for strategy, globs in m.get("strategies", {}).items():
        if len(set(globs)) != len(globs):
            seen = set()
            for g in globs:
                if g in seen:
                    print(f"duplicate glob in {strategy}: {g}", file=sys.stderr)
                seen.add(g)
            return 1
    return 0
```

In `main()`, add dispatch entries:

```python
    if subcmd == "bootstrap-ids":
        return cmd_bootstrap_ids(path_arg)
    if subcmd == "dangerous-ids":
        return cmd_dangerous_ids(path_arg)
    if subcmd == "has-glob-overlap":
        return cmd_has_glob_overlap(path_arg)
```

- [ ] **Step 4: Run tests — verify passing**

```bash
bats tests/template-manifest.bats
```

Expected: 13 of 13 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/manifest_parse.py tests/template-manifest.bats
git commit -m "feat: manifest bootstrap-ids, dangerous-ids, has-glob-overlap subcmds"
```

---

## Task 5: `.awiki/config` parser

**Files:**
- Create: `scripts/template-config.sh`
- Create: `tests/template-config.bats`

- [ ] **Step 1: Write the failing tests**

Write `tests/template-config.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
}

teardown() { rm -rf "$TMP"; }

@test "config get: existing key returns value" {
  cat > "$TMP/config" <<EOF
default_branch=main
no_template_check=true
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/config" default_branch
  [ "$status" -eq 0 ]
  [ "$output" = "main" ]
}

@test "config get: missing key returns default" {
  echo "default_branch=main" > "$TMP/config"
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/config" require_signature false
  [ "$status" -eq 0 ]
  [ "$output" = "false" ]
}

@test "config get: missing file returns default" {
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/nonexistent" default_branch fallback
  [ "$status" -eq 0 ]
  [ "$output" = "fallback" ]
}

@test "config get: comments and blank lines ignored" {
  cat > "$TMP/config" <<'EOF'
# this is a comment
default_branch=main

# another comment
require_signature=true
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" get "$TMP/config" require_signature
  [ "$status" -eq 0 ]
  [ "$output" = "true" ]
}

@test "config validate: unknown key warns" {
  cat > "$TMP/config" <<EOF
default_branch=main
unknown_key=foo
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" validate "$TMP/config"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "unknown_key"
}

@test "config validate: malformed line errors" {
  cat > "$TMP/config" <<EOF
default_branch main
EOF
  run bash "$REPO_ROOT/scripts/template-config.sh" validate "$TMP/config"
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run tests — verify failure**

```bash
bats tests/template-config.bats
```

- [ ] **Step 3: Implement**

Create `scripts/template-config.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# Recognized keys (v1 spec).
KNOWN_KEYS=("default_branch" "no_template_check" "require_signature" "ingest_lint_threshold")

usage() {
  cat <<EOF
usage:
  template-config.sh get <path> <key> [default]
  template-config.sh validate <path>
EOF
  exit 2
}

cmd_get() {
  local path="$1" key="$2" default="${3:-}"
  if [[ ! -f "$path" ]]; then
    echo -n "$default"
    echo
    return 0
  fi
  local val
  val=$(awk -F= -v k="$key" '
    !/^#/ && !/^[[:space:]]*$/ && $1==k { sub(/^[^=]*=/, ""); print; exit }
  ' "$path")
  if [[ -z "$val" ]]; then
    echo -n "$default"
  else
    echo -n "$val"
  fi
  echo
}

cmd_validate() {
  local path="$1"
  if [[ ! -f "$path" ]]; then return 0; fi
  local lineno=0
  local rc=0
  while IFS= read -r line; do
    lineno=$((lineno + 1))
    [[ "$line" =~ ^[[:space:]]*$ ]] && continue
    [[ "$line" =~ ^[[:space:]]*# ]] && continue
    if ! [[ "$line" =~ ^[A-Za-z_][A-Za-z0-9_]*=.*$ ]]; then
      echo "config error: malformed line $lineno: $line" >&2
      rc=1
      continue
    fi
    local key="${line%%=*}"
    local known=0
    for k in "${KNOWN_KEYS[@]}"; do
      if [[ "$k" == "$key" ]]; then known=1; break; fi
    done
    if [[ $known -eq 0 ]]; then
      echo "config warning: unknown key $key (line $lineno)" >&2
    fi
  done < "$path"
  return $rc
}

main() {
  if [[ $# -lt 2 ]]; then usage; fi
  local subcmd="$1"; shift
  case "$subcmd" in
    get) cmd_get "$@" ;;
    validate) cmd_validate "$@" ;;
    *) usage ;;
  esac
}

main "$@"
```

- [ ] **Step 4: Run tests — verify passing**

```bash
bats tests/template-config.bats
```

Expected: 6 of 6 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/template-config.sh tests/template-config.bats
git commit -m "feat: .awiki/config parser (get + validate)"
```

---

## Task 6: Bootstrap content_hash util

**Files:**
- Create: `scripts/_template_helpers/bootstrap_hash.py`
- Create: `tests/template-bootstrap-hash.bats`

- [ ] **Step 1: Write the failing test**

Write `tests/template-bootstrap-hash.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
}

@test "bootstrap-hash: computes sha256 for known step" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$FIXTURE/BOOTSTRAP.md" dep-check
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^sha256: ]]
  [ "${#output}" -eq 71 ]   # sha256: + 64 hex
}

@test "bootstrap-hash: same body -> same hash (whitespace-normalized)" {
  TMP=$(mktemp -d)
  cat > "$TMP/a.md" <<'EOF'
### Step 1.
<!-- bootstrap-step: x -->
hello world

EOF
  cat > "$TMP/b.md" <<'EOF'
### Step 1.
<!-- bootstrap-step: x -->
   hello   world
EOF
  HASH_A=$(python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$TMP/a.md" x)
  HASH_B=$(python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$TMP/b.md" x)
  [ "$HASH_A" = "$HASH_B" ]
  rm -rf "$TMP"
}

@test "bootstrap-hash: missing step -> nonzero exit" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" hash "$FIXTURE/BOOTSTRAP.md" nonexistent
  [ "$status" -ne 0 ]
}

@test "bootstrap-hash: list emits all step IDs" {
  run python3 "$REPO_ROOT/scripts/_template_helpers/bootstrap_hash.py" list "$FIXTURE/BOOTSTRAP.md"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "^dep-check$"
  echo "$output" | grep -q "^domain$"
  echo "$output" | grep -q "^stage-commit$"
}
```

- [ ] **Step 2: Run tests — verify failure**

```bash
bats tests/template-bootstrap-hash.bats
```

- [ ] **Step 3: Implement**

Create `scripts/_template_helpers/bootstrap_hash.py`:

```python
#!/usr/bin/env python3
"""Bootstrap step body extractor + sha256 hasher.

Subcommands:
  list <bootstrap-md>        — print all step IDs in order
  hash <bootstrap-md> <id>   — print sha256 of step body (whitespace-normalized)
  body <bootstrap-md> <id>   — print raw step body (for replay)
"""
from __future__ import annotations

import hashlib
import re
import sys
from pathlib import Path

STEP_RE = re.compile(r"<!--\s*bootstrap-step:\s*([a-z0-9-]+)\s*-->")
HEADING_RE = re.compile(r"^###\s+", re.MULTILINE)


def parse(md: str) -> dict[str, str]:
    """Return {id: body} where body is text from the marker to next heading or EOF."""
    out: dict[str, str] = {}
    matches = list(STEP_RE.finditer(md))
    for i, m in enumerate(matches):
        sid = m.group(1)
        start = m.end()
        # End at next ### heading after this marker, or EOF.
        next_heading = HEADING_RE.search(md, start)
        end = next_heading.start() if next_heading else len(md)
        body = md[start:end]
        out[sid] = body
    return out


def list_ids(md_path: Path) -> int:
    md = md_path.read_text(encoding="utf-8")
    matches = STEP_RE.finditer(md)
    for m in matches:
        print(m.group(1))
    return 0


def normalize_whitespace(s: str) -> str:
    # Collapse runs of whitespace, strip leading/trailing.
    return re.sub(r"\s+", " ", s).strip()


def hash_step(md_path: Path, sid: str) -> int:
    md = md_path.read_text(encoding="utf-8")
    bodies = parse(md)
    if sid not in bodies:
        print(f"step not found: {sid}", file=sys.stderr)
        return 1
    norm = normalize_whitespace(bodies[sid])
    h = hashlib.sha256(norm.encode("utf-8")).hexdigest()
    print(f"sha256:{h}")
    return 0


def body_step(md_path: Path, sid: str) -> int:
    md = md_path.read_text(encoding="utf-8")
    bodies = parse(md)
    if sid not in bodies:
        print(f"step not found: {sid}", file=sys.stderr)
        return 1
    sys.stdout.write(bodies[sid])
    return 0


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: bootstrap_hash.py <list|hash|body> <bootstrap-md> [id]", file=sys.stderr)
        return 2
    subcmd = sys.argv[1]
    md_path = Path(sys.argv[2])
    if not md_path.is_file():
        print(f"not a file: {md_path}", file=sys.stderr)
        return 1
    if subcmd == "list":
        return list_ids(md_path)
    if subcmd in ("hash", "body"):
        if len(sys.argv) < 4:
            print(f"usage: bootstrap_hash.py {subcmd} <bootstrap-md> <id>", file=sys.stderr)
            return 2
        sid = sys.argv[3]
        return hash_step(md_path, sid) if subcmd == "hash" else body_step(md_path, sid)
    print(f"unknown subcmd: {subcmd}", file=sys.stderr)
    return 2


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Run tests — verify passing**

```bash
bats tests/template-bootstrap-hash.bats
```

Expected: 4 of 4 pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/_template_helpers/bootstrap_hash.py tests/template-bootstrap-hash.bats
git commit -m "feat: bootstrap step content_hash util (list/hash/body)"
```

---

## Task 7: Provenance helper (`template.json` reader/writer)

**Files:**
- Create: `scripts/_template_helpers/provenance.py`
- Create: `scripts/template-provenance.sh`
- Create: `tests/template-provenance.bats`

- [ ] **Step 1: Write failing tests**

Write `tests/template-provenance.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  TMP=$(mktemp -d)
  PJ="$TMP/template.json"
}

teardown() { rm -rf "$TMP"; }

@test "provenance init: writes valid JSON with required fields" {
  run bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" \
    https://github.com/jmcarbo/awiki main 1.0.0 abc123def456
  [ "$status" -eq 0 ]
  [ -f "$PJ" ]
  python3 -c "import json; d=json.load(open('$PJ')); assert d['repo']=='https://github.com/jmcarbo/awiki'; assert d['original_repo']==d['repo']; assert d['commit']=='abc123def456'"
}

@test "provenance get: reads commit field" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" get "$PJ" commit
  [ "$status" -eq 0 ]
  [ "$output" = "abc123" ]
}

@test "provenance set: updates commit field, original_repo unchanged" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" set "$PJ" commit def456 >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" get "$PJ" commit
  [ "$output" = "def456" ]
  run bash "$REPO_ROOT/scripts/template-provenance.sh" get "$PJ" original_repo
  [ "$output" = "url" ]
}

@test "provenance set original_repo: refused (immutable)" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" set "$PJ" original_repo other
  [ "$status" -ne 0 ]
}

@test "provenance append-migration: adds entry" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-migration "$PJ" 0001-foo applied >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-migration "$PJ" 0002-bar skipped "user --skip-migration" >/dev/null
  run python3 -c "import json; d=json.load(open('$PJ')); print(len(d['applied_migrations']))"
  [ "$output" = "2" ]
}

@test "provenance append-bootstrap-step: adds with content_hash" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-bootstrap-step "$PJ" domain applied "" sha256:deadbeef >/dev/null
  run python3 -c "import json; d=json.load(open('$PJ')); s=d['bootstrap_steps_done'][0]; print(s['id'], s['status'], s['content_hash'])"
  [ "$output" = "domain applied sha256:deadbeef" ]
}

@test "provenance has-step-applied: returns 0 if applied with matching hash" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-bootstrap-step "$PJ" domain applied "" sha256:deadbeef >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" has-step-applied "$PJ" domain sha256:deadbeef
  [ "$status" -eq 0 ]
}

@test "provenance has-step-applied: nonzero if hash differs" {
  bash "$REPO_ROOT/scripts/template-provenance.sh" init "$PJ" url ref ver abc123 >/dev/null
  bash "$REPO_ROOT/scripts/template-provenance.sh" append-bootstrap-step "$PJ" domain applied "" sha256:deadbeef >/dev/null
  run bash "$REPO_ROOT/scripts/template-provenance.sh" has-step-applied "$PJ" domain sha256:00ff
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run tests — verify failure**

- [ ] **Step 3: Implement helper**

Create `scripts/_template_helpers/provenance.py`:

```python
#!/usr/bin/env python3
"""template.json reader/writer with schema validation.

Subcommands:
  init <path> <repo> <ref> <version> <commit>
  get <path> <field>
  set <path> <field> <value>      — refuses original_repo, schema_version
  append-migration <path> <id> <status> [reason]
  append-bootstrap-step <path> <id> <status> [reason] [content_hash]
  has-step-applied <path> <id> <expected_hash>   — exit 0 if applied + hash matches
  list-steps <path>               — emit "id status content_hash" per line
"""
from __future__ import annotations

import json
import sys
from pathlib import Path
from typing import Any

SCHEMA_VERSION = 1
IMMUTABLE_FIELDS = {"original_repo", "schema_version"}


def load(path: Path) -> dict:
    if not path.is_file():
        return {}
    with path.open("r", encoding="utf-8") as f:
        return json.load(f)


def save(path: Path, data: dict) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    with path.open("w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)
        f.write("\n")


def cmd_init(path: Path, repo: str, ref: str, version: str, commit: str) -> int:
    data = {
        "schema_version": SCHEMA_VERSION,
        "repo": repo,
        "original_repo": repo,
        "ref": ref,
        "version": version,
        "commit": commit,
        "applied_migrations": [],
        "deleted": [],
        "bootstrap_steps_done": [],
    }
    save(path, data)
    return 0


def cmd_get(path: Path, field: str) -> int:
    d = load(path)
    if field not in d:
        print(f"field not found: {field}", file=sys.stderr)
        return 1
    val = d[field]
    if isinstance(val, (dict, list)):
        print(json.dumps(val))
    else:
        print(val)
    return 0


def cmd_set(path: Path, field: str, value: str) -> int:
    if field in IMMUTABLE_FIELDS:
        print(f"field is immutable: {field}", file=sys.stderr)
        return 1
    d = load(path)
    d[field] = value
    save(path, d)
    return 0


def cmd_append_migration(path: Path, mid: str, status: str, reason: str = "") -> int:
    d = load(path)
    entry: dict[str, Any] = {"id": mid, "status": status}
    if reason:
        entry["reason"] = reason
    d.setdefault("applied_migrations", []).append(entry)
    save(path, d)
    return 0


def cmd_append_bootstrap_step(
    path: Path, sid: str, status: str, reason: str = "", content_hash: str = ""
) -> int:
    d = load(path)
    entry: dict[str, Any] = {"id": sid, "status": status}
    if reason:
        entry["reason"] = reason
    if content_hash:
        entry["content_hash"] = content_hash
    d.setdefault("bootstrap_steps_done", []).append(entry)
    save(path, d)
    return 0


def cmd_has_step_applied(path: Path, sid: str, expected_hash: str) -> int:
    d = load(path)
    for s in d.get("bootstrap_steps_done", []):
        if s.get("id") == sid and s.get("status") == "applied" and s.get("content_hash") == expected_hash:
            return 0
    return 1


def cmd_list_steps(path: Path) -> int:
    d = load(path)
    for s in d.get("bootstrap_steps_done", []):
        print(f"{s.get('id')} {s.get('status')} {s.get('content_hash', '')}")
    return 0


DISPATCH = {
    "init": (5, cmd_init),
    "get": (2, cmd_get),
    "set": (3, cmd_set),
    "append-migration": (3, cmd_append_migration),
    "append-bootstrap-step": (3, cmd_append_bootstrap_step),
    "has-step-applied": (3, cmd_has_step_applied),
    "list-steps": (1, cmd_list_steps),
}


def main() -> int:
    if len(sys.argv) < 3:
        print("usage: provenance.py <subcmd> <path> [args...]", file=sys.stderr)
        return 2
    subcmd = sys.argv[1]
    if subcmd not in DISPATCH:
        print(f"unknown subcmd: {subcmd}", file=sys.stderr)
        return 2
    min_args, fn = DISPATCH[subcmd]
    path = Path(sys.argv[2])
    args = sys.argv[3:]
    if len(args) < min_args - 1:
        print(f"too few args for {subcmd}", file=sys.stderr)
        return 2
    return fn(path, *args)


if __name__ == "__main__":
    sys.exit(main())
```

- [ ] **Step 4: Write bash wrapper**

Create `scripts/template-provenance.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
exec python3 "$SCRIPT_DIR/_template_helpers/provenance.py" "$@"
```

`chmod +x scripts/template-provenance.sh scripts/_template_helpers/provenance.py`.

- [ ] **Step 5: Run tests — verify passing**

```bash
bats tests/template-provenance.bats
```

Expected: 8 of 8 pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/_template_helpers/provenance.py scripts/template-provenance.sh tests/template-provenance.bats
git commit -m "feat: template.json provenance helper (init/get/set/append/has-step-applied)"
```

---

## Task 8: `template-init.sh` integration

**Files:**
- Create: `scripts/template-init.sh`
- Create: `tests/template-init.bats`
- Modify: `justfile` (add `template-init` recipe)

- [ ] **Step 1: Write the failing test**

Write `tests/template-init.bats`:

```bash
#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/.." && pwd)"
  FIXTURE="$REPO_ROOT/tests/fixtures/template-update/v0"
  TMP=$(mktemp -d)
  # Mock a "bootstrapped" repo: copy fixture into TMP as a git repo at a known SHA.
  cp -R "$FIXTURE/." "$TMP/"
  cd "$TMP"
  git init -q
  git add -A
  git -c user.email=test@example.com -c user.name=Test commit -q -m "init"
  COMMIT=$(git rev-parse HEAD)
}

teardown() { rm -rf "$TMP"; }

@test "template-init: writes .awiki/template.json with current commit" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo https://github.com/jmcarbo/awiki \
    --ref main \
    --version 1.0.0 \
    --commit "$COMMIT"
  [ "$status" -eq 0 ]
  [ -f .awiki/template.json ]
  PJ_COMMIT=$(bash "$REPO_ROOT/scripts/template-provenance.sh" get .awiki/template.json commit)
  [ "$PJ_COMMIT" = "$COMMIT" ]
}

@test "template-init: snapshots template tree to template-cache/<commit>/" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  [ -d ".awiki/template-cache/$COMMIT" ]
  [ -f ".awiki/template-cache/$COMMIT/template.manifest.toml" ]
}

@test "template-init: records bootstrap_steps_done from BOOTSTRAP.md with content_hash" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" \
    --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  STEPS=$(bash "$REPO_ROOT/scripts/template-provenance.sh" list-steps .awiki/template.json)
  echo "$STEPS" | grep -qE '^dep-check applied sha256:'
  echo "$STEPS" | grep -qE '^domain applied sha256:'
  echo "$STEPS" | grep -qE '^stage-commit applied sha256:'
}

@test "template-init: idempotent (re-run rebuilds cache, doesn't error)" {
  cd "$TMP"
  bash "$REPO_ROOT/scripts/template-init.sh" --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  run bash "$REPO_ROOT/scripts/template-init.sh" --repo url --ref main --version 1.0.0 --commit "$COMMIT"
  [ "$status" -eq 0 ]
}

@test "template-init: missing required arg fails" {
  cd "$TMP"
  run bash "$REPO_ROOT/scripts/template-init.sh" --repo url
  [ "$status" -ne 0 ]
}
```

- [ ] **Step 2: Run test — verify failure**

```bash
bats tests/template-init.bats
```

- [ ] **Step 3: Implement**

Create `scripts/template-init.sh`:

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(pwd)"

usage() {
  cat <<EOF
usage: template-init.sh --repo <url> --ref <ref> --version <version> --commit <sha>

Seeds .awiki/template.json + template-cache/<commit>/ from current working tree.
Idempotent: re-run rebuilds cache from current state.
EOF
  exit 2
}

REPO=""
REF=""
VERSION=""
COMMIT=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --repo) REPO="$2"; shift 2 ;;
    --ref) REF="$2"; shift 2 ;;
    --version) VERSION="$2"; shift 2 ;;
    --commit) COMMIT="$2"; shift 2 ;;
    *) usage ;;
  esac
done

[[ -z "$REPO" || -z "$REF" || -z "$VERSION" || -z "$COMMIT" ]] && usage

MANIFEST="$REPO_ROOT/template.manifest.toml"
BOOTSTRAP_MD="$REPO_ROOT/BOOTSTRAP.md"
PJ="$REPO_ROOT/.awiki/template.json"

[[ -f "$MANIFEST" ]] || { echo "template.manifest.toml not found at repo root" >&2; exit 1; }
[[ -f "$BOOTSTRAP_MD" ]] || { echo "BOOTSTRAP.md not found at repo root" >&2; exit 1; }

# 1. Write template.json (init or rewrite-with-existing-progress).
if [[ -f "$PJ" ]]; then
  echo "info: $PJ already exists; preserving applied_migrations + bootstrap_steps_done" >&2
else
  bash "$SCRIPT_DIR/template-provenance.sh" init "$PJ" "$REPO" "$REF" "$VERSION" "$COMMIT"
fi

# Always update commit/version/ref to current values.
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" commit "$COMMIT" >/dev/null
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" ref "$REF" >/dev/null
bash "$SCRIPT_DIR/template-provenance.sh" set "$PJ" version "$VERSION" >/dev/null

# 2. Snapshot template tree to cache.
CACHE_DIR="$REPO_ROOT/.awiki/template-cache/$COMMIT"
mkdir -p "$CACHE_DIR"
# Use git archive to snapshot tracked files only (matches what template ships).
git archive --format=tar HEAD | tar -x -C "$CACHE_DIR"

# 3. Record bootstrap_steps_done — but only if NOT already present (idempotent).
EXISTING_STEPS=$(python3 -c "
import json, sys
try:
    d = json.load(open('$PJ'))
except Exception:
    sys.exit(0)
for s in d.get('bootstrap_steps_done', []):
    print(s.get('id', ''))
" 2>/dev/null)

while IFS= read -r STEP_ID; do
  [[ -z "$STEP_ID" ]] && continue
  if echo "$EXISTING_STEPS" | grep -qx "$STEP_ID"; then
    continue  # already recorded
  fi
  HASH=$(python3 "$SCRIPT_DIR/_template_helpers/bootstrap_hash.py" hash "$BOOTSTRAP_MD" "$STEP_ID")
  bash "$SCRIPT_DIR/template-provenance.sh" append-bootstrap-step "$PJ" "$STEP_ID" applied "" "$HASH" >/dev/null
done < <(python3 "$SCRIPT_DIR/_template_helpers/bootstrap_hash.py" list "$BOOTSTRAP_MD")

echo "template-init: pinned $REPO @ $COMMIT (version $VERSION)"
```

`chmod +x scripts/template-init.sh`.

- [ ] **Step 4: Add justfile recipe**

Edit `justfile`, add at the end:

```just
# === template ===
template-init:
    bash scripts/template-init.sh --repo "${TEMPLATE_REPO:?TEMPLATE_REPO required}" \
                                   --ref "${TEMPLATE_REF:-main}" \
                                   --version "${TEMPLATE_VERSION:?TEMPLATE_VERSION required}" \
                                   --commit "${TEMPLATE_COMMIT:?TEMPLATE_COMMIT required}"
```

(Justfile recipe wraps `template-init.sh` with env-var defaults; BOOTSTRAP step in Phase 09 will set them.)

- [ ] **Step 5: Run tests — verify passing**

```bash
bats tests/template-init.bats
```

Expected: 5 of 5 pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/template-init.sh justfile tests/template-init.bats
git commit -m "feat: template-init.sh seeds .awiki/template.json + cache + bootstrap_steps_done"
```

---

## Task 9: Verify all Phase 01 tests pass together

**Files:** none — verification only.

- [ ] **Step 1: Run full Phase 01 BATS suite**

```bash
bats tests/template-manifest.bats tests/template-config.bats tests/template-bootstrap-hash.bats tests/template-provenance.bats tests/template-init.bats
```

Expected: all tests pass; total count matches sum across files.

- [ ] **Step 2: Confirm helper scripts are executable**

```bash
ls -l scripts/template-manifest.sh scripts/template-config.sh scripts/template-provenance.sh scripts/template-init.sh
ls -l scripts/_template_helpers/manifest_parse.py scripts/_template_helpers/provenance.py scripts/_template_helpers/bootstrap_hash.py
```

Expected: all marked executable (`-rwx...`).

- [ ] **Step 3: Sanity-test the wrapper layer end-to-end**

```bash
bash scripts/template-manifest.sh load tests/fixtures/template-update/v0/template.manifest.toml
bash scripts/template-manifest.sh resolve tests/fixtures/template-update/v0/template.manifest.toml WIKI.md
```

Expected: emits `schema_version=1` ... and `three_way` respectively.

- [ ] **Step 4: Add Phase 01 entry to CHANGELOG**

Append to `CHANGELOG.md` (create if absent):

```markdown
## [Unreleased]

### Added
- `template-update` foundation: manifest parser with glob-precedence resolver, `.awiki/config` parser, `template.json` provenance helper, bootstrap step content_hash util, `template-init.sh` (Phase 01).
```

- [ ] **Step 5: Commit CHANGELOG**

```bash
git add CHANGELOG.md
git commit -m "docs: changelog — phase 01 template-update foundation"
```

---

## Phase 01 — Definition of done

- [ ] All 5 BATS test files green: manifest, config, bootstrap-hash, provenance, init.
- [ ] `scripts/template-manifest.sh load|resolve|bootstrap-ids|dangerous-ids|has-glob-overlap` work.
- [ ] `scripts/template-config.sh get|validate` work.
- [ ] `scripts/template-provenance.sh init|get|set|append-migration|append-bootstrap-step|has-step-applied|list-steps` work.
- [ ] `scripts/_template_helpers/bootstrap_hash.py list|hash|body` work.
- [ ] `scripts/template-init.sh` is idempotent and seeds provenance + cache + bootstrap_steps_done.
- [ ] `template.json` writes `original_repo` once at init; refused on `set`.
- [ ] `.gitignore` excludes `.awiki/template-cache/`.
- [ ] CHANGELOG entry added.
- [ ] All commits granular (one feature per commit).

Phase 02 depends on Tasks 1–8 here.
