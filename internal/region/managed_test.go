package region

import "testing"

func TestManagedReplaceInsertsWhenAbsent(t *testing.T) {
	in := "intro\n"
	got, err := ManagedReplace(in, "agenda", "today", "line A\nline B\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "intro\n\n<!-- BEGIN agenda:today -->\nline A\nline B\n<!-- END agenda:today -->\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestManagedReplaceUpdatesWhenPresent(t *testing.T) {
	in := "head\n<!-- BEGIN agenda:today -->\nstale\n<!-- END agenda:today -->\ntail\n"
	got, err := ManagedReplace(in, "agenda", "today", "fresh\n")
	if err != nil {
		t.Fatal(err)
	}
	want := "head\n<!-- BEGIN agenda:today -->\nfresh\n<!-- END agenda:today -->\ntail\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestManagedReplaceIsIdempotent(t *testing.T) {
	in := "head\n<!-- BEGIN agenda:today -->\nbody\n<!-- END agenda:today -->\n"
	once, err := ManagedReplace(in, "agenda", "today", "body\n")
	if err != nil {
		t.Fatal(err)
	}
	twice, err := ManagedReplace(once, "agenda", "today", "body\n")
	if err != nil {
		t.Fatal(err)
	}
	if once != twice {
		t.Fatalf("not idempotent:\nonce: %q\ntwice: %q", once, twice)
	}
}

func TestManagedExtract(t *testing.T) {
	in := "x\n<!-- BEGIN k:i -->\nhello\n<!-- END k:i -->\ny\n"
	got, ok := ManagedExtract(in, "k", "i")
	if !ok {
		t.Fatal("expected ok=true")
	}
	if got != "hello" {
		t.Fatalf("got %q want %q", got, "hello")
	}
}

func TestManagedExtractMissing(t *testing.T) {
	if _, ok := ManagedExtract("nothing here\n", "k", "i"); ok {
		t.Fatal("expected ok=false on missing region")
	}
}
