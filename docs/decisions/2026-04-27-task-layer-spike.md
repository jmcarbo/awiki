# Phase-17 Spike — Obsidian Tasks plugin + `~`-separator block-ID round-trip

**Date:** 2026-04-27
**Phase:** 17 (Scanner + Agenda)
**Status:** Tentative — operator verification pending. Default = `~`.

## Context

Two assumptions to settle before phase 17 commits to inline-action grammar:
1. Obsidian Tasks plugin Dataview-mode interop with awiki's key:value tail metadata.
2. `~` separator in block-IDs (`^a05~2`) round-trips through Obsidian source/preview AND Hugo HTML rendering.

This spike was authored without an interactive Obsidian session. The fixture vault
at `tests/fixtures/spike-obsidian-vault/` is in place; an operator can run the
verification checklist when convenient. Phase 17 proceeds with the default
assumptions (separator = `~`, plugin compat is best-effort) and documents the
fallback path here for use if either assumption fails.

## Assumption 1 — Obsidian Tasks plugin Dataview-mode interop

**Plugin:** `obsidian-tasks-plugin` (Tasks)
**Required setting:** "Dataview format" toggled ON.

| Check | Expected | Operator result |
|-------|----------|-----------------|
| All six status markers (`[ ]`, `[/]`, `[?]`, `[>]`, `[x]`, `[-]`) render distinctly in source + preview | YES | _pending_ |
| Tail keys (`due:`, `defer:`, `wait:`, `since:`, `every:`, `done:`, `priority:`, `est:`) survive a focus/blur cycle without being rewritten | YES | _pending_ |
| Default emoji mode (with Dataview format also on) does not silently misparse our key:value tokens | YES | _pending_ |

**Default outcome (if not yet verified):** assume "compatible if user enables
Dataview format; not required for awiki workflows." This matches the spec's
Goal G2 wording exactly. No code change needed; the awiki scripts do not
depend on the plugin being installed.

**Fallback if any check fails:**
- Downgrade Goal G2 in `2026-04-27-task-layer-design.md` from
  "interoperable with Obsidian Tasks plugin (Dataview format)" to
  "renders correctly in vanilla Obsidian without plugin-compat claim."
- No script changes required.
- Update WIKI.md task-layer block (phase-19 polish task) to drop the
  plugin reference.

## Assumption 2 — `~`-separator block-ID round-trip

**Default:** `AWIKI_RECUR_SEP="~"` (set in phase-16 `scripts/lib/action-grammar.sh`).
The block-ID regex is `^[a-z0-9]{3,16}(~[0-9]+)?$`; chain instances are
`^<base>~<n>` for `n ≥ 2`.

| Check | Expected | Operator result |
|-------|----------|-----------------|
| `^a05~2` renders in Obsidian source mode without breaking | YES | _pending_ |
| `^a05~2` renders in Obsidian preview mode as a block anchor | YES | _pending_ |
| `[[notes#^a05~2]]` resolves on Cmd-click | YES | _pending_ |
| Hugo emits an HTML anchor preserving the `~` (or a deterministic safe escape) | YES | _pending_ |

**Default outcome (if not yet verified):** proceed with `~`. Awiki's grammar lib
already exports `AWIKI_RECUR_SEP="${AWIKI_RECUR_SEP:-~}"` so a future flip is
one env var override + one regex rebuild.

**Fallback if any check fails — switch to `__`:**

1. Edit `scripts/lib/action-grammar.sh`: change the default to
   `AWIKI_RECUR_SEP="${AWIKI_RECUR_SEP:-__}"`.
2. Phase-16 grammar tests already parameterise on `AWIKI_RECUR_SEP`, so they
   continue to pass — but flip any literal `~` in test assertions to `__`.
3. Patch the spec at `docs/superpowers/specs/2026-04-27-task-layer-design.md`:
   ```bash
   sed -i.bak -e 's/\^<base>~<n>/\^<base>__<n>/g' \
              -e "s/\\\`~\\\` is unambiguous/\\\`__\\\` is unambiguous/g" \
     docs/superpowers/specs/2026-04-27-task-layer-design.md
   rm -f docs/superpowers/specs/2026-04-27-task-layer-design.md.bak
   ```
4. Update T14 (`bad-id-shape`) lint rule to match.
5. Re-run `bats tests/action_grammar_test.bats` — should still pass (chain
   regex is rebuilt from the separator constant).

## Decision

**Tentative:** Proceed with `~` separator. Plugin-compat is documented as
"works if user enables Dataview format; not required."

If/when an operator runs the fixture vault and finds an issue, update the
"Operator result" columns above and apply the fallback patch in this same
document's commit history.
