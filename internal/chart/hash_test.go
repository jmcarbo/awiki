package chart

import (
	"testing"
)

func TestHashDeterministic(t *testing.T) {
	spec := map[string]any{
		"mark": "bar",
		"encoding": map[string]any{
			"x": map[string]any{"field": "month", "type": "nominal"},
			"y": map[string]any{"field": "revenue", "type": "quantitative"},
		},
	}
	h1 := Hash(spec)
	h2 := Hash(spec)
	if h1 != h2 {
		t.Errorf("hash not deterministic: %q vs %q", h1, h2)
	}
	if len(h1) != 40 {
		t.Errorf("expected 40-char SHA1 hex, got %d chars: %q", len(h1), h1)
	}
}

func TestHashKeyOrderIndependent(t *testing.T) {
	// Two specs with same content but different key insertion order must
	// hash identically.
	specA := map[string]any{"b": 2, "a": 1}
	specB := map[string]any{"a": 1, "b": 2}
	if Hash(specA) != Hash(specB) {
		t.Error("hash should be key-order independent")
	}
}

func TestHashFixture(t *testing.T) {
	spec := map[string]any{
		"mark": "bar",
		"data": map[string]any{"name": "[[sales]]"},
	}
	h := Hash(spec)
	// Just verify it's a valid hex SHA1 string.
	if len(h) != 40 {
		t.Errorf("expected 40-char SHA1 hex, got %q", h)
	}
}

func TestHashChangesWithSpec(t *testing.T) {
	spec1 := map[string]any{"mark": "bar"}
	spec2 := map[string]any{"mark": "line"}
	if Hash(spec1) == Hash(spec2) {
		t.Error("different specs should have different hashes")
	}
}
