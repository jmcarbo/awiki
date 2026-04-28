# Template Updates

This wiki was bootstrapped from the [awiki template](https://github.com/jmcarbo/awiki). When the template ships new scripts, MCP server fixes, deploy templates, schema changes, or new bootstrap steps, you can pull them in with `just template-update`.

## Quick start

```bash
# Read pending status (no mutation):
just template-status

# Preview what an update would do (default = dry-run):
just template-update

# Apply onto a dedicated review branch:
just template-update --apply

# Review the branch:
git diff main

# Merge:
git switch main && git merge --no-ff awiki-template-update/<sha>
```

## Trust model

`just template-update` executes scripts shipped from the upstream template (`migrations/*.sh`) and stages instructions for your agent (`migrations/*.prompt.md`). Both are **trusted code** — review before running.

- `--source` defaults to the URL recorded at bootstrap (`.awiki/template.json.repo`). Changing it requires `--accept-source-change` and a confirmation prompt.
- `--print-migrations` shows full migration script bodies in the dry-run plan.
- `--verify-signature` requires the upstream tag/commit be GPG-signed (you provide the trust roots in `~/.gnupg/`).
- Optional: set `require_signature=true` in `.awiki/config` to make signature checks mandatory.

If you don't trust the source, don't run the update.

## Recovery from a bad update

### Soft revert (keep new pin, retry without one migration)

```bash
git revert <merge-commit>
just template-update --skip-migration <id> --apply
```

### Hard re-pin (downgrade)

```bash
git revert -m 1 <merge-commit>
just template-update --re-pin <previous-commit>
git add .awiki/template.json && git commit -m "chore(template): re-pin to <prev-version>"
```

## Multi-machine wikis

Same wiki cloned to multiple machines: pull updates on one machine, push, then `git pull` on the others. The first `just template-update` on a fresh machine auto-recovers `.awiki/template-cache/<pin>/` from upstream — no manual seeding required. If the upstream is unreachable, run `just template-retrofit` to re-seed.

## Pending LLM migrations

Some updates ship as agent prompts under `.awiki/pending-prompts/`. After merging, your agent should surface each prompt to you before acting (see `WIKI.md` §14). Lint warns on prompts older than 14 days.

## Common flags

| flag                              | use                                                                    |
|-----------------------------------|------------------------------------------------------------------------|
| `--apply`                         | Execute (default = dry-run plan)                                       |
| `--continue`                      | Resume after conflicts / migration failure                             |
| `--abort`                         | Discard in-progress update branch                                      |
| `--non-interactive`               | Auto-resolve prompts to safe defaults; CI mode                         |
| `--print-migrations`              | Show full migration bodies in plan                                     |
| `--accept-source-change`          | Confirm `--source` differs from pinned `repo`                          |
| `--accept-attribute-changes`      | Confirm `.gitattributes` filter changes                                |
| `--accept-manual-commits`         | Bypass `--continue` HEAD check after manual edits on update branch     |
| `--skip-migration <id>`           | Skip a specific failing migration                                      |
| `--rerun-bootstrap-step <id>`     | Re-run a single (often dangerous) bootstrap step                       |
| `--re-pin <commit>`               | Roll back the recorded pin (rebuilds cache from upstream)              |
| `--gc`                            | Prune orphaned cache dirs                                              |
| `--verify-signature`              | Require signed tag/commit                                              |

## Updating from a pre-v1 repo

If your wiki was bootstrapped before awiki v1.0.0, run:

```bash
just template-retrofit
```

This walks you through naming your original awiki commit/version and seeds `.awiki/template.json` with content_hashes for completed bootstrap steps. From then on, regular `just template-update` works.
