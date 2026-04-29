package chart

import (
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"sort"
)

// Hash returns the SHA1 hex digest of the canonical JSON representation of
// spec. Canonical JSON has keys sorted at every nesting level, matching the
// sort-key approach used by the bash pipeline.
func Hash(spec map[string]any) string {
	b, _ := json.Marshal(sortedNode(spec))
	h := sha1.Sum(b)
	return fmt.Sprintf("%x", h)
}

// sortedNode recursively converts map[string]any into a value whose JSON
// encoding has sorted keys at every nesting level.
func sortedNode(node any) any {
	switch typed := node.(type) {
	case map[string]any:
		keys := make([]string, 0, len(typed))
		for k := range typed {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		// Use a slice of key-value pairs so the custom marshaler can
		// encode them in the deterministic order we need.
		return &sortedMap{keys: keys, vals: typed}
	case []any:
		out := make([]any, len(typed))
		for i, v := range typed {
			out[i] = sortedNode(v)
		}
		return out
	default:
		return node
	}
}

// sortedMap is a map that marshals its entries in key-sorted order.
type sortedMap struct {
	keys []string
	vals map[string]any
}

func (m *sortedMap) MarshalJSON() ([]byte, error) {
	buf := []byte{'{'}
	for i, k := range m.keys {
		key, err := json.Marshal(k)
		if err != nil {
			return nil, err
		}
		val, err := json.Marshal(sortedNode(m.vals[k]))
		if err != nil {
			return nil, err
		}
		if i > 0 {
			buf = append(buf, ',')
		}
		buf = append(buf, key...)
		buf = append(buf, ':')
		buf = append(buf, val...)
	}
	buf = append(buf, '}')
	return buf, nil
}
