package chart

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"awiki/internal/dataset"
)

var dataNameRE = regexp.MustCompile(`^\[\[([a-z0-9][a-z0-9-]*)\]\]$`)

// ResolveSpec resolves [[slug]] dataset references inside a Vega-Lite spec.
//
// It mirrors the behaviour of scripts/lib/vl-resolve.py:
//   - Walk every dict/list in spec recursively.
//   - Find `data` keys whose value is `{"name": "[[<slug>]]"}`.
//   - For storage=file:  replace with {"url": "<baseURL>/<data_path>", "format": {"type": <format>}}.
//   - For storage=inline: replace with {"values": [{col: val, ...}, ...]}.
//
// On failure the error message contains a RESOLVE|<src>|<slug>|<reason> record.
func ResolveSpec(repoRoot, baseURL, src string, spec map[string]any) (map[string]any, error) {
	result, err := walkNode(spec, repoRoot, baseURL, src)
	if err != nil {
		return nil, err
	}
	if m, ok := result.(map[string]any); ok {
		return m, nil
	}
	return spec, nil
}

func walkNode(node any, repoRoot, baseURL, src string) (any, error) {
	switch typed := node.(type) {
	case map[string]any:
		if data, ok := typed["data"]; ok {
			if dataMap, ok := data.(map[string]any); ok {
				resolved, err := resolveDataBlock(dataMap, repoRoot, baseURL, src)
				if err != nil {
					return nil, err
				}
				typed["data"] = resolved
			}
		}
		for k, v := range typed {
			walked, err := walkNode(v, repoRoot, baseURL, src)
			if err != nil {
				return nil, err
			}
			typed[k] = walked
		}
		return typed, nil
	case []any:
		for i, item := range typed {
			walked, err := walkNode(item, repoRoot, baseURL, src)
			if err != nil {
				return nil, err
			}
			typed[i] = walked
		}
		return typed, nil
	default:
		return node, nil
	}
}

func resolveDataBlock(data map[string]any, repoRoot, baseURL, src string) (any, error) {
	nameRaw, ok := data["name"]
	if !ok {
		return data, nil
	}
	name, ok := nameRaw.(string)
	if !ok {
		return data, nil
	}
	m := dataNameRE.FindStringSubmatch(name)
	if m == nil {
		// Not a [[slug]] reference — pass through.
		return data, nil
	}
	slug := m[1]
	return resolveSlug(slug, repoRoot, baseURL, src)
}

func resolveSlug(slug, repoRoot, baseURL, src string) (any, error) {
	pageFile := filepath.Join(repoRoot, "content", "datasets", slug+".md")
	text, err := os.ReadFile(pageFile)
	if err != nil {
		return nil, fmt.Errorf("RESOLVE|%s|%s|not_found", src, slug)
	}
	textStr := string(text)

	pageType, ok := dataset.FmGet(textStr, "type")
	if !ok || pageType != "dataset" {
		return nil, fmt.Errorf("RESOLVE|%s|%s|not_a_dataset", src, slug)
	}

	storage, _ := dataset.FmGet(textStr, "storage")
	format, _ := dataset.FmGet(textStr, "format")

	switch storage {
	case "file":
		dataPath, ok := dataset.FmGet(textStr, "data_path")
		if !ok {
			dataPath = "data/" + slug + "." + format
		}
		url := strings.TrimRight(baseURL, "/") + "/" + strings.TrimLeft(dataPath, "/")
		return map[string]any{
			"url":    url,
			"format": map[string]any{"type": format},
		}, nil

	case "inline":
		content, _, ok := dataset.ExtractDataFence(textStr)
		if !ok {
			content = []byte{}
		}
		values, err := parseInlineValues(content, dataset.Format(format))
		if err != nil {
			return nil, fmt.Errorf("RESOLVE|%s|%s|parse_error: %w", src, slug, err)
		}
		return map[string]any{"values": values}, nil

	default:
		return nil, fmt.Errorf("RESOLVE|%s|%s|bad_storage", src, slug)
	}
}

// parseInlineValues converts fence content to the Vega-Lite values format
// ([]map[string]any with column names as keys).
func parseInlineValues(content []byte, format dataset.Format) ([]map[string]any, error) {
	rows, err := dataset.LoadInlineRows(format, content)
	if err != nil {
		return nil, err
	}
	if len(rows) == 0 {
		return []map[string]any{}, nil
	}
	header := rows[0]
	var out []map[string]any
	for _, row := range rows[1:] {
		rec := make(map[string]any, len(header))
		for i, col := range header {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			rec[col] = val
		}
		out = append(out, rec)
	}
	if out == nil {
		return []map[string]any{}, nil
	}
	return out, nil
}

// parseCSV is kept for reference but we now use dataset.LoadInlineRows.
var _ = parseCSV

func parseCSV(content []byte, delimiter rune) ([]map[string]any, error) {
	r := csv.NewReader(strings.NewReader(string(content)))
	r.Comma = delimiter
	r.FieldsPerRecord = -1
	records, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(records) == 0 {
		return []map[string]any{}, nil
	}
	header := records[0]
	var out []map[string]any
	for _, row := range records[1:] {
		rec := make(map[string]any, len(header))
		for i, col := range header {
			val := ""
			if i < len(row) {
				val = row[i]
			}
			rec[col] = val
		}
		out = append(out, rec)
	}
	return out, nil
}

// MarshalSpec serialises a resolved spec to canonical JSON (compact).
func MarshalSpec(spec map[string]any) ([]byte, error) {
	return json.Marshal(spec)
}
