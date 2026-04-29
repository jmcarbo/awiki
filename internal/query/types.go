package query

// QueryPage represents a query wiki page (type:query).
type QueryPage struct {
	Slug    string
	SQL     string
	OutSlug string
}

// Result holds the parsed output of a DuckDB query.
type Result struct {
	Columns []string
	Rows    [][]string
}

// FenceSpec describes an awiki-query fenced block found in markdown source.
// Start and End are byte offsets into the source text for the entire fence
// (from the opening ``` to the closing ``` inclusive).
type FenceSpec struct {
	SQL        string
	Out        string // optional out=<slug> from info-line
	Start, End int    // byte offsets in source text
}
