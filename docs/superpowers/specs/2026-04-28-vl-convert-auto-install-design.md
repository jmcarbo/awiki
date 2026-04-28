# vl-convert auto-install — Design

**Status:** Draft
**Date:** 2026-04-28
**Amends:** [2026-04-28-data-layer-vega-lite-design.md](./2026-04-28-data-layer-vega-lite-design.md) — promotes Task 17 of Plan 2 (`check-deps.sh` advisory) to an active install during `just data-init`.

## Goal

`just data-init` shall install the pinned `vl-convert` Rust CLI binary into `.awiki/bin/` so that chart sidecar rendering works out of the box on a fresh wiki clone, without requiring the operator to install a Rust toolchain or fetch the binary by hand. The install is best-effort: a download failure must not block `data-init`; charts simply fall back to the existing advisory path until the next successful run.

## Non-Goals

- No Windows support (advisory only; warn-and-skip).
- No source build / `cargo install` fallback. Operators with a custom toolchain put `vl-convert` on PATH themselves; detection picks it up.
- No automatic version bump. Maintainer edits `template.manifest.toml`; `template-update` propagates.
- No alternative renderers (`vl-convert-python`, `vega-cli` Node bundle). Plan 2's single-renderer assumption stands.
- No `data-init --remove` / uninstall path. Manual `rm .awiki/bin/vl-convert` if needed.

## Decisions (locked via brainstorm)

| # | Question | Decision |
|---|---|---|
| Q1 | Install method | Pre-built binary download from GitHub releases (`curl` + `tar -xJ`) |
| Q2 | Install location | `.awiki/bin/vl-convert` (project-local, gitignored via existing `.awiki/*`) |
| Q3 | Install failure behavior | Warn + continue (`data-init` exits 0; charts skip until next success) |
| Q4 | Already-installed detection | Both: `.awiki/bin/vl-convert --version` matches pin → skip; else `command -v vl-convert` succeeds → skip; else download |
| Q5 | Version pin location | `template.manifest.toml` `[deps.vl-convert]` table with version + per-arch sha256 |
| Q6 | OS/arch coverage | macOS arm64, macOS x86_64, Linux x86_64, Linux arm64 — others warn-and-skip |
| Q7 | Code structure | Separate `scripts/install-vl-convert.sh` invoked from `data-init.sh` step; shared resolver in `scripts/lib/vl-convert-path.sh` |

## Architecture

```
scripts/
  install-vl-convert.sh    # NEW — downloads, sha256-verifies, atomic-installs
  data-init.sh             # Plan 1 — calls install-vl-convert.sh as one idempotent step
  check-deps.sh            # extend (Plan 2 Task 17): use vl_convert_path resolver, sharper warning
  chart.sh                 # Plan 2 Task 14: invoke "$bin" vl2svg via resolver
  lib/
    vl-convert-path.sh     # NEW — single-source resolver: local pinned > PATH > empty

template.manifest.toml     # extend: [deps.vl-convert] table
.awiki/bin/                # gitignored (subset of .awiki/*) — vl-convert + .version stamp
.awiki/tmp/                # gitignored — staging for download/extract; cleaned per run
```

### Flow on `just data-init`

1. `data-init.sh` runs all steps idempotent (mirrors `task-init.sh` pattern).
2. New step "vl-convert install" → `bash scripts/install-vl-convert.sh`.
3. Script reads `template.manifest.toml` for version + per-arch sha256 + URL template.
4. Detect host triple from `uname -s`/`uname -m`; unsupported → `VLCONVERT|skip|reason=unsupported-target` and exit 0.
5. Detection (Q4): if `.awiki/bin/vl-convert --version` matches pin → skip; else if PATH copy exists → skip; else download.
6. Download to `.awiki/tmp/`, sha256-verify, extract, atomic-mv into `.awiki/bin/vl-convert`, write `.awiki/bin/vl-convert.version` stamp.
7. Smoke-check `--version` matches pin; emit `VLCONVERT|ok|version=…|target=…`.
8. Cleanup `.awiki/tmp/` regardless of outcome.

### Flow on `just charts-render` (post-install)

`chart.sh` and `check-deps.sh` resolve the binary via `lib/vl-convert-path.sh`:

```bash
source "$SCRIPT_DIR/lib/vl-convert-path.sh"
bin="$(vl_convert_path)" || { echo "WARN: vl-convert missing — run 'just data-init'"; exit 0; }
"$bin" vl2svg --input "$tmp" --output "$sidecar"
```

Order: project-local pinned copy → host PATH → empty (warn+skip).

## Manifest schema

Append to `template.manifest.toml`:

```toml
[deps.vl-convert]
version = "1.7.0"
release-url-template = "https://github.com/vega/vl-convert/releases/download/v{version}/vl-convert_{version}-{target}.tar.xz"
binary-in-archive = "vl-convert_{version}-{target}/vl-convert"

[deps.vl-convert.sha256]
x86_64-apple-darwin       = "<64-hex>"
aarch64-apple-darwin      = "<64-hex>"
x86_64-unknown-linux-gnu  = "<64-hex>"
aarch64-unknown-linux-gnu = "<64-hex>"
```

- `release-url-template` and `binary-in-archive` use `{version}` and `{target}` placeholders — single-line version bump.
- `sha256` keyed by Rust target triple; install script computes triple locally and looks up.
- Maintainer bump procedure: edit `version`, replace four sha256 values, run `bash scripts/install-vl-convert.sh --force` on each supported arch (or rely on CI matrix), commit.
- Initial sha256 values to be filled by maintainer when this design is implemented (placeholder strings during scaffold).

## `scripts/install-vl-convert.sh` contract

```
usage: install-vl-convert.sh [--force] [--check] [--quiet]
```

| Flag | Meaning |
|---|---|
| `--force` | Skip detection step; redownload even if pinned binary present |
| `--check` | Detection only; rc=0 if pinned binary present, rc=1 if missing. No download. Used by tests + CI gating |
| `--quiet` | Suppress `ok` and `skip` log lines on stderr; `warn` lines still emitted |

`--force` and `--check` are mutually exclusive. Passing both → rc=2 with usage message.

### Steps

All log lines: `VLCONVERT|<phase>|key=val|...` to stderr. No bare prose.

1. **Parse manifest** — extract `version`, URL template, `binary-in-archive`, sha256 map. Use `awk`/`grep` (no TOML library dependency). Parse failure → `VLCONVERT|skip|reason=manifest-missing` rc=0.
2. **Detect target** — map `uname -s`×`uname -m` → triple:
   - Darwin × arm64 → `aarch64-apple-darwin`
   - Darwin × x86_64 → `x86_64-apple-darwin`
   - Linux × x86_64 → `x86_64-unknown-linux-gnu`
   - Linux × aarch64|arm64 → `aarch64-unknown-linux-gnu`
   - Anything else → `VLCONVERT|skip|reason=unsupported-target|target=<host>` rc=0.
3. **Detection** (skipped if `--force`):
   - `.awiki/bin/vl-convert --version` matches `version` → `VLCONVERT|skip|reason=already-pinned|version=…` rc=0.
   - `command -v vl-convert` succeeds → `VLCONVERT|skip|reason=on-path|path=…|version=…` rc=0.
   - Else fall through to download.
4. **Download** — `curl -fL --max-time 60 -o "$tmp_archive"` from resolved URL into `.awiki/tmp/`. Curl rc≠0 → `VLCONVERT|warn|reason=download-failed|url=…|rc=…` rc=0 (warn+continue per Q3); cleanup tmp.
5. **Verify sha256** — `shasum -a 256` (mac) / `sha256sum` (linux). Mismatch → `VLCONVERT|warn|reason=sha-mismatch|expected=…|got=…` rc=0; delete tmp archive.
6. **Extract** — `tar -xJf "$tmp_archive" -C .awiki/tmp/extract/`. Locate `binary-in-archive` path. Tar failure or missing binary → `VLCONVERT|warn|reason=extract-failed` rc=0.
7. **Atomic install** — `chmod +x` extracted binary, `mv` into `.awiki/bin/vl-convert.tmp.$$`, `mv` rename to `.awiki/bin/vl-convert`. Two-step rename ensures concurrent runs don't corrupt the published path. Write `.awiki/bin/vl-convert.version` stamp containing the pinned version string.
8. **Smoke** — `.awiki/bin/vl-convert --version` matches pin → `VLCONVERT|ok|version=…|target=…` rc=0. Mismatch → `VLCONVERT|warn|reason=smoke-failed|expected=…|got=…`, remove the just-installed binary, rc=0.
9. **Cleanup** — always remove `.awiki/tmp/` on exit (trap).

### Exit codes

- `0` — normal (success or warn-and-continue per Q3).
- `1` — `--check` mode, pinned binary missing.
- `2` — bad CLI args.

No other rc values — Q3 mandates resilience.

## `scripts/lib/vl-convert-path.sh`

```bash
#!/usr/bin/env bash
# Echo the resolved vl-convert binary path on stdout, or return 1 if missing.
# Order: project-local pinned copy > host PATH > nothing.
vl_convert_path() {
  local local_bin=".awiki/bin/vl-convert"
  if [[ -x "$local_bin" ]]; then
    printf '%s' "$local_bin"
    return 0
  fi
  if command -v vl-convert >/dev/null 2>&1; then
    command -v vl-convert
    return 0
  fi
  return 1
}
```

Sourced by `chart.sh`, `check-deps.sh`, and any future caller. No global PATH mutation; callers capture the resolved path into a local variable.

## `data-init.sh` integration

Append one step (per-step idempotent — mirrors `task-init.sh`):

```bash
echo "DATA-INIT|step=vl-convert"
bash "$SCRIPT_DIR/install-vl-convert.sh"
```

No flags passed in the default path. Operators wanting force-redownload run `bash scripts/install-vl-convert.sh --force` directly.

## `check-deps.sh` upgrade (Plan 2 Task 17)

Replace direct `command -v vl-convert` with the resolver, and sharpen the message:

```bash
source "$SCRIPT_DIR/lib/vl-convert-path.sh"
if ! vl_convert_path >/dev/null; then
  echo "WARN: data layer is on but vl-convert is missing — run 'just data-init' to install"
  echo "      manual: download from https://github.com/vega/vl-convert/releases"
fi
```

## `chart.sh` invocation (Plan 2 Task 14)

Replace direct `vl-convert vl2svg ...` with:

```bash
source "$SCRIPT_DIR/lib/vl-convert-path.sh"
bin="$(vl_convert_path)" || { echo "RENDER|skip|reason=vl-convert-missing"; return 0; }
if "$bin" vl2svg --input "$resolved_tmp" --output "$sidecar" 2>/tmp/vlc.err; then
  ...
```

No other change to Plan 2's chart pipeline.

## Test plan

### `tests/install_vl_convert_test.bats` (new)

Stub the network with an env-hook `AWIKI_VLCONVERT_FETCH_CMD` that receives the URL and output path and copies a fixture tarball into place. Mirrors the existing `AWIKI_INGEST_CMD` / `AWIKI_LINT_CMD` test hooks in `ingest.sh`. No real network in CI.

| # | Test | Setup | Asserts |
|---|---|---|---|
| 1 | installs binary into `.awiki/bin/` | fixture tarball + matching sha256 | `.awiki/bin/vl-convert` exists, executable, `--version` matches pin |
| 2 | idempotent on re-run | run twice | second run logs `VLCONVERT\|skip\|reason=already-pinned`, mtime unchanged |
| 3 | skips when on PATH | shim `vl-convert` on PATH (returns matching `--version`) | logs `skip\|reason=on-path`, no fetch attempt |
| 4 | warn+continue on sha mismatch | tamper fixture | logs `warn\|reason=sha-mismatch`, rc=0, no binary installed |
| 5 | warn+continue on download fail | hook returns nonzero | logs `warn\|reason=download-failed`, rc=0 |
| 6 | warn+skip on unsupported arch | mock `uname -m` to `riscv64` | logs `skip\|reason=unsupported-target`, rc=0 |
| 7 | `--force` redownloads even when pinned present | install, then `--force` | second run hits download path |
| 8 | `--check` mode | pinned present → rc=0; absent → rc=1 | no download in either case |
| 9 | atomic install — interrupted extract leaves no half-binary | force tar fail mid-step | `.awiki/bin/vl-convert` absent (not partial) |

### `tests/data_init_test.bats` (extend Plan 1 fixture)

One new test: `data-init invokes install-vl-convert step` — assert log line `DATA-INIT|step=vl-convert` emitted, downstream call recorded via env-hook spy.

### `tests/lib_vl_convert_path_test.bats` (new, small)

Three cases: local present → echoes local path; local absent + PATH present → echoes PATH path; both absent → rc=1.

### Fixture

`tests/fixtures/vl-convert/` — minimal `.tar.xz` containing a stub `vl-convert` shell script that prints the pinned version on `--version`. Included sha256 in fixture manifest matches the tarball bytes.

## Log discipline

Every line: `VLCONVERT|<phase>|key=val|...` to stderr (stdout is reserved for `--check`-style structured output if any future flag needs it; today no stdout is emitted). Greppable, mirrors existing `INGEST-OK|`, `WATCHDOG|` style. Phases:

- `start` (only when `--force` or actual download path triggered, to keep happy-path quiet)
- `skip` — detection or unsupported target
- `warn` — recoverable failure, install did not happen
- `ok` — install succeeded

## Risks

- **GitHub rate limits** on unauthenticated `curl` — mitigated by skip-if-pinned (one download per version per clone). Not expected to hit limits at normal usage.
- **sha256 mismatch on legitimate version bumps** if maintainer forgets to update manifest — visible warn (`VLCONVERT|warn|reason=sha-mismatch`); fixture test #4 catches the test path; manual review of manifest bumps still required.
- **Rust target triple drift** (vega renames artifact paths) — `release-url-template` + `binary-in-archive` placeholders absorb a rename without script edits.
- **Concurrent `data-init` runs** racing on the install path — atomic mv via unique tmp suffix `.tmp.$$` avoids collision; last writer wins, both end up with the same correct binary.
- **Tarball compression drift** — vl-convert releases use `.tar.xz`. If they switch to `.zip` or `.tar.gz`, `binary-in-archive` semantics still hold but `tar -xJf` becomes wrong. Trade: keep simple now, surface as a manifest-level `archive-format` field only when needed.

## Out of scope (deferred)

- `data-init --remove` / clean uninstall.
- Mirror/proxy override for environments without GitHub access. Caller can override `release-url-template` per-wiki via `.awiki/config` `AWIKI_VLCONVERT_URL_OVERRIDE` if pressure builds; not in initial scope.
- Auto-bump from upstream releases via a CI cron.
- Windows support.
- Multi-version coexistence in `.awiki/bin/` (single pinned version only).

## Files touched (final tally)

**New:**
- `scripts/install-vl-convert.sh`
- `scripts/lib/vl-convert-path.sh`
- `tests/install_vl_convert_test.bats`
- `tests/lib_vl_convert_path_test.bats`
- `tests/fixtures/vl-convert/` (stub tarball)

**Modified:**
- `template.manifest.toml` — `[deps.vl-convert]` table
- `scripts/data-init.sh` — append one step (Plan 1 owns this file; this design appends)
- `scripts/check-deps.sh` — Plan 2 Task 17 advisory upgrade
- `scripts/chart.sh` — Plan 2 Task 14 invocation site (resolve via helper)
- `tests/data_init_test.bats` — one new test

`.gitignore` — `.awiki/bin/` already covered by existing `.awiki/*` rule. No new entry needed.

## Plan layering

This design is an *amendment* to Plan 2's Task 17 (`check-deps.sh` advisory). The implementation plan should add the work as new tasks **17a** (`scripts/install-vl-convert.sh` + tests + manifest stanza) and **17b** (`lib/vl-convert-path.sh` + chart.sh + check-deps.sh wiring) inserted into the existing Plan 2 task sequence, not a separate plan. Plan 1 owns `data-init.sh`; the one-line "call install-vl-convert.sh" step is added as the final step of Plan 1's data-init scaffolding.
