#!/usr/bin/env bash
set -euo pipefail

# scripts/task-init.sh — per-step idempotent enabler for the awiki task layer.
# Each step detects its own existing state and skips work that is already done,
# so partial-failure re-runs are safe. There is no binary "already enabled"
# gate.

REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel 2>/dev/null || pwd)}"
cd "$REPO_ROOT"

CONFIG_FILE=".awiki/config"
LAST_REVIEW=".awiki/last-review"
TASK_COUNT=".awiki/task-count"
WIKI_MD="WIKI.md"
TEMPLATE="scripts/templates/wiki-task-layer.md"

note() { echo "TASK-INIT|$*"; }
warn() { echo "TASK-INIT|WARN|$*" >&2; }

step_state_files() {
  mkdir -p .awiki
  if [[ ! -f "$LAST_REVIEW" ]]; then
    date '+%Y-%m-%d' > "$LAST_REVIEW"
    note "created $LAST_REVIEW"
  else
    note "skip $LAST_REVIEW (exists)"
  fi
  if [[ ! -f "$TASK_COUNT" ]]; then
    echo 0 > "$TASK_COUNT"
    note "created $TASK_COUNT"
  else
    note "skip $TASK_COUNT (exists)"
  fi
}

step_config() {
  if [[ ! -f "$CONFIG_FILE" ]]; then
    mkdir -p .awiki
    : > "$CONFIG_FILE"
  fi
  if ! grep -q '^AWIKI_AGENDA_AFTER_N=' "$CONFIG_FILE"; then
    printf -- 'AWIKI_AGENDA_AFTER_N=5\n' >> "$CONFIG_FILE"
    note "appended AWIKI_AGENDA_AFTER_N=5"
  else
    note "skip AWIKI_AGENDA_AFTER_N (already set)"
  fi
  if ! grep -q '^AWIKI_TASK_LAYER=' "$CONFIG_FILE"; then
    printf -- 'AWIKI_TASK_LAYER=on\n' >> "$CONFIG_FILE"
    note "appended AWIKI_TASK_LAYER=on"
  else
    note "skip AWIKI_TASK_LAYER (already set)"
  fi
}

step_pages() {
  local today; today="$(date '+%Y-%m-%d')"

  # 1. content/inbox.md
  if [[ ! -f content/inbox.md ]]; then
    mkdir -p content
    cat > content/inbox.md <<INBOX
---
title: "Inbox"
type: inbox
draft: true
---

INBOX
    note "created content/inbox.md"
  else
    note "skip content/inbox.md (exists)"
  fi

  # 2. Section indexes.
  mkdir -p content/projects content/contexts content/agenda
  for sec in projects contexts agenda; do
    local idx="content/$sec/_index.md"
    if [[ ! -f "$idx" ]]; then
      cat > "$idx" <<INDEX
---
title: "${sec^}"
type: section
draft: false
---

{{< page-list >}}
INDEX
      note "created $idx"
    else
      note "skip $idx (exists)"
    fi
  done

  # 3. Five agenda views with empty managed-region pair.
  local view
  for view in next-actions today waiting someday stuck-projects; do
    local f="content/agenda/$view.md"
    if [[ ! -f "$f" ]]; then
      cat > "$f" <<AGENDA
---
title: "Agenda — ${view//-/ }"
type: agenda
last_updated: $today
draft: false
---

<!-- BEGIN managed-region -->
<!-- END managed-region -->
AGENDA
      note "created $f"
    else
      note "skip $f (exists)"
    fi
  done

  # 4. agenda/review-log.md (append-only history; no managed region).
  if [[ ! -f content/agenda/review-log.md ]]; then
    cat > content/agenda/review-log.md <<RLOG
---
title: "Review Log"
type: agenda
last_updated: $today
draft: false
---

# Review Log

Append-only history of weekly reviews. New entries are added by
\`scripts/review-status.sh mark_done\` (phase 19).
RLOG
    note "created content/agenda/review-log.md"
  else
    note "skip content/agenda/review-log.md (exists)"
  fi

  # 5. Three starter context pages.
  local ctx
  for ctx in phone errands computer; do
    local f="content/contexts/$ctx.md"
    if [[ ! -f "$f" ]]; then
      cat > "$f" <<CTX
---
title: "@$ctx"
date: $today
last_updated: $today
type: context
aliases: ['@$ctx']
tools: []
draft: false
---

Actions tagged \`@$ctx\` reference this page.
CTX
      note "created $f"
    else
      note "skip $f (exists)"
    fi
  done
}

step_wiki_md() {
  if [[ ! -f "$WIKI_MD" ]]; then
    warn "WIKI.md not found at repo root; skipping task-layer block patch"
    return 0
  fi
  if [[ ! -f "$TEMPLATE" ]]; then
    warn "template missing: $TEMPLATE; skipping"
    return 0
  fi

  if grep -q '^<!-- BEGIN task-layer -->$' "$WIKI_MD"; then
    # Marker present. Compute reference template content between markers
    # and current file content between markers. If they differ, warn and skip.
    local tmpl_body cur_body
    tmpl_body="$(awk '/^<!-- BEGIN task-layer -->$/{f=1} f{print} /^<!-- END task-layer -->$/{f=0}' "$TEMPLATE")"
    cur_body="$(awk '/^<!-- BEGIN task-layer -->$/{f=1} f{print} /^<!-- END task-layer -->$/{f=0}' "$WIKI_MD")"
    if [[ "$tmpl_body" != "$cur_body" ]]; then
      warn "WIKI.md task-layer block has been customised; skipping (manual reconciliation required)"
    else
      note "skip WIKI.md (block already up to date)"
    fi
    return 0
  fi

  # Append a blank line then the block.
  {
    printf -- '\n'
    cat "$TEMPLATE"
  } >> "$WIKI_MD"
  note "appended task-layer block to WIKI.md"
}

main() {
  note "start"
  step_pages
  step_wiki_md
  step_config
  step_state_files
  note "done"
}

main "$@"
