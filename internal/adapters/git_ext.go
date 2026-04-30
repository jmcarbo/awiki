package adapters

import "context"

// GitExt extends the basic Git adapter with the clone/init/fetch
// operations required by `awiki ingest-git`. Subsequent slices add the
// ingest-git transformer driver that depends on this interface.
type GitExt interface {
	Git
	Init(ctx context.Context, dir string) (code int, err error)
	Fetch(ctx context.Context, dir, remote string) (code int, err error)
}

// ExecGitExt is the production adapter. Stubs return ErrNotImplemented;
// the real implementation lands in the ingest-git slice.
type ExecGitExt struct{}

func (ExecGitExt) Clone(_ context.Context, _, _ string) (int, error) {
	return 0, ErrNotImplemented
}

func (ExecGitExt) RevParse(_ context.Context, _, _ string) (string, int, error) {
	return "", 0, ErrNotImplemented
}

func (ExecGitExt) Init(_ context.Context, _ string) (int, error) {
	return 0, ErrNotImplemented
}

func (ExecGitExt) Fetch(_ context.Context, _, _ string) (int, error) {
	return 0, ErrNotImplemented
}
