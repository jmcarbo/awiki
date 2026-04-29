package query

import "testing"

func TestFormatTable_NoRows(t *testing.T) {
	got := FormatTable(Result{Columns: []string{"a", "b"}, Rows: nil})
	if got != "(no rows)\n" {
		t.Errorf("want %q, got %q", "(no rows)\n", got)
	}
}

func TestFormatTable_OneRow(t *testing.T) {
	r := Result{
		Columns: []string{"name"},
		Rows:    [][]string{{"alice"}},
	}
	want := "| name |\n|---|\n| alice |\n"
	got := FormatTable(r)
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestFormatTable_MultiColumn(t *testing.T) {
	r := Result{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "2"}, {"3", "4"}},
	}
	want := "| a | b |\n|---|---|\n| 1 | 2 |\n| 3 | 4 |\n"
	got := FormatTable(r)
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}

func TestFormatTable_EscapePipe(t *testing.T) {
	r := Result{
		Columns: []string{"val"},
		Rows:    [][]string{{"a|b"}},
	}
	want := "| val |\n|---|\n| a\\|b |\n"
	got := FormatTable(r)
	if got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
