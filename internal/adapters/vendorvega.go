package adapters

import (
	"context"
	"errors"
	"os/exec"
	"path/filepath"
)

// ExecVendorVega implements VendorVega by shelling out to
// scripts/lib/vendor-vega.sh inside the repository root.
type ExecVendorVega struct{}

// Ensure runs `bash <repoRoot>/scripts/lib/vendor-vega.sh` to vendor the Vega
// JavaScript libraries. It is idempotent: re-running is a no-op if the files
// are already present.
func (e ExecVendorVega) Ensure(ctx context.Context, repoRoot string) error {
	script := filepath.Join(repoRoot, "scripts", "lib", "vendor-vega.sh")
	cmd := exec.CommandContext(ctx, "bash", script)
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return &VendorVegaError{Code: exitErr.ExitCode(), Output: string(out)}
	}
	return err
}

// VendorVegaError is returned when the vendor-vega.sh script fails.
type VendorVegaError struct {
	Code   int
	Output string
}

func (e *VendorVegaError) Error() string {
	return e.Output
}
