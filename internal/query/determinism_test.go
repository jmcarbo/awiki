package query

import "testing"

func TestCanonicalHash_RowOrderIndependent(t *testing.T) {
	r1 := Result{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "2"}, {"3", "4"}},
	}
	r2 := Result{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"3", "4"}, {"1", "2"}},
	}
	h1 := CanonicalHash(r1)
	h2 := CanonicalHash(r2)
	if h1 != h2 {
		t.Errorf("same data different order should hash equal: %q vs %q", h1, h2)
	}
}

func TestCanonicalHash_ColumnOrderMatters(t *testing.T) {
	r1 := Result{
		Columns: []string{"a", "b"},
		Rows:    [][]string{{"1", "2"}},
	}
	r2 := Result{
		Columns: []string{"b", "a"},
		Rows:    [][]string{{"1", "2"}},
	}
	if CanonicalHash(r1) == CanonicalHash(r2) {
		t.Error("different column order should produce different hash")
	}
}

func TestCanonicalHash_Deterministic(t *testing.T) {
	r := Result{
		Columns: []string{"name", "value"},
		Rows:    [][]string{{"alice", "100"}, {"bob", "200"}},
	}
	h1 := CanonicalHash(r)
	h2 := CanonicalHash(r)
	if h1 != h2 {
		t.Errorf("hash should be deterministic: %q vs %q", h1, h2)
	}
}

func TestIsDeterministic_Valid(t *testing.T) {
	sql := "SELECT a FROM t ORDER BY a"
	if err := IsDeterministic(sql); err != nil {
		t.Errorf("valid SQL should pass: %v", err)
	}
}

func TestIsDeterministic_NoOrderBy(t *testing.T) {
	sql := "SELECT a FROM t"
	if err := IsDeterministic(sql); err == nil {
		t.Error("SQL without ORDER BY should fail determinism check")
	}
}

func TestIsDeterministic_BannedToken(t *testing.T) {
	sql := "SELECT NOW() FROM t ORDER BY 1"
	if err := IsDeterministic(sql); err == nil {
		t.Error("SQL with NOW() should fail determinism check")
	}
}

func TestIsDeterministic_InnerOrderByNotCounted(t *testing.T) {
	// ORDER BY inside subquery doesn't count as top-level ORDER BY
	sql := "SELECT * FROM (SELECT a FROM t ORDER BY a)"
	if err := IsDeterministic(sql); err == nil {
		t.Error("inner ORDER BY should not satisfy top-level requirement")
	}
}
