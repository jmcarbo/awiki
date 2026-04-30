package ops

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupCatalogRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content", "entities"))
	mustWrite(t, filepath.Join(tmp, "content", "catalog.md"),
		"---\ntitle: \"Catalog\"\ntype: catalog\n---\n\n# Catalog\n")
	mustWrite(t, filepath.Join(tmp, "content", "entities", "foo.md"),
		"---\ntitle: \"Foo\"\ndate: 2026-04-27\nlast_updated: 2026-04-27\ntype: entity\ntags: []\naliases: []\nsources: []\ndraft: false\n---\n\nBody.\n")
	return tmp
}

func TestUpdateCatalogInsertsEntry(t *testing.T) {
	tmp := setupCatalogRepo(t)
	var stdout, stderr bytes.Buffer
	code := UpdateCatalog(UpdateCatalogOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "CATALOG-OK") {
		t.Fatalf("missing CATALOG-OK: %q", stdout.String())
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "catalog.md"))
	if !strings.Contains(string(body), "[[foo]] — Foo") {
		t.Fatalf("missing entry: %q", body)
	}
	if !strings.Contains(string(body), "## Entities") {
		t.Fatalf("missing section: %q", body)
	}
}

func TestUpdateCatalogPreservesPrelude(t *testing.T) {
	tmp := setupCatalogRepo(t)
	mustWrite(t, filepath.Join(tmp, "content", "catalog.md"),
		"---\ntitle: \"Catalog\"\ntype: catalog\n---\n\n# Catalog\n\nIntro paragraph that must survive.\n\n## Entities\n\n- old entry\n")
	var stdout, stderr bytes.Buffer
	code := UpdateCatalog(UpdateCatalogOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "catalog.md"))
	if !strings.Contains(string(body), "Intro paragraph that must survive.") {
		t.Fatalf("prelude missing: %q", body)
	}
	// Old entries replaced by regenerated ones.
	if strings.Contains(string(body), "- old entry") {
		t.Fatalf("old entry should be removed: %q", body)
	}
}

func TestUpdateCatalogMissingFile(t *testing.T) {
	tmp := t.TempDir()
	mustMkdir(t, filepath.Join(tmp, "content"))
	var stdout, stderr bytes.Buffer
	code := UpdateCatalog(UpdateCatalogOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("expected 1, got %d", code)
	}
	if !strings.Contains(stderr.String(), "no catalog at") {
		t.Fatalf("stderr=%q", stderr.String())
	}
}

func TestUpdateCatalogGroupsByType(t *testing.T) {
	tmp := setupCatalogRepo(t)
	mustMkdir(t, filepath.Join(tmp, "content", "concepts"))
	mustWrite(t, filepath.Join(tmp, "content", "concepts", "axle.md"),
		"---\ntitle: \"Axle\"\ntype: concept\n---\nBody.\n")
	mustMkdir(t, filepath.Join(tmp, "content", "topics"))
	mustWrite(t, filepath.Join(tmp, "content", "topics", "diy.md"),
		"---\ntitle: \"DIY\"\ntype: topic\n---\nBody.\n")
	var stdout, stderr bytes.Buffer
	code := UpdateCatalog(UpdateCatalogOptions{RepoRoot: tmp}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("code=%d", code)
	}
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "catalog.md"))
	for _, want := range []string{"## Entities", "## Concepts", "## Topics", "[[foo]] — Foo", "[[axle]] — Axle", "[[diy]] — DIY"} {
		if !strings.Contains(string(body), want) {
			t.Fatalf("missing %q in catalog: %s", want, body)
		}
	}
	// Order: Entities then Concepts then Topics.
	idxE := strings.Index(string(body), "## Entities")
	idxC := strings.Index(string(body), "## Concepts")
	idxT := strings.Index(string(body), "## Topics")
	if !(idxE < idxC && idxC < idxT) {
		t.Fatalf("section order wrong: %d/%d/%d", idxE, idxC, idxT)
	}
}

func TestUpdateCatalogSkipsLogAndCatalog(t *testing.T) {
	tmp := setupCatalogRepo(t)
	mustWrite(t, filepath.Join(tmp, "content", "log.md"),
		"---\ntitle: \"Log\"\ntype: log\n---\n")
	mustWrite(t, filepath.Join(tmp, "content", "section-index.md"),
		"---\ntitle: \"Index\"\ntype: section-index\n---\n")
	var stdout, stderr bytes.Buffer
	UpdateCatalog(UpdateCatalogOptions{RepoRoot: tmp}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "catalog.md"))
	if strings.Contains(string(body), "[[log]]") || strings.Contains(string(body), "[[catalog]]") {
		t.Fatalf("excluded slugs leaked: %q", body)
	}
}

func TestUpdateCatalogMisc(t *testing.T) {
	tmp := setupCatalogRepo(t)
	mustMkdir(t, filepath.Join(tmp, "content", "weird"))
	mustWrite(t, filepath.Join(tmp, "content", "weird", "thing.md"),
		"---\ntitle: \"Thing\"\ntype: zzz-unknown\n---\nx\n")
	var stdout, stderr bytes.Buffer
	UpdateCatalog(UpdateCatalogOptions{RepoRoot: tmp}, &stdout, &stderr)
	body, _ := os.ReadFile(filepath.Join(tmp, "content", "catalog.md"))
	if !strings.Contains(string(body), "## Misc") {
		t.Fatalf("missing Misc section: %q", body)
	}
	if !strings.Contains(string(body), "[[thing]] — Thing") {
		t.Fatalf("missing thing entry: %q", body)
	}
}
