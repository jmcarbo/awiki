package synth

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// ErrNoPlugins matches scripts/synth.sh cmd_list: exit 1 with an
// "no parseable plugins under <dir>" message when the plugin
// directory has no parseable manifests.
var ErrNoPlugins = errors.New("no parseable plugins")

// List walks PluginDir, parses every plugin manifest, and emits one
// tab-separated record per plugin in name-sorted order:
//
//	<name>\t<output_type>\t<output_subtype>\t<description>
//
// Matches scripts/synth.sh:cmd_list. Returns ErrNoPlugins if the
// directory has no parseable manifests.
func (r *Runner) List(out io.Writer) error {
	entries, err := os.ReadDir(r.PluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("%w under %s", ErrNoPlugins, r.PluginDir)
		}
		return err
	}
	var plugins []Plugin
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		p, err := LoadPlugin(filepath.Join(r.PluginDir, e.Name()))
		if err != nil {
			// Bash logs malformed plugins and continues. Mirror.
			fmt.Fprintf(os.Stderr, "skipping malformed plugin: %s\n", e.Name())
			continue
		}
		plugins = append(plugins, p)
	}
	if len(plugins) == 0 {
		return fmt.Errorf("%w under %s", ErrNoPlugins, r.PluginDir)
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	for _, p := range plugins {
		fmt.Fprintf(out, "%s\t%s\t%s\t%s\n",
			p.Name, p.OutputType, p.OutputSubtype, p.Description)
	}
	return nil
}
