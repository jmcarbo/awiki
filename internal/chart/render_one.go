package chart

import (
	"io"
)

// RenderOne finds the fence with the given chartID in the content directory
// and renders it. It is the Go port of cmd_render_one in scripts/chart.sh.
func (r *Runner) RenderOne(chartID string, stdout, stderr io.Writer) error {
	return renderOneImpl(r, chartID, stdout, stderr)
}
