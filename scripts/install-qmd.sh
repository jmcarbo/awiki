#!/usr/bin/env bash
set -euo pipefail

# install-qmd.sh — install the qmd local-search binary from source.
#
# Decision recorded in docs/decisions/qmd-install.md:
# - Source: github.com/qntx-labs/qmd (Rust workspace)
# - Build: cargo build --workspace --release
# - Symlink: $HOME/.local/bin/qmd -> <src>/target/release/qmd
# - Status flag: .awiki/qmd-status -> ok | missing
#
# Failure to build is non-fatal at the awiki level: the script writes
# `missing` to the status file and exits non-zero so the agent falls
# back to grep (per spec).

mkdir -p .awiki
STATUS_FILE=".awiki/qmd-status"

if command -v qmd >/dev/null 2>&1; then
  echo "qmd already installed at: $(command -v qmd)"
  echo "ok" > "$STATUS_FILE"
  exit 0
fi

if ! command -v cargo >/dev/null 2>&1; then
  echo "missing" > "$STATUS_FILE"
  echo "cargo not found; install Rust (https://rustup.rs) and re-run." >&2
  echo "agent will use grep fallback" >&2
  exit 1
fi

SRC_DIR="${XDG_DATA_HOME:-$HOME/.local/share}/qmd-src"
mkdir -p "$(dirname "$SRC_DIR")"

if [[ ! -d "$SRC_DIR/.git" ]]; then
  git clone https://github.com/qntx-labs/qmd "$SRC_DIR"
fi
( cd "$SRC_DIR" && git pull --ff-only )

# Build per upstream Makefile target (see docs/decisions/qmd-install.md).
# We omit --all-features intentionally; the release profile alone produces
# the qmd binary, and --all-features would pull optional model weights at
# build time.
if ! ( cd "$SRC_DIR" && cargo build --workspace --release ); then
  echo "missing" > "$STATUS_FILE"
  echo "qmd build failed; agent will use grep fallback" >&2
  exit 1
fi

BIN_SRC="$SRC_DIR/target/release/qmd"
if [[ ! -x "$BIN_SRC" ]]; then
  echo "missing" > "$STATUS_FILE"
  echo "qmd binary not found at $BIN_SRC after build; agent will use grep fallback" >&2
  exit 1
fi

mkdir -p "$HOME/.local/bin"
ln -sf "$BIN_SRC" "$HOME/.local/bin/qmd"

if ! command -v qmd >/dev/null 2>&1; then
  echo "Add ~/.local/bin to PATH:"
  echo '  export PATH="$HOME/.local/bin:$PATH"'
fi

echo "ok" > "$STATUS_FILE"
echo "qmd installed at $HOME/.local/bin/qmd"
