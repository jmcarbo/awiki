package ops

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

type fakeQmd struct {
	out  string
	code int
}

func (f fakeQmd) Search(context.Context, string, string) (string, int, error) {
	return "", 0, nil
}
func (f fakeQmd) Reindex(context.Context, string) (string, int, error) {
	return f.out, f.code, nil
}

func TestReindexOkRoutesToStdout(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Reindex(fakeQmd{out: "QMD-INDEX|ok\n", code: 0}, ".", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stdout.String(), "QMD-INDEX|ok") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr should be empty: %q", stderr.String())
	}
}

func TestReindexSkipRoutesToStderr(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := Reindex(fakeQmd{out: "QMD-INDEX|skip|reason=no-content-dir\n", code: 0}, ".", &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(stderr.String(), "QMD-INDEX|skip|reason=no-content-dir") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestReindexCLIArgsReject(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := ReindexCLI([]string{"extra"}, &stdout, &stderr)
	if code != 2 {
		t.Fatalf("expected 2, got %d", code)
	}
	if !strings.Contains(stderr.String(), "usage:") {
		t.Fatalf("expected usage, got %q", stderr.String())
	}
}
