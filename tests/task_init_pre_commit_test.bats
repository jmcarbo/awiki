#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content"
  cd "$WORK"
  git init -q
  git config user.email t@t.t
  git config user.name t
  printf -- "AWIKI_LINT_AFTER_N=5\nAWIKI_STALE_DAYS=90\nAWIKI_LOG_QUERIES=0\n" > .awiki-config-seed
  mkdir -p .awiki
  cp .awiki-config-seed .awiki/config
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > WIKI.md
}

teardown() {
  if [[ -n "${WORK:-}" && -d "$WORK" ]]; then
    rm -rf "$WORK"
  fi
}

@test "task-init installs pre-commit hook when ASSUME_YES" {
  AWIKI_TASK_INIT_ASSUME_YES=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -x .git/hooks/pre-commit ]
  run grep -E '^# task-layer$' .git/hooks/pre-commit
  [ "$status" -eq 0 ]
  run grep -F 'action-scan.sh' .git/hooks/pre-commit
  [ "$status" -eq 0 ]
  run grep -F 'lint.sh --alias-build-only' .git/hooks/pre-commit
  [ "$status" -eq 0 ]
}

@test "task-init pre-commit install is idempotent (re-run does not duplicate)" {
  AWIKI_TASK_INIT_ASSUME_YES=1 bash scripts/task-init.sh
  before="$(wc -l < .git/hooks/pre-commit)"
  AWIKI_TASK_INIT_ASSUME_YES=1 bash scripts/task-init.sh
  after="$(wc -l < .git/hooks/pre-commit)"
  [ "$before" -eq "$after" ]
  # Marker appears exactly once.
  run grep -cE '^# task-layer$' .git/hooks/pre-commit
  [[ "$output" == "1" ]]
}

@test "task-init pre-commit appends to an existing non-task-layer hook" {
  cat > .git/hooks/pre-commit <<'HOOK'
#!/usr/bin/env bash
set -e
just lint
HOOK
  chmod +x .git/hooks/pre-commit

  AWIKI_TASK_INIT_ASSUME_YES=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]

  # Original line preserved.
  run grep -E '^just lint$' .git/hooks/pre-commit
  [ "$status" -eq 0 ]
  # Task-layer addition appended after marker.
  run grep -E '^# task-layer$' .git/hooks/pre-commit
  [ "$status" -eq 0 ]
  run grep -F 'action-scan.sh' .git/hooks/pre-commit
  [ "$status" -eq 0 ]
}

@test "task-init does NOT install hook when ASSUME_NO" {
  AWIKI_TASK_INIT_ASSUME_NO=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  if [[ -f .git/hooks/pre-commit ]]; then
    run grep -E '^# task-layer$' .git/hooks/pre-commit
    [ "$status" -ne 0 ]
  fi
}

@test "task-init pre-commit skipped when not in a git repo" {
  rm -rf .git
  AWIKI_TASK_INIT_ASSUME_YES=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"skip pre-commit"* ]]
}
