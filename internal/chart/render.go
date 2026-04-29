package chart

import (
	"io"
)

// RenderOptions configures Runner.Render.
type RenderOptions struct {
	KeepOrphans bool
}

// Render walks the content directory for vega-lite fences, renders each chart
// to SVG using VLConvert, writes sidecars, and injects preview regions.
// It is the Go port of cmd_render in scripts/chart.sh.
func (r *Runner) Render(opts RenderOptions, stdout, stderr io.Writer) error {
	return renderImpl(r, opts, stdout, stderr)
}
