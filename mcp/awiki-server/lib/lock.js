// flock wrapper for MCP shell-outs. Mirrors scripts/lib/lock.sh's contract:
// mutating tools take exclusive ("x") with 30s timeout; read-only tools take
// shared ("s") with 30s timeout. The MCP boundary calls runLocked() instead
// of execFileSync() directly so every shell-out is automatically guarded.
//
// Implementation: builds argv ["flock", "-<x|s>", "-w", "<sec>",
// ".awiki/lock", <user argv...>] and invokes via execFileSync. We rely
// on flock(1) from util-linux >= 2.36 (Linux ships this; macOS users
// install via Homebrew util-linux bottle, documented in WIKI.md / task-init).
// Note: flock does not accept the GNU "--" terminator between the lock
// path and the wrapped command; the lock path itself acts as the boundary.
//
// Note divergence from scripts/lib/lock.sh: bash exits 7 on lock timeout,
// this module throws LockTimeoutError. Callers translate as needed.

import { execFileSync } from "node:child_process";
import { existsSync } from "node:fs";
import { join } from "node:path";

export class LockTimeoutError extends Error {
  constructor(timeoutSec) {
    super(`flock timed out after ${timeoutSec}s`);
    this.name = "LockTimeoutError";
    this.timeoutSec = timeoutSec;
  }
}

const FLOCK_TIMEOUT_EXIT = 1;

export function runLocked({
  repoRoot,
  mode,
  timeoutSec,
  argv,
  env,
  encoding = "utf8",
}) {
  if (mode !== "x" && mode !== "s") {
    throw new TypeError(
      `runLocked: mode must be "x" or "s", got ${JSON.stringify(mode)}`,
    );
  }
  if (!Array.isArray(argv) || argv.some((a) => typeof a !== "string")) {
    throw new TypeError("runLocked: argv must be an array of strings");
  }
  if (!Number.isInteger(timeoutSec) || timeoutSec < 1) {
    throw new RangeError("runLocked: timeoutSec must be a positive integer");
  }
  const lockPath = join(repoRoot, ".awiki", "lock");
  if (!existsSync(lockPath)) {
    throw new Error(
      `runLocked: lock file ${lockPath} missing — did task-init run?`,
    );
  }

  const flockArgv = [
    `-${mode}`,
    "-w",
    String(timeoutSec),
    lockPath,
    ...argv,
  ];

  try {
    return execFileSync("flock", flockArgv, {
      cwd: repoRoot,
      encoding,
      env: env ?? process.env,
      stdio: ["ignore", "pipe", "pipe"],
    });
  } catch (e) {
    // flock(1) returns the wrapped command's exit code on success, or its
    // own conflict-exit (1 by default; configurable via -E) on timeout.
    // Because we did NOT pass -E, exit 1 means timeout.
    const stderrText =
      e.stderr == null ? "" : e.stderr.toString().trim();
    if (e.status === FLOCK_TIMEOUT_EXIT && stderrText === "") {
      throw new LockTimeoutError(timeoutSec);
    }
    throw e;
  }
}

// Convenience wrappers for the common modes.
export function runLockedExclusive(opts) {
  return runLocked({ ...opts, mode: "x", timeoutSec: opts.timeoutSec ?? 30 });
}
export function runLockedShared(opts) {
  return runLocked({ ...opts, mode: "s", timeoutSec: opts.timeoutSec ?? 30 });
}
