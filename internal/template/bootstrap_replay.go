package template

import (
	"errors"
	"fmt"
	"os"
)

// BootstrapClassification is the stdout label printed by
// `bootstrap_replay.py classify`: "ok", "dangerous", or "missing".
type BootstrapClassification string

const (
	BootstrapClassOK        BootstrapClassification = "ok"
	BootstrapClassDangerous BootstrapClassification = "dangerous"
	BootstrapClassMissing   BootstrapClassification = "missing"
)

// ClassifyBootstrapStep returns the replay classification for id given
// the bootstrap markdown at bootstrapPath and the manifest at
// manifestPath. Mirrors `bootstrap_replay.py classify`:
//   - "ok"        when the step exists and is not in dangerous IDs
//   - "dangerous" when the step exists and IS in dangerous IDs
//   - "missing"   when the step is absent from the bootstrap doc
//
// Python returns exit 0 for ok/dangerous and exit 1 for missing. The
// Go equivalent surfaces missing as ErrStepNotFound so callers can
// translate to an exit code; ok/dangerous return nil.
func ClassifyBootstrapStep(bootstrapPath, manifestPath, id string) (BootstrapClassification, error) {
	mdData, err := os.ReadFile(bootstrapPath)
	if err != nil {
		return "", err
	}
	bodies := ParseBootstrapBodies(string(mdData))
	if _, ok := bodies[id]; !ok {
		return BootstrapClassMissing, fmt.Errorf("%w: %s", ErrStepNotFound, id)
	}
	manifest, err := LoadManifest(manifestPath)
	if err != nil {
		return "", err
	}
	for _, dangerous := range manifest.Bootstrap.Dangerous.IDs {
		if dangerous == id {
			return BootstrapClassDangerous, nil
		}
	}
	return BootstrapClassOK, nil
}

// ReplayBootstrapStepBody returns the raw step body (for orchestrator
// to surface to the user). Mirrors `bootstrap_replay.py body`. Returns
// ErrStepNotFound if the step is absent.
func ReplayBootstrapStepBody(bootstrapPath, id string) (string, error) {
	data, err := os.ReadFile(bootstrapPath)
	if err != nil {
		return "", err
	}
	bodies := ParseBootstrapBodies(string(data))
	body, ok := bodies[id]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrStepNotFound, id)
	}
	return body, nil
}

// IsBootstrapMissing reports whether err signals a missing step (so
// callers can map to exit code 1).
func IsBootstrapMissing(err error) bool {
	return errors.Is(err, ErrStepNotFound)
}
