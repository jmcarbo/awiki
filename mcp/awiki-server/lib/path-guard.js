// Path-resolution guard. Run AFTER regex validation has cleared every slug
// arg. Resolves the canonical parent dir and the candidate file path, then
// asserts that the file path is a direct child of the parent (no traversal,
// no nested subdir, no symlink whose realpath escapes the parent).

import { realpathSync, lstatSync } from "node:fs";
import { resolve, sep, dirname } from "node:path";

export class PathGuardError extends Error {
  constructor(message) {
    super(message);
    this.name = "PathGuardError";
  }
}

// repoRoot: absolute path to the repo root.
// parentRel: e.g. "content/projects" — relative to repoRoot.
// slug: the validated slug (regex already passed at the MCP boundary).
// Returns the absolute resolved path; throws PathGuardError on any escape.
export function resolveUnder(repoRoot, parentRel, slug) {
  if (typeof slug !== "string" || slug.length === 0) {
    throw new PathGuardError("slug must be a non-empty string");
  }
  // Reject obvious traversal / nested forms even before resolve(); resolve()
  // would normalize them, but we want to see the raw shape so error messages
  // are useful.
  if (slug === "." || slug === "..") {
    throw new PathGuardError(`slug "${slug}" is not allowed`);
  }
  if (slug.includes("/") || slug.includes(sep) || slug.startsWith(".")) {
    // Note: leading "." excludes ".hidden" too; project / context / topic
    // slugs are never hidden. Underscore-prefix (_loose, _someday) is allowed
    // and is NOT a leading-dot.
    throw new PathGuardError(
      `slug "${slug}" must not contain path separators or leading dot`,
    );
  }

  const canonicalParent = resolve(repoRoot, parentRel);
  const candidate = resolve(canonicalParent, `${slug}.md`);

  // Basename + dirname check on the resolved candidate. The candidate must
  // live directly under canonicalParent, not in a sub-tree.
  if (dirname(candidate) !== canonicalParent) {
    throw new PathGuardError(
      `resolved path ${candidate} is not a direct child of ${canonicalParent}`,
    );
  }

  // If the candidate exists, follow symlinks and re-check. This catches the
  // case where an attacker placed a symlink in content/projects/ pointing at
  // /etc/passwd or out-of-tree content. We use lstatSync so we detect the
  // symlink itself rather than its target — important because a dangling
  // symlink to a missing file should still be rejected.
  let lst;
  try {
    lst = lstatSync(candidate);
  } catch {
    // Does not exist — fine for create-paths (new project, new ref page).
    return candidate;
  }
  if (lst.isSymbolicLink()) {
    // Symlink: resolve to its real target. realpathSync requires the target
    // to exist; for a dangling symlink it throws, which we treat as an
    // escape attempt (safer default).
    let real;
    try {
      real = realpathSync(candidate);
    } catch (e) {
      throw new PathGuardError(
        `symlink ${candidate} could not be resolved (${e.code ?? e.message})`,
      );
    }
    if (dirname(real) !== canonicalParent) {
      throw new PathGuardError(
        `symlink ${candidate} resolves to ${real}, outside ${canonicalParent}`,
      );
    }
  } else {
    // Regular file or dir. Still check realpath in case any ancestor
    // component is a symlink that escapes (paranoid).
    const real = realpathSync(candidate);
    if (dirname(real) !== canonicalParent) {
      throw new PathGuardError(
        `path ${candidate} realpath ${real} escapes ${canonicalParent}`,
      );
    }
  }
  return candidate;
}

// Convenience: resolve all the destination paths a triage_apply call might
// touch given the validated, normalized args. Returns an array of absolute
// paths; throws on any escape. The MCP boundary calls this once before the
// shell-out so the failure mode is "MCP error, nothing written".
export function resolveTriageDestinations(repoRoot, { outcome, params }) {
  const out = [];
  if (params.project_slug) {
    out.push(resolveUnder(repoRoot, "content/projects", params.project_slug));
  }
  if (params.context_slug) {
    out.push(resolveUnder(repoRoot, "content/contexts", params.context_slug));
  }
  if (outcome === "reference" && params.ref_slug && params.page_type) {
    out.push(
      resolveUnder(repoRoot, `content/${params.page_type}s`, params.ref_slug),
    );
  }
  return out;
}
