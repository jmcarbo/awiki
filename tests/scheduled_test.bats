#!/usr/bin/env bats

@test "github-action.yml.example is valid YAML" {
  command -v python3 >/dev/null 2>&1 || skip "python3 missing"
  python3 -c "import yaml" 2>/dev/null || skip "pyyaml missing"
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
