# qmd install decision

**Date:** 2026-04-27
**Status:** Adopted (Phase 4)
**Context:** awiki Phase 4 — qmd Integration

## Spike summary

Cloned `https://github.com/qntx-labs/qmd` to `/tmp/qmd-spike` and confirmed the
upstream toolchain + build pipeline.

### Toolchain

- **Language:** Rust (edition 2024)
- **Toolchain pin:** `rust-toolchain.toml` -> `channel = "stable"`
- **Workspace members:** `qmd` (library) and `qmd-cli` (binary, name `qmd`)
- **macOS verification:** `cargo 1.94.1`, `rustc 1.94.1` (Homebrew) — release
  build completed in ~60s on Apple Silicon, single optimized binary at
  `target/release/qmd`.

### Upstream-documented install paths

The README offers three install routes, in declared priority order:

1. **Prebuilt release binary** via `curl -fsSL https://sh.qntx.fun/qmd | sh`
   (script is vendored in the repo as `install.sh`; downloads
   `qmd-<ver>-<target>.tar.gz` from GitHub releases into `~/.local/bin`).
2. **`cargo install qmd-cli`** from crates.io.
3. **From source:** `make build` (which runs
   `cargo build --workspace --release --all-features`).

### awiki choice: build from source via cargo

`scripts/install-qmd.sh` clones the qntx-labs fork to
`${XDG_DATA_HOME:-$HOME/.local/share}/qmd-src` and runs:

```bash
cargo build --workspace --release
```

Then symlinks `target/release/qmd` -> `$HOME/.local/bin/qmd`.

Rationale:

- The release-binary route hits `sh.qntx.fun`, an upstream-controlled
  redirect. Awiki should not depend on third-party DNS for first-run install.
- `cargo install qmd-cli` pulls from crates.io which currently publishes
  upstream `qntx/qmd`, not the `qntx-labs/qmd` fork the spec pins.
- Building from source against the pinned fork keeps awiki decoupled from
  upstream release cadence and matches the spec wording ("qntx-labs fork").
- `--all-features` is omitted from the awiki invocation; the release profile
  alone is sufficient and avoids pulling optional fastembed model weights at
  build time.

### Install target

- **Source clone:** `${XDG_DATA_HOME:-$HOME/.local/share}/qmd-src`
- **Binary symlink:** `$HOME/.local/bin/qmd`
- **Status flag:** `.awiki/qmd-status` -> `ok` | `missing`

If `cargo` is unavailable or the build fails, the script writes
`missing` to `.awiki/qmd-status`, logs a warning, and exits non-zero. Per spec
this is non-fatal at the awiki level: the agent falls back to grep.

### Linux verification

Not exercised in this spike (macOS-only Phase 4 environment), but the
toolchain is portable: any Linux box with stable Rust 1.85+ and a system
sqlite/gcc will produce the same binary path
(`target/release/qmd`).

## CLI surface — divergence from the spec

The `qntx-labs/qmd` v0.5.0 CLI does **not** expose the subcommands referenced
in the awiki spec/plan:

| Spec/plan command       | Actual CLI                                |
| ----------------------- | ----------------------------------------- |
| `qmd index content/`    | Not present. Use `qmd collection add` + `qmd update`. |
| `qmd mcp --root ...`    | Not present. The `qmd-mcp` crate is listed in upstream README but is not yet shipped in this fork's workspace. |

Implications for Phase 4 scripts:

- **`scripts/qmd-index.sh`** uses `qmd update` (re-index all registered
  collections). On first run, if no `awiki` collection is registered, it
  registers `content/` as collection `awiki` then updates. The
  `QMD-INDEX|ok` / `QMD-INDEX|skip` contract is preserved.
- **`scripts/wire-qmd-mcp.sh`** still wires the agent harness with the
  command + args from the spec. When upstream ships `qmd-mcp`, the wired
  config will start working with no awiki change. Until then, the wiring is
  inert (the harness will simply fail to start the server). The script logs a
  warning to make this visible.

These divergences are recorded here rather than rewriting the spec, since
they are upstream-driven and likely to converge as `qntx-labs/qmd` matures.
