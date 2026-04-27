# awiki

A template repository for building personal LLM-maintained wikis. Domain-agnostic. Multi-agent (Claude Code, Codex, OpenCode). Hugo-renderable. Obsidian-friendly. Search via qmd.

## Quick start

```bash
git clone <this-repo> mywiki
cd mywiki
just                          # see available commands
```

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

## Example

Browse `examples/sample-wiki/` for a tiny reference wiki with full frontmatter, wikilinks, and catalog.

## License

Choose your own per-clone. Template ships without a LICENSE file.
