package initverb

import (
	"fmt"
	"strings"
)

// stepDomainImpl runs Step 1: ask which domain bucket the wiki falls
// in. Five canonical letters (A-E) map to fixed labels; any other
// input is treated as free-text and stored verbatim.
//
// In --non-interactive mode the prompt is bypassed and the previously
// recorded answer (or stub default "other") is preserved.
type stepDomain struct{}

func (stepDomain) ID() string          { return "domain" }
func (stepDomain) Description() string { return "pick wiki domain" }
func (stepDomain) Kind() Kind          { return KindInteractive }

var domainLabels = map[string]string{
	"A": "personal",
	"B": "research",
	"C": "reading",
	"D": "business",
	"E": "other",
}

func (stepDomain) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if ans.Domain != "" {
		return Result{Status: StatusApplied, Note: "domain=" + ans.Domain}, nil
	}
	if ctx.NonInteractive {
		ans.Domain = "other"
		return Result{Status: StatusApplied, Note: "domain=other (non-interactive default)"}, nil
	}
	if ctx.Prompt == nil {
		return Result{Status: StatusFailed}, fmt.Errorf("domain: Prompt is nil")
	}
	fmt.Fprintln(ctx.Stdout, "Step 1 — Domain. Pick one:")
	fmt.Fprintln(ctx.Stdout, "  A. Personal")
	fmt.Fprintln(ctx.Stdout, "  B. Research")
	fmt.Fprintln(ctx.Stdout, "  C. Reading")
	fmt.Fprintln(ctx.Stdout, "  D. Business / team")
	fmt.Fprintln(ctx.Stdout, "  E. Other (free text)")
	val, err := ctx.Prompt("Choice (A-E or free text)", "")
	if err != nil {
		return Result{Status: StatusFailed}, err
	}
	val = strings.TrimSpace(val)
	if val == "" {
		return Result{Status: StatusFailed}, fmt.Errorf("domain: empty answer")
	}
	if label, ok := domainLabels[strings.ToUpper(val)]; ok {
		ans.Domain = label
	} else {
		ans.Domain = val
	}
	return Result{Status: StatusApplied, Note: "domain=" + ans.Domain}, nil
}
