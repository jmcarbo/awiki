package synth

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// List walks PluginDir, parses every plugin manifest, and emits one
// PLUGIN| record per plugin in name-sorted order.
func (r *Runner) List(out io.Writer) error {
	entries, err := os.ReadDir(r.PluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // no plugin dir → empty list, exit 0
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
			return err
		}
		plugins = append(plugins, p)
	}
	sort.Slice(plugins, func(i, j int) bool { return plugins[i].Name < plugins[j].Name })
	for _, p := range plugins {
		fmt.Fprintf(out, "PLUGIN|%s|version=%s|min=%d|max=%d|sections=%s\n",
			p.Name, p.Version, p.MinSources, p.MaxSources,
			strings.Join(p.RequiredSections, ","))
	}
	return nil
}
