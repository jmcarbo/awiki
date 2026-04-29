package dataset

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// LoadRows reads a data file in the given format and returns all rows as
// [][]string where row[0] is the header row.
func LoadRows(format Format, path string) ([][]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseRows(format, data)
}

// LoadInlineRows parses in-memory bytes in the given format and returns all
// rows where row[0] is the header row.
func LoadInlineRows(format Format, fenceContent []byte) ([][]string, error) {
	return parseRows(format, fenceContent)
}

// CountRows returns the number of data rows (header not counted) in the file.
func CountRows(format Format, path string) (int, error) {
	rows, err := LoadRows(format, path)
	if err != nil {
		return 0, err
	}
	if len(rows) == 0 {
		return 0, nil
	}
	return len(rows) - 1, nil
}

// SampleRows returns the first n rows. If the total number of rows (including
// header) exceeds 60, it returns the header + first 50 data rows + last 10
// data rows.
func SampleRows(rows [][]string, n int) [][]string {
	if len(rows) == 0 {
		return rows
	}
	// rows[0] is the header; data rows start at rows[1]
	data := rows[1:]
	total := len(data)

	var sampled [][]string
	if total <= 60 {
		sampled = data
	} else {
		sampled = make([][]string, 0, 60)
		sampled = append(sampled, data[:50]...)
		sampled = append(sampled, data[total-10:]...)
	}

	if n > 0 && len(sampled) > n {
		sampled = sampled[:n]
	}

	out := make([][]string, 0, len(sampled)+1)
	out = append(out, rows[0])
	out = append(out, sampled...)
	return out
}

// ValidateRows validates data rows against column schema declarations and
// returns all validation errors found.
func ValidateRows(rows [][]string, columns []Column) []ValidationError {
	if len(rows) == 0 || len(columns) == 0 {
		return nil
	}
	header := rows[0]
	// Build column-index map from header.
	colIndex := make(map[string]int, len(header))
	for i, h := range header {
		colIndex[h] = i
	}

	var errs []ValidationError
	for rowIdx, row := range rows[1:] {
		rowNum := rowIdx + 1 // 1-based
		for _, col := range columns {
			idx, ok := colIndex[col.Name]
			if !ok {
				errs = append(errs, ValidationError{
					Row:     rowNum,
					Column:  col.Name,
					Want:    col.Type,
					Missing: true,
				})
				continue
			}
			val := ""
			if idx < len(row) {
				val = row[idx]
			}
			if !coerces(val, col.Type) {
				errs = append(errs, ValidationError{
					Row:    rowNum,
					Column: col.Name,
					Want:   col.Type,
					Got:    val,
				})
			}
		}
	}
	return errs
}

// parseRows dispatches to the appropriate parser for the format.
func parseRows(format Format, data []byte) ([][]string, error) {
	switch format {
	case FormatCSV:
		return parseDelimited(data, ',')
	case FormatTSV:
		return parseDelimited(data, '\t')
	case FormatDSV:
		delim := sniffDelimiter(data)
		return parseDelimited(data, delim)
	case FormatJSON:
		return parseJSON(data, false)
	case FormatTopoJSON:
		return parseJSON(data, true)
	default:
		return nil, fmt.Errorf("unknown format: %s", format)
	}
}

// sniffDelimiter picks the delimiter from {',', ';', '|', '\t'} by checking
// which one produces the most consistent column count across the first 5 lines.
// The score for each candidate is: (frequency of modal column count) * (modal
// column count). This prefers delimiters that split the data into many
// consistent columns over delimiters that consistently produce only 1 column.
func sniffDelimiter(data []byte) rune {
	candidates := []rune{',', ';', '|', '\t'}
	lines := strings.SplitN(strings.ReplaceAll(string(data), "\r\n", "\n"), "\n", 6)
	// Use up to first 5 non-empty lines.
	var sampleLines []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			sampleLines = append(sampleLines, l)
		}
		if len(sampleLines) == 5 {
			break
		}
	}
	if len(sampleLines) == 0 {
		return ','
	}

	bestDelim := rune(',')
	bestScore := -1

	for _, d := range candidates {
		// Count columns per line for this delimiter.
		counts := make(map[int]int)
		for _, line := range sampleLines {
			n := strings.Count(line, string(d)) + 1
			counts[n]++
		}
		// Score = frequency * columnCount for the modal bucket.
		// Multiplying by columnCount breaks ties in favour of delimiters that
		// actually split the data (e.g. 3 lines × 3 cols beats 3 lines × 1 col).
		for cols, freq := range counts {
			score := freq * cols
			if score > bestScore {
				bestScore = score
				bestDelim = d
			}
		}
	}
	return bestDelim
}

// parseDelimited parses CSV-style data with the given delimiter and returns
// rows[0]=header, rows[1:]=data rows.
func parseDelimited(data []byte, delimiter rune) ([][]string, error) {
	r := csv.NewReader(bytes.NewReader(data))
	r.Comma = delimiter
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	return records, nil
}

// parseJSON parses JSON bytes into rows. For topojson (singleObject=true),
// the entire document is treated as one row. For regular JSON, arrays expand
// into multiple rows and objects become one row.
func parseJSON(data []byte, singleObject bool) ([][]string, error) {
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return nil, err
	}

	if singleObject {
		// TopoJSON: the whole document is one row keyed by top-level keys.
		if m, ok := value.(map[string]any); ok {
			return jsonObjectToRows(m), nil
		}
		// Fallback: single synthetic row.
		return [][]string{{"_"}, {fmt.Sprint(value)}}, nil
	}

	switch typed := value.(type) {
	case []any:
		return jsonArrayToRows(typed)
	case map[string]any:
		return jsonObjectToRows(typed), nil
	default:
		return [][]string{{"_"}, {fmt.Sprint(typed)}}, nil
	}
}

// jsonObjectToRows converts a JSON object into a 2-row slice: [keys], [values].
func jsonObjectToRows(m map[string]any) [][]string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	// Stable sort for determinism in tests.
	sortStrings(keys)
	vals := make([]string, len(keys))
	for i, k := range keys {
		vals[i] = jsonValueString(m[k])
	}
	return [][]string{keys, vals}
}

// jsonArrayToRows converts a JSON array into rows. The header row is derived
// from the union of keys across all objects in the array.
func jsonArrayToRows(arr []any) ([][]string, error) {
	if len(arr) == 0 {
		return [][]string{}, nil
	}

	// Collect ordered unique keys.
	seen := map[string]bool{}
	var keys []string
	for _, item := range arr {
		if m, ok := item.(map[string]any); ok {
			for k := range m {
				if !seen[k] {
					seen[k] = true
					keys = append(keys, k)
				}
			}
		}
	}
	if len(keys) == 0 {
		// Scalar array — use synthetic "_" column.
		header := []string{"_"}
		rows := [][]string{header}
		for _, item := range arr {
			rows = append(rows, []string{jsonValueString(item)})
		}
		return rows, nil
	}

	rows := make([][]string, 0, len(arr)+1)
	rows = append(rows, keys)
	for _, item := range arr {
		row := make([]string, len(keys))
		if m, ok := item.(map[string]any); ok {
			for i, k := range keys {
				row[i] = jsonValueString(m[k])
			}
		} else {
			row[0] = jsonValueString(item)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func jsonValueString(v any) string {
	if v == nil {
		return ""
	}
	switch typed := v.(type) {
	case string:
		return typed
	case float64:
		// Render integers without decimal point.
		if typed == float64(int64(typed)) {
			return strconv.FormatInt(int64(typed), 10)
		}
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		b, _ := json.Marshal(v)
		return string(b)
	}
}

// coerces returns true if the string value is compatible with the type name.
func coerces(s, ty string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true // empty / null always passes
	}
	switch ty {
	case "string":
		return true
	case "integer":
		neg := false
		digits := s
		if strings.HasPrefix(s, "-") {
			neg = true
			digits = s[1:]
		}
		if digits == "" {
			return false
		}
		_ = neg
		for _, r := range digits {
			if r < '0' || r > '9' {
				return false
			}
		}
		return true
	case "number":
		_, err := strconv.ParseFloat(s, 64)
		return err == nil
	case "boolean":
		switch strings.ToLower(s) {
		case "true", "false", "1", "0":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// sortStrings sorts a string slice in place using a simple insertion sort
// (avoids importing "sort" for a small utility).
func sortStrings(ss []string) {
	for i := 1; i < len(ss); i++ {
		for j := i; j > 0 && ss[j] < ss[j-1]; j-- {
			ss[j], ss[j-1] = ss[j-1], ss[j]
		}
	}
}
