package adapters

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
)

// ExecWhisper shells `whisper-cpp` to transcribe an audio file. Mirrors
// the bash invocation in scripts/ingest-audio.sh:21 —
//
//	whisper-cpp -m models/ggml-base.en.bin -otxt -f "$AUDIO"
//
// whisper-cpp writes its plain-text transcription to a sibling
// `<audio>.txt` file. Extract reads that file, returns its contents,
// and removes it (mirrors `mv "${AUDIO}.txt" "$OUT"` in bash, where
// the rename is the side that consumes the txt file).
//
// Like ExecPDFToText, the production adapter does not reproduce
// alternative whisper bindings (whisper.cpp variants, openai-whisper).
// Callers that need a different binary should set
// AWIKI_INGEST_AUDIO_LEGACY=1 to fall back to the bash script.
type ExecWhisper struct{}

// Transcribe runs whisper-cpp against audioPath and returns the
// resulting transcript text. The sibling `<audioPath>.txt` is removed
// after read so the caller does not need to clean up. Stderr is
// preserved in the returned error when the binary exits non-zero.
func (ExecWhisper) Transcribe(ctx context.Context, audioPath string) (string, int, error) {
	cmd := exec.CommandContext(ctx, "whisper-cpp", "-m", "models/ggml-base.en.bin", "-otxt", "-f", audioPath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			msg := stderr.String()
			if msg == "" {
				msg = stdout.String()
			}
			return "", exitErr.ExitCode(), errors.New(msg)
		}
		return "", 127, err
	}
	txtPath := audioPath + ".txt"
	data, readErr := os.ReadFile(txtPath)
	if readErr != nil {
		return "", 1, readErr
	}
	_ = os.Remove(txtPath)
	return string(data), 0, nil
}
