package lint

import (
	"context"
	"sort"

	chartlint "awiki/internal/lint/chart"
	datalint "awiki/internal/lint/data"
	synthlint "awiki/internal/lint/synth"
	tasklint "awiki/internal/lint/task"
	"awiki/internal/wiki"
)

type RuleSet interface {
	Name() string
	Run(context.Context, Options, *wiki.Index) (Collector, error)
}

type ruleRegistry struct {
	ruleSets map[string]RuleSet
}

var defaultRuleRegistry = newRuleRegistry(synthRuleSet{}, taskRuleSet{}, dataRuleSet{}, chartRuleSet{})

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

type taskRuleSet struct{}

func (taskRuleSet) Name() string { return "task" }

func (taskRuleSet) Run(_ context.Context, opts Options, _ *wiki.Index) (Collector, error) {
	var c Collector
	diagnostics, fixes, err := tasklint.Run(tasklint.Options{
		RepoRoot:   opts.RepoRoot,
		ContentDir: opts.ContentDir,
		Fix:        opts.Fix,
		Today:      opts.Today,
	})
	for _, fix := range fixes {
		c.AddFix(FixRecord{File: fix.File, Message: fix.Message})
	}
	for _, diagnostic := range diagnostics {
		message := diagnostic.Message
		if diagnostic.Code != "" {
			message = diagnostic.Code + ": " + message
		}
		c.Add(Diagnostic{
			Level:   Level(diagnostic.Level),
			File:    diagnostic.File,
			Message: message,
		})
	}
	return c, err
}

type dataRuleSet struct{}

func (dataRuleSet) Name() string { return "data" }

func (dataRuleSet) Run(_ context.Context, opts Options, _ *wiki.Index) (Collector, error) {
	var c Collector
	diagnostics, fixes, err := datalint.Run(datalint.Options{
		RepoRoot:   opts.RepoRoot,
		ContentDir: opts.ContentDir,
		OnlyFile:   opts.OnlyFile,
		Fix:        opts.Fix,
	})
	for _, fix := range fixes {
		c.AddFix(FixRecord{File: fix.File, Message: fix.Message})
	}
	for _, diagnostic := range diagnostics {
		c.Add(Diagnostic{
			Level:   Level(diagnostic.Level),
			File:    diagnostic.File,
			Code:    diagnostic.Code,
			Message: diagnostic.Message,
		})
	}
	return c, err
}

type chartRuleSet struct{}

func (chartRuleSet) Name() string { return "chart" }

func (chartRuleSet) Run(_ context.Context, opts Options, _ *wiki.Index) (Collector, error) {
	var c Collector
	diagnostics, fixes, err := chartlint.Run(chartlint.Options{
		RepoRoot:   opts.RepoRoot,
		ContentDir: opts.ContentDir,
		OnlyFile:   opts.OnlyFile,
		Fix:        opts.Fix,
	})
	for _, fix := range fixes {
		c.AddFix(FixRecord{File: fix.File, Message: fix.Message})
	}
	for _, diagnostic := range diagnostics {
		c.Add(Diagnostic{
			Level:   Level(diagnostic.Level),
			File:    diagnostic.File,
			Code:    diagnostic.Code,
			Message: diagnostic.Message,
		})
	}
	return c, err
}
