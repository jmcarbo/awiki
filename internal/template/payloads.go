// Package template ports the awiki template-update domain helpers from
// bash + Python to Go. This file embeds the optional layer payloads
// (wiki-data-layer, wiki-task-layer, wiki-weekly-review) used by the
// init/data-init/task-init verbs.
//
// The payloads are mirrored from scripts/templates/*.md and must remain
// byte-identical to the source. A unit test asserts the copy matches.
package template

import (
	"embed"
	"fmt"
	"io/fs"
)

//go:embed payloads/wiki-data-layer.md payloads/wiki-task-layer.md payloads/wiki-weekly-review.md
var payloadsFS embed.FS

// PayloadID names a known embedded payload.
type PayloadID string

const (
	PayloadDataLayer    PayloadID = "wiki-data-layer"
	PayloadTaskLayer    PayloadID = "wiki-task-layer"
	PayloadWeeklyReview PayloadID = "wiki-weekly-review"
)

// AllPayloadIDs returns every known embedded payload identifier.
func AllPayloadIDs() []PayloadID {
	return []PayloadID{PayloadDataLayer, PayloadTaskLayer, PayloadWeeklyReview}
}

// PayloadBytes returns the raw embedded payload bytes for id, or an
// error if id is unknown.
func PayloadBytes(id PayloadID) ([]byte, error) {
	name := "payloads/" + string(id) + ".md"
	data, err := payloadsFS.ReadFile(name)
	if err != nil {
		return nil, fmt.Errorf("template: payload %s: %w", id, err)
	}
	return data, nil
}

// PayloadFS returns a read-only filesystem view of the embedded
// payloads, rooted at "payloads/". Useful for callers that need to
// walk the set without going through PayloadBytes.
func PayloadFS() fs.FS {
	sub, err := fs.Sub(payloadsFS, "payloads")
	if err != nil {
		// Embed paths are validated at compile time; this should be
		// unreachable.
		panic(fmt.Errorf("template: payload sub-fs: %w", err))
	}
	return sub
}
