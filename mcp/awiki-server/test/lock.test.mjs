import { runLocked, runLockedExclusive, runLockedShared, LockTimeoutError } from "../lib/lock.js";
import { mkdtempSync, writeFileSync, mkdirSync, rmSync, existsSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { spawn } from "node:child_process";
import { strict as assert } from "node:assert";

// macOS Homebrew util-linux ships flock at /opt/homebrew/opt/util-linux/bin/flock.
// Linux tends to have it on PATH already. Prepend the brew prefix so both
// platforms resolve `flock` the same way.
const HOMEBREW_FLOCK_DIR = "/opt/homebrew/opt/util-linux/bin";
if (existsSync(join(HOMEBREW_FLOCK_DIR, "flock"))) {
  process.env.PATH = `${HOMEBREW_FLOCK_DIR}:${process.env.PATH ?? ""}`;
}

const root = mkdtempSync(join(tmpdir(), "awiki-lock-"));
mkdirSync(join(root, ".awiki"), { recursive: true });
writeFileSync(join(root, ".awiki/lock"), "");

// Happy path: shared lock, command runs.
{
  const out = runLocked({
    repoRoot: root,
    mode: "s",
    timeoutSec: 5,
    argv: ["echo", "hello"],
  });
  assert.equal(out.toString().trim(), "hello");
}

// Happy path: exclusive lock, command runs.
{
  const out = runLocked({
    repoRoot: root,
    mode: "x",
    timeoutSec: 5,
    argv: ["echo", "world"],
  });
  assert.equal(out.toString().trim(), "world");
}

// Convenience wrappers default to 30s and the right mode.
{
  const out = runLockedShared({
    repoRoot: root,
    timeoutSec: 5,
    argv: ["echo", "shared"],
  });
  assert.equal(out.toString().trim(), "shared");
}
{
  const out = runLockedExclusive({
    repoRoot: root,
    timeoutSec: 5,
    argv: ["echo", "exclusive"],
  });
  assert.equal(out.toString().trim(), "exclusive");
}

// Timeout: a competing exclusive holder forces this call to fail fast.
{
  const blocker = spawn("bash", [
    "-c",
    `flock -x ${join(root, ".awiki/lock")} sleep 3`,
  ]);
  // Give blocker time to acquire.
  await new Promise((r) => setTimeout(r, 300));
  let threw = null;
  try {
    runLocked({
      repoRoot: root,
      mode: "x",
      timeoutSec: 1,
      argv: ["echo", "should-not-reach"],
    });
  } catch (e) {
    threw = e;
  }
  blocker.kill("SIGTERM");
  assert.ok(
    threw instanceof LockTimeoutError,
    `expected LockTimeoutError, got ${threw}`,
  );
}

// Argv must be an array of strings (no shell interpolation).
assert.throws(() =>
  runLocked({ repoRoot: root, mode: "x", timeoutSec: 5, argv: "echo hi" }),
);
assert.throws(() =>
  runLocked({ repoRoot: root, mode: "x", timeoutSec: 5, argv: ["echo", 123] }),
);

// Mode validation.
assert.throws(() =>
  runLocked({ repoRoot: root, mode: "rwx", timeoutSec: 5, argv: ["true"] }),
);

// Timeout must be a positive integer.
assert.throws(() =>
  runLocked({ repoRoot: root, mode: "x", timeoutSec: 0, argv: ["true"] }),
);
assert.throws(() =>
  runLocked({ repoRoot: root, mode: "x", timeoutSec: -1, argv: ["true"] }),
);
assert.throws(() =>
  runLocked({ repoRoot: root, mode: "x", timeoutSec: 1.5, argv: ["true"] }),
);

// Missing lock file: surfaced as an explicit error, not LockTimeoutError.
{
  const root2 = mkdtempSync(join(tmpdir(), "awiki-lock-no-init-"));
  let threw = null;
  try {
    runLocked({
      repoRoot: root2,
      mode: "x",
      timeoutSec: 5,
      argv: ["echo", "x"],
    });
  } catch (e) {
    threw = e;
  }
  assert.ok(threw, "expected error when lock file missing");
  assert.ok(!(threw instanceof LockTimeoutError));
  rmSync(root2, { recursive: true, force: true });
}

rmSync(root, { recursive: true, force: true });
console.log("lock.js: ok");
