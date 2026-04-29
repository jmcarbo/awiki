// Package synth implements the awiki synth domain commands
// (awiki synth list|resolve|refine|new|regen|accept-stage|finalize).
package synth

// Plugin is the parsed contents of plugins/synth/<name>.md.
type Plugin struct {
	Name                  string
	Description           string
	OutputType            string
	OutputSubtype         string
	MinSources            int
	MaxSources            int
	MaxEvidenceTotalWords int
	RequiredSections      []string
	PostHook              string
	Render                string
	Version               string
	// Body is the prompt template (file content after the closing
	// `---` of the YAML frontmatter, with a single trailing newline).
	Body string
}

// Scope is the parsed scope block from a synth page frontmatter.
type Scope struct {
	Kind           string // "tag" | "slugs" | "query"
	Tag            string
	Slugs          []string
	Query          string
	ExcludeTags    []string
	MinLastUpdated string
	Types          []string
	AllowPrivate   bool
}

// Page is a synth page on disk plus its parsed metadata.
type Page struct {
	Slug          string
	Path          string
	Plugin        string
	Topic         string
	Scope         Scope
	LastGenerated string
	ScopeHash     string
	Frontmatter   map[string]string
}
