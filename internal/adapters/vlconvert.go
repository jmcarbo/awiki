package adapters

import (
	"context"
	"errors"
	"os/exec"
)

// ExecVLConvert implements VLConvert by shelling out to the vl-convert CLI.
type ExecVLConvert struct{}

// RenderSVG calls `vl-convert vl2svg --input <specPath> --output <outPath>`.
// Returns the process exit code and any error.
func (e ExecVLConvert) RenderSVG(ctx context.Context, specPath, outPath string) (int, error) {
	cmd := exec.CommandContext(ctx, "vl-convert", "vl2svg", "--input", specPath, "--output", outPath)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return exitErr.ExitCode(), &VLConvertError{Code: exitErr.ExitCode(), Output: string(out)}
	}
	return 127, err
}

// VLConvertError is returned when vl-convert exits with a non-zero status.
type VLConvertError struct {
	Code   int
	Output string
}

func (e *VLConvertError) Error() string {
	return e.Output
}
