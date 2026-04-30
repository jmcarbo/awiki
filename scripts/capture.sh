#!/usr/bin/env bash
set -euo pipefail

# scripts/capture.sh — append a sanitized capture line to content/inbox.md.
# Usage: capture.sh -- "<text...>"
# Exit codes: 0 OK; 1 usage; 4 hard-rejection (control-char / checkbox-prefix);
#             other non-zero unexpected.

# Slice 2 of the Go ingest port: when bin/awiki is built and the user has
# not opted into the legacy bash path, exec the Go implementation.
# Mirrors scripts/synth.sh:9-26. Set AWIKI_CAPTURE_LEGACY=1 to force the
# original bash path (used by the bats oracle and parity smoke tests
# until cleanup in slice 10).
AWIKI_CAPTURE_SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
AWIKI_CAPTURE_REPO_ROOT="$(cd "$AWIKI_CAPTURE_SCRIPT_DIR/.." && pwd)"
AWIKI_CAPTURE_GO_BIN="$AWIKI_CAPTURE_REPO_ROOT/bin/awiki"

if [[ "${AWIKI_CAPTURE_LEGACY:-0}" != "1" && -x "$AWIKI_CAPTURE_GO_BIN" ]]; then
  cd "$AWIKI_CAPTURE_REPO_ROOT" || exit 1
  exec "$AWIKI_CAPTURE_GO_BIN" capture "$@"
fi

INBOX="${AWIKI_INBOX_FILE:-content/inbox.md}"

usage() {
  cat <<USAGE
usage: capture.sh -- "<text>"
  Appends a sanitized capture line to $INBOX.
  See capture --help / docs/just-help.txt for the sanitization table.
USAGE
}

# Parse args: require leading "--" so callers can pass any text safely.
if [[ $# -eq 0 ]]; then usage >&2; exit 1; fi
if [[ "$1" == "--help" || "$1" == "-h" ]]; then usage; exit 0; fi
if [[ "$1" != "--" ]]; then
  echo "ERROR|missing '--' separator (use: capture.sh -- \"<text>\")" >&2
  exit 1
fi
shift

if [[ $# -eq 0 ]]; then
  echo "ERROR|empty: no text after '--'" >&2
  exit 4
fi

raw="$*"
if [[ -z "$raw" ]]; then
  echo "ERROR|empty: text is blank after argument join" >&2
  exit 4
fi

# AWIKI_CAPTURE_PRESANITIZED=1 — caller already ran sanitization (e.g.,
# the MCP `capture` handler in phase 18b). Skip the sanitization pass
# but KEEP the hard-rejection scan as a defense-in-depth check.
# When set, do not emit `SANITIZATION-APPLIED|...` (already reported by
# the upstream caller).
presanitized="${AWIKI_CAPTURE_PRESANITIZED:-0}"

# === Hard-rejections ===

# Newline / CR / NUL / non-tab control chars.
# Bash-native byte scan (portable: no dependency on GNU vs BSD grep).
# We forbid bytes 0x00-0x08, 0x0a-0x1f, 0x7f. Tab (0x09) is allowed (and
# normalized to space below). NUL bytes are stripped by the shell on
# argument parsing on most platforms, but we keep an explicit check for
# defensive depth.
_ctrl_chars=$'\x01\x02\x03\x04\x05\x06\x07\x08\x0a\x0b\x0c\x0d\x0e\x0f\x10\x11\x12\x13\x14\x15\x16\x17\x18\x19\x1a\x1b\x1c\x1d\x1e\x1f\x7f'
_i=0
while (( _i < ${#_ctrl_chars} )); do
  _c="${_ctrl_chars:$_i:1}"
  if [[ "$raw" == *"$_c"* ]]; then
    echo "ERROR|control: text contains a newline / CR / NUL / control char (only tab is allowed and converts to space)" >&2
    exit 4
  fi
  _i=$(( _i + 1 ))
done
unset _ctrl_chars _i _c

# Checkbox-prefix-at-start: "[ ]" / "[/]" / "[?]" / "[>]" / "[x]" / "[-]" at column 1.
if [[ "$raw" =~ ^\[[\ /\?\>x-]\] ]]; then
  echo "ERROR|checkbox: line begins with a checkbox marker; captures are not actions" >&2
  exit 4
fi

# === Replace-category sanitizations ===

text="$raw"
applied=()

# Tab → space (always; not counted as a sanitization since it's a benign normalization).
text="${text//$'\t'/ }"

if [[ "$presanitized" == "1" ]]; then
  # Caller (e.g. the MCP capture handler) already sanitized.
  # Skip neutralization; do NOT populate $applied.
  :
else

# Length > 2000.
if [[ ${#text} -gt 2000 ]]; then
  text="${text:0:2000}…"
  applied+=("length-truncated")
fi

# Wikilink open/close.
if [[ "$text" == *"[["* || "$text" == *"]]"* ]]; then
  text="${text//\[\[/[ [}"
  text="${text//\]\]/] ]}"
  applied+=("wikilink-neutralized")
fi

# HTML comment markers.
if [[ "$text" == *"<!--"* || "$text" == *"-->"* ]]; then
  text="${text//<!--/< !--}"
  text="${text//-->/--  >}"
  applied+=("comment-neutralized")
fi

# Block-ID-shaped tokens: ^[a-z0-9]{4,} → prefix with backslash.
# We use bash extended-regex via a loop to be conservative.
if [[ "$text" =~ \^[a-z0-9]{4,} ]]; then
  text="$(printf -- '%s' "$text" | sed -E 's/(\^)([a-z0-9]{4,})/\\\1\2/g')"
  applied+=("block-id-escaped")
fi

fi  # end !presanitized branch

# === Emit ===

mkdir -p "$(dirname "$INBOX")"
if [[ ! -f "$INBOX" ]]; then
  {
    printf -- '---\n'
    printf -- 'title: "Inbox"\n'
    printf -- 'type: inbox\n'
    printf -- 'draft: true\n'
    printf -- '---\n'
  } > "$INBOX"
fi

ts="$(date '+%Y-%m-%d %H:%M')"
line="- $ts $text"
printf -- '%s\n' "$line" >> "$INBOX"

if (( ${#applied[@]} > 0 )); then
  IFS=, ; tags="${applied[*]}" ; unset IFS
  echo "SANITIZATION-APPLIED|$tags" >&2
  # Print a minimal diff (raw vs final) so the user can audit.
  echo "  raw  : $raw" >&2
  echo "  final: $text" >&2
fi

# Best-effort log (non-fatal if log-append.sh missing in the test sandbox).
if [[ -x scripts/log-append.sh ]]; then
  bash scripts/log-append.sh -- capture "$(printf -- '%s' "$text" | head -c 80)" >/dev/null 2>&1 || true
fi

echo "OK|appended|$line"
