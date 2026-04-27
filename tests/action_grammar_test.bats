#!/usr/bin/env bats

setup() {
  WORK="$(mktemp -d)"
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  pushd "$WORK" >/dev/null
}

teardown() {
  popd >/dev/null
  rm -rf "$WORK"
}

@test "library defines all advertised symbols" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && [[ -n "$AWIKI_ACTION_LINE_RE" ]] \
      && [[ -n "$AWIKI_TAIL_KEY_RE" ]] \
      && [[ -n "$AWIKI_BLOCK_ID_RE" ]] \
      && declare -p AWIKI_STATUS_MARKERS >/dev/null \
      && type -t awiki_grammar_parse_action \
      && type -t awiki_grammar_split_tail'
  [ "$status" -eq 0 ]
  [[ "$output" == *"function"* ]]
}

@test "parses minimal action line" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_parse_action "- [ ] call dentist ^a01" \
      && echo "$AWIKI_AG_STATUS|$AWIKI_AG_TEXT|$AWIKI_AG_CONTEXT|$AWIKI_AG_TAIL|$AWIKI_AG_ID"'
  [ "$status" -eq 0 ]
  [[ "$output" == " |call dentist||"*"|^a01" ]]
}

@test "parses action line with context, tail, and id" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_parse_action "- [ ] call dentist about crown @phone due:2026-05-01 ^a01" \
      && echo "$AWIKI_AG_STATUS|$AWIKI_AG_CONTEXT|$AWIKI_AG_TAIL|$AWIKI_AG_ID"'
  [ "$status" -eq 0 ]
  [[ "$output" == " |@phone|due:2026-05-01|^a01" ]]
}

@test "parses each valid status marker" {
  for s in " " "/" "?" ">" "x" "-"; do
    run bash -c "source scripts/lib/action-grammar.sh && awiki_grammar_parse_action '- [$s] x ^id1' && echo \"\$AWIKI_AG_STATUS\""
    [ "$status" -eq 0 ]
    [[ "$output" == "$s" ]]
  done
}

@test "rejects line without checkbox" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_parse_action "- not an action"'
  [ "$status" -ne 0 ]
}

@test "rejects line with bogus status marker" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_parse_action "- [Z] x"'
  [ "$status" -ne 0 ]
}

@test "rejects double-checkbox line" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_parse_action "- [ ] [ ] doubled"'
  [ "$status" -ne 0 ]
}

@test "block-id regex accepts plain id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^a01" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == "OK" ]]
}

@test "block-id regex accepts recurrence chain id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^a01~2" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK'
  [ "$status" -eq 0 ]
  [[ "$output" == "OK" ]]
}

@test "block-id regex rejects too-short id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^xx" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK || echo NO'
  [ "$status" -eq 0 ]
  [[ "$output" == "NO" ]]
}

@test "block-id regex rejects too-long id" {
  run bash -c 'source scripts/lib/action-grammar.sh && [[ "^abcdefghijklmnopq" =~ $AWIKI_BLOCK_ID_RE ]] && echo OK || echo NO'
  [ "$status" -eq 0 ]
  [[ "$output" == "NO" ]]
}

@test "split_tail emits one key=value per line for known keys" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_grammar_split_tail "due:2026-05-01 every:1w priority:2"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"due=2026-05-01"* ]]
  [[ "$output" == *"every=1w"* ]]
  [[ "$output" == *"priority=2"* ]]
}

@test "split_tail flags bad key via AWIKI_AG_REJECT_REASON" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_split_tail "due:2026-05-01 bogus:xyz" \
      && echo "REASON=$AWIKI_AG_REJECT_REASON"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"due=2026-05-01"* ]]
  [[ "$output" == *"REASON=bad-key"* ]]
}

@test "split_tail flags bad date via AWIKI_AG_REJECT_REASON" {
  run bash -c 'source scripts/lib/action-grammar.sh \
      && awiki_grammar_split_tail "due:2026-13-40" \
      && echo "REASON=$AWIKI_AG_REJECT_REASON"'
  [ "$status" -eq 0 ]
  [[ "$output" == *"REASON=bad-date"* ]]
}

@test "split_tail accepts every: named cadences and Nd/Nw/Nm" {
  for v in 1d 7d 1w 2w 1m 3m daily weekly monthly; do
    run bash -c "source scripts/lib/action-grammar.sh \
        && AWIKI_AG_REJECT_REASON='' \
        && awiki_grammar_split_tail 'every:$v' \
        && echo \"REASON=\$AWIKI_AG_REJECT_REASON\""
    [ "$status" -eq 0 ]
    [[ "$output" == *"every=$v"* ]]
    [[ "$output" != *"REASON=bad-"* ]]
  done
}

@test "awiki_date_add_days adds positive days" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_days 2026-04-27 7'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-05-04" ]
}

@test "awiki_date_add_days handles month boundary" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_days 2026-01-30 5'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-02-04" ]
}

@test "awiki_date_add_days handles year boundary" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_days 2026-12-30 7'
  [ "$status" -eq 0 ]
  [ "$output" = "2027-01-06" ]
}

@test "awiki_date_add_days handles negative delta" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_days 2026-04-27 -7'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-04-20" ]
}

@test "awiki_date_add_days rejects bad date" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_days 2026-02-30 1'
  [ "$status" -ne 0 ]
}

@test "awiki_date_add_months clamps to last day of target month" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_months 2026-01-31 1'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-02-28" ]
}

@test "awiki_date_add_months handles leap year" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_months 2024-01-31 1'
  [ "$status" -eq 0 ]
  [ "$output" = "2024-02-29" ]
}

@test "awiki_date_add_months crosses year boundary" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_months 2026-11-15 3'
  [ "$status" -eq 0 ]
  [ "$output" = "2027-02-15" ]
}

@test "awiki_date_add_months rejects bad input" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_date_add_months not-a-date 1'
  [ "$status" -ne 0 ]
}

@test "awiki_recur_compute_due: weekly = +7d" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2026-05-04 weekly'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-05-11" ]
}

@test "awiki_recur_compute_due: 1w = +7d" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2026-05-04 1w'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-05-11" ]
}

@test "awiki_recur_compute_due: daily = +1d" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2026-04-27 daily'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-04-28" ]
}

@test "awiki_recur_compute_due: 3d = +3d" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2026-04-27 3d'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-04-30" ]
}

@test "awiki_recur_compute_due: monthly clamps last-day" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2026-01-31 monthly'
  [ "$status" -eq 0 ]
  [ "$output" = "2026-02-28" ]
}

@test "awiki_recur_compute_due: 1m clamps last-day in leap year" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2024-01-31 1m'
  [ "$status" -eq 0 ]
  [ "$output" = "2024-02-29" ]
}

@test "awiki_recur_compute_due: rejects bad token" {
  run bash -c 'source scripts/lib/action-grammar.sh && awiki_recur_compute_due 2026-04-27 banana'
  [ "$status" -ne 0 ]
}
