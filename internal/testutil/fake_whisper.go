package testutil

import (
	"context"
	"os"

	"awiki/internal/adapters"
)

// FakeWhisper is a test double for adapters.Whisper. It returns canned
// text from an on-disk fixture path (TextPath) or an inline string
// (Text), ignoring the actual audio contents. Tests inject it into
// ingest.Runner.Whisper to drive the ingest-audio verb without
// requiring a real whisper binary or model file.
//
// One of TextPath or Text must be set. TextPath wins when both are
// non-empty; the file is read on every Transcribe call so fixtures can
// share the on-disk test tree. Code/Err let tests simulate adapter
// failures.
type FakeWhisper struct {
	TextPath string
	Text     string
	Code     int
	Err      error
}

// Transcribe returns the canned transcript. The audioPath argument is
// ignored — the fake exists precisely so tests need not provide a real
// audio file.
func (f *FakeWhisper) Transcribe(_ context.Context, _ string) (string, int, error) {
	if f.Err != nil || f.Code != 0 {
		return f.Text, f.Code, f.Err
	}
	if f.TextPath != "" {
		data, err := os.ReadFile(f.TextPath)
		if err != nil {
			return "", 1, err
		}
		return string(data), 0, nil
	}
	return f.Text, 0, nil
}

// Compile-time interface check.
var _ adapters.Whisper = (*FakeWhisper)(nil)
