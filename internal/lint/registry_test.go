package lint

import (
	"context"
	"testing"

	"awiki/internal/wiki"
)

type fakeRuleSet struct {
	name string
	ran  bool
}

func (r *fakeRuleSet) Name() string { return r.name }

func (r *fakeRuleSet) Run(_ context.Context, _ Options, _ *wiki.Index) (Collector, error) {
	r.ran = true
	var c Collector
	c.Add(Diagnostic{Level: Info, File: r.name, Code: "X1", Message: "registered namespace ran"})
	return c, nil
}

func TestRunOnlyRegisteredNamespaceSkipsLegacyFallback(t *testing.T) {
	contentDir := t.TempDir()
	writeLintPage(t, contentDir, "entities/source.md", pageContent("Source", "entity", nil, nil, longBody("Source references [[missing-target]] for coverage.")))
	ruleSet := &fakeRuleSet{name: "demo"}
	restore := replaceDefaultRegistryForTest(newRuleRegistry(ruleSet))
	defer restore()
	runner := &fakeRunner{code: 127, output: "legacy should not run"}

	collector, code := Run(Options{ContentDir: contentDir, Only: "demo", Runner: runner})

	if code != 0 {
		t.Fatalf("Run() code = %d, want 0; diagnostics = %#v", code, collector.Diagnostics)
	}
	if !ruleSet.ran {
		t.Fatal("registered rule set did not run")
	}
	assertDiagnosticContains(t, collector, Info, "demo", "registered namespace ran")
	if len(runner.calls) != 0 {
		t.Fatalf("legacy runner was called: %#v", runner.calls)
	}
	assertNoDiagnosticContains(t, collector, Error, "entities/source.md", "broken wikilink")
}
