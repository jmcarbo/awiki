# Migrations

Migration scripts run during `just template-update --apply` to evolve user content alongside template changes. Two kinds:

- **Mechanical** (`NNNN-<slug>.sh`) — bash, idempotent, runs with stripped env.
- **LLM-assisted** (`NNNN-<slug>.prompt.md`) — staged into `.awiki/pending-prompts/`, agent picks up next session.

Numbering: zero-padded 4-digit, monotonic, never renumbered. Gaps OK.

Schema upgrades use `schema-NN-to-MM.sh` (separate sequence; see spec §Schema upgrades).

## Mechanical migration template

```bash
#!/usr/bin/env bash
# migration: 0007-rewrite-sources-blocks
# requires: agent=false
# touches: content/synthesis/**/*.md WIKI.md
# idempotent: yes
set -euo pipefail

if [[ "${1:-}" == "--dry-run" ]]; then
  echo "would rewrite ## Sources blocks"
  exit 0
fi

# Real run.
find content/synthesis -name '*.md' -exec sed -i.bak 's/^## Sources$/## Sources\n/' {} \;
find content/synthesis -name '*.bak' -delete
```

### Required header keys

| key            | meaning                                                                  |
|----------------|--------------------------------------------------------------------------|
| `migration`    | Slug matching filename (without `.sh`).                                  |
| `requires`     | `agent=true` or `agent=false`. Mechanical = `false`.                     |
| `touches`      | Space-separated globs. Runner enforces post-run `git status` matches.    |
| `idempotent`   | `yes` / `no`. Should always be `yes` for v1.                             |

### Banned `touches:` patterns

- `secrets/`, `themes/`, `.awiki/`, `.git/` — runner halts pre-run.
- Schema upgrades bypass `.awiki/` ban via `--schema-upgrade` flag.

### Stripped env

Runner clears `GH_TOKEN`, `GITHUB_TOKEN`, etc. Migrations cannot rely on auth tokens.

## LLM prompt template

```markdown
---
id: 0008-reformat-tags
requires: [agent]
scope_glob: "content/**/*.md"
risk: medium
---

Rewrite each `tags: [a, b]` line to `tags:\n  - a\n  - b`. Run `just lint` after to verify frontmatter parses.
```

### Required frontmatter keys

| key          | meaning                                                                  |
|--------------|--------------------------------------------------------------------------|
| `id`         | Slug (full filename minus `.prompt.md`).                                 |
| `requires`   | `[agent]`.                                                               |
| `scope_glob` | Path pattern. MUST NOT match `secrets/`, `.awiki/`, `.git/`, `themes/`.  |
| `risk`       | `low | medium | high`. `high` auto-declines under `--non-interactive`.   |

### Authoring guidelines

1. **Be specific.** Resolved file list is appended to the staged prompt; agent operates on that list, not your glob.
2. **No `secrets/`.** Runner rejects.
3. **Idempotent intent.** If applied twice, second pass should no-op or be safe.
4. **State acceptance criteria.** Most prompts end with "After completing, run `just lint`."

## Numbering rules

- Strictly linear. Authors MUST NOT ship a migration that depends on a skippable predecessor.
- If `0008` requires `0007`'s effect, document it in `0008`'s header and have it self-detect missing precondition (exit nonzero with clear error).
- Skipped migrations cannot be retroactively un-skipped automatically — user must edit `.awiki/template.json`.
