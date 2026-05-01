package initverb

import (
	"fmt"

	"awiki/internal/ops"
)

// stepWireQmdMCP runs Step 9a: prompt to wire the qmd MCP server,
// only when qmd-status is `ok`. Skipped silently when qmd never
// installed.
type stepWireQmdMCP struct{}

func (stepWireQmdMCP) ID() string          { return "wire-qmd-mcp" }
func (stepWireQmdMCP) Description() string { return "wire qmd MCP server" }
func (stepWireQmdMCP) Kind() Kind          { return KindHybrid }

func (stepWireQmdMCP) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if QmdStatus(ctx.RepoRoot) != "ok" {
		return Result{Status: StatusSkipped, Note: "qmd-status not ok"}, nil
	}
	if !ans.WireQmdMCP && !ctx.NonInteractive {
		if ctx.Confirm == nil {
			return Result{Status: StatusFailed}, fmt.Errorf("wire-qmd-mcp: Confirm is nil")
		}
		ok, err := ctx.Confirm("Wire qmd MCP server into your agent harness?", false)
		if err != nil {
			return Result{Status: StatusFailed}, err
		}
		ans.WireQmdMCP = ok
	}
	if !ans.WireQmdMCP {
		return Result{Status: StatusSkipped, Note: "wire_qmd_mcp=false"}, nil
	}
	rc := ops.WireQmdMCP(ctx.RepoRoot, ctx.Stdout, ctx.Stderr)
	if rc != 0 {
		return Result{Status: StatusFailed}, fmt.Errorf("wire-qmd-mcp: rc=%d", rc)
	}
	return Result{Status: StatusApplied, Note: "wire_qmd_mcp=true"}, nil
}
