# Go synth domain design

Date: 2026-04-29

## Goal

Port the awiki synth domain — currently driven by `scripts/synth.sh`
(~854 LOC) plus five Python helpers and two satellite shell scripts —
into the `awiki` Go binary as the nested command group `awiki synth`.

This is the first per-domain spec under the umbrella roadmap at
`docs/superpowers/specs/2026-04-29-go-port-roadmap-design.md`. The
shared building blocks landed in
`docs/superpowers/specs/2026-04-29-go-port-roadmap-design.md` (lifted
into `internal/{fsutil,emit,region,action,config,adapters}`); this
spec consumes them.

## Non-goals

- Reimplementing externals: `qmd` (scope query), `mmdc` (mermaid post-
  hook), `genanki`/Python (Anki export), arbitrary user post-hooks.
- Changing user-visible contracts: synth page bodies, frontmatter keys,
  region markers, log records (`SYNTH-*` stderr lines, `LINT|S*`
  records via lint).
- Removing the bash files before parity is proven and one release has
  shipped with the shim active.
- Reworking synth lint (S1–S9) — already ported into
  `internal/lint/synth/` and re-exported via the shared `region`
  package.
- Porting `scripts/synth-export-anki.{sh,py}` and
  `scripts/synth-mindmap-validate.sh` to pure Go. Both stay bash/Python
  forever (they wrap `genanki` and `mmdc` and are thin glue around
  those externals).

## Compatibility contract

Locked before the infra slice merges:

- Justfile recipes keep their current names and argument shapes:
  `synth`, `synth-regen`, `synth-finalize`, `synth-accept-stage`,
  `synth-refine`, `synth-list`, `synth-resolve`.
- Stderr `SYNTH-*` markers are byte-identical to the bash output (per-
  record; sorted-set when the bash emits in non-deterministic order).
- All written file shapes are byte-identical:
  - Page scaffold (frontmatter + `## Sources` + `<!-- BEGIN GENERATED v1 -->`
    region with empty body).
  - Frontmatter mutations: `last_generated`, `sources:`, `scope_hash`.
  - Region replacement on `regen` (clear region body, optionally write
    to `.staged/`).
  - Feedback append (`## Feedback` section, single bullet per call).
- Exit codes match: 0 on success, 1 on user error (missing slug, bad
  flag combo), 2 on internal error.
- Existing bats coverage continues to pass against the shim during
  transition (`synth_test.bats`, `synth_e2e_test.bats`,
  `synth_plugin_load_test.bats`, `synth_export_anki_test.bats`).

## Strategy

### CLI shape

Nested per the roadmap:

- `awiki synth list`
- `awiki synth resolve <slug>`
- `awiki synth refine <slug> <note...>`
- `awiki synth new <plugin> <topic> [--tag=… | --slugs=… | --query=…]
  [--exclude-tags=…] [--min-last-updated=YYYY-MM-DD] [--types=…]
  [--allow-private]`
- `awiki synth regen <slug> [--stage]`
- `awiki synth accept-stage <slug>`
- `awiki synth finalize <slug>`

The Anki export and mindmap-validate satellites stay bash and remain
flat verbs at the `awiki` level: `awiki synth-export-anki <slug>` and
`awiki synth-mindmap-validate <slug>` are thin Go wrappers that
`os/exec` the bash. They land in the cleanup slice along with the
shim flip.

### Sub-slice order (cheap leaf first)

1. **Infra slice** — package skeleton, types, fixture harness, CLI
   dispatch (returns "not yet ported" for every verb). No bash
   changes. PR-1.
2. **`list`** — read every plugin manifest under
   `plugins/synth/*.md`, emit one `name|description|sources|version`
   line per plugin. No filesystem mutation. PR-2.
3. **`resolve`** — given a synth page slug, parse its `scope:` block
   and emit the resolved slug list to stdout. Read-only. PR-3.
4. **`refine`** — append a `- <note>` bullet to the `## Feedback`
   section (idempotent for byte-identical notes). PR-4.
5. **`new`** — scaffold a synth page (frontmatter + sources + region
   markers). Calls scope resolution + plugin loader. PR-5.
6. **`regen`** — clear the GENERATED region, optionally copy the page
   to `.staged/`. PR-6.
7. **`accept-stage`** — promote `.staged/<slug>.md` over live and run
   `post_hook` if declared. PR-7.
8. **`finalize`** — recompute scope hash, refresh `sources:`, update
   `last_generated`, run `post_hook`. PR-8.
9. **Cleanup slice** — flip shim to `exec awiki synth …`, delete bash
   verb branches as they ported (already deleted in each sub-slice;
   this slice deletes the residual `scripts/synth.sh` shim entry plus
   the now-unused python helpers consumed only by lint that are not
   already lifted, plus the bats cases now redundant). PR-9.

Total: 9 PRs, plus the eventual final cleanup once the release window
closes.

### Compatibility strategy: shim then delete

Each verb sub-slice:

1. Implements the verb in Go.
2. Adds a golden fixture under
   `tests/fixtures/synth/<verb>/{input,args,expected_stdout,expected_files}`.
3. Removes the corresponding `case` branch from `scripts/synth.sh`.
4. Justfile remains unchanged.

The infra slice does not touch `scripts/synth.sh`. Verb 1 (`list`)
flips that script to begin executing the Go binary if `awiki` is on
PATH and `AWIKI_SYNTH_LEGACY != 1`, mirroring the lint shim pattern.

### Externals

Stays exec, wrapped behind `internal/adapters` interfaces:

- **`qmd`** — `Qmd.Search(repoRoot, query)` (already in
  `internal/adapters/interfaces.go`).
- **`mmdc`** — used only by `synth-mindmap-validate.sh`; not exposed
  via Go this slice.
- **`genanki`** — used only by `synth-export-anki.{sh,py}`; not exposed
  via Go this slice.
- **Plugin post-hooks** — arbitrary user-supplied bash. New adapter:
  `PostHook.Run(ctx, scriptPath, pagePath) (output string, code int,
  err error)`. Lives in `internal/adapters/posthook.go`. Gated by
  `.awiki/config` `ALLOW_PLUGIN_POST_HOOKS=1`.
- **`git`** — only used by lint S6, already covered.

## Architecture

### Package layout

```
cmd/awiki/main.go
internal/
  cli/synth.go         # nested-group dispatch
  synth/               # domain pkg
    plugin.go          # plugin manifest loader + body extraction
    plugin_test.go
    scaffold.go        # page scaffold (new)
    scaffold_test.go
    scope.go           # scope resolution (tag/slugs/query)
    scope_test.go      # delegates query to adapters.Qmd via fake
    region.go          # GENERATED region read/write helpers
    region_test.go
    list.go            # list verb
    resolve.go         # resolve verb
    refine.go          # refine verb
    new.go             # new verb
    regen.go           # regen verb
    accept.go          # accept-stage verb
    finalize.go        # finalize verb
    runner.go          # shared verb context (paths, config, adapters)
    runner_test.go
  adapters/posthook.go # PostHook interface + ExecPostHook
internal/testutil/
  synth_fixture.go     # fixture loader + golden differ shared by verb tests
```

### Shared types

```go
// Plugin is the parsed contents of plugins/synth/<name>.md.
type Plugin struct {
    Name                  string
    Description           string
    OutputType            string
    OutputSubtype         string
    MinSources            int
    MaxSources            int
    MaxEvidenceTotalWords int
    RequiredSections      []string
    PostHook              string
    Render                string
    Version               string
    Body                  string // prompt template
}

// Scope is the parsed scope block from a synth page frontmatter.
type Scope struct {
    Kind          string   // "tag" | "slugs" | "query"
    Tag           string
    Slugs         []string
    Query         string
    ExcludeTags   []string
    MinLastUpdated string
    Types         []string
    AllowPrivate  bool
}

// Page is a synth page on disk, plus its parsed scope and metadata.
type Page struct {
    Slug          string
    Path          string
    Plugin        string
    Topic         string
    Scope         Scope
    LastGenerated string
    SourcesHash   string
    Frontmatter   map[string]string
}
```

`internal/synth/runner.go` exposes `type Runner struct` carrying
`fsutil.Locker`, `adapters.Qmd`, `adapters.PostHook`, and the resolved
repo + content + plugin dirs. Each verb is `func (r *Runner)
<Verb>(ctx, args) error`. Tests inject fakes via the runner.

## Domain scope

### `list`

Walk `plugins/synth/*.md`, parse each manifest, emit one line per
plugin to stdout in name-sorted order:

```
PLUGIN|<name>|version=<version>|min=<min_sources>|max=<max_sources>|sections=<comma-list>
```

Pin against the existing bash output (golden fixture). No mutations.

### `resolve`

Read `<content-root>/synth/<slug>.md`, parse the `scope:` block from
frontmatter, return the resolved slug list. The `query` flavor calls
`adapters.Qmd.Search`. Filtering (`exclude_tags`, `min_last_updated`,
`types`, `allow_private`) is applied in Go. Output: one slug per line,
ASCII sorted.

### `refine`

Locate the `## Feedback` section (create if missing), append a
`- <note>` bullet, write atomically via `fsutil.AtomicWrite`. If the
identical bullet (text-only match) already exists as the last line of
the section, the call is a no-op (same byte output). Logs to stderr:
`SYNTH-REFINE|<slug>|<note-hash>`.

### `new`

1. Validate plugin exists (via plugin loader).
2. Resolve scope; validate `min_sources` ≤ resolved-count ≤
   `max_sources`.
3. Compute `scope_hash` = first 6 hex chars of SHA-256 of newline-
   joined slugs (matches `lint-synth-hash.py`).
4. Render scaffold:
   - Frontmatter with `slug`, `type: synth`, `synth_plugin`, `topic`,
     `scope:` block, `sources:`, `scope_hash`, `created`, `tags`.
   - `## Sources` section listing wikilinks.
   - Empty `<!-- BEGIN GENERATED v1 ... --> ... <!-- END GENERATED -->`
     region (begin line carries `scope_hash=…|sources_hash=…`).
   - `## Feedback` section header.
5. Write to `<content-root>/synth/<topic>-<plugin>.md`. Refuse to
   overwrite (exit 1) unless `--force`.
6. Stderr: `SYNTH-NEW|<slug>|sources=<count>|hash=<hash>`.

### `regen`

1. Read live page; verify it's a synth page.
2. Strip the GENERATED region body (preserve markers).
3. If `--stage`, copy the result to
   `<content-root>/.staged/<slug>.md` instead of writing live.
4. Stderr: `SYNTH-REGEN|<slug>|staged=<bool>`.

### `accept-stage`

1. Read `<content-root>/.staged/<slug>.md`. Error if absent.
2. Atomically replace `<content-root>/synth/<slug>.md`.
3. Run `post_hook` if declared in plugin AND
   `ALLOW_PLUGIN_POST_HOOKS=1` is in `.awiki/config`. Hook receives
   `--page <path>` argument.
4. Stderr: `SYNTH-ACCEPT|<slug>|hook=<bool>`.
5. Remove the staged file on success.

### `finalize`

1. Read live page.
2. Recompute scope (re-resolve scope; refresh `sources:` block;
   recompute `scope_hash`).
3. Update region begin line: `scope_hash=…|sources_hash=…`.
4. Update frontmatter: `last_generated: <today>`, `sources:`,
   `scope_hash`.
5. Run `post_hook` if declared + gated.
6. Stderr: `SYNTH-FINALIZE|<slug>|hash=<hash>|hook=<bool>`.

## Parity oracle

**Golden** (per-verb, under `tests/fixtures/synth/<verb>/`):

- Input: a small wiki tree (`content/`, `plugins/synth/`, `.awiki/`).
- Args: the exact CLI args.
- `expected_stdout`: byte-equivalent output.
- `expected_stderr`: byte-equivalent records (sorted set).
- `expected_files/`: every file produced or modified, byte-equivalent.

The differ normalizes timestamps (`last_generated`, `created`) and
absolute paths.

**Behavioral** (orchestration glue):

- `accept-stage` post-hook fires (assert hook side effect, not stdout).
- `refine` idempotent under repeated identical notes.
- `new --stage` is a no-op (refuse).
- Scope query path actually invokes the `Qmd` adapter.

The 6 existing bats files stay green throughout via the shim. They
delete in the cleanup slice.

## Cleanup criteria

Standard, per the roadmap:

- All 7 verbs ported and Go-tested.
- Bats coverage matched or migrated.
- One release shipped with shim active and no parity bug filed.
- All bash and Python files in the synth domain deleted in the
  cleanup PR (`scripts/synth.sh`, `scripts/synth-plugin-load.sh`).
- `scripts/synth-export-anki.{sh,py}` and
  `scripts/synth-mindmap-validate.sh` stay (they are Python/mmdc glue,
  exposed via `awiki synth-export-anki` / `awiki synth-mindmap-validate`
  flat verbs that exec the bash).
- Justfile updated to call `awiki synth <verb>` directly.

## Risks and mitigations

- **Plugin manifest YAML edge cases.** The bash uses awk-style
  scalar/list parsing (`required_sections:` block-form vs. inline
  form). Mitigation: golden fixture covers both forms; if the Go YAML
  parser disagrees with the bash for a real plugin, lock the bash
  behavior in tests before fixing.
- **Scope query non-determinism.** `qmd query` returns slugs in
  unspecified order on some indexes. Mitigation: sort + dedupe in Go
  before emitting; pin sort key in spec.
- **Post-hook safety.** Arbitrary shell. Mitigation: gate by
  `ALLOW_PLUGIN_POST_HOOKS=1`; the adapter does not pipe stdin and
  enforces a fixed timeout (default 60s).
- **Frontmatter-write fidelity.** Existing `internal/wiki` parses
  frontmatter but write-back path is untested for synth shape.
  Mitigation: golden fixture covers all five mutated keys; whitespace
  preservation tested explicitly.
- **Staged-file collision.** `accept-stage` while another writer is
  finalizing. Mitigation: `fsutil.WithExclusiveLock` around the
  read-modify-write; exit 7 on contention (matches lock.sh).

## Open questions (defer to per-verb sub-slice)

- **Plugin discovery dir.** Bash uses `$AWIKI_SYNTH_PLUGINS_DIR` env
  override falling back to `plugins/synth/`. Go should support the
  same env override; pin in `list` sub-slice.
- **`refine` idempotence semantics.** If the user appends the same
  note twice intentionally (e.g. as emphasis), do we honor it? Bash
  appends unconditionally. Mitigation: keep bash semantics (always
  append). Drop the "idempotent for byte-identical notes" claim above
  if the bats tests show otherwise.
- **`new --force` flag.** Bash refuses to overwrite. Should Go expose
  a `--force` escape hatch, or stay strict? Default: strict. Pin in
  `new` sub-slice.

## Out of scope of this domain spec

- The other six domains (dataset, chart, query, ingest, template,
  ops) — each gets its own spec when its turn arrives.
- Synth lint rule changes — already shipped; this spec only consumes
  `internal/lint/synth/` for the `--lint` toggle inside `finalize`.
- Anki export and mindmap validate Go ports — they stay bash forever.
- MCP integration of synth verbs — `mcp/` evolves separately.
