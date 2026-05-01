package initverb

import (
	"fmt"
	"path/filepath"

	"awiki/internal/ops"
)

// stepWireAwikiMCP runs Step 9b: prompt to wire the awiki wiki-ops
// MCP server. On consent, npm-installs mcp/awiki-server then calls
// ops.WireAwikiMCP to register the entry.
type stepWireAwikiMCP struct{}

func (stepWireAwikiMCP) ID() string          { return "wire-awiki-mcp" }
func (stepWireAwikiMCP) Description() string { return "wire awiki MCP server" }
func (stepWireAwikiMCP) Kind() Kind          { return KindHybrid }

func (stepWireAwikiMCP) Execute(ctx StepContext, ans *Answers) (Result, error) {
	if !ans.WireAwikiMCP && !ctx.NonInteractive {
		if ctx.Confirm == nil {
			return Result{Status: StatusFailed}, fmt.Errorf("wire-awiki-mcp: Confirm is nil")
		}
		ok, err := ctx.Confirm("Wire awiki wiki-ops MCP server (ingest/lint/query/update_catalog)?", false)
		if err != nil {
			return Result{Status: StatusFailed}, err
		}
		ans.WireAwikiMCP = ok
	}
	if !ans.WireAwikiMCP {
		return Result{Status: StatusSkipped, Note: "wire_awiki_mcp=false"}, nil
	}
	if ctx.NPM == nil {
		return Result{Status: StatusFailed}, fmt.Errorf("wire-awiki-mcp: NPM adapter is nil")
	}
	mcpDir := filepath.Join(ctx.RepoRoot, "mcp", "awiki-server")
	out, code, err := ctx.NPM.Install(ctx.Ctx, mcpDir)
	if out != "" {
		fmt.Fprint(ctx.Stdout, out)
	}
	if err != nil || code != 0 {
		return Result{Status: StatusFailed},
			fmt.Errorf("npm install: code=%d err=%v", code, err)
	}
	rc := ops.WireAwikiMCP(ops.WireAwikiMCPOptions{RepoRoot: ctx.RepoRoot}, ctx.Stdout, ctx.Stderr)
	if rc != 0 {
		return Result{Status: StatusFailed}, fmt.Errorf("wire-awiki-mcp: rc=%d", rc)
	}
	return Result{Status: StatusApplied, Note: "wire_awiki_mcp=true"}, nil
}
