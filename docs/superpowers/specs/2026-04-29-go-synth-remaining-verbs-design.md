# Go synth remaining verbs design

Date: 2026-04-29

## Goal

Port the four remaining synth verbs (`new`, `regen`, `accept-stage`,
`finalize`) plus their shared helpers from `scripts/synth.sh` (~520 LOC
across these verbs and helpers) into `internal/synth/` under
`feat/synth-remaining-verbs`.

This is the follow-up slice to the synth domain port shipped as
`d07276b` (`list`/`resolve`/`refine`). It consumes the shared building
blocks already shipped in `internal/{fsutil,emit,region,action,config}`
plus the synth primitives (`Plugin`, `Scope`, `Runner`, scope resolver,
plugin loader) shipped in the prior slice.

After this slice ships: all seven `awiki synth` verbs run in Go;
`scripts/synth.sh` becomes a thin shim that always delegates to Go;
the python helpers consumed by these verbs are still on disk and
deletable in a future cleanup slice.

## Non-goals

- Reimplementing `qmd`, `mmdc`, `genanki`, post-hook user shell, `git`
  CLI. All stay exec.
- Deleting `scripts/synth.sh` or any python helpers. Cleanup is gated
  on the release window per the umbrella roadmap.
- Changing user-visible contracts. Stdout (prompt bundle) and stderr
  (`SYNTH-*` records) byte-equivalent to bash. File shapes preserved
  byte-equivalent except for `last_generated` (UTC timestamp) and
  `last_updated` (date) which are time-dependent and normalized by
  the golden differ.
- Re-implementing the synth lint S1–S9 rules. They already live in
  `internal/lint/synth/` and are invoked via the existing
  `scripts/lint.sh` shim called from `accept-stage` and `finalize`.

## Compatibility contract

Locked before merge:

- Justfile recipes (`synth`, `synth-regen`, `synth-finalize`,
  `synth-accept-stage`) keep their current names and argument shapes.
- Stderr `SYNTH-NEW`, `SYNTH-REGEN`, `SYNTH-ACCEPT-STAGE`,
  `SYNTH-FINALIZE` records byte-identical to bash.
- Stdout prompt bundle byte-identical (modulo any in-line dates that
  bash also varies by run).
- Page on-disk byte-identical except for time-dependent fields
  (`last_updated`, `last_generated`) and the BEGIN GENERATED line's
  `scope_hash=` (deterministic given inputs).
- Exit codes match bash:
  - 0 — success
  - 1 — usage error (missing arg, unknown flag, missing file)
  - 2 — privacy violation, plugin min/max source bound violation
  - 3 — overwrite refused (`new` only)
  - 4 — hand-edit detected (`regen` only)
  - 5 — marker integrity failure (`finalize` only)
  - 6 — scoped lint or post-hook failure
  - 7 — no staged file (`accept-stage` only)
- Existing bats coverage (`synth_test.bats`, `synth_e2e_test.bats`)
  continues to pass against the shim during transition.

## Strategy

### CLI surface (already wired)

The dispatcher in `internal/cli/synth.go` returns "verb not yet ported"
for these four verbs today. This slice replaces those branches with
real implementations. Each verb is added to the `AWIKI_SYNTH_GO_VERBS`
array in `scripts/synth.sh` so the shim diverts to Go once the verb
ships.

### Shared helpers

Ported once and consumed by multiple verbs:

- `internal/synth/region.go` (new): `ClearRegion`, `CheckMarkers`,
  `SetScopeHashInMarker`. The synth-flavor BEGIN/END parser already
  lives in `internal/region`; these helpers wrap it for the synth-
  specific writes.
- `internal/synth/frontmatter.go` (new): `FmSetScalar`, `FmSetSources`.
  Mutate scalar/list keys in YAML frontmatter while preserving the
  rest byte-equivalent.
- `internal/synth/handedit.go` (new): `HandEditCheck` — `regen`-only
  guard. Uses `adapters.Git` for `ls-files` and `show HEAD:<path>`.
  Returns `(handEdited bool, untracked bool, err error)`. Untracked or
  no-HEAD ⇒ treat as fresh (no error, no edit).
- `internal/synth/scaffold.go` (new): `WriteScaffold` — page bytes
  built from a `ScaffoldInput` struct, shape locked to the bash
  `synth_write_scaffold` heredoc.
- `internal/synth/prompt.go` (new): `EmitPrompt` — render the plugin's
  prompt template with mustache-style substitution (`{{scope_description}}`
  literal; `{{#pages}}...{{/pages}}` block expansion;
  `{{#feedback}}...{{/feedback}}` conditional). Helpers
  `pagesBlock(slugs, lookup)` and `renderFeedbackBlock(pageBytes)` move
  here.
- `internal/synth/posthook.go` (new): `RunPluginPostHook(plugin,
  pagePath)` — gates on `r.AllowPostHooks()`, then dispatches to the
  injected `adapters.PostHook`. Stderr emits `SYNTH|post-hook-blocked`
  or `SYNTH|post-hook-failed` per bash.

### Verb implementations

Each is a method on `*Runner`. Order in implementation:

1. **`Regen`** — first because `ClearRegion`, `HandEditCheck`,
   `EmitPrompt` land with it and the other two verbs reuse them.
2. **`AcceptStage`** — depends on lint-shell-out + `RunPluginPostHook`.
3. **`Finalize`** — depends on `CheckMarkers`, `SetScopeHashInMarker`,
   frontmatter setters, `RunPluginPostHook`, ingest counter bump.
4. **`New`** — depends on `WriteScaffold`, `EmitPrompt`, scope two-pass
   with privacy detection. Largest verb; lands last so all helpers are
   in place.

### Helpers shelling out

- `r.Lint(file string)` — wraps `bash scripts/lint.sh --only=synth
  --file=<path>` via `os/exec`. Used by `accept-stage` and `finalize`.
  Returns exit code; non-zero is non-fatal here only when caller maps
  it to exit 6.
- `r.Git` — new field on `Runner`, wired to `adapters.ExecGit` (a
  concrete impl alongside `ExecQmd`/`ExecPostHook` we already have).
  `ExecGit.LsFiles(repoRoot, path)` and `ExecGit.ShowHead(repoRoot,
  path)` are the only methods needed by `regen`.

### Two-pass scope resolution (`new`)

`new` resolves scope twice:

1. **Without** privacy stripping (force `SCOPE_TARGET_PRIVATE=1`),
   collect the full slug list. Walk it; if any slug has the `private`
   tag and the synth target is not itself in `content/private/` and
   the user did not pass `--allow-private`, fail with exit 2.
2. **With** real privacy enforcement, compute the canonical slug list
   that becomes `sources:` and feeds `scope_hash`.

The existing `Resolve` in `resolve.go` already supports a "ignore
privacy" mode via the `Scope.AllowPrivate` flag. The `new` verb sets
this flag on the first pass, restores on the second.

## Architecture

### Package layout (additions to existing `internal/synth/`)

```
internal/synth/
  ...existing (list.go, resolve.go, refine.go, plugin.go, scope.go,
  runner.go, types.go)...
  region.go              # ClearRegion, CheckMarkers,
                         # SetScopeHashInMarker
  region_test.go
  frontmatter.go         # FmSetScalar, FmSetSources
  frontmatter_test.go
  scaffold.go            # WriteScaffold
  scaffold_test.go
  prompt.go              # EmitPrompt + pagesBlock +
                         # renderFeedbackBlock
  prompt_test.go
  handedit.go            # HandEditCheck (regen only)
  handedit_test.go
  posthook.go            # RunPluginPostHook (accept-stage, finalize)
  posthook_test.go
  new.go                 # (*Runner).New
  new_test.go
  regen.go               # (*Runner).Regen
  regen_test.go
  accept.go              # (*Runner).AcceptStage
  accept_test.go
  finalize.go            # (*Runner).Finalize
  finalize_test.go

internal/adapters/
  git.go                 # ExecGit{LsFiles, ShowHead}
  git_test.go            # compile-time assertion only
  lint.go                # ExecLint shells `bash scripts/lint.sh`;
                         # caller maps exit to verb-specific code
  lint_test.go
```

### Runner additions

```go
// Runner extends with adapters needed by the new verbs.
type Runner struct {
    // existing fields...
    Git  adapters.Git
    Lint adapters.Lint
}
```

`Lint` is a new interface on `internal/adapters/`:

```go
type Lint interface {
    Run(ctx context.Context, repoRoot string, args ...string) (output string, code int, err error)
}
```

`ExecLint` shells `bash <repoRoot>/scripts/lint.sh <args...>`. Each
verb passes its own arg list (`--only=synth --file=<path>`).

## Verb scope

### `new`

CLI: `awiki synth new <plugin> <topic> (--tag=… | --slugs=… |
--query=…) [--exclude-tags=…] [--min-last-updated=YYYY-MM-DD]
[--types=…] [--allow-private]`.

Pipeline:

1. Parse + validate flags. Exit 1 on shape errors.
2. Validate `topic` slug pattern.
3. Load plugin (existing `LoadPlugin`).
4. Build a `Scope` from the flag values.
5. Privacy first-pass: clone `Scope` with `AllowPrivate=true`, run
   `Resolve` against it, collect all slugs that resolve. If any has
   the `private` tag and (target not under `content/private/`) and
   (`!--allow-private`) → exit 2.
6. Privacy second-pass: real resolve with `AllowPrivate` flag from
   CLI.
7. Validate `MinSources <= len(slugs) <= MaxSources` if plugin
   declares them. Exit 2 on bound violation.
8. Refuse overwrite. Exit 3 if `<SynthDir>/<topic>-<plugin>.md` exists.
9. Compute `ScopeHash(slugs)`.
10. Build scope YAML block matching bash exactly:
    ```
    scope:
      tag: <val>          # OR slugs: [<val>]  OR query: "<val>"
      exclude_tags: [<val>]   # if non-empty
      min_last_updated: <val> # if non-empty
      types: [<val>]          # if non-empty
    ```
11. `WriteScaffold` (see helper) into the target.
12. `EmitPrompt(slugs, scopeDesc, "")` to stdout.
13. `bash scripts/log-append.sh synth-scaffold -- "<plugin> <topic>"`.
14. If declassified: `bash scripts/log-append.sh synth-declassify -- "<topic> sources=<n>"`.
15. Stderr: `SYNTH-NEW|target=<path>|plugin=<name>|sources=<n>|scope_hash=<hash>`.

### `regen`

CLI: `awiki synth regen <slug> [--force] [--stage]`.

Pipeline:

1. Parse flags. Exit 1 on shape errors.
2. Read live page; missing → exit 1.
3. Read `plugin:` from frontmatter; missing → exit 1.
4. Load plugin.
5. If `!--force && !--stage`: `HandEditCheck`. If hand-edited → exit 4.
   Untracked or no-HEAD → proceed.
6. Read scope; privacy fail-closed re-check (`new`'s first-pass
   logic). If newly-private source in scope and target not private →
   exit 2.
7. Real `Resolve` for canonical slug list.
8. Compute `ScopeHash`.
9. Choose target: `--stage` ⇒ copy live to `<ContentDir>/.synth-staged/<slug>.md`;
   else target is live.
10. `ClearRegion(target, plugin, scopeHash)` — preserves markers,
    rewrites BEGIN line with the new plugin+hash, empties body to
    a single newline.
11. `EmitPrompt(slugs, scopeDesc, renderFeedbackBlock(target))` to
    stdout.
12. Stderr: `SYNTH-REGEN|target=<path>|stage=<0|1>|scope_hash=<hash>`.

### `accept-stage`

CLI: `awiki synth accept-stage <slug>`.

Pipeline:

1. Parse args. Exit 1 if missing slug.
2. Staged file at `<ContentDir>/.synth-staged/<slug>.md`. Missing →
   exit 7.
3. `r.Lint.Run(ctx, repoRoot, "--only=synth", "--file="+staged)`. Non-
   zero → exit 6.
4. Atomic move (`os.Rename`) staged → live. (Bash uses `mv`; same
   semantics on local filesystems.)
5. Read `plugin:` from live; load plugin.
6. If plugin declares `post_hook`: `RunPluginPostHook(plugin, live)`.
   Failure → exit 6.
7. `bash scripts/log-append.sh synth -- "<plugin> <slug> accept-stage"`.
8. Stderr: `SYNTH-ACCEPT-STAGE|target=<live>`.

### `finalize`

CLI: `awiki synth finalize <slug>`.

Pipeline:

1. Parse args. Exit 1 on missing slug.
2. Target = staged if present, else live. Missing → exit 1.
3. `CheckMarkers(target)`. Failure → exit 5.
4. `r.Lint.Run(ctx, repoRoot, "--only=synth", "--file="+target)`.
   Non-zero → exit 6.
5. Read scope; resolve canonically.
6. Compute `ScopeHash`.
7. `SetScopeHashInMarker(target, hash)` — `sub` regex on
   `scope_hash=[0-9a-f]+` inside the BEGIN line.
8. `FmSetSources(target, slugs)` — replace the `sources:` value
   line with `["[[s1]]", "[[s2]]", ...]`. Block form not supported
   (matches bash).
9. `FmSetScalar(target, "last_generated", time.Now().UTC().Format(time.RFC3339))`.
   The bash uses `'+%Y-%m-%dT%H:%M:%SZ'` — equivalent to RFC3339 in
   UTC.
10. Read `plugin:`; load plugin; if `post_hook` declared,
    `RunPluginPostHook`. Failure → exit 6.
11. `bash scripts/log-append.sh synth -- "<plugin> <slug>"`.
12. Bump `.awiki/ingest-count` (read int, +1, write back).
13. Stderr: `SYNTH-FINALIZE|target=<path>|sources=<n>|scope_hash=<hash>`.

## Parity oracle

Per verb:

- **Golden** for stderr records (byte-identical) and stdout prompt
  bundle (byte-identical modulo time fields normalized by the differ).
- **Behavioral** for filesystem mutations: assert
  `<live>/<staged>` matches expected after mutation, ignoring
  `last_updated`, `last_generated`, and any timestamp lines.
- **Live wiki diff** as a smoke step in each task: take a real synth
  page, snapshot, run bash, snapshot, restore, run go, snapshot, diff.

Test harness: `internal/testutil.CopyTree` (already shipped) plus a
new helper `Diff(t, expected, actual)` that strips date lines and
compares.

## Risks and mitigations

- **`EmitPrompt` template fidelity.** The mustache-style interpolation
  has three forms (`{{scope_description}}`, `{{#pages}}...{{/pages}}`,
  `{{#feedback}}...{{/feedback}}`) and a feedback-empty-block strip.
  Mitigation: golden fixture using the real `briefing.md` plugin from
  `synthesis-plugins/`, plus a unit test for each substitution form.
- **Hand-edit detection.** Bash extracts BEGIN..END from both
  `git show HEAD:<path>` and the working tree, then string-compares.
  Mitigation: `HandEditCheck` ports the same string compare; tests
  cover untracked, no-HEAD, equal regions, divergent regions.
- **Frontmatter mutations.** The bash helpers replace lines via awk;
  preserving line order and existing whitespace is the contract.
  Mitigation: golden fixture for each setter with adjacent-line
  preservation asserted.
- **Staged dir naming.** Bash uses `STAGED_DIR` which is
  `<ContentDir>/.synth-staged` per phase 13 (verify against
  `scripts/synth.sh` constants). The Go must use the same path.
- **`log-append.sh` invocation.** Bash callers go through
  `bash $REPO_ROOT/scripts/log-append.sh`. Mitigation: introduce a
  small adapter (or piggyback on `Lint` adapter pattern) — call
  `bash <repoRoot>/scripts/log-append.sh <topic> -- <message>`. The
  log file is append-only; if the script fails we surface but do not
  fail the verb.
- **UTC timestamp fidelity.** `time.Now().UTC().Format(time.RFC3339)`
  yields `2026-04-29T15:30:00Z`. Bash `date -u '+%Y-%m-%dT%H:%M:%SZ'`
  yields the same shape. Verify with a unit test.

## Open questions

- **Should `r.Lint.Run` fail closed or log-and-continue?** Bash fails
  closed with exit 6. Go should mirror this. Confirm during
  implementation.
- **`accept-stage` lint includes the staged file's full path on
  command-line.** Bash passes `--file="$staged"` (relative). Go
  should pass the same. Confirm during smoke test.
- **`AGENT-PROMPT|` records.** None of these four verbs emit
  `AGENT-PROMPT|`; this stays a non-issue.

## Out of scope

- All deferred items from the umbrella roadmap.
- Cleanup slice (delete `scripts/synth.sh` and python helpers).
- MCP synth handlers — separate binary.
