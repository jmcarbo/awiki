package lint

import (
	"context"
	"sort"

	synthlint "awiki/internal/lint/synth"
	"awiki/internal/wiki"
)

type RuleSet interface {
	Name() string
	Run(context.Context, Options, *wiki.Index) (Collector, error)
}

type ruleRegistry struct {
	ruleSets map[string]RuleSet
}

var defaultRuleRegistry = newRuleRegistry(synthRuleSet{})

func newRuleRegistry(ruleSets ...RuleSet) *ruleRegistry {
	r := &ruleRegistry{ruleSets: make(map[string]RuleSet)}
	for _, ruleSet := range ruleSets {
		r.register(ruleSet)
	}
	return r
}

func (r *ruleRegistry) register(ruleSet RuleSet) {
	if ruleSet == nil || ruleSet.Name() == "" {
		return
	}
	r.ruleSets[ruleSet.Name()] = ruleSet
}

func (r *ruleRegistry) lookup(name string) (RuleSet, bool) {
	if r == nil {
		return nil, false
	}
	ruleSet, ok := r.ruleSets[name]
	return ruleSet, ok
}

func (r *ruleRegistry) all() []RuleSet {
	if r == nil {
		return nil
	}
	names := make([]string, 0, len(r.ruleSets))
	for name := range r.ruleSets {
		names = append(names, name)
	}
	sort.Strings(names)
	ruleSets := make([]RuleSet, 0, len(names))
	for _, name := range names {
		ruleSets = append(ruleSets, r.ruleSets[name])
	}
	return ruleSets
}

func replaceDefaultRegistryForTest(registry *ruleRegistry) func() {
	previous := defaultRuleRegistry
	defaultRuleRegistry = registry
	return func() {
		defaultRuleRegistry = previous
	}
}

type synthRuleSet struct{}

func (synthRuleSet) Name() string { return "synth" }

func (synthRuleSet) Run(_ context.Context, opts Options, idx *wiki.Index) (Collector, error) {
	var c Collector
	paths, err := synthlint.SynthesisPages(opts.ContentDir, opts.OnlyFile)
	if err != nil {
		return c, err
	}
	if opts.Fix {
		for _, path := range paths {
			fixes, err := synthlint.FixPage(path, opts.ContentDir)
			if err != nil {
				return c, err
			}
			for _, fix := range fixes {
				c.AddFix(FixRecord{File: fix.File, Message: fix.Message})
			}
		}
		pages, err := wiki.DiscoverPages(opts.ContentDir)
		if err != nil {
			return c, err
		}
		next := wiki.BuildIndex(pages)
		idx = &next
	}
	pageByPath := make(map[string]wiki.Page)
	for _, page := range idx.Pages {
		pageByPath[page.Path] = page
		pageByPath[page.RelPath] = page
	}
	for _, path := range paths {
		page, ok := pageByPath[path]
		if !ok {
			parsed, err := wiki.ParsePage(path, opts.ContentDir)
			if err != nil {
				return c, err
			}
			page = parsed
		}
		for _, diagnostic := range synthlint.LintPage(page, opts.RepoRoot, opts.ContentDir, idx) {
			c.Add(Diagnostic{
				Level:   Level(diagnostic.Level),
				File:    diagnostic.File,
				Code:    diagnostic.Code,
				Message: diagnostic.Message,
			})
		}
	}
	return c, nil
}
