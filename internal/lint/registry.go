package lint

import (
	"context"

	"awiki/internal/wiki"
)

type RuleSet interface {
	Name() string
	Run(context.Context, Options, *wiki.Index) (Collector, error)
}

type ruleRegistry struct {
	ruleSets map[string]RuleSet
}

var defaultRuleRegistry = newRuleRegistry()

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

func replaceDefaultRegistryForTest(registry *ruleRegistry) func() {
	previous := defaultRuleRegistry
	defaultRuleRegistry = registry
	return func() {
		defaultRuleRegistry = previous
	}
}
