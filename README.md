# awiki

A template repository for building personal LLM-maintained wikis. Domain-agnostic. Multi-agent (Claude Code, Codex, OpenCode). Hugo-renderable. Obsidian-friendly. Search via qmd.

## Quick start

```bash
git clone --recurse-submodules <this-repo> mywiki
cd mywiki
just                          # see available commands
```

(If you cloned without `--recurse-submodules`, run `git submodule update --init --recursive` to fetch the Hugo theme.)

Open your agent (Claude Code, Codex, OpenCode) in this directory. Say:

> init wiki

The agent reads `BOOTSTRAP.md` and walks through customization (domain, name, encryption, theme, qmd install). Done.

## Smoke test (post-bootstrap)

```bash
echo "# Sample\nA test source about caves." > raw/inbox/interactive/sample.md
just ingest raw/inbox/interactive/sample.md   # agent processes via WIKI.md flow
just lint                                     # validate wiki integrity
just serve                                    # local Hugo preview
just search "cave"                            # qmd search (if installed)
```

## Layout

- `content/` — LLM-owned wiki pages (Hugo-served).
- `raw/` — immutable source documents.
- `WIKI.md` — schema (primary contract for agents).
- `BOOTSTRAP.md` — first-run walkthrough.
- `scripts/` — bookkeeping helpers.
- `justfile` — recipe entry-point.

Full architecture: see `docs/superpowers/specs/2026-04-27-llm-wiki-scaffold-design.md`.

## Common commands

| Recipe | Purpose |
|--------|---------|
| `just ingest <path>` | Process a source from inbox into wiki. |
| `just lint` | Validate wiki integrity. |
| `just lint-fix` | Auto-fix mechanical lint issues. |
| `just serve` | Local Hugo preview at http://localhost:1313. |
| `just build` | Build static site to `public/`. |
| `just search "<q>"` | Search wiki via qmd. |
| `just reindex` | Refresh qmd index. |
| `just encrypt-init` | Set up git-crypt for sensitive content. |
| `just rename <old> <new>` | Rename a slug, updating all wikilinks. |
| `just delete <slug>` | Remove a page, marking broken wikilinks. |
| `just test` | Run BATS test suite. |
| `just help` | Extended help. |

## Dependencies

| Tool | Required? |
|------|-----------|
| bash 4+ | yes |
| git 2.30+ | yes |
| just 1.13+ | yes |
| hugo 0.120+ extended | yes |
| bats-core 1.10+ | yes (for `just test`) |
| qmd (qntx-labs fork) | optional, recommended |
| git-crypt 0.7+ | optional (encryption path) |
| age 1.0+ | optional (encryption path) |

Run `bash scripts/check-deps.sh` to verify.

## Scheduled lint

Pre-built configs ship under `scheduled/`:

- **macOS (launchd):** `cp scheduled/launchd.plist.example ~/Library/LaunchAgents/com.user.awiki.lint.plist`, edit the absolute path, `launchctl load ~/Library/LaunchAgents/com.user.awiki.lint.plist`.
- **Linux (systemd):** copy `scheduled/awiki-lint.service.example` and `scheduled/awiki-lint.timer.example` to `~/.config/systemd/user/`, drop the `.example`, edit the absolute path, then `systemctl --user daemon-reload && systemctl --user enable --now awiki-lint.timer`.
- **GitHub Actions:** `cp scheduled/github-action.yml.example .github/workflows/awiki-ci.yml`. If the repo uses git-crypt, set the `GIT_CRYPT_KEY` repo secret to the base64-encoded key; the workflow unlocks before lint/test.

## Deployment

Three pre-built templates under `deploy/`:

| Target | Files | Setup |
|--------|-------|-------|
| Netlify | `deploy/netlify.toml` | Copy to repo root, link the repo in the Netlify UI. |
| Cloudflare Pages | `deploy/cloudflare-pages.toml` | Settings live in CF UI; this file documents them. |
| GitHub Pages | `deploy/github-pages.yml.example` | `cp deploy/github-pages.yml.example .github/workflows/deploy-pages.yml` |

**Encrypted wikis:** if you ran `just encrypt-init` with git-crypt, set the `GIT_CRYPT_KEY` repo secret to the base64-encoded export of `secrets/.git-crypt-key`. Each template will unlock the tree before building. **If unlock fails or the secret is missing on a wiki with encryption enabled, the build fails closed — no plaintext fallback.**

## Task layer

awiki ships an opt-in **task layer** for tracking actions, projects, and contexts inside the wiki. The methodology is the classic capture-clarify-organize-reflect-engage flow popularized by personal-productivity literature, expressed as wiki-native primitives — captures land in `content/inbox.md`, the agent triages each one to a project / context page, a scanner builds derived agenda views, and a structured weekly review keeps the loop closing.

### Enable

```bash
just task-init
```

This is per-step idempotent. It scaffolds `content/inbox.md`, `content/projects/`, `content/contexts/`, `content/agenda/*`, patches `WIKI.md` between `<!-- BEGIN task-layer -->` markers, sets `AWIKI_TASK_LAYER=on` in `.awiki/config`, and (with consent) installs a task-aware pre-commit hook. If `encrypt-init` was previously run, you'll be prompted to extend git-crypt coverage to `content/inbox.md` and `content/agenda/**`. If you run `encrypt-init` AFTER `task-init`, the coverage is added automatically.

### 6-step manual smoke test

After `just task-init`, verify the layer end-to-end:

1. **Capture**

   ```bash
   just capture "call dentist"
   ```

   Expected: `content/inbox.md` gains a line `- 2026-MM-DD HH:MM call dentist`.

2. **Triage** — open your agent (Claude / Codex / etc.) and ask it to triage the inbox.

   ```
   triage inbox
   ```

   The agent calls `triage_inbox()` and walks each item. For "call dentist", choose `act` outcome, project `_loose`, context `phone`. The agent removes the inbox line and writes `- [ ] call dentist @phone ^<id>` to `content/projects/_loose.md`.

3. **Build agenda**

   ```bash
   just agenda
   ```

   Expected: `content/agenda/next-actions.md` gains a `@phone` section with the new action. Managed-region markers (`<!-- BEGIN agenda:next-actions -->` / `<!-- END agenda:next-actions -->`) bracket the generated content; user notes outside the markers survive.

4. **Verify next-actions render**

   Open `content/agenda/next-actions.md` in Obsidian or any markdown viewer. The action shows under `@phone` with the project tag `[[_loose]]`.

5. **Complete**

   Edit `content/projects/_loose.md`, flip `[ ]` to `[x]`, save, then:

   ```bash
   just agenda
   ```

   Expected: action no longer appears in `next-actions.md`. If the action carried `every:`, a fresh `[ ]` line with the next due-date appears above the completed line.

6. **Review**

   ```bash
   just review
   ```

   Expected: structured report shows `REVIEW|completed-since-last-review|count=1` among the eight `REVIEW|` lines plus the closing `REVIEW-SUMMARY|...`.

If any step fails, see `docs/just-help.txt` for per-recipe expected output.

### Recipes

| Recipe | Purpose |
|--------|---------|
| `just task-init` | Enable / re-bless the task layer (per-step idempotent). |
| `just capture "<text>"` | Append a quick capture to `content/inbox.md`. |
| `just triage` | Walk the inbox interactively (bash fallback to MCP). |
| `just scan` | Rebuild `.awiki/maps/actions.tsv` only (cheap). |
| `just agenda` | Scan + regenerate the five managed-region agenda pages. |
| `just review` | Run `agenda` + `lint` + structured weekly-review report. |
| `just recur` | Re-emit open copies for completed `every:` actions. |

Full per-recipe usage / output / when notes live in `docs/just-help.txt`.

### Methodology note

The five-step flow — capture every open loop without judgment, clarify each capture into a concrete next action or non-action, organize by project and context, reflect on the system on a fixed cadence, engage with the next action that fits your current context — predates awiki by decades. awiki's contribution is making each step a wiki-native primitive: captures are markdown lines, projects and contexts are pages with frontmatter, agenda views are managed-region renderings, and the weekly review is a structured shell report plus an MCP tool. No app, no daemon, no cloud sync; everything is git-committed text the agent can read and write.

## Example

Browse `examples/sample-wiki/` for a tiny reference wiki with full frontmatter, wikilinks, and catalog. The synthesis demo lives at `examples/sample-wiki/content/synthesis/memex-briefing.md`.

### Synthesis smoke test

After running the base smoke test:

1. Tag 3+ sources with the same tag (e.g. `demo`):

   ```bash
   for i in 1 2 3; do echo "demo source $i" > raw/inbox/interactive/demo-$i.md; just ingest "raw/inbox/interactive/demo-$i.md"; done
   ```

2. Scaffold a briefing:

   ```bash
   just synth briefing demo --tag=demo
   ```

3. Fill the generated region (agent task; or use `tests/util/fill-good-body.sh` for a stubbed pass).

4. Finalize and verify:

   ```bash
   just synth-finalize demo-briefing
   just lint
   just build
   ```

5. The page appears under the Synthesis section of `content/catalog.md` after running `just update-catalog` (or via the MCP `update_catalog` tool).

## License

Choose your own per-clone. Template ships without a LICENSE file.
