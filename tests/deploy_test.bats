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
