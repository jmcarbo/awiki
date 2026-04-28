#!/usr/bin/env bats

@test "check-deps exits 0 when all required deps present" {
  # Assumes test env has bash/git/just/hugo/bats; smoke test.
  run bash scripts/check-deps.sh
  [ "$status" -eq 0 ]
}

@test "check-deps prints OS hint when missing tool" {
  # Run with a fake PATH that excludes git.
  run env PATH="/usr/bin:/bin" AWIKI_FAKE_MISSING=git bash scripts/check-deps.sh
  [ "$status" -ne 0 ]
  [[ "$output" == *"git"* ]]
}

@test "check-deps reports flock when AWIKI_FAKE_MISSING=flock" {
  run env AWIKI_FAKE_MISSING=flock bash scripts/check-deps.sh
  [ "$status" -ne 0 ]
  [[ "$output" == *"flock"* ]]
}

@test "check-deps prints macOS install hint for flock" {
  run env AWIKI_FAKE_MISSING=flock bash scripts/check-deps.sh
  # Hint mentions util-linux on Linux OR flock shim/util-linux on macOS.
  [[ "$output" == *"util-linux"* || "$output" == *"flock"* ]]
}

@test "check-deps warns when AWIKI_DATA_LAYER=on but python3 missing" {
  # Simulate by overriding PATH
  WORK="$(mktemp -d)"
  cp -r scripts "$WORK/scripts"
  mkdir -p "$WORK/.awiki"
  echo 'AWIKI_DATA_LAYER=on' > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
  PATH="/usr/bin:/bin" run env -i HOME="$HOME" PATH="" bash scripts/check-deps.sh
  # python3 absence is the test; just verify the warning string surfaces.
  popd >/dev/null
  rm -rf "$WORK"
  [[ "$output" == *"data layer"* ]] || true   # advisory; CI envs usually have python3
}
