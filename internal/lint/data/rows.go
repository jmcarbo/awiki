package data

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

type rowSet struct {
	rows []map[string]any
}

type column struct {
	Name string
	Type string
}

func loadRows(path, format string) (rowSet, error) {
	switch format {
	case "csv":
		return loadDelimitedRows(path, ',')
	case "tsv":
		return loadDelimitedRows(path, '\t')
	case "dsv":
		data, err := os.ReadFile(path)
		if err != nil {
			return rowSet{}, err
		}
		delimiter := byte(',')
		if lines := strings.SplitN(string(data), "\n", 2); len(lines) > 0 {
			for _, candidate := range []byte{',', ';', '|', '\t'} {
				if strings.ContainsRune(lines[0], rune(candidate)) {
					delimiter = candidate
					break
				}
			}
		}
		return parseDelimitedRows(string(data), rune(delimiter))
	case "json":
		return loadJSONRows(path, false)
	case "topojson":
		return loadJSONRows(path, true)
	default:
		return rowSet{}, fmt.Errorf("unknown format: %s", format)
	}
}

func loadDelimitedRows(path string, delimiter rune) (rowSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rowSet{}, err
	}
	return parseDelimitedRows(string(data), delimiter)
}

func parseDelimitedRows(data string, delimiter rune) (rowSet, error) {
	reader := csv.NewReader(strings.NewReader(data))
	reader.Comma = delimiter
	reader.FieldsPerRecord = -1
	records, err := reader.ReadAll()
	if err != nil {
		return rowSet{}, err
	}
	if len(records) == 0 {
		return rowSet{}, nil
	}
	headers := records[0]
	rows := make([]map[string]any, 0, len(records)-1)
	for _, record := range records[1:] {
		row := make(map[string]any, len(headers))
		for i, header := range headers {
			value := ""
			if i < len(record) {
				value = record[i]
			}
			row[header] = value
		}
		rows = append(rows, row)
	}
	return rowSet{rows: rows}, nil
}

func loadJSONRows(path string, singleObject bool) (rowSet, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return rowSet{}, err
	}
	var value any
	if err := json.Unmarshal(data, &value); err != nil {
		return rowSet{}, err
	}
	if singleObject {
		return rowSet{rows: []map[string]any{{"_": value}}}, nil
	}
	switch typed := value.(type) {
	case []any:
		rows := make([]map[string]any, 0, len(typed))
		for _, item := range typed {
			if m, ok := item.(map[string]any); ok {
				rows = append(rows, m)
			} else {
				rows = append(rows, map[string]any{"_": item})
			}
		}
		return rowSet{rows: rows}, nil
	case map[string]any:
		return rowSet{rows: []map[string]any{typed}}, nil
	default:
		return rowSet{rows: []map[string]any{{"_": typed}}}, nil
	}
}

func sampleRows(rows []map[string]any) []map[string]any {
	if len(rows) <= 60 {
		return rows
	}
	out := make([]map[string]any, 0, 60)
	out = append(out, rows[:50]...)
	out = append(out, rows[len(rows)-10:]...)
	return out
}

func validateRows(rows []map[string]any, schema []column) []string {
	var errs []string
	for i, row := range rows {
		for _, col := range schema {
			value, ok := row[col.Name]
			if !ok {
				errs = append(errs, fmt.Sprintf("row=%d col=%s missing", i+1, col.Name))
				continue
			}
			if !coerces(value, col.Type) {
				errs = append(errs, fmt.Sprintf("row=%d col=%s want=%s got=%q", i+1, col.Name, col.Type, fmt.Sprint(value)))
			}
		}
	}
	return errs
}

func coerces(value any, ty string) bool {
	if value == nil {
		return true
	}
	s := strings.TrimSpace(fmt.Sprint(value))
	if s == "" {
		return true
	}
	switch ty {
	case "string":
		return true
	case "integer":
		if _, ok := value.(bool); ok {
			return false
		}
		if f, ok := value.(float64); ok {
			return f == float64(int64(f))
		}
		if strings.HasPrefix(s, "-") {
			s = s[1:]
		}
		if s == "" {
			return false
		}
		for _, r := range s {
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
