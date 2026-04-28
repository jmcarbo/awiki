#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="${AWIKI_REPO_ROOT:-$(git rev-parse --show-toplevel)}"
cd "$REPO_ROOT"

# Load config
if [[ -f .awiki/config ]]; then
  # shellcheck disable=SC1091
  source .awiki/config
fi
THRESHOLD="${AWIKI_LINT_AFTER_N:-5}"

# Optional --agent <cli> flag: after bookkeeping, invoke the given agent
# CLI in --print mode with a one-shot ingestion prompt. Only used for
# `batch` queue (interactive/checkpoint require human-in-the-loop). If
# AWIKI_AGENT is set in .awiki/config, that's the default.
AGENT_CLI="${AWIKI_AGENT:-}"
ARGS=()
while [[ $# -gt 0 ]]; do
  case "$1" in
    --agent) AGENT_CLI="$2"; shift 2 ;;
    --agent=*) AGENT_CLI="${1#--agent=}"; shift ;;
    --) shift; ARGS+=("$@"); break ;;
    *) ARGS+=("$1"); shift ;;
  esac
done
set -- "${ARGS[@]}"

if [[ $# -lt 1 ]]; then
  echo "usage: ingest.sh [--agent <cli>] <path-under-raw/inbox/>" >&2
  exit 1
fi

SRC="$1"
# Normalize to relative path from repo root.
if [[ "$SRC" = /* ]]; then
  SRC="${SRC#"$REPO_ROOT/"}"
fi

if [[ ! "$SRC" =~ ^raw/inbox/(interactive|batch|checkpoint)/.+ ]]; then
  echo "ERROR: path must be under raw/inbox/{interactive,batch,checkpoint}/" >&2
  exit 2
fi
MODE="${BASH_REMATCH[1]}"

if [[ ! -f "$SRC" ]]; then
  echo "ERROR: source not found: $SRC" >&2
  exit 1
fi

# Strip leading raw/inbox/<mode>/ to get relative subtree
REL="${SRC#raw/inbox/"$MODE"/}"
DEST="raw/processed/$MODE/$REL"
DEST_DIR="$(dirname "$DEST")"

if [[ -f "$DEST" ]]; then
  echo "ERROR: already processed: $DEST" >&2
  exit 3
fi

mkdir -p "$DEST_DIR"
mv "$SRC" "$DEST"

bash "$SCRIPT_DIR/log-append.sh" ingest "$(basename "$SRC") mode=$MODE"

# Increment counter
COUNTER_FILE=".awiki/ingest-count"
mkdir -p .awiki
COUNT=0
[[ -f "$COUNTER_FILE" ]] && COUNT="$(cat "$COUNTER_FILE")"
COUNT=$((COUNT + 1))
echo "$COUNT" > "$COUNTER_FILE"

echo "INGEST-OK|src=$SRC|dest=$DEST|mode=$MODE|count=$COUNT"

# Auto-lint
LINT_RC=0
if [[ "$COUNT" -ge "$THRESHOLD" ]]; then
  echo "AUTO-LINT|threshold=$THRESHOLD|count=$COUNT"
  if ! bash "$SCRIPT_DIR/lint.sh"; then
    LINT_RC=4
  fi
  echo "0" > "$COUNTER_FILE"
fi

# Auto-reindex (best-effort)
QMD_RC=0
if [[ "${AWIKI_QMD_STATUS:-}" != "missing" ]] && command -v qmd >/dev/null 2>&1; then
  if ! bash "$SCRIPT_DIR/qmd-index.sh" 2>/dev/null; then
    QMD_RC=5
  fi
fi

# Emit a paste-ready agent prompt. The script intentionally does NOT
# perform the LLM-driven part of ingestion (read source, write summary,
# update entity pages, update catalog) — that is the agent's job per
# WIKI.md §4.1 steps 3-9. When this script is called from outside an
# agent session, the human/automation can pipe this line into their
# agent of choice.
PROMPT="Process the source at $DEST per WIKI.md §4.1 ingest workflow steps 3-9 (mode=$MODE). Read it, write content/sources/<slug>.md, update affected entity/concept/topic pages, update content/catalog.md, update section indexes if section purpose changed. Run \`just lint\` afterward."
echo "AGENT-PROMPT|$PROMPT"

# Optional: shell out to an agent CLI for batch ingestion. Skip for
# interactive/checkpoint queues — those expect human-in-the-loop and
# would block on a non-interactive --print invocation.
AGENT_RC=0
if [[ -n "$AGENT_CLI" && "$MODE" == "batch" ]]; then
  if command -v "$AGENT_CLI" >/dev/null 2>&1; then
    echo "AGENT-INVOKE|cli=$AGENT_CLI|mode=$MODE"
    case "$AGENT_CLI" in
      claude) "$AGENT_CLI" --print "$PROMPT" || AGENT_RC=6 ;;
      codex|opencode|gemini) "$AGENT_CLI" -p "$PROMPT" || AGENT_RC=6 ;;
      *) "$AGENT_CLI" "$PROMPT" || AGENT_RC=6 ;;
    esac
  else
    echo "AGENT-SKIP|reason=cli-not-found|cli=$AGENT_CLI" >&2
    AGENT_RC=6
  fi
elif [[ -n "$AGENT_CLI" ]]; then
  echo "AGENT-SKIP|reason=mode-needs-human|mode=$MODE|cli=$AGENT_CLI" >&2
fi

if [[ "$LINT_RC" -ne 0 ]]; then exit 4; fi
if [[ "$QMD_RC" -ne 0 ]]; then exit 5; fi
if [[ "$AGENT_RC" -ne 0 ]]; then exit 6; fi
exit 0
