package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
)

// RunPluginPostHook runs the plugin's post-hook script if declared
// and gated by ALLOW_PLUGIN_POST_HOOKS=1 in .awiki/config. Stderr
// emits status records mirroring scripts/synth.sh.
//
// PostHook may be either a path to a script (legacy bash form) or a
// whitespace-separated command (e.g. `awiki synth-mindmap-validate`).
// Multi-token commands are not stat'd; we hand them to the adapter as
// program + extra args.
func (r *Runner) RunPluginPostHook(ctx context.Context, plugin Plugin, pagePath string, stderr io.Writer) error {
	if plugin.PostHook == "" {
		return nil
	}
	if !r.AllowPostHooks() {
		fmt.Fprintln(stderr, "SYNTH|post-hook-blocked|ALLOW_PLUGIN_POST_HOOKS=0; set to 1 in .awiki/config to enable")
		return nil
	}
	tokens := strings.Fields(plugin.PostHook)
	if len(tokens) == 0 {
		return nil
	}
	if len(tokens) == 1 {
		// Legacy single-token form: must point to an executable file.
		info, err := os.Stat(tokens[0])
		if err != nil || (info.Mode()&0o111 == 0 && !info.Mode().IsRegular()) {
			// matches bash: only run if executable or a regular file
			return nil
		}
	}
	out, code, err := r.PostHook.Run(ctx, plugin.PostHook, pagePath)
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "SYNTH|post-hook-failed|%s exit non-zero\n", plugin.PostHook)
		_ = out
		return fmt.Errorf("post-hook failed: %s", plugin.PostHook)
	}
	return nil
}
