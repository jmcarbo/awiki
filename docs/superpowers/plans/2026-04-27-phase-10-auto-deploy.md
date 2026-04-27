# awiki Plan — Phase 10: Auto-Deploy Templates

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 3, 5
**Previous:** [Phase 09](./2026-04-27-phase-09-scheduled-lint.md)
**Next:** [Phase 11](./2026-04-27-phase-11-multimodal-ingest.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** Netlify, Cloudflare Pages, GitHub Pages templates + README per-target instructions.

**Branch:** `phase-10-deploy`

## Task 10.1: Branch + Netlify

```bash
git checkout -b phase-10-deploy
```

- [ ] **Step 1: Write `deploy/netlify.toml`**

```toml
[build]
  command = "scripts/deploy-build.sh"
  publish = "public"

[build.environment]
  HUGO_VERSION = "0.120.4"

[[plugins]]
  package = "netlify-plugin-cache"
```

- [ ] **Step 2: Write `deploy/cloudflare-pages.toml`**

```toml
# Cloudflare Pages — paste into project UI:
# Build command:    scripts/deploy-build.sh
# Build output:     public
# Environment vars:
#   HUGO_VERSION = 0.120.4
#   GIT_CRYPT_KEY = <base64> (only if wiki uses git-crypt; sets a CI-only secret)
```

- [ ] **Step 3: Write `deploy/github-pages.yml.example`**

```yaml
name: deploy-pages
on:
  push:
    branches: [main]
permissions:
  contents: read
  pages: write
  id-token: write
jobs:
  build:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive
      - uses: peaceiris/actions-hugo@v2
        with:
          hugo-version: '0.120.4'
          extended: true
      - name: Install just
        run: curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin
      - name: Optional unlock for encrypted wikis
        if: ${{ secrets.GIT_CRYPT_KEY != '' }}
        env:
          GIT_CRYPT_KEY: ${{ secrets.GIT_CRYPT_KEY }}
        run: |
          sudo apt-get update && sudo apt-get install -y git-crypt
          echo "$GIT_CRYPT_KEY" | base64 -d > /tmp/git-crypt-key
          git-crypt unlock /tmp/git-crypt-key
          rm /tmp/git-crypt-key
      - run: bash scripts/deploy-build.sh
      - uses: actions/upload-pages-artifact@v3
        with:
          path: public
  deploy:
    needs: build
    runs-on: ubuntu-latest
    environment:
      name: github-pages
      url: ${{ steps.deployment.outputs.page_url }}
    steps:
      - uses: actions/deploy-pages@v4
        id: deployment
```

- [ ] **Step 4: Write `scripts/deploy-build.sh`**

```bash
cat > scripts/deploy-build.sh <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

# Refuse to publish if encryption is configured but tree is locked.
if [[ -d .git/git-crypt ]]; then
  if ! git-crypt status -e 2>/dev/null | head -1 | grep -q 'encrypted:'; then
    echo "ERROR: git-crypt initialized but tree appears locked. Set GIT_CRYPT_KEY secret and unlock before deploy." >&2
    exit 1
  fi
fi

bash scripts/build.sh --full
EOF
chmod +x scripts/deploy-build.sh
```

- [ ] **Step 5: README deploy section**

Append exact content to README:

```markdown
## Deployment

Three pre-built templates under `deploy/`:

| Target | Files | Setup |
|--------|-------|-------|
| Netlify | `deploy/netlify.toml` | Copy to repo root, link the repo in the Netlify UI. |
| Cloudflare Pages | `deploy/cloudflare-pages.toml` | Settings live in CF UI; this file documents them. |
| GitHub Pages | `deploy/github-pages.yml.example` | `cp deploy/github-pages.yml.example .github/workflows/deploy-pages.yml` |

**Encrypted wikis:** if you ran `just encrypt-init` with git-crypt, set the `GIT_CRYPT_KEY` repo secret to the base64-encoded export of `secrets/.git-crypt-key`. Each template will unlock the tree before building. **If unlock fails or the secret is missing on a wiki with encryption enabled, the build fails closed — no plaintext fallback.**
```

- [ ] **Step 6: Add validation tests**

```bash
cat > tests/deploy_test.bats <<'EOF'
#!/usr/bin/env bats

@test "netlify.toml parses as TOML" {
  command -v python3 >/dev/null 2>&1 || skip
  python3 -c "import tomllib; tomllib.load(open('deploy/netlify.toml','rb'))" 2>/dev/null \
    || python3 -c "import tomli; tomli.load(open('deploy/netlify.toml','rb'))"
}

@test "github-pages.yml.example parses as YAML" {
  python3 -c "import yaml; yaml.safe_load(open('deploy/github-pages.yml.example'))"
}

@test "deploy-build.sh refuses locked git-crypt tree" {
  WORK="$(mktemp -d)"
  cp scripts/deploy-build.sh "$WORK/"
  cd "$WORK"
  mkdir -p .git/git-crypt
  run bash deploy-build.sh
  [ "$status" -ne 0 ]
  cd - >/dev/null
  rm -rf "$WORK"
}
EOF
```

- [ ] **Step 7: Commit + merge**

```bash
bats tests/deploy_test.bats
git add deploy scripts/deploy-build.sh README.md tests/deploy_test.bats
git commit -m "feat: add auto-deploy templates with git-crypt unlock + tests"
git checkout main
git merge --no-ff phase-10-deploy -m "feat: complete phase 10 deploy templates"
git branch -d phase-10-deploy
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 11](./2026-04-27-phase-11-multimodal-ingest.md).
