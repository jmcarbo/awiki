package synth

import (
	"context"
	"fmt"
	"io"
	"os"
)

// RunPluginPostHook runs the plugin's post-hook script if declared
// and gated by ALLOW_PLUGIN_POST_HOOKS=1 in .awiki/config. Stderr
// emits status records mirroring scripts/synth.sh.
func (r *Runner) RunPluginPostHook(ctx context.Context, plugin Plugin, pagePath string, stderr io.Writer) error {
	if plugin.PostHook == "" {
		return nil
	}
	if !r.AllowPostHooks() {
		fmt.Fprintln(stderr, "SYNTH|post-hook-blocked|ALLOW_PLUGIN_POST_HOOKS=0; set to 1 in .awiki/config to enable")
		return nil
	}
	script := plugin.PostHook
	info, err := os.Stat(script)
	if err != nil || (info.Mode()&0o111 == 0 && !info.Mode().IsRegular()) {
		// matches bash: only run if executable or a regular file
		return nil
	}
	out, code, err := r.PostHook.Run(ctx, script, pagePath)
	if err != nil || code != 0 {
		fmt.Fprintf(stderr, "SYNTH|post-hook-failed|%s exit non-zero\n", script)
		_ = out
		return fmt.Errorf("post-hook failed: %s", script)
	}
	return nil
}
