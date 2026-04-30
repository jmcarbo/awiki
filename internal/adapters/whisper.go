package adapters

import "context"

// ExecWhisper shells `whisper` to transcribe an audio file. The stub
// method returns ErrNotImplemented; the real implementation lands in
// the ingest-audio slice.
type ExecWhisper struct{}

func (ExecWhisper) Transcribe(_ context.Context, _ string) (string, int, error) {
	return "", 0, ErrNotImplemented
}
