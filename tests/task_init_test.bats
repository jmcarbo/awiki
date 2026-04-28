#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/content" "$WORK/.awiki"
  printf -- "AWIKI_LINT_AFTER_N=5\nAWIKI_STALE_DAYS=90\nAWIKI_LOG_QUERIES=0\n" > "$WORK/.awiki/config"
  printf -- "---\ntitle: \"WIKI\"\n---\n\n# WIKI\n" > "$WORK/WIKI.md"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "task-init creates .awiki/last-review = today" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f .awiki/last-review ]
  today="$(date '+%Y-%m-%d')"
  run cat .awiki/last-review
  [[ "$output" == "$today" ]]
}

@test "task-init creates .awiki/task-count = 0" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f .awiki/task-count ]
  run cat .awiki/task-count
  [[ "$output" == "0" ]]
}

@test "task-init appends AWIKI_AGENDA_AFTER_N=5 and AWIKI_TASK_LAYER=on if absent" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_AGENDA_AFTER_N=5$' .awiki/config
  [ "$status" -eq 0 ]
  run grep -E '^AWIKI_TASK_LAYER=on$' .awiki/config
  [ "$status" -eq 0 ]
}

@test "task-init preserves existing user-set config values" {
  echo 'AWIKI_AGENDA_AFTER_N=10' >> .awiki/config
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  # Must not duplicate, must not overwrite.
  run grep -c '^AWIKI_AGENDA_AFTER_N=' .awiki/config
  [[ "$output" == "1" ]]
  run grep '^AWIKI_AGENDA_AFTER_N=' .awiki/config
  [[ "$output" == "AWIKI_AGENDA_AFTER_N=10" ]]
}

@test "task-init re-run is idempotent: state files unchanged" {
  bash scripts/task-init.sh
  cp .awiki/last-review /tmp/awiki-lr-1
  cp .awiki/task-count /tmp/awiki-tc-1
  bash scripts/task-init.sh
  diff /tmp/awiki-lr-1 .awiki/last-review
  diff /tmp/awiki-tc-1 .awiki/task-count
  rm -f /tmp/awiki-lr-1 /tmp/awiki-tc-1
}

@test "task-init creates content/inbox.md with type: inbox frontmatter" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/inbox.md ]
  run grep '^type: inbox$' content/inbox.md
  [ "$status" -eq 0 ]
  run grep '^draft: true$' content/inbox.md
  [ "$status" -eq 0 ]
}

@test "task-init creates section indexes" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/projects/_index.md ]
  [ -f content/contexts/_index.md ]
  [ -f content/agenda/_index.md ]
}

@test "task-init creates five agenda pages with per-region marker pair" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  for view in next-actions today waiting someday stuck-projects; do
    [ -f "content/agenda/$view.md" ]
    run grep "^<!-- BEGIN agenda:$view -->\$" "content/agenda/$view.md"
    [ "$status" -eq 0 ]
    run grep "^<!-- END agenda:$view -->\$" "content/agenda/$view.md"
    [ "$status" -eq 0 ]
    run grep '^type: agenda$' "content/agenda/$view.md"
    [ "$status" -eq 0 ]
  done
}

@test "task-init creates agenda/review-log.md without managed region" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [ -f content/agenda/review-log.md ]
  run grep '^type: agenda$' content/agenda/review-log.md
  [ "$status" -eq 0 ]
  run grep '^<!-- BEGIN agenda:' content/agenda/review-log.md
  [ "$status" -ne 0 ]
}

@test "task-init creates three starter context pages" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  for ctx in phone errands computer; do
    [ -f "content/contexts/$ctx.md" ]
    run grep '^type: context$' "content/contexts/$ctx.md"
    [ "$status" -eq 0 ]
    run grep -F "@$ctx" "content/contexts/$ctx.md"
    [ "$status" -eq 0 ]
  done
}

@test "task-init does not overwrite an existing custom page" {
  mkdir -p content/contexts
  printf -- "---\ntype: context\ncustom: yes\n---\n\nMy own thing.\n" > content/contexts/phone.md
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep '^custom: yes$' content/contexts/phone.md
  [ "$status" -eq 0 ]
}

@test "task-init appends task-layer block to WIKI.md" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep '^<!-- BEGIN task-layer -->$' WIKI.md
  [ "$status" -eq 0 ]
  run grep '^<!-- END task-layer -->$' WIKI.md
  [ "$status" -eq 0 ]
}

@test "task-init WIKI.md block contains expected sections" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -F 'Task Layer (opt-in)' WIKI.md
  [ "$status" -eq 0 ]
  run grep -F '## Task Layer' WIKI.md
  [ "$status" -eq 0 ]
  run grep -F 'T15' WIKI.md
  [ "$status" -eq 0 ]
}

@test "task-init does not double-append if marker already present" {
  bash scripts/task-init.sh
  bash scripts/task-init.sh
  run grep -c '^<!-- BEGIN task-layer -->$' WIKI.md
  [[ "$output" == "1" ]]
}

@test "task-init prints WARN if marker present but body modified" {
  bash scripts/task-init.sh
  # Add stray text inside the block (simulate user customization).
  awk '/^<!-- BEGIN task-layer -->$/ {print; print "MY CUSTOM TEXT"; next} {print}' WIKI.md > WIKI.md.tmp
  mv WIKI.md.tmp WIKI.md
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"WARN"* ]] || [[ "$output" == *"customized"* || "$output" == *"customised"* ]]
  # Block is still there, MY CUSTOM TEXT preserved.
  run grep -F 'MY CUSTOM TEXT' WIKI.md
  [ "$status" -eq 0 ]
}

@test "task-init skips encryption step when no .gitattributes" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"skip encryption"* ]] || [[ "$output" == *"no git-crypt"* ]]
}

@test "task-init appends git-crypt patterns when ASSUME_YES" {
  cat > .gitattributes <<EOF2
content/private/** filter=git-crypt diff=git-crypt
EOF2
  AWIKI_TASK_INIT_ASSUME_YES=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  run grep -F 'content/inbox.md filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
  run grep -F 'content/agenda/** filter=git-crypt diff=git-crypt' .gitattributes
  [ "$status" -eq 0 ]
}

@test "task-init does NOT append patterns when ASSUME_NO" {
  cat > .gitattributes <<EOF2
content/private/** filter=git-crypt diff=git-crypt
EOF2
  AWIKI_TASK_INIT_ASSUME_NO=1 run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  script_output="$output"
  run grep -F 'content/inbox.md filter=git-crypt' .gitattributes
  [ "$status" -ne 0 ]
  [[ "$script_output" == *"WARN"* ]] || [[ "$script_output" == *"declined"* ]]
}

@test "task-init does not double-append patterns on rerun" {
  cat > .gitattributes <<EOF2
content/private/** filter=git-crypt diff=git-crypt
EOF2
  AWIKI_TASK_INIT_ASSUME_YES=1 bash scripts/task-init.sh
  AWIKI_TASK_INIT_ASSUME_YES=1 bash scripts/task-init.sh
  run grep -c -F 'content/inbox.md filter=git-crypt diff=git-crypt' .gitattributes
  [[ "$output" == "1" ]]
}

@test "task-init prints smoke instructions on success" {
  run bash scripts/task-init.sh
  [ "$status" -eq 0 ]
  [[ "$output" == *"just capture"* ]]
  [[ "$output" == *"just scan"* ]]
  [[ "$output" == *"just review"* ]]
}

@test "task-init reports first-run vs noop on rerun (when log-append present)" {
  # Provide a stub log-append and an empty log file so we can grep.
  cat > scripts/log-append.sh <<'STUB'
#!/usr/bin/env bash
shift  # drop --
echo "LOG|$*" >> "${AWIKI_LOG_FILE:-content/log.md}"
STUB
  chmod +x scripts/log-append.sh
  printf -- "---\ntype: log\n---\n" > content/log.md
  AWIKI_LOG_FILE="content/log.md" bash scripts/task-init.sh
  AWIKI_LOG_FILE="content/log.md" bash scripts/task-init.sh
  run grep 'task-init enabled' content/log.md
  [ "$status" -eq 0 ]
  run grep 'task-init noop' content/log.md
  [ "$status" -eq 0 ]
}
