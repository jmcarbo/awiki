# awiki Plan — Phase 9: Scheduled Lint Configs

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Spec:** [`2026-04-27-llm-wiki-scaffold-design.md`](../specs/2026-04-27-llm-wiki-scaffold-design.md)
**Master:** [`2026-04-27-awiki-master-plan.md`](./2026-04-27-awiki-master-plan.md)
**Depends on:** Phase 2
**Previous:** [Phase 08](./2026-04-27-phase-08-mcp-server.md)
**Next:** [Phase 10](./2026-04-27-phase-10-auto-deploy.md)

**Tech stack:** bash 4+, just 1.13+, hugo 0.120+ extended, hugo-book theme, qmd (qntx-labs fork), git-crypt 0.7+, age 1.0+, bats-core 1.10+, python3 3.8+, Node 20+ (phase 8 only).

**Conventions:**
- Scripts: `#!/usr/bin/env bash`, `set -euo pipefail`.
- Commit after every task. Conventional Commits.
- TDD where applicable: write failing test → run → implement → run → commit.
- Branch per phase. Merge to main only after `just test && just lint` are clean.

---

**Deliverable:** Pre-built configs for launchd (macOS), systemd (Linux), GitHub Actions (CI).

**Branch:** `phase-9-scheduled`
**Depends on:** Phase 2.

## Task 9.1: Branch + launchd plist

- [ ] **Step 1: Branch**

```bash
git checkout -b phase-9-scheduled
```

- [ ] **Step 2: Write `scheduled/launchd.plist.example`**

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
  <key>Label</key>
  <string>com.user.awiki.lint</string>
  <key>ProgramArguments</key>
  <array>
    <string>/bin/bash</string>
    <string>-lc</string>
    <string>cd /ABSOLUTE/PATH/TO/WIKI && /opt/homebrew/bin/just lint</string>
  </array>
  <key>StartCalendarInterval</key>
  <dict>
    <key>Hour</key><integer>9</integer>
    <key>Minute</key><integer>0</integer>
  </dict>
  <key>StandardOutPath</key>
  <string>/tmp/awiki-lint.out</string>
  <key>StandardErrorPath</key>
  <string>/tmp/awiki-lint.err</string>
</dict>
</plist>
```

- [ ] **Step 3: Write `scheduled/systemd.timer.example` and `.service.example`**

```ini
# scheduled/awiki-lint.service.example
[Unit]
Description=awiki lint pass

[Service]
Type=oneshot
WorkingDirectory=/ABSOLUTE/PATH/TO/WIKI
ExecStart=/usr/bin/just lint
```

```ini
# scheduled/awiki-lint.timer.example
[Unit]
Description=Run awiki lint daily

[Timer]
OnCalendar=daily
Persistent=true

[Install]
WantedBy=timers.target
```

- [ ] **Step 4: Write `scheduled/github-action.yml.example`**

```yaml
name: awiki-ci
on:
  push:
  pull_request:
  schedule:
    - cron: '0 9 * * *'
jobs:
  lint:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          submodules: recursive
      - name: Install just
        run: |
          curl --proto '=https' --tlsv1.2 -sSf https://just.systems/install.sh | bash -s -- --to /usr/local/bin
      - name: Install hugo extended
        run: |
          curl -fsSL https://github.com/gohugoio/hugo/releases/download/v0.120.4/hugo_extended_0.120.4_linux-amd64.tar.gz | tar xz -C /tmp
          sudo mv /tmp/hugo /usr/local/bin/hugo
      - name: Install bats-core (pinned)
        run: |
          curl -fsSL https://github.com/bats-core/bats-core/archive/refs/tags/v1.10.0.tar.gz | tar xz -C /tmp
          sudo /tmp/bats-core-1.10.0/install.sh /usr/local
      - name: Optional unlock for encrypted wikis
        if: ${{ env.GIT_CRYPT_KEY != '' }}
        env:
          GIT_CRYPT_KEY: ${{ secrets.GIT_CRYPT_KEY }}
        run: |
          sudo apt-get update && sudo apt-get install -y git-crypt
          echo "$GIT_CRYPT_KEY" | base64 -d > /tmp/git-crypt-key
          git-crypt unlock /tmp/git-crypt-key
          rm /tmp/git-crypt-key
      - run: just check-deps
      - run: just lint
      - run: just test
```

- [ ] **Step 5: Document in README + WIKI.md**

Append exact content to `README.md`:

```markdown
## Scheduled lint

Pre-built configs ship under `scheduled/`:

- **macOS (launchd):** `cp scheduled/launchd.plist.example ~/Library/LaunchAgents/com.user.awiki.lint.plist`, edit the absolute path, `launchctl load ~/Library/LaunchAgents/com.user.awiki.lint.plist`.
- **Linux (systemd):** copy `scheduled/awiki-lint.service.example` and `scheduled/awiki-lint.timer.example` to `~/.config/systemd/user/`, drop the `.example`, edit the absolute path, then `systemctl --user daemon-reload && systemctl --user enable --now awiki-lint.timer`.
- **GitHub Actions:** `cp scheduled/github-action.yml.example .github/workflows/awiki-ci.yml`. If the repo uses git-crypt, set the `GIT_CRYPT_KEY` repo secret to the base64-encoded key; the workflow unlocks before lint/test.
```

Append exact content to `WIKI.md` section 4 (workflows), as a new sub-section "Scheduled lint":

```markdown
### 4.5 Scheduled lint

If the user has installed one of the configs in `scheduled/`, lint runs automatically on a cadence. Lint output is captured to logs (`/tmp/awiki-lint.{out,err}` for launchd; `journalctl --user -u awiki-lint` for systemd; the Actions run log for CI). Agent should treat scheduled lint failures as the next-session priority.
```

- [ ] **Step 6: Add a yaml-lint smoke test for the GH Actions example**

```bash
cat > tests/scheduled_test.bats <<'EOF'
#!/usr/bin/env bats

@test "github-action.yml.example is valid YAML" {
  command -v python3 >/dev/null 2>&1 || skip "python3 missing"
  python3 -c "import yaml; yaml.safe_load(open('scheduled/github-action.yml.example'))"
}

@test "launchd plist parses as XML" {
  command -v plutil >/dev/null 2>&1 || skip "plutil missing"
  run plutil -lint scheduled/launchd.plist.example
  [ "$status" -eq 0 ]
}

@test "systemd timer has [Install] section" {
  run grep -F '[Install]' scheduled/awiki-lint.timer.example
  [ "$status" -eq 0 ]
}
EOF
```

(Add `pyyaml` to BOOTSTRAP step 0 dep check or print install hint; the test skips gracefully if missing.)

- [ ] **Step 7: Commit**

```bash
git add scheduled README.md WIKI.md tests/scheduled_test.bats
git commit -m "feat: scheduled lint configs (launchd/systemd/gh-actions) + smoke tests"
```

## Task 9.2: Phase 9 merge

```bash
git checkout main
git merge --no-ff phase-9-scheduled -m "feat: complete phase 9 scheduled lint configs"
git branch -d phase-9-scheduled
```

---

---

## Phase complete

Return to [master plan](./2026-04-27-awiki-master-plan.md) or proceed to [Phase 10](./2026-04-27-phase-10-auto-deploy.md).
