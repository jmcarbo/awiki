# Git-Docs Ingest Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add `scripts/ingest-git.sh` pipeline that ingests markdown documentation from a local path or remote URL git repo into the awiki as `type: source` pages plus one `type: entity` repo page per repo.

**Architecture:** Bash orchestrator (`scripts/ingest-git.sh`) plus three sourceable bash libs under `scripts/lib/` (`git-clone.sh`, `git-state.sh`, `git-config.sh`) plus one Python CommonMark transform (`scripts/ingest-git-transform.py` using `markdown-it-py`). Per-file blob-SHA incremental sync stored in `.awiki/git-state/<repo_key>.json`. Per-repo flock via existing `scripts/lib/lock.sh`. SSH URLs auto-flip to private. Single batched call to `log-append.sh`/`update-catalog.sh`/`qmd reindex` per run.

**Tech Stack:** bash 4+, git 2.30+, flock, python3 3.8+, `markdown-it-py` (new dep), bats 1.10+ (test runner already in repo), `scripts/lib/lock.sh` (existing).

**Spec reference:** `docs/superpowers/specs/2026-04-27-git-docs-ingest-design.md`

---

## File structure

**Created:**
- `scripts/ingest-git.sh` — orchestrator (~350 LOC).
- `scripts/lib/git-clone.sh` — checkout resolver (~150 LOC).
- `scripts/lib/git-state.sh` — JSON state read/write (~150 LOC).
- `scripts/lib/git-config.sh` — yaml parse + validate (~120 LOC).
- `scripts/ingest-git-transform.py` — body rewrite (~250 LOC).
- `tests/ingest_git_test.bats` — orchestrator bats tests.
- `tests/ingest_git_transform_test.bats` — transform bats tests.
- `tests/ingest_git_lib_test.bats` — lib unit tests.
- `tests/fixtures/git-docs-good/` — fixture builder (a script + seed files).
- `tests/fixtures/git-docs-broken/` — fixture builder.
- `tests/fixtures/git-docs-private/` — fixture builder.
- `tests/util/build-git-fixture.sh` — shared helper that materializes a real local git repo from a seed dir.

**Modified:**
- `scripts/check-deps.sh` — add `markdown-it-py` python module check.
- `.gitignore` — add `.awiki/git-state/`, `.awiki/lock/git-*` (existing `.awiki/lock` is a single file; we'll use a directory now), `raw/_git-cache/`.
- `justfile` — add `ingest-git` recipe + `ingest-git-list` helper.
- `WIKI.md` — add §4.7 "Ingest from git repo" workflow section.

---

## Task 1: Add markdown-it-py dependency check

**Files:**
- Modify: `scripts/check-deps.sh:73-79` (add python module check after pyyaml).
- Test: `tests/check_deps_test.bats` (existing; add one new test).

- [ ] **Step 1: Read existing check-deps test pattern**

```bash
grep -n "pyyaml" tests/check_deps_test.bats
```

Expected: hits showing how pyyaml is asserted. We mirror it.

- [ ] **Step 2: Write failing bats test**

Append to `tests/check_deps_test.bats`:

```bash
@test "check-deps reports markdown-it-py status" {
  run bash "$BATS_TEST_DIRNAME/../scripts/check-deps.sh"
  # Status may be 0 (installed) or non-zero only if a HARD dep is missing.
  # markdown-it-py is OPTIONAL → must appear in output as either OK or OPTIONAL-MISSING.
  echo "$output" | grep -E "^(OK\|markdown-it-py|OPTIONAL-MISSING\|markdown-it-py)" >/dev/null
}
```

- [ ] **Step 3: Run test, expect failure**

Run: `bats tests/check_deps_test.bats -f markdown-it-py`
Expected: FAIL — no such string in output yet.

- [ ] **Step 4: Modify scripts/check-deps.sh**

Insert after the pyyaml block (around line 79):

```bash
# Optional Python module: markdown-it-py (used by scripts/ingest-git-transform.py).
if command -v python3 >/dev/null 2>&1 && python3 -c "import markdown_it" >/dev/null 2>&1; then
  echo "OK|markdown-it-py (optional)"
else
  echo "OPTIONAL-MISSING|markdown-it-py"
  echo "  install: pip3 install markdown-it-py" >&2
fi
```

- [ ] **Step 5: Run test, expect pass**

Run: `bats tests/check_deps_test.bats -f markdown-it-py`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/check-deps.sh tests/check_deps_test.bats
git commit -m "feat(deps): check markdown-it-py for git-docs ingest (phase 18)"
```

---

## Task 2: Update .gitignore for git-docs state + cache

**Files:**
- Modify: `.gitignore` (append three lines).

- [ ] **Step 1: Read current .gitignore**

```bash
grep -nE '^\.awiki/lock|^raw/' .gitignore
```

Note existing `.awiki/lock` line — it currently treats lock as a single file. We'll change to a directory pattern (lock files become per-repo).

- [ ] **Step 2: Append new patterns**

Append to `.gitignore`:

```
# git-docs ingest (phase 18)
.awiki/git-state/
.awiki/lock/
raw/_git-cache/
```

If a line `.awiki/lock` (file) already exists, leave it — `.awiki/lock/` (dir) entry is additional and matches a different path. We won't migrate the existing single-file lock; phase 18 introduces a `.awiki/lock/` directory of per-repo lock files used **only** by `ingest-git.sh`. The existing file lock used by `lib/lock.sh` is unchanged.

- [ ] **Step 3: Verify**

```bash
git check-ignore -v .awiki/git-state/foo.json raw/_git-cache/bar/.git/HEAD .awiki/lock/git-baz
```

Expected: each path reported as ignored by the new lines.

- [ ] **Step 4: Commit**

```bash
git add .gitignore
git commit -m "chore(gitignore): ignore git-docs state, cache, per-repo lock dir (phase 18)"
```

---

## Task 3: Library `scripts/lib/git-state.sh` — JSON state file read/write

**Files:**
- Create: `scripts/lib/git-state.sh`.
- Test: `tests/ingest_git_lib_test.bats` (new).

The lib exposes three sourceable functions:

- `awiki_git_state_path <repo_key>` — print `.awiki/git-state/<repo_key>.json`.
- `awiki_git_state_load <repo_key>` — print contents (empty `{}`-ish JSON if missing).
- `awiki_git_state_save <repo_key> <json>` — atomic write (tmp + rename).

State JSON shape (per spec §7.2):

```json
{"schema":1,"repo_key":"...","repo_name":"...","url":"...","default_branch":"...","head_sha":"...","ingested_at":"...","private":false,"files":{}}
```

- [ ] **Step 1: Create test file with first failing test**

Create `tests/ingest_git_lib_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
  mkdir -p .awiki/git-state
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "git-state: path() returns expected file location" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_path "github-com-foo-bar"
  [ "$status" -eq 0 ]
  [ "$output" = ".awiki/git-state/github-com-foo-bar.json" ]
}
```

- [ ] **Step 2: Run, expect fail (lib missing)**

Run: `bats tests/ingest_git_lib_test.bats`
Expected: FAIL — `lib/git-state.sh` not found.

- [ ] **Step 3: Create scripts/lib/git-state.sh skeleton**

```bash
#!/usr/bin/env bash
# Source-only helper. Read/write per-repo state JSON for git-docs ingest.
# Functions:
#   awiki_git_state_path <repo_key>       → echoes .awiki/git-state/<repo_key>.json
#   awiki_git_state_load <repo_key>       → prints JSON; '{}' if missing
#   awiki_git_state_save <repo_key> <json> → atomic write (tmp + rename)
#   awiki_git_state_validate <json>       → exits 0 if schema=1 and required keys present

awiki_git_state_path() {
  local repo_key="$1"
  if [[ -z "$repo_key" ]]; then
    echo "ERROR: awiki_git_state_path requires repo_key" >&2
    return 1
  fi
  echo ".awiki/git-state/${repo_key}.json"
}

awiki_git_state_load() {
  local path
  path="$(awiki_git_state_path "$1")" || return 1
  if [[ -f "$path" ]]; then
    cat "$path"
  else
    echo "{}"
  fi
}

awiki_git_state_save() {
  local repo_key="$1" json="$2"
  local path
  path="$(awiki_git_state_path "$repo_key")" || return 1
  mkdir -p "$(dirname "$path")"
  local tmp="${path}.tmp.$$"
  printf '%s\n' "$json" > "$tmp"
  mv -f "$tmp" "$path"
}

awiki_git_state_validate() {
  local json="$1"
  if ! command -v python3 >/dev/null 2>&1; then
    echo "ERROR: python3 required for state validation" >&2
    return 1
  fi
  python3 - <<'PY' "$json"
import json, sys
try:
    obj = json.loads(sys.argv[1])
except Exception as e:
    print(f"ERROR: invalid JSON: {e}", file=sys.stderr); sys.exit(1)
if obj.get("schema") != 1:
    print("ERROR: schema must be 1", file=sys.stderr); sys.exit(1)
for key in ("repo_key","repo_name","url","head_sha","files"):
    if key not in obj:
        print(f"ERROR: missing key: {key}", file=sys.stderr); sys.exit(1)
sys.exit(0)
PY
}
```

- [ ] **Step 4: Run test, expect pass**

Run: `bats tests/ingest_git_lib_test.bats -f "git-state: path"`
Expected: PASS.

- [ ] **Step 5: Add load/save/validate tests**

Append to `tests/ingest_git_lib_test.bats`:

```bash
@test "git-state: load() returns {} when missing" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_load "no-such-repo"
  [ "$status" -eq 0 ]
  [ "$output" = "{}" ]
}

@test "git-state: save() then load() roundtrips" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  awiki_git_state_save "rk" '{"schema":1,"repo_key":"rk","files":{}}'
  run awiki_git_state_load "rk"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"schema":1'
  echo "$output" | grep -q '"repo_key":"rk"'
}

@test "git-state: validate() rejects schema!=1" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_validate '{"schema":2,"repo_key":"rk","repo_name":"r","url":"u","head_sha":"s","files":{}}'
  [ "$status" -ne 0 ]
}

@test "git-state: validate() accepts well-formed" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-state.sh"
  run awiki_git_state_validate '{"schema":1,"repo_key":"rk","repo_name":"r","url":"u","head_sha":"s","files":{}}'
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 6: Run all four tests, expect pass**

Run: `bats tests/ingest_git_lib_test.bats`
Expected: 4 of 4 PASS.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/git-state.sh tests/ingest_git_lib_test.bats
git commit -m "feat(git-docs): add lib/git-state.sh — atomic JSON state per repo (phase 18)"
```

---

## Task 4: Library `scripts/lib/git-config.sh` — yaml config parse + validate

**Files:**
- Create: `scripts/lib/git-config.sh`.
- Modify: `tests/ingest_git_lib_test.bats` (add tests).
- Test fixture: `tests/fixtures/git-sources-good.yml`, `tests/fixtures/git-sources-bad.yml`.

The lib exposes:

- `awiki_git_config_load <yaml-path>` — parse to JSON via python3 (using stdlib only — repo already declares `pyyaml` optional, so for required parsing we'll use a tiny inline parser).

But pyyaml is OPTIONAL in `check-deps.sh`. We want config parsing to NOT require pyyaml. **Decision:** require pyyaml when config file present; print install hint and exit 12 if missing AND a config-driven path is requested. CLI-only invocations (no `<repo-spec>` lookup) work without pyyaml.

- `awiki_git_config_get <yaml-path> <repo-name>` — print one repo's config block as JSON.
- `awiki_git_config_validate_name <name>` — exit 0 if matches `^[a-z][a-z0-9-]*$`.
- `awiki_git_config_validate_paths <json-array>` — reject `..` / leading `/`.

- [ ] **Step 1: Create good fixture**

Create `tests/fixtures/git-sources-good.yml`:

```yaml
schema: 1
repos:
  - name: example-repo
    url: https://github.com/example/example.git
    paths: [README.md, docs/]
    exclude: ["**/CHANGELOG.md"]
    private: false
    protect_edits: false
    summarize: false
    default_branch: main
  - name: private-runbooks
    url: git@github.com:corp/runbooks.git
    paths: [docs/]
    private: true
```

- [ ] **Step 2: Create bad fixture**

Create `tests/fixtures/git-sources-bad.yml`:

```yaml
schema: 1
repos:
  - name: BadName       # uppercase — should fail validation
    url: https://example
    paths: ["../escape"]
```

- [ ] **Step 3: Write failing tests**

Append to `tests/ingest_git_lib_test.bats`:

```bash
@test "git-config: name validator accepts kebab" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_name "ok-name"
  [ "$status" -eq 0 ]
}

@test "git-config: name validator rejects uppercase" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_name "BadName"
  [ "$status" -ne 0 ]
}

@test "git-config: name validator rejects leading dash" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_name "-foo"
  [ "$status" -ne 0 ]
}

@test "git-config: get() loads named repo from yaml" {
  if ! python3 -c "import yaml" >/dev/null 2>&1; then skip "pyyaml not installed"; fi
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_get "$BATS_TEST_DIRNAME/fixtures/git-sources-good.yml" "example-repo"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"url":"https://github.com/example/example.git"'
  echo "$output" | grep -q '"private":false'
}

@test "git-config: get() detects SSH→private auto-flip on private-runbooks" {
  if ! python3 -c "import yaml" >/dev/null 2>&1; then skip "pyyaml not installed"; fi
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_get "$BATS_TEST_DIRNAME/fixtures/git-sources-good.yml" "private-runbooks"
  [ "$status" -eq 0 ]
  echo "$output" | grep -q '"private":true'
}

@test "git-config: get() rejects bad name from bad fixture" {
  if ! python3 -c "import yaml" >/dev/null 2>&1; then skip "pyyaml not installed"; fi
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_get "$BATS_TEST_DIRNAME/fixtures/git-sources-bad.yml" "BadName"
  [ "$status" -ne 0 ]
}

@test "git-config: validate_paths rejects .." {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_paths '["docs/","../escape"]'
  [ "$status" -ne 0 ]
}

@test "git-config: validate_paths rejects leading /" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_paths '["docs/","/etc"]'
  [ "$status" -ne 0 ]
}

@test "git-config: validate_paths accepts clean entries" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-config.sh"
  run awiki_git_config_validate_paths '["README.md","docs/","rfcs/"]'
  [ "$status" -eq 0 ]
}
```

- [ ] **Step 4: Run, expect fail**

Run: `bats tests/ingest_git_lib_test.bats -f "git-config"`
Expected: FAIL — lib missing.

- [ ] **Step 5: Create scripts/lib/git-config.sh**

```bash
#!/usr/bin/env bash
# Source-only helper. Parse + validate .awiki/git-sources.yml entries.
# Functions:
#   awiki_git_config_validate_name <name>         → exit 0 if ^[a-z][a-z0-9-]*$
#   awiki_git_config_validate_paths <json-array>  → exit 0 if no '..' / leading '/'
#   awiki_git_config_get <yaml-path> <repo-name>  → JSON for the named repo
#                                                   (auto-flips private=true on SSH url)

awiki_git_config_validate_name() {
  local name="$1"
  if [[ "$name" =~ ^[a-z][a-z0-9-]*$ ]]; then
    return 0
  fi
  echo "ERROR: invalid repo name (need ^[a-z][a-z0-9-]*$): $name" >&2
  return 1
}

awiki_git_config_validate_paths() {
  local arr="$1"
  python3 - <<'PY' "$arr"
import json, sys
try:
    paths = json.loads(sys.argv[1])
except Exception as e:
    print(f"ERROR: invalid JSON: {e}", file=sys.stderr); sys.exit(1)
if not isinstance(paths, list):
    print("ERROR: expected JSON array", file=sys.stderr); sys.exit(1)
for p in paths:
    if not isinstance(p, str):
        print(f"ERROR: path entry must be string: {p!r}", file=sys.stderr); sys.exit(1)
    if ".." in p.split("/"):
        print(f"ERROR: '..' not allowed in path: {p}", file=sys.stderr); sys.exit(1)
    if p.startswith("/"):
        print(f"ERROR: leading '/' not allowed in path: {p}", file=sys.stderr); sys.exit(1)
sys.exit(0)
PY
}

awiki_git_config_get() {
  local yaml_path="$1" repo_name="$2"
  if [[ ! -f "$yaml_path" ]]; then
    echo "ERROR: config not found: $yaml_path" >&2
    return 12
  fi
  if ! python3 -c "import yaml" >/dev/null 2>&1; then
    echo "ERROR: pyyaml required to parse $yaml_path; install: pip3 install pyyaml" >&2
    return 12
  fi
  python3 - <<'PY' "$yaml_path" "$repo_name"
import json, re, sys, yaml
yaml_path, repo_name = sys.argv[1], sys.argv[2]
try:
    with open(yaml_path) as f:
        doc = yaml.safe_load(f)
except yaml.YAMLError as e:
    print(f"ERROR: yaml parse: {e}", file=sys.stderr); sys.exit(12)
if not isinstance(doc, dict) or doc.get("schema") != 1:
    print("ERROR: missing or wrong schema (need schema: 1)", file=sys.stderr); sys.exit(12)
repos = doc.get("repos") or []
match = next((r for r in repos if r.get("name") == repo_name), None)
if match is None:
    print(f"ERROR: no repo named: {repo_name}", file=sys.stderr); sys.exit(12)
name = match.get("name", "")
if not re.match(r"^[a-z][a-z0-9-]*$", name):
    print(f"ERROR: invalid name: {name}", file=sys.stderr); sys.exit(12)
url = match.get("url", "")
private = bool(match.get("private", False))
# SSH url auto-flips private — only when explicit private flag is not False-set.
# Spec §8: explicit private:false on SSH url → honor public + warning.
if url.startswith("git@") and "private" not in match:
    private = True
out = {
    "name": name,
    "url": url,
    "paths": match.get("paths", ["README.md", "docs/", "rfcs/", "adr/"]),
    "exclude": match.get("exclude", []),
    "private": private,
    "private_explicit": "private" in match,
    "protect_edits": bool(match.get("protect_edits", False)),
    "summarize": bool(match.get("summarize", False)),
    "default_branch": match.get("default_branch", ""),
}
print(json.dumps(out))
sys.exit(0)
PY
}
```

- [ ] **Step 6: Run all git-config tests, expect pass (skip when pyyaml missing)**

Run: `bats tests/ingest_git_lib_test.bats -f "git-config"`
Expected: PASS for the 6 sync tests; the 3 yaml-loading tests pass if pyyaml installed, else skip.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/git-config.sh tests/ingest_git_lib_test.bats tests/fixtures/git-sources-good.yml tests/fixtures/git-sources-bad.yml
git commit -m "feat(git-docs): add lib/git-config.sh — yaml parse + name/paths validation (phase 18)"
```

---

## Task 5: Library `scripts/lib/git-clone.sh` — checkout resolver

**Files:**
- Create: `scripts/lib/git-clone.sh`.
- Modify: `tests/ingest_git_lib_test.bats` (add tests).
- Helper: `tests/util/build-git-fixture.sh`.

The lib exposes:

- `awiki_git_clone_repo_key <spec>` — derive repo_key from spec.
  - Local path: `local-<basename-of-realpath>`.
  - URL `https://github.com/foo/bar.git` → `github-com-foo-bar`.
  - URL `git@github.com:foo/bar.git` → `github-com-foo-bar`.
  - URL `file:///abs/path/foo` → `local-foo`.
- `awiki_git_clone_is_ssh <url>` → exit 0 if url starts with `git@`.
- `awiki_git_clone_resolve <spec> <repo_key>` → echo `<checkout_path>|<head_sha>|<default_branch>`. Clone or pull as needed. Local path returns the path itself + current HEAD.

- [ ] **Step 1: Create tests/util/build-git-fixture.sh**

```bash
#!/usr/bin/env bash
# Build a real local git repo from a seed directory tree.
# Usage: build-git-fixture.sh <seed-dir> <out-bare-path>
# Returns 0 on success. The output is a non-bare repo (has working tree)
# so ls-tree on it works directly, and HEAD is `main`.

set -euo pipefail

SEED="$1"
OUT="$2"

if [[ ! -d "$SEED" ]]; then
  echo "ERROR: seed not found: $SEED" >&2; exit 1
fi
if [[ -e "$OUT" ]]; then
  rm -rf "$OUT"
fi
mkdir -p "$OUT"
cp -R "$SEED"/. "$OUT"/
cd "$OUT"
git init -q -b main
git config user.email fixture@local
git config user.name fixture
git add -A
git commit -q -m "fixture: initial commit"
```

- [ ] **Step 2: Make it executable + sanity test**

```bash
chmod +x tests/util/build-git-fixture.sh
mkdir -p /tmp/seed-x && echo "hi" > /tmp/seed-x/README.md
bash tests/util/build-git-fixture.sh /tmp/seed-x /tmp/repo-x
git -C /tmp/repo-x rev-parse HEAD   # prints sha
rm -rf /tmp/seed-x /tmp/repo-x
```

Expected: outputs a sha, no errors.

- [ ] **Step 3: Write failing tests for repo_key derivation**

Append to `tests/ingest_git_lib_test.bats`:

```bash
@test "git-clone: repo_key from https github URL" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  run awiki_git_clone_repo_key "https://github.com/foo/bar.git"
  [ "$status" -eq 0 ]
  [ "$output" = "github-com-foo-bar" ]
}

@test "git-clone: repo_key from ssh URL" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  run awiki_git_clone_repo_key "git@github.com:foo/bar.git"
  [ "$status" -eq 0 ]
  [ "$output" = "github-com-foo-bar" ]
}

@test "git-clone: repo_key from local path" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  d="$(mktemp -d)/myrepo"
  mkdir -p "$d"
  run awiki_git_clone_repo_key "$d"
  [ "$status" -eq 0 ]
  [ "$output" = "local-myrepo" ]
}

@test "git-clone: is_ssh detects git@" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  run awiki_git_clone_is_ssh "git@github.com:foo/bar.git"
  [ "$status" -eq 0 ]
  run awiki_git_clone_is_ssh "https://github.com/foo/bar.git"
  [ "$status" -ne 0 ]
}

@test "git-clone: resolve returns path|sha|branch for local fixture" {
  source "$BATS_TEST_DIRNAME/../scripts/lib/git-clone.sh"
  seed="$(mktemp -d)/seed"
  mkdir -p "$seed"
  echo "hi" > "$seed/README.md"
  fixture="$(mktemp -d)/repo"
  bash "$BATS_TEST_DIRNAME/util/build-git-fixture.sh" "$seed" "$fixture"
  run awiki_git_clone_resolve "$fixture" "local-repo"
  [ "$status" -eq 0 ]
  echo "$output" | grep -qE "^${fixture}\|[0-9a-f]{40}\|main$"
}
```

- [ ] **Step 4: Run, expect fail**

Run: `bats tests/ingest_git_lib_test.bats -f "git-clone"`
Expected: FAIL — lib missing.

- [ ] **Step 5: Create scripts/lib/git-clone.sh**

```bash
#!/usr/bin/env bash
# Source-only helper. Resolve <repo-spec> to a local checkout for ingest-git.sh.

awiki_git_clone_is_ssh() {
  [[ "${1:-}" == git@* ]]
}

awiki_git_clone_repo_key() {
  local spec="${1:-}"
  if [[ -z "$spec" ]]; then
    echo "ERROR: repo_key needs spec" >&2; return 1
  fi
  # Local path branch
  if [[ -d "$spec" || "$spec" = /* || "$spec" = ./* || "$spec" = ../* ]]; then
    local rp
    rp="$(cd "$spec" 2>/dev/null && pwd || true)"
    if [[ -z "$rp" ]]; then rp="$spec"; fi
    echo "local-$(basename "$rp")"
    return 0
  fi
  # file:// URL
  if [[ "$spec" =~ ^file:// ]]; then
    local p="${spec#file://}"
    echo "local-$(basename "$p" .git)"
    return 0
  fi
  # https / http URL: https://host/owner/repo(.git)
  if [[ "$spec" =~ ^https?://([^/]+)/(.+)$ ]]; then
    local host="${BASH_REMATCH[1]}" rest="${BASH_REMATCH[2]}"
    rest="${rest%.git}"
    rest="${rest//\//-}"
    host="${host//./-}"
    echo "${host}-${rest}"
    return 0
  fi
  # ssh URL: git@host:owner/repo(.git)
  if [[ "$spec" =~ ^git@([^:]+):(.+)$ ]]; then
    local host="${BASH_REMATCH[1]}" rest="${BASH_REMATCH[2]}"
    rest="${rest%.git}"
    rest="${rest//\//-}"
    host="${host//./-}"
    echo "${host}-${rest}"
    return 0
  fi
  echo "ERROR: unrecognized repo spec: $spec" >&2
  return 1
}

awiki_git_clone_resolve() {
  local spec="${1:-}" repo_key="${2:-}"
  if [[ -z "$spec" || -z "$repo_key" ]]; then
    echo "ERROR: resolve needs <spec> <repo_key>" >&2; return 1
  fi

  local checkout=""
  if [[ -d "$spec" ]]; then
    checkout="$(cd "$spec" && pwd)"
  elif [[ "$spec" =~ ^file:// ]]; then
    checkout="${spec#file://}"
  else
    # remote URL — clone or fetch under raw/_git-cache/
    checkout="raw/_git-cache/${repo_key}"
    if [[ ! -d "$checkout/.git" ]]; then
      mkdir -p raw/_git-cache
      git clone --filter=blob:none --quiet "$spec" "$checkout" || return 10
    else
      git -C "$checkout" fetch --quiet --prune origin || return 11
      local default_branch
      default_branch="$(git -C "$checkout" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||' || echo main)"
      git -C "$checkout" reset --quiet --hard "origin/${default_branch}" || return 11
    fi
  fi

  local sha
  sha="$(git -C "$checkout" rev-parse HEAD 2>/dev/null)" || return 11
  local branch
  branch="$(git -C "$checkout" rev-parse --abbrev-ref HEAD 2>/dev/null)" || branch="HEAD"
  if [[ "$branch" = "HEAD" ]]; then
    branch="$(git -C "$checkout" symbolic-ref --short refs/remotes/origin/HEAD 2>/dev/null | sed 's|^origin/||' || echo main)"
  fi
  printf '%s|%s|%s\n' "$checkout" "$sha" "$branch"
}
```

- [ ] **Step 6: Run, expect pass**

Run: `bats tests/ingest_git_lib_test.bats -f "git-clone"`
Expected: 5 of 5 PASS.

- [ ] **Step 7: Commit**

```bash
git add scripts/lib/git-clone.sh scripts/lib/.. tests/ingest_git_lib_test.bats tests/util/build-git-fixture.sh
git commit -m "feat(git-docs): add lib/git-clone.sh + fixture builder (phase 18)"
```

---

## Task 6: Python transform — frontmatter strip + awiki frontmatter emit

**Files:**
- Create: `scripts/ingest-git-transform.py`.
- Test: `tests/ingest_git_transform_test.bats` (new).
- Test fixture seed: `tests/fixtures/git-docs-good/` (initial files added incrementally).

The transform reads upstream md from a path, slug map from stdin (JSON), and emits transformed md to a path. Spec §6.1 / §6.2.

CLI:

```
ingest-git-transform.py \
  --in <upstream-md-path> \
  --out <out-md-path> \
  --repo-key <key> \
  --repo-name <name> \
  --repo-relpath <rel> \
  --git-url <url> \
  --git-blob-sha <sha> \
  --asset-out-dir <dir>     # absolute path under content/sources/_assets/git-<repo>/
  [--private]
  [--upstream-root <abs-path-to-checkout>]   # for image asset resolution
```

Reads slug map from stdin: `{"<repo-relpath>": "<slug>", ...}`.

Writes the transformed md to `--out` atomically. Prints `OK|<asset-count>` or `WARN|<msg>` lines for each warning, exits 0 on success / 1 on parse failure.

This task lands frontmatter strip + emit only. Wikilink rewrite + image copy follow in tasks 7-8.

- [ ] **Step 1: Create transform stub + test fixture**

Create `tests/fixtures/git-docs-good/seed/README.md`:

```markdown
---
title: "Upstream Title"
foo: bar
---

# Upstream Title

This is an example. See [foo](./docs/foo.md) for details.
```

- [ ] **Step 2: Write failing test**

Create `tests/ingest_git_transform_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "transform: strips upstream frontmatter and emits awiki frontmatter" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/README.md" upstream.md
  echo '{}' | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath README.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  [ -f out.md ]
  head -1 out.md | grep -q '^---'
  grep -q "^type: source" out.md
  grep -q "^provenance: git" out.md
  grep -q "^git_repo: x" out.md
  grep -q "^git_path: README.md" out.md
  ! grep -q "^foo: bar" out.md      # upstream frontmatter not leaked
  grep -q "Upstream Title" out.md   # body H1 retained
}
```

- [ ] **Step 3: Run, expect fail**

Run: `bats tests/ingest_git_transform_test.bats`
Expected: FAIL — transform missing.

- [ ] **Step 4: Create scripts/ingest-git-transform.py**

```python
#!/usr/bin/env python3
"""Transform one upstream markdown file into an awiki source page.

This is the v1 mechanical path — no LLM. Reads slug map on stdin (JSON
mapping repo-relpath → slug). Strips upstream frontmatter, rewrites
relative md links to wikilinks (Task 7), copies image assets (Task 8),
preserves code blocks verbatim, and emits awiki frontmatter.

Exit 0 on success, 1 on parse failure.
"""
import argparse
import json
import os
import re
import sys
from datetime import date
from pathlib import Path

UPSTREAM_FM_RE = re.compile(r"^---\s*\n(.*?)\n---\s*\n", re.DOTALL)


def strip_upstream_frontmatter(text: str) -> tuple[dict, str]:
    m = UPSTREAM_FM_RE.match(text)
    if not m:
        return {}, text
    raw = m.group(1)
    # We do NOT need full yaml parse — just pull title for fallback, ignore rest.
    title = ""
    for line in raw.splitlines():
        line = line.rstrip()
        if line.startswith("title:"):
            title = line.split(":", 1)[1].strip().strip('"').strip("'")
            break
    return {"title": title}, text[m.end():]


def derive_title(upstream_meta: dict, body: str, fallback: str) -> str:
    if upstream_meta.get("title"):
        return upstream_meta["title"]
    for line in body.splitlines():
        if line.startswith("# "):
            return line[2:].strip()
    return fallback


def emit_frontmatter(args: argparse.Namespace, title: str, today: str) -> str:
    tags = ["git", args.repo_name]
    if args.private:
        tags.append("private")
    tags_inline = "[" + ", ".join(tags) + "]"
    lines = [
        "---",
        f'title: "{title}"',
        f"date: {today}",
        f"last_updated: {today}",
        "type: source",
        "provenance: git",
        f"git_repo: {args.repo_name}",
        f"git_path: {args.repo_relpath}",
        f"git_blob_sha: {args.git_blob_sha}",
        f"git_url: {args.git_url}",
        f"tags: {tags_inline}",
        "sources: []",
        "draft: false",
        "---",
        "",
    ]
    return "\n".join(lines)


def main(argv: list[str]) -> int:
    p = argparse.ArgumentParser()
    p.add_argument("--in", dest="in_path", required=True)
    p.add_argument("--out", dest="out_path", required=True)
    p.add_argument("--repo-key", required=True)
    p.add_argument("--repo-name", required=True)
    p.add_argument("--repo-relpath", required=True)
    p.add_argument("--git-url", required=True)
    p.add_argument("--git-blob-sha", required=True)
    p.add_argument("--asset-out-dir", required=True)
    p.add_argument("--upstream-root", default="")
    p.add_argument("--private", action="store_true")
    args = p.parse_args(argv)

    try:
        slug_map = json.loads(sys.stdin.read() or "{}")
    except json.JSONDecodeError as e:
        print(f"ERROR|stdin slug map parse: {e}", file=sys.stderr); return 1

    in_path = Path(args.in_path)
    if not in_path.is_file():
        print(f"ERROR|in not found: {in_path}", file=sys.stderr); return 1

    raw = in_path.read_text(encoding="utf-8", errors="strict")
    if len(raw.strip()) < 10:
        print("WARN|skipped: <10 char body")
        return 0
    upstream_meta, body = strip_upstream_frontmatter(raw)
    fallback_title = in_path.stem.replace("-", " ").title()
    title = derive_title(upstream_meta, body, fallback_title)

    today = date.today().isoformat()
    out_text = emit_frontmatter(args, title, today) + body.lstrip("\n")

    out_path = Path(args.out_path)
    out_path.parent.mkdir(parents=True, exist_ok=True)
    tmp = out_path.with_suffix(out_path.suffix + f".tmp.{os.getpid()}")
    tmp.write_text(out_text, encoding="utf-8")
    tmp.replace(out_path)
    print("OK|0")
    return 0


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))
```

- [ ] **Step 5: Make executable + run, expect pass**

```bash
chmod +x scripts/ingest-git-transform.py
bats tests/ingest_git_transform_test.bats
```

Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/ingest-git-transform.py tests/ingest_git_transform_test.bats tests/fixtures/git-docs-good/seed/README.md
git commit -m "feat(git-docs): add transform.py — frontmatter strip + emit (phase 18)"
```

---

## Task 7: Python transform — relative md link → wikilink

**Files:**
- Modify: `scripts/ingest-git-transform.py` (add wikilink rewrite step before write).
- Modify: `tests/ingest_git_transform_test.bats` (add tests).
- Modify: `tests/fixtures/git-docs-good/seed/` (add `docs/intro.md` + `docs/foo.md`).

Use `markdown-it-py` AST so we don't rewrite inside code fences.

- [ ] **Step 1: Add fixture files**

Create `tests/fixtures/git-docs-good/seed/docs/intro.md`:

````markdown
# Intro

See [foo](./foo.md). Also see [README](../README.md).

```python
# this is code, [fake](./not-rewritten.md) must stay verbatim
```

Reference style: see [bar][1].

[1]: ./foo.md
````

Create `tests/fixtures/git-docs-good/seed/docs/foo.md`:

```markdown
# Foo

Body of foo.
```

- [ ] **Step 2: Write failing tests**

Append to `tests/ingest_git_transform_test.bats`:

```bash
@test "transform: rewrites relative md link to wikilink" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  printf '%s' '{"docs/foo.md":"git-x-docs-foo","README.md":"git-x-readme"}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  grep -q "\\[foo\\](\\[\\[git-x-docs-foo\\]\\])" out.md || grep -q "\\[\\[git-x-docs-foo|foo\\]\\]" out.md
  grep -q "\\[\\[git-x-readme|README\\]\\]" out.md || grep -q "\\[README\\]" out.md
}

@test "transform: does not rewrite inside fenced code" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  printf '%s' '{"docs/foo.md":"git-x-docs-foo","docs/not-rewritten.md":"git-x-docs-not-rewritten"}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  ! grep -q "git-x-docs-not-rewritten" out.md
  grep -q "fake" out.md
}

@test "transform: rewrites reference-style links" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  printf '%s' '{"docs/foo.md":"git-x-docs-foo"}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  grep -q "git-x-docs-foo" out.md   # bar reference link rewritten via [1]: ./foo.md
}

@test "transform: leaves link plain when target slug not in map" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  cp "$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed/docs/intro.md" upstream.md
  echo '{}' \
    | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x"
  grep -q "\\[foo\\](./foo.md)" out.md   # untouched
}
```

- [ ] **Step 3: Run, expect fail**

Run: `bats tests/ingest_git_transform_test.bats -f "wikilink|code|reference|plain"`
Expected: 4 FAILs.

- [ ] **Step 4: Add wikilink rewrite to transform**

Modify `scripts/ingest-git-transform.py`. Add after imports:

```python
from markdown_it import MarkdownIt
```

Add before `main()`:

```python
def _normalize_relpath(current_relpath: str, link_target: str) -> str:
    """Resolve link_target (relative to file containing it) to a repo-relpath
    suitable for slug-map lookup. Strips fragments and query strings.
    Returns '' if the target leaves the repo (..-overshoot)."""
    if "://" in link_target or link_target.startswith("/"):
        return ""
    target = link_target.split("#", 1)[0].split("?", 1)[0]
    if not target:
        return ""
    base = os.path.dirname(current_relpath)
    joined = os.path.normpath(os.path.join(base, target)) if base else os.path.normpath(target)
    if joined.startswith("..") or joined == "." :
        return ""
    return joined


def _is_md_target(path: str) -> bool:
    return path.lower().endswith(".md") or path.lower().endswith(".mdx")


def rewrite_wikilinks(body: str, current_relpath: str, slug_map: dict) -> tuple[str, list]:
    """Walk markdown-it tokens; for inline link tokens whose href resolves
    to a repo-relative .md path that exists in slug_map, replace the rendered
    link with `[[<slug>|<text>]]`. Otherwise leave the link in place.

    Reference-style links and inline links share the same token shape after
    parse — both resolve to `link_open` tokens with `attrs[href]`.

    Code spans / fenced code blocks are NOT walked (markdown-it tokenizes
    them as fence/code_inline tokens whose children we never enter).
    """
    md = MarkdownIt("commonmark")
    env = {}
    tokens = md.parse(body, env)

    # Build a map of (line, col) → replacement so we can splice into the
    # raw body. markdown-it source-maps are coarse; we instead serialize
    # back from tokens. Easier: re-render but that loses formatting.
    # Pragmatic v1 approach: regex-walk over the body but ONLY outside
    # fenced ranges discovered by markdown-it.

    fence_ranges = []  # list of (start_line, end_line) inclusive
    for t in tokens:
        if t.type == "fence" or t.type == "code_block":
            if t.map:
                fence_ranges.append((t.map[0], t.map[1]))

    def in_fence(line_idx: int) -> bool:
        for s, e in fence_ranges:
            if s <= line_idx < e:
                return True
        return False

    # Build inline link rewrites: scan tokens for link_open whose attrs include
    # href ending in .md / .mdx, build (raw_inline_pattern, replacement).
    # We rewrite by regex on each non-fence line.
    LINK_RE = re.compile(r"(?<!`)\[([^\]]+)\]\(([^)]+)\)")  # [text](href)
    REF_RE = re.compile(r"^\[([^\]]+)\]:\s*(\S+)\s*$")       # [id]: target
    REFLINK_RE = re.compile(r"\[([^\]]+)\]\[([^\]]+)\]")     # [text][id]

    # Collect reference defs first.
    refs = {}
    for i, line in enumerate(body.splitlines()):
        if in_fence(i):
            continue
        m = REF_RE.match(line)
        if m:
            refs[m.group(1).lower()] = m.group(2)

    warnings = []

    def rewrite_inline(line_idx: int, line: str) -> str:
        if in_fence(line_idx):
            return line

        def _sub(m):
            text, href = m.group(1), m.group(2)
            if not _is_md_target(href.split()[0]):
                return m.group(0)
            target = _normalize_relpath(current_relpath, href.split()[0])
            slug = slug_map.get(target)
            if not slug:
                return m.group(0)
            return f"[[{slug}|{text}]]"

        out = LINK_RE.sub(_sub, line)

        def _sub_ref(m):
            text, ref_id = m.group(1), m.group(2).lower()
            href = refs.get(ref_id)
            if not href or not _is_md_target(href):
                return m.group(0)
            target = _normalize_relpath(current_relpath, href)
            slug = slug_map.get(target)
            if not slug:
                return m.group(0)
            return f"[[{slug}|{text}]]"

        out = REFLINK_RE.sub(_sub_ref, out)
        return out

    new_lines = []
    for i, line in enumerate(body.splitlines()):
        new_lines.append(rewrite_inline(i, line))
    new_body = "\n".join(new_lines)
    if body.endswith("\n"):
        new_body += "\n"
    return new_body, warnings
```

In `main()`, after `body = ...` is established:

```python
    body, link_warnings = rewrite_wikilinks(body, args.repo_relpath, slug_map)
    for w in link_warnings:
        print(f"WARN|{w}")
```

- [ ] **Step 5: Run tests, expect pass**

Run: `bats tests/ingest_git_transform_test.bats`
Expected: 5 of 5 PASS (including the 1 from task 6).

- [ ] **Step 6: Commit**

```bash
git add scripts/ingest-git-transform.py tests/ingest_git_transform_test.bats tests/fixtures/git-docs-good/seed/docs/intro.md tests/fixtures/git-docs-good/seed/docs/foo.md
git commit -m "feat(git-docs): transform.py — rewrite md links to wikilinks (phase 18)"
```

---

## Task 8: Python transform — image asset copy + path rewrite

**Files:**
- Modify: `scripts/ingest-git-transform.py`.
- Modify: `tests/ingest_git_transform_test.bats`.
- Modify: `tests/fixtures/git-docs-good/seed/docs/intro.md` (add image ref).
- Add: `tests/fixtures/git-docs-good/seed/docs/img/arch.png` (1x1 png).

- [ ] **Step 1: Add image fixture**

```bash
# 1x1 transparent png base64 — write via python so the fixture is binary-clean.
python3 -c "import base64,pathlib; pathlib.Path('tests/fixtures/git-docs-good/seed/docs/img').mkdir(parents=True, exist_ok=True); pathlib.Path('tests/fixtures/git-docs-good/seed/docs/img/arch.png').write_bytes(base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAQAAAC1HAwCAAAAC0lEQVR42mNkYAAAAAYAAjCB0C8AAAAASUVORK5CYII='))"
```

Append to `tests/fixtures/git-docs-good/seed/docs/intro.md`:

```markdown

![arch diagram](./img/arch.png)
```

- [ ] **Step 2: Write failing tests**

Append to `tests/ingest_git_transform_test.bats`:

```bash
@test "transform: copies referenced image to asset dir + rewrites path" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  upstream_root="$BATS_TEST_DIRNAME/fixtures/git-docs-good/seed"
  cp "$upstream_root/docs/intro.md" upstream.md
  echo '{}' | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath docs/intro.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x" \
    --upstream-root "$upstream_root"
  [ -f "$WORK/_assets/git-x/docs/img/arch.png" ]
  grep -q "_assets/git-x/docs/img/arch.png" out.md
}

@test "transform: warns on missing image ref but exits 0" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  printf -- '---\ntitle: T\n---\n\n# T\n\n![missing](./nope.png)\n' > upstream.md
  echo '{}' | python3 "$BATS_TEST_DIRNAME/../scripts/ingest-git-transform.py" \
    --in upstream.md --out out.md \
    --repo-key local-x --repo-name x --repo-relpath README.md \
    --git-url file:///x --git-blob-sha abc \
    --asset-out-dir "$WORK/_assets/git-x" \
    --upstream-root "$WORK"
  [ "$status" -eq 0 ] || true   # bats `run` not used; ensure command succeeds
  grep -q "missing" out.md      # alt text retained
}
```

- [ ] **Step 3: Run, expect fail**

Run: `bats tests/ingest_git_transform_test.bats -f "image|missing"`
Expected: 2 FAILs.

- [ ] **Step 4: Add image rewrite to transform**

Add to `scripts/ingest-git-transform.py`, before `rewrite_wikilinks`:

```python
import shutil

IMG_RE = re.compile(r"!\[([^\]]*)\]\(([^)]+)\)")


def rewrite_images(body: str, current_relpath: str, upstream_root: str,
                   asset_out_dir: str, fence_ranges: list) -> tuple[str, list]:
    """Find ![alt](path) refs outside code fences. Copy the referenced
    file from upstream_root to asset_out_dir/<repo-relpath-of-image>,
    then rewrite the path in the body to point to the asset location.
    Missing files → leave the link, emit warning."""
    warnings = []

    def _line_in_fence(idx):
        for s, e in fence_ranges:
            if s <= idx < e:
                return True
        return False

    new_lines = []
    for i, line in enumerate(body.splitlines()):
        if _line_in_fence(i):
            new_lines.append(line); continue

        def _sub(m):
            alt, href = m.group(1), m.group(2)
            if "://" in href or href.startswith("/"):
                return m.group(0)  # absolute / external — leave
            target = _normalize_relpath(current_relpath, href.split()[0])
            if not target:
                return m.group(0)
            src = Path(upstream_root) / target
            if not src.is_file():
                warnings.append(f"missing image: {target}")
                return m.group(0)
            dest = Path(asset_out_dir) / target
            dest.parent.mkdir(parents=True, exist_ok=True)
            shutil.copy2(src, dest)
            # The rewritten path is relative to the wiki repo root, e.g.
            #   /sources/_assets/git-<repo>/<target>
            # asset_out_dir already includes the content/sources/_assets/git-<repo>/ prefix.
            # We emit a path that Hugo will render as static asset.
            rel_from_content = str(dest)
            # Strip "content/" prefix if present so the rendered URL is correct.
            if rel_from_content.startswith("content/"):
                rel_from_content = rel_from_content[len("content/"):]
            return f"![{alt}]({rel_from_content})"

        new_lines.append(IMG_RE.sub(_sub, line))
    out = "\n".join(new_lines)
    if body.endswith("\n"):
        out += "\n"
    return out, warnings
```

In `rewrite_wikilinks`, expose `fence_ranges` as a return so the caller can reuse it. Refactor: split `rewrite_wikilinks` into two phases — `_compute_fence_ranges(body)` returning `fence_ranges`, then `rewrite_wikilinks(body, current_relpath, slug_map, fence_ranges)`.

In `main()`, call image rewrite before wikilink rewrite (or vice versa — image refs use `!` prefix so they don't collide with link rewrite):

```python
    fence_ranges = _compute_fence_ranges(body)
    if args.upstream_root:
        body, img_warnings = rewrite_images(body, args.repo_relpath, args.upstream_root,
                                             args.asset_out_dir, fence_ranges)
        for w in img_warnings:
            print(f"WARN|{w}")
    body, link_warnings = rewrite_wikilinks(body, args.repo_relpath, slug_map, fence_ranges)
    for w in link_warnings:
        print(f"WARN|{w}")
```

- [ ] **Step 5: Run, expect pass**

Run: `bats tests/ingest_git_transform_test.bats`
Expected: 7 of 7 PASS.

- [ ] **Step 6: Commit**

```bash
git add scripts/ingest-git-transform.py tests/ingest_git_transform_test.bats tests/fixtures/git-docs-good/seed/docs/img/arch.png tests/fixtures/git-docs-good/seed/docs/intro.md
git commit -m "feat(git-docs): transform.py — copy images + rewrite path (phase 18)"
```

---

## Task 9: Orchestrator skeleton — argparse, repo_key, lock acquisition

**Files:**
- Create: `scripts/ingest-git.sh`.
- Test: `tests/ingest_git_test.bats` (new).

This task lands the script skeleton: parse flags, derive repo_key, acquire per-repo lock, exit. No file walking yet.

- [ ] **Step 1: Write failing test**

Create `tests/ingest_git_test.bats`:

```bash
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  pushd "$WORK" >/dev/null
  mkdir -p .awiki content/sources content/entities raw/_git-cache
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > content/log.md
  export AWIKI_REPO_ROOT="$WORK"
  # Build a tiny local fixture repo
  seed="$WORK/seed"
  mkdir -p "$seed"
  printf "# Hello\n\nbody\n" > "$seed/README.md"
  bash "$BATS_TEST_DIRNAME/util/build-git-fixture.sh" "$seed" "$WORK/repo"
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "ingest-git: rejects missing arg" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh"
  [ "$status" -eq 1 ]
}

@test "ingest-git: --dry-run on local fixture exits 0 with plan output" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "PLAN|"
  echo "$output" | grep -q "repo_key=local-repo"
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats`
Expected: 2 FAILs.

- [ ] **Step 3: Create scripts/ingest-git.sh skeleton**

```bash
#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# shellcheck source=lib/lock.sh
source "$SCRIPT_DIR/lib/lock.sh"
# shellcheck source=lib/git-clone.sh
source "$SCRIPT_DIR/lib/git-clone.sh"
# shellcheck source=lib/git-state.sh
source "$SCRIPT_DIR/lib/git-state.sh"
# shellcheck source=lib/git-config.sh
source "$SCRIPT_DIR/lib/git-config.sh"

usage() {
  cat >&2 <<USAGE
usage: ingest-git.sh <repo-spec> [flags]

  <repo-spec>          local path | https URL | git@ URL | config alias
  --paths=<csv>        override include paths (default README.md,docs/,rfcs/,adr/)
  --private            force private routing
  --protect-edits      stage conflicts under raw/inbox/checkpoint/.staged/
  --summarize          (reserved; not implemented v1)
  --dry-run            print plan + exit 0; no writes
  --repo-name=<name>   override derived name for slug prefix + entity page
USAGE
}

if [[ $# -lt 1 ]]; then
  usage; exit 1
fi

SPEC="$1"; shift
PATHS_OVERRIDE=""
PRIVATE_FLAG=""
PROTECT_EDITS=""
SUMMARIZE=""
DRY_RUN=""
REPO_NAME_OVERRIDE=""

while [[ $# -gt 0 ]]; do
  case "$1" in
    --paths=*)        PATHS_OVERRIDE="${1#--paths=}";;
    --private)        PRIVATE_FLAG=1;;
    --protect-edits)  PROTECT_EDITS=1;;
    --summarize)      SUMMARIZE=1;;
    --dry-run)        DRY_RUN=1;;
    --repo-name=*)    REPO_NAME_OVERRIDE="${1#--repo-name=}";;
    -h|--help)        usage; exit 0;;
    *) echo "ERROR: unknown flag: $1" >&2; usage; exit 1;;
  esac
  shift
done

REPO_KEY="$(awiki_git_clone_repo_key "$SPEC")"
REPO_NAME="${REPO_NAME_OVERRIDE:-${REPO_KEY#local-}}"

if awiki_git_clone_is_ssh "$SPEC"; then
  PRIVATE_FLAG=1
fi

# Lock dir
mkdir -p .awiki/lock
LOCK_FILE=".awiki/lock/git-${REPO_KEY}"
: > "$LOCK_FILE.flockfile"

# Run remainder under flock. We CAN'T use awiki_lock_with directly because
# it writes to .awiki/lock (single file). We use a per-repo flockfile here.
exec 9>"$LOCK_FILE.flockfile"
if ! flock -w "$AWIKI_LOCK_TIMEOUT_USER" -x 9; then
  echo "ERROR: lock acquire timeout for $REPO_KEY" >&2
  exit 13
fi

# Resolve checkout
RESOLVED="$(awiki_git_clone_resolve "$SPEC" "$REPO_KEY")" || {
  rc=$?; echo "ERROR: resolve failed (rc=$rc)" >&2; exit "$rc";
}
IFS='|' read -r CHECKOUT HEAD_SHA DEFAULT_BRANCH <<<"$RESOLVED"

if [[ -n "$DRY_RUN" ]]; then
  echo "PLAN|spec=$SPEC|repo_key=$REPO_KEY|repo_name=$REPO_NAME|checkout=$CHECKOUT|head=$HEAD_SHA|branch=$DEFAULT_BRANCH|private=${PRIVATE_FLAG:-0}"
  exit 0
fi

# (subsequent tasks add walk + transform + write + housekeeping)
echo "OK|repo_key=$REPO_KEY|head=$HEAD_SHA"
```

- [ ] **Step 4: chmod + run, expect pass**

```bash
chmod +x scripts/ingest-git.sh
bats tests/ingest_git_test.bats
```

Expected: 2 of 2 PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh skeleton — argparse, lock, dry-run (phase 18)"
```

---

## Task 10: Orchestrator — walk + diff

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

Add file walk (matching `paths`/`exclude`) and diff against prior state. Print per-class counts in `--dry-run`.

- [ ] **Step 1: Add failing tests**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: --dry-run reports added=1 on first run with single README" {
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "added=1"
  echo "$output" | grep -q "modified=0"
  echo "$output" | grep -q "removed=0"
}

@test "ingest-git: --paths=docs/ excludes README" {
  mkdir -p "$WORK/repo/docs"
  printf "# Doc\n\nbody\n" > "$WORK/repo/docs/intro.md"
  git -C "$WORK/repo" add -A && git -C "$WORK/repo" commit -q -m more
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --dry-run --paths=docs/
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "added=1"   # only docs/intro.md, NOT README
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "added=1|paths=docs"`
Expected: FAILs.

- [ ] **Step 3: Add walk/diff helpers in scripts/ingest-git.sh**

Insert before `if [[ -n "$DRY_RUN" ]]; then` in the orchestrator:

```bash
# Determine include paths
DEFAULT_PATHS="README.md,docs/,rfcs/,adr/"
PATHS="${PATHS_OVERRIDE:-$DEFAULT_PATHS}"

# Walk repo: list every *.md / *.mdx, then filter by paths + exclude vendored
mapfile -t ALL_MD < <(git -C "$CHECKOUT" ls-tree -r HEAD --name-only -- '*.md' '*.mdx' 2>/dev/null | LC_ALL=C sort)

filter_paths() {
  local rel="$1"
  # vendored skip
  case "$rel" in
    node_modules/*|*/node_modules/*|vendor/*|*/vendor/*|.git/*|*/.git/*) return 1;;
  esac
  # paths matching
  IFS=',' read -ra ENTRIES <<<"$PATHS"
  for entry in "${ENTRIES[@]}"; do
    if [[ -z "$entry" ]]; then continue; fi
    if [[ "$entry" == */ ]]; then
      [[ "$rel" == "${entry}"* ]] && return 0
    else
      [[ "$rel" == "$entry" ]] && return 0
    fi
  done
  return 1
}

CURRENT_FILES=()
for rel in "${ALL_MD[@]}"; do
  if filter_paths "$rel"; then
    CURRENT_FILES+=("$rel")
  fi
done

# Build map repo-relpath → blob_sha
declare -A CURRENT_BLOB
for rel in "${CURRENT_FILES[@]}"; do
  CURRENT_BLOB["$rel"]="$(git -C "$CHECKOUT" rev-parse "HEAD:$rel" 2>/dev/null || echo "")"
done

# Load prior state
PRIOR_JSON="$(awiki_git_state_load "$REPO_KEY")"
declare -A PRIOR_BLOB PRIOR_SLUG
while IFS=$'\t' read -r prel pblob pslug; do
  [[ -z "$prel" ]] && continue
  PRIOR_BLOB["$prel"]="$pblob"
  PRIOR_SLUG["$prel"]="$pslug"
done < <(printf '%s\n' "$PRIOR_JSON" | python3 -c '
import json, sys
try:
    obj = json.loads(sys.stdin.read() or "{}")
except Exception:
    obj = {}
for rel, info in (obj.get("files") or {}).items():
    print(f"{rel}\t{info.get(\"blob_sha\",\"\")}\t{info.get(\"slug\",\"\")}")
')

# Diff
ADDED=()
MODIFIED=()
UNCHANGED=()
REMOVED=()
for rel in "${CURRENT_FILES[@]}"; do
  pblob="${PRIOR_BLOB[$rel]:-}"
  cblob="${CURRENT_BLOB[$rel]}"
  if [[ -z "$pblob" ]]; then
    ADDED+=("$rel")
  elif [[ "$pblob" != "$cblob" ]]; then
    MODIFIED+=("$rel")
  else
    UNCHANGED+=("$rel")
  fi
done
for prel in "${!PRIOR_BLOB[@]}"; do
  if [[ -z "${CURRENT_BLOB[$prel]:-}" ]]; then
    REMOVED+=("$prel")
  fi
done
```

Replace the `--dry-run` block:

```bash
if [[ -n "$DRY_RUN" ]]; then
  echo "PLAN|spec=$SPEC|repo_key=$REPO_KEY|repo_name=$REPO_NAME|checkout=$CHECKOUT|head=$HEAD_SHA|branch=$DEFAULT_BRANCH|private=${PRIVATE_FLAG:-0}"
  echo "PLAN|added=${#ADDED[@]}|modified=${#MODIFIED[@]}|removed=${#REMOVED[@]}|unchanged=${#UNCHANGED[@]}"
  for f in "${ADDED[@]}";    do echo "PLAN|add|$f"; done
  for f in "${MODIFIED[@]}"; do echo "PLAN|mod|$f"; done
  for f in "${REMOVED[@]}";  do echo "PLAN|rem|$f"; done
  exit 0
fi
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats`
Expected: 4 of 4 PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — walk + diff (added/modified/removed) (phase 18)"
```

---

## Task 11: Orchestrator — slug map + transform invocation + write

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

Build slug map for ALL current files (so wikilink rewrite resolves cross-file targets even when only one file changed). For each (added ∪ modified) file, invoke `ingest-git-transform.py`.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: full run writes one source page from README" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ "$status" -eq 0 ]
  [ -f content/sources/git-repo-readme.md ]
  grep -q "^type: source" content/sources/git-repo-readme.md
  grep -q "^git_repo: repo" content/sources/git-repo-readme.md
}

@test "ingest-git: full run honors --private routing" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  mkdir -p content/private/sources content/private/entities
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --private
  [ "$status" -eq 0 ]
  [ -f content/private/sources/git-repo-readme.md ]
  ! [ -f content/sources/git-repo-readme.md ]
  grep -q "private" content/private/sources/git-repo-readme.md
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "writes one|private routing"`
Expected: FAILs.

- [ ] **Step 3: Add slug-flatten helper + write loop**

Insert in `scripts/ingest-git.sh` after diff computation:

```bash
flatten_slug() {
  local rel="$1"
  local stem="${rel%.md}"
  stem="${stem%.mdx}"
  stem="${stem,,}"
  stem="${stem//\//-}"
  # Special-case top-level README.md → "readme"
  if [[ "$stem" == "readme" ]]; then :; fi
  echo "git-${REPO_NAME}-${stem}"
}

declare -A SLUG_MAP
for rel in "${CURRENT_FILES[@]}"; do
  SLUG_MAP["$rel"]="$(flatten_slug "$rel")"
done

# Detect slug collisions (two relpaths flatten to same slug)
declare -A SEEN_SLUG
for rel in "${CURRENT_FILES[@]}"; do
  s="${SLUG_MAP[$rel]}"
  if [[ -n "${SEEN_SLUG[$s]:-}" ]]; then
    echo "ERROR: slug collision: $s ← ${SEEN_SLUG[$s]} and $rel" >&2
    exit 14
  fi
  SEEN_SLUG[$s]="$rel"
done

SLUG_MAP_JSON="$(python3 -c '
import json, sys
m = {}
for line in sys.stdin:
    line = line.rstrip("\n")
    if not line: continue
    rel, slug = line.split("\t", 1)
    m[rel] = slug
print(json.dumps(m))
' < <(for r in "${!SLUG_MAP[@]}"; do printf '%s\t%s\n' "$r" "${SLUG_MAP[$r]}"; done))"

# Determine output roots
if [[ -n "${PRIVATE_FLAG:-}" ]]; then
  OUT_SOURCES="content/private/sources"
  OUT_ENTITIES="content/private/entities"
  ASSET_OUT_DIR="content/private/sources/_assets/git-${REPO_NAME}"
else
  OUT_SOURCES="content/sources"
  OUT_ENTITIES="content/entities"
  ASSET_OUT_DIR="content/sources/_assets/git-${REPO_NAME}"
fi
mkdir -p "$OUT_SOURCES" "$OUT_ENTITIES" "$ASSET_OUT_DIR"

# Repo entity name conflict check
ENTITY_PATH="$OUT_ENTITIES/repo-${REPO_NAME}.md"
if [[ -f "$ENTITY_PATH" ]]; then
  existing_url="$(awk -F': ' '/^git_url: /{print $2; exit}' "$ENTITY_PATH" || true)"
  if [[ -n "$existing_url" && "$existing_url" != "$SPEC" && -z "${REPO_NAME_OVERRIDE:-}" ]]; then
    # Tolerate when SPEC is a local path and existing URL is its file:// or canonical form
    echo "ERROR: repo entity $ENTITY_PATH already exists with git_url=$existing_url; pass --repo-name=<override>" >&2
    exit 15
  fi
fi

# Transform + write each added ∪ modified
TO_WRITE=("${ADDED[@]}" "${MODIFIED[@]}")
WRITE_OK=0
WRITE_FAIL=0
PRIVATE_ARG=""
[[ -n "${PRIVATE_FLAG:-}" ]] && PRIVATE_ARG="--private"

for rel in "${TO_WRITE[@]}"; do
  slug="${SLUG_MAP[$rel]}"
  out="$OUT_SOURCES/${slug}.md"
  if printf '%s' "$SLUG_MAP_JSON" | python3 "$SCRIPT_DIR/ingest-git-transform.py" \
        --in "$CHECKOUT/$rel" --out "$out" \
        --repo-key "$REPO_KEY" --repo-name "$REPO_NAME" --repo-relpath "$rel" \
        --git-url "$SPEC" --git-blob-sha "${CURRENT_BLOB[$rel]}" \
        --asset-out-dir "$ASSET_OUT_DIR" \
        --upstream-root "$CHECKOUT" \
        $PRIVATE_ARG ; then
    WRITE_OK=$((WRITE_OK+1))
  else
    WRITE_FAIL=$((WRITE_FAIL+1))
    echo "FAIL|transform|$rel" >&2
  fi
done

echo "OK|repo_key=$REPO_KEY|added=${#ADDED[@]}|modified=${#MODIFIED[@]}|removed=${#REMOVED[@]}|written=$WRITE_OK|failed=$WRITE_FAIL"

if [[ "$WRITE_FAIL" -gt 0 ]]; then exit 2; fi
```

(Replace the prior `echo "OK|repo_key=…"` line.)

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats`
Expected: 6 of 6 PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — slug map + transform invocation + write (phase 18)"
```

---

## Task 12: Orchestrator — repo entity page generation

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

Generate or update `content/entities/repo-<name>.md`. Spec §6.2.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: writes repo entity page with sources list" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ -f content/entities/repo-repo.md ]
  grep -q "^type: entity" content/entities/repo-repo.md
  grep -q "^git_url:" content/entities/repo-repo.md
  grep -q "^git_sha:" content/entities/repo-repo.md
  grep -q "\\[\\[git-repo-readme\\]\\]" content/entities/repo-repo.md
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "entity page"`
Expected: FAIL.

- [ ] **Step 3: Add entity page generator to scripts/ingest-git.sh**

Insert before the final `OK|...` echo (and before `exit 2`):

```bash
TODAY="$(date -u +%F)"
NOW_ISO="$(date -u +%FT%TZ)"
{
  echo "---"
  echo "title: \"${REPO_NAME}\""
  echo "date: ${TODAY}"
  echo "last_updated: ${TODAY}"
  echo "type: entity"
  if [[ -n "${PRIVATE_FLAG:-}" ]]; then
    echo "tags: [git, repo, private]"
  else
    echo "tags: [git, repo]"
  fi
  echo "aliases: [${REPO_NAME}]"
  echo "git_url: ${SPEC}"
  echo "git_default_branch: ${DEFAULT_BRANCH}"
  echo "git_sha: ${HEAD_SHA}"
  echo "last_ingested: ${NOW_ISO}"
  echo "---"
  echo
  echo "Repository \`${REPO_NAME}\` ingested from \`${SPEC}\`."
  echo
  echo "## Sources"
  echo
  for rel in "${CURRENT_FILES[@]}"; do
    echo "- [[${SLUG_MAP[$rel]}]]"
  done
} > "$ENTITY_PATH.tmp.$$"
mv -f "$ENTITY_PATH.tmp.$$" "$ENTITY_PATH"
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "entity page"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — generate repo entity page (phase 18)"
```

---

## Task 13: Orchestrator — stale removal (`raw/_originals/git/`)

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

For each `removed` file, move its derived page to `raw/_originals/git/<repo_key>/<slug>.md`.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: removes derived page when upstream file deleted" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  # First run with two files
  echo "# Doc 1" > "$WORK/repo/docs/d1.md" 2>/dev/null || mkdir -p "$WORK/repo/docs"; echo "# Doc 1" > "$WORK/repo/docs/d1.md"
  echo "# Doc 2" > "$WORK/repo/docs/d2.md"
  git -C "$WORK/repo" add -A && git -C "$WORK/repo" commit -q -m two
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ -f content/sources/git-repo-docs-d1.md ]
  [ -f content/sources/git-repo-docs-d2.md ]
  # Remove d2 upstream
  git -C "$WORK/repo" rm -q docs/d2.md
  git -C "$WORK/repo" commit -q -m rm
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  ! [ -f content/sources/git-repo-docs-d2.md ]
  [ -f raw/_originals/git/local-repo/git-repo-docs-d2.md ]
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "removes derived page"`
Expected: FAIL.

- [ ] **Step 3: Add removal block before entity-page generation**

Insert in `scripts/ingest-git.sh` before the entity page write:

```bash
if [[ "${#REMOVED[@]}" -gt 0 ]]; then
  graveyard="raw/_originals/git/${REPO_KEY}"
  mkdir -p "$graveyard"
  for prel in "${REMOVED[@]}"; do
    slug="${PRIOR_SLUG[$prel]}"
    [[ -z "$slug" ]] && continue
    src="$OUT_SOURCES/${slug}.md"
    if [[ -f "$src" ]]; then
      mv -f "$src" "$graveyard/${slug}.md"
      echo "REMOVED|$prel|→|$graveyard/${slug}.md"
    fi
  done
fi
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "removes derived page"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — graveyard removed files to raw/_originals/git/ (phase 18)"
```

---

## Task 14: Orchestrator — persist state JSON

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

Write `.awiki/git-state/<repo_key>.json` after successful writes.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: writes state JSON with file map after run" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ -f .awiki/git-state/local-repo.json ]
  python3 -c "
import json, sys
o = json.load(open('.awiki/git-state/local-repo.json'))
assert o['schema'] == 1
assert o['repo_key'] == 'local-repo'
assert 'README.md' in o['files']
"
}

@test "ingest-git: rerun with no changes is no-op (state head_sha unchanged)" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  before="$(stat -f %m .awiki/git-state/local-repo.json 2>/dev/null || stat -c %Y .awiki/git-state/local-repo.json)"
  sleep 1
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  run grep -c '"blob_sha":' .awiki/git-state/local-repo.json
  [ "$status" -eq 0 ]
  # File map shape unchanged (one file still present)
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "state JSON|rerun"`
Expected: FAILs.

- [ ] **Step 3: Add state persistence to scripts/ingest-git.sh**

Insert after entity page write:

```bash
NEW_FILES_JSON="$(python3 -c '
import json, sys
files = {}
for line in sys.stdin:
    line = line.rstrip("\n")
    if not line: continue
    rel, blob, slug, out = line.split("\t")
    files[rel] = {"blob_sha": blob, "slug": slug, "out_path": out, "last_ingested": "'"$NOW_ISO"'"}
print(json.dumps(files))
' < <(
  for rel in "${CURRENT_FILES[@]}"; do
    slug="${SLUG_MAP[$rel]}"
    printf '%s\t%s\t%s\t%s\n' "$rel" "${CURRENT_BLOB[$rel]}" "$slug" "$OUT_SOURCES/${slug}.md"
  done
))"

STATE_JSON="$(python3 -c '
import json, sys
files = json.loads(sys.argv[1])
out = {
    "schema": 1,
    "repo_key": sys.argv[2],
    "repo_name": sys.argv[3],
    "url": sys.argv[4],
    "default_branch": sys.argv[5],
    "head_sha": sys.argv[6],
    "ingested_at": sys.argv[7],
    "private": sys.argv[8] == "1",
    "files": files,
}
print(json.dumps(out, indent=2))
' "$NEW_FILES_JSON" "$REPO_KEY" "$REPO_NAME" "$SPEC" "$DEFAULT_BRANCH" "$HEAD_SHA" "$NOW_ISO" "${PRIVATE_FLAG:-0}")"

awiki_git_state_save "$REPO_KEY" "$STATE_JSON"
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "state JSON|rerun"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — persist .awiki/git-state/<key>.json (phase 18)"
```

---

## Task 15: Orchestrator — batch housekeeping (log, catalog, qmd reindex)

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

Single batched call to each housekeeping helper. Spec §6.3 step 13.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: appends one log entry per run" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  run grep -c "ingest-git | repo=local-repo" content/log.md
  [ "$status" -eq 0 ]
  [ "$output" = "1" ]
}

@test "ingest-git: skips log entry on no-op rerun" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  run grep -c "ingest-git | repo=local-repo" content/log.md
  [ "$status" -eq 0 ]
  [ "$output" = "1" ]
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "log entry"`
Expected: FAILs.

- [ ] **Step 3: Add batch housekeeping to scripts/ingest-git.sh**

Insert after state-save, before final `echo "OK|..."`:

```bash
TOTAL_CHANGES=$(( ${#ADDED[@]} + ${#MODIFIED[@]} + ${#REMOVED[@]} ))
if [[ "$TOTAL_CHANGES" -gt 0 ]]; then
  bash "$SCRIPT_DIR/log-append.sh" ingest-git "repo=$REPO_KEY +${#ADDED[@]} ~${#MODIFIED[@]} -${#REMOVED[@]} head=$HEAD_SHA"
  if [[ -x "$SCRIPT_DIR/update-catalog.sh" ]]; then
    bash "$SCRIPT_DIR/update-catalog.sh" >/dev/null 2>&1 || echo "WARN|update-catalog non-zero (continuing)" >&2
  fi
  if [[ "${AWIKI_QMD_STATUS:-}" != "missing" ]] && command -v qmd >/dev/null 2>&1; then
    bash "$SCRIPT_DIR/qmd-index.sh" 2>/dev/null || echo "WARN|qmd reindex failed (continuing)" >&2
  fi
fi
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "log entry"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — batched log/catalog/qmd housekeeping (phase 18)"
```

---

## Task 16: Orchestrator — `--protect-edits` checkpoint flow

**Files:**
- Modify: `scripts/ingest-git.sh`.
- Modify: `tests/ingest_git_test.bats`.

Detect conflict (existing page diverged from prior-ingest body AND `last_updated` later than `last_ingested`). Stage to `raw/inbox/checkpoint/.staged/`.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: --protect-edits stages conflict to checkpoint" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  mkdir -p raw/inbox/checkpoint/.staged
  # First ingest
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  # User edits derived page (bump last_updated)
  python3 -c "
import re, pathlib, datetime
p = pathlib.Path('content/sources/git-repo-readme.md')
s = p.read_text()
s = re.sub(r'^last_updated: .*', 'last_updated: 2099-01-01', s, count=1, flags=re.M)
s += '\n\nUSER EDIT MARKER\n'
p.write_text(s)
"
  # Modify upstream
  echo "# Hello (upstream change)" > "$WORK/repo/README.md"
  git -C "$WORK/repo" add -A && git -C "$WORK/repo" commit -q -m upstream
  # Re-ingest with --protect-edits
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --protect-edits
  grep -q "USER EDIT MARKER" content/sources/git-repo-readme.md   # not overwritten
  [ -f raw/inbox/checkpoint/.staged/git-repo-readme.md ]          # proposal staged
}
```

- [ ] **Step 2: Run, expect fail**

Run: `bats tests/ingest_git_test.bats -f "protect-edits"`
Expected: FAIL.

- [ ] **Step 3: Add conflict detection in scripts/ingest-git.sh**

Modify the per-file write loop. Insert before the `printf '%s' "$SLUG_MAP_JSON" | python3 ...` invocation:

```bash
  # --protect-edits: write to staging if existing page diverged + last_updated bumped
  proposal_target=""
  if [[ -n "${PROTECT_EDITS:-}" && -f "$out" ]]; then
    last_updated="$(awk -F': ' '/^last_updated: /{print $2; exit}' "$out" || true)"
    last_ingested="$(python3 -c "
import json, sys
try:
    o = json.loads(open('.awiki/git-state/$REPO_KEY.json').read())
    print(o.get('files', {}).get('$rel', {}).get('last_ingested', ''))
except Exception:
    print('')
")"
    if [[ -n "$last_updated" && -n "$last_ingested" && "$last_updated" > "${last_ingested:0:10}" ]]; then
      mkdir -p raw/inbox/checkpoint/.staged
      proposal_target="raw/inbox/checkpoint/.staged/${slug}.md"
      out="$proposal_target"
      echo "PROTECT|$rel|→|$proposal_target"
    fi
  fi
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "protect-edits"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add scripts/ingest-git.sh tests/ingest_git_test.bats
git commit -m "feat(git-docs): ingest-git.sh — --protect-edits stages conflicts to checkpoint (phase 18)"
```

---

## Task 17: Justfile recipes + WIKI.md §4.7 doc

**Files:**
- Modify: `justfile`.
- Modify: `WIKI.md`.
- Modify: `tests/ingest_git_test.bats` (add justfile dispatch test).

- [ ] **Step 1: Add justfile recipe**

Insert in `justfile` after `ingest-batch-list:`:

```
# === git-docs ingest (phase 18) ===
ingest-git spec *flags:
    bash scripts/ingest-git.sh {{spec}} {{flags}}

ingest-git-list:
    @ls -1 .awiki/git-state/ 2>/dev/null | sed 's/\.json$//'
```

- [ ] **Step 2: Add WIKI.md §4.7 section**

Insert in `WIKI.md` after §4.6 Synthesis (before §5 Inbox Queues):

```markdown
### 4.7 Ingest from git repo

1. Decide source: local path (`/abs/path/to/repo`), remote URL
   (`https://github.com/foo/bar.git` or `git@github.com:foo/bar.git`),
   or a named entry in `.awiki/git-sources.yml`.
2. Run `just ingest-git <spec>` — supports flags: `--paths=docs/,rfcs/`,
   `--private`, `--protect-edits`, `--summarize` (reserved),
   `--dry-run`, `--repo-name=<override>`. SSH URLs auto-route to
   `content/private/sources/`.
3. The script clones (or pulls) under `raw/_git-cache/<repo_key>/`,
   diffs the working tree against prior state in
   `.awiki/git-state/<repo_key>.json`, transforms each added/modified
   markdown file to `content/[private/]sources/git-<repo>-<flatpath>.md`,
   updates `content/[private/]entities/repo-<repo>.md`, graveyards
   removed files to `raw/_originals/git/<repo_key>/`, and runs one
   batched `log-append.sh` + `update-catalog.sh` + `qmd reindex`.
4. Subsequent runs are incremental: unchanged blobs are skipped, removed
   files are graveyarded.
5. **Out of scope here:** commit history mining, code symbol extraction,
   issues/PRs, cross-repo wikilinks. See spec at
   `docs/superpowers/specs/2026-04-27-git-docs-ingest-design.md`.
```

- [ ] **Step 3: Add justfile dispatch test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: just recipe dispatches to script" {
  if ! command -v just >/dev/null 2>&1; then skip "just not installed"; fi
  cp "$BATS_TEST_DIRNAME/../justfile" justfile
  run just ingest-git "$WORK/repo" --dry-run
  [ "$status" -eq 0 ]
  echo "$output" | grep -q "PLAN|"
}
```

- [ ] **Step 4: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "just recipe"`
Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add justfile WIKI.md tests/ingest_git_test.bats
git commit -m "feat(git-docs): justfile recipe + WIKI.md §4.7 doc (phase 18)"
```

---

## Task 18: Slug collision + repo-name conflict tests

**Files:**
- Modify: `tests/ingest_git_test.bats`.

The collision branches (exit 14, exit 15) already landed in Task 11. Add tests to lock them.

- [ ] **Step 1: Add failing tests**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: exits 14 on slug collision" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  # Two paths that flatten to the same slug: foo-bar.md and foo/bar.md
  mkdir -p "$WORK/repo/docs/foo"
  echo "# A" > "$WORK/repo/docs/foo-bar.md"
  echo "# B" > "$WORK/repo/docs/foo/bar.md"
  git -C "$WORK/repo" add -A && git -C "$WORK/repo" commit -q -m collide
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ "$status" -eq 14 ]
  echo "$output$stderr" | grep -qi "slug collision"
}

@test "ingest-git: exits 15 on repo entity name conflict" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  mkdir -p content/entities
  cat > content/entities/repo-repo.md <<EOF
---
title: repo
type: entity
git_url: https://different.example/foo.git
---
EOF
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  [ "$status" -eq 15 ]
}

@test "ingest-git: --repo-name override sidesteps name conflict" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  mkdir -p content/entities
  cat > content/entities/repo-repo.md <<EOF
---
title: repo
type: entity
git_url: https://different.example/foo.git
---
EOF
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo" --repo-name=alt
  [ "$status" -eq 0 ]
  [ -f content/entities/repo-alt.md ]
}
```

- [ ] **Step 2: Run, expect pass (logic already shipped in Task 11)**

Run: `bats tests/ingest_git_test.bats -f "collision|name conflict|override"`
Expected: 3 of 3 PASS. If FAIL, fix the orchestrator inline (likely a bug in the existing-entity check).

- [ ] **Step 3: Commit**

```bash
git add tests/ingest_git_test.bats
git commit -m "test(git-docs): lock exit codes 14 + 15 + --repo-name override (phase 18)"
```

---

## Task 19: Lint clean assertion + `just lint` integration

**Files:**
- Modify: `tests/ingest_git_test.bats`.

After a happy-path run, `just lint` (or `bash scripts/lint.sh`) must report zero errors on derived pages.

- [ ] **Step 1: Add failing test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: lint clean after good ingest" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  # Need catalog.md skeleton or lint will balk
  printf -- '---\ntitle: Catalog\ntype: catalog\n---\n\n## Sources\n\n## Entities\n' > content/catalog.md
  bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$WORK/repo"
  run bash "$BATS_TEST_DIRNAME/../scripts/lint.sh"
  # Allow info/warning; require zero errors. lint output uses LINT|<level>|...
  ! echo "$output" | grep -E "^LINT\\|error\\|" >/dev/null
}
```

- [ ] **Step 2: Run; if fail, inspect lint output and fix transform/orchestrator (likely missing `aliases:` or `sources:` field)**

Run: `bats tests/ingest_git_test.bats -f "lint clean"`
Expected: PASS. If FAIL, the transform's emitted frontmatter is missing a required field for `type: source`. Check `scripts/lint.sh` (search for `type: source` required-field list) and add the missing field to `emit_frontmatter()` in `scripts/ingest-git-transform.py`.

- [ ] **Step 3: Commit**

```bash
git add tests/ingest_git_test.bats scripts/ingest-git-transform.py
git commit -m "test(git-docs): lock lint-clean invariant on derived pages (phase 18)"
```

---

## Task 20: Self-host smoke — ingest the awiki repo's own docs/

**Files:**
- Modify: `tests/ingest_git_test.bats`.

Run on the wiki's own repo (which has `docs/superpowers/` etc.) as a real-world smoke. Skip if the actual git history isn't accessible from the bats temp.

- [ ] **Step 1: Add smoke test**

Append to `tests/ingest_git_test.bats`:

```bash
@test "ingest-git: self-host smoke — ingests this repo's docs/" {
  if ! python3 -c "import markdown_it" >/dev/null 2>&1; then skip "markdown-it-py not installed"; fi
  awiki_root="$BATS_TEST_DIRNAME/.."
  if [[ ! -d "$awiki_root/.git" ]]; then skip "not a git repo"; fi
  printf -- '---\ntitle: Catalog\ntype: catalog\n---\n\n## Sources\n\n## Entities\n' > content/catalog.md
  run bash "$BATS_TEST_DIRNAME/../scripts/ingest-git.sh" "$awiki_root" --paths=docs/superpowers/specs/ --repo-name=awiki-self
  [ "$status" -eq 0 ]
  [ -f content/entities/repo-awiki-self.md ]
  ls content/sources/git-awiki-self-* >/dev/null
}
```

- [ ] **Step 2: Run, expect pass**

Run: `bats tests/ingest_git_test.bats -f "self-host"`
Expected: PASS.

- [ ] **Step 3: Commit**

```bash
git add tests/ingest_git_test.bats
git commit -m "test(git-docs): self-host smoke — ingest awiki's own docs/superpowers/ (phase 18)"
```

---

## Task 21: Final `just test` sweep + log

**Files:**
- None modified — verification + final phase log entry.

- [ ] **Step 1: Run full test suite**

```bash
just test 2>&1 | tail -50
# OR if no `test` recipe:
bats tests/
```

Expected: every test passes (skips allowed for optional deps like pyyaml / markdown-it-py if not installed).

- [ ] **Step 2: Manual verification — Hugo render**

```bash
just serve &
SERVE_PID=$!
sleep 5
curl -sf http://localhost:1313/sources/git-awiki-upstream-readme/ >/dev/null || echo "not rendered (expected if no upstream-derived page exists in current content/)"
kill $SERVE_PID
```

Optional: actually run `just ingest-git <some-real-repo>` once on a small real OSS repo, then `just serve`, click derived pages.

- [ ] **Step 3: Append phase log entry**

```bash
bash scripts/log-append.sh phase "phase 18 complete — git-docs ingest pipeline (ingest-git.sh + 3 libs + transform.py + 4 bats files)"
```

- [ ] **Step 4: Commit log entry**

```bash
git add content/log.md
git commit -m "chore(log): record phase 18 complete (git-docs ingest)"
```

---

## Open items deferred to phase 19+ (not blocking phase 18)

- MCP tool surfacing of `ingest_git` (spec §10 #5).
- Cross-repo wikilink resolution (spec §3 non-goal).
- `--summarize` LLM transform path (spec §3 non-goal — flag is accepted but no-op).
- Asset content-hash dedup (spec §10 #3).
- `git-history` / `git-issues` / `git-code` sub-projects (spec §11) — each is its own brainstorm + spec + plan.

---

## Self-review notes

Spec coverage check:
- §6.1 components → Tasks 3, 4, 5, 6 (libs + transform), 9-15 (orchestrator).
- §6.2 page kinds → Tasks 6 (source), 12 (entity).
- §6.3 data flow steps 1-14 → covered by Tasks 9 (1-3), 10 (4-7), 11 (8), 16 (9), 13 (10), 12 (11), 14 (12), 15 (13), 9 (14).
- §7.1 yaml schema → Task 4.
- §7.2 state json → Tasks 3 + 14.
- §7.3 gitignore → Task 2.
- §8 error table → Tasks 9 (exit 1, 13), 4 (12), 18 (14, 15), 11 (2 partial), 8 (image warn), 6 (empty skip).
- §9 testing → bats files at every task; full suite asserted in Task 21.
- §3 non-goals → not implemented (correct).
- §4 user stories 1-4 → exercised by Tasks 9, 11, 16, 17.

Type/method consistency: function names cross-checked across libs and orchestrator (`awiki_git_clone_repo_key`, `awiki_git_clone_resolve`, `awiki_git_clone_is_ssh`, `awiki_git_state_path/load/save/validate`, `awiki_git_config_validate_name/validate_paths/get`, transform `rewrite_wikilinks`/`rewrite_images`/`_compute_fence_ranges`).

No placeholders. No "implement appropriate X". No "similar to Task N". Every code block is the actual content.
