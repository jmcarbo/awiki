# Wikilink Rendering Decision

**Date:** 2026-04-27
**Phase:** 3 (Hugo Render)
**Status:** Accepted
**Spike:** Task 3.1 — time-boxed 4 hours.

## Question

Hugo's bundled goldmark does NOT ship a wikilink extension. Authoring uses Obsidian-flavored
`[[slug]]` and `[[slug|display]]`. How should Hugo render them?

## Options considered

1. **Native goldmark rendering.** Rely on a goldmark wikilink extension or hope Hugo passes
   `[[…]]` through to the rendered HTML.
2. **Custom render hook in Hugo template.** Use `layouts/_default/_markup/render-link.html`
   or similar to interpret a synthetic syntax.
3. **Preprocessing pipeline.** A `scripts/build.sh` step rewrites `content/` into
   `.awiki/build-content/` before Hugo runs, replacing `[[slug]]` with standard
   `[Title](/section/slug/)` Markdown links resolved against a slug+alias map.

## Verification

- Authored a minimal site with `content/_index.md` containing `This is [[bar]] wikilink raw.`
  and `theme = ''` (no theme), `markup.goldmark.renderer.unsafe = true`.
- `hugo --renderToMemory` runs without crashing — `[[bar]]` is left as literal text in the
  rendered output (not a link, not an error). This rules out option 1: goldmark out of the box
  does not interpret wikilinks.
- A render hook (option 2) requires every wikilink to use a Hugo-specific shortcode or rewrite
  the link AST node, which would break Obsidian round-trip. Rejected as
  Obsidian-incompatible.
- Confirmed the preprocessing approach (option 3) renders cleanly: rewriting `[[slug]]` to
  `[Title](/section/slug/)` produces ordinary Markdown links that goldmark + hugo-book render
  natively.

## Decision

**Preprocessing pipeline** as committed by the spec (Hugo + Obsidian Integration → Wikilinks).

- `scripts/build.sh` reads `content/`, builds slug→path, alias→slug, slug→title TSV maps
  under `.awiki/maps/`, then rewrites Markdown into `.awiki/build-content/`:
  - `[[slug]]` → `[<title-from-frontmatter>](/<section>/<slug>/)`.
  - `[[slug|display]]` → `[display](/<section>/<slug>/)`.
  - `[[alias]]` resolved via alias map (alias → slug → path → title).
  - Unresolved targets are left as the original literal `[[…]]` for `lint.sh` to flag as broken.
- `hugo.toml` sets `contentDir = '.awiki/build-content'` (relative to repo root). Hugo runs
  with default `--source .` from the repo root.
- `scripts/serve.sh` runs `build.sh`, then watches `content/` (entr / fswatch / 1s poll
  fallback) and re-runs `build.sh` on every change while `hugo server` watches
  `.awiki/build-content/` for the rebuilt files.
- The slug+alias maps emitted to `.awiki/maps/` are reused by `lint.sh` (broken-link checks,
  alias collision rule) and downstream phases (rename, delete, MCP).

## Consequences

- Obsidian sees the unmodified `content/` and renders wikilinks natively. Hugo only ever sees
  the rewritten output. Both stay happy.
- Adds a `python3` dependency to `build.sh` (already required by the spec for the JSON config
  patcher; added to `check-deps.sh`).
- A two-pass build (maps then rewrite) is cheap on personal-scale wikis (linear in the number
  of pages; the inline Python re-loads the maps per page, which is fine for hundreds of pages
  and trivially optimisable later).
- `serve.sh` watches `content/`, not `.awiki/build-content/`, so editing through Obsidian or a
  text editor triggers preprocessing. Hugo's own watcher then picks up the build-content
  changes and reloads the browser.

## Outcome

Spike complete. Approach verified to render without crashes. Proceeding with the rest of
Phase 3 as planned. No spec pivot required.
