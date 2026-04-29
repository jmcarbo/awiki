package data

import (
	"fmt"
	"strings"

	ds "awiki/internal/dataset"
)

// rowSet is the internal representation used by lintDataset rules. It holds
// rows as maps keyed by header name, which is what the rules-layer expects.
type rowSet struct {
	rows []map[string]any
}

// column is an alias so rules.go can keep using the unqualified name.
type column = ds.Column

// loadRows loads a data file in the given format and returns a rowSet.
func loadRows(path, format string) (rowSet, error) {
	rawRows, err := ds.LoadRows(ds.Format(format), path)
	if err != nil {
		return rowSet{}, err
	}
	return rawRowsToRowSet(rawRows), nil
}

// sampleRows returns a sample of rows for validation (up to 60 total).
func sampleRows(rows []map[string]any) []map[string]any {
	if len(rows) == 0 {
		return rows
	}
	// Collect all keys from all rows to form a synthetic header.
	keys := make([]string, 0)
	keySet := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			if !keySet[k] {
				keySet[k] = true
				keys = append(keys, k)
			}
		}
	}

	matrix := make([][]string, 0, len(rows)+1)
	matrix = append(matrix, keys)
	for _, row := range rows {
		r := make([]string, len(keys))
		for i, k := range keys {
			r[i] = fmt.Sprint(row[k])
		}
		matrix = append(matrix, r)
	}

	sampled := ds.SampleRows(matrix, 0)
	if len(sampled) == 0 {
		return nil
	}
	// Convert back to []map[string]any.
	header := sampled[0]
	out := make([]map[string]any, 0, len(sampled)-1)
	for _, row := range sampled[1:] {
		m := make(map[string]any, len(header))
		for i, k := range header {
			if i < len(row) {
				m[k] = row[i]
			}
		}
		out = append(out, m)
	}
	return out
}

// validateRows validates rows against the column schema and returns error
// messages in the format the lint rules emit.
func validateRows(rows []map[string]any, schema []column) []string {
	if len(rows) == 0 || len(schema) == 0 {
		return nil
	}

	// Collect all keys from all rows to build the header.
	keys := make([]string, 0)
	keySet := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			if !keySet[k] {
				keySet[k] = true
				keys = append(keys, k)
			}
		}
	}

	matrix := make([][]string, 0, len(rows)+1)
	matrix = append(matrix, keys)
	for _, row := range rows {
		r := make([]string, len(keys))
		for i, k := range keys {
			r[i] = fmt.Sprint(row[k])
		}
		matrix = append(matrix, r)
	}

	cols := make([]ds.Column, len(schema))
	copy(cols, schema)

	errs := ds.ValidateRows(matrix, cols)
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		if e.Missing {
			msgs = append(msgs, fmt.Sprintf("row=%d col=%s missing", e.Row, e.Column))
		} else {
			msgs = append(msgs, fmt.Sprintf("row=%d col=%s want=%s got=%q", e.Row, e.Column, e.Want, e.Got))
		}
	}
	return msgs
}

// rawRowsToRowSet converts [][]string (header + data rows) to a rowSet.
func rawRowsToRowSet(rawRows [][]string) rowSet {
	if len(rawRows) == 0 {
		return rowSet{}
	}
	header := rawRows[0]
	rows := make([]map[string]any, 0, len(rawRows)-1)
	for _, record := range rawRows[1:] {
		row := make(map[string]any, len(header))
		for i, h := range header {
			val := ""
			if i < len(record) {
				val = record[i]
			}
			row[h] = val
		}
		rows = append(rows, row)
	}
	return rowSet{rows: rows}
}

// coerces delegates to ds.ValidateRows for type checking. Retained for any
// direct callers within the lint package.
func coerces(value any, ty string) bool {
	if value == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(value))
	return ds.ValidateRows(
		[][]string{{"_col"}, {s}},
		[]ds.Column{{Name: "_col", Type: ty}},
	) == nil
}
