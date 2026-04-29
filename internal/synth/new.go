package synth

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// NewOptions holds arguments for the New verb.
type NewOptions struct {
	Plugin         string
	Topic          string
	ScopeKind      string // "tag"|"slugs"|"query"
	ScopeValue     string
	ExcludeTags    string
	MinLastUpdated string
	Types          string
	AllowPrivate   bool
}

// synthSlugRegexpMatch reports whether s matches ^[a-z0-9][a-z0-9-]*$.
// We reuse the same pattern as the cli-level synthSlugRegexp but keep
// a local definition so the synth package stays independent of cli.
func synthSlugRegexpMatch(s string) bool {
	if len(s) == 0 {
		return false
	}
	for i, c := range s {
		if c >= 'a' && c <= 'z' {
			continue
		}
		if c >= '0' && c <= '9' {
			continue
		}
		if i > 0 && c == '-' {
			continue
		}
		return false
	}
	return true
}

// New implements `awiki synth new <plugin> <topic> --<scope> [options]`.
// It mirrors scripts/synth.sh:cmd_new step by step.
func (r *Runner) New(ctx context.Context, opts NewOptions, stdout, stderr io.Writer) error {
	// Step 1: Validate topic slug.
	if !synthSlugRegexpMatch(opts.Topic) {
		return &ExitError{Code: 1, Msg: "invalid topic-slug: " + opts.Topic}
	}

	// Step 2: Load plugin.
	p, err := LoadPlugin(filepath.Join(r.PluginDir, opts.Plugin+".md"))
	if err != nil {
		return &ExitError{Code: 1, Msg: "plugin load failed: " + opts.Plugin}
	}

	// Target path and privacy.
	target := filepath.Join(r.synthDir(), opts.Topic+"-"+opts.Plugin+".md")
	targetPrivate := isPrivatePath(target)

	// Build the base scope struct from options.
	scope := Scope{
		Kind:           opts.ScopeKind,
		ExcludeTags:    splitCommaList(opts.ExcludeTags),
		MinLastUpdated: opts.MinLastUpdated,
		Types:          splitCommaList(opts.Types),
		AllowPrivate:   true, // first pass: see all pages including private
	}
	switch opts.ScopeKind {
	case "tag":
		scope.Tag = opts.ScopeValue
	case "slugs":
		scope.Slugs = splitCommaList(opts.ScopeValue)
	case "query":
		scope.Query = opts.ScopeValue
	default:
		return &ExitError{Code: 1, Msg: "must pass --tag, --slugs or --query"}
	}

	// Step 4: First pass — privacy detection (resolve with AllowPrivate=true).
	rawSlugs, err := r.resolveScope(ctx, scope, true)
	if err != nil {
		return err
	}
	hasPrivate := false
	for _, s := range rawSlugs {
		tags := readPageTags(r.slugToPath(s))
		if hasTag(tags, "private") {
			hasPrivate = true
			break
		}
	}
	if hasPrivate && !targetPrivate && !opts.AllowPrivate {
		return &ExitError{Code: 2,
			Msg: "private source in scope; either tag the synthesis page private and place under content/private/, or pass --allow-private"}
	}

	// Step 5: Second pass — canonical resolution.
	scope.AllowPrivate = targetPrivate || opts.AllowPrivate
	slugs, err := r.resolveScope(ctx, scope, scope.AllowPrivate)
	if err != nil {
		return err
	}

	// Step 6: Validate min/max sources.
	if p.MinSources > 0 && len(slugs) < p.MinSources {
		return &ExitError{Code: 2,
			Msg: fmt.Sprintf("scope resolves to %d sources; plugin requires min_sources=%d", len(slugs), p.MinSources)}
	}
	if p.MaxSources > 0 && len(slugs) > p.MaxSources {
		return &ExitError{Code: 2,
			Msg: fmt.Sprintf("scope resolves to %d sources; plugin allows max_sources=%d", len(slugs), p.MaxSources)}
	}

	// Step 7: Refuse overwrite.
	if _, err := os.Stat(target); err == nil {
		return &ExitError{Code: 3, Msg: "target already exists: " + target + " — use 'synth.sh regen' to refresh"}
	}

	// Step 8: Compute scope hash.
	hash := ScopeHash(slugs)

	// Step 9: Build scope YAML block.
	scopeBlock := buildScopeBlock(opts.ScopeKind, opts.ScopeValue,
		opts.ExcludeTags, opts.MinLastUpdated, opts.Types)

	// Step 10: Write scaffold.
	scaffold := WriteScaffold(ScaffoldInput{
		Topic:       opts.Topic,
		Plugin:      opts.Plugin,
		Today:       r.today(),
		ScopeBlock:  scopeBlock,
		SourcesYAML: "sources: []\n",
		ScopeHash:   hash,
	})
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, scaffold, 0o644); err != nil {
		return err
	}

	// Step 11: Emit prompt to stdout.
	scopeDesc := newScopeDescription(opts.ScopeKind, opts.ScopeValue, len(slugs))
	if err := r.EmitPrompt(stdout, p, slugs, scopeDesc, ""); err != nil {
		return err
	}

	// Steps 12–13: Log entries.
	r.LogAppend(ctx, "synth-scaffold", opts.Plugin+" "+opts.Topic)
	if hasPrivate && opts.AllowPrivate {
		r.LogAppend(ctx, "synth-declassify", opts.Topic+" sources="+strconv.Itoa(len(slugs)))
	}

	// Step 14: Stderr telemetry.
	fmt.Fprintf(stderr, "SYNTH-NEW|target=%s|plugin=%s|sources=%d|scope_hash=%s\n",
		target, opts.Plugin, len(slugs), hash)
	return nil
}

// buildScopeBlock assembles the scope: YAML block byte-equivalent to bash.
// The returned string always ends with a newline.
func buildScopeBlock(kind, val, excludeTags, minLU, types string) string {
	var b strings.Builder
	b.WriteString("scope:\n")
	switch kind {
	case "tag":
		b.WriteString("  tag: " + val + "\n")
	case "slugs":
		b.WriteString("  slugs: [" + val + "]\n")
	case "query":
		b.WriteString("  query: \"" + val + "\"\n")
	}
	if excludeTags != "" {
		b.WriteString("  exclude_tags: [" + excludeTags + "]\n")
	}
	if minLU != "" {
		b.WriteString("  min_last_updated: " + minLU + "\n")
	}
	if types != "" {
		b.WriteString("  types: [" + types + "]\n")
	}
	return b.String()
}

// newScopeDescription returns the human-readable scope description used
// in the `new` prompt bundle. Matches bash cmd_new scope_desc strings.
func newScopeDescription(kind, val string, n int) string {
	switch kind {
	case "tag":
		return "pages tagged '" + val + "'"
	case "slugs":
		return fmt.Sprintf("explicit slug list (%d pages)", n)
	case "query":
		return "qmd query: " + val
	}
	return ""
}
