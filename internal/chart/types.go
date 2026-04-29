package chart

// ChartFence represents a vega-lite fenced code block extracted from a
// markdown page.
type ChartFence struct {
	// ID is the unique chart identifier (slug or slug-figN).
	ID string
	// Spec holds the raw JSON bytes of the Vega-Lite spec.
	Spec []byte
	// PageSlug is the slug of the containing markdown page.
	PageSlug string
	// Index is the 0-based position of this fence within the page.
	Index int
	// IsDedicatedChartPage is true when the page has type:chart AND contains
	// exactly one fence (so the chart ID is the plain slug, not slug-figN).
	IsDedicatedChartPage bool
}
