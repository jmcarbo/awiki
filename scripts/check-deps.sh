#!/usr/bin/env bash
set -uo pipefail

OS="$(uname -s)"
ERRORS=0

check() {
  local cmd="$1" min_version="$2" install_macos="$3" install_linux="$4"
  if [[ -n "${AWIKI_FAKE_MISSING:-}" && "$cmd" == "$AWIKI_FAKE_MISSING" ]]; then
    echo "MISSING|$cmd|min=$min_version" >&2
    case "$OS" in
      Darwin) echo "  install: $install_macos" >&2 ;;
      Linux)  echo "  install: $install_linux" >&2 ;;
    esac
    ERRORS=$((ERRORS + 1))
    return
  fi
  if ! command -v "$cmd" >/dev/null 2>&1; then
    echo "MISSING|$cmd|min=$min_version" >&2
    case "$OS" in
      Darwin) echo "  install: $install_macos" >&2 ;;
      Linux)  echo "  install: $install_linux" >&2 ;;
    esac
    ERRORS=$((ERRORS + 1))
    return
  fi
  echo "OK|$cmd"
}

check_bash_version() {
  local major="${BASH_VERSION%%.*}"
  if [[ "$major" -lt 4 ]]; then
    echo "MISSING|bash|need=4+|have=$BASH_VERSION" >&2
    case "$OS" in
      Darwin) echo "  install: brew install bash" >&2 ;;
      Linux)  echo "  install: bash 4+ should be default; check distro packages" >&2 ;;
    esac
    ERRORS=$((ERRORS + 1))
    return
  fi
  echo "OK|bash|$BASH_VERSION"
}

check_bash_version
check git "2.30" "brew install git" "apt install git"
check just "1.13" "brew install just" "cargo install just"
check hugo "0.120" "brew install hugo" "see https://gohugo.io/installation/"
check bats "1.10" "brew install bats-core" "apt install bats"

# Optional tools — warn but do not fail.
for opt in qmd git-crypt age entr fswatch pdftotext; do
  if command -v "$opt" >/dev/null 2>&1; then
    echo "OK|$opt (optional)"
  else
    echo "OPTIONAL-MISSING|$opt"
  fi
done

if [[ "$ERRORS" -gt 0 ]]; then
  echo "DEPS-SUMMARY|errors=$ERRORS" >&2
  exit 1
fi
echo "DEPS-SUMMARY|errors=0"
