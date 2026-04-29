package adapters

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
)

// ExecDuckDB implements the DuckDB interface by shelling out to the duckdb CLI.
type ExecDuckDB struct{}

// Run executes sql using the duckdb CLI in the given format (-csv, -json, -line).
// format should be one of "csv", "json", "line" (without the leading dash).
// Returns stdout content, exit code, and any error.
func (ExecDuckDB) Run(ctx context.Context, sql, format string) (output string, code int, err error) {
	if format == "" {
		format = "json"
	}
	flag := fmt.Sprintf("-%s", format)
	cmd := exec.CommandContext(ctx, "duckdb", flag, "-c", sql)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	runErr := cmd.Run()
	if runErr != nil {
		if exitErr, ok := runErr.(*exec.ExitError); ok {
			combined := stdout.String()
			if combined == "" {
				combined = stderr.String()
			}
			return combined, exitErr.ExitCode(), nil
		}
		// Command could not start (e.g. duckdb not found).
		return "", 2, runErr
	}
	return stdout.String(), 0, nil
}
