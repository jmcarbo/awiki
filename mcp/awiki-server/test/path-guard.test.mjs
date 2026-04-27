import { resolveUnder, resolveTriageDestinations, PathGuardError } from "../lib/path-guard.js";
import { mkdtempSync, mkdirSync, writeFileSync, symlinkSync, rmSync, realpathSync } from "node:fs";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { strict as assert } from "node:assert";

// On macOS, the system tmpdir is itself a symlink (/var -> /private/var), so
// resolve it once up front so dirname(candidate) comparisons line up with
// realpathSync results.
const root = realpathSync(mkdtempSync(join(tmpdir(), "awiki-pathguard-")));
mkdirSync(join(root, "content/projects"), { recursive: true });
mkdirSync(join(root, "content/contexts"), { recursive: true });
mkdirSync(join(root, "content/topics"), { recursive: true });
writeFileSync(join(root, "content/projects/foo.md"), "ok");

// Happy path: legal slug under canonical parent (file exists).
{
  const p = resolveUnder(root, "content/projects", "foo");
  assert.equal(p, join(root, "content/projects/foo.md"));
}

// Happy path: file does not yet exist (creating new project page).
{
  const p = resolveUnder(root, "content/projects", "newproj");
  assert.equal(p, join(root, "content/projects/newproj.md"));
}

// Underscore-prefix project slug (e.g., _loose) — accepted as long as the
// resolved path is directly under content/projects/.
{
  const p = resolveUnder(root, "content/projects", "_loose");
  assert.equal(p, join(root, "content/projects/_loose.md"));
}

// Reject: parent traversal in slug.
assert.throws(
  () => resolveUnder(root, "content/projects", "../evil"),
  PathGuardError,
);

// Reject: absolute path masquerading as slug.
assert.throws(
  () => resolveUnder(root, "content/projects", "/etc/passwd"),
  PathGuardError,
);

// Reject: nested path in slug.
assert.throws(
  () => resolveUnder(root, "content/projects", "sub/dir"),
  PathGuardError,
);

// Reject: slug containing only dots.
assert.throws(() => resolveUnder(root, "content/projects", ".."), PathGuardError);
assert.throws(() => resolveUnder(root, "content/projects", "."), PathGuardError);

// Reject: empty slug.
assert.throws(() => resolveUnder(root, "content/projects", ""), PathGuardError);

// Reject: slug with leading dot (hidden file).
assert.throws(
  () => resolveUnder(root, "content/projects", ".hidden"),
  PathGuardError,
);

// Reject: symlink whose target escapes the parent.
mkdirSync(join(root, "content/escape-source"), { recursive: true });
writeFileSync(join(root, "content/escape-source/target.md"), "evil");
symlinkSync(
  "../escape-source/target.md",
  join(root, "content/projects/symlink.md"),
);
assert.throws(
  () => resolveUnder(root, "content/projects", "symlink"),
  PathGuardError,
);

// resolveTriageDestinations: empty params yields empty array.
{
  const dests = resolveTriageDestinations(root, { outcome: "trash", params: {} });
  assert.deepEqual(dests, []);
}

// resolveTriageDestinations: project_slug.
{
  const dests = resolveTriageDestinations(root, {
    outcome: "act",
    params: { project_slug: "newproj" },
  });
  assert.deepEqual(dests, [join(root, "content/projects/newproj.md")]);
}

// resolveTriageDestinations: reference outcome resolves under content/<page_type>s/.
{
  const dests = resolveTriageDestinations(root, {
    outcome: "reference",
    params: { ref_slug: "memex", page_type: "topic" },
  });
  assert.deepEqual(dests, [join(root, "content/topics/memex.md")]);
}

// Cleanup.
rmSync(root, { recursive: true, force: true });
console.log("path-guard.js: ok");
