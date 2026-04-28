#!/usr/bin/env bats

setup() {
  REPO_ROOT="$(git rev-parse --show-toplevel)"
  WORK="$(mktemp -d)"
  cp -r "$REPO_ROOT/scripts" "$WORK/scripts"
  mkdir -p "$WORK/.awiki"
  printf -- "AWIKI_DATA_LAYER=on\nAWIKI_QUERY_LAYER=on\n" > "$WORK/.awiki/config"
  pushd "$WORK" >/dev/null
}
teardown() { popd >/dev/null; rm -rf "$WORK"; }

@test "check-deps emits advisory when duckdb missing + query layer on" {
  # Shadow duckdb to force "not found".
  mkdir -p shimbin
  PATH="$(pwd)/shimbin:$PATH" AWIKI_FAKE_MISSING=duckdb run bash scripts/check-deps.sh
  # Advisory is a warn line; do not fail check-deps for optional dep.
  [[ "$output" == *"data layer is on but duckdb is missing"* ]]
}
