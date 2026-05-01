package initverb

import "fmt"

// stepSmokeTest runs Step 13: print the smoke-test instructions.
// Mechanical-only (no user input).
type stepSmokeTest struct{}

func (stepSmokeTest) ID() string          { return "smoke-test" }
func (stepSmokeTest) Description() string { return "smoke test prompt" }
func (stepSmokeTest) Kind() Kind          { return KindMechanical }

func (stepSmokeTest) Execute(ctx StepContext, _ *Answers) (Result, error) {
	fmt.Fprintln(ctx.Stdout, "")
	fmt.Fprintln(ctx.Stdout, "Bootstrap complete. Try the smoke test in README.md to verify everything works:")
	fmt.Fprintln(ctx.Stdout, "  1. Drop a sample source into raw/inbox/interactive/sample.md")
	fmt.Fprintln(ctx.Stdout, "  2. just ingest raw/inbox/interactive/sample.md")
	fmt.Fprintln(ctx.Stdout, "  3. just lint")
	fmt.Fprintln(ctx.Stdout, "  4. just serve")
	fmt.Fprintln(ctx.Stdout, "  5. just search \"test\"   (if qmd installed)")
	return Result{Status: StatusApplied, Note: "smoke-test instructions printed"}, nil
}
