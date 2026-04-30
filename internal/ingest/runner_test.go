package ingest

import (
	"path/filepath"
	"testing"
)

// TestRunnerDirs sanity-checks the path helpers wired in the infra
// slice. Subsequent slices add behavioral tests for the verbs that
// consume these directories.
func TestRunnerDirs(t *testing.T) {
	r := &Runner{RepoRoot: "/tmp/awiki"}

	cases := []struct {
		name string
		got  string
		want string
	}{
		{"InboxDir", r.InboxDir(), filepath.Join("/tmp/awiki", "raw", "inbox")},
		{"BatchDir", r.BatchDir(), filepath.Join("/tmp/awiki", "raw", "inbox", "batch")},
		{"ArchiveDir", r.ArchiveDir(), filepath.Join("/tmp/awiki", "raw", "inbox", "_archive")},
		{"GitStateDir", r.GitStateDir(), filepath.Join("/tmp/awiki", ".awiki", "git-state")},
	}
	for _, c := range cases {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
}

// TestExitError confirms the sentinel error type exposes the dispatcher
// contract used by future verb sub-slices.
func TestExitError(t *testing.T) {
	err := &ExitError{Code: 3, Msg: "boom"}
	if err.Error() != "boom" {
		t.Errorf("Error() = %q", err.Error())
	}
	if err.ExitCode() != 3 {
		t.Errorf("ExitCode() = %d", err.ExitCode())
	}
}
