import { execFileSync, spawnSync } from "node:child_process";
import { realpathSync, statSync, readFileSync } from "node:fs";
import { join } from "node:path";

// Convert a validated scope_descriptor into the orchestrator's CLI flags.
// Order: --tag | --slugs | --query (one only; oneOf already enforced),
// then --exclude-tags, --min-last-updated, --types.
function scopeToArgv(scope) {
  const args = [];
  if ("tag" in scope) {
    args.push(`--tag=${scope.tag}`);
  } else if ("slugs" in scope) {
    args.push(`--slugs=${scope.slugs.join(",")}`);
  } else if ("query" in scope) {
    args.push(`--query=${scope.query}`);
  }
  if (scope.exclude_tags?.length) {
    args.push(`--exclude-tags=${scope.exclude_tags.join(",")}`);
  }
  if (scope.min_last_updated) {
    args.push(`--min-last-updated=${scope.min_last_updated}`);
  }
  if (scope.types?.length) {
    args.push(`--types=${scope.types.join(",")}`);
  }
  return args;
}

// Verify the resolved plugin manifest path is a child of synthesis-plugins/.
// Closes symlink-swap TOCTOU: an attacker who places a symlink at
// synthesis-plugins/foo.md pointing at /etc/passwd cannot escape the directory.
export function assertManifestUnderPluginDir(repoRoot, plugin) {
  const pluginDir = realpathSync(join(repoRoot, "synthesis-plugins"));
  const manifest = join(repoRoot, "synthesis-plugins", `${plugin}.md`);
  let resolved;
  try {
    resolved = realpathSync(manifest);
  } catch (e) {
    throw new Error(`plugin manifest missing or unreadable: ${plugin}`);
  }
  if (!resolved.startsWith(pluginDir + "/") && resolved !== pluginDir) {
    throw new Error(`plugin manifest path escape: ${plugin} resolves outside synthesis-plugins/`);
  }
  // Manifest must be a regular file (not a directory, FIFO, device, etc).
  const st = statSync(resolved);
  if (!st.isFile()) {
    throw new Error(`plugin manifest is not a regular file: ${plugin}`);
  }
  return resolved;
}

// Parse the orchestrator's stderr trailer for a SYNTH-NEW line:
//   SYNTH-NEW|target=<path>|plugin=<plugin>|sources=<n>|scope_hash=<hash>
// Returns {target, sources, scopeHash} on match, null otherwise.
function parseSynthNewTrailer(stderr) {
  const lines = stderr.split("\n");
  for (let i = lines.length - 1; i >= 0; i--) {
    if (lines[i].startsWith("SYNTH-NEW|")) {
      const fields = {};
      for (const part of lines[i].split("|").slice(1)) {
        const eq = part.indexOf("=");
        if (eq < 0) continue;
        fields[part.slice(0, eq)] = part.slice(eq + 1);
      }
      return {
        target: fields.target ?? null,
        sources: fields.sources ?? null,
        scopeHash: fields.scope_hash ?? null,
      };
    }
  }
  return null;
}

export function runSynthesize({ repoRoot, plugin, topicSlug, scope }) {
  const argv = ["scripts/synth.sh", "new", "--", plugin, topicSlug, ...scopeToArgv(scope)];
  const r = spawnSync("bash", argv, {
    encoding: "utf8",
    cwd: repoRoot,
  });
  const stdout = r.stdout ?? "";
  const stderr = r.stderr ?? "";
  const code = r.status;

  if (code !== 0) {
    if (code === 2) {
      return {
        error: "scope_resolution_failed",
        reason: stderr.trim() || "scope resolution returned no sources or out of range",
        suggested_action: "widen the scope (looser tag, more slugs, broader query) or adjust min/max_sources",
      };
    }
    if (code === 3) {
      return {
        error: "target_exists",
        reason: stderr.trim() || `synthesis page already exists for topic_slug=${topicSlug}`,
        suggested_action: `call regen via 'just synth-regen ${topicSlug}-${plugin}' or pick a different topic_slug`,
      };
    }
    if (code === 1) {
      return {
        error: "invalid_plugin",
        reason: stderr.trim() || `plugin ${plugin} not found or invalid`,
        suggested_action: "call list_synth_plugins() and pick a valid name",
      };
    }
    const err = new Error(`synth.sh new exited ${code}: ${stderr.trim()}`);
    err.status = code;
    err.stderr = stderr;
    throw err;
  }

  // Parse the SYNTH-NEW trailer line from stderr to recover target path.
  const trailer = parseSynthNewTrailer(stderr);
  const target_path = trailer?.target ?? `content/synthesis/${topicSlug}-${plugin}.md`;

  // Resolve the synthesis page's scope to a sorted slug list. The orchestrator's
  // `resolve` subcommand prints one slug per line and reuses the same scope
  // resolver as `new` (so the list is deterministic and matches the scope_hash
  // in the BEGIN marker).
  let resolved_slugs = [];
  try {
    const slug = target_path.replace(/^.*\//, "").replace(/\.md$/, "");
    const r2 = spawnSync("bash", ["scripts/synth.sh", "resolve", "--", slug], {
      encoding: "utf8",
      cwd: repoRoot,
    });
    if (r2.status === 0) {
      resolved_slugs = (r2.stdout ?? "")
        .split("\n")
        .map((s) => s.trim())
        .filter(Boolean);
    }
  } catch {
    // Resolution failures are non-fatal for the synthesize() return —
    // the page exists, the agent can resolve later if needed.
  }

  return {
    prompt_bundle: stdout.trimEnd(),
    resolved_slugs,
    target_path,
  };
}

export function runFinalize({ repoRoot, topicSlug }) {
  const argv = ["scripts/synth.sh", "finalize", "--", topicSlug];
  try {
    const stdout = execFileSync("bash", argv, {
      encoding: "utf8",
      cwd: repoRoot,
      stdio: ["ignore", "pipe", "pipe"],
    });
    return { ok: true, output: stdout.trimEnd() };
  } catch (e) {
    const code = e.status;
    const stderr = (e.stderr || "").toString();
    const stdout = (e.stdout || "").toString();
    if (code === 5) {
      return {
        error: "marker_integrity_failed",
        reason: stderr.trim() || "BEGIN/END marker invariants violated",
        suggested_action: "fix markers and call finalize again",
      };
    }
    if (code === 6) {
      return {
        error: "lint_failed",
        reason: stderr.trim() || "synth lint reported errors",
        lint_output: stdout,
        suggested_action: "address lint findings and call finalize again",
      };
    }
    throw e;
  }
}
