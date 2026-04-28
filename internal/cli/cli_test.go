package cli

import (
	"bytes"
	"testing"
)

func TestRunRequiresCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for missing command")
	}
	if stderr.String() == "" {
		t.Fatalf("expected usage text on stderr")
	}
}

func TestRunUnknownCommand(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Run([]string{"nope"}, &stdout, &stderr)
	if code == 0 {
		t.Fatalf("expected non-zero exit for unknown command")
	}
	if got := stderr.String(); got == "" || !bytes.Contains([]byte(got), []byte("unknown command")) {
		t.Fatalf("stderr = %q, want unknown command message", got)
	}
}
