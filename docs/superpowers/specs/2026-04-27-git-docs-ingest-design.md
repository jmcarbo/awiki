# Spec: Git-Docs Ingest

**Status:** draft
**Date:** 2026-04-27
**Author:** brainstormed with Claude (Opus 4.7)
**Phase target:** phase 18 (follows phase 17 task-layer)

## 1. Summary

Add a `scripts/ingest-git.sh` pipeline that ingests markdown documentation
from a git repository (local path or remote URL) into the awiki as
`type: source` pages, plus a single repo entity page per repo.

Scope is **docs only** — README, `docs/`, `rfcs/`, `adr/` and similar
markdown trees. Commit history mining, code symbol extraction, and
issues/PRs are **out of scope** for this spec; each is a separate future
sub-project.

## 2. Goals

- Ingest 1..N markdown files from a single repo in one bulk run.
- Both **local path** and **remote URL** (HTTPS + SSH) supported.
- **Incremental sync**: re-running ingests only the files whose blob SHA
  changed since the last run; removes derived pages for files deleted
  upstream.
- **Privacy-safe by default**: SSH URLs auto-route to `content/private/`;
  explicit per-repo `private: true` flag; no token-match heuristics.
- **No agent-in-loop required**: bulk path is mechanical (script-only).
  Optional `--summarize` flag reserves a future agent path.
- **Preserves wiki invariants**: passes existing `just lint` after a run;
  catalog/log/qmd reindex updated once per run.

## 3. Non-goals

- Cross-repo wikilinks (a doc in repo A linking to a doc in repo B).
- Commit-history narratives (deferred to `git-history` sub-project).
- Code symbol extraction / module entities (deferred to `git-code`).
- GitHub/GitLab issues + pull requests (deferred to `git-issues`).
- Auto-extraction of `entities/`/`concepts/`/`topics/` from ingested
  source bodies. Existing semantic-lint loop handles that.
- LLM-summarized source bodies — flag accepted, transform path TODO.
- Heuristic privacy scanning (token match like `SECRET`, `INTERNAL`).
- Submodule-based repo registration.
- GitHub API enrichment (stars, license, branch protection).
- Web/TUI UI for path selection.
- Auto-resolution of edit conflicts; `--protect-edits` stages proposals
  for the user only.

## 4. User stories

1. *"I want my project's `docs/` tree available in this wiki for search
   and cross-link."* → `just ingest-git /path/to/project --paths=docs/`.
2. *"I want a public OSS repo's documentation in the wiki, refreshed
   weekly."* → register repo in `.awiki/git-sources.yml`,
   `just ingest-git <name>` cron.
3. *"I want my private internal runbooks in the wiki without leaking
   them into Hugo render."* → SSH URL auto-flips to private; outputs
   land under `content/private/`.
4. *"I sometimes hand-edit derived pages."* → run with `--protect-edits`;
   conflicts written to `raw/inbox/checkpoint/.staged/`.

## 5. Decisions log (from brainstorm)

| # | Topic | Decision |
|---|---|---|
| Q3 | File scope | Curated dirs (default `README.md`, `docs/`, `rfcs/`, `adr/`) via config + `--paths=` CLI override. |
| Q4 | Repo source | Both local path and remote URL (HTTPS + SSH). |
| Q5 | Slug naming | `git-<repo>-<flatpath>` flat in `content/sources/`. Plus one repo entity page at `content/entities/repo-<repo>.md`. |
| Q6 | Frontmatter | Strip + replace upstream frontmatter (default). `--summarize` flag reserved for future LLM pass. |
| Q7 | Re-ingest | Per-file blob SHA diff (default). `--protect-edits` opt-in for conflict checkpoint. |
| Q8 | Privacy | (a) explicit `private: true` per repo, (b) SSH URL auto-private, (c) no heuristics, (d) defer auth to git's own credential layer. |
| Q9 | Body transform | Wikilink rewrite within same repo, image asset copy, code blocks verbatim. |
| Q10 | Pipeline integration | Hybrid: own bulk pipeline, single batched calls to log + catalog + qmd reindex. Frontmatter `provenance: git`. |
| Q11 | Entity/concept extraction | Out of scope v1; defer to existing semantic lint loop. |
| Approach | Implementation | Bash orchestrator + bash sub-libs + Python transform (CommonMark parser). |

## 6. Architecture

### 6.1 Components

**Orchestrator** — `scripts/ingest-git.sh`

- Args: `<repo-spec>` (path | URL | config alias), flags
  `--paths=`, `--private`, `--protect-edits`, `--summarize`, `--dry-run`,
  `--repo-name=<override>`.
- Justfile recipe `just ingest-git <spec>` proxies through (sibling of
  `ingest-pdf`, `ingest-audio`).

**Bash libs** — `scripts/lib/`

- `git-clone.sh` — resolve `<repo-spec>` to a local checkout under
  `raw/_git-cache/<repo_key>/`. Local path → use as-is. Remote URL →
  clone (`--filter=blob:none`) or fetch + reset. SSH URL OR `--private`
  flag OR config `private: true` → set `PRIVATE=1`. Surface git stderr
  verbatim on failure. Acquires lock via `lib/lock.sh` (30s profile).
- `git-state.sh` — read/write `.awiki/git-state/<repo_key>.json`. Atomic
  write (tmp + rename).
- `git-config.sh` — parse `.awiki/git-sources.yml`. Validates `name`
  matches `^[a-z][a-z0-9-]*$` and `paths` entries reject `..` / leading
  `/`. Default `paths` if absent: `[README.md, docs/, rfcs/, adr/]`.
  `paths` entries are matched as **directory prefix** (entry ending
  `/`) or **exact filename** (entry not ending `/`); glob syntax
  (`**`, `*`) is supported only in `exclude` entries.

**Python transform** — `scripts/ingest-git-transform.py`

- Single-file body rewrite. Real CommonMark parse (likely
  `markdown-it-py` — pinned in `scripts/check-deps.sh`).
- Strips upstream frontmatter, rewrites relative md links to wikilinks
  using a slug map passed on stdin (JSON), copies image assets to
  `content/sources/_assets/git-<repo>/<repo-relpath>/`, preserves code
  blocks verbatim, emits awiki frontmatter.
- Exit non-zero on parse failure; orchestrator records per-file failure
  but continues with remaining files.

### 6.2 Page kinds produced

**Source pages** at
`content/sources/git-<repo>-<flatpath>.md`
(or `content/private/sources/...` if private).

```yaml
---
title: "Doc title (from H1 or filename)"
date: 2026-04-27          # ingest date
last_updated: 2026-04-27
type: source
provenance: git
git_repo: awiki-upstream
git_path: docs/intro.md
git_blob_sha: 789...
git_url: https://github.com/foo/awiki.git
tags: [git, awiki-upstream]
sources: []
draft: false
---

(transformed upstream body)
```

**Repo entity page** at
`content/entities/repo-<repo>.md`
(or `content/private/entities/...` if private).

```yaml
---
title: "awiki-upstream"
date: 2026-04-27
last_updated: 2026-04-27
type: entity
tags: [git, repo]
aliases: [awiki-upstream]
git_url: https://github.com/foo/awiki.git
git_default_branch: main
git_sha: abc123...
last_ingested: 2026-04-27T14:32:11Z
---

Lead paragraph (from upstream README first paragraph).

## Sources

- [[git-awiki-upstream-readme]]
- [[git-awiki-upstream-docs-intro]]
- ...
```

### 6.3 Data flow (single run)

```
1.  ARGPARSE — merge CLI flags with config; compute repo_key.
2.  ACQUIRE LOCK — flock .awiki/lock/git-<repo_key> (30s).
3.  RESOLVE CHECKOUT — clone/pull or use local path; detect privacy.
4.  LOAD STATE — read prior_files map.
5.  WALK FILES — git ls-tree --name-only, filter to *.md / *.mdx,
    apply `paths` (directory prefix or exact filename), then apply
    `exclude` (glob); always skip vendored (node_modules/, vendor/,
    .git/).
6.  DIFF — added / modified / removed / unchanged classes by blob SHA.
7.  BUILD SLUG MAP — {repo-relpath: slug} for ALL current files (so
    wikilink rewrite resolves cross-file even for unchanged targets).
8.  TRANSFORM + WRITE — for each (added ∪ modified) file, pipe to
    Python transform with slug map; atomic write to out_path; copy
    referenced image assets.
9.  PROTECT EDITS — only if --protect-edits: detect conflicts (existing
    page diverged from prior-ingest body AND last_updated bumped),
    write proposal to raw/inbox/checkpoint/.staged/ instead of
    overwrite.
10. REMOVE STALE — for each removed file, move out_path →
    raw/_originals/git/<repo_key>/.
11. UPDATE REPO ENTITY PAGE — regenerate ## Sources section, stamp
    git_sha + last_ingested.
12. PERSIST STATE — write new .awiki/git-state/<repo_key>.json.
13. BATCH HOUSEKEEPING — one log entry, one update-catalog.sh, one
    qmd reindex.
14. RELEASE LOCK + SUMMARY.
```

`--dry-run` runs steps 1-7 then exits with the diff plan printed.

## 7. Config + state schemas

### 7.1 `.awiki/git-sources.yml` (optional, user-edited)

`paths` entries are directory prefix (trailing `/`) or exact filename
(no trailing `/`). Globs (`**`, `*`) are valid only in `exclude`. If
`paths` is omitted, default is `[README.md, docs/, rfcs/, adr/]`.

```yaml
schema: 1
repos:
  - name: awiki-upstream
    url: https://github.com/foo/awiki.git
    paths: [README.md, docs/, rfcs/]
    exclude: [docs/legacy/, "**/CHANGELOG.md"]
    private: false
    protect_edits: false
    summarize: false
    default_branch: main
```

### 7.2 `.awiki/git-state/<repo_key>.json` (machine-written, gitignored)

```json
{
  "schema": 1,
  "repo_key": "github-com-foo-awiki",
  "repo_name": "awiki-upstream",
  "url": "https://github.com/foo/awiki.git",
  "default_branch": "main",
  "head_sha": "abc123...",
  "ingested_at": "2026-04-27T14:32:11Z",
  "private": false,
  "files": {
    "docs/intro.md": {
      "blob_sha": "789...",
      "slug": "git-awiki-upstream-docs-intro",
      "out_path": "content/sources/git-awiki-upstream-docs-intro.md",
      "last_ingested": "2026-04-27T14:32:11Z"
    }
  }
}
```

### 7.3 `.gitignore` additions

```
.awiki/git-state/
.awiki/lock/
raw/_git-cache/
```

## 8. Error handling

| Failure | Exit | Behavior |
|---|---|---|
| Clone fails (auth/network/404) | 10 | Surface git stderr verbatim; state untouched. |
| Pull fails on existing cache | 11 | Print stderr + path; suggest `rm -rf raw/_git-cache/<key>/`. No auto-delete. |
| Config yaml parse error | 12 | Print line + column; lock not acquired. |
| Lock acquire timeout (30s) | 13 | Print holding pid + state-file path. |
| Slug collision | 14 | Hard error before any write; print both relpaths; suggest `exclude:` or `--repo-name=`. |
| Repo entity name conflict (existing `content/entities/repo-<name>.md` carries different `git_url`) | 15 | Hard error after step 3, before step 8 writes; force `--repo-name=<override>`. |
| Catalog update fails | 16 | Hard fail. |
| Python transform fails on file F | 2 (partial) | Append to failed_list; continue. State updates only successful files. |
| Asset copy fails on file F | 2 (partial) | Same as transform failure. |
| Image ref to missing file | 0 | Leave plain text; warn. |
| Wikilink target not in repo's slug map | 0 | Leave plain text (no cross-repo links v1). |
| Empty upstream body (<10 chars) | 0 | Skip; do not write near-empty source. |
| Binary / non-utf8 file | 0 | Skip; warn. |
| Public URL but `private: true` config | 0 | Honor explicit config (private). |
| SSH URL but `private: false` config | 0 | Honor explicit config (public); print warning. |
| `--protect-edits` conflict | 0 | Stage proposal in `raw/inbox/checkpoint/.staged/`; success. |
| Removed file's page hand-edited | 0 | Add to `manual_review` list; skip auto-graveyard. |
| qmd reindex fails | 0 | Warn (matches existing `ingest.sh` convention). |

### 8.1 Atomicity + idempotency

- Per-file write is atomic (tmp + rename within `content/`).
- State file write is atomic (tmp + rename in `.awiki/git-state/`).
- The batch is **not transactional** — partial run may leave `content/`
  partially updated. State file reflects only successful files, so
  re-run resumes correctly. Per-file ops are idempotent.
- Re-run with no upstream changes = no-op (head_sha + all blob_shas
  match → empty diff → no writes / no log / no catalog update).
- Manual `rm content/sources/git-foo-bar.md` between runs self-heals
  (out_path missing → next run re-creates).

### 8.2 Privacy invariants

- `private` flag computed once per run before any write.
- If `private=true`, assertion at every write: `out_path` starts with
  `content/private/`. Crash on mismatch (defense in depth).
- `tags: [private]` injected when `private=true`. Lint privacy rule
  catches a missing case.

## 9. Testing

### 9.1 Fixtures (under `tests/fixtures/`)

- `git-docs-good/` — local `git init` repo with README, docs/intro.md
  (relative link, fenced code, image), docs/foo.md, docs/img/arch.png,
  rfcs/0001-thing.md (with own Hugo frontmatter — Q6-A test),
  docs/legacy/old.md (excluded), node_modules/foo/README.md (vendored
  skip), CHANGELOG.md (exclude pattern test), docs/empty.md (<10 char
  test), docs/blob.md (utf-16 BOM, non-utf8 test).
- `git-docs-broken/` — slug collision pair, bad yaml in
  `.awiki/git-sources.yml`, missing image ref.
- `git-docs-private/` — repo with `private: true`.

### 9.2 Smoke tests (bash, under `tests/`)

- `smoke-git-docs-basic.sh` — happy path: 4 sources written, exclusions
  respected, asset copied, relative link → wikilink, upstream
  frontmatter stripped, repo entity present, state file present,
  catalog has new rows, one log entry.
- `smoke-git-docs-rerun.sh` — second run with no upstream change is
  no-op.
- `smoke-git-docs-incremental.sh` — modify + add + remove between runs;
  each class handled.
- `smoke-git-docs-protect-edits.sh` — user-edited derived page +
  upstream change → proposal in checkpoint, no overwrite.
- `smoke-git-docs-private.sh` — private fixture: outputs under
  `content/private/`, `tags: [private]`, lint clean.
- `smoke-git-docs-broken.sh` — slug collision (exit 14), bad yaml
  (exit 12), missing image (exit 0 + warning).
- `smoke-git-docs-collision.sh` — two repos both with `docs/intro.md`
  coexist (different `<repo>` prefix).
- `smoke-git-docs-lint.sh` — `just lint` clean after good ingest.

### 9.3 Python transform unit tests

In `scripts/test-ingest-git-transform.py` (only if a Python test
runner is already present in repo; otherwise inline coverage in smoke
tests). Cover:

- Frontmatter strip with/without upstream block.
- Wikilink rewrite NOT firing inside fenced code blocks.
- Image ref to existing file → copy + path rewrite.
- Image ref to missing file → leave + warn.
- Reference-style links (`[foo][1]\n\n[1]: ./foo.md`) → rewrite.
- Empty file detection.

### 9.4 Manual verification (post-impl)

- Self-host: ingest `awiki` repo's own `docs/` into the wiki.
- Smoke against a small public OSS docs repo.
- `just serve` then click derived pages in Hugo, confirm wikilink
  resolution and rendered images.

## 10. Open questions (resolve in implementation)

1. **CommonMark parser pin** — `markdown-it-py` likely; confirm during
   impl. Add to `scripts/check-deps.sh`.
2. **Cache fetch shape** — `--filter=blob:none` default; consider
   `--depth=1` config option if cache bloats.
3. **Asset dedup** — duplicates copied per-source v1 (cheap). v2 may
   content-hash.
4. **Hugo `_assets/` skip** — verify the underscore prefix excludes
   from render. Likely safe but verify.
5. **MCP tool** — surface `ingest_git` alongside existing
   `ingest_source`/`lint`/`query_wiki`/`update_catalog` MCP tools.
   Defer to phase 18.5 or v2.
6. **Justfile recipe naming** — `ingest-git` chosen (matches
   `ingest-pdf`, `ingest-audio` siblings).
7. **Default include paths** — locked at
   `[README.md, docs/, rfcs/, adr/]` (§6.1, §7.1). Consider adding
   `CONTRIBUTING.md`, `ARCHITECTURE.md`, `SPECIFICATION.md` after
   sampling typical repos in v2.

## 11. Phase + sequencing

- **Phase 18:** this spec.
- **Phase 19+ candidates** (separate specs): `git-history`, `git-issues`,
  `git-code`. Each should be brainstormed independently before any
  implementation work.
