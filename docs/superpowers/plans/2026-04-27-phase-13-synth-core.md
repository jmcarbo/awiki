# awiki Plan — Phase 13: Synth Core

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-synthesis-generator-design.md`](../specs/2026-04-27-synthesis-generator-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phases 1, 2, 3, 6, 12
**Previous:** [Phase 12](./2026-04-27-phase-12-polish-examples.md)
**Next:** [Phase 14](./2026-04-27-phase-14-synth-lint-plugins.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, PyYAML (stdlib alt: pure-bash YAML parsing acceptable for the small frontmatter shape used).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.
- All shell-out sites in `scripts/synth.sh` use `--` to terminate flag parsing before positional arguments (per spec hardening).
- Slug regex `^[a-z0-9][a-z0-9-]*$` (no leading hyphen). Plugin name regex `^[a-z][a-z0-9-]*$`.
- Marker syntax is `<!-- BEGIN GENERATED ... -->` / `<!-- END GENERATED -->` — never bare `BEGIN`/`END`.
- Never construct `python3 -c '...'` strings by interpolating user-supplied content. Helper scripts read paths only (deferred to phase 14 anyway).

---

**Deliverable:** `scripts/synth.sh` orchestrator (subcommands `new`, `regen`, `accept-stage`, `finalize`, `list`, `resolve`, `refine`); plugin loader (single-file form, with directory-form rejected with a clear "not yet supported" message so phase 14 can light it up); `synthesis-plugins/briefing.md` shipped; scope resolution with fail-closed private-content check; marker scaffolding; `## Notes` preservation; BATS tests for the orchestrator; `tests/fixtures/wiki-synth/` populated with sources covering smart quotes, NBSP, multi-paragraph quotes, hyphen variants, ZWSP, and a `[private]`-tagged source (re-used by phase 14 lint tests); WIKI.md Section 4 sub-workflow "Synthesis (briefing-only)".

**Branch:** `phase-13-synth-core`

---

## Task 13.1: Branch + directory scaffolding

- [ ] **Step 1: Create branch**

```bash
git checkout main
git pull --ff-only
git checkout -b phase-13-synth-core
```

Expected: `Switched to a new branch 'phase-13-synth-core'`.

- [ ] **Step 2: Create new top-level directories**

```bash
mkdir -p synthesis-plugins
mkdir -p content/synthesis/.staged
mkdir -p tests/fixtures/wiki-synth/content/{sources,private}
mkdir -p tests/fixtures/wiki-synth/.awiki/maps
touch synthesis-plugins/.gitkeep
touch content/synthesis/.staged/.gitkeep
```

- [ ] **Step 3: Update `.gitignore` to track `.staged/.gitkeep` only and exclude staged artefacts**

Append to `.gitignore`:

```
# === phase 13: synthesis ===
content/synthesis/.staged/*
!content/synthesis/.staged/.gitkeep
```

- [ ] **Step 4: Commit**

```bash
git add synthesis-plugins/.gitkeep content/synthesis/.staged/.gitkeep tests/fixtures/wiki-synth .gitignore
git commit -m "chore: scaffold synthesis-plugins/ and content/synthesis/.staged/"
```

Expected: one new commit; `git status` clean.

---

## Task 13.2: Plugin loader — `scripts/synth-plugin-load.sh`

The loader is a sourced helper used by every `synth.sh` subcommand that needs a plugin manifest. Single-file form only in phase 13. Directory form is detected and rejected with a "not yet supported" error so phase 14 can drop in its directory-form support without renaming the entry point.

**Files:** Create: `scripts/synth-plugin-load.sh`, `tests/synth_plugin_load_test.bats`, `tests/fixtures/synth-plugins-fixture/briefing.md`, `tests/fixtures/synth-plugins-fixture/bad-name.md`, `tests/fixtures/synth-plugins-fixture/missing-fields.md`, `tests/fixtures/synth-plugins-fixture/dirform/plugin.yaml`.

- [ ] **Step 1: Write fixture plugins**

```bash
mkdir -p tests/fixtures/synth-plugins-fixture/dirform
cat > tests/fixtures/synth-plugins-fixture/briefing.md <<'EOF'
---
name: briefing
description: One-page exec summary.
version: 1
output_type: synthesis
output_subtype: briefing
min_sources: 2
max_sources: 50
max_evidence_total_words: 500
required_sections:
  - "## TL;DR"
  - "## Key Findings"
  - "## Open Questions"
  - "## Evidence"
post_hook: null
---

# Prompt
Body of prompt template here.
EOF

cat > tests/fixtures/synth-plugins-fixture/bad-name.md <<'EOF'
---
name: -bad-name
description: leading hyphen
output_type: synthesis
required_sections: ["## X"]
min_sources: 1
---
EOF

cat > tests/fixtures/synth-plugins-fixture/missing-fields.md <<'EOF'
---
name: missing
description: no required_sections
output_type: synthesis
min_sources: 1
---
EOF

cat > tests/fixtures/synth-plugins-fixture/dirform/plugin.yaml <<'EOF'
name: dirform
description: directory form, phase-14 territory
output_type: synthesis
required_sections: ["## X"]
min_sources: 1
EOF
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/synth_plugin_load_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  PLUGINS_DIR="tests/fixtures/synth-plugins-fixture"
  export AWIKI_SYNTH_PLUGINS_DIR="$PLUGINS_DIR"
}

@test "loader parses briefing manifest" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load briefing && echo "$SYNTH_PLUGIN_NAME|$SYNTH_PLUGIN_OUTPUT_TYPE|$SYNTH_PLUGIN_MIN_SOURCES|$SYNTH_PLUGIN_OUTPUT_SUBTYPE"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"briefing|synthesis|2|briefing"* ]]
}

@test "loader exposes required_sections as newline-separated string" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load briefing && printf -- "%s" "$SYNTH_PLUGIN_REQUIRED_SECTIONS"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"## TL;DR"* ]]
  [[ "$output" == *"## Evidence"* ]]
}

@test "loader rejects plugin name with leading hyphen" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load -bad-name'
  [ "$status" -ne 0 ]
  [[ "$output" == *"invalid plugin name"* ]]
}

@test "loader rejects manifest missing required_sections" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load missing'
  [ "$status" -ne 0 ]
  [[ "$output" == *"required_sections"* ]]
}

@test "loader rejects directory-form plugins in phase 13" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load dirform'
  [ "$status" -ne 0 ]
  [[ "$output" == *"directory-form plugins not yet supported"* ]]
}

@test "loader supplies output_subtype default = name" {
  TMP="$(mktemp -d)"
  cat > "$TMP/nodefault.md" <<EOF2
---
name: nodefault
description: x
output_type: synthesis
required_sections: ["## A"]
min_sources: 1
---
EOF2
  AWIKI_SYNTH_PLUGINS_DIR="$TMP" run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load nodefault && echo "$SYNTH_PLUGIN_OUTPUT_SUBTYPE"'
  [ "$status" -eq 0 ]
  [[ "$output" == "nodefault" ]]
}

@test "loader rejects unknown plugin" {
  run bash -c 'source scripts/synth-plugin-load.sh && synth_plugin_load nonexistent'
  [ "$status" -ne 0 ]
  [[ "$output" == *"plugin not found"* ]]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL — loader absent)**

```bash
bats tests/synth_plugin_load_test.bats
```

Expected: all tests FAIL with `bash: scripts/synth-plugin-load.sh: No such file or directory` or similar.

- [ ] **Step 4: Write `scripts/synth-plugin-load.sh`**

```bash
cat > scripts/synth-plugin-load.sh <<'EOF'
#!/usr/bin/env bash
# Source-only helper. Defines:
#   synth_plugin_load <name>      → sets SYNTH_PLUGIN_* vars; returns 0/1
#   synth_plugin_list_all         → echoes one plugin name per line
#   synth_plugin_extract_prompt   → echoes prompt body to stdout for last loaded plugin
# Caller is responsible for set -e discipline.

SYNTH_PLUGIN_NAME_REGEX='^[a-z][a-z0-9-]*$'

synth_plugin_load() {
  local name="$1"
  local plugins_dir="${AWIKI_SYNTH_PLUGINS_DIR:-synthesis-plugins}"

  # Validate name regex BEFORE any directory scan — defeats traversal attempts.
  if ! [[ "$name" =~ $SYNTH_PLUGIN_NAME_REGEX ]]; then
    echo "ERROR: invalid plugin name '$name' (must match $SYNTH_PLUGIN_NAME_REGEX)" >&2
    return 1
  fi

  local single_file="$plugins_dir/$name.md"
  local dir_form="$plugins_dir/$name/plugin.yaml"

  if [[ -f "$single_file" && -f "$dir_form" ]]; then
    echo "ERROR: plugin '$name' has both single-file and directory form; remove one" >&2
    return 1
  fi
  if [[ -f "$dir_form" ]]; then
    echo "ERROR: directory-form plugins not yet supported (phase 14); use single-file <name>.md" >&2
    return 1
  fi
  if [[ ! -f "$single_file" ]]; then
    echo "ERROR: plugin not found: $name (looked at $single_file)" >&2
    return 1
  fi

  # Reset state from prior loads.
  SYNTH_PLUGIN_NAME=""; SYNTH_PLUGIN_DESCRIPTION=""; SYNTH_PLUGIN_VERSION="1"
  SYNTH_PLUGIN_OUTPUT_TYPE=""; SYNTH_PLUGIN_OUTPUT_SUBTYPE=""
  SYNTH_PLUGIN_MIN_SOURCES=""; SYNTH_PLUGIN_MAX_SOURCES=""
  SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS="500"
  SYNTH_PLUGIN_REQUIRED_SECTIONS=""
  SYNTH_PLUGIN_POST_HOOK=""
  SYNTH_PLUGIN_RENDER=""
  SYNTH_PLUGIN_PATH="$single_file"

  # Extract frontmatter (between first two `---` lines).
  local fm
  fm="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==1 {print}' "$single_file")"

  # Scalar fields via grep+sed (the manifest shape is small, fixed, and our own).
  SYNTH_PLUGIN_NAME="$(printf -- '%s\n' "$fm" | awk -F': ' '/^name: /{print $2; exit}')"
  SYNTH_PLUGIN_DESCRIPTION="$(printf -- '%s\n' "$fm" | awk -F': ' '/^description: /{sub(/^description: /,""); print; exit}')"
  SYNTH_PLUGIN_VERSION="$(printf -- '%s\n' "$fm" | awk -F': ' '/^version: /{print $2; exit}')"
  SYNTH_PLUGIN_OUTPUT_TYPE="$(printf -- '%s\n' "$fm" | awk -F': ' '/^output_type: /{print $2; exit}')"
  SYNTH_PLUGIN_OUTPUT_SUBTYPE="$(printf -- '%s\n' "$fm" | awk -F': ' '/^output_subtype: /{print $2; exit}')"
  SYNTH_PLUGIN_MIN_SOURCES="$(printf -- '%s\n' "$fm" | awk -F': ' '/^min_sources: /{print $2; exit}')"
  SYNTH_PLUGIN_MAX_SOURCES="$(printf -- '%s\n' "$fm" | awk -F': ' '/^max_sources: /{print $2; exit}')"
  SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS="$(printf -- '%s\n' "$fm" | awk -F': ' '/^max_evidence_total_words: /{print $2; exit}')"
  SYNTH_PLUGIN_POST_HOOK="$(printf -- '%s\n' "$fm" | awk -F': ' '/^post_hook: /{print $2; exit}')"
  SYNTH_PLUGIN_RENDER="$(printf -- '%s\n' "$fm" | awk -F': ' '/^render: /{print $2; exit}')"

  # required_sections is a YAML list. Two accepted forms: inline `[a, b]` or block `- "a"`.
  SYNTH_PLUGIN_REQUIRED_SECTIONS="$(printf -- '%s\n' "$fm" | awk '
    BEGIN{ in_list=0 }
    /^required_sections:[[:space:]]*\[/ {
      line=$0; sub(/^required_sections:[[:space:]]*\[/,"",line); sub(/\][[:space:]]*$/,"",line)
      n=split(line, parts, ",")
      for (i=1; i<=n; i++) {
        gsub(/^[[:space:]]*"?|"?[[:space:]]*$/, "", parts[i])
        gsub(/^[[:space:]]*'\''?|'\''?[[:space:]]*$/, "", parts[i])
        if (parts[i] != "") print parts[i]
      }
      next
    }
    /^required_sections:[[:space:]]*$/ { in_list=1; next }
    in_list==1 && /^[^[:space:]-]/ { in_list=0 }
    in_list==1 && /^[[:space:]]+-[[:space:]]+/ {
      line=$0
      sub(/^[[:space:]]+-[[:space:]]+/, "", line)
      gsub(/^["'\'']|["'\'']$/, "", line)
      print line
    }
  ')"

  # Defaults
  [[ -z "$SYNTH_PLUGIN_VERSION" ]] && SYNTH_PLUGIN_VERSION="1"
  [[ -z "$SYNTH_PLUGIN_OUTPUT_SUBTYPE" ]] && SYNTH_PLUGIN_OUTPUT_SUBTYPE="$SYNTH_PLUGIN_NAME"
  [[ -z "$SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS" ]] && SYNTH_PLUGIN_MAX_EVIDENCE_TOTAL_WORDS="500"
  [[ "$SYNTH_PLUGIN_POST_HOOK" == "null" ]] && SYNTH_PLUGIN_POST_HOOK=""

  # Required-field validation.
  for f in NAME DESCRIPTION OUTPUT_TYPE MIN_SOURCES; do
    local var="SYNTH_PLUGIN_$f"
    if [[ -z "${!var:-}" ]]; then
      echo "ERROR: plugin manifest $name missing required field: $(echo "$f" | tr 'A-Z_' 'a-z-' | tr '_' '-')" >&2
      return 1
    fi
  done
  if [[ -z "$SYNTH_PLUGIN_REQUIRED_SECTIONS" ]]; then
    echo "ERROR: plugin manifest $name missing required field: required_sections" >&2
    return 1
  fi
  if [[ "$SYNTH_PLUGIN_NAME" != "$name" ]]; then
    echo "ERROR: plugin filename ($name.md) does not match manifest name ($SYNTH_PLUGIN_NAME)" >&2
    return 1
  fi

  return 0
}

synth_plugin_extract_prompt() {
  # Echo body after second `---` line. Caller must have run synth_plugin_load first.
  if [[ -z "${SYNTH_PLUGIN_PATH:-}" ]]; then
    echo "ERROR: no plugin loaded" >&2
    return 1
  fi
  awk 'BEGIN{c=0} /^---$/ {c++; next} c>=2 {print}' "$SYNTH_PLUGIN_PATH"
}

synth_plugin_list_all() {
  local plugins_dir="${AWIKI_SYNTH_PLUGINS_DIR:-synthesis-plugins}"
  if [[ ! -d "$plugins_dir" ]]; then return 0; fi
  find "$plugins_dir" -maxdepth 1 -type f -name '*.md' -print0 \
    | xargs -0 -n1 basename \
    | sed 's/\.md$//' \
    | sort
}
EOF
chmod +x scripts/synth-plugin-load.sh
```

- [ ] **Step 5: Run test (expect PASS)**

```bash
bats tests/synth_plugin_load_test.bats
```

Expected: 7 tests pass.

- [ ] **Step 6: Commit**

```bash
git add scripts/synth-plugin-load.sh tests/synth_plugin_load_test.bats tests/fixtures/synth-plugins-fixture
git commit -m "feat(synth): add plugin loader (single-file form) with name validation"
```

---

## Task 13.3: Test fixture wiki under `tests/fixtures/wiki-synth/`

The fixture wiki is referenced by phase 14 lint tests for the full normalization matrix. Phase 13 ships the fixture and only uses the simple slug-resolution paths from it.

**Files:** Create: `tests/fixtures/wiki-synth/content/sources/{s1,s2,s3,s-smart-quotes,s-nbsp,s-multipara,s-hyphens,s-zwsp}.md`, `tests/fixtures/wiki-synth/content/private/s-private.md`, `tests/fixtures/wiki-synth/.awiki/maps/slug-to-path.tsv`.

- [ ] **Step 1: Write the three plain memex sources**

```bash
cat > tests/fixtures/wiki-synth/content/sources/s1.md <<'EOF'
---
title: "As We May Think"
date: 1945-07-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: ["As We May Think"]
sources: []
draft: false
---

A 1945 essay by Vannevar Bush proposing the memex, an associative-trail
device that would augment human memory by linking documents together.
EOF

cat > tests/fixtures/wiki-synth/content/sources/s2.md <<'EOF'
---
title: "Augmenting Human Intellect"
date: 1962-10-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

Engelbart's 1962 framework treating the augmentation of human intellect
as a system in which symbols, methodology, training and tools co-evolve.
EOF

cat > tests/fixtures/wiki-synth/content/sources/s3.md <<'EOF'
---
title: "Computer Lib / Dream Machines"
date: 1974-01-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

Ted Nelson's 1974 polemic introducing hypertext, transclusion, and the
Xanadu vision in which all documents are deeply linked.
EOF
```

- [ ] **Step 2: Write the variant fixtures (used by phase 14 lint tests)**

```bash
cat > tests/fixtures/wiki-synth/content/sources/s-smart-quotes.md <<'EOF'
---
title: "Smart Quotes Source"
date: 2026-01-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

Bush wrote that “the human mind operates by association,” and warned that
record-keeping had outpaced our ability to use it.
EOF

cat > tests/fixtures/wiki-synth/content/sources/s-nbsp.md <<'EOF'
---
title: "NBSP Source"
date: 2026-01-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

The associative trail is the heart of the memex, recorded once and reused
for ever after.
EOF

cat > tests/fixtures/wiki-synth/content/sources/s-multipara.md <<'EOF'
---
title: "Multi-paragraph Source"
date: 2026-01-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

The first paragraph of the source describes the trail as the fundamental
mechanism of selection by association rather than indexing.

The second paragraph extends the trail concept across multiple users, so
that knowledge accretes communally over generations.
EOF

cat > tests/fixtures/wiki-synth/content/sources/s-hyphens.md <<'EOF'
---
title: "Hyphen Variants Source"
date: 2026-01-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

Selection by association — as opposed to indexing — is the very mechanism
the memex aims to mechanise.
EOF

cat > tests/fixtures/wiki-synth/content/sources/s-zwsp.md <<'EOF'
---
title: "ZWSP Source"
date: 2026-01-01
last_updated: 2026-04-27
type: source
tags: [memex]
aliases: []
sources: []
draft: false
---

The memex​ is a device in which an individual stores all his books,
records, and communications.
EOF

cat > tests/fixtures/wiki-synth/content/private/s-private.md <<'EOF'
---
title: "Confidential Memo"
date: 2026-04-01
last_updated: 2026-04-27
type: source
tags: [memex, private]
aliases: []
sources: []
draft: false
---

Internal-only memo on memex prototypes; do not redistribute.
EOF
```

Note: `s-nbsp.md` should contain a real NBSP and `s-zwsp.md` should contain a real ZWSP. We embed them on commit using a small python helper rather than relying on heredoc fidelity:

- [ ] **Step 3: Inject the unicode characters**

```bash
python3 - <<'PY'
import pathlib
nbsp = pathlib.Path("tests/fixtures/wiki-synth/content/sources/s-nbsp.md")
nbsp.write_text(nbsp.read_text().replace("once and reused", "once\u00a0and reused"))
zwsp = pathlib.Path("tests/fixtures/wiki-synth/content/sources/s-zwsp.md")
zwsp.write_text(zwsp.read_text().replace("memex\u200b", "memex\u200b"))  # ensure ZWSP present
PY
```

(The `s-zwsp.md` file as written above already contains the ZWSP embedded directly between `memex` and the following character. The python step is a sanity reinjection in case heredoc transport stripped it.)

- [ ] **Step 4: Build the slug-to-path map for the fixture**

```bash
cat > tests/fixtures/wiki-synth/.awiki/maps/slug-to-path.tsv <<'EOF'
s1	content/sources/s1.md
s2	content/sources/s2.md
s3	content/sources/s3.md
s-smart-quotes	content/sources/s-smart-quotes.md
s-nbsp	content/sources/s-nbsp.md
s-multipara	content/sources/s-multipara.md
s-hyphens	content/sources/s-hyphens.md
s-zwsp	content/sources/s-zwsp.md
s-private	content/private/s-private.md
EOF
```

- [ ] **Step 5: Commit**

```bash
git add tests/fixtures/wiki-synth
git commit -m "test(synth): add wiki-synth fixture (memex sources + lint variants)"
```

---

## Task 13.4: `synth.sh list` subcommand (smallest end-to-end slice)

Start with the simplest subcommand to bootstrap the orchestrator file. Every later subcommand will dispatch through the same top-level `case "$1"`.

**Files:** Create: `scripts/synth.sh`, `tests/synth_test.bats`. Ship a placeholder `synthesis-plugins/briefing.md` so `list` has something to enumerate; the canonical briefing prompt lands in Task 13.10.

- [ ] **Step 1: Drop a minimal briefing manifest (we'll overwrite the body in Task 13.10)**

```bash
cat > synthesis-plugins/briefing.md <<'EOF'
---
name: briefing
description: One-page exec summary of a curated source set.
version: 1
output_type: synthesis
output_subtype: briefing
min_sources: 2
max_sources: 50
max_evidence_total_words: 500
required_sections:
  - "## TL;DR"
  - "## Key Findings"
  - "## Open Questions"
  - "## Evidence"
post_hook: null
---

# Prompt
(populated in task 13.10)
EOF
```

- [ ] **Step 2: Write the test**

```bash
cat > tests/synth_test.bats <<'EOF'
#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  cp -r "$REPO_ROOT/synthesis-plugins" "$WORK/synthesis-plugins"
  cp -r "$REPO_ROOT/tests/fixtures/wiki-synth/content" "$WORK/content"
  cp -r "$REPO_ROOT/tests/fixtures/wiki-synth/.awiki" "$WORK/.awiki"
  mkdir -p "$WORK/content/synthesis/.staged"
  printf -- "---\ntitle: Log\ntype: log\ndraft: true\n---\n" > "$WORK/content/log.md"
  # Stub log-append.sh / lint.sh for hermetic tests.
  cat > "$WORK/scripts/log-append.sh" <<'STUB'
#!/usr/bin/env bash
echo "LOG-APPEND|$*" >> "${AWIKI_LOG_FILE:-content/log.md}"
STUB
  cat > "$WORK/scripts/lint.sh" <<'STUB'
#!/usr/bin/env bash
# stub lint — phase 13 only checks markers; phase 14 will replace this.
exit 0
STUB
  chmod +x "$WORK/scripts/log-append.sh" "$WORK/scripts/lint.sh"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "synth list prints briefing row" {
  run bash scripts/synth.sh list
  [ "$status" -eq 0 ]
  [[ "$output" == *"briefing"* ]]
  [[ "$output" == *"synthesis"* ]]
}
EOF
```

- [ ] **Step 3: Run test (expect FAIL — script absent)**

```bash
bats tests/synth_test.bats
```

Expected: FAIL: `bash: scripts/synth.sh: No such file or directory`.

- [ ] **Step 4: Write the orchestrator skeleton + `list`**

```bash
cat > scripts/synth.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# scripts/synth.sh — orchestrator for awiki synthesis pages.
# Subcommands: new | regen | accept-stage | finalize | list | resolve | refine

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

# shellcheck disable=SC1091
source "$REPO_ROOT/scripts/synth-plugin-load.sh"

PLUGINS_DIR="${AWIKI_SYNTH_PLUGINS_DIR:-synthesis-plugins}"
SYNTH_DIR="${AWIKI_SYNTH_DIR:-content/synthesis}"
STAGED_DIR="$SYNTH_DIR/.staged"
SLUG_MAP="${AWIKI_SLUG_MAP:-.awiki/maps/slug-to-path.tsv}"

SLUG_REGEX='^[a-z0-9][a-z0-9-]*$'
PLUGIN_NAME_REGEX='^[a-z][a-z0-9-]*$'

die() { echo "ERROR: $*" >&2; exit "${EXIT_CODE:-1}"; }
log() { echo "$*" >&2; }

require_slug() {
  local s="$1" label="${2:-slug}"
  if ! [[ "$s" =~ $SLUG_REGEX ]]; then
    EXIT_CODE=1 die "invalid $label: '$s' (must match $SLUG_REGEX, no leading hyphen)"
  fi
}

cmd_list() {
  local count=0
  while IFS= read -r name; do
    [[ -z "$name" ]] && continue
    if synth_plugin_load "$name" 2>/dev/null; then
      printf -- "%s\t%s\t%s\t%s\n" \
        "$SYNTH_PLUGIN_NAME" \
        "$SYNTH_PLUGIN_OUTPUT_TYPE" \
        "$SYNTH_PLUGIN_OUTPUT_SUBTYPE" \
        "$SYNTH_PLUGIN_DESCRIPTION"
      count=$((count + 1))
    else
      log "skipping malformed plugin: $name"
    fi
  done < <(synth_plugin_list_all)
  if [[ "$count" -eq 0 ]]; then
    EXIT_CODE=1 die "no parseable plugins under $PLUGINS_DIR"
  fi
}

main() {
  if [[ $# -lt 1 ]]; then
    cat <<USAGE
usage: synth.sh <subcommand> [args...]
subcommands: new | regen | accept-stage | finalize | list | resolve | refine
USAGE
    exit 1
  fi
  local sub="$1"; shift
  case "$sub" in
    list) cmd_list "$@" ;;
    new|regen|accept-stage|finalize|resolve|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
    *) EXIT_CODE=1 die "unknown subcommand: $sub" ;;
  esac
}

main "$@"
EOF
chmod +x scripts/synth.sh
```

- [ ] **Step 5: Run test (expect PASS)**

```bash
bats tests/synth_test.bats
```

Expected: 1 test passes.

- [ ] **Step 6: Commit**

```bash
git add scripts/synth.sh synthesis-plugins/briefing.md tests/synth_test.bats
git commit -m "feat(synth): add synth.sh skeleton + list subcommand"
```

---

## Task 13.5: Scope resolution helpers (`tag`, `slugs`, `query`)

Resolution is invoked by `new`, `regen`, and `resolve`. Phase 13 supports tag-and-slugs natively; `query` shells out to `qmd search` if available, otherwise exits 2 with a clear "phase-15 grep fallback" message.

**Files:** Modify: `scripts/synth.sh`. Add tests to `tests/synth_test.bats`.

- [ ] **Step 1: Append tests**

```bash
cat >> tests/synth_test.bats <<'EOF'

@test "resolve --tag=memex returns 8 sorted slugs" {
  cat > content/synthesis/memex-briefing.md <<EOF2
---
title: "Memex"
type: synthesis
plugin: briefing
scope:
  tag: memex
sources: []
draft: false
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
<!-- END GENERATED -->
EOF2
  run bash scripts/synth.sh resolve memex-briefing
  [ "$status" -eq 0 ]
  [[ "$output" == *"s-hyphens"* ]]
  [[ "$output" == *"s1"* ]]
  # No private leak — s-private should NOT be included unless target is private.
  [[ "$output" != *"s-private"* ]]
}

@test "resolve with explicit --slugs subset" {
  cat > content/synthesis/manual-briefing.md <<EOF2
---
title: "Manual"
type: synthesis
plugin: briefing
scope:
  slugs: [s1, s2]
sources: []
draft: false
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
<!-- END GENERATED -->
EOF2
  run bash scripts/synth.sh resolve manual-briefing
  [ "$status" -eq 0 ]
  [[ "$output" == *"s1"* ]]
  [[ "$output" == *"s2"* ]]
  [[ "$output" != *"s3"* ]]
}

@test "resolve query exits 2 when qmd not installed" {
  cat > content/synthesis/q-briefing.md <<EOF2
---
title: "Q"
type: synthesis
plugin: briefing
scope:
  query: "memex history"
sources: []
draft: false
---

<!-- BEGIN GENERATED plugin=briefing scope_hash=000000 -->
<!-- END GENERATED -->
EOF2
  PATH="/nonexistent" run bash scripts/synth.sh resolve q-briefing
  [ "$status" -eq 2 ]
  [[ "$output" == *"qmd"* ]]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/synth_test.bats
```

Expected: the 3 new tests fail (`resolve` not implemented).

- [ ] **Step 3: Implement scope-resolution helpers and `cmd_resolve`**

Insert before `main()` in `scripts/synth.sh`:

```bash
# Read scope: block from a synthesis page's frontmatter.
# Sets SCOPE_TAG, SCOPE_SLUGS, SCOPE_QUERY, SCOPE_EXCLUDE_TAGS, SCOPE_MIN_LAST_UPDATED, SCOPE_TYPES.
synth_read_scope() {
  local page="$1"
  SCOPE_TAG=""; SCOPE_SLUGS=""; SCOPE_QUERY=""
  SCOPE_EXCLUDE_TAGS=""; SCOPE_MIN_LAST_UPDATED=""; SCOPE_TYPES=""
  local fm
  fm="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==1 {print}' "$page")"
  # Read scope: block (we expect block-form keys: scope: + indented children).
  # Flat parse: indented lines starting with two spaces under the line "scope:".
  local in_scope=0
  while IFS= read -r line; do
    if [[ "$line" =~ ^scope:[[:space:]]*$ ]]; then in_scope=1; continue; fi
    if [[ "$in_scope" -eq 1 ]]; then
      if [[ "$line" =~ ^[^[:space:]] ]]; then in_scope=0; continue; fi
      case "$line" in
        *"  tag:"*)    SCOPE_TAG="${line#*tag:}"; SCOPE_TAG="${SCOPE_TAG# }" ;;
        *"  slugs:"*)  SCOPE_SLUGS="${line#*slugs:}"; SCOPE_SLUGS="${SCOPE_SLUGS# }"; SCOPE_SLUGS="${SCOPE_SLUGS#[}"; SCOPE_SLUGS="${SCOPE_SLUGS%]}" ;;
        *"  query:"*)  SCOPE_QUERY="${line#*query:}"; SCOPE_QUERY="${SCOPE_QUERY# }"; SCOPE_QUERY="${SCOPE_QUERY#\"}"; SCOPE_QUERY="${SCOPE_QUERY%\"}" ;;
        *"  exclude_tags:"*) SCOPE_EXCLUDE_TAGS="${line#*exclude_tags:}"; SCOPE_EXCLUDE_TAGS="${SCOPE_EXCLUDE_TAGS# }"; SCOPE_EXCLUDE_TAGS="${SCOPE_EXCLUDE_TAGS#[}"; SCOPE_EXCLUDE_TAGS="${SCOPE_EXCLUDE_TAGS%]}" ;;
        *"  min_last_updated:"*) SCOPE_MIN_LAST_UPDATED="${line#*min_last_updated:}"; SCOPE_MIN_LAST_UPDATED="${SCOPE_MIN_LAST_UPDATED# }" ;;
        *"  types:"*) SCOPE_TYPES="${line#*types:}"; SCOPE_TYPES="${SCOPE_TYPES# }"; SCOPE_TYPES="${SCOPE_TYPES#[}"; SCOPE_TYPES="${SCOPE_TYPES%]}" ;;
      esac
    fi
  done <<< "$fm"
}

# Returns frontmatter scalar value for a single page.
# Usage: synth_fm_field <page> <field>
synth_fm_field() {
  awk -v field="$2" '
    BEGIN{c=0}
    /^---$/ {c++; next}
    c==1 && $1==(field":") { sub(/^[^:]+:[[:space:]]*/, ""); print; exit }
  ' "$1"
}

# Echo space-separated tag list (lowered, no quotes/brackets).
synth_fm_tags() {
  local raw
  raw="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==1 && /^tags:/ {sub(/^tags:[[:space:]]*/,""); print; exit}' "$1")"
  raw="${raw#[}"; raw="${raw%]}"
  printf -- '%s' "$raw" | tr ',' ' ' | tr -d '"' | tr -d "'" | tr -s ' '
}

# Resolve scope to a sorted, post-filter slug list, one per line.
# Inputs: $SCOPE_*, plus $SCOPE_TARGET_PRIVATE (1 if target is under content/private/).
synth_resolve_to_slugs() {
  local content_root="${AWIKI_CONTENT_DIR:-content}"
  local found
  found="$(mktemp)"
  trap 'rm -f "$found"' RETURN

  if [[ -n "$SCOPE_SLUGS" ]]; then
    printf -- '%s' "$SCOPE_SLUGS" | tr ',' '\n' | sed -E 's/^[[:space:]"'\''-]+|[[:space:]"'\'']+$//g' | grep -v '^$' >> "$found"
  elif [[ -n "$SCOPE_TAG" ]]; then
    while IFS= read -r -d '' page; do
      local tags; tags="$(synth_fm_tags "$page")"
      if [[ " $tags " == *" $SCOPE_TAG "* ]]; then
        basename "$page" .md >> "$found"
      fi
    done < <(find "$content_root" -name '*.md' -type f -print0)
  elif [[ -n "$SCOPE_QUERY" ]]; then
    if ! command -v qmd >/dev/null 2>&1; then
      EXIT_CODE=2 die "scope.query requires qmd; phase-15 grep fallback not yet shipped"
    fi
    qmd search -- "$SCOPE_QUERY" 2>/dev/null \
      | awk '{print $1}' \
      | grep -E "$SLUG_REGEX" >> "$found" || true
  else
    EXIT_CODE=2 die "scope must include exactly one of: tag, slugs, query"
  fi

  # Filter: exclude_tags
  if [[ -n "$SCOPE_EXCLUDE_TAGS" ]]; then
    local excl
    excl="$(printf -- '%s' "$SCOPE_EXCLUDE_TAGS" | tr ',' ' ' | tr -d '"' | tr -d "'" | tr -s ' ')"
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local tags; tags="$(synth_fm_tags "$path")"
      local skip=0
      for ex in $excl; do
        [[ -z "$ex" ]] && continue
        if [[ " $tags " == *" $ex "* ]]; then skip=1; break; fi
      done
      [[ "$skip" -eq 0 ]] && echo "$slug" >> "$kept"
    done < "$found"
    mv "$kept" "$found"
  fi

  # Filter: min_last_updated (lex compare on YYYY-MM-DD)
  if [[ -n "$SCOPE_MIN_LAST_UPDATED" ]]; then
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local lu; lu="$(synth_fm_field "$path" "last_updated")"
      if [[ "$lu" > "$SCOPE_MIN_LAST_UPDATED" || "$lu" == "$SCOPE_MIN_LAST_UPDATED" ]]; then
        echo "$slug" >> "$kept"
      fi
    done < "$found"
    mv "$kept" "$found"
  fi

  # Filter: types
  if [[ -n "$SCOPE_TYPES" ]]; then
    local types
    types="$(printf -- '%s' "$SCOPE_TYPES" | tr ',' ' ' | tr -d '"' | tr -d "'" | tr -s ' ')"
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local t; t="$(synth_fm_field "$path" "type")"
      for want in $types; do
        if [[ "$t" == "$want" ]]; then echo "$slug" >> "$kept"; break; fi
      done
    done < "$found"
    mv "$kept" "$found"
  fi

  # Privacy filter: drop slugs tagged `private` if target is not private.
  # The fail-closed enforcement happens in cmd_new; here we only emit the
  # post-filter slug list so resolve/regen can compute scope_hash deterministically.
  if [[ "${SCOPE_TARGET_PRIVATE:-0}" -ne 1 ]]; then
    local kept; kept="$(mktemp)"
    while IFS= read -r slug; do
      [[ -z "$slug" ]] && continue
      local path; path="$(synth_slug_to_path "$slug")"
      [[ -z "$path" ]] && continue
      local tags; tags="$(synth_fm_tags "$path")"
      [[ " $tags " != *" private "* ]] && echo "$slug" >> "$kept"
    done < "$found"
    mv "$kept" "$found"
  fi

  # Sort + dedupe ASCII-ascending (post-filter ordering is load-bearing for scope_hash).
  sort -u "$found"
}

# Return path for a slug via the slug-to-path map; fall back to find.
synth_slug_to_path() {
  local slug="$1"
  if [[ -f "$SLUG_MAP" ]]; then
    awk -F '\t' -v s="$slug" '$1==s {print $2; exit}' "$SLUG_MAP"
    return
  fi
  find "${AWIKI_CONTENT_DIR:-content}" -name "$slug.md" -type f -print -quit
}

# scope_hash = first 6 hex chars of sha256(sorted-slugs-newline-joined)
synth_scope_hash() {
  local slugs="$1"
  printf -- '%s\n' "$slugs" | tr -d '\r' | shasum -a 256 | awk '{print substr($1,1,6)}'
}

cmd_resolve() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh resolve <slug>"; fi
  local slug="$1"; require_slug "$slug" "synthesis-page-slug"
  local page="$SYNTH_DIR/$slug.md"
  [[ -f "$page" ]] || { EXIT_CODE=2 die "synthesis page not found: $page"; }

  synth_read_scope "$page"
  local target_path; target_path="$page"
  if [[ "$target_path" == *"/private/"* || "$target_path" == content/private/* ]]; then
    SCOPE_TARGET_PRIVATE=1
  else
    SCOPE_TARGET_PRIVATE=0
  fi
  synth_resolve_to_slugs
}
```

Then update the dispatch in `main()`:

```bash
# Replace the line: new|regen|accept-stage|finalize|resolve|refine)
# with:
    resolve) cmd_resolve "$@" ;;
    new|regen|accept-stage|finalize|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/synth_test.bats
```

Expected: 4 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/synth.sh tests/synth_test.bats
git commit -m "feat(synth): add scope resolution + resolve subcommand"
```

---

## Task 13.6: `synth.sh new` — privacy fail-closed + scaffold writer

This task is the largest. It composes the privacy check, marker-bearing scaffold, and prompt-bundle emission. Tests cover all four orchestrator-side exit codes (1 plugin, 2 scope, 2 privacy, 3 already-exists) plus the happy path.

**Files:** Modify: `scripts/synth.sh`, `tests/synth_test.bats`.

- [ ] **Step 1: Append tests**

```bash
cat >> tests/synth_test.bats <<'EOF'

@test "synth new happy path scaffolds page with markers and emits prompt" {
  run bash scripts/synth.sh new briefing memex --tag=memex
  [ "$status" -eq 0 ]
  [ -f content/synthesis/memex-briefing.md ]
  run grep '<!-- BEGIN GENERATED plugin=briefing scope_hash=' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  run grep '<!-- END GENERATED -->' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  run grep '^## Notes$' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  # No empty Feedback section on new (lazy creation).
  run grep '^## Feedback$' content/synthesis/memex-briefing.md
  [ "$status" -ne 0 ]
}

@test "synth new exits 1 on bogus plugin" {
  run bash scripts/synth.sh new bogus topic --tag=memex
  [ "$status" -eq 1 ]
}

@test "synth new exits 2 when scope resolves below min_sources" {
  run bash scripts/synth.sh new briefing too-narrow --tag=nonexistent
  [ "$status" -eq 2 ]
  [[ "$output" == *"min_sources"* || "$output" == *"too few"* ]]
}

@test "synth new fails closed when scope contains private and target is public" {
  run bash scripts/synth.sh new briefing leaky --slugs=s1,s-private,s2
  [ "$status" -eq 2 ]
  [[ "$output" == *"private source"* ]]
}

@test "synth new with --allow-private logs declassify and proceeds" {
  run bash scripts/synth.sh new briefing intentional --slugs=s1,s-private,s2 --allow-private
  [ "$status" -eq 0 ]
  run grep "synth-declassify" content/log.md
  [ "$status" -eq 0 ]
}

@test "synth new exits 3 if target page already exists" {
  bash scripts/synth.sh new briefing memex --tag=memex
  run bash scripts/synth.sh new briefing memex --tag=memex
  [ "$status" -eq 3 ]
}

@test "synth new rejects topic-slug with leading hyphen" {
  run bash scripts/synth.sh new briefing -bad-topic --tag=memex
  [ "$status" -eq 1 ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL — `new` not implemented)**

```bash
bats tests/synth_test.bats
```

Expected: 7 new tests fail.

- [ ] **Step 3: Implement `cmd_new` in `scripts/synth.sh`**

Insert before `main()`:

```bash
# Build the lead-paragraph + frontmatter snippet for the {{pages}} interpolation.
synth_pages_block() {
  local slugs="$1"
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    local path; path="$(synth_slug_to_path "$slug")"
    [[ -z "$path" ]] && continue
    local title; title="$(synth_fm_field "$path" "title")"
    local type;  type="$(synth_fm_field "$path" "type")"
    title="${title#\"}"; title="${title%\"}"
    # First non-blank non-frontmatter line as lead paragraph (truncated to ~200 chars).
    local lead
    lead="$(awk 'BEGIN{c=0} /^---$/ {c++; next} c==2 && NF>0 {print; exit}' "$path")"
    local lead_short="${lead:0:200}"
    printf -- '- [[%s]] (%s) — %s\n' "$slug" "$type" "$lead_short"
  done <<< "$slugs"
}

# Render the prompt bundle to stdout.
synth_emit_prompt() {
  local slugs="$1" scope_desc="$2" feedback="$3"
  local body; body="$(synth_plugin_extract_prompt)"
  # Interpolate {{scope_description}} (single-line replace).
  body="${body//\{\{scope_description\}\}/$scope_desc}"
  # Interpolate {{#pages}}...{{/pages}} block: replace whole tag-pair with
  # page rows. Only one such block in v1 plugin templates.
  local pages_rows
  pages_rows="$(synth_pages_block "$slugs")"
  body="$(printf -- '%s\n' "$body" | awk -v rows="$pages_rows" '
    BEGIN{ in_pages=0 }
    /\{\{#pages\}\}/ { in_pages=1; next }
    /\{\{\/pages\}\}/ { in_pages=0; print rows; next }
    in_pages==0 { print }
  ')"
  # Interpolate {{#feedback}}...{{/feedback}}.
  # Phase 13: feedback is empty on `new` and emits a stub note for plugin authors.
  if [[ -z "$feedback" ]]; then
    body="$(printf -- '%s\n' "$body" | awk '
      BEGIN{ in_fb=0 }
      /\{\{#feedback\}\}/ { in_fb=1; next }
      /\{\{\/feedback\}\}/ { in_fb=0; print "<!-- feedback channel arrives in phase 15 -->"; next }
      in_fb==0 { print }
    ')"
  else
    body="${body//\{\{feedback\}\}/$feedback}"
    body="$(printf -- '%s\n' "$body" | sed -e 's/{{#feedback}}//g' -e 's/{{\/feedback}}//g')"
  fi
  printf -- '%s\n' "$body"
}

# Write the synthesis-page scaffold (frontmatter + lead placeholder + ## Notes + markers).
synth_write_scaffold() {
  local page="$1" plugin="$2" scope_block="$3" scope_hash="$4" sources_yaml="$5" topic="$6"
  local today; today="$(date '+%Y-%m-%d')"
  mkdir -p "$(dirname "$page")"
  {
    echo '---'
    printf -- 'title: "%s — %s"\n' "$topic" "$plugin"
    printf -- 'date: %s\n' "$today"
    printf -- 'last_updated: %s\n' "$today"
    printf -- 'last_generated:\n'
    printf -- 'type: synthesis\n'
    printf -- 'plugin: %s\n' "$plugin"
    printf -- '%s' "$scope_block"
    printf -- 'tags: [%s, %s]\n' "$topic" "$plugin"
    printf -- 'aliases: []\n'
    printf -- '%s' "$sources_yaml"
    printf -- 'draft: false\n'
    echo '---'
    echo
    echo "Lead paragraph — written once by user/agent, NOT regenerated."
    echo
    echo "## Notes"
    echo
    echo "<!-- user notes; survives regen -->"
    echo
    printf -- '<!-- BEGIN GENERATED plugin=%s scope_hash=%s -->\n' "$plugin" "$scope_hash"
    echo
    echo '<!-- END GENERATED -->'
  } > "$page"
}

cmd_new() {
  local plugin="" topic="" scope_arg_kind="" scope_arg_val=""
  local exclude_tags="" min_last_updated="" types="" allow_private=0

  if [[ $# -lt 2 ]]; then
    EXIT_CODE=1 die "usage: synth.sh new <plugin> <topic-slug> (--tag=X|--slugs=a,b|--query=\"...\") [...]"
  fi
  plugin="$1"; topic="$2"; shift 2
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --tag=*)              scope_arg_kind="tag";   scope_arg_val="${1#--tag=}" ;;
      --slugs=*)            scope_arg_kind="slugs"; scope_arg_val="${1#--slugs=}" ;;
      --query=*)            scope_arg_kind="query"; scope_arg_val="${1#--query=}" ;;
      --exclude-tags=*)     exclude_tags="${1#--exclude-tags=}" ;;
      --min-last-updated=*) min_last_updated="${1#--min-last-updated=}" ;;
      --types=*)            types="${1#--types=}" ;;
      --allow-private)      allow_private=1 ;;
      --) shift; break ;;
      *) EXIT_CODE=1 die "unknown flag: $1" ;;
    esac
    shift
  done

  require_slug "$topic" "topic-slug"
  if ! synth_plugin_load "$plugin"; then EXIT_CODE=1 die "plugin load failed: $plugin"; fi
  [[ -n "$scope_arg_kind" ]] || { EXIT_CODE=1 die "must pass --tag, --slugs or --query"; }

  # Build scope state.
  SCOPE_TAG=""; SCOPE_SLUGS=""; SCOPE_QUERY=""
  SCOPE_EXCLUDE_TAGS="$exclude_tags"; SCOPE_MIN_LAST_UPDATED="$min_last_updated"; SCOPE_TYPES="$types"
  case "$scope_arg_kind" in
    tag)   SCOPE_TAG="$scope_arg_val" ;;
    slugs) SCOPE_SLUGS="$scope_arg_val" ;;
    query) SCOPE_QUERY="$scope_arg_val" ;;
  esac

  local target="$SYNTH_DIR/$topic-$plugin.md"
  if [[ "$target" == content/private/* ]]; then SCOPE_TARGET_PRIVATE=1; else SCOPE_TARGET_PRIVATE=0; fi

  # First pass: resolve WITHOUT private-stripping, so we can detect leakage.
  local saved_target_priv=$SCOPE_TARGET_PRIVATE
  SCOPE_TARGET_PRIVATE=1   # force "no privacy strip" so we see private slugs
  local raw_slugs; raw_slugs="$(synth_resolve_to_slugs)"
  SCOPE_TARGET_PRIVATE=$saved_target_priv

  local has_private=0
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    local path; path="$(synth_slug_to_path "$slug")"
    [[ -z "$path" ]] && continue
    local tags; tags="$(synth_fm_tags "$path")"
    if [[ " $tags " == *" private "* ]]; then has_private=1; fi
  done <<< "$raw_slugs"

  if [[ "$has_private" -eq 1 && "$SCOPE_TARGET_PRIVATE" -ne 1 && "$allow_private" -ne 1 ]]; then
    EXIT_CODE=2 die "private source in scope; either tag the synthesis page private and place under content/private/, or pass --allow-private"
  fi

  # Second pass: post-filter slug list (the canonical one used for scope_hash + sources:).
  local slugs; slugs="$(synth_resolve_to_slugs)"
  local nslugs; nslugs="$(printf -- '%s\n' "$slugs" | grep -c .)"

  if [[ -n "$SYNTH_PLUGIN_MIN_SOURCES" && "$nslugs" -lt "$SYNTH_PLUGIN_MIN_SOURCES" ]]; then
    EXIT_CODE=2 die "scope resolves to $nslugs sources; plugin requires min_sources=$SYNTH_PLUGIN_MIN_SOURCES"
  fi
  if [[ -n "$SYNTH_PLUGIN_MAX_SOURCES" && "$nslugs" -gt "$SYNTH_PLUGIN_MAX_SOURCES" ]]; then
    EXIT_CODE=2 die "scope resolves to $nslugs sources; plugin allows max_sources=$SYNTH_PLUGIN_MAX_SOURCES"
  fi

  if [[ -e "$target" ]]; then
    EXIT_CODE=3 die "target already exists: $target — use 'synth.sh regen' to refresh"
  fi

  local scope_hash; scope_hash="$(synth_scope_hash "$slugs")"

  # Build scope: YAML block.
  local scope_block; scope_block="scope:"$'\n'
  case "$scope_arg_kind" in
    tag)   scope_block+="  tag: $scope_arg_val"$'\n' ;;
    slugs) scope_block+="  slugs: [$scope_arg_val]"$'\n' ;;
    query) scope_block+="  query: \"$scope_arg_val\""$'\n' ;;
  esac
  [[ -n "$exclude_tags"     ]] && scope_block+="  exclude_tags: [$exclude_tags]"$'\n'
  [[ -n "$min_last_updated" ]] && scope_block+="  min_last_updated: $min_last_updated"$'\n'
  [[ -n "$types"            ]] && scope_block+="  types: [$types]"$'\n'

  # Build sources: list (empty on `new`; populated at finalize-time, per spec).
  local sources_yaml; sources_yaml="sources: []"$'\n'

  synth_write_scaffold "$target" "$plugin" "$scope_block" "$scope_hash" "$sources_yaml" "$topic"

  # Emit prompt bundle to stdout.
  local scope_desc
  case "$scope_arg_kind" in
    tag)   scope_desc="pages tagged '$scope_arg_val'" ;;
    slugs) scope_desc="explicit slug list ($nslugs pages)" ;;
    query) scope_desc="qmd query: $scope_arg_val" ;;
  esac
  synth_emit_prompt "$slugs" "$scope_desc" ""

  bash "$REPO_ROOT/scripts/log-append.sh" synth-scaffold -- "$plugin $topic"
  if [[ "$allow_private" -eq 1 && "$has_private" -eq 1 ]]; then
    bash "$REPO_ROOT/scripts/log-append.sh" synth-declassify -- "$topic sources=$nslugs"
  fi

  echo "SYNTH-NEW|target=$target|plugin=$plugin|sources=$nslugs|scope_hash=$scope_hash" >&2
}
```

Update the dispatch in `main()`:

```bash
# Replace the line: new|regen|accept-stage|finalize|refine)
# with:
    new) cmd_new "$@" ;;
    regen|accept-stage|finalize|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/synth_test.bats
```

Expected: 11 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/synth.sh tests/synth_test.bats
git commit -m "feat(synth): add 'new' subcommand with privacy fail-closed and scaffolding"
```

---

## Task 13.7: `synth.sh finalize` — marker check, scoped lint, stamping

`finalize` operates transparently on `.staged/<slug>.md` if present, else live.

**Files:** Modify: `scripts/synth.sh`, `tests/synth_test.bats`.

- [ ] **Step 1: Append tests**

```bash
cat >> tests/synth_test.bats <<'EOF'

@test "synth finalize stamps last_generated and populates sources" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  # Simulate agent filling the region.
  python3 - <<PY
import pathlib
p = pathlib.Path("content/synthesis/memex-briefing.md")
text = p.read_text()
filled = text.replace(
  "<!-- BEGIN GENERATED plugin=briefing scope_hash=",
  "<!-- BEGIN GENERATED plugin=briefing scope_hash=",
)
# Insert dummy body between markers.
filled = filled.replace(
  "<!-- END GENERATED -->",
  "## TL;DR\n- claim [[s1]]\n## Key Findings\n- finding [[s2]]\n## Open Questions\n- q [[s3]]\n## Evidence\n> \"associative\" — [[s1]]\n<!-- END GENERATED -->",
)
p.write_text(filled)
PY
  run bash scripts/synth.sh finalize memex-briefing
  [ "$status" -eq 0 ]
  run grep '^last_generated: 2' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  run grep -E '^sources: \[.*s1.*\]' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
}

@test "synth finalize exits 5 on missing END marker" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  python3 - <<PY
import pathlib
p = pathlib.Path("content/synthesis/memex-briefing.md")
p.write_text(p.read_text().replace("<!-- END GENERATED -->", ""))
PY
  run bash scripts/synth.sh finalize memex-briefing
  [ "$status" -eq 5 ]
}

@test "synth finalize prefers .staged file when present" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  cp content/synthesis/memex-briefing.md content/synthesis/.staged/memex-briefing.md
  # Mutate ONLY the staged file.
  python3 - <<PY
import pathlib
p = pathlib.Path("content/synthesis/.staged/memex-briefing.md")
p.write_text(p.read_text().replace(
  "<!-- END GENERATED -->",
  "## TL;DR\n- s [[s1]]\n## Key Findings\n- k [[s2]]\n## Open Questions\n- q [[s3]]\n## Evidence\n> \"q\" — [[s1]]\n<!-- END GENERATED -->",
))
PY
  run bash scripts/synth.sh finalize memex-briefing
  [ "$status" -eq 0 ]
  # Live page is unchanged (still has empty body).
  run grep '## TL;DR' content/synthesis/memex-briefing.md
  [ "$status" -ne 0 ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/synth_test.bats
```

Expected: 3 new tests fail.

- [ ] **Step 3: Implement `cmd_finalize`**

Insert before `main()`:

```bash
# Validate marker integrity. Returns 0 on OK, prints reason and returns 5 on failure.
synth_check_markers() {
  local page="$1"
  local begins; begins="$(grep -c '^<!-- BEGIN GENERATED ' "$page" || true)"
  local ends;   ends="$(grep -c '^<!-- END GENERATED -->' "$page" || true)"
  if [[ "$begins" -ne 1 || "$ends" -ne 1 ]]; then
    echo "ERROR: marker integrity failure: BEGIN=$begins END=$ends" >&2
    return 5
  fi
  local b_line e_line
  b_line="$(grep -n '^<!-- BEGIN GENERATED ' "$page" | head -1 | cut -d: -f1)"
  e_line="$(grep -n '^<!-- END GENERATED -->' "$page" | head -1 | cut -d: -f1)"
  if [[ "$b_line" -gt "$e_line" ]]; then
    echo "ERROR: BEGIN marker after END marker" >&2
    return 5
  fi
  return 0
}

# Rewrite frontmatter scalars / list fields in-place.
synth_fm_set_scalar() {
  local page="$1" key="$2" value="$3"
  awk -v k="$key" -v v="$value" '
    BEGIN{c=0; done=0}
    /^---$/ {c++; print; next}
    c==1 && index($0, k":")==1 && done==0 { print k": "v; done=1; next }
    {print}
    END {
      if (c==1 && done==0) {
        # not present — nothing inserted (caller should ensure key exists).
      }
    }
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

synth_fm_set_sources() {
  local page="$1" slugs="$2"
  local list="["
  local first=1
  while IFS= read -r slug; do
    [[ -z "$slug" ]] && continue
    if [[ $first -eq 1 ]]; then list+="\"[[$slug]]\""; first=0; else list+=", \"[[$slug]]\""; fi
  done <<< "$slugs"
  list+="]"
  awk -v v="$list" '
    BEGIN{c=0; done=0}
    /^---$/ {c++; print; next}
    c==1 && /^sources:/ && done==0 { print "sources: " v; done=1; next }
    {print}
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

synth_fm_set_scope_hash_in_marker() {
  local page="$1" hash="$2"
  awk -v h="$hash" '
    /^<!-- BEGIN GENERATED plugin=/ {
      sub(/scope_hash=[0-9a-f]+/, "scope_hash=" h)
    }
    {print}
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

cmd_finalize() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh finalize <slug>"; fi
  local slug="$1"; require_slug "$slug" "synthesis-page-slug"

  local live="$SYNTH_DIR/$slug.md"
  local staged="$STAGED_DIR/$slug.md"
  local target=""
  if [[ -f "$staged" ]]; then target="$staged"; else target="$live"; fi
  [[ -f "$target" ]] || { EXIT_CODE=1 die "synthesis page not found: $target"; }

  if ! synth_check_markers "$target"; then EXIT_CODE=5 die "marker integrity failed"; fi

  # Scoped lint (phase 13: only marker + required-sections + slug existence;
  # full S1-S9 lands in phase 14).
  if ! bash "$REPO_ROOT/scripts/lint.sh" --only=synth --file="$target" 2>/dev/null; then
    EXIT_CODE=6 die "scoped lint failed for $target"
  fi

  # Re-resolve scope, recompute hash, populate sources:.
  synth_read_scope "$target"
  if [[ "$target" == */private/* ]]; then SCOPE_TARGET_PRIVATE=1; else SCOPE_TARGET_PRIVATE=0; fi
  local slugs; slugs="$(synth_resolve_to_slugs)"
  local hash;  hash="$(synth_scope_hash "$slugs")"
  synth_fm_set_scope_hash_in_marker "$target" "$hash"
  synth_fm_set_sources "$target" "$slugs"
  local now_utc; now_utc="$(date -u '+%Y-%m-%dT%H:%M:%SZ')"
  synth_fm_set_scalar "$target" "last_generated" "$now_utc"

  # If we operated on a staged file, leave it staged; user runs `accept-stage`.
  bash "$REPO_ROOT/scripts/log-append.sh" synth -- "$(synth_fm_field "$target" plugin) $slug"

  # Bump ingest counter (synthesis counts toward auto-lint cadence).
  mkdir -p .awiki
  local cnt=0; [[ -f .awiki/ingest-count ]] && cnt="$(cat .awiki/ingest-count)"
  echo "$((cnt + 1))" > .awiki/ingest-count

  echo "SYNTH-FINALIZE|target=$target|sources=$(printf -- '%s' "$slugs" | grep -c .)|scope_hash=$hash" >&2
}
```

Update dispatch:

```bash
# Replace: regen|accept-stage|finalize|refine)
# with:
    finalize) cmd_finalize "$@" ;;
    regen|accept-stage|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/synth_test.bats
```

Expected: 14 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/synth.sh tests/synth_test.bats
git commit -m "feat(synth): add 'finalize' subcommand with marker check + stamping"
```

---

## Task 13.8: `synth.sh regen` with hand-edit detection

`regen` re-runs the privacy check, detects manual edits inside markers via `git show HEAD:<path>`, and either rewrites in place (default) or writes to `.staged/<slug>.md` (`--stage`).

**Files:** Modify: `scripts/synth.sh`, `tests/synth_test.bats`.

- [ ] **Step 1: Append tests**

```bash
cat >> tests/synth_test.bats <<'EOF'

@test "synth regen on clean tree rescaffolds region" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  # Commit so HEAD has the page.
  git -C "$WORK" init -q . 2>/dev/null || true
  git -C "$WORK" add -A 2>/dev/null || true
  git -C "$WORK" -c user.email=t@t -c user.name=t commit -q -m init 2>/dev/null || true
  run bash scripts/synth.sh regen memex-briefing --force
  [ "$status" -eq 0 ]
  run grep '<!-- BEGIN GENERATED plugin=briefing scope_hash=' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
}

@test "synth regen exits 4 on hand-edit inside markers" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  git -C "$WORK" init -q . 2>/dev/null || true
  git -C "$WORK" add -A 2>/dev/null || true
  git -C "$WORK" -c user.email=t@t -c user.name=t commit -q -m init 2>/dev/null || true
  python3 - <<PY
import pathlib
p = pathlib.Path("content/synthesis/memex-briefing.md")
p.write_text(p.read_text().replace(
  "<!-- END GENERATED -->",
  "manually inserted line inside marker region\n<!-- END GENERATED -->",
))
PY
  run bash scripts/synth.sh regen memex-briefing
  [ "$status" -eq 4 ]
}

@test "synth regen --stage writes only to .staged file" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  git -C "$WORK" init -q . 2>/dev/null || true
  git -C "$WORK" add -A 2>/dev/null || true
  git -C "$WORK" -c user.email=t@t -c user.name=t commit -q -m init 2>/dev/null || true
  run bash scripts/synth.sh regen memex-briefing --stage
  [ "$status" -eq 0 ]
  [ -f content/synthesis/.staged/memex-briefing.md ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/synth_test.bats
```

Expected: 3 new tests fail.

- [ ] **Step 3: Implement `cmd_regen`**

Insert before `main()`:

```bash
# Detect hand-edits inside the marker region by diffing against HEAD.
# Returns 0 if no marker-region diff, 1 if intersecting diff found, 2 if untracked/no HEAD.
synth_handedit_check() {
  local page="$1"
  if ! git ls-files --error-unmatch -- "$page" >/dev/null 2>&1; then
    return 2  # untracked: treat as fresh
  fi
  if ! git rev-parse --verify HEAD >/dev/null 2>&1; then
    return 2  # no HEAD yet
  fi
  local head_tmp work_tmp; head_tmp="$(mktemp)"; work_tmp="$(mktemp)"
  git show "HEAD:$page" > "$head_tmp" 2>/dev/null || { rm -f "$head_tmp" "$work_tmp"; return 2; }
  cp "$page" "$work_tmp"

  # Extract the BEGIN..END block from each side.
  local extract='
    /^<!-- BEGIN GENERATED / { p=1 }
    p { print }
    /^<!-- END GENERATED -->/ { p=0 }
  '
  local h w
  h="$(awk "$extract" "$head_tmp")"
  w="$(awk "$extract" "$work_tmp")"
  rm -f "$head_tmp" "$work_tmp"
  if [[ "$h" != "$w" ]]; then return 1; fi
  return 0
}

# Rewrite the marker region of $page with empty body and updated scope_hash.
synth_clear_region() {
  local page="$1" plugin="$2" hash="$3"
  awk -v plugin="$plugin" -v hash="$hash" '
    BEGIN{ in_region=0 }
    /^<!-- BEGIN GENERATED / {
      print "<!-- BEGIN GENERATED plugin=" plugin " scope_hash=" hash " -->"
      print ""
      in_region=1; next
    }
    /^<!-- END GENERATED -->/ {
      print
      in_region=0; next
    }
    in_region==0 { print }
  ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
}

cmd_regen() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh regen <slug> [--force] [--stage]"; fi
  local slug="$1"; shift; require_slug "$slug" "synthesis-page-slug"

  local force=0 stage=0
  while [[ $# -gt 0 ]]; do
    case "$1" in
      --force) force=1 ;;
      --stage) stage=1 ;;
      --) shift; break ;;
      *) EXIT_CODE=1 die "unknown flag: $1" ;;
    esac
    shift
  done

  local live="$SYNTH_DIR/$slug.md"
  [[ -f "$live" ]] || { EXIT_CODE=1 die "synthesis page not found: $live"; }

  local plugin; plugin="$(synth_fm_field "$live" "plugin")"
  [[ -n "$plugin" ]] || { EXIT_CODE=1 die "$live missing 'plugin:' frontmatter"; }
  if ! synth_plugin_load "$plugin"; then EXIT_CODE=1 die "plugin load failed: $plugin"; fi

  if [[ "$force" -eq 0 && "$stage" -eq 0 ]]; then
    synth_handedit_check "$live"
    case $? in
      1) EXIT_CODE=4 die "hand-edit detected inside marker region; pass --force or --stage" ;;
      2) : ;;  # untracked or no HEAD; proceed
    esac
  fi

  synth_read_scope "$live"
  if [[ "$live" == */private/* ]]; then SCOPE_TARGET_PRIVATE=1; else SCOPE_TARGET_PRIVATE=0; fi

  # Re-run privacy fail-closed check (catches newly-private sources).
  local saved=$SCOPE_TARGET_PRIVATE
  SCOPE_TARGET_PRIVATE=1
  local raw_slugs; raw_slugs="$(synth_resolve_to_slugs)"
  SCOPE_TARGET_PRIVATE=$saved
  while IFS= read -r s; do
    [[ -z "$s" ]] && continue
    local sp; sp="$(synth_slug_to_path "$s")"
    [[ -z "$sp" ]] && continue
    local stags; stags="$(synth_fm_tags "$sp")"
    if [[ " $stags " == *" private "* && "$SCOPE_TARGET_PRIVATE" -ne 1 ]]; then
      EXIT_CODE=2 die "private source $s now in scope; tag the synthesis page private or pass --allow-private (regen)"
    fi
  done <<< "$raw_slugs"

  local slugs; slugs="$(synth_resolve_to_slugs)"
  local hash;  hash="$(synth_scope_hash "$slugs")"

  local target="$live"
  if [[ "$stage" -eq 1 ]]; then
    mkdir -p "$STAGED_DIR"
    target="$STAGED_DIR/$slug.md"
    cp "$live" "$target"
  fi

  synth_clear_region "$target" "$plugin" "$hash"

  # Emit prompt bundle (no feedback in phase 13).
  local scope_desc=""
  [[ -n "$SCOPE_TAG"   ]] && scope_desc="pages tagged '$SCOPE_TAG'"
  [[ -n "$SCOPE_SLUGS" ]] && scope_desc="explicit slug list"
  [[ -n "$SCOPE_QUERY" ]] && scope_desc="qmd query: $SCOPE_QUERY"
  synth_emit_prompt "$slugs" "$scope_desc" ""

  echo "SYNTH-REGEN|target=$target|stage=$stage|scope_hash=$hash" >&2
}
```

Update dispatch:

```bash
# Replace: regen|accept-stage|refine)
# with:
    regen) cmd_regen "$@" ;;
    accept-stage|refine)
      EXIT_CODE=1 die "subcommand '$sub' not yet implemented"
      ;;
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/synth_test.bats
```

Expected: 17 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/synth.sh tests/synth_test.bats
git commit -m "feat(synth): add 'regen' subcommand with hand-edit detection + privacy re-check"
```

---

## Task 13.9: `accept-stage` and `refine` subcommands

`accept-stage` promotes a finalized staged file into place. `refine` appends a feedback bullet idempotently and bumps `last_updated`.

**Files:** Modify: `scripts/synth.sh`, `tests/synth_test.bats`.

- [ ] **Step 1: Append tests**

```bash
cat >> tests/synth_test.bats <<'EOF'

@test "synth accept-stage promotes staged file" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  git -C "$WORK" init -q . 2>/dev/null || true
  git -C "$WORK" add -A 2>/dev/null || true
  git -C "$WORK" -c user.email=t@t -c user.name=t commit -q -m init 2>/dev/null || true
  bash scripts/synth.sh regen memex-briefing --stage >/dev/null
  # Fill staged file
  python3 - <<PY
import pathlib
p = pathlib.Path("content/synthesis/.staged/memex-briefing.md")
p.write_text(p.read_text().replace(
  "<!-- END GENERATED -->",
  "## TL;DR\n- s [[s1]]\n## Key Findings\n- k [[s2]]\n## Open Questions\n- q [[s3]]\n## Evidence\n> \"q\" — [[s1]]\n<!-- END GENERATED -->",
))
PY
  bash scripts/synth.sh finalize memex-briefing >/dev/null
  run bash scripts/synth.sh accept-stage memex-briefing
  [ "$status" -eq 0 ]
  [ ! -f content/synthesis/.staged/memex-briefing.md ]
  run grep '## TL;DR' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
}

@test "synth accept-stage exits 7 if no staged file" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  run bash scripts/synth.sh accept-stage memex-briefing
  [ "$status" -eq 7 ]
}

@test "synth refine appends feedback bullet and creates section" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  run bash scripts/synth.sh refine memex-briefing "tighten the TL;DR"
  [ "$status" -eq 0 ]
  run grep -F '## Feedback' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  run grep -F '- tighten the TL;DR' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
}

@test "synth refine is idempotent on exact-duplicate bullet" {
  bash scripts/synth.sh new briefing memex --tag=memex >/dev/null
  bash scripts/synth.sh refine memex-briefing "be concise"
  bash scripts/synth.sh refine memex-briefing "be concise"
  run grep -c -F '- be concise' content/synthesis/memex-briefing.md
  [ "$output" = "1" ]
}
EOF
```

- [ ] **Step 2: Run tests (expect FAIL)**

```bash
bats tests/synth_test.bats
```

Expected: 4 new tests fail.

- [ ] **Step 3: Implement `cmd_accept_stage` and `cmd_refine`**

Insert before `main()`:

```bash
cmd_accept_stage() {
  if [[ $# -lt 1 ]]; then EXIT_CODE=1 die "usage: synth.sh accept-stage <slug>"; fi
  local slug="$1"; require_slug "$slug" "synthesis-page-slug"
  local staged="$STAGED_DIR/$slug.md"
  local live="$SYNTH_DIR/$slug.md"
  [[ -f "$staged" ]] || { EXIT_CODE=7 die "no staged file at $staged"; }

  if ! bash "$REPO_ROOT/scripts/lint.sh" --only=synth --file="$staged" 2>/dev/null; then
    EXIT_CODE=6 die "scoped lint failed for staged file"
  fi
  mv "$staged" "$live"
  bash "$REPO_ROOT/scripts/log-append.sh" synth -- "$(synth_fm_field "$live" plugin) $slug accept-stage"
  echo "SYNTH-ACCEPT-STAGE|target=$live" >&2
}

cmd_refine() {
  if [[ $# -lt 2 ]]; then EXIT_CODE=1 die "usage: synth.sh refine <slug> \"<note>\""; fi
  local slug="$1"; shift; require_slug "$slug" "synthesis-page-slug"
  local note="$*"
  local page="$SYNTH_DIR/$slug.md"
  [[ -f "$page" ]] || { EXIT_CODE=1 die "synthesis page not found: $page"; }

  # Idempotence guard.
  if grep -F -q -- "- $note" "$page"; then
    echo "SYNTH-REFINE|skipped (duplicate)|$slug" >&2
    return 0
  fi

  if grep -q '^## Feedback$' "$page"; then
    # Append bullet to existing section.
    awk -v note="$note" '
      BEGIN{ done=0 }
      { print }
      /^## Feedback$/ && done==0 {
        # collect bullets until the next blank line or marker; we just append at end of section.
      }
      END {}
    ' "$page" > /dev/null  # placeholder; we append below via simpler logic
    # Robust append: insert bullet immediately before the BEGIN marker, after the
    # last existing Feedback bullet. Use a two-pass awk.
    awk -v note="$note" '
      BEGIN{ in_fb=0; emitted=0 }
      /^## Feedback$/ { in_fb=1; print; next }
      in_fb==1 && /^<!-- BEGIN GENERATED / && emitted==0 {
        print "- " note
        print ""
        emitted=1
        in_fb=0
      }
      in_fb==1 && /^## / && !/^## Feedback$/ && emitted==0 {
        # next H2 starts; insert before it.
        print "- " note
        print ""
        emitted=1
        in_fb=0
      }
      { print }
    ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
  else
    # Insert "## Feedback\n- <note>\n" before the BEGIN marker.
    awk -v note="$note" '
      /^<!-- BEGIN GENERATED / && inserted==0 {
        print "## Feedback"
        print ""
        print "- " note
        print ""
        inserted=1
      }
      { print }
    ' "$page" > "$page.tmp" && mv "$page.tmp" "$page"
  fi

  local today; today="$(date '+%Y-%m-%d')"
  synth_fm_set_scalar "$page" "last_updated" "$today"
  echo "SYNTH-REFINE|appended|$slug|$note" >&2
}
```

Update dispatch — replace the unimplemented case entirely:

```bash
case "$sub" in
  list)         cmd_list "$@" ;;
  new)          cmd_new "$@" ;;
  regen)        cmd_regen "$@" ;;
  accept-stage) cmd_accept_stage "$@" ;;
  finalize)     cmd_finalize "$@" ;;
  resolve)      cmd_resolve "$@" ;;
  refine)       cmd_refine "$@" ;;
  *) EXIT_CODE=1 die "unknown subcommand: $sub" ;;
esac
```

- [ ] **Step 4: Run tests (expect PASS)**

```bash
bats tests/synth_test.bats
```

Expected: 21 tests pass.

- [ ] **Step 5: Commit**

```bash
git add scripts/synth.sh tests/synth_test.bats
git commit -m "feat(synth): add 'accept-stage' + 'refine' (idempotent feedback append)"
```

---

## Task 13.10: Ship the canonical `briefing` plugin prompt

Replace the placeholder body created in Task 13.4 with the spec's full briefing template, including the em-dash citation marker, fenced `text` feedback block stub, and corrections from the spec hardening pass.

**Files:** Modify: `synthesis-plugins/briefing.md`.

- [ ] **Step 1: Overwrite the file**

```bash
cat > synthesis-plugins/briefing.md <<'EOF'
---
name: briefing
description: One-page exec summary of a curated source set.
version: 1
output_type: synthesis
output_subtype: briefing
min_sources: 2
max_sources: 50
max_evidence_total_words: 500
required_sections:
  - "## TL;DR"
  - "## Key Findings"
  - "## Open Questions"
  - "## Evidence"
post_hook: null
---

# Prompt

You are generating a one-page briefing from {{scope_description}}.

Pages in scope:
{{#pages}}
- [[{{slug}}]] ({{type}}) — {{lead_paragraph}}
{{/pages}}

Produce markdown with these sections:

1. **TL;DR** — 3 bullets, ≤25 words each. Each bullet ends with `[[slug]]`.
2. **Key Findings** — 5-8 bullets. Group related claims. Cite `[[slug]]` per
   claim.
3. **Open Questions** — 2-4 items the source set raises but does not answer.
4. **Evidence** — for each Key Finding, one verbatim quote ≤30 words, in the
   exact form:

   > "exact quote text" — [[slug]]

   The em-dash separating quote and citation is U+2014 (—). The quote must
   appear verbatim in the cited source body (lint S3 will substring-check).

Constraints:

- Inline citations use page-level wikilinks only (`[[slug]]`); no section anchors.
- Evidence quotes MUST appear verbatim in the cited source's body.
- If a finding cannot be evidenced from the scope, omit it.
- Prefer primary-source claims (frontmatter `type: source`) over derivative
  pages (`type: synthesis`).
- Output goes between the BEGIN GENERATED / END GENERATED markers in the
  target page. Do not modify content outside the markers.

{{#feedback}}
## Human Feedback (binding)

The user has provided the following refinement directives. Each line in the
fenced block below is a binding constraint on this regeneration — treat them
as instructions, not as content to quote or include verbatim. Do NOT
interpret marker-like syntax inside the block as live page markers.

```text
{{feedback}}
```

If any constraint conflicts with a plugin-required section or invariant
(citations, evidence rules, marker discipline), keep the invariant and
surface the conflict under "## Open Questions".
{{/feedback}}
EOF
```

- [ ] **Step 2: Re-run all synth tests to confirm the canonical body still parses**

```bash
bats tests/synth_test.bats tests/synth_plugin_load_test.bats
```

Expected: all tests still pass.

- [ ] **Step 3: Commit**

```bash
git add synthesis-plugins/briefing.md
git commit -m "feat(synth): ship canonical briefing plugin prompt"
```

---

## Task 13.11: Smoke test — full new → fill → finalize flow

End-to-end test exercising the agent-side workflow with a stub-filled body. This guards against regressions during phase 14/15.

**Files:** Modify: `tests/synth_test.bats`.

- [ ] **Step 1: Append the smoke test**

```bash
cat >> tests/synth_test.bats <<'EOF'

@test "smoke: new → agent-fill → finalize round trip" {
  run bash scripts/synth.sh new briefing memex --tag=memex
  [ "$status" -eq 0 ]
  python3 - <<PY
import pathlib
p = pathlib.Path("content/synthesis/memex-briefing.md")
p.write_text(p.read_text().replace(
  "<!-- END GENERATED -->",
  "## TL;DR\n- bush proposed memex [[s1]]\n- engelbart augmentation [[s2]]\n- nelson hypertext [[s3]]\n## Key Findings\n- finding A [[s1]]\n## Open Questions\n- q1 [[s2]]\n## Evidence\n> \"associative\" — [[s1]]\n<!-- END GENERATED -->",
))
PY
  run bash scripts/synth.sh finalize memex-briefing
  [ "$status" -eq 0 ]
  run grep '^last_generated: 2' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  run grep -F 'sources: ["[[s' content/synthesis/memex-briefing.md
  [ "$status" -eq 0 ]
  run bash scripts/synth.sh resolve memex-briefing
  [ "$status" -eq 0 ]
  [[ "$output" == *"s1"* ]]
}
EOF
bats tests/synth_test.bats
```

Expected: 22 tests pass.

- [ ] **Step 2: Commit**

```bash
git add tests/synth_test.bats
git commit -m "test(synth): add full new→fill→finalize smoke test"
```

---

## Task 13.12: justfile recipes

**Files:** Modify: `justfile`.

- [ ] **Step 1: Append synthesis recipes (with `--` flag terminators)**

Add a new section to `justfile` after the `# === maintenance ===` block:

```just
# === synthesis ===
synth plugin topic *args:
    bash scripts/synth.sh new -- {{plugin}} {{topic}} {{args}}

synth-regen slug *args:
    bash scripts/synth.sh regen -- {{slug}} {{args}}

synth-finalize slug:
    bash scripts/synth.sh finalize -- {{slug}}

synth-accept-stage slug:
    bash scripts/synth.sh accept-stage -- {{slug}}

synth-refine slug *note:
    bash scripts/synth.sh refine -- {{slug}} "{{note}}"

synth-list:
    bash scripts/synth.sh list

synth-resolve slug:
    bash scripts/synth.sh resolve -- {{slug}}
```

- [ ] **Step 2: Verify recipes parse**

```bash
just --list | grep synth
```

Expected output (order may vary):

```
    synth plugin topic *args
    synth-accept-stage slug
    synth-finalize slug
    synth-list
    synth-refine slug *note
    synth-regen slug *args
    synth-resolve slug
```

- [ ] **Step 3: Append documentation to `docs/just-help.txt`**

Append:

```bash
cat >> docs/just-help.txt <<'EOF'

## synthesis

just synth <plugin> <topic-slug> --tag=<tag>
just synth <plugin> <topic-slug> --slugs=<slug1>,<slug2>
just synth-regen <synthesis-slug> [--force|--stage]
just synth-finalize <synthesis-slug>
just synth-accept-stage <synthesis-slug>
just synth-refine <synthesis-slug> "tighten the TL;DR"
just synth-list
just synth-resolve <synthesis-slug>

Quoting: refinement notes must be quoted because they're variadic.
Topic slugs must match ^[a-z0-9][a-z0-9-]*$ (no leading hyphen).
All recipes use -- to terminate flag parsing before positional args.

Example end-to-end:

    just synth briefing memex --tag=memex
    # agent reads the prompt bundle and fills between markers
    just synth-finalize memex-briefing
    # later:
    just synth-refine memex-briefing "shorten Key Findings to 5 bullets"
    just synth-regen memex-briefing
EOF
```

- [ ] **Step 4: Commit**

```bash
git add justfile docs/just-help.txt
git commit -m "feat(synth): add justfile recipes + just-help docs for synthesis"
```

---

## Task 13.13: WIKI.md Section 4 sub-workflow "Synthesis (briefing-only)"

Phase 13 adds the briefing-only flow to `WIKI.md` Section 4 (Workflows). Phase 14 expands with lint discipline + remaining plugins; phase 15 adds Feedback + MCP subsections.

**Files:** Modify: `WIKI.md`.

- [ ] **Step 1: Add the Synthesis sub-workflow under Section 4**

Insert after the `**Lint:**` block (and before Section 5, "Inbox Queues") in `WIKI.md`:

```markdown
**Synthesis (briefing-only):**
1. Pick a scope. Either a tag (`--tag=memex`), an explicit slug list
   (`--slugs=s1,s2,s3`), or a saved query (`--query="…"` — requires qmd).
2. Run `just synth briefing <topic-slug> --tag=<tag>`.
   - Orchestrator validates the plugin, resolves scope, fails closed if
     any source is tagged `private` and the target page is not under
     `content/private/` (pass `--allow-private` to acknowledge intentional
     declassification).
   - Scaffolds `content/synthesis/<topic>-briefing.md` with frontmatter,
     a lead-paragraph placeholder, `## Notes`, and the BEGIN/END markers.
   - Emits the prompt bundle to stdout. The bundle is the briefing plugin
     template with the resolved page list interpolated.
3. Read the prompt. Generate the briefing body (TL;DR / Key Findings /
   Open Questions / Evidence) and write it between the BEGIN and END
   markers. Do not edit anything outside the markers.
4. Run `just synth-finalize <topic>-briefing`.
   - Validates marker integrity (exit 5 if broken).
   - Runs scoped lint (`scripts/lint.sh --only=synth --file=<path>`;
     phase 13 covers markers and required sections only — full S1–S9
     arrives in phase 14).
   - Stamps `last_generated`, populates frontmatter `sources:` from the
     resolved scope, log-appends.
5. To refresh later: `just synth-regen <topic>-briefing`. The orchestrator
   re-runs the privacy check, refuses if it detects a manual edit inside
   the marker region (pass `--force` to override or `--stage` to write to
   `.staged/<slug>.md` for review). After review, promote with
   `just synth-accept-stage <topic>-briefing`.

Topic-slug constraint: `^[a-z0-9][a-z0-9-]*$` (no leading hyphen). Plugin
name constraint: `^[a-z][a-z0-9-]*$`. All justfile recipes use `--` to
terminate flag parsing before positional args.

Mindmap, timeline, and study-guide plugins arrive in phase 14, with the
full lint suite (S1–S9) and feedback channel.
```

- [ ] **Step 2: Add a one-line note to Section 6 (Output Formats)**

Find the line in Section 6 that lists `synthesis|deck|chart|canvas` and append:

```markdown
- Synthesis pages may carry `plugin: <name>` frontmatter pointing to a
  manifest under `synthesis-plugins/`. Phase 13 ships `briefing`; phase 14
  adds `mindmap`, `timeline`, `study-guide`. Required sections per plugin
  are enforced by lint S2.
```

- [ ] **Step 3: Add S1 + S2 to Section 7 lint checklist (placeholder rows for phase 14)**

Append to Section 7's mechanical-checks list:

```markdown
- **S1 (synth, error)** — synthesis pages with `plugin:` frontmatter must
  contain exactly one BEGIN GENERATED marker and one END GENERATED marker,
  in that order.
- **S2 (synth, error)** — every section listed in the plugin manifest's
  `required_sections` must appear inside the marker region.

(S3–S9 ship in phase 14.)
```

- [ ] **Step 4: Commit**

```bash
git add WIKI.md
git commit -m "docs(synth): add WIKI.md Section 4 'Synthesis (briefing-only)' workflow"
```

---

## Task 13.14: Phase-done checklist + branch merge

- [ ] **Step 1: Run the full test suite + lint**

```bash
bats tests/
```

Expected: every test in the suite passes (synth tests + every prior phase's tests).

```bash
just lint
```

Expected: clean, or warnings only — no errors. (Lint at phase 13 is still mechanical-only from phase 2; synth-aware S1–S9 lands in phase 14.)

- [ ] **Step 2: Verify phase-done checklist**

Manually walk through the items below and confirm each:

- [ ] `scripts/synth.sh` ships with all 7 subcommands wired.
- [ ] `scripts/synth-plugin-load.sh` parses single-file manifests; rejects
  directory form with the "phase 14" message.
- [ ] `synthesis-plugins/briefing.md` is the canonical spec body (em-dash
  citation marker present; `text`-fenced feedback block stub present).
- [ ] `tests/fixtures/wiki-synth/` contains 3 plain memex sources plus
  variants (smart-quotes, NBSP, multi-paragraph, hyphens, ZWSP) plus a
  `[private]`-tagged source.
- [ ] `tests/synth_test.bats` covers: `new` happy path, exit 1/2/3/4/5/7
  paths, `resolve`, `finalize` happy + exit 5, `regen` clean + handedit +
  stage, `accept-stage`, `refine` (append + idempotent), and the full
  smoke test.
- [ ] `tests/synth_plugin_load_test.bats` covers: parse, missing-fields,
  bad name, directory-form rejection, default subtype, unknown plugin.
- [ ] `justfile` exposes `synth`, `synth-regen`, `synth-finalize`,
  `synth-accept-stage`, `synth-refine`, `synth-list`, `synth-resolve` —
  every one with `--` flag terminator.
- [ ] `docs/just-help.txt` documents each recipe with quoting examples.
- [ ] `WIKI.md` Section 4 has the "Synthesis (briefing-only)" sub-workflow.
- [ ] `WIKI.md` Section 6 mentions the `plugin:` frontmatter.
- [ ] `WIKI.md` Section 7 lists S1 and S2 (S3–S9 are placeholders for phase 14).
- [ ] No `python3 -c '...'` invocations anywhere in `scripts/`.
- [ ] All slug regex usage is `^[a-z0-9][a-z0-9-]*$` (topic / synthesis page slug)
  and `^[a-z][a-z0-9-]*$` (plugin name).
- [ ] Marker syntax is `<!-- BEGIN GENERATED ... -->` / `<!-- END GENERATED -->` everywhere
  (never bare `BEGIN`/`END`).
- [ ] `query`-scoped pages do NOT have `S5` enforcement wired (will be
  noted in phase 14 lint as exempt; orchestrator already records `sources:`
  at gen-time).
- [ ] `cmd_new` runs the privacy fail-closed check BEFORE writing any file
  to disk; on `--allow-private` it logs `synth-declassify` after writing.
- [ ] All shell-out sites (`bash scripts/log-append.sh ...`,
  `bash scripts/lint.sh ...`, `qmd search ...`) use `--` before positional
  args.

- [ ] **Step 3: Final commit if any checklist item required a fix-up**

```bash
git status
# If anything is unstaged:
git add -p
git commit -m "fix: phase 13 final cleanup per phase-done checklist"
```

- [ ] **Step 4: Merge to main**

```bash
git checkout main
git merge --no-ff phase-13-synth-core -m "feat: complete phase 13 synth core"
git branch -d phase-13-synth-core
```

Expected: fast-forward refused (`--no-ff`); merge commit created on `main`.

- [ ] **Step 5: Tag (optional, only if reaching a stable boundary)**

Skip tagging until phases 14 and 15 ship; phase 13 is mid-feature.

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 14](./2026-04-27-phase-14-synth-lint-plugins.md).
