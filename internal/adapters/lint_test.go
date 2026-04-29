package adapters

import "testing"

func TestExecSynthLintImplementsSynthLint(t *testing.T) {
	var _ SynthLint = (*ExecSynthLint)(nil)
	t.Log("compile-time SynthLint conformance ok")
}
