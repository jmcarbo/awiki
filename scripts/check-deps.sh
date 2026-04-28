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
check flock "n/a" "brew install util-linux  # then add the flock binary to PATH (see brew info util-linux)" "apt install util-linux  # provides /usr/bin/flock"
check python3 "3.8" "brew install python" "apt install python3"

# Optional tools — warn but do not fail.
for opt in qmd git-crypt age entr fswatch inotifywait pdftotext; do
  if command -v "$opt" >/dev/null 2>&1; then
    echo "OK|$opt (optional)"
  else
    echo "OPTIONAL-MISSING|$opt"
  fi
done

# Optional: 'expect' is needed only by the BATS interactive triage driver;
# the triage.sh --interactive walker itself runs without it.
if ! command -v expect >/dev/null 2>&1; then
  echo "OPTIONAL-MISSING|expect"
  echo "  warn: 'expect' not installed; interactive triage tests will be skipped." >&2
  echo "    macOS: brew install expect" >&2
  echo "    debian: apt-get install expect" >&2
else
  echo "OK|expect (optional)"
fi

# Optional Python module: pyyaml (used by tests/scheduled_test.bats to validate
# scheduled/github-action.yml.example). Warn but do not fail.
if command -v python3 >/dev/null 2>&1 && python3 -c "import yaml" >/dev/null 2>&1; then
  echo "OK|pyyaml (optional)"
else
  echo "OPTIONAL-MISSING|pyyaml"
  echo "  install: pip3 install pyyaml" >&2
fi

# Data-layer dependency advisory.
if [[ -f .awiki/config ]] && grep -q '^AWIKI_DATA_LAYER=on' .awiki/config; then
  if ! command -v python3 >/dev/null 2>&1; then
    echo "WARN: data layer is on but python3 is missing — dataset-rows.py won't run"
  fi
fi

if [[ "$ERRORS" -gt 0 ]]; then
  echo "DEPS-SUMMARY|errors=$ERRORS" >&2
  exit 1
fi
echo "DEPS-SUMMARY|errors=0"
