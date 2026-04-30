# Bootstrap

Run this once when the user first opens an agent in a fresh clone of the awiki template. Walk through each step in order. Do NOT skip steps; later steps depend on earlier ones.

### Step 0: Submodules + dependency check
<!-- bootstrap-step: dep-check -->

First, initialize git submodules (Hugo themes ship as submodules):

```bash
git submodule update --init --recursive
```

Then run `bash scripts/check-deps.sh`. If it exits non-zero, surface the printed install hints and halt. Re-run after the user installs missing tools.

### Step 1: Domain
<!-- bootstrap-step: domain -->

Ask the user one of:

- A. Personal — health, goals, journals, self-improvement.
- B. Research — deep topic over weeks/months (papers, articles).
- C. Reading — book, series, course; characters, themes, plot.
- D. Business / team — Slack, meetings, project docs.
- E. Other — let user describe.

Record the answer.

### Step 2: Wiki name + purpose
<!-- bootstrap-step: wiki-name -->

Ask: "What's the wiki called?" — kebab-case identifier.
Ask: "One-line purpose?" — single sentence, ≤120 chars.

### Step 3: Encryption decision
<!-- bootstrap-step: privacy -->

Ask:

- A. None — public wiki, nothing sensitive.
- B. git-crypt — symmetric, transparent. Recommended for personal/health/journal.
- C. age — asymmetric, manual.

If A: continue.
If B: run `bash scripts/encrypt-init.sh`.
If C: run `bash scripts/encrypt-init.sh --age`.

After encrypt-init, run `git status` and verify expected encrypted-vs-cleartext patterns before any commit.

### Step 4: Track ingested sources?
<!-- bootstrap-step: track-processed -->

Ask: "Track ingested sources in git? (y/N)". Default N.

If y: edit `.gitignore` and remove these two lines:
```
raw/processed/*
!raw/processed/.gitkeep
```
The `_originals/` and `private/` exceptions remain ignored (still privacy-protected).

Note to user: tracking sources may include copyrighted material. History-rewrite cost is non-trivial if revoked.

### Step 5: Hugo theme
<!-- bootstrap-step: theme -->

Ask: "Hugo theme? (default: hugo-book)"

The default `hugo-book` submodule is already present from the template. Skip the add step if the user accepts the default. If the user picks an alternative:

```bash
git submodule deinit -f themes/hugo-book
git rm -f themes/hugo-book
rm -rf .git/modules/themes/hugo-book
git submodule add <theme-url> themes/<theme-name>
```

After switching, also rewrite `theme = "hugo-book"` in `hugo.toml` to the chosen theme name.

### Step 6: Publish log?
<!-- bootstrap-step: publish-log -->

Ask: "Publish log to rendered site? (y/N)". Default N.

- N → leave `content/log.md` frontmatter `draft: true`.
- y → set `content/log.md` frontmatter `draft: false`.

### Step 7: Patch identity
<!-- bootstrap-step: patch-identity -->

Edit:
- `WIKI.md` Identity section: fill `wiki_name`, `domain`, `purpose` from steps 1-2.
- `hugo.toml`: set `title`, `baseURL` (ask if not known yet — placeholder fine).
- `content/_index.md`: set `title`, write a one-paragraph wiki landing.
- `content/log.md`: frontmatter from step 6.

### Step 8: Install qmd
<!-- bootstrap-step: install-qmd -->

Run `just install-qmd`. Treat failure as non-fatal:
- On success: confirm `.awiki/qmd-status=ok`, run `just reindex`.
- On failure: `.awiki/qmd-status=missing`, print warning, continue. Agent uses `grep -r` fallback per WIKI.md.

If `~/.local/bin` is not on PATH, print:

```
Add this to ~/.zshrc or ~/.bashrc:
export PATH="$HOME/.local/bin:$PATH"
```

### Step 9a: Wire qmd MCP server (optional)
<!-- bootstrap-step: wire-qmd-mcp -->

Ask: "Wire qmd MCP server into your agent harness? (y/N)" — only if `.awiki/qmd-status=ok`.

If y: run `bash scripts/wire-qmd-mcp.sh`. Print verification instructions.

### Step 9b: Wire awiki wiki-ops MCP server (optional)
<!-- bootstrap-step: wire-awiki-mcp -->

Ask: "Wire awiki wiki-ops MCP server (ingest/lint/query/update_catalog)? (y/N)".

If y, install Node deps and wire:

```bash
( cd mcp/awiki-server && npm install --silent )
bash scripts/wire-awiki-mcp.sh
```

The wire script registers the awiki server in:
- Claude Code: project-level `./.mcp.json` (created if absent).
- Codex: `./.codex/config.toml` `[mcp.servers.awiki]` block.

Verify registration by listing tools in the next agent session. The MCP tools are: `ingest_source`, `lint`, `query_wiki`, `update_catalog`.

### Step 10: Initial log entry
<!-- bootstrap-step: log-init -->

```bash
bash scripts/log-append.sh init "wiki '$WIKI_NAME' initialized for domain '$DOMAIN'"
```

### Step 11: Initial commit
<!-- bootstrap-step: stage-commit -->

Stage:
```bash
git add -A
git status
```

Show user the file list. Ask: "Stage all and commit? (y/N)". On y:

```bash
git commit -m "chore: initialize wiki '$WIKI_NAME'"
```

### Step 12: Seed template provenance
<!-- bootstrap-step: template-init -->

Run `awiki template init --repo <upstream-url> --ref main --version <version> --commit <commit>` to write `.awiki/template.json`, snapshot the template tree to `.awiki/template-cache/<commit>/`, and record `bootstrap_steps_done[]` with content_hash for all completed steps. This wires the wiki up for `just template-update` going forward.

### Step 13: Smoke test prompt
<!-- bootstrap-step: smoke-test -->

Tell the user:

> Bootstrap complete. Try the smoke test in README.md to verify everything works:
> 1. Drop a sample source into `raw/inbox/interactive/sample.md`.
> 2. `just ingest raw/inbox/interactive/sample.md`.
> 3. `just lint`.
> 4. `just serve`.
> 5. `just search "test"` (if qmd installed).
