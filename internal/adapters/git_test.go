package adapters

import "testing"

func TestExecSynthGitImplementsSynthGit(t *testing.T) {
	var _ SynthGit = (*ExecSynthGit)(nil)
	t.Log("compile-time SynthGit conformance ok")
}
