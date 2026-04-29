package dataset_test

import (
	"os"
	"path/filepath"
	"testing"

	"awiki/internal/dataset"
)

// writeTemp writes content to a temp file with the given extension.
func writeTemp(t *testing.T, content, ext string) string {
	t.Helper()
	f, err := os.CreateTemp(t.TempDir(), "testdata-*"+ext)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return f.Name()
}

func TestLoadRows_CSV(t *testing.T) {
	content := "name,age\nAlice,30\nBob,25\n"
	path := writeTemp(t, content, ".csv")

	rows, err := dataset.LoadRows(dataset.FormatCSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows (header+2), got %d", len(rows))
	}
	if rows[0][0] != "name" || rows[0][1] != "age" {
		t.Errorf("unexpected header: %v", rows[0])
	}
	if rows[1][0] != "Alice" {
		t.Errorf("want Alice, got %s", rows[1][0])
	}
}

func TestLoadRows_TSV(t *testing.T) {
	content := "name\tage\nAlice\t30\nBob\t25\n"
	path := writeTemp(t, content, ".tsv")

	rows, err := dataset.LoadRows(dataset.FormatTSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[1][1] != "30" {
		t.Errorf("want 30, got %s", rows[1][1])
	}
}

func TestLoadRows_DSV_Comma(t *testing.T) {
	content := "a,b,c\n1,2,3\n4,5,6\n"
	path := writeTemp(t, content, ".dsv")
	rows, err := dataset.LoadRows(dataset.FormatDSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
}

func TestLoadRows_DSV_Semicolon(t *testing.T) {
	content := "a;b;c\n1;2;3\n4;5;6\n"
	path := writeTemp(t, content, ".dsv")
	rows, err := dataset.LoadRows(dataset.FormatDSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[0][1] != "b" {
		t.Errorf("unexpected col: %v", rows[0])
	}
}

func TestLoadRows_DSV_Pipe(t *testing.T) {
	content := "a|b|c\n1|2|3\n4|5|6\n"
	path := writeTemp(t, content, ".dsv")
	rows, err := dataset.LoadRows(dataset.FormatDSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
	if rows[1][2] != "3" {
		t.Errorf("unexpected value: %s", rows[1][2])
	}
}

func TestLoadRows_DSV_Tab(t *testing.T) {
	content := "a\tb\tc\n1\t2\t3\n4\t5\t6\n"
	path := writeTemp(t, content, ".dsv")
	rows, err := dataset.LoadRows(dataset.FormatDSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
}

func TestLoadRows_JSON_Array(t *testing.T) {
	content := `[{"name":"Alice","age":30},{"name":"Bob","age":25}]`
	path := writeTemp(t, content, ".json")

	rows, err := dataset.LoadRows(dataset.FormatJSON, path)
	if err != nil {
		t.Fatal(err)
	}
	// header + 2 data rows
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
}

func TestLoadRows_JSON_Object(t *testing.T) {
	content := `{"name":"Alice","age":30}`
	path := writeTemp(t, content, ".json")

	rows, err := dataset.LoadRows(dataset.FormatJSON, path)
	if err != nil {
		t.Fatal(err)
	}
	// header + 1 data row
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (header+1), got %d", len(rows))
	}
}

func TestLoadRows_TopoJSON(t *testing.T) {
	content := `{"type":"Topology","objects":{},"arcs":[]}`
	path := writeTemp(t, content, ".topojson")

	rows, err := dataset.LoadRows(dataset.FormatTopoJSON, path)
	if err != nil {
		t.Fatal(err)
	}
	// header + 1 data row
	if len(rows) != 2 {
		t.Fatalf("want 2 rows (header+1), got %d", len(rows))
	}
}

func TestLoadInlineRows(t *testing.T) {
	content := []byte("x,y\n1,2\n3,4\n")
	rows, err := dataset.LoadInlineRows(dataset.FormatCSV, content)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 3 {
		t.Fatalf("want 3 rows, got %d", len(rows))
	}
}

func TestCountRows(t *testing.T) {
	content := "name,age\nAlice,30\nBob,25\nCarol,40\n"
	path := writeTemp(t, content, ".csv")

	count, err := dataset.CountRows(dataset.FormatCSV, path)
	if err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Errorf("want 3, got %d", count)
	}
}

func TestSampleRows_Small(t *testing.T) {
	// 60 or fewer data rows → all returned
	var rows [][]string
	rows = append(rows, []string{"col"})
	for i := 0; i < 60; i++ {
		rows = append(rows, []string{"val"})
	}
	sampled := dataset.SampleRows(rows, 0)
	if len(sampled) != 61 { // header + 60
		t.Errorf("want 61, got %d", len(sampled))
	}
}

func TestSampleRows_Large(t *testing.T) {
	// More than 60 data rows → header + first 50 + last 10
	var rows [][]string
	rows = append(rows, []string{"col"})
	for i := 0; i < 100; i++ {
		rows = append(rows, []string{"val"})
	}
	sampled := dataset.SampleRows(rows, 0)
	if len(sampled) != 61 { // header + 50 + 10
		t.Errorf("want 61, got %d", len(sampled))
	}
}

func TestValidateRows_Hit(t *testing.T) {
	rows := [][]string{
		{"name", "age", "score", "active"},
		{"Alice", "30", "9.5", "true"},
	}
	cols := []dataset.Column{
		{Name: "name", Type: "string"},
		{Name: "age", Type: "integer"},
		{Name: "score", Type: "number"},
		{Name: "active", Type: "boolean"},
	}
	errs := dataset.ValidateRows(rows, cols)
	if len(errs) != 0 {
		t.Errorf("expected no errors, got %v", errs)
	}
}

func TestValidateRows_Miss_Integer(t *testing.T) {
	rows := [][]string{
		{"val"},
		{"not-an-int"},
	}
	cols := []dataset.Column{{Name: "val", Type: "integer"}}
	errs := dataset.ValidateRows(rows, cols)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
	if errs[0].Want != "integer" {
		t.Errorf("wrong want: %s", errs[0].Want)
	}
}

func TestValidateRows_Miss_Number(t *testing.T) {
	rows := [][]string{{"val"}, {"abc"}}
	cols := []dataset.Column{{Name: "val", Type: "number"}}
	errs := dataset.ValidateRows(rows, cols)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateRows_Miss_Boolean(t *testing.T) {
	rows := [][]string{{"val"}, {"yes"}}
	cols := []dataset.Column{{Name: "val", Type: "boolean"}}
	errs := dataset.ValidateRows(rows, cols)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error, got %d", len(errs))
	}
}

func TestValidateRows_Missing_Column(t *testing.T) {
	rows := [][]string{
		{"name"},
		{"Alice"},
	}
	cols := []dataset.Column{
		{Name: "name", Type: "string"},
		{Name: "missing_col", Type: "integer"},
	}
	errs := dataset.ValidateRows(rows, cols)
	if len(errs) != 1 {
		t.Fatalf("expected 1 error (missing col), got %d", len(errs))
	}
	if !errs[0].Missing {
		t.Error("expected Missing=true")
	}
}

func TestValidateRows_Empty_Value_OK(t *testing.T) {
	rows := [][]string{
		{"val"},
		{""},
	}
	cols := []dataset.Column{{Name: "val", Type: "integer"}}
	errs := dataset.ValidateRows(rows, cols)
	if len(errs) != 0 {
		t.Errorf("empty value should pass type check, got %v", errs)
	}
}

func TestLoadRows_FileNotFound(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nonexistent.csv")
	_, err := dataset.LoadRows(dataset.FormatCSV, path)
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestLoadRows_UnknownFormat(t *testing.T) {
	path := writeTemp(t, "a,b\n1,2\n", ".xyz")
	_, err := dataset.LoadRows("xml", path)
	if err == nil {
		t.Error("expected error for unknown format")
	}
}
